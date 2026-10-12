package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func proxy(target string, gateway bool) http.Handler {
	u, _ := url.Parse(target)
	return &httputil.ReverseProxy{FlushInterval: -1, Rewrite: func(p *httputil.ProxyRequest) {
		p.SetURL(u)
		p.SetXForwarded() // Magpie must treat proxy traffic as remote and require a gateway key.
		p.Out.Header.Set("X-Forwarded-Proto", "https")
		if gateway {
			p.Out.URL.Path = strings.TrimPrefix(p.In.URL.Path, "/gateway")
			p.Out.URL.RawPath = ""
		}
	}, ModifyResponse: func(r *http.Response) error {
		// The public endpoint is HTTPS; protect Magpie's management cookie accordingly.
		cookies := r.Header.Values("Set-Cookie")
		if len(cookies) > 0 {
			r.Header.Del("Set-Cookie")
			for _, c := range cookies {
				if !strings.Contains(strings.ToLower(c), "; secure") {
					c += "; Secure"
				}
				r.Header.Add("Set-Cookie", c)
			}
		}
		return nil
	}}
}

func readiness(w http.ResponseWriter, r *http.Request) {
	c := &http.Client{Timeout: 2 * time.Second}
	for _, target := range []string{"http://127.0.0.1:3425/", "http://127.0.0.1:3430/"} {
		res, err := c.Get(target)
		if err != nil {
			http.Error(w, "starting", 503)
			return
		}
		res.Body.Close()
		if res.StatusCode != 200 && !(strings.Contains(target, ":3430") && res.StatusCode == 401) {
			http.Error(w, "not ready", 503)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"ready"}`)
}

// An opt-in local fixture exercises upstream routing and SSE without vendor credentials.
// It is never routed by the public listener and is off in the published template.
func fixture() {
	m := http.NewServeMux()
	m.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []any{map[string]any{"id": "railway-smoke", "object": "model", "owned_by": "fixture"}}})
	})
	m.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Stream bool `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		if b.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"id\":\"smoke\",\"object\":\"chat.completion.chunk\",\"model\":\"railway-smoke\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"railway-smoke-ok\"},\"finish_reason\":null}]}\n\n")
			w.(http.Flusher).Flush()
			fmt.Fprint(w, "data: {\"id\":\"smoke\",\"object\":\"chat.completion.chunk\",\"model\":\"railway-smoke\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"smoke","object":"chat.completion","model":"railway-smoke","choices":[{"index":0,"message":{"role":"assistant","content":"railway-smoke-ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	})
	go func() { log.Fatal(http.ListenAndServe("127.0.0.1:8090", m)) }()
}

func main() {
	if len(os.Getenv("MAGPIE_WEB_KEY")) < 16 {
		log.Fatal("MAGPIE_WEB_KEY must contain at least 16 URL-safe characters")
	}
	if os.Getenv("SMOKE_TEST_UPSTREAM") == "1" {
		fixture()
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	child := exec.Command("/magpie", "web", "--addr", "127.0.0.1:3430", "--no-open", "--gateway")
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		log.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", readiness)
	m.HandleFunc("/gateway", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/gateway/", http.StatusTemporaryRedirect)
	})
	m.Handle("/gateway/", proxy("http://127.0.0.1:3425", true))
	m.Handle("/", proxy("http://127.0.0.1:3430", false))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: net.JoinHostPort("0.0.0.0", port), Handler: m, ReadHeaderTimeout: 15 * time.Second, IdleTimeout: 120 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Print(err)
			stop()
		}
	}()
	childExited := false
	failed := false
	select {
	case err := <-done:
		childExited = true
		failed = true
		log.Printf("Magpie exited unexpectedly: %v", err)
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	server.Shutdown(shutdown)
	if !childExited {
		child.Process.Signal(syscall.SIGTERM)
		select {
		case err := <-done:
			if err != nil {
				log.Print(err)
			}
		case <-shutdown.Done():
			child.Process.Kill()
		}
	}
	if failed {
		os.Exit(1)
	}
}

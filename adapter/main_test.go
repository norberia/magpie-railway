package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGatewayProxyPreservesAuthenticationAndMarksRemote(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.URL.RawQuery != "key=query-key" {
			t.Errorf("unexpected URL: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer client-key" || r.Header.Get("X-Api-Key") != "anthropic-key" {
			t.Error("lost client credentials")
		}
		if r.Header.Get("X-Forwarded-For") == "" || r.Header.Get("X-Forwarded-For") == "spoofed" {
			t.Error("proxy traffic could be classified as local")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: hello\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	req := httptest.NewRequest("POST", "https://cloud.example/gateway/v1/chat/completions?key=query-key", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer client-key")
	req.Header.Set("X-Api-Key", "anthropic-key")
	req.Header.Set("X-Forwarded-For", "spoofed")
	out := httptest.NewRecorder()
	proxy(upstream.URL, true).ServeHTTP(out, req)
	if out.Code != 200 || !strings.Contains(out.Body.String(), "[DONE]") {
		t.Fatal("SSE forwarding failed")
	}
}

func TestManagementCookieIsSecureAndRedirectIsRelative(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "magpie_web_3430=secret; Path=/; HttpOnly; SameSite=Lax")
		w.Header().Set("Location", "/")
		w.WriteHeader(303)
	}))
	defer upstream.Close()
	out := httptest.NewRecorder()
	proxy(upstream.URL, false).ServeHTTP(out, httptest.NewRequest("GET", "https://cloud.example/?k=secret", nil))
	if !strings.Contains(out.Header().Get("Set-Cookie"), "; Secure") || out.Header().Get("Location") != "/" {
		t.Fatal("management login forwarding failed")
	}
}

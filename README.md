# Magpie on Railway

Deploy a persistent Magpie AI gateway and browser management UI with one HTTPS domain.
Upstream: https://github.com/yetone/magpie (MIT). Pinned image: `ghcr.io/yetone/magpie:0.1.1203`.

## Railway template configuration

- Source: this repository, branch `main`; Dockerfile: `Dockerfile`.
- One replica, sleeping disabled, HTTP domain target port **8080**.
- Attach one Volume at **/config**.
- `PORT=8080`
- `MAGPIE_ADDR=0.0.0.0:3425`
- `MAGPIE_WEB_KEY=${{secret(32)}}` (template expression; independent key per deployment).
- `RAILWAY_RUN_UID=0` (Railway's root-owned Volume; inherited image otherwise uses UID 65532).
- `MAGPIE_PUBLIC_URL=https://${{RAILWAY_PUBLIC_DOMAIN}}/gateway`
- `MAGPIE_WEB_URL=https://${{RAILWAY_PUBLIC_DOMAIN}}`
- Healthcheck `/healthz`, timeout 120 seconds, restart on failure.
- Leave `SMOKE_TEST_UPSTREAM` unset. It enables a loopback-only deterministic test fixture, never a real model.

## First use

1. Read `MAGPIE_WEB_KEY` in your own Railway project's Variables.
2. Open `https://YOUR-DOMAIN/?k=YOUR-WEB-KEY`. Magpie exchanges this for a secure management cookie.
3. Add a provider in Providers, then create a client key under Gateway → Gateway keys.
4. OpenAI clients: Base URL `https://YOUR-DOMAIN/gateway/v1`.
5. Anthropic/Gemini clients: gateway base URL `https://YOUR-DOMAIN/gateway` (append protocol paths as required by the client).
6. Use the enabled Gateway Key as the client's API key. The management key is not a model API key.

Providers, gateway keys, sign-ins, caches and usage persist in `/config`, using the upstream image's HOME/XDG environment.
Subscriptions need their own login; remote OAuth callbacks may need to be pasted into the UI. Claude subscriptions require the upstream Claude Code installation flow.

## Architecture and authentication

The adapter starts Magpie Web on loopback port 3430; Magpie also starts its gateway on 3425.
The only Railway public target is 8080. `/gateway/` is stripped and forwarded to the model gateway;
all other paths go to the management UI. Both retain upstream authentication.
The adapter replaces untrusted forwarding headers and adds X-Forwarded-For, ensuring even loopback proxy
connections are treated by Magpie as remote clients requiring enabled Gateway Keys. Streaming is flushed immediately.
`/healthz` checks the gateway and management listener without upstream model calls.

Do not expose internal ports 3425 or 3430 via additional domains/TCP proxies. Keep the service at one replica.
Railway deployments with a Volume may have a short restart interruption. Back up the Volume before upgrading.
Update the pinned upstream image deliberately and repeat authentication, streaming and persistence checks.

## Local checks

`go test ./...` and `go vet ./...` validate the adapter.
The runtime expects `/magpie`; production validation must run the Docker image, not the adapter alone.

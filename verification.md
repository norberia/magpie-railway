# Railway deployment verification

Verified on 2026-10-12, Asia/Shanghai.

- Repository: https://github.com/norberia/magpie-railway
- Shareable template: https://railway.com/deploy/FehS2N
- Project: https://railway.com/project/5b749157-d6db-4bcf-8262-892ba3bc4125
- Live endpoint: https://magpie-production-7a9c.up.railway.app
- Upstream image: ghcr.io/yetone/magpie:0.1.1203
- Architecture: one service, one 1 GB Volume at /config, one public target port 8080.

| Check | Observed result |
|---|---|
| Go tests and vet | Passed |
| Railway Docker build | Succeeded |
| First startup on empty persistent Volume | Succeeded |
| GET /healthz | 200, status ready |
| Management page and /api/providers without credentials | 401 |
| Management login with deployment's web key | 200; secure cookie |
| GET /gateway/v1/models without client key | 401 |
| Same endpoint with a newly created Gateway Key | 200 |
| Forwarded header claiming loopback, invalid Gateway Key | 401 |
| Model request through Magpie to loopback test upstream | 200, railway-smoke-ok |
| Streamed model request through Magpie | 200, SSE chunks and [DONE] |
| New deployment on the same Volume | Succeeded |
| Provider configuration after new deployment | Retained |
| Same Gateway Key after new deployment | Retained and accepted |
| Management key after new deployment | Accepted |
| Temporary provider and Gateway Key cleanup | Removed; old test key rejected with 401 |
| Test fixture | Disabled in deployed service; unset in template |
| Template deployment form | One service, /config Volume, six preconfigured variables, no required values |

Model response tests used a deterministic fixture bound to 127.0.0.1 inside the container.
No real model-vendor credentials were supplied, and no paid model inference or subscription OAuth was tested.
The live project was deployed from the adapter repository with the template's settings; the generated template's
configuration and deployment form were inspected, but a second fresh project was not deployed from the template.

The template is created and shareable by link. It has not been published to Railway's searchable Marketplace.
Access the management UI with the MAGPIE_WEB_KEY stored in your own project's Railway Variables.
OpenAI-compatible Base URL: https://magpie-production-7a9c.up.railway.app/gateway/v1.

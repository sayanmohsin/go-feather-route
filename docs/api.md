# API reference

The gateway implements the OpenAI-compatible subset needed by the MVP.

## Health

`GET /health/liveliness` is unauthenticated and returns a small JSON health
document. Use it for container liveness, not provider readiness.

## Models

`GET /v1/models` lists configured model aliases. API routes require the gateway
bearer token.

## Chat completions

`POST /v1/chat/completions` accepts an OpenAI-compatible chat request. The
`model`, `messages`, and optional generation parameters are forwarded to the
selected provider. Set `stream: true` for SSE streaming.

See the normative [OpenAPI contract](openapi.yaml) and [streaming guide](streaming.md).

# Go Feather Route

[![CI](https://github.com/sayanmohsin/go-feather-route/actions/workflows/ci.yml/badge.svg)](https://github.com/sayanmohsin/go-feather-route/actions/workflows/ci.yml)
[![Documentation](https://github.com/sayanmohsin/go-feather-route/actions/workflows/docs.yml/badge.svg)](https://sayanmohsin.github.io/go-feather-route/)
[![Docker Pulls](https://img.shields.io/docker/pulls/sayanmohsin/go-feather-route)](https://hub.docker.com/r/sayanmohsin/go-feather-route)
[![Go](https://img.shields.io/badge/go-1.26-00ADD8.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache--2.0-00ADD8.svg)](LICENSE)

## A fast, featherweight model-routing gateway written in Go

Go Feather Route is a small OpenAI-compatible gateway for routing chat
requests to OpenAI-compatible provider endpoints. It is designed for small
VMs, edge services, homelabs, and applications that need a private provider
boundary without a large platform runtime.

The project is early-stage. Review the [feature status](docs/roadmap.md) before
depending on it in production.

## Why Go Feather Route?

- One small static Go binary.
- Streaming responses without buffering complete generations.
- Provider keys stay server-side.
- Bounded request bodies and concurrent work.
- OpenAI-compatible requests and errors.
- No database, Redis, dashboard, or background worker required.
- Optional Thingd MCP integration is designed as a separate connector.

## Quickstart

Prerequisites: Go 1.26 and provider credentials.

```bash
cp .env.example .env
export GOFEATHERROUTE_API_KEY=local-gateway-key
export OPENAI_API_KEY=your-key
go run ./cmd/go-feather-route
```

Check health and models:

```bash
curl http://127.0.0.1:4000/health/liveliness
curl -H 'Authorization: Bearer local-gateway-key' \
  http://127.0.0.1:4000/v1/models
```

Send a chat request:

```bash
curl http://127.0.0.1:4000/v1/chat/completions \
  -H 'Authorization: Bearer local-gateway-key' \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Say hello."}]}'
```

Run the public image:

```bash
docker run --rm -p 4000:4000 \
  -e GOFEATHERROUTE_API_KEY=local-gateway-key \
  -e OPENAI_API_KEY=your-key \
  sayanmohsin/go-feather-route:latest
```

For a reproducible deployment, use the immutable release tag instead of
`latest`:

```bash
docker run --rm -p 4000:4000 \
  -e GOFEATHERROUTE_API_KEY=local-gateway-key \
  -e OPENAI_API_KEY=your-key \
  sayanmohsin/go-feather-route:0.1.0
```

The gateway key protects `/v1/models` and `/v1/chat/completions`. Provider keys
are supplied only at runtime. Never put real credentials in a Dockerfile,
`.env.example`, a committed Compose file, or an image layer.

Try streaming:

```bash
curl -N http://127.0.0.1:4000/v1/chat/completions \
  -H 'Authorization: Bearer local-gateway-key' \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"Tell me a short story."}]}'
```

Configure any supported OpenAI-compatible provider through the provider base
URL, model alias, and runtime secret. See the [Docker guide](docs/docker.md)
and [environment guide](docs/environment.md) for configuration, limits, health
checks, and deployment patterns.

## Architecture

```text
OpenAI-compatible client
          |
          v
   Go Feather Route
    |            |
    v            v
  Provider A  Provider B

Optional later connector:
   Go Feather Route ---> Thingd MCP
```

The router does not embed Thingd and does not access databases directly. The
optional connector uses the authenticated Thingd MCP boundary when enabled.

## Design goals

Go Feather Route favors predictable memory, explicit limits, streaming, clear
configuration, and a narrow dependency surface over dashboards and a large
feature matrix. See the [performance](docs/performance.md),
[security](docs/security.md), and [configuration](docs/configuration.md) guides.

## Documentation

- [Documentation site](https://sayanmohsin.github.io/go-feather-route/)
- [Getting started](docs/getting-started.md)
- [API reference](docs/api.md)
- [OpenAPI contract](docs/openapi.yaml)
- [Environment configuration](docs/environment.md)
- [Streaming](docs/streaming.md)
- [Docker deployment](docs/docker.md)
- [Benchmarks](docs/benchmarks.md)
- [LiteLLM benchmark harness](benchmarks/README.md)
- [Optional Thingd MCP connector](docs/thingd-mcp.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)

## Development

```bash
make fmt
make test
make race
make lint
make security
make bench
```

See [coding standards](docs/standards.md) for the project contract.

## License

Go Feather Route is available under the [Apache License 2.0](LICENSE).

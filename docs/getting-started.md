# Getting started

Install Go 1.27.1, clone the repository, and configure at least one provider.

```bash
git clone https://github.com/sayanmohsin/go-feather-route.git
cd go-feather-route
export GOFEATHERROUTE_API_KEY=local-gateway-key
export OPENAI_API_KEY=your-key
go run ./cmd/go-feather-route
```

Verify the service:

```bash
curl http://127.0.0.1:4000/health/liveliness
curl -H 'Authorization: Bearer local-gateway-key' http://127.0.0.1:4000/v1/models
```

The gateway key protects API routes. Provider keys are used only for outbound
provider calls and must not be sent by clients.

Runtime diagnostics are disabled by default. For local debugging only, set
`GOFEATHERROUTE_PPROF_ADDR=127.0.0.1:6060` before starting the gateway and use
the [health and operations guide](health.md) for profile endpoints.

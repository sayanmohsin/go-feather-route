# Testing

Run the standard checks:

```bash
make fmt-check
make test
make race
make lint
make security
make bench
```

Provider behavior is tested with local `httptest.Server` fixtures. Tests must
cover both complete responses and streaming cancellation paths.

When investigating a stalled response locally:

```bash
GOFEATHERROUTE_PPROF_ADDR=127.0.0.1:6060 go run ./cmd/go-feather-route
go tool pprof http://127.0.0.1:6060/debug/pprof/goroutine
curl 'http://127.0.0.1:6060/debug/pprof/goroutineleak?debug=1'
```

The diagnostics listener is opt-in and separate from the authenticated gateway
listener. Never enable it on a public interface or include profile output in
logs, issues, or committed artifacts.

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

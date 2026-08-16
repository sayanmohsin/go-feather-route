# Contributing

Go Feather Route is a small, standard-library-first Go project. Keep the
runtime dependency graph and memory behavior intentional.

Before opening a pull request:

```bash
make fmt-check
make check
go test -race ./...
```

Code must use contexts for network work, return wrapped errors, avoid logging
secrets or prompts, and preserve streaming without buffering complete upstream
responses. Configuration is parsed only in `internal/config`.

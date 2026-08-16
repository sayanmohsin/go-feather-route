# Coding standards

Go Feather Route uses standard Go formatting and a small, explicit dependency
surface. Run `make fmt` before committing and `make check` before opening a
pull request.

Network operations must accept and propagate `context.Context`. HTTP request
bodies and upstream responses must be bounded or streamed; never buffer an
unbounded provider response. Errors should add operation context with `%w` and
must not expose credentials, authorization headers, prompts, or provider
secrets.

All configuration access belongs in `internal/config`. Runtime packages receive
typed configuration and must not read process environment variables directly.
Public behavior changes require tests, API documentation, and benchmark updates
when the change affects allocations, streaming, routing, or concurrency.

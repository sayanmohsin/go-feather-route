# Go 1.27.1 diagnostics rollout

## Scope

This handoff covers Go Feather Route only. The public Thingd repository and
Thingd Cloud are out of scope.

## Changes

- The module and Docker builder use Go 1.27.1.
- Runtime diagnostics are opt-in through `GOFEATHERROUTE_PPROF_ADDR`.
- Diagnostics run on a separate loopback-only listener and never share the
  authenticated gateway handler.
- Standard runtime profiles and Go 1.27's `goroutineleak` profile are exposed
  for local stream, cancellation, and shutdown investigations.
- Existing `gofmt`, static analysis, security checks, and Nice Code remain the
  normal validation tools. No new formatting or security tool is required.

## Validation

- `go test ./...` passes.
- `go test -race ./...` passes.
- `go vet ./...` passes.
- Environment schema validation, `git diff --check`, and a CGO-free build pass.

## Non-goals

This change does not expose profiling in production, migrate to a new JSON API,
add embedded inference, or adopt experimental `gomodjail`, `gofumpt`, VHS, or
SIMD tooling. Those require separate evidence and review.

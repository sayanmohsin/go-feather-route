# Performance

The performance target is predictable low memory rather than maximum request
throughput. The gateway bounds request bodies and concurrent work and streams
responses without retaining the full generation.

Always benchmark the exact provider, concurrency, request size, and container
limit used in deployment.

For local stream and goroutine investigations, enable the separate diagnostics
listener with `GOFEATHERROUTE_PPROF_ADDR=127.0.0.1:6060`. Capture profiles
outside the repository and compare repeated runs with `benchstat`; do not treat
one profile or one workload as a general performance claim. Check the
`goroutineleak` profile after cancellation and idle-stream workloads.

The [benchmark harness](./benchmarks.md) also records CPU, memory, network I/O,
block I/O, process count, swap, and OOM/restart state for Go Feather Route and
the pinned LiteLLM comparison image.

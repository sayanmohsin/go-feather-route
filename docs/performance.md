# Performance

The performance target is predictable low memory rather than maximum request
throughput. The gateway bounds request bodies and concurrent work and streams
responses without retaining the full generation.

Always benchmark the exact provider, concurrency, request size, and container
limit used in deployment.

The [benchmark harness](./benchmarks.md) also records CPU, memory, network I/O,
block I/O, process count, swap, and OOM/restart state for Go Feather Route and
the pinned LiteLLM comparison image.

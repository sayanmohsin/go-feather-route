# Benchmarks

Go Feather Route has two benchmark layers:

1. Go microbenchmarks for routing, authentication, provider selection, proxying,
   streaming, concurrency, allocations, and bytes allocated.
2. A Docker comparison harness that measures Go Feather Route and pinned LiteLLM
   against the same deterministic fake OpenAI-compatible provider.

## Go benchmarks

Run the native suite:

```bash
go test -bench=. -benchmem ./...
```

Compare two saved benchmark outputs with:

```bash
benchstat before.txt after.txt
```

## LiteLLM comparison

Run each gateway against the same fake provider:

```bash
make benchmark-go
make benchmark-litellm
```

The harness uses the LiteLLM image digest pinned by the Cloud deployment and
does not call a paid model. It records sanitized request results under
`benchmarks/results/` and captures Docker resource samples every 250 ms.

Configure the workload:

```bash
BENCHMARK_REQUESTS=256 BENCHMARK_CONCURRENCY=8 make benchmark-go
BENCHMARK_REQUESTS=256 BENCHMARK_CONCURRENCY=8 BENCHMARK_STREAMING=true make benchmark-litellm
```

## Measurements

Each run records:

- cold start and time to health readiness;
- p50, p95, and p99 latency;
- throughput, errors, and response bytes;
- streaming time-to-first-byte;
- Docker CPU percentage and memory usage/limit;
- network I/O, block I/O, and process count;
- host memory where available;
- final restart and OOM state;
- image and container metadata.

Linux hosts may expose additional cgroup peak-memory and CPU-throttling data.
Docker Desktop reports the VM resource envelope, so host and container values
must be interpreted with the recorded platform and architecture.

## Reference measurement

The reference measurement was recorded on an Apple M1 Max with Docker
Desktop. The Go image ran natively as arm64; the pinned LiteLLM image ran as
amd64 under emulation. Workload: 16 fake-provider requests at concurrency 4.

| Measurement | Go Feather Route | LiteLLM |
| --- | ---: | ---: |
| Observed gateway memory | ~4.5 MiB | ~1,008 MiB |
| p50 proxy latency | 0.64 ms | 24.20 ms |
| p95 proxy latency | 8.03 ms | 823.08 ms |
| Requests per second | 1,522.64 | 17.98 |
| Image size | ~8.7 MB | upstream image |

These figures describe that reference environment and workload; they are not
universal production guarantees.
LiteLLM’s emulated architecture and startup behavior materially affect this
reference result. Run the harness on the target deployment architecture before
making capacity decisions.

## Optional DeepSeek smoke test

The real-provider test is manual only and requires Doppler or another secret
injector. It runs five sequential requests through each gateway with a 64-token
output cap:

```bash
export GOFEATHERROUTE_URL=http://127.0.0.1:4000
export LITELLM_URL=http://127.0.0.1:4001
export GOFEATHERROUTE_API_KEY=...
export LITELLM_API_KEY=...
doppler run -- make benchmark-deepseek
```

The provider key is used by the already-running gateways and is never written
to benchmark output. Do not run this command in CI or commit its results.

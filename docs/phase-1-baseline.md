# Phase 1 baseline and gap audit

Date: 2026-09-05
Source revision: `4ab8a74` (`v0.1.3`)
Scope: standalone Go Feather Route repository

## Scope and decision

This phase establishes an evidence baseline before further runtime changes.
Thingd Cloud remains on LiteLLM. No Thingd Cloud, Compose, Doppler, or
production configuration was changed.

The measurements below are development evidence, not universal performance
claims. Docker comparison measurements were attempted but could not be
completed because Docker Desktop could not fetch the Go builder image metadata.

## Compatibility ledger

| Capability | Status | Evidence and remaining qualification |
| --- | --- | --- |
| Chat completions | Supported | `internal/router/router.go`, `internal/compatibility`, and router tests cover OpenAI-compatible forwarding. |
| Structured JSON responses | Partial | `response_format` is forwarded and covered by fixtures; Cloud-shaped JSON output needs consumer-level qualification. |
| Model aliases and provider routing | Supported | `internal/gateway` and provider-selection tests cover configured aliases and routes. |
| Gateway and provider authentication | Supported | Bearer-key tests cover gateway auth and upstream credentials remain server-side. |
| Provider usage metadata | Supported | Compatibility fixtures verify usage passthrough; Cloud accounting is outside this gateway. |
| Streaming SSE | Partial | Tests cover prompt forwarding, keep-alives, final usage, `[DONE]`, and cancellation; slow clients, stalled providers, and shutdown still need qualification. |
| Client cancellation | Partial | Context cancellation is propagated and tested; end-to-end connection teardown needs lifecycle tests. |
| Retries | Supported with boundary | Retryable status responses are bounded; ambiguous POST transport failures are not replayed after an uncertain write. Consumer expectations need explicit verification. |
| Timeouts and limits | Supported | Request, response, upstream timeout, concurrency, and stream limits are implemented and tested. |
| Embeddings | Supported | Single/batch forwarding, ordering, vector validation, usage, and errors are covered. Worker and project-memory payloads are not yet verified. |
| OpenAI-compatible errors | Partial | Provider status/body handling is covered; the complete Cloud error corpus remains to be replayed. |
| Request IDs | Supported | Request correlation is generated/propagated and covered by tests. |
| Usage, reservations, quotas, tenancy, billing | Not required | These remain consumer responsibilities; Go Feather only forwards provider usage and correlation data. |
| Response caching | Missing in Go Feather; additive | LiteLLM supports in-memory, disk, Redis, semantic, and object-store caching. Caching is a future compatibility feature, not evidence of a LiteLLM gap. |
| RAG and project memory | Missing | Embedding transport alone is not RAG; Thingd-backed workloads require a separate integration qualification. |
| Thingd MCP | Not required | Optional future capability; it is not part of the LiteLLM gateway contract. |

## Implementation audit

Confirmed in the current source:

- One configured HTTP client and transport are reused by the server.
- `httptrace` records connection establishment and first-response timing.
- Streaming forwards response data without buffering the complete upstream body.
- A single stream watchdog is used for idle-timeout handling.
- `[DONE]` detection is bounded across read boundaries.
- Retries are limited to safe pre-output cases; streamed output is never replayed.
- Embedding responses are validated and preserve provider ordering.
- Chat, embeddings, and streams have independent concurrency controls.
- Request and response bodies/errors are bounded.
- Structured logs do not include prompts, request bodies, tokens, or provider keys.

Unproven hypotheses are intentionally retained for the next phase:

- Warm connection reuse may reduce gateway overhead, but cold/warm container
  measurements were not obtained.
- Stream watchdog and per-chunk processing may still be measurable under
  stalled providers or slow clients.
- Embedding validation still parses each response; further optimization needs
  an allocation/profile result.
- Provider/model metric-label bounds need an explicit cardinality test.
- Provider selection appears inexpensive, but no production-shaped profile has
  been collected.

## In-process Go baseline

Command:

```bash
go test -run '^$' -bench=. -benchmem -benchtime=1s -count=1 ./internal/router
```

Environment: macOS arm64, Apple M1 Max, Go 1.27.0, in-process deterministic
fake provider. These numbers measure gateway code paths, not network or model
latency.

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkRouteRequest` | 64,206 | 17,593 | 161 |
| `BenchmarkProviderSelection` | 13.66 | 0 | 0 |
| `BenchmarkAuth` | 3,462 | 2,365 | 47 |
| `BenchmarkNonStreamingProxy` | 76,756 | 17,704 | 161 |
| `BenchmarkStreamingProxy` | 123,390 | 53,521 | 170 |
| `BenchmarkConcurrentRequests` | 59,143 | 18,175 | 161 |

## Container benchmark status

The harness was invoked separately for the immutable Go image and the pinned
LiteLLM image with 16 requests, concurrency 4, and warmup 2. Docker Desktop
stopped during builder-image metadata retrieval for
`golang:1.27-bookworm` with a context deadline. Therefore this phase reports
the following as unavailable rather than zero:

- container p50/p95/p99 latency and throughput;
- streaming TTFB and total duration;
- container RSS/peak RSS, CPU, throttling, I/O, process count, and goroutines;
- cgroup memory/CPU, restart count, OOM state, and swap activity;
- cold/warm connection comparison;
- startup time, image size, and current image digest from a completed run.

Historical artifacts under `benchmarks/results/` are retained for context but
are not treated as before/after evidence for `4ab8a74`.

## Reproduction commands

Run the code-path baseline:

```bash
go test -run '^$' -bench=. -benchmem -benchtime=1s -count=1 ./internal/router
```

Run the container harness when Docker image retrieval is available:

```bash
BENCHMARK_OUTPUT_DIR=/tmp/go-feather-baseline \
BENCHMARK_REQUESTS=16 \
BENCHMARK_CONCURRENCY=4 \
BENCHMARK_WARMUP=2 \
./benchmarks/run.sh go

BENCHMARK_OUTPUT_DIR=/tmp/litellm-baseline \
BENCHMARK_REQUESTS=16 \
BENCHMARK_CONCURRENCY=4 \
BENCHMARK_WARMUP=2 \
./benchmarks/run.sh litellm
```

The compose harness uses the immutable Go image and pinned LiteLLM digest
declared in `benchmarks/docker-compose.yml`. Raw results belong outside the
repository unless explicitly reviewed for publication.

## Recommended next improvement

The next code phase should qualify streaming lifecycle behavior: slow
downstream clients, stalled providers, client disconnects, mid-stream
failures, graceful shutdown, and goroutine/connection cleanup. This is the
highest-confidence compatibility risk identified by the source audit and can
be measured without changing Thingd Cloud.

After that qualification, repeat cold/warm transport measurements. Do not add
caching, RAG, MCP, gRPC, or speculative provider features until a workload or
profile demonstrates a need.

Expected benefit: improved evidence and safer streaming behavior, with low
compatibility risk if behavior-preserving tests pass. Rollback: revert the
focused Go Feather commit; Thingd Cloud remains unchanged and LiteLLM remains
the active reference gateway.

## Validation

Passed before this report-only change:

```text
go test ./...
go test -race ./...
go vet ./...
make lint
make security
go test -run '^$' -bench=. -benchmem -benchtime=1s -count=1 ./internal/router
```

`git diff --check` is run for this commit. Docker comparison is unavailable
until the image metadata fetch succeeds.

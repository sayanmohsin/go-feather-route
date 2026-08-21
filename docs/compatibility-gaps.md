# LiteLLM compatibility gap matrix

This matrix tracks standalone Go Feather Route parity with the LiteLLM
behavior currently consumed by Thingd Cloud. It is an engineering checklist,
not a migration announcement. Cloud remains on LiteLLM.

| Area | Status | Evidence or remaining work | Priority |
| --- | --- | --- | --- |
| Chat endpoint and OpenAI request shape | Supported | Router forwards the bounded request body to `/v1/chat/completions` | P0 |
| JSON-object responses | Partial | `response_format` is forwarded; add Cloud-shaped fixture assertions | P0 |
| Model aliases and provider routing | Supported | Configuration routes model aliases to provider clients | P0 |
| Gateway authentication | Supported | Protected API endpoints require the gateway bearer key | P0 |
| Provider authentication | Supported | Provider keys are injected at runtime and sent upstream | P0 |
| Non-streaming usage passthrough | Supported | Upstream response and headers are copied within response limits | P0 |
| Streaming SSE forwarding | Partial | Forwarding and termination exist; verify Cloud stream parsing and final usage | P0 |
| Client cancellation | Partial | Request context is propagated; add provider-observed cancellation fixture | P0 |
| Bounded retries | Supported | Retry policy covers transport errors and retryable upstream statuses | P1 |
| Timeout behavior | Supported | Request context uses the configured upstream timeout | P1 |
| Embedding endpoint | Supported | Single and batch forwarding plus response validation tests exist | P0 |
| Embedding worker payloads | Not verified | Replay the Cloud worker-shaped batch and failure cases | P0 |
| Project memory workloads | Not verified | Validate indexing, retrieval, dimensions, and rebuild behavior through the gateway | P0 |
| OpenAI-compatible errors | Partial | Error forwarding exists; compare exact Cloud error parsing and status expectations | P1 |
| Request ID propagation | Supported | Gateway request ID is forwarded as `X-Request-ID` | P1 |
| Usage and reservation accounting | Cloud-owned | Go Feather must forward usage; Cloud integration must prove exactly-once settlement later | P0 |
| Cloud caching and quotas | Cloud-owned | Must remain outside the gateway and be tested during a future integration phase | P0 |
| Health and readiness | Supported | Liveness, readiness, status, model status, and metrics endpoints exist | P1 |
| Low-memory operation | Measured baseline | Repeat representative workloads with immutable images and recorded cgroup data | P1 |

## Priority meaning

- **P0** — required before any Cloud integration review.
- **P1** — required before a compatibility-focused release.

The matrix should be updated only when source, tests, or reproducible benchmark
evidence changes the status. It must not be used to enable Go Feather Route in
Thingd Cloud.

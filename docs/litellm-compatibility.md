# LiteLLM compatibility contract

This document defines the Cloud-facing contract that Go Feather Route must
match before it can be considered as a future LiteLLM replacement. Thingd Cloud
continues to use LiteLLM while this contract is implemented and validated.

Thingd Cloud remains the authority for users, projects, operation labels,
reservations, quotas, billing attribution, usage rollups, caching, and Thingd
tool execution. Go Feather Route is only a provider gateway.

## Required operations

| Operation | Wire endpoint | Required behavior | Current Go status |
| --- | --- | --- | --- |
| Chat completion | `POST /v1/chat/completions` | OpenAI-compatible request/response, model routing, usage passthrough | Implemented; contract tests required |
| Structured completion | `POST /v1/chat/completions` | `response_format.type=json_object`, bounded `max_tokens` | Implemented as request passthrough; fixture coverage required |
| Agent streaming | `POST /v1/chat/completions` | SSE forwarding, cancellation, `[DONE]`, optional final usage | Implemented; Cloud-shaped stream coverage required |
| NLQ | `POST /v1/chat/completions` | Structured JSON response and normal error semantics | Uses chat contract |
| Classification | `POST /v1/chat/completions` | Structured JSON response and bounded retries | Uses chat contract |
| Summarization | `POST /v1/chat/completions` | Structured JSON response and cancellation | Uses chat contract |
| Embeddings | `POST /v1/embeddings` | Single/batch input, ordered vectors, dimensions, usage | Implemented; workload integration required |

## Chat request contract

Cloud may send:

- `model`
- `messages`
- `max_tokens`
- `response_format: {"type":"json_object"}`
- `stream`

The gateway must preserve the selected model, provider usage, request ID, and
OpenAI-compatible status/error behavior. Cloud application logic performs any
bounded Thingd capability execution from structured model output; native
provider tool calling is not a prerequisite for this contract.

## Streaming contract

The gateway must:

- Forward SSE chunks promptly without buffering the complete response.
- Preserve JSON chunk structure and `choices[0].delta.content`.
- Preserve optional final usage metadata.
- Preserve `[DONE]`.
- Cancel upstream work when the Cloud client disconnects.
- Apply request, idle, and concurrency limits.
- Surface provider failures before or during a stream.

Cloud accumulates the streamed content and records usage once after successful
completion. The gateway must not create Cloud usage events.

## Embedding contract

Cloud sends `model` and `input`, including batches of approved text values. A
compatible response must provide one vector per input, preserve indexes, use a
consistent dimension, and include provider usage when available. Cloud checks
ordering, dimensions, finite numeric values, and batch limits before storing
vectors.

## Operational contract

The gateway must provide:

- `GET /health/liveliness`
- `GET /ready`
- `GET /status`
- `GET /metrics`
- `GET /v1/models`

Authentication, provider credentials, timeouts, bounded retries, body limits,
concurrency limits, request IDs, readiness, and resource diagnostics belong to
the gateway boundary. Tenant identity, reservations, quotas, usage settlement,
and reporting remain outside it.

## Replacement gate

This contract is necessary but not sufficient for replacement. A future Cloud
integration requires passing the project-memory, embedding-worker, accounting,
cancellation, reliability, and benchmark gates described in the project plan.
Until then, LiteLLM remains the only active Cloud gateway.

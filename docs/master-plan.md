# Go Feather Route master development plan

Planning baseline: 2026-09-05. This document governs future development; it is
not a declaration that every capability below exists or is production-qualified.

## Product purpose

Build a general-purpose, OpenAI-compatible AI routing gateway in Go that is
simple to operate, inexpensive to run, and reliable on small machines. Any
application should be able to connect through a documented compatible API.
Thingd Cloud is an initial compatibility consumer, not a required dependency
or the limit of the product's audience.

The long-term goal is to replace LiteLLM for explicitly qualified workloads,
then expand provider and feature coverage gradually. Go Feather replaces the
gateway, not the underlying language model or inference server. It cannot
guarantee faster provider generation; it can reduce gateway delay, forward
tokens promptly, reuse connections, and avoid provider calls through caching
when explicitly permitted.

Use Go's runtime and standard library effectively. A static Go binary avoids
a Python runtime dependency, but the language alone does not establish better
latency, CPU usage, memory consumption, or reliability.

LiteLLM already supports response caching and semantic caching. Do not base
positioning on their absence. See the [official caching documentation](https://docs.litellm.ai/docs/proxy/caching).
RAG combines retrieval and generation; supporting embeddings alone does not
make a gateway a complete RAG system. Evaluate competitor capabilities against
a pinned release instead of assuming missing functionality.

## Non-negotiable boundaries

- Benchmark-driven iteration takes priority over rapid deployment.
- Preserve documented API and configuration compatibility; introduce new options
  additively and document behavior changes with migration guidance.
- Core routing must need no database, queue, cache service, dashboard, Thingd,
  or retrieval engine. Optional capabilities must have bounded idle overhead.
- Existing applications keep their current gateway until an explicitly approved
  experiment. No automatic Thingd Cloud, Compose, Doppler, or production change.
- Replacement is all-or-nothing for a selected application environment. Do not
  introduce chat/embedding hybrid routing as a migration shortcut.
- Keep LiteLLM available as a reference and rollback path until representative
  workloads and a separate permanent replacement review pass.
- Preserve provider usage and correlation data. Applications remain responsible
  for billing, quotas, reservations, and user/project attribution.

## Baseline and evidence status

At inspected commit `4ab8a74`, the implementation includes chat, embeddings,
SSE forwarding, bearer authentication, local model discovery, health/status/
metrics endpoints, shared HTTP transport, independent embedding concurrency,
and a stream watchdog. These are implementation facts, not complete parity.

Existing tests and documentation need reconciliation before further performance
claims. For example, the gap matrix describes transport-error retries while
the provider code intentionally avoids replaying ambiguous POST failures.
Model alias-to-provider selection must also be distinguished from rewriting
an alias to an upstream model identifier.

Prior short canary and microbenchmark results are historical evidence only.
The previous Docker comparison timed out retrieving its builder image, so it
did not prove improvements from that commit or lower CPU/RSS than LiteLLM.
A benchmark using the older pinned public image cannot qualify newer source.

Use these supporting documents, but resolve conflicts against source and tests:

- [Compatibility contract](litellm-compatibility.md)
- [Gap matrix](compatibility-gaps.md)
- [Performance guidance](performance.md)
- [Benchmark methodology](benchmarks.md)
- [API contract](openapi.yaml)

## Architecture and engineering principles

The supported development and build toolchain is Go 1.27.1, pinned by
`go.mod` and the Docker builder image. Historical benchmark documents retain
the toolchain version used for their measurements.

Keep transport/provider adaptation, routing policy, protocol validation,
configuration, and observability in focused packages with explicit ownership.
Apply SOLID pragmatically: introduce small consumer-owned interfaces where
multiple implementations or test boundaries require them, not for every type.
Prefer composition and readable control flow over generic plugin frameworks.

Use immutable routing/configuration snapshots, reused HTTP transports, bounded
request/response sizes, bounded admission queues, and independent workload
limits. Deadlines must include queueing. Define who closes bodies, cancels
contexts, drains work, and stops timers and goroutines.

Profile before using pools, custom JSON parsing, atomics, lock-free structures,
PGO, GC tuning, or specialized networking. Pools must not retain giant or
sensitive buffers indefinitely. Test GOMEMLIMIT/GOGC/GOMAXPROCS under actual
container limits; a Go heap limit is not a hard process-RSS limit. Prefer the
supported Go toolchain and upgrade analyzers with it. No unsafe code, assembly,
gRPC migration, or framework replacement solely for presumed speed.

Keep prompts and credentials out of logs, metrics, public fixtures, and image
layers. Bound metric labels even when arbitrary provider/model names are
accepted. Separate gateway failures from provider errors without leaking
sensitive upstream diagnostics.

## Phased execution

### M0 — Reconcile contracts and establish reproducible evidence

Inventory LiteLLM-consumed request shapes, model mapping, headers, usage,
streaming, errors, retries, cancellation, embeddings, caching, and accounting
boundaries. Use sanitized generic fixtures; keep private customer and Cloud
operational details out of this public repository.

Create a capability ledger with Supported, Partial, Missing, Incompatible,
and Not required states, each backed by test paths and reproducible evidence.
Audit the existing performance changes before accepting them as improvements.
Exit: reproducible baseline and an ordered gap list; no inherited green status
without supporting evidence. This is the immediate next phase.

### M1 — Qualify core protocol compatibility

Prove chat, structured JSON, usage fields, embeddings (single/batch/token
inputs where required), dimensions, vector ordering, model mapping, request
IDs, authentication, safe headers, and error shapes. Preserve extension fields
when normalizing responses. Reject unsupported behavior explicitly.

Prove retry safety and bounded retry body handling. Never retry after partial
stream output or replay ambiguous accepted POSTs. Test disconnects, timeouts,
malformed responses, final usage, keep-alive frames, split termination markers,
and termination-like text inside normal JSON content.

Exit: deterministic request/response contract suite passes for every required
operation, with remaining unsupported APIs explicitly listed.

### M2 — Qualify streaming and low-memory reliability

Measure and fix slow-reader backpressure, idle timeout races, trace callback
races, bounded queueing, provider stalls, resource isolation, and shutdown.
Embedding saturation must not consume interactive admission slots; measure
shared CPU/network contention as well. Count successful, aborted, and failed
streams exactly once.

Test 64, 128, and 256 MiB gateway limits as research profiles, including a
representative total 1 GiB host budget. Select a supported profile from evidence.
Use at least a one-hour soak for candidate qualification and a 24-hour soak
before a permanent replacement review. Require zero unexpected OOM/restarts,
bounded queues, stable post-warm-up memory, and no accumulating goroutines or
connections. Overload must fail predictably rather than wedge the host.

Exit: all lifecycle tests pass and a reproducible supported resource envelope
is documented. Do not disable OOM killing or depend on unlimited swap.

### M3 — Optimize measured hot paths

Profile transport reuse, DNS/TLS establishment, JSON processing, copying,
stream reads/flushes, logging, metrics, and routing. Compare the same workload
before and after each candidate change. Keep beneficial changes independently
revertible; discard unsupported micro-optimizations.

Report first response headers, first body byte, first content token, inter-token
gaps, and completion time separately. A flushed empty header is not visible
generation. Local model discovery does not need a provider-cache TTL unless
a real remote metadata lookup is introduced.

Exit: repeated evidence demonstrates the claimed benefit and compatibility
and resource gates remain green. Reliability fixes may be retained without a
speedup when described honestly and their overhead is measured.

### M4 — Broaden providers and modern model APIs

Expand from compatible endpoints to explicit provider adapters with a published
capability matrix. Add provider-native authentication and translation only
against fixtures and documented contracts. Qualify hosted and self-hosted
endpoints separately; never claim every model works automatically.

Prioritize structured-output schemas, native tool calling, Responses-style
APIs, multimodal input, and audio/batch APIs by demonstrated consumer demand.
Every addition requires cancellation, streaming, usage, error, and memory tests.
Native tool-call support is protocol handling; it does not authorize the gateway
to execute tools. Add bounded fallback/circuit policies with no post-output
fallback. Keep unsupported features explicit.

Exit: release-specific provider/API support is documented and tested.

### M5 — Optional caching

Start with opt-in exact-response and embedding caches with byte/entry limits,
TTL, eviction, invalidation, bypass, and hit/miss metrics. Keys must include
trusted access scope, provider/model revision, and all output-affecting inputs.
Never share results across unauthorized users or tenants. Do not cache failures
or partial streams. Initially bypass streaming and stateful/tool requests.

Preserve original usage as provenance without reporting a cache hit as a fresh
provider bill. Define this additive metadata contract with consuming applications
before enabling caching. Keep distributed cache adapters optional. Evaluate
semantic caching later with relevance/freshness tests, false-hit rates, memory,
and embedding cost; lower latency alone is insufficient.

Exit: isolation and eviction tests pass, cached/uncached accounting is explicit,
and measured savings exceed cache lookup/maintenance costs for the workload.

### M6 — Optional retrieval and RAG integration

Provide an opt-in external retrieval boundary with document access checks,
source attribution, bounded context size, retrieval deadlines, and failure
policy. Keep ingestion/indexing outside the core gateway. Benchmark retrieval
quality and end-to-end latency independently of pure routing.

Thingd may connect externally through MCP; it is never embedded or required.
Start read-only with tool allowlists and bounded loops. Retrieved content is
untrusted data, not executable instructions. Write/destructive capabilities
require a separate explicit policy. Core-only deployments remain useful.

Exit: retrieval relevance, authorization, citations, cancellation, and resource
cost are tested. Do not claim RAG quality from embedding endpoint coverage.

### M7 — Release and operations qualification

Build minimal non-root static amd64/arm64 images with CA certificates,
healthchecks, version metadata, SBOM, provenance, and immutable digests. Validate
the exact published digest, not only a source build. Test key rotation,
configuration validation, redaction, readiness, overload, and graceful shutdown.

Keep non-secret settings in versioned app configuration and secrets injected
at runtime; preserve existing configuration precedence and schema/example parity.
Update API docs, deployment examples, and Docker Hub guidance for actual behavior.
Tag creation, successful CI, and artifact publication are distinct checkpoints.

Exit: reproducible artifact, passing checks, measured profile, and honest
release notes. This gate may repeat for incremental releases after M1–M3;
optional M4–M6 features do not block a core-only release.

### M8 — Temporary application experiment and feedback

Only after explicit user approval, create a separate consumer integration plan
with a fixed time window, pinned Docker Hub image, redacted baseline, and
process-scoped whole-gateway override. Keep LiteLLM healthy and rehearse rollback
before testing. Do not persist temporary switches in secret-manager defaults.

Test real chat, streaming, structured output, embeddings, retrieval/project
memory, usage, reservation settlement/release, quotas, cancellation, and retries.
Collect operator feedback with request IDs and sanitized failures. Roll back
with one selection change and restart; verify original flows afterward.

Exit: reviewed experiment evidence and a new gap list. Repeat standalone work
and experiments as needed; no automatic permanent deployment.

### M9 — Long-term replacement decision

Approve replacement only for a named provider/API/workload/resource envelope
after correctness, accounting, reliability, operation, and rollback gates pass.
Do not label a subset as universal LiteLLM parity. Broaden support through the
same evidence cycle. Permanent application migration requires separate approval.

## Mandatory benchmark protocol

1. Capture source commit, dirty diff identity, Go/tool versions, host CPU/OS/RAM,
   image digest/architecture, native/emulated execution, limits, fixture revision,
   request/response sizes, prompt/model/token limits, concurrency, and cache mode.
2. Run identical deterministic providers sequentially for each gateway to avoid
   cross-contamination. Pin LiteLLM by digest. Use at least five repetitions,
   alternating gateway order, and retain individual runs plus aggregate results.
3. Separate warm steady-state, process startup, and forced cold upstream
   connection tests. Client-side warm-up alone does not establish either cold
   upstream behavior or a fully warm concurrent connection pool.
4. Exercise chat concurrency 1/4/16, streaming 1/4, single/batch embeddings,
   large bounded payloads, saturation, provider errors/timeouts, and cancellation.
   Use at least 1,000 successful measured requests for tail-latency reporting;
   label smaller samples as exploratory. Publish failure rates beside latency.
5. Measure latency p50/p95/p99, throughput, token delivery timing, CPU/time per
   request, RSS/peak, allocations, GC, goroutines, connections, throttling,
   network/block I/O, startup, image size, restarts, OOM, and swap/cgroup data.
   Mark unavailable measurements unavailable rather than substituting zero.
6. Capture CPU, allocation, and blocking profiles separately from timed runs.
   Use benchstat for repeated Go benchmarks. Distinguish client, gateway,
   upstream, and host costs; resource samples must span long enough runs to
   capture peaks. Never compare emulated LiteLLM with native Go as language proof.
7. Define a hypothesis and workload-specific acceptance threshold before editing.
   Investigate repeatable regressions above 5% in latency, CPU, or allocations;
   do not enforce noisy shared-runner timing as a hard release gate. Require
   explicit evidence/tradeoff review for retained regressions.

Keep raw artifacts ignored or outside source. Publish only reviewed, sanitized,
platform-labeled summaries. Live provider tests are a separate confirmation;
provider variability cannot isolate gateway overhead.

## Agent execution and handoff contract

Before each task, read this plan, inspect Git status, identify the earliest unmet
gate, and state the intended change and evidence needed. Do not reimplement a
feature solely because it appears in this plan. Prefer one focused phase or
hypothesis per change. Parallel agents must own disjoint files/workstreams;
benchmark execution on the same machine must be serialized.

Every completed implementation handoff must record:

- Phase, objective, source revision, files changed, and compatibility impact.
- Before/after evidence with exact reproduction commands and artifact locations.
- Checks actually run, failures, unavailable measurements, and remaining risks.
- Dependency/idle overhead changes and rollback instructions.
- Capability-ledger updates supported by tests, and the next unmet gate.

Run formatting, tests, race tests, vet, staticcheck/lint, security/vulnerability
checks, module verification, environment-schema parity, and diff checks for
runtime changes. Run targeted protocol/fuzz/lifecycle tests where applicable.
Benchmark performance changes; validate documentation links and rendering for
documentation changes. Do not repeat unrelated expensive checks without cause.

Keep authorized commits focused and conventional. Do not include unrelated
dirty files. Do not push or release just because a phase suggests a commit.
This master plan authorizes planning direction, not external deployment.

## Immediate next handoff

Start M0 with a source/test audit of stream termination, watchdog reset timing,
trace concurrency, queue deadlines, bounded metric labels, and embedding
normalization preservation. Reconcile the gap matrix and construct an exact
before/after benchmark for `4ab8a74`. The next decision is what evidence-backed
fix to implement, not whether to switch Thingd Cloud.

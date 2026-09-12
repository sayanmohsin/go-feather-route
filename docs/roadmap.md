# Roadmap

Go Feather Route is shaped around a small, dependable routing boundary. The
roadmap describes product direction; the API and configuration documentation
are the source of truth for available behavior.

## Core gateway

- OpenAI-compatible chat routing.
- Model aliases and provider isolation.
- Authentication, limits, retries, timeouts, and streaming.
- Liveness, readiness, status, and lightweight metrics.
- OpenAI-compatible embeddings requests with single and batch inputs.
- Multi-architecture non-root container packaging.

## Longer-term direction

- More OpenAI-compatible provider adapters and private endpoints.
- Multimodal model APIs.
- Per-tenant credentials, quotas, and usage controls.
- Health-aware routing, fallback policy, and graceful degradation.
- Prometheus/OpenTelemetry integration.
- Memory-aware deployment profiles for small hosts.
- Planned Thingd MCP data capabilities through a separate connector boundary.
- Managed-platform and edge deployment patterns.

Thingd remains a planned external capability boundary. Router-only
installations will continue to work without a Thingd instance.

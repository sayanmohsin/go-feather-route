# Go Feather Route

Go Feather Route is a fast, featherweight, OpenAI-compatible model-routing
gateway written in Go.

It keeps provider credentials behind one private service and routes requests
to configured providers without requiring a database, queue, or dashboard.

## Start here

- [Getting started](getting-started.md)
- [Configuration](configuration.md)
- [API reference](api.md)
- [Docker deployment](docker.md)

## Current status

The project is early-stage. The MVP supports OpenAI-compatible chat routing,
OpenAI and DeepSeek provider configuration, authentication, request limits, and
SSE streaming. See the [roadmap](roadmap.md) for maturity and planned work.

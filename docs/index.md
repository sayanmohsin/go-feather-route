---
layout: home
title: Go Feather Route — fast model routing in Go
description: A fast, featherweight, OpenAI-compatible model-routing gateway written in Go.

hero:
  name: Go Feather Route
  text: Route models. Keep the footprint small.
  tagline: A fast, featherweight, OpenAI-compatible gateway for OpenAI, DeepSeek, and private provider boundaries.
  actions:
    - theme: brand
      text: Start routing →
      link: /getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/sayanmohsin/go-feather-route
    - theme: alt
      text: Docker Hub
      link: https://hub.docker.com/r/sayanmohsin/go-feather-route
    - theme: alt
      text: Benchmark LiteLLM
      link: /benchmarks

features:
  - icon: ⚡
    title: Fast startup
    details: A static Go binary with a narrow dependency surface and no database, queue, or dashboard required.
  - icon: ◌
    title: Small footprint
    details: Bounded bodies, concurrency, timeouts, and memory-conscious container packaging for small hosts.
  - icon: ≋
    title: Streaming first
    details: Forward Server-Sent Events promptly, preserve [DONE], and cancel upstream work when clients disconnect.
  - icon: ⇄
    title: OpenAI compatible
    details: Keep existing clients while routing model aliases to OpenAI-compatible providers such as OpenAI and DeepSeek.
  - icon: ▣
    title: Provider isolation
    details: Provider credentials stay server-side behind one private gateway boundary.
  - icon: ◈
    title: Thingd optional
    details: Add the future Thingd MCP connector when you need data-aware capabilities without embedding Thingd in the router.
---

<div class="benchmark-note">
  <strong>Measured speed snapshot:</strong> the local fake-provider comparison is
  reproducible and reports latency, CPU, memory, I/O, process count, and OOM
  state. Read the methodology before interpreting the numbers.
</div>

<div class="metric-grid">
  <div class="metric-card"><div class="label">Go gateway idle RSS</div><div class="value">~4.5 MiB</div><div class="note">Local Docker smoke snapshot</div></div>
  <div class="metric-card"><div class="label">LiteLLM idle RSS</div><div class="value">~1.0 GiB</div><div class="note">Pinned image, local snapshot</div></div>
  <div class="metric-card"><div class="label">Go p50 proxy</div><div class="value">0.64 ms</div><div class="note">16 requests / concurrency 4</div></div>
  <div class="metric-card"><div class="label">Go image</div><div class="value">8.7 MB</div><div class="note">Static arm64 image</div></div>
</div>

<div class="benchmark-note">
  Snapshot context: Apple M1 Max, Docker Desktop, 16 fake-provider requests at
  concurrency 4. LiteLLM ran its pinned amd64 image under local emulation. These
  values are a labeled development snapshot, not a production guarantee.
</div>

## Choose a starting path

<div class="path-grid">
  <a class="path-card" href="/getting-started"><h3>Run locally</h3><p>Start the Go gateway, configure one provider, and send an OpenAI-compatible request.</p></a>
  <a class="path-card" href="/docker"><h3>Run with Docker</h3><p>Use the non-root multi-architecture image with runtime-injected secrets.</p></a>
  <a class="path-card" href="/benchmarks"><h3>Compare with LiteLLM</h3><p>Run the deterministic harness and inspect CPU, memory, latency, I/O, and OOM results.</p></a>
  <a class="path-card" href="/thingd-mcp"><h3>Add Thingd later</h3><p>Keep the router standalone, then connect to Thingd MCP when data-aware routing is needed.</p></a>
</div>

## Start in one command

<div class="terminal-panel">
docker run --rm -p 4000:4000 \
  -e GOFEATHERROUTE_API_KEY=local-gateway-key \
  -e OPENAI_API_KEY=your-key \
  sayanmohsin/go-feather-route:latest
</div>

Go Feather Route is early-stage. Review the [roadmap](/roadmap),
[security model](/security), and [benchmark methodology](/benchmarks) before
using it in production.

## How it fits together

```text
OpenAI-compatible client
          |
          v
   Go Feather Route
      |         |
      v         v
   OpenAI    DeepSeek

Optional future path:
   Go Feather Route ---> Thingd MCP
```

The router owns request limits, authentication, provider selection, streaming,
and operational boundaries. LiteLLM remains a supported comparison and gateway
option; Go Feather Route is designed for deployments where a smaller router
footprint is important.

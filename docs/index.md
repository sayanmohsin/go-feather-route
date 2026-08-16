---
layout: home
title: Go Feather Route — fast model routing in Go
description: A fast, featherweight, OpenAI-compatible model-routing gateway written in Go.

hero:
  name: Go Feather Route
  text: Route models. Keep the footprint small.
  tagline: A focused OpenAI-compatible gateway for model APIs, private provider boundaries, and streaming workloads.
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
      text: Benchmark details
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
    details: Forward Server-Sent Events promptly and cancel upstream work when clients disconnect.
  - icon: ⇄
    title: OpenAI compatible
    details: Keep existing clients while routing model aliases to OpenAI-compatible provider endpoints.
  - icon: ▣
    title: Provider isolation
    details: Provider credentials stay server-side behind one private gateway boundary.
  - icon: ◈
    title: Thingd optional
    details: Add data-aware capabilities through Thingd MCP without embedding Thingd in the router.
---

<div class="benchmark-note">
  <strong>Reference performance:</strong> the benchmark harness reports latency,
  throughput, CPU, memory, I/O, process count, cgroup peaks, and OOM state
  where the host exposes those measurements. See the benchmark guide for the
  measurement environment and interpretation rules.
</div>

<div class="metric-grid">
  <div class="metric-card"><div class="label">Gateway idle RSS</div><div class="value">~4.5 MiB</div><div class="note">Reference container measurement</div></div>
  <div class="metric-card"><div class="label">Proxy p50</div><div class="value">0.64 ms</div><div class="note">16 requests / concurrency 4</div></div>
  <div class="metric-card"><div class="label">Gateway image</div><div class="value">8.7 MB</div><div class="note">Static arm64 image</div></div>
  <div class="metric-card"><div class="label">Streaming</div><div class="value">SSE</div><div class="note">Forwarded without full buffering</div></div>
</div>

## How a request moves

```mermaid
flowchart LR
    client[OpenAI-compatible client] --> auth[Bearer authentication]
    auth --> limits[Body, timeout, and concurrency limits]
    limits --> route[Model alias routing]
    route --> upstream[OpenAI-compatible provider]
    upstream --> response[JSON or streamed SSE response]
    response --> client
```

## Choose a starting path

<div class="path-grid">
  <a class="path-card" href="https://sayanmohsin.github.io/go-feather-route/getting-started"><h3>Start routing</h3><p>Configure one provider and send an OpenAI-compatible request.</p></a>
  <a class="path-card" href="https://sayanmohsin.github.io/go-feather-route/docker"><h3>Deploy with Docker</h3><p>Use the non-root multi-architecture image with runtime-injected secrets.</p></a>
  <a class="path-card" href="https://sayanmohsin.github.io/go-feather-route/benchmarks"><h3>Measure the gateway</h3><p>Compare routing overhead and resource use with the pinned LiteLLM reference image.</p></a>
  <a class="path-card" href="https://sayanmohsin.github.io/go-feather-route/thingd-mcp"><h3>Connect Thingd later</h3><p>Keep the router standalone, then add data-aware capabilities through MCP.</p></a>
</div>

## Long-term direction

The project is designed to remain a small operational boundary as its
capabilities grow: more compatible providers, embeddings and multimodal
requests, per-tenant quotas, usage metrics, health-aware routing, graceful
degradation, memory-aware deployment profiles, and optional Thingd MCP data
capabilities.

```mermaid
flowchart TB
    gateway[Go Feather Route]
    gateway --> chat[Chat and streaming]
    gateway --> models[Model discovery and aliases]
    gateway --> ops[Health, readiness, and metrics]
    gateway -. future .-> embeddings[Embeddings and multimodal APIs]
    gateway -. optional .-> thingd[Thingd MCP data capabilities]
```

## Start in one command

<pre class="terminal-panel"><code>docker run --rm -p 4000:4000 \
  -e GOFEATHERROUTE_API_KEY=gateway-key \
  -e OPENAI_API_KEY=your-key \
  sayanmohsin/go-feather-route:0.1.0</code></pre>

The router owns authentication, limits, provider selection, streaming, and
operational boundaries. Provider credentials are injected at runtime. Read the
  [security model](https://sayanmohsin.github.io/go-feather-route/security),
  [API reference](https://sayanmohsin.github.io/go-feather-route/api), and
  [deployment guide](https://sayanmohsin.github.io/go-feather-route/deployment)
  before placing it behind a public endpoint.

# Gateway benchmark harness

This harness compares Go Feather Route with the pinned LiteLLM image against a
deterministic fake OpenAI-compatible provider. It does not call a paid model.

Requirements: Docker, Docker Compose, Go 1.26, `curl`, and Bash.

Run one gateway:

```bash
./benchmarks/run.sh go
./benchmarks/run.sh litellm
```

Control the workload:

```bash
BENCHMARK_REQUESTS=64 BENCHMARK_CONCURRENCY=8 ./benchmarks/run.sh go
BENCHMARK_REQUESTS=64 BENCHMARK_CONCURRENCY=8 BENCHMARK_STREAMING=true ./benchmarks/run.sh litellm
```

Each run writes sanitized request results, raw Docker resource samples, and
container inspection data under `benchmarks/results/`.

Resource samples include Docker CPU percentage, memory usage/limit, network
I/O, block I/O, process count, host memory where available, and final OOM and
restart state. On Linux hosts the sampler also records cgroup peak/current
memory, CPU usage, CPU throttled time, and throttle count when the host exposes
the cgroup files. Docker Desktop may omit those host-level cgroup fields.

## Optional DeepSeek smoke test

The real-provider test is manual only. Inject `DEEPSEEK_API_KEY` and
`DEEPSEEK_API_BASE` with Doppler, run five identical prompts through each
gateway, and cap each request at 64 output tokens. Never commit the resulting
JSON or credentials.

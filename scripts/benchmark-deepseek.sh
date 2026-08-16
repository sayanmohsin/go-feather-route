#!/usr/bin/env bash
set -Eeuo pipefail

: "${DEEPSEEK_API_KEY:?Inject DEEPSEEK_API_KEY with Doppler before running this test}"
: "${GOFEATHERROUTE_URL:?Set GOFEATHERROUTE_URL to the running Go Feather Route gateway}"
: "${LITELLM_URL:?Set LITELLM_URL to the running LiteLLM gateway}"

output_dir="${BENCHMARK_OUTPUT_DIR:-benchmarks/results}/deepseek-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$output_dir"

go run ./benchmarks/cmd/runner \
  -url "$GOFEATHERROUTE_URL" \
  -gateway go-feather-route-deepseek \
  -api-key="${GOFEATHERROUTE_API_KEY:-}" \
  -model "${DEEPSEEK_MODEL:-deepseek-v4-flash}" \
  -requests 5 \
  -concurrency 1 \
  -output "$output_dir/go-feather-route.json"

go run ./benchmarks/cmd/runner \
  -url "$LITELLM_URL" \
  -gateway litellm-deepseek \
  -api-key="${LITELLM_API_KEY:-}" \
  -model "${DEEPSEEK_MODEL:-deepseek-v4-flash}" \
  -requests 5 \
  -concurrency 1 \
  -output "$output_dir/litellm.json"

echo "DeepSeek benchmark complete: $output_dir"

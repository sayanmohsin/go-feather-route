#!/usr/bin/env bash
set -Eeuo pipefail

target="${1:-go}"
if [[ "$target" != "go" && "$target" != "litellm" ]]; then
  echo "usage: $0 {go|litellm}" >&2
  exit 2
fi

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
stamp="$(date -u +%Y%m%dT%H%M%SZ)-$$"
output_dir="${BENCHMARK_OUTPUT_DIR:-${TMPDIR:-/tmp}/go-feather-route-benchmarks/$stamp}"
mkdir -p "$output_dir"

run_case() {
  local operation="$1"
  local streaming="$2"
  local concurrency="$3"
  local warmup="$4"
  local mode="$5"
  local result_name="${stamp}-${operation}-${mode}-c${concurrency}"

  echo "running $target operation=$operation streaming=$streaming concurrency=$concurrency mode=$mode"
  BENCHMARK_OUTPUT_DIR="$output_dir" \
    BENCHMARK_RESULT_NAME="$result_name" \
    BENCHMARK_OPERATION="$operation" \
    BENCHMARK_STREAMING="$streaming" \
    BENCHMARK_CONCURRENCY="$concurrency" \
    BENCHMARK_WARMUP="$warmup" \
    "$root_dir/benchmarks/run.sh" "$target"
}

for concurrency in 1 4 16; do
  run_case chat false "$concurrency" 0 cold
  run_case chat false "$concurrency" 3 warm
done

for concurrency in 1 4; do
  run_case chat true "$concurrency" 0 cold-streaming
  run_case chat true "$concurrency" 3 warm-streaming
done

run_case embeddings false 1 0 cold-embedding
run_case embeddings false 1 3 warm-embedding

cat <<EOF
matrix complete: $output_dir
metrics are enabled by the gateway for every request; the matrix does not
invent a disabled-instrumentation mode because that would not represent the
production request path.
EOF

#!/usr/bin/env bash
set -Eeuo pipefail

target="${1:-go}"
if [[ "$target" != "go" && "$target" != "litellm" ]]; then
  echo "usage: $0 {go|litellm}" >&2
  exit 2
fi
service="$target"
if [[ "$target" == "go" ]]; then
  service="go-feather-route"
fi

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
benchmark_dir="$root_dir/benchmarks"
project="go-feather-route-benchmark-$target"
compose=(docker compose --project-name "$project" --file "$benchmark_dir/docker-compose.yml" --profile "$target")
result_dir="${BENCHMARK_OUTPUT_DIR:-$benchmark_dir/results}/$(date -u +%Y%m%dT%H%M%SZ)-$target"
mkdir -p "$result_dir"
if [[ -z "${BENCHMARK_HOST_PORT:-}" ]]; then
  if [[ "$target" == "go" ]]; then
    export BENCHMARK_HOST_PORT=4401
  else
    export BENCHMARK_HOST_PORT=4402
  fi
fi

cleanup() {
  "${compose[@]}" down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

"${compose[@]}" up --build --detach fake-provider "$service"
gateway_id="$("${compose[@]}" ps --quiet "$service")"
fake_id="$("${compose[@]}" ps --quiet fake-provider)"
gateway_url="http://127.0.0.1:${BENCHMARK_HOST_PORT}"
gateway_image="$("${compose[@]}" images -q "$service" 2>/dev/null || true)"
gateway_architecture="$(docker image inspect --format '{{.Architecture}}' "$gateway_image" 2>/dev/null || true)"
gateway_digest="$(docker image inspect --format '{{index .RepoDigests 0}}' "$gateway_image" 2>/dev/null || true)"
gateway_image_size="$(docker image inspect --format '{{.Size}}' "$gateway_image" 2>/dev/null || true)"
gateway_memory_limit="$(docker inspect --format '{{.HostConfig.Memory}}' "$gateway_id" 2>/dev/null || true)"
host_architecture="$(uname -m)"
case "$host_architecture" in
  x86_64) normalized_host_architecture="amd64" ;;
  aarch64|arm64) normalized_host_architecture="arm64" ;;
  *) normalized_host_architecture="$host_architecture" ;;
esac
execution_mode="native"
if [[ -n "$gateway_architecture" && "$gateway_architecture" != "$normalized_host_architecture" ]]; then
  execution_mode="emulated-or-translated"
fi
printf '{"gateway":"%s","operation":"%s","streaming":%s,"requests":%s,"concurrency":%s,"host_architecture":"%s","gateway_image_id":"%s","gateway_image_digest":"%s","gateway_image_size_bytes":%s,"gateway_memory_limit_bytes":%s,"gateway_architecture":"%s","native_or_emulated":"%s"}\n' \
  "$target" "${BENCHMARK_OPERATION:-chat}" "${BENCHMARK_STREAMING:-false}" "${BENCHMARK_REQUESTS:-32}" "${BENCHMARK_CONCURRENCY:-1}" \
  "$normalized_host_architecture" "$gateway_image" "$gateway_digest" "${gateway_image_size:-0}" "${gateway_memory_limit:-0}" "$gateway_architecture" "$execution_mode" \
  >"$result_dir/metadata.json"

for attempt in $(seq 1 60); do
  if curl --silent --fail "$gateway_url/health/liveliness" >/dev/null; then
    break
  fi
  if [[ "$attempt" == 60 ]]; then
    echo "gateway did not become healthy" >&2
    exit 1
  fi
  sleep 1
done

resource_file="$result_dir/resources.jsonl"
: >"$resource_file"

sample_resources() {
  while true; do
    record_resources
    if [[ "${runner_pid:-}" != "" ]] && ! kill -0 "$runner_pid" 2>/dev/null; then
      break
    fi
    sleep 0.25
  done
}

record_resources() {
  timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  docker_stats="$(docker stats --no-stream --format '{{json .}}' "$gateway_id" "$fake_id" 2>/dev/null || true)"
  printf '{"timestamp":"%s","docker_stats":[%s]}\n' "$timestamp" "$(printf '%s\n' "$docker_stats" | paste -sd, -)" >>"$resource_file"
  if command -v free >/dev/null 2>&1; then
    free -b | awk 'NR==2 {printf "{\"timestamp\":\"%s\",\"host_memory_total\":%s,\"host_memory_used\":%s,\"host_memory_free\":%s,\"host_memory_available\":%s}\n", strftime("%Y-%m-%dT%H:%M:%SZ"), $2, $3, $4, $7}' >>"$resource_file"
  fi
  record_cgroup "$gateway_id" gateway
  record_cgroup "$fake_id" fake_provider
}

record_cgroup() {
  container_id="$1"
  label="$2"
  for cgroup_dir in \
    "/sys/fs/cgroup/system.slice/docker-${container_id}.scope" \
    "/sys/fs/cgroup/docker/${container_id}" \
    "/sys/fs/cgroup/${container_id}"; do
    if [[ -f "$cgroup_dir/memory.peak" ]]; then
      memory_peak="$(cat "$cgroup_dir/memory.peak")"
      memory_current="$(cat "$cgroup_dir/memory.current")"
      cpu_usage="$(awk '$1 == "usage_usec" {print $2}' "$cgroup_dir/cpu.stat")"
      cpu_throttled="$(awk '$1 == "throttled_usec" {print $2}' "$cgroup_dir/cpu.stat")"
      cpu_throttle_count="$(awk '$1 == "nr_throttled" {print $2}' "$cgroup_dir/cpu.stat")"
      printf '{"timestamp":"%s","cgroup_label":"%s","memory_peak":%s,"memory_current":%s,"cpu_usage_usec":%s,"cpu_throttled_usec":%s,"cpu_throttle_count":%s}\n' \
        "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$label" "${memory_peak:-0}" "${memory_current:-0}" "${cpu_usage:-0}" "${cpu_throttled:-0}" "${cpu_throttle_count:-0}" >>"$resource_file"
      return
    fi
  done
}

record_resources
go run ./benchmarks/cmd/runner \
  -url "$gateway_url" \
  -gateway "$target" \
  -requests "${BENCHMARK_REQUESTS:-32}" \
  -concurrency "${BENCHMARK_CONCURRENCY:-1}" \
  -operation "${BENCHMARK_OPERATION:-chat}" \
  -stream="${BENCHMARK_STREAMING:-false}" \
  -warmup "${BENCHMARK_WARMUP:-0}" \
  -output "$result_dir/requests.json" &
runner_pid=$!
sample_resources &
sampler_pid=$!
wait "$runner_pid"
record_resources
kill "$sampler_pid" 2>/dev/null || true
wait "$sampler_pid" 2>/dev/null || true

docker inspect "$gateway_id" >"$result_dir/gateway-inspect.json"
docker inspect "$fake_id" >"$result_dir/fake-provider-inspect.json"
docker inspect --format '{"restart_count":{{.RestartCount}},"oom_killed":{{.State.OOMKilled}},"status":"{{.State.Status}}"}' "$gateway_id" >"$result_dir/gateway-state.json"
printf 'benchmark complete: %s\n' "$result_dir"

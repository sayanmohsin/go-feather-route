#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "$0")/.." && pwd)"
schema="$root_dir/config/env.schema.yaml"
example="$root_dir/.env.example"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

awk '/^  [A-Z][A-Z0-9_]*:/{name=$1; sub(/:$/, "", name); print name}' "$schema" | sort -u >"$tmp_dir/schema"
awk -F= '/^[A-Z][A-Z0-9_]*=/{print $1}' "$example" | sort -u >"$tmp_dir/example"

if ! diff -u "$tmp_dir/schema" "$tmp_dir/example"; then
  echo ".env.example does not match config/env.schema.yaml" >&2
  exit 1
fi

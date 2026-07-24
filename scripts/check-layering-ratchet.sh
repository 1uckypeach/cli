#!/usr/bin/env bash
# Copyright (c) 2026 Lark Technologies Pte. Ltd.
# SPDX-License-Identifier: MIT

set -euo pipefail

if root="$(git rev-parse --show-toplevel 2>/dev/null)"; then
  cd "$root"
else
  echo "Layering ratchet must run inside a Git worktree." >&2
  exit 1
fi

base_revision="${1:-${QUALITY_GATE_CHANGED_FROM:-}}"
ratchet_file="internal/qualitygate/deptest/layering-edges.txt"
initial_count="${LAYERING_RATCHET_INITIAL_COUNT:-37}"
initial_hash="${LAYERING_RATCHET_INITIAL_HASH:-cf61e4149cb4d539a1430f4a4266b91f972c253cedd300830c94e35dda1f0265}"

if [[ -z "$base_revision" ]]; then
  echo "Layering ratchet requires a base revision." >&2
  exit 1
fi
if ! git cat-file -e "$base_revision^{commit}" 2>/dev/null; then
  echo "Layering ratchet base revision does not exist: $base_revision" >&2
  exit 1
fi
if [[ ! -f "$ratchet_file" ]]; then
  echo "Layering ratchet file is missing: $ratchet_file" >&2
  exit 1
fi

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/layering-ratchet.XXXXXX")"
base_file="$tmp_dir/base.txt"
base_keys="$tmp_dir/base.keys"
current_keys="$tmp_dir/current.keys"
additions="$tmp_dir/additions.keys"

cleanup_tmp() {
  rm -f "$base_file" "$base_keys" "$current_keys" "$additions"
  rmdir "$tmp_dir"
}
trap cleanup_tmp EXIT

extract_keys() {
  local source_file="$1"
  local output_file="$2"
  awk -F '\t' '
    function trim(value) {
      sub(/^[[:space:]]+/, "", value)
      sub(/[[:space:]]+$/, "", value)
      return value
    }
    {
      content = trim($0)
      if (content == "" || substr(content, 1, 1) == "#") {
        next
      }
      if (NF != 5) {
        printf "Malformed layering ratchet row at %s:%d: expected five tab-separated fields.\n", FILENAME, FNR > "/dev/stderr"
        exit 2
      }
      from = trim($1)
      denied = trim($2)
      owner = trim($3)
      reason = trim($4)
      added_at = trim($5)
      if (from == "" || denied == "" || owner == "" || reason == "" || added_at !~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/) {
        printf "Malformed layering ratchet row at %s:%d: fields must be non-empty and added_at must use YYYY-MM-DD.\n", FILENAME, FNR > "/dev/stderr"
        exit 2
      }
      print from "\t" denied
    }
  ' "$source_file" | LC_ALL=C sort >"$output_file"

  if [[ -n "$(uniq -d "$output_file")" ]]; then
    echo "Layering ratchet contains duplicate (from, denied) keys: $source_file" >&2
    uniq -d "$output_file" >&2
    return 1
  fi
}

hash_keys() {
  local source_file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$source_file" | awk '{ print $1 }'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$source_file" | awk '{ print $1 }'
    return
  fi
  echo "Layering ratchet requires sha256sum or shasum." >&2
  return 1
}

extract_keys "$ratchet_file" "$current_keys"

if ! git cat-file -e "$base_revision:$ratchet_file" 2>/dev/null; then
  current_count="$(wc -l <"$current_keys" | tr -d '[:space:]')"
  current_hash="$(hash_keys "$current_keys")"
  if [[ "$current_count" != "$initial_count" || "$current_hash" != "$initial_hash" ]]; then
    echo "::error::Layering ratchet bootstrap differs from the approved $initial_count-edge snapshot." >&2
    exit 1
  fi
  exit 0
fi

git show "$base_revision:$ratchet_file" >"$base_file"
extract_keys "$base_file" "$base_keys"
LC_ALL=C comm -13 "$base_keys" "$current_keys" >"$additions"

if [[ -s "$additions" ]]; then
  echo "::error::Layering ratchet contains new (from, denied) keys. Fix the dependency instead of adding rows." >&2
  while IFS=$'\t' read -r from denied; do
    printf 'from=%s denied=%s\n' "$from" "$denied" >&2
  done <"$additions"
  exit 1
fi

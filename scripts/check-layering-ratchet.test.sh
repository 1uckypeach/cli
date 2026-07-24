#!/usr/bin/env bash
# Copyright (c) 2026 Lark Technologies Pte. Ltd.
# SPDX-License-Identifier: MIT

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/check-layering-ratchet.sh"
ratchet_file="internal/qualitygate/deptest/layering-edges.txt"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/check-layering-ratchet-test.XXXXXX")"

cleanup_tmp() {
  rm -rf "$tmp"
}
trap cleanup_tmp EXIT

row() {
  printf '%s\t%s\towner\treason\t2026-07-24\n' "$1" "$2"
}

git_init() {
  local dir="$1"
  git init -q -b main "$dir"
  git -C "$dir" config user.name test
  git -C "$dir" config user.email test@example.com
  mkdir -p "$dir/$(dirname "$ratchet_file")"
}

write_rows() {
  local dir="$1"
  shift
  {
    printf '# from\tdenied\towner\treason\tadded_at\n'
    while (( $# > 0 )); do
      row "$1" "$2"
      shift 2
    done
  } >"$dir/$ratchet_file"
}

commit_ratchet() {
  local dir="$1"
  git -C "$dir" add "$ratchet_file"
  git -C "$dir" commit -q -m "ratchet"
}

expect_pass() {
  local dir="$1"
  local base="$2"
  if ! (cd "$dir" && bash "$script" "$base"); then
    echo "Expected layering ratchet check to pass in $dir." >&2
    return 1
  fi
}

expect_fail() {
  local dir="$1"
  local base="$2"
  if (cd "$dir" && bash "$script" "$base" >/dev/null 2>&1); then
    echo "Expected layering ratchet check to fail in $dir." >&2
    return 1
  fi
}

hash_file() {
  local source_file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$source_file" | awk '{ print $1 }'
  else
    shasum -a 256 "$source_file" | awk '{ print $1 }'
  fi
}

bootstrap_keys() {
  local source_file="$1"
  awk -F '\t' 'NF == 5 && $1 !~ /^[[:space:]]*#/ { print $1 "\t" $2 }' "$source_file" | LC_ALL=C sort
}

expect_bootstrap_pass() {
  local dir="$1"
  local base="$2"
  local count="$3"
  local hash="$4"
  if ! (
    cd "$dir"
    LAYERING_RATCHET_INITIAL_COUNT="$count" \
      LAYERING_RATCHET_INITIAL_HASH="$hash" \
      bash "$script" "$base"
  ); then
    echo "Expected layering ratchet bootstrap check to pass in $dir." >&2
    return 1
  fi
}

expect_bootstrap_fail() {
  local dir="$1"
  local base="$2"
  local count="$3"
  local hash="$4"
  if (
    cd "$dir"
    LAYERING_RATCHET_INITIAL_COUNT="$count" \
      LAYERING_RATCHET_INITIAL_HASH="$hash" \
      bash "$script" "$base" >/dev/null 2>&1
  ); then
    echo "Expected layering ratchet bootstrap check to fail in $dir." >&2
    return 1
  fi
}

test_unchanged_and_metadata_changes_pass() {
  local dir="$tmp/unchanged"
  git_init "$dir"
  write_rows "$dir" from/a denied/a from/b denied/b
  commit_ratchet "$dir"
  local base
  base="$(git -C "$dir" rev-parse HEAD)"

  expect_pass "$dir" "$base"
  sed -i.bak 's/\towner\treason\t/\tnew-owner\tnew-reason\t/' "$dir/$ratchet_file"
  rm -f "$dir/$ratchet_file.bak"
  expect_pass "$dir" "$base"
}

test_deletion_passes() {
  local dir="$tmp/deletion"
  git_init "$dir"
  write_rows "$dir" from/a denied/a from/b denied/b
  commit_ratchet "$dir"
  local base
  base="$(git -C "$dir" rev-parse HEAD)"

  write_rows "$dir" from/a denied/a
  expect_pass "$dir" "$base"
}

test_addition_fails() {
  local dir="$tmp/addition"
  git_init "$dir"
  write_rows "$dir" from/a denied/a
  commit_ratchet "$dir"
  local base
  base="$(git -C "$dir" rev-parse HEAD)"

  write_rows "$dir" from/a denied/a from/b denied/b
  expect_fail "$dir" "$base"
}

test_equal_count_replacement_fails() {
  local dir="$tmp/replacement"
  git_init "$dir"
  write_rows "$dir" from/a denied/a from/b denied/b
  commit_ratchet "$dir"
  local base
  base="$(git -C "$dir" rev-parse HEAD)"

  write_rows "$dir" from/a denied/a from/c denied/c
  expect_fail "$dir" "$base"
}

test_malformed_and_missing_current_file_fail() {
  local dir="$tmp/malformed"
  git_init "$dir"
  write_rows "$dir" from/a denied/a
  commit_ratchet "$dir"
  local base
  base="$(git -C "$dir" rev-parse HEAD)"

  printf 'from/a\tdenied/a\towner\treason\n' >"$dir/$ratchet_file"
  expect_fail "$dir" "$base"
  rm -f "$dir/$ratchet_file"
  expect_fail "$dir" "$base"
}

test_bootstrap_requires_the_approved_snapshot() {
  local dir="$tmp/bootstrap"
  git_init "$dir"
  printf 'base\n' >"$dir/base.txt"
  git -C "$dir" add base.txt
  git -C "$dir" commit -q -m "base"
  local base
  base="$(git -C "$dir" rev-parse HEAD)"

  local args=()
  local index
  for index in $(seq 1 37); do
    args+=("from/$index" "denied/$index")
  done
  write_rows "$dir" "${args[@]}"
  local keys_file="$dir/initial.keys"
  bootstrap_keys "$dir/$ratchet_file" >"$keys_file"
  local initial_hash
  initial_hash="$(hash_file "$keys_file")"
  expect_bootstrap_pass "$dir" "$base" 37 "$initial_hash"

  args[72]="from/replacement"
  write_rows "$dir" "${args[@]}"
  expect_bootstrap_fail "$dir" "$base" 37 "$initial_hash"

  args[72]="from/37"
  args+=("from/38" "denied/38")
  write_rows "$dir" "${args[@]}"
  expect_bootstrap_fail "$dir" "$base" 37 "$initial_hash"
}

test_invalid_base_fails() {
  local dir="$tmp/invalid-base"
  git_init "$dir"
  write_rows "$dir" from/a denied/a
  commit_ratchet "$dir"
  expect_fail "$dir" missing-revision
}

test_unchanged_and_metadata_changes_pass
test_deletion_passes
test_addition_fails
test_equal_count_replacement_fails
test_malformed_and_missing_current_file_fail
test_bootstrap_requires_the_approved_snapshot
test_invalid_base_fails

#!/usr/bin/env bash
# Copyright (c) 2026 Lark Technologies Pte. Ltd.
# SPDX-License-Identifier: MIT

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

mkdir -p "$TMP_DIR/.githooks"
cp "$ROOT_DIR/.githooks/pre-commit" "$TMP_DIR/.githooks/pre-commit"
chmod +x "$TMP_DIR/.githooks/pre-commit"

git -C "$TMP_DIR" init --quiet
git -C "$TMP_DIR" config core.hooksPath .githooks
git -C "$TMP_DIR" config user.name "Third-Party Notices Test"
git -C "$TMP_DIR" config user.email "third-party-notices@example.invalid"

printf 'module example.com/test\n' > "$TMP_DIR/go.mod"
git -C "$TMP_DIR" add go.mod
if git -C "$TMP_DIR" commit --quiet -m "test: missing notices"; then
  echo "pre-commit should block staged dependency metadata without notices" >&2
  exit 1
fi

printf '# Third-Party Notices\n' > "$TMP_DIR/THIRD_PARTY_NOTICES.md"
git -C "$TMP_DIR" add THIRD_PARTY_NOTICES.md
git -C "$TMP_DIR" commit --quiet -m "test: include notices"

if rg -q '^[[:space:]]*(make|python3?|git[[:space:]]+add)\b' "$ROOT_DIR/.githooks/pre-commit"; then
  echo "pre-commit must only inspect the Git index; it must not execute or stage worktree code" >&2
  exit 1
fi

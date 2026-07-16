#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

PREFLIGHT_OUTPUT=$(node "${SCRIPT_DIR}/release-preflight.js")
VERSION=$(node -e 'const result = JSON.parse(process.argv[1]); process.stdout.write(result.data.packageVersion)' "${PREFLIGHT_OUTPUT}")
TAG="v${VERSION}"

node "${SCRIPT_DIR}/release-preflight.js" --tag "${TAG}"

echo "Version: ${VERSION}"
echo "Tag: ${TAG}"

CURRENT_BRANCH=$(git branch --show-current)
if [ "${CURRENT_BRANCH}" != "main" ]; then
  echo "Error: releases must be tagged from main; current branch is '${CURRENT_BRANCH}'." >&2
  exit 1
fi

if ! git diff --quiet -- package.json package-lock.json; then
  echo "Error: package.json or package-lock.json has unstaged changes. Please commit them before tagging." >&2
  exit 1
fi

if ! git diff --cached --quiet -- package.json package-lock.json; then
  echo "Error: package.json or package-lock.json has staged changes. Please commit them before tagging." >&2
  exit 1
fi

git fetch origin main

HEAD_SHA=$(git rev-parse HEAD)
FETCHED_MAIN_SHA=$(git rev-parse "FETCH_HEAD^{commit}")
if [ "${HEAD_SHA}" != "${FETCHED_MAIN_SHA}" ]; then
  echo "Error: HEAD must exactly match origin/main before tagging." >&2
  exit 1
fi

LOCAL_TAG_EXISTS=0
if LOCAL_TAG_SHA=$(git rev-parse -q --verify "refs/tags/${TAG}^{commit}" 2>/dev/null); then
  LOCAL_TAG_EXISTS=1
  if [ "${LOCAL_TAG_SHA}" != "${HEAD_SHA}" ]; then
    echo "Error: local tag ${TAG} does not point to HEAD." >&2
    exit 1
  fi
else
  STATUS=$?
  if [ "${STATUS}" -ne 1 ]; then
    echo "Error: could not resolve local tag ${TAG}." >&2
    exit "${STATUS}"
  fi
fi

if REMOTE_TAG_OUTPUT=$(git ls-remote --tags origin "refs/tags/${TAG}" "refs/tags/${TAG}^{}"); then
  :
else
  STATUS=$?
  echo "Error: could not query tag ${TAG} on origin." >&2
  exit "${STATUS}"
fi

REMOTE_TAG_SHA=$(printf '%s\n' "${REMOTE_TAG_OUTPUT}" | awk '
  $2 ~ /\^\{\}$/ { peeled = $1; next }
  { direct = $1 }
  END {
    if (peeled != "") print peeled
    else if (direct != "") print direct
  }
')

if [ -n "${REMOTE_TAG_SHA}" ]; then
  if [ "${REMOTE_TAG_SHA}" != "${HEAD_SHA}" ]; then
    echo "Error: remote tag ${TAG} does not point to HEAD." >&2
    exit 1
  fi
  echo "Tag ${TAG} already exists on remote and points to HEAD, skipping."
  exit 0
fi

if [ "${LOCAL_TAG_EXISTS}" -eq 0 ]; then
  git tag "${TAG}" "${HEAD_SHA}"
fi

git push origin "refs/tags/${TAG}"

echo "Successfully pushed tag ${TAG}"

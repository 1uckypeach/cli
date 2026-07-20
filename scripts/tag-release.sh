#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

VERSION=$(node -p "require('./package.json').version")
TAG="v${VERSION}"
REHEARSAL_BRANCH="test/npm-staged-publish-rehearsal"
PUSH_TAG=false

if [ "$#" -eq 1 ] && [ "$1" = "--push" ]; then
  PUSH_TAG=true
elif [ "$#" -ne 0 ]; then
  echo "Usage: $0 [--push]" >&2
  exit 1
fi

node "${SCRIPT_DIR}/release-preflight.js" --tag "${TAG}"

if [[ ! "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+-beta\.[0-9]+$ ]]; then
  echo "Error: rehearsal releases require an X.Y.Z-beta.N version." >&2
  exit 1
fi

echo "Version: ${VERSION}"
echo "Tag: ${TAG}"

CURRENT_BRANCH=$(git branch --show-current)
if [ "${CURRENT_BRANCH}" != "${REHEARSAL_BRANCH}" ]; then
  echo "Error: rehearsal tags must be created from ${REHEARSAL_BRANCH}; current branch is '${CURRENT_BRANCH}'." >&2
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "Error: the working tree must be clean before tagging." >&2
  exit 1
fi

git fetch origin "${REHEARSAL_BRANCH}"

HEAD_SHA=$(git rev-parse HEAD)
FETCHED_REHEARSAL_SHA=$(git rev-parse "FETCH_HEAD^{commit}")
if [ "${HEAD_SHA}" != "${FETCHED_REHEARSAL_SHA}" ]; then
  echo "Error: HEAD must exactly match origin/${REHEARSAL_BRANCH} before tagging." >&2
  exit 1
fi

WORKFLOW=$(git show "${HEAD_SHA}:.github/workflows/release.yml")
if ! grep -Fq 'args: release --clean --skip=publish' <<<"${WORKFLOW}" ||
   ! grep -Eq 'npm stage publish .*--tag beta' <<<"${WORKFLOW}" ||
   grep -Eq '(^|[[:space:]])npm publish([[:space:]]|$)' <<<"${WORKFLOW}" ||
   grep -Eq 'gh[[:space:]]+release([[:space:]]|$)' <<<"${WORKFLOW}" ||
   grep -Eq 'contents:[[:space:]]*write' <<<"${WORKFLOW}" ||
   grep -Fq 'GITHUB_TOKEN:' <<<"${WORKFLOW}"; then
  echo "Error: the tagged workflow must be stage-only, read-only for repository contents, and must not create a GitHub Release or publish npm live." >&2
  exit 1
fi

set +e
NPM_VIEW_OUTPUT=$(npm view "@larksuite/cli@${VERSION}" version --registry=https://registry.npmjs.org/ 2>&1)
NPM_VIEW_STATUS=$?
set -e
if [ "${NPM_VIEW_STATUS}" -eq 0 ]; then
  echo "Error: @larksuite/cli@${VERSION} already exists on npm." >&2
  exit 1
fi
if ! grep -Eq 'E404|404 Not Found' <<<"${NPM_VIEW_OUTPUT}"; then
  echo "Error: npm version lookup failed; refusing to assume the version is unused." >&2
  echo "${NPM_VIEW_OUTPUT}" >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  echo "Error: local tag ${TAG} already exists." >&2
  exit 1
fi

REMOTE_TAG=$(git ls-remote --tags origin "refs/tags/${TAG}")
if [ -n "${REMOTE_TAG}" ]; then
  echo "Error: remote tag ${TAG} already exists." >&2
  exit 1
fi

if [ "${PUSH_TAG}" != true ]; then
  echo "Checks passed. No tag was created or pushed."
  echo "Run '$0 --push' only after reviewing the commit and workflow."
  exit 0
fi

echo "Branch: ${CURRENT_BRANCH}"
echo "Commit: ${HEAD_SHA}"
printf 'Type %s to create and push this tag: ' "${TAG}"
read -r CONFIRM_TAG
if [ "${CONFIRM_TAG}" != "${TAG}" ]; then
  echo "Error: confirmation did not exactly match ${TAG}." >&2
  exit 1
fi

git tag "${TAG}" "${HEAD_SHA}"
git push origin "refs/tags/${TAG}:refs/tags/${TAG}"

echo "Successfully pushed tag ${TAG}"

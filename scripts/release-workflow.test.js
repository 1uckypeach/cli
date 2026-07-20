// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { describe, it } = require("node:test");

const repoRoot = path.resolve(__dirname, "..");
const releaseWorkflow = fs.readFileSync(
  path.join(repoRoot, ".github/workflows/release.yml"),
  "utf8",
);
const previewWorkflow = fs.readFileSync(
  path.join(repoRoot, ".github/workflows/pkg-pr-new.yml"),
  "utf8",
);
function topLevelBlock(source, name) {
  const match = source.match(
    new RegExp(
      `^${name}:\\n([\\s\\S]*?)(?=^[A-Za-z][A-Za-z0-9_-]*:|(?![\\s\\S]))`,
      "m",
    ),
  );
  assert.ok(match, `missing top-level ${name} block`);
  return match[0];
}

function jobBlock(source, name) {
  const jobs = topLevelBlock(source, "jobs");
  const match = jobs.match(
    new RegExp(
      `^  ${name}:\\n([\\s\\S]*?)(?=^  [A-Za-z][A-Za-z0-9_-]*:|(?![\\s\\S]))`,
      "m",
    ),
  );
  assert.ok(match, `missing ${name} job`);
  return match[0];
}

function assertInOrder(source, snippets) {
  let previous = -1;
  for (const snippet of snippets) {
    const index = source.indexOf(snippet);
    assert.ok(index >= 0, `missing workflow fragment: ${snippet}`);
    assert.ok(index > previous, `workflow fragment is out of order: ${snippet}`);
    previous = index;
  }
}

function permissionLines(job) {
  const match = job.match(/^    permissions:\n((?:      .+\n)+)/m);
  assert.ok(match, "missing job permissions");
  return match[1].trim().split("\n").map((line) => line.trim()).sort();
}

describe("release workflow contract", () => {
  it("has only the version-tag production trigger", () => {
    const trigger = topLevelBlock(releaseWorkflow, "on");

    assert.match(trigger, /^on:\n  push:\n    tags:\n      - 'v\*'\n+$/);
    for (const forbidden of [
      "workflow_dispatch:",
      "workflow_run:",
      "pull_request:",
      "pull_request_target:",
    ]) {
      assert.equal(releaseWorkflow.includes(forbidden), false, forbidden);
    }
  });

  it("runs preflight before every release side effect", () => {
    const preflight = jobBlock(releaseWorkflow, "preflight");

    assert.deepEqual(permissionLines(preflight), ["contents: read"]);
    assertInOrder(preflight, [
      "actions/checkout@",
      "fetch-depth: 0",
      "actions/setup-node@",
      "node-version: '22.14.0'",
      "node scripts/release-preflight.js --tag \"$TAG\"",
      "git rev-parse --verify 'HEAD^{commit}'",
      "git rev-parse --verify \"refs/tags/${TAG}^{commit}\"",
      'if [[ "$TAG" == *-beta.* ]]',
      'git fetch origin "$REHEARSAL_BRANCH"',
      "git rev-parse --verify 'FETCH_HEAD^{commit}'",
      "git fetch origin main",
      'git merge-base --is-ancestor "$HEAD_SHA" "$MAIN_SHA"',
    ]);
    assert.equal(preflight.includes("gh release"), false);
    assert.equal(preflight.includes("npm publish"), false);
  });

  it("builds artifacts and stages npm after one protected approval", () => {
    const publish = jobBlock(releaseWorkflow, "stage-release");

    assert.match(publish, /needs: preflight/);
    assert.deepEqual(permissionLines(publish), ["contents: read", "id-token: write"]);
    assert.match(publish, /^    environment: npm-production$/m);
    assert.match(publish, /actions\/setup-go@[0-9a-f]{40}/);
    assert.match(publish, /actions\/setup-python@[0-9a-f]{40}/);
    assert.match(
      publish,
      /actions\/setup-node@249970729cb0ef3589644e2896645e5dc5ba9c38 # v6/,
    );
    assert.match(publish, /node-version: '22.14.0'/);
    assert.match(publish, /registry-url: 'https:\/\/registry\.npmjs\.org'/);
    assert.match(publish, /package-manager-cache: false/);
    assert.match(publish, /npm install --global npm@11\.16\.0/);
    assert.match(publish, /goreleaser\/goreleaser-action@[0-9a-f]{40}/);
    assert.match(publish, /args: release --clean --skip=publish/);
    assertInOrder(publish, [
      "actions/setup-go@",
      "actions/setup-python@",
      "actions/setup-node@",
      "npm install --global npm@11.16.0",
      "goreleaser/goreleaser-action@",
      "sha256sum --check checksums.txt",
      "cp dist/checksums.txt checksums.txt",
      "npm pack --ignore-scripts --json",
      "actions/upload-artifact@",
      'npm stage publish "${{ steps.pack.outputs.filename }}" --access public --tag beta',
    ]);
    for (const forbidden of [
      "actions/download-artifact",
      "verify-release-assets",
      "gh release download",
      "npm view",
      "LOCAL_INTEGRITY",
      "REMOTE_INTEGRITY",
      "secrets.NPM_TOKEN",
      "NODE_AUTH_TOKEN",
      "GITHUB_TOKEN:",
      "gh release create",
    ]) {
      assert.equal(releaseWorkflow.includes(forbidden), false, forbidden);
    }
    assert.equal(/(^|\s)npm publish(?:\s|$)/m.test(publish), false);
  });
});

describe("preview isolation", () => {
  it("keeps preview publishing away from production credentials and registry", () => {
    assert.equal(previewWorkflow.includes("id-token: write"), false);
    assert.equal(previewWorkflow.includes("npm publish"), false);
    assert.equal(previewWorkflow.includes("registry.npmjs.org"), false);
    assert.equal(previewWorkflow.includes("secrets.NPM_TOKEN"), false);
    assert.equal(previewWorkflow.includes("NODE_AUTH_TOKEN"), false);
  });
});

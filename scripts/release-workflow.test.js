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
      "git fetch origin main",
      "git rev-parse --verify 'HEAD^{commit}'",
      "git rev-parse --verify 'FETCH_HEAD^{commit}'",
      "git rev-parse --verify \"refs/tags/${TAG}^{commit}\"",
      'git merge-base --is-ancestor "$HEAD_SHA" "$MAIN_SHA"',
    ]);
    assert.equal(preflight.includes("gh release"), false);
    assert.equal(preflight.includes("gh api"), false);
  });

  it("builds immutable run-scoped assets without publishing a release", () => {
    const goreleaser = jobBlock(releaseWorkflow, "goreleaser");

    assert.match(goreleaser, /needs: preflight/);
    assert.deepEqual(permissionLines(goreleaser), ["contents: read"]);
    assert.match(goreleaser, /goreleaser\/goreleaser-action@[0-9a-f]{40}/);
    assert.match(goreleaser, /args: release --clean --skip=publish/);
    assert.match(goreleaser, /cp dist\/\*\.tar\.gz dist\/\*\.zip dist\/checksums\.txt release-assets\//);
    assert.match(goreleaser, /actions\/upload-artifact@[0-9a-f]{40}/);
    assert.match(goreleaser, /name: release-assets-\$\{\{ github\.run_id \}\}/);
    assert.match(goreleaser, /overwrite: true/);
    assert.equal(goreleaser.includes("github.run_attempt"), false);
    assert.equal(goreleaser.includes("GITHUB_TOKEN"), false);
  });

  it("verifies all assets from the run-scoped build artifact", () => {
    const verify = jobBlock(releaseWorkflow, "verify-release-assets");

    assert.match(verify, /needs: \[preflight, goreleaser\]/);
    assert.deepEqual(permissionLines(verify), ["contents: read"]);
    assert.match(verify, /actions\/checkout@[0-9a-f]{40}/);
    assert.match(verify, /actions\/download-artifact@[0-9a-f]{40}/);
    assert.match(verify, /name: release-assets-\$\{\{ github\.run_id \}\}/);
    assert.equal(verify.includes("github.run_attempt"), false);
    assert.match(verify, /node scripts\/verify-release-assets\.js release-assets/);
    assertInOrder(verify, [
      "actions/download-artifact@",
      "node scripts/verify-release-assets.js release-assets",
    ]);
    assert.equal(verify.includes("gh release"), false);
  });

  it("publishes GitHub and npm behind one protected production job", () => {
    const publish = jobBlock(releaseWorkflow, "publish-release");

    assert.match(publish, /needs: verify-release-assets/);
    assert.deepEqual(permissionLines(publish), ["contents: write", "id-token: write"]);
    assert.match(publish, /^    environment: npm-production$/m);
    assert.match(
      publish,
      /actions\/setup-node@249970729cb0ef3589644e2896645e5dc5ba9c38 # v6/,
    );
    assert.match(publish, /node-version: '22.14.0'/);
    assert.match(publish, /registry-url: 'https:\/\/registry\.npmjs\.org'/);
    assert.match(publish, /package-manager-cache: false/);
    assert.match(publish, /npm install --global npm@11\.16\.0/);
    assert.match(publish, /actions\/download-artifact@[0-9a-f]{40}/);
    assert.match(publish, /name: release-assets-\$\{\{ github\.run_id \}\}/);
    assert.equal(publish.includes("github.run_attempt"), false);
    assert.match(publish, /path: release-assets/);
    assertInOrder(publish, [
      "actions/download-artifact@",
      "node scripts/release-preflight.js --tag \"$TAG\"",
      "node scripts/verify-release-assets.js release-assets",
      "cp release-assets/checksums.txt checksums.txt",
      "gh release create \"$TAG\" release-assets/* --verify-tag",
      "gh release download \"$TAG\" --dir published-assets",
      "node scripts/verify-release-assets.js published-assets",
      "diff -qr release-assets published-assets",
      "npm pack --json",
      "npm view \"${PACKAGE_NAME}@${VERSION}\" dist.integrity",
      "npm publish \"$TARBALL\" --access public",
    ]);
    assert.match(publish, /LOCAL_INTEGRITY/);
    assert.match(publish, /REMOTE_INTEGRITY/);
    assert.match(publish, /LOCAL_INTEGRITY.*REMOTE_INTEGRITY|REMOTE_INTEGRITY.*LOCAL_INTEGRITY/s);
    assert.equal(/npm publish\s+(?:--access public\s*)?$/.test(publish), false);
  });

  it("verifies both new and existing GitHub Releases before npm publishing", () => {
    const publish = jobBlock(releaseWorkflow, "publish-release");

    assert.match(publish, /actions\/download-artifact@[0-9a-f]{40}/);
    assert.match(publish, /node scripts\/verify-release-assets\.js release-assets/);
    assert.match(publish, /gh release create \"\$TAG\" release-assets\/\* --verify-tag/);
    assert.match(publish, /typeof r\.draft.*typeof r\.prerelease/);
    assert.match(publish, /existing Draft or prerelease GitHub Release/);
    assert.match(publish, /gh release download \"\$TAG\" --dir published-assets/);
    assert.match(publish, /node scripts\/verify-release-assets\.js published-assets/);
    assert.match(publish, /diff -qr release-assets published-assets/);
    assert.equal(publish.includes("gh release edit"), false);
    assert.equal(/gh release create[^\n]*\n\s*exit 0/.test(publish), false);
  });

  it("keeps pre-publication jobs isolated from existing releases", () => {
    for (const jobName of [
      "preflight",
      "goreleaser",
      "verify-release-assets",
    ]) {
      const job = jobBlock(releaseWorkflow, jobName);
      assert.equal(job.includes("gh release download"), false, jobName);
      assert.equal(job.includes("releases/tags/"), false, jobName);
    }

    const publishGitHub = jobBlock(releaseWorkflow, "publish-release");
    assert.match(publishGitHub, /existing Draft or prerelease GitHub Release.*not trusted/);
    assert.match(publishGitHub, /diff -qr release-assets published-assets/);
  });

  it("keeps job order and removes legacy release credentials", () => {
    assertInOrder(releaseWorkflow, [
      "  preflight:",
      "  goreleaser:",
      "  verify-release-assets:",
      "  publish-release:",
    ]);
    assert.equal(
      (releaseWorkflow.match(/^    environment: npm-production$/gm) || []).length,
      1,
    );
    for (const forbidden of ["secrets.NPM_TOKEN", "NODE_AUTH_TOKEN", "npm stage"] ) {
      assert.equal(releaseWorkflow.includes(forbidden), false, forbidden);
    }
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

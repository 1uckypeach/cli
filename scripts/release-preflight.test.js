// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

const assert = require("node:assert/strict");
const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { describe, it } = require("node:test");

const {
  validateReleasePreflight,
} = require("./release-preflight");

const repoRoot = path.resolve(__dirname, "..");

function createReleaseFixture(t, env = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "tag-release-test-"));
  const scriptsDir = path.join(root, "scripts");
  const binDir = path.join(root, "bin");
  const stateDir = path.join(root, "state");
  const logPath = path.join(root, "git-calls.jsonl");
  const npmLogPath = path.join(root, "npm-calls.jsonl");
  fs.mkdirSync(scriptsDir);
  fs.mkdirSync(binDir);
  fs.mkdirSync(stateDir);
  fs.copyFileSync(
    path.join(repoRoot, "scripts/release-preflight.js"),
    path.join(scriptsDir, "release-preflight.js"),
  );
  fs.copyFileSync(
    path.join(repoRoot, "scripts/tag-release.sh"),
    path.join(scriptsDir, "tag-release.sh"),
  );
  fs.writeFileSync(path.join(root, "package.json"), '{"version":"1.2.3-beta.0"}\n');
  fs.writeFileSync(
    path.join(root, "package-lock.json"),
    '{"version":"1.2.3-beta.0","packages":{"":{"version":"1.2.3-beta.0"}}}\n',
  );

  const fakeGitPath = path.join(binDir, "git");
  fs.writeFileSync(fakeGitPath, String.raw`#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");

const args = process.argv.slice(2);
const stateDir = process.env.FAKE_GIT_STATE_DIR;
const localTagPath = path.join(stateDir, "local-tag");
if (process.cwd() !== process.env.FAKE_EXPECTED_GIT_CWD) {
  process.stderr.write("git invoked outside repository root: " + process.cwd() + "\n");
  process.exit(96);
}
fs.appendFileSync(process.env.FAKE_GIT_LOG, JSON.stringify(args) + "\n");

function print(value) {
  process.stdout.write(value + "\n");
}

switch (args[0]) {
  case "branch":
    print(process.env.FAKE_BRANCH || "test/npm-staged-publish-rehearsal");
    break;
  case "status":
    if (process.env.FAKE_STATUS_OUTPUT) print(process.env.FAKE_STATUS_OUTPUT);
    break;
  case "fetch":
    break;
  case "rev-parse": {
    const ref = args[args.length - 1];
    if (ref === "HEAD") {
      print(process.env.FAKE_HEAD_SHA);
      break;
    }
    if (ref === "FETCH_HEAD^{commit}") {
      print(process.env.FAKE_REHEARSAL_SHA);
      break;
    }
    if (ref.startsWith("refs/tags/")) {
      if (fs.existsSync(localTagPath)) {
        print(fs.readFileSync(localTagPath, "utf8").trim());
        break;
      }
      process.exit(1);
    }
    process.stderr.write("unexpected rev-parse ref: " + ref + "\n");
    process.exit(97);
    break;
  }
  case "ls-remote": {
    const tagRef = args.find((arg) => arg.startsWith("refs/tags/") && !arg.endsWith("^{}"));
    const kind = process.env.FAKE_REMOTE_TAG_KIND || "absent";
    if (kind === "lightweight" || kind === "annotated") {
      print(process.env.FAKE_REMOTE_TAG_SHA + "\t" + tagRef);
    }
    break;
  }
  case "show":
    print(process.env.FAKE_WORKFLOW);
    break;
  case "tag":
    fs.writeFileSync(localTagPath, args[2] || process.env.FAKE_HEAD_SHA);
    break;
  case "push": {
    const failedMarker = path.join(stateDir, "push-failed");
    if (process.env.FAKE_PUSH_FAIL_ONCE && !fs.existsSync(failedMarker)) {
      fs.writeFileSync(failedMarker, "1");
      process.exit(Number(process.env.FAKE_PUSH_FAIL_ONCE));
    }
    break;
  }
  default:
    process.stderr.write("unexpected git command: " + args.join(" ") + "\n");
    process.exit(97);
}
`);
  fs.chmodSync(fakeGitPath, 0o755);

  const fakeNpmPath = path.join(binDir, "npm");
  fs.writeFileSync(fakeNpmPath, String.raw`#!/usr/bin/env node
const fs = require("node:fs");
const args = process.argv.slice(2);
fs.appendFileSync(process.env.FAKE_NPM_LOG, JSON.stringify(args) + "\n");
if (args[0] !== "view") {
  process.stderr.write("unexpected npm command: " + args.join(" ") + "\n");
  process.exit(97);
}
const output = process.env.FAKE_NPM_VIEW_OUTPUT || "npm error code E404\nnpm error 404 Not Found";
(Number(process.env.FAKE_NPM_VIEW_STATUS || "1") === 0 ? process.stdout : process.stderr).write(output + "\n");
process.exit(Number(process.env.FAKE_NPM_VIEW_STATUS || "1"));
`);
  fs.chmodSync(fakeNpmPath, 0o755);
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));

  return {
    root,
    stateDir,
    logPath,
    env: {
      ...process.env,
      PATH: `${binDir}${path.delimiter}${process.env.PATH}`,
      LANG: "C",
      LC_ALL: "C",
      FAKE_GIT_LOG: logPath,
      FAKE_NPM_LOG: npmLogPath,
      FAKE_GIT_STATE_DIR: stateDir,
      FAKE_EXPECTED_GIT_CWD: fs.realpathSync(root),
      FAKE_HEAD_SHA: "aaaaaaaa",
      FAKE_REHEARSAL_SHA: "aaaaaaaa",
      FAKE_WORKFLOW: "args: release --clean --skip=publish\nrun: npm stage publish package.tgz --access public --tag beta",
      ...env,
    },
  };
}

function runTagRelease(fixture, options = {}) {
  const { cwd = fixture.root, args = [], input = "" } = options;
  return spawnSync("bash", [path.join(fixture.root, "scripts/tag-release.sh"), ...args], {
    cwd,
    env: fixture.env,
    encoding: "utf8",
    input,
  });
}

function readGitCalls(fixture) {
  if (!fs.existsSync(fixture.logPath)) {
    return [];
  }
  return fs.readFileSync(fixture.logPath, "utf8")
    .trim()
    .split("\n")
    .filter(Boolean)
    .map((line) => JSON.parse(line));
}

function assertNoTagOperations(calls) {
  const tagOperations = calls.filter((args) =>
    args[0] === "ls-remote" ||
    args[0] === "tag" ||
    args[0] === "push" ||
    (args[0] === "rev-parse" && args.some((arg) => arg.startsWith("refs/tags/"))),
  );
  assert.deepEqual(tagOperations, []);
}

function assertNoTagWrites(calls) {
  assert.equal(calls.some((args) => args[0] === "tag" || args[0] === "push"), false);
}

function validInputs(version = "1.2.3") {
  return {
    packageJson: { version },
    packageLockJson: {
      version,
      packages: { "": { version } },
    },
  };
}

function assertStructuredError(result) {
  assert.equal(result.ok, false);
  assert.equal(result.error.type, "release_preflight");
  assert.equal(typeof result.error.message, "string");
  assert.ok(result.error.message.length > 0);
  assert.equal(typeof result.error.observed, "object");
  assert.equal(typeof result.error.hint, "string");
  assert.ok(result.error.hint.length > 0);
}

function assertInOrder(source, snippets) {
  let previous = -1;
  for (const snippet of snippets) {
    const index = source.indexOf(snippet);
    assert.ok(index >= 0, `missing fragment: ${snippet}`);
    assert.ok(index > previous, `fragment is out of order: ${snippet}`);
    previous = index;
  }
}

describe("validateReleasePreflight", () => {
  it("accepts matching stable and beta rehearsal versions", () => {
    for (const version of ["1.2.3", "1.2.3-beta.0"]) {
      const { packageJson, packageLockJson } = validInputs(version);

      assert.deepEqual(validateReleasePreflight(packageJson, packageLockJson), {
        ok: true,
        data: {
          packageVersion: version,
          lockVersion: version,
          lockRootVersion: version,
          tagVersion: null,
        },
      });
      assert.deepEqual(
        validateReleasePreflight(packageJson, packageLockJson, `v${version}`),
        {
          ok: true,
          data: {
            packageVersion: version,
            lockVersion: version,
            lockRootVersion: version,
            tagVersion: version,
          },
        },
      );
    }
  });

  it("rejects prerelease forms other than beta rehearsal versions", () => {
    const { packageJson, packageLockJson } = validInputs("1.2.3-rc.1");

    const result = validateReleasePreflight(packageJson, packageLockJson);

    assertStructuredError(result);
    assert.equal(
      result.error.message,
      "package.json.version must use X.Y.Z or the rehearsal form X.Y.Z-beta.N",
    );
    assert.equal(
      result.error.hint,
      "Use the same version in all package fields; only stable releases and the temporary beta rehearsal form are allowed.",
    );
  });

  it("rejects build metadata package versions with the stable release contract", () => {
    const { packageJson, packageLockJson } = validInputs("1.2.3+build.7");

    const result = validateReleasePreflight(packageJson, packageLockJson);

    assertStructuredError(result);
    assert.equal(
      result.error.message,
      "package.json.version must use X.Y.Z or the rehearsal form X.Y.Z-beta.N",
    );
    assert.equal(
      result.error.hint,
      "Use the same version in all package fields; only stable releases and the temporary beta rehearsal form are allowed.",
    );
  });

  it("rejects invalid and missing package or lock SemVer values", () => {
    const invalid = validInputs();
    invalid.packageJson.version = "01.2.3";
    const missing = validInputs();
    delete missing.packageLockJson.packages[""].version;

    for (const result of [
      validateReleasePreflight(invalid.packageJson, invalid.packageLockJson),
      validateReleasePreflight(missing.packageJson, missing.packageLockJson),
    ]) {
      assertStructuredError(result);
    }
  });

  it("rejects a top-level package-lock version mismatch", () => {
    const { packageJson, packageLockJson } = validInputs();
    packageLockJson.version = "1.2.4";

    const result = validateReleasePreflight(packageJson, packageLockJson);

    assertStructuredError(result);
    assert.deepEqual(result.error.observed, {
      packageVersion: "1.2.3",
      lockVersion: "1.2.4",
      lockRootVersion: "1.2.3",
      tagVersion: null,
    });
  });

  it("rejects a package-lock root package version mismatch", () => {
    const { packageJson, packageLockJson } = validInputs();
    packageLockJson.packages[""].version = "1.2.4";

    const result = validateReleasePreflight(packageJson, packageLockJson);

    assertStructuredError(result);
    assert.deepEqual(result.error.observed, {
      packageVersion: "1.2.3",
      lockVersion: "1.2.3",
      lockRootVersion: "1.2.4",
      tagVersion: null,
    });
  });

  it("rejects invalid and mismatched tags", () => {
    const { packageJson, packageLockJson } = validInputs();

    for (const tag of ["1.2.3", "v01.2.3", "v1.2.4"]) {
      const result = validateReleasePreflight(packageJson, packageLockJson, tag);
      assertStructuredError(result);
      assert.equal(result.error.observed.tag, tag);
    }
  });
});

describe("release configuration", () => {
  it("writes success to stdout and structured failures to stderr", () => {
    const scriptPath = path.join(repoRoot, "scripts/release-preflight.js");
    const packageVersion = require(path.join(repoRoot, "package.json")).version;
    const success = spawnSync(process.execPath, [scriptPath, "--tag", `v${packageVersion}`], {
      cwd: repoRoot,
      encoding: "utf8",
    });
    const failure = spawnSync(process.execPath, [scriptPath, "--tag", "invalid"], {
      cwd: repoRoot,
      encoding: "utf8",
    });

    assert.equal(success.status, 0);
    assert.equal(success.stderr, "");
    assert.deepEqual(JSON.parse(success.stdout), {
      ok: true,
      data: {
        packageVersion,
        lockVersion: packageVersion,
        lockRootVersion: packageVersion,
        tagVersion: packageVersion,
      },
    });
    assert.equal(failure.status, 1);
    assert.equal(failure.stdout, "");
    assertStructuredError(JSON.parse(failure.stderr));
  });

  it("keeps package metadata synchronized without changing the Node engine", () => {
    const packageJson = require(path.join(repoRoot, "package.json"));
    const packageLockJson = require(path.join(repoRoot, "package-lock.json"));

    assert.equal(packageJson.scripts["release:check"], "node scripts/release-preflight.js");
    assert.equal(packageJson.engines.node, ">=16");
    assert.equal(packageLockJson.version, packageJson.version);
    assert.equal(packageLockJson.packages[""].version, packageJson.version);
  });

  it("runs every release gate before any tag query, creation, or push", () => {
    const script = fs.readFileSync(path.join(repoRoot, "scripts/tag-release.sh"), "utf8");
    const preflight = script.indexOf('node "${SCRIPT_DIR}/release-preflight.js" --tag "${TAG}"');
    const requiredGates = [
      'CURRENT_BRANCH=$(git branch --show-current)',
      'git status --porcelain',
      'git fetch origin "${REHEARSAL_BRANCH}"',
      'git rev-parse "FETCH_HEAD^{commit}"',
      'git show "${HEAD_SHA}:.github/workflows/release.yml"',
      'npm view "@larksuite/cli@${VERSION}" version',
    ];
    const tagOperations = [
      'git rev-parse -q --verify "refs/tags/${TAG}"',
      'git ls-remote --tags origin "refs/tags/${TAG}"',
      'git tag "${TAG}" "${HEAD_SHA}"',
      'git push origin "refs/tags/${TAG}:refs/tags/${TAG}"',
    ];

    assert.ok(preflight >= 0, "release preflight invocation is missing");
    assertInOrder(script, [
      'REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"',
      'cd "${REPO_ROOT}"',
      'node "${SCRIPT_DIR}/release-preflight.js"',
    ]);
    assert.equal(script.includes("require('${REPO_ROOT}/package.json')"), false);
    for (const gate of requiredGates) {
      const index = script.indexOf(gate);
      assert.ok(index >= 0, `required release gate is missing: ${gate}`);
      assert.ok(index < script.indexOf(tagOperations[0]), `${gate} must run before tag queries`);
    }
    for (const operation of tagOperations) {
      const index = script.indexOf(operation);
      assert.ok(index >= 0, `tag operation is missing: ${operation}`);
      assert.ok(preflight < index, `preflight must run before: ${operation}`);
    }
    assertInOrder(script, [
      'if [ "${PUSH_TAG}" != true ]',
      'read -r CONFIRM_TAG',
      'git tag "${TAG}" "${HEAD_SHA}"',
      'git push origin "refs/tags/${TAG}:refs/tags/${TAG}"',
    ]);
  });
});

describe("tag-release.sh behavior", () => {
  it("runs repository checks from the script repository when invoked elsewhere", (t) => {
    const fixture = createReleaseFixture(t);
    const outside = fs.mkdtempSync(path.join(os.tmpdir(), "tag-release-cwd-"));
    t.after(() => fs.rmSync(outside, { recursive: true, force: true }));

    const result = runTagRelease(fixture, { cwd: outside });

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects a non-rehearsal branch before querying or modifying tags", (t) => {
    const fixture = createReleaseFixture(t, { FAKE_BRANCH: "feature/release" });

    const result = runTagRelease(fixture);
    const calls = readGitCalls(fixture);

    assert.equal(result.status, 1);
    assert.match(result.stderr, /must be created from test\/npm-staged-publish-rehearsal/i);
    assertNoTagOperations(calls);
  });

  it("rejects a dirty working tree before tag operations", (t) => {
    const fixture = createReleaseFixture(t, { FAKE_STATUS_OUTPUT: " M README.md" });
    const result = runTagRelease(fixture);
    const calls = readGitCalls(fixture);

    assert.equal(result.status, 1);
    assert.match(result.stderr, /working tree must be clean/i);
    assertNoTagOperations(calls);
  });

  it("rejects HEAD that differs from the fetched rehearsal branch", (t) => {
    const fixture = createReleaseFixture(t, { FAKE_REHEARSAL_SHA: "bbbbbbbb" });

    const result = runTagRelease(fixture);
    const calls = readGitCalls(fixture);

    assert.equal(result.status, 1);
    assert.match(result.stderr, /HEAD must exactly match origin\/test\/npm-staged-publish-rehearsal/i);
    assertNoTagOperations(calls);
  });

  it("compares HEAD with the exact fetched rehearsal commit", (t) => {
    const fixture = createReleaseFixture(t);

    const result = runTagRelease(fixture);
    const calls = readGitCalls(fixture);

    assert.equal(result.status, 0, result.stderr);
    assert.ok(calls.some((args) => args.join(" ") === "fetch origin test/npm-staged-publish-rehearsal"));
    assert.ok(calls.some((args) => args.join(" ") === "rev-parse FETCH_HEAD^{commit}"));
    assert.equal(calls.some((args) => args.includes("origin/test/npm-staged-publish-rehearsal")), false);
  });

  it("fails when the local tag already exists", (t) => {
    const fixture = createReleaseFixture(t);
    fs.writeFileSync(path.join(fixture.stateDir, "local-tag"), "bbbbbbbb");

    const result = runTagRelease(fixture);
    const calls = readGitCalls(fixture);

    assert.equal(result.status, 1);
    assert.match(result.stderr, /local tag .* already exists/i);
    assert.equal(calls.some((args) => args[0] === "ls-remote"), false);
    assert.equal(calls.some((args) => args[0] === "push"), false);
  });

  it("fails when a lightweight or annotated remote tag already exists", (t) => {
    for (const kind of ["lightweight", "annotated"]) {
      const fixture = createReleaseFixture(t, {
        FAKE_REMOTE_TAG_KIND: kind,
        FAKE_REMOTE_TAG_SHA: "aaaaaaaa",
      });

      const result = runTagRelease(fixture);
      const calls = readGitCalls(fixture);

      assert.equal(result.status, 1, `${kind}: ${result.stderr}`);
      assert.match(result.stderr, /remote tag .* already exists/i);
      assert.equal(calls.some((args) => args[0] === "tag"), false);
      assert.equal(calls.some((args) => args[0] === "push"), false);
    }
  });

  it("check mode completes without creating or pushing a tag", (t) => {
    const fixture = createReleaseFixture(t);

    const result = runTagRelease(fixture);
    const calls = readGitCalls(fixture);

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /No tag was created or pushed/);
    assertNoTagWrites(calls);
  });

  it("rejects a production version before invoking git", (t) => {
    const fixture = createReleaseFixture(t);
    fs.writeFileSync(path.join(fixture.root, "package.json"), '{"version":"1.2.3"}\n');
    fs.writeFileSync(
      path.join(fixture.root, "package-lock.json"),
      '{"version":"1.2.3","packages":{"":{"version":"1.2.3"}}}\n',
    );

    const result = runTagRelease(fixture);

    assert.equal(result.status, 1);
    assert.match(result.stderr, /require an X\.Y\.Z-beta\.N version/);
    assert.deepEqual(readGitCalls(fixture), []);
  });

  it("rejects a workflow that can publish live", (t) => {
    for (const workflow of [
      "args: release --clean --skip=publish\nrun: npm publish --access public",
      "args: release --clean --skip=publish\nrun: npm stage publish package.tgz --access public --tag beta\nrun: gh release create v1.2.3-beta.0",
      "args: release --clean --skip=publish\npermissions:\n  contents: write\nrun: npm stage publish package.tgz --access public --tag beta",
      "args: release --clean --skip=publish\nenv:\n  GITHUB_TOKEN: ${{ github.token }}\nrun: npm stage publish package.tgz --access public --tag beta",
    ]) {
      const fixture = createReleaseFixture(t, { FAKE_WORKFLOW: workflow });
      const result = runTagRelease(fixture);

      assert.equal(result.status, 1);
      assert.match(result.stderr, /must be stage-only/i);
      assertNoTagWrites(readGitCalls(fixture));
    }
  });

  it("fails closed when npm cannot prove that the version is unused", (t) => {
    const fixture = createReleaseFixture(t, {
      FAKE_NPM_VIEW_OUTPUT: "npm error code ETIMEDOUT",
    });

    const result = runTagRelease(fixture);

    assert.equal(result.status, 1);
    assert.match(result.stderr, /npm version lookup failed/i);
    assertNoTagWrites(readGitCalls(fixture));
  });

  it("rejects an existing npm version", (t) => {
    const fixture = createReleaseFixture(t, {
      FAKE_NPM_VIEW_STATUS: "0",
      FAKE_NPM_VIEW_OUTPUT: "1.2.3-beta.0",
    });

    const result = runTagRelease(fixture);

    assert.equal(result.status, 1);
    assert.match(result.stderr, /already exists on npm/i);
    assertNoTagWrites(readGitCalls(fixture));
  });

  it("requires the full tag confirmation in push mode", (t) => {
    const fixture = createReleaseFixture(t);

    const result = runTagRelease(fixture, { args: ["--push"], input: "no\n" });

    assert.equal(result.status, 1);
    assert.match(result.stderr, /confirmation did not exactly match/i);
    assertNoTagWrites(readGitCalls(fixture));
  });

  it("pushes only the exact confirmed tag ref", (t) => {
    const fixture = createReleaseFixture(t);

    const result = runTagRelease(fixture, {
      args: ["--push"],
      input: "v1.2.3-beta.0\n",
    });
    const calls = readGitCalls(fixture);

    assert.equal(result.status, 0, result.stderr);
    assert.ok(calls.some((args) => args.join(" ") === "tag v1.2.3-beta.0 aaaaaaaa"));
    assert.ok(calls.some((args) =>
      args.join(" ") === "push origin refs/tags/v1.2.3-beta.0:refs/tags/v1.2.3-beta.0"));
    assert.equal(
      calls.some((args) => args[0] === "push" && args.includes("--tags")),
      false,
    );
  });

  it("reports an invalid package version before invoking git", (t) => {
    const fixture = createReleaseFixture(t);
    fs.writeFileSync(path.join(fixture.root, "package.json"), '{"version":"01.2.3"}\n');

    const result = runTagRelease(fixture);

    assert.equal(result.status, 1);
    assert.equal(result.stdout, "");
    assertStructuredError(JSON.parse(result.stderr));
    assert.deepEqual(readGitCalls(fixture), []);
  });
});

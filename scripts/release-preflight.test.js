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
  fs.writeFileSync(path.join(root, "package.json"), '{"version":"1.2.3"}\n');
  fs.writeFileSync(
    path.join(root, "package-lock.json"),
    '{"version":"1.2.3","packages":{"":{"version":"1.2.3"}}}\n',
  );

  const fakeGitPath = path.join(binDir, "git");
  fs.writeFileSync(fakeGitPath, String.raw`#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");

const args = process.argv.slice(2);
const stateDir = process.env.FAKE_GIT_STATE_DIR;
const localTagPath = path.join(stateDir, "local-tag");
fs.appendFileSync(process.env.FAKE_GIT_LOG, JSON.stringify(args) + "\n");

function print(value) {
  process.stdout.write(value + "\n");
}

switch (args[0]) {
  case "branch":
    print(process.env.FAKE_BRANCH || "main");
    break;
  case "diff":
  case "fetch":
    break;
  case "rev-parse": {
    const ref = args[args.length - 1];
    if (ref === "HEAD") {
      print(process.env.FAKE_HEAD_SHA);
      break;
    }
    if (ref === "FETCH_HEAD^{commit}") {
      print(process.env.FAKE_MAIN_SHA);
      break;
    }
    if (ref.startsWith("refs/tags/") && ref.endsWith("^{commit}")) {
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
    if (kind === "lightweight") {
      print(process.env.FAKE_REMOTE_TAG_SHA + "\t" + tagRef);
    } else if (kind === "annotated") {
      print((process.env.FAKE_REMOTE_TAG_OBJECT_SHA || "cccccccc") + "\t" + tagRef);
      print(process.env.FAKE_REMOTE_TAG_SHA + "\t" + tagRef + "^{}");
    }
    break;
  }
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
      FAKE_GIT_STATE_DIR: stateDir,
      FAKE_HEAD_SHA: "aaaaaaaa",
      FAKE_MAIN_SHA: "aaaaaaaa",
      ...env,
    },
  };
}

function runTagRelease(fixture) {
  return spawnSync("bash", ["scripts/tag-release.sh"], {
    cwd: fixture.root,
    env: fixture.env,
    encoding: "utf8",
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

describe("validateReleasePreflight", () => {
  it("accepts matching stable versions and an optional matching tag", () => {
    const { packageJson, packageLockJson } = validInputs("1.2.3");

    assert.deepEqual(validateReleasePreflight(packageJson, packageLockJson), {
      ok: true,
      data: {
        packageVersion: "1.2.3",
        lockVersion: "1.2.3",
        lockRootVersion: "1.2.3",
        tagVersion: null,
      },
    });
    assert.deepEqual(
      validateReleasePreflight(
        packageJson,
        packageLockJson,
        "v1.2.3",
      ),
      {
        ok: true,
        data: {
          packageVersion: "1.2.3",
          lockVersion: "1.2.3",
          lockRootVersion: "1.2.3",
          tagVersion: "1.2.3",
        },
      },
    );
  });

  it("rejects prerelease package versions with the stable release contract", () => {
    const { packageJson, packageLockJson } = validInputs("1.2.3-rc.1");

    const result = validateReleasePreflight(packageJson, packageLockJson);

    assertStructuredError(result);
    assert.equal(
      result.error.message,
      "package.json.version must be a stable release version in X.Y.Z form",
    );
    assert.equal(
      result.error.hint,
      "Use the same stable X.Y.Z version in all package fields; prerelease and build metadata are not allowed for production releases.",
    );
  });

  it("rejects build metadata package versions with the stable release contract", () => {
    const { packageJson, packageLockJson } = validInputs("1.2.3+build.7");

    const result = validateReleasePreflight(packageJson, packageLockJson);

    assertStructuredError(result);
    assert.equal(
      result.error.message,
      "package.json.version must be a stable release version in X.Y.Z form",
    );
    assert.equal(
      result.error.hint,
      "Use the same stable X.Y.Z version in all package fields; prerelease and build metadata are not allowed for production releases.",
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
      'git diff --quiet -- package.json package-lock.json',
      'git diff --cached --quiet -- package.json package-lock.json',
      "git fetch origin main",
      'git rev-parse "FETCH_HEAD^{commit}"',
    ];
    const tagOperations = [
      'git rev-parse -q --verify "refs/tags/${TAG}^{commit}"',
      'git ls-remote --tags origin "refs/tags/${TAG}" "refs/tags/${TAG}^{}"',
      'git tag "${TAG}" "${HEAD_SHA}"',
      'git push origin "refs/tags/${TAG}"',
    ];

    assert.ok(preflight >= 0, "release preflight invocation is missing");
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
  });
});

describe("tag-release.sh behavior", () => {
  it("compares HEAD with the exact fetched main commit", (t) => {
    const fixture = createReleaseFixture(t);

    const result = runTagRelease(fixture);
    const calls = readGitCalls(fixture);

    assert.equal(result.status, 0, result.stderr);
    assert.ok(calls.some((args) => args.join(" ") === "fetch origin main"));
    assert.ok(calls.some((args) => args.join(" ") === "rev-parse FETCH_HEAD^{commit}"));
    assert.equal(calls.some((args) => args.includes("origin/main")), false);
  });

  it("retries only the push when a failed push left the correct local tag", (t) => {
    const fixture = createReleaseFixture(t, { FAKE_PUSH_FAIL_ONCE: "23" });

    const first = runTagRelease(fixture);
    const second = runTagRelease(fixture);
    const calls = readGitCalls(fixture);

    assert.equal(first.status, 23);
    assert.equal(second.status, 0, second.stderr);
    assert.equal(calls.filter((args) => args[0] === "tag").length, 1);
    assert.equal(calls.filter((args) => args[0] === "push").length, 2);
  });

  it("fails when an existing local tag points at another commit", (t) => {
    const fixture = createReleaseFixture(t);
    fs.writeFileSync(path.join(fixture.stateDir, "local-tag"), "bbbbbbbb");

    const result = runTagRelease(fixture);
    const calls = readGitCalls(fixture);

    assert.equal(result.status, 1);
    assert.match(result.stderr, /local tag .* does not point to HEAD/i);
    assert.equal(calls.some((args) => args[0] === "ls-remote"), false);
    assert.equal(calls.some((args) => args[0] === "push"), false);
  });

  it("accepts matching lightweight and annotated remote tag targets", (t) => {
    for (const kind of ["lightweight", "annotated"]) {
      const fixture = createReleaseFixture(t, {
        FAKE_REMOTE_TAG_KIND: kind,
        FAKE_REMOTE_TAG_SHA: "aaaaaaaa",
      });

      const result = runTagRelease(fixture);
      const calls = readGitCalls(fixture);

      assert.equal(result.status, 0, `${kind}: ${result.stderr}`);
      assert.equal(calls.some((args) => args[0] === "tag"), false);
      assert.equal(calls.some((args) => args[0] === "push"), false);
    }
  });

  it("fails when an existing remote tag targets another commit", (t) => {
    const fixture = createReleaseFixture(t, {
      FAKE_REMOTE_TAG_KIND: "annotated",
      FAKE_REMOTE_TAG_SHA: "bbbbbbbb",
    });

    const result = runTagRelease(fixture);
    const calls = readGitCalls(fixture);

    assert.equal(result.status, 1);
    assert.match(result.stderr, /remote tag .* does not point to HEAD/i);
    assert.equal(calls.some((args) => args[0] === "tag"), false);
    assert.equal(calls.some((args) => args[0] === "push"), false);
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

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

const { describe, it } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { spawnSync } = require("child_process");

const MARKER = "failed to launch the native binary";
const IS_WINDOWS = process.platform === "win32";
const BIN_NAME = "lark-cli" + (IS_WINDOWS ? ".exe" : "");
const SENTINEL = "--sentinel=LEAKCANARY";
const SENTINEL_TOKEN = "LEAKCANARY";

// run.js resolves the native binary relative to its own directory
// (<dir>/../bin/lark-cli), so a fixture needs a full scripts/ + bin/ layout.
// Building that under fs.mkdtempSync keeps the developer's real ./bin/lark-cli
// untouched and keeps the location unpredictable. Never replace this with a
// fixed path such as %TEMP%\lark-cli-test — dropping a copy of the node
// executable into a predictable user-writable directory changes Windows' DLL
// search order.
function makeSandbox(t) {
  // fs.realpathSync canonicalises the temp root. assertLaunchFailure checks
  // res.stderr.includes(sandbox.bin), but the path run.js prints comes from
  // __dirname, and Node canonicalises a main module's path. On macOS the
  // difference is /var -> /private/var, a prefix extension, so substring
  // containment still holds and the tests pass unnoticed. On Windows it is
  // NOT a prefix extension: libuv's realpath uses GetFinalPathNameByHandle
  // with FILE_NAME_NORMALIZED, which expands 8.3 short names, and
  // GitHub-hosted Windows runners set TEMP to
  // C:\Users\RUNNER~1\AppData\Local\Temp while the profile directory is
  // runneradmin. Short-name expansion is a substitution, not a prefix
  // extension, so includes() returns false and every launch-failure case
  // fails on the one platform the shim-test-windows job exists to cover. Do
  // not "simplify" this back to the bare mkdtempSync result.
  const root = fs.realpathSync(
    fs.mkdtempSync(path.join(os.tmpdir(), "lark-cli-run-"))
  );
  t.after(() =>
    fs.rmSync(root, {
      recursive: true,
      force: true,
      maxRetries: 5,
      retryDelay: 100,
    })
  );
  fs.mkdirSync(path.join(root, "scripts"));
  fs.mkdirSync(path.join(root, "bin"));
  fs.copyFileSync(
    path.join(__dirname, "run.js"),
    path.join(root, "scripts", "run.js")
  );
  // run.js runs install.js when the binary is missing. A stub that exits 0
  // without producing one lets the "installer succeeded but produced nothing"
  // path reach the launch attempt instead of stopping earlier.
  fs.writeFileSync(
    path.join(root, "scripts", "install.js"),
    "process.exit(0);\n"
  );
  return {
    root,
    bin: path.join(root, "bin", BIN_NAME),
    runJs: path.join(root, "scripts", "run.js"),
  };
}

function runShim(sandbox, args) {
  return spawnSync(process.execPath, [sandbox.runJs, ...args], {
    encoding: "utf8",
  });
}

// A fixture that actually launches: a copy of the running node executable.
// A #!/bin/sh script cannot be started as lark-cli.exe on Windows, which would
// make every "the binary ran" case inert there.
function installRunnableBinary(sandbox) {
  fs.copyFileSync(process.execPath, sandbox.bin);
  if (!IS_WINDOWS) {
    fs.chmodSync(sandbox.bin, 0o755);
  }
}

// `--` stops node from claiming the sentinel as one of its own options; without
// it node bails out with "bad option" and exits 9, which would silently
// invalidate the fixture instead of exercising the intended exit status.
function runRanBinary(sandbox, code) {
  return runShim(sandbox, ["-e", code, "--", SENTINEL]);
}

function assertBinaryRan(res) {
  assert.ok(
    !res.stderr.includes(MARKER),
    `launch-failure diagnostic on a binary that ran: ${res.stderr}`
  );
  assert.ok(
    !res.stderr.includes(SENTINEL_TOKEN),
    `caller arguments leaked into stderr: ${res.stderr}`
  );
}

function assertLaunchFailure(res, sandbox) {
  assert.equal(res.status, 1);
  assert.equal(res.stdout, "", `stdout should stay empty: ${res.stdout}`);
  assert.ok(
    res.stderr.includes(MARKER),
    `missing diagnostic, stderr was: ${res.stderr}`
  );
  assert.ok(
    res.stderr.includes(sandbox.bin),
    `diagnostic omits the binary path: ${res.stderr}`
  );
  const line = res.stderr.match(/^ {2}error: (.+)$/m);
  assert.ok(line, `diagnostic omits an error line: ${res.stderr}`);
  const reason = line[1].trim();
  assert.notEqual(reason, "", "error line is empty");
  // Closure of a security review finding: a user facing EACCES with no
  // pointer tends to reach for sudo / chmod 777 / disabling endpoint
  // protection. Without this assertion, deleting the tracker line (and its
  // follow-up sentence) from run.js leaves the suite fully green.
  assert.ok(
    res.stderr.includes(
      "Report this error at https://github.com/larksuite/cli/issues"
    ),
    `diagnostic omits the issue tracker line: ${res.stderr}`
  );
  assert.ok(
    res.stderr.includes("Please include the path and error shown above."),
    `diagnostic omits the follow-up line: ${res.stderr}`
  );
  return reason;
}

describe("run.js launch-failure diagnostics", () => {
  it("reports a zero-byte binary instead of exiting silently", (t) => {
    const sandbox = makeSandbox(t);
    fs.writeFileSync(sandbox.bin, "");
    if (!IS_WINDOWS) {
      fs.chmodSync(sandbox.bin, 0o755);
    }

    const res = runShim(sandbox, ["--version"]);
    const reason = assertLaunchFailure(res, sandbox);

    // The errno is deliberately not pinned to a literal here: POSIX reports
    // ENOEXEC, and finding out what Windows reports is the reason this file
    // also runs on windows-latest. Only require that something specific was
    // captured.
    assert.notEqual(reason, "UNKNOWN", "the OS error was not captured");

    // Endpoint protection can quarantine a zero-byte .exe, which would silently
    // turn this into the missing-binary case.
    assert.ok(fs.existsSync(sandbox.bin), "fixture binary vanished before launch");
    assert.equal(
      fs.statSync(sandbox.bin).size,
      0,
      "fixture binary is no longer zero-byte"
    );

    if (process.env.GITHUB_STEP_SUMMARY) {
      fs.appendFileSync(
        process.env.GITHUB_STEP_SUMMARY,
        `- \`run.js\` zero-byte binary on ${process.platform}: \`${reason}\`\n`
      );
    }
  });

  it(
    "reports a binary that cannot be executed",
    { skip: IS_WINDOWS ? "POSIX execute bit" : false },
    (t) => {
      const sandbox = makeSandbox(t);
      fs.writeFileSync(sandbox.bin, "#!/bin/sh\nexit 0\n");
      fs.chmodSync(sandbox.bin, 0o000);

      const res = runShim(sandbox, ["--version"]);

      assert.equal(assertLaunchFailure(res, sandbox), "EACCES");
    }
  );

  it("reports a missing binary when the installer produced nothing", (t) => {
    const sandbox = makeSandbox(t);

    const res = runShim(sandbox, ["--version"]);

    assert.equal(assertLaunchFailure(res, sandbox), "ENOENT");
  });
});

describe("run.js pass-through when the binary runs", () => {
  it("forwards a non-zero exit status unchanged", (t) => {
    const sandbox = makeSandbox(t);
    installRunnableBinary(sandbox);

    const res = runRanBinary(sandbox, "process.exit(7)");

    assert.equal(res.status, 7);
    assertBinaryRan(res);
  });

  it(
    "exits 1 without a diagnostic when the binary is killed by a signal",
    { skip: IS_WINDOWS ? "POSIX signals" : false },
    (t) => {
      const sandbox = makeSandbox(t);
      installRunnableBinary(sandbox);

      const res = runRanBinary(sandbox, "process.kill(process.pid, 'SIGTERM')");

      assert.equal(res.status, 1);
      assertBinaryRan(res);
    }
  );

  it("passes stdout and stderr through and exits 0", (t) => {
    const sandbox = makeSandbox(t);
    installRunnableBinary(sandbox);

    const res = runRanBinary(
      sandbox,
      "console.log('shim-passthrough'); console.error('shim-stderr')"
    );

    assert.equal(res.status, 0);
    assert.match(res.stdout, /shim-passthrough/);
    // A shim that swallowed child stderr would be a variant of the very bug
    // this branch repairs (issue #2053: launch failures with no evidence).
    // This does not collide with assertBinaryRan's checks below: it asserts
    // MARKER and the argv sentinel are absent from stderr, and "shim-stderr"
    // is neither.
    assert.match(res.stderr, /shim-stderr/);
    assertBinaryRan(res);
  });
});

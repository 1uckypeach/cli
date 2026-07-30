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

// run.js resolves the native binary relative to its own directory
// (<dir>/../bin/lark-cli), so a fixture needs a full scripts/ + bin/ layout.
// Building that under fs.mkdtempSync keeps the developer's real ./bin/lark-cli
// untouched and keeps the location unpredictable. Never replace this with a
// fixed path such as %TEMP%\lark-cli-test — dropping a copy of the node
// executable into a predictable user-writable directory changes Windows' DLL
// search order.
function makeSandbox(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "lark-cli-run-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
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

function assertLaunchFailure(res, sandbox) {
  assert.equal(res.status, 1);
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

#!/usr/bin/env node
// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

const { execFileSync } = require("child_process");
const fs = require("fs");
const path = require("path");

const ext = process.platform === "win32" ? ".exe" : "";
const bin = path.join(__dirname, "..", "bin", "lark-cli" + ext);

// On Windows, a crashed self-update may have left the binary renamed to .old.
// Recover it before proceeding so the CLI remains functional.
const oldBin = bin + ".old";
function restoreOldBinary() {
  try {
    if (fs.existsSync(bin)) {
      fs.rmSync(bin, { force: true });
    }
    fs.renameSync(oldBin, bin);
    return true;
  } catch (_) {
    return false;
  }
}

if (process.platform === "win32" && fs.existsSync(oldBin)) {
  if (!fs.existsSync(bin)) {
    restoreOldBinary();
  } else {
    try {
      execFileSync(bin, ["--version"], { stdio: "ignore", timeout: 10000 });
      try {
        fs.rmSync(oldBin, { force: true });
      } catch (_) {
        // Best-effort cleanup; keep running the healthy binary.
      }
    } catch (_) {
      restoreOldBinary();
    }
  }
}

// Intercept "install" subcommand — run the setup wizard directly,
// bypassing the native binary (which may not exist yet under npx).
const args = process.argv.slice(2);
if (args[0] === "install") {
  require("./install-wizard.js");
} else {
  // Auto-download binary if missing (e.g. npx skipped postinstall).
  if (!fs.existsSync(bin)) {
    try {
      execFileSync(process.execPath, [path.join(__dirname, "install.js")], {
        stdio: "inherit",
        env: { ...process.env, LARK_CLI_RUN: "true" },
      });
    } catch (_) {
      console.error(
        `\nFailed to auto-install lark-cli binary.\n` +
        `To fix, run the install script manually:\n` +
        `  node "${path.join(__dirname, "install.js")}"\n`
      );
      process.exit(1);
    }
  }

  try {
    execFileSync(bin, args, { stdio: "inherit" });
  } catch (e) {
    // The binary ran and chose its own status — forward it untouched.
    if (typeof e.status === "number") {
      process.exit(e.status);
    }
    // SIGINT and SIGTERM are the explicit quiet allowlist for intentional
    // interruption (Ctrl+C during `auth login`, for one). Other signals are
    // crash evidence worth reporting, but do not prove the binary failed to
    // launch. Only print e.signal and the known bin path: e.message and related
    // error fields can contain the caller's full argv.
    if (e.signal) {
      if (e.signal === "SIGINT" || e.signal === "SIGTERM") {
        process.exit(1);
      }
      console.error(
        `\nlark-cli: the native binary was terminated by signal ${e.signal}.\n` +
        `  path:  ${bin}\n\n` +
        `Report this error at https://github.com/larksuite/cli/issues\n` +
        `Please include the path and signal shown above.\n`
      );
      process.exit(1);
    }
    // Neither: the launch itself failed. Report only what is actually known —
    // permissions, file format, CPU architecture and endpoint policy all land
    // here, and the errno is the only evidence. Print e.code and never
    // e.message: when the child does run, e.message carries the full argv,
    // which can contain values the caller passed on the command line.
    const reason = typeof e.code === "string" ? e.code : "UNKNOWN";
    console.error(
      `\nlark-cli: failed to launch the native binary.\n` +
      `  path:  ${bin}\n` +
      `  error: ${reason}\n\n` +
      `Report this error at https://github.com/larksuite/cli/issues\n` +
      `Please include the path and error shown above.\n`
    );
    process.exit(1);
  }
}

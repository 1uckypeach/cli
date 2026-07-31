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
    // Every branch below that prints a diagnostic ends with
    // `process.exitCode = 1` and a plain return/fallthrough, never
    // `process.exit(1)`. On POSIX, writes to a piped stderr are asynchronous
    // (unlike a TTY); `process.exit()` tears the process down immediately and
    // can drop a write that has not finished flushing yet. That is exactly
    // what happens when the child has just filled a piped stderr (the
    // AI-agent/log-wrapper case this CLI is built for): `process.exit(1)`
    // right after `console.error(...)` can lose the entire diagnostic.
    // Leaving `process.exitCode` set and returning naturally lets the event
    // loop drain pending writes before the process actually exits, so the
    // diagnostic survives; the exit code is still 1 either way. The silent
    // branches (numeric-status forward, quiet SIGINT/SIGTERM) print nothing,
    // so they keep using `process.exit` directly.
    // The binary ran and chose its own status — forward it untouched.
    if (typeof e.status === "number") {
      // Windows has no signals: a crashing native binary (access violation
      // 0xC0000005, missing DLL 0xC0000135, killed by endpoint protection) shows
      // up as an NTSTATUS number here, not via e.signal. Surface the error-severity
      // range (0xC0000000+) as a crash instead of forwarding it silently. Exclude
      // 0xC000013A (STATUS_CONTROL_C_EXIT), the Windows Ctrl+C code, to stay
      // symmetric with the quiet SIGINT/SIGTERM allowlist. A Go binary never exits
      // with a code in this range deliberately.
      if (
        process.platform === "win32" &&
        e.status >= 0xc0000000 &&
        e.status !== 0xc000013a
      ) {
        console.error(
          `\nlark-cli: the native binary crashed (status 0x${(e.status >>> 0).toString(16)}).\n` +
          `  path:  ${bin}`
        );
        process.exitCode = 1;
        return;
      }
      process.exit(e.status);
    }
    // SIGINT and SIGTERM are the explicit quiet allowlist for intentional
    // interruption (Ctrl+C during `auth login`, for one). Other signals are
    // crash evidence worth surfacing, but do not prove the binary failed to
    // launch. Only print e.signal and the known bin path: e.message and related
    // error fields can contain the caller's full argv.
    if (e.signal) {
      if (e.signal === "SIGINT" || e.signal === "SIGTERM") {
        process.exit(1);
      }
      console.error(
        `\nlark-cli: the native binary was terminated by signal ${e.signal}.\n` +
        `  path:  ${bin}`
      );
      process.exitCode = 1;
      return;
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
      `  error: ${reason}`
    );
    process.exitCode = 1;
  }
}

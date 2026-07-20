// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

const assert = require("node:assert/strict");
const { spawnSync } = require("node:child_process");
const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { describe, it } = require("node:test");

const {
  parseChecksumManifest,
  validateArchiveSet,
  verifyReleaseAssets,
} = require("./verify-release-assets");

const scriptPath = path.join(__dirname, "verify-release-assets.js");

function sha256(content) {
  return crypto.createHash("sha256").update(content).digest("hex");
}

function createFixture(t, files = {}) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "release-assets-test-"));
  for (const [name, content] of Object.entries(files)) {
    fs.writeFileSync(path.join(directory, name), content);
  }
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  return directory;
}

function manifestFor(files) {
  return Object.entries(files)
    .map(([name, content]) => `${sha256(content)}  ${name}`)
    .join("\n") + "\n";
}

function releaseArchives() {
  return {
    "lark-cli-darwin-arm64.tar.gz": "darwin archive",
    "lark-cli-windows-amd64.zip": "windows archive",
  };
}

async function assertRejectsMessage(promise, pattern) {
  await assert.rejects(promise, (error) => {
    assert.match(error.message, pattern);
    return true;
  });
}

describe("checksum manifest validation", () => {
  it("parses multiple archives and ignores blank lines", () => {
    const first = "a".repeat(64);
    const second = "B".repeat(64);

    assert.deepEqual(
      [...parseChecksumManifest(`${first}  cli-linux.tar.gz\n\n${second}  cli.zip\n`)],
      [
        ["cli-linux.tar.gz", first],
        ["cli.zip", second.toLowerCase()],
      ],
    );
  });

  it("rejects duplicate file names", () => {
    const hash = "a".repeat(64);
    assert.throws(
      () => parseChecksumManifest(`${hash}  cli.zip\n${hash}  cli.zip\n`),
      /duplicate checksum entry.*cli\.zip/i,
    );
  });

  it("rejects invalid digests", () => {
    for (const digest of ["a".repeat(63), "g".repeat(64)]) {
      assert.throws(
        () => parseChecksumManifest(`${digest}  cli.zip\n`),
        /malformed checksum line/i,
      );
    }
  });

  it("rejects malformed lines and non-archive entries", () => {
    const hash = "a".repeat(64);
    for (const line of [
      `${hash} cli.zip`,
      `${hash}   cli.zip`,
      `${hash}\tcli.zip`,
      `${hash}  notes.txt`,
    ]) {
      assert.throws(
        () => parseChecksumManifest(`${line}\n`),
        /malformed checksum line|release archive/i,
      );
    }
  });

  it("requires exact bidirectional agreement with archive files", () => {
    const checksums = new Map([
      ["cli-linux.tar.gz", "a".repeat(64)],
      ["cli.zip", "b".repeat(64)],
    ]);

    assert.doesNotThrow(() => validateArchiveSet(checksums, ["cli.zip", "cli-linux.tar.gz"]));
    assert.throws(
      () => validateArchiveSet(checksums, ["cli-linux.tar.gz"]),
      /checksum entries without archive files.*cli\.zip/i,
    );
    assert.throws(
      () => validateArchiveSet(new Map([["cli.zip", "b".repeat(64)]]), ["cli.zip", "extra.tar.gz"]),
      /archive files without checksum entries.*extra\.tar\.gz/i,
    );
  });
});

describe("verifyReleaseAssets", () => {
  it("verifies every archive listed in checksums.txt while ignoring auxiliary files", async (t) => {
    const archives = releaseArchives();
    const directory = createFixture(t, {
      ...archives,
      "checksums.txt": manifestFor(archives),
      "release-notes.md": "notes",
    });

    assert.deepEqual(await verifyReleaseAssets(directory), {
      archiveCount: 2,
    });
  });

  it("rejects a missing or empty checksums.txt", async (t) => {
    const missing = createFixture(t, { "cli.zip": "archive" });
    const empty = createFixture(t, {
      "cli.zip": "archive",
      "checksums.txt": "\n  \n",
    });

    await assertRejectsMessage(verifyReleaseAssets(missing), /checksums\.txt.*missing/i);
    await assertRejectsMessage(verifyReleaseAssets(empty), /checksums\.txt.*empty/i);
  });

  it("rejects a directory without archive files", async (t) => {
    const directory = createFixture(t, {
      "checksums.txt": `${"a".repeat(64)}  cli.zip\n`,
      "notes.txt": "helper",
    });

    await assertRejectsMessage(verifyReleaseAssets(directory), /no release archives/i);
  });

  it("rejects an archive missing from checksums.txt", async (t) => {
    const archives = releaseArchives();
    const missingChecksum = "lark-cli-windows-amd64.zip";
    const manifestArchives = { ...archives };
    delete manifestArchives[missingChecksum];
    const directory = createFixture(t, {
      ...archives,
      "checksums.txt": manifestFor(manifestArchives),
    });

    await assertRejectsMessage(
      verifyReleaseAssets(directory),
      /archive files without checksum entries.*windows-amd64\.zip/i,
    );
  });

  it("rejects a checksum entry whose archive does not exist", async (t) => {
    const archives = releaseArchives();
    const directory = createFixture(t, {
      ...archives,
      "checksums.txt": [
        manifestFor(archives).trimEnd(),
        `${sha256("missing")}  missing.tar.gz`,
      ].join("\n"),
    });

    await assertRejectsMessage(
      verifyReleaseAssets(directory),
      /checksum entries without archive files.*missing\.tar\.gz/i,
    );
  });

  it("rejects a digest mismatch", async (t) => {
    const archives = releaseArchives();
    const mismatch = "lark-cli-windows-amd64.zip";
    const directory = createFixture(t, {
      ...archives,
      "checksums.txt": manifestFor({ ...archives, [mismatch]: "different" }),
    });

    await assertRejectsMessage(
      verifyReleaseAssets(directory),
      /SHA-256 mismatch.*windows-amd64\.zip/i,
    );
  });
});

describe("command line interface", () => {
  it("uses the current directory by default and prints deterministic success", (t) => {
    const archives = releaseArchives();
    const directory = createFixture(t, {
      ...archives,
      "checksums.txt": manifestFor(archives),
    });

    const result = spawnSync(process.execPath, [scriptPath], {
      cwd: directory,
      encoding: "utf8",
    });

    assert.equal(result.status, 0);
    assert.equal(result.stderr, "");
    assert.equal(result.stdout, "Verified 2 release archives.\n");
  });

  it("accepts a directory argument and reports failures on stderr", (t) => {
    const directory = createFixture(t, { "cli.zip": "zip" });

    const result = spawnSync(process.execPath, [scriptPath, directory], {
      encoding: "utf8",
    });

    assert.equal(result.status, 1);
    assert.equal(result.stdout, "");
    assert.match(result.stderr, /^verify-release-assets: .*checksums\.txt.*missing\n$/i);
  });
});

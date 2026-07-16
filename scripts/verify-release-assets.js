#!/usr/bin/env node
// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");

const CHECKSUM_PATTERN = /^([0-9a-fA-F]{64})  ([^\r\n]+)$/;
const STABLE_VERSION_PATTERN = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;
const RELEASE_ARCHIVE_SUFFIXES = [
  "darwin-amd64.tar.gz",
  "darwin-arm64.tar.gz",
  "linux-amd64.tar.gz",
  "linux-arm64.tar.gz",
  "linux-riscv64.tar.gz",
  "windows-amd64.zip",
  "windows-arm64.zip",
];

function isReleaseArchive(name) {
  return name.endsWith(".tar.gz") || name.endsWith(".zip");
}

function parseChecksumManifest(contents) {
  const checksums = new Map();

  for (const [index, line] of contents.split(/\r?\n/).entries()) {
    if (line.trim() === "") {
      continue;
    }

    const match = CHECKSUM_PATTERN.exec(line);
    if (!match) {
      throw new Error(`Malformed checksum line ${index + 1}`);
    }

    const [, digest, name] = match;
    if (name.trim() !== name) {
      throw new Error(`Malformed checksum line ${index + 1}`);
    }
    if (
      name === "." ||
      name === ".." ||
      /^[A-Za-z]:/.test(name) ||
      name.includes("/") ||
      name.includes("\\") ||
      path.posix.isAbsolute(name) ||
      path.win32.isAbsolute(name)
    ) {
      throw new Error(`Checksum line ${index + 1} must contain a basename`);
    }
    if (!isReleaseArchive(name)) {
      throw new Error(`Checksum line ${index + 1} does not name a release archive`);
    }
    if (checksums.has(name)) {
      throw new Error(`Duplicate checksum entry for ${name}`);
    }

    checksums.set(name, digest.toLowerCase());
  }

  if (checksums.size === 0) {
    throw new Error("checksums.txt is empty");
  }

  return checksums;
}

function validateArchiveSet(checksums, archiveNames) {
  const archives = new Set(archiveNames);
  const missingFiles = [...checksums.keys()]
    .filter((name) => !archives.has(name))
    .sort();
  if (missingFiles.length > 0) {
    throw new Error(`Checksum entries without archive files: ${missingFiles.join(", ")}`);
  }

  const missingChecksums = [...archives]
    .filter((name) => !checksums.has(name))
    .sort();
  if (missingChecksums.length > 0) {
    throw new Error(`Archive files without checksum entries: ${missingChecksums.join(", ")}`);
  }
}

function validateReleaseArchiveMatrix(archiveNames, version) {
  if (typeof version !== "string" || !STABLE_VERSION_PATTERN.test(version)) {
    throw new Error("Release version must use stable X.Y.Z form");
  }

  const expected = new Set(
    RELEASE_ARCHIVE_SUFFIXES.map((suffix) => `lark-cli-${version}-${suffix}`),
  );
  const archives = new Set(archiveNames);
  const missing = [...expected].filter((name) => !archives.has(name)).sort();
  const unexpected = [...archives].filter((name) => !expected.has(name)).sort();

  if (missing.length > 0) {
    throw new Error(`Required release archives are missing: ${missing.join(", ")}`);
  }
  if (unexpected.length > 0) {
    throw new Error(`Unexpected release archives: ${unexpected.join(", ")}`);
  }
}

function hashFile(filePath) {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash("sha256");
    const stream = fs.createReadStream(filePath);

    stream.on("error", reject);
    stream.on("data", (chunk) => hash.update(chunk));
    stream.on("end", () => resolve(hash.digest("hex")));
  });
}

async function verifyReleaseAssets(directory = process.cwd(), version) {
  const resolvedDirectory = path.resolve(directory);
  let entries;
  try {
    entries = await fs.promises.readdir(resolvedDirectory, { withFileTypes: true });
  } catch (error) {
    throw new Error(`Could not read release assets directory: ${error.message}`);
  }

  const archiveEntries = entries
    .filter((entry) => isReleaseArchive(entry.name))
    .sort((left, right) => left.name.localeCompare(right.name));
  const nonRegularArchives = archiveEntries
    .filter((entry) => !entry.isFile())
    .map((entry) => entry.name);
  if (nonRegularArchives.length > 0) {
    throw new Error(
      `Release archives are not regular files: ${nonRegularArchives.join(", ")}`,
    );
  }

  const archiveNames = archiveEntries
    .map((entry) => entry.name)
    .sort();
  if (archiveNames.length === 0) {
    throw new Error("No release archives found");
  }

  const checksumPath = path.join(resolvedDirectory, "checksums.txt");
  let checksumStat;
  let manifest;
  try {
    checksumStat = await fs.promises.lstat(checksumPath);
    if (!checksumStat.isFile()) {
      throw new Error("not a regular file");
    }
    manifest = await fs.promises.readFile(checksumPath, "utf8");
  } catch (error) {
    if (error.code === "ENOENT") {
      throw new Error("checksums.txt is missing");
    }
    throw new Error(`Could not read checksums.txt: ${error.message}`);
  }

  const checksums = parseChecksumManifest(manifest);
  validateArchiveSet(checksums, archiveNames);
  validateReleaseArchiveMatrix(archiveNames, version);

  for (const name of archiveNames) {
    let actual;
    try {
      actual = await hashFile(path.join(resolvedDirectory, name));
    } catch (error) {
      throw new Error(`Could not hash ${name}: ${error.message}`);
    }
    if (actual !== checksums.get(name)) {
      throw new Error(`SHA-256 mismatch for ${name}`);
    }
  }

  return { archiveCount: archiveNames.length };
}

async function main() {
  const args = process.argv.slice(2);
  if (args.length > 1) {
    process.stderr.write("verify-release-assets: expected zero or one directory argument\n");
    process.exitCode = 1;
    return;
  }

  try {
    const packageVersion = require(path.join(__dirname, "..", "package.json")).version;
    const result = await verifyReleaseAssets(args[0], packageVersion);
    process.stdout.write(`Verified ${result.archiveCount} release archives.\n`);
  } catch (error) {
    process.stderr.write(`verify-release-assets: ${error.message}\n`);
    process.exitCode = 1;
  }
}

module.exports = {
  isReleaseArchive,
  parseChecksumManifest,
  validateArchiveSet,
  validateReleaseArchiveMatrix,
  verifyReleaseAssets,
};

if (require.main === module) {
  main();
}

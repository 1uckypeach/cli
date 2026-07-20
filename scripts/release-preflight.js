#!/usr/bin/env node
// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

const fs = require("node:fs");
const path = require("node:path");

const STABLE_VERSION_PATTERN = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;

function isStableVersion(value) {
  if (typeof value !== "string") {
    return false;
  }

  const match = STABLE_VERSION_PATTERN.exec(value);
  return match !== null && match.slice(1).every(
    (component) => Number(component) <= Number.MAX_SAFE_INTEGER,
  );
}

function releaseError(message, observed, hint) {
  return {
    ok: false,
    error: {
      type: "release_preflight",
      message,
      observed,
      hint,
    },
  };
}

function observedVersions(packageVersion, lockVersion, lockRootVersion, tagVersion) {
  return {
    packageVersion: packageVersion === undefined ? null : packageVersion,
    lockVersion: lockVersion === undefined ? null : lockVersion,
    lockRootVersion: lockRootVersion === undefined ? null : lockRootVersion,
    tagVersion: tagVersion === undefined ? null : tagVersion,
  };
}

function validateReleasePreflight(packageJson, packageLockJson, tag) {
  const packageVersion = packageJson && packageJson.version;
  const lockVersion = packageLockJson && packageLockJson.version;
  const lockRootVersion = packageLockJson &&
    packageLockJson.packages &&
    packageLockJson.packages[""] &&
    packageLockJson.packages[""].version;
  const observed = observedVersions(
    packageVersion,
    lockVersion,
    lockRootVersion,
    null,
  );
  const versionFields = [
    ["package.json.version", packageVersion],
    ["package-lock.json.version", lockVersion],
    ['package-lock.json.packages[""].version', lockRootVersion],
  ];

  for (const [field, value] of versionFields) {
    if (!isStableVersion(value)) {
      return releaseError(
        `${field} must be a stable release version in X.Y.Z form`,
        observed,
        "Use the same stable X.Y.Z version in all package fields; prerelease and build metadata are not allowed for production releases.",
      );
    }
  }

  if (packageVersion !== lockVersion || packageVersion !== lockRootVersion) {
    return releaseError(
      "Package version fields do not match",
      observed,
      "Synchronize package.json.version and both package-lock.json version fields.",
    );
  }

  let tagVersion = null;
  if (tag !== undefined) {
    if (
      typeof tag !== "string" ||
      !tag.startsWith("v") ||
      !isStableVersion(tag.slice(1))
    ) {
      return releaseError(
        "--tag must use the stable release form vX.Y.Z",
        { ...observed, tag },
        `Use --tag v${packageVersion}; prerelease and build metadata are not allowed for production releases.`,
      );
    }

    tagVersion = tag.slice(1);
    if (tagVersion !== packageVersion) {
      return releaseError(
        "Tag version does not match the package version",
        {
          ...observedVersions(packageVersion, lockVersion, lockRootVersion, tagVersion),
          tag,
        },
        `Use --tag v${packageVersion}.`,
      );
    }
  }

  return {
    ok: true,
    data: {
      packageVersion,
      lockVersion,
      lockRootVersion,
      tagVersion,
    },
  };
}

function parseTagArgument(args) {
  if (args.length === 0) {
    return { tag: undefined };
  }
  if (args.length === 2 && args[0] === "--tag") {
    return { tag: args[1] };
  }
  return {
    error: releaseError(
      "Expected no arguments or --tag vX.Y.Z",
      { arguments: args },
      "Run release:check without arguments or pass exactly one --tag value.",
    ),
  };
}

function main() {
  const parsedArguments = parseTagArgument(process.argv.slice(2));
  if (parsedArguments.error) {
    process.stderr.write(`${JSON.stringify(parsedArguments.error)}\n`);
    process.exitCode = 1;
    return;
  }

  const repoRoot = path.resolve(__dirname, "..");
  let packageJson;
  let packageLockJson;
  try {
    packageJson = JSON.parse(fs.readFileSync(path.join(repoRoot, "package.json"), "utf8"));
    packageLockJson = JSON.parse(fs.readFileSync(path.join(repoRoot, "package-lock.json"), "utf8"));
  } catch (error) {
    const result = releaseError(
      "Could not read release package metadata",
      { reason: error.message },
      "Ensure package.json and package-lock.json exist and contain valid JSON.",
    );
    process.stderr.write(`${JSON.stringify(result)}\n`);
    process.exitCode = 1;
    return;
  }

  const result = validateReleasePreflight(
    packageJson,
    packageLockJson,
    parsedArguments.tag,
  );
  const stream = result.ok ? process.stdout : process.stderr;
  stream.write(`${JSON.stringify(result)}\n`);
  if (!result.ok) {
    process.exitCode = 1;
  }
}

module.exports = {
  validateReleasePreflight,
};

if (require.main === module) {
  main();
}

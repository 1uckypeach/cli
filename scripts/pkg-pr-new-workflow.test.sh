#!/usr/bin/env bash
# Copyright (c) 2026 Lark Technologies Pte. Ltd.
# SPDX-License-Identifier: MIT

set -euo pipefail

ruby -ryaml <<'RUBY'
workflow = YAML.load_file(".github/workflows/pkg-pr-new.yml")
abort("pkg.pr.new workflow must have read-only contents permission") unless workflow.fetch("permissions") == { "contents" => "read" }

job = workflow.fetch("jobs").fetch("publish")
abort("pkg.pr.new workflow must time out") unless job.fetch("timeout-minutes") == 15
abort("pkg.pr.new workflow must not reference secrets") if job.to_s.include?("secrets.")

steps = job.fetch("steps")
notices = steps.index { |step| step["name"] == "Check third-party notices" }
build = steps.index { |step| step["name"] == "Build preview package" }
publish = steps.index { |step| step["name"] == "Publish to pkg.pr.new" }
abort("pkg.pr.new must check notices before building") unless notices && build && notices < build
abort("pkg.pr.new must check notices before publishing") unless notices < publish
abort("pkg.pr.new must use Node 22.14.0") unless steps.any? { |step| step.dig("with", "node-version") == "22.14.0" }
abort("pkg.pr.new must pin npm 11.16.0") unless steps.any? { |step| step["run"] == "npm install --global npm@11.16.0" }
RUBY

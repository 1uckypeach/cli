// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

// The example provider self-registers via init(); in production it is pulled in
// by the top-level agent package (blank-imported from cmd/build.go), not by
// cmd/agent. Several tests here exercise the real example scheme (example:echo /
// example:reporter), so register it explicitly for the test binary.
import _ "github.com/larksuite/cli/agent/example"

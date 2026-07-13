// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"regexp"
	"strings"
	"testing"

	"github.com/larksuite/cli/shortcuts/common"
)

// The 12 high-frequency IM shortcuts covered by the governance closeout.
// Every entry must carry at least one copyable "Example:" tip locked by the
// tests below; other IM shortcuts (feed/flag series) are intentionally exempt.
var tipsExampleTargets = []string{
	"+messages-send", "+messages-search", "+chat-messages-list", "+messages-reply",
	"+chat-search", "+chat-list", "+messages-mget", "+threads-messages-list",
	"+messages-resources-download", "+chat-create", "+chat-update", "+chat-members-list",
	// Extension beyond the original high-frequency 12: feed/flag shortcuts with a
	// real guessing surface (oc_-only chat ids, --head/--tail exclusivity,
	// message- vs feed-layer flag types, ofg_ id sourcing). Pagination-only
	// shortcuts (+feed-shortcut-list, +feed-group-list, +flag-list) are
	// intentionally exempt — an example there would only restate flag Desc.
	"+feed-shortcut-create", "+feed-shortcut-remove",
	"+feed-group-list-item", "+feed-group-query-item",
	"+flag-create", "+flag-cancel",
}

var exampleFlagTokenRe = regexp.MustCompile(`--[a-z][a-z0-9-]*`)

// Flags injected by the shortcut runner framework rather than declared in
// Shortcut.Flags. --format comes with HasFormat, --json with HasJSON.
var frameworkInjectedFlags = map[string]bool{
	"--json": true, "--dry-run": true, "--as": true, "--yes": true, "--format": true,
}

func shortcutByCommand(t *testing.T, command string) common.Shortcut {
	t.Helper()
	for _, sc := range Shortcuts() {
		if sc.Command == command {
			return sc
		}
	}
	t.Fatalf("shortcut %s not registered in Shortcuts()", command)
	return common.Shortcut{}
}

// exampleCommands returns the command lines of "Example: ..." tips, with the
// "Example: " prefix stripped.
func exampleCommands(sc common.Shortcut) []string {
	prefix := "Example: lark-cli im " + sc.Command
	var out []string
	for _, tip := range sc.Tips {
		if strings.HasPrefix(tip, prefix+" ") || tip == prefix {
			out = append(out, strings.TrimPrefix(tip, "Example: "))
		}
	}
	return out
}

func TestIMTipsExamplesPresent(t *testing.T) {
	for _, cmd := range tipsExampleTargets {
		sc := shortcutByCommand(t, cmd)
		examples := exampleCommands(sc)
		if len(examples) < 1 {
			t.Errorf("%s: want >=1 tip starting with %q, got none (tips=%q)",
				cmd, "Example: lark-cli im "+cmd, sc.Tips)
		}
		if len(examples) > 3 {
			t.Errorf("%s: want <=3 examples to keep help focused, got %d", cmd, len(examples))
		}
	}
}

func TestIMTipsExampleFlagsExist(t *testing.T) {
	for _, cmd := range tipsExampleTargets {
		sc := shortcutByCommand(t, cmd)
		declared := map[string]bool{}
		for _, f := range sc.Flags {
			declared["--"+f.Name] = true
		}
		for _, example := range exampleCommands(sc) {
			for _, tok := range exampleFlagTokenRe.FindAllString(example, -1) {
				if !declared[tok] && !frameworkInjectedFlags[tok] {
					t.Errorf("%s: example uses %s which is neither a declared flag nor framework-injected\nexample: %s",
						cmd, tok, example)
				}
			}
		}
	}
}

func TestIMTipsFirstExampleCoversRequired(t *testing.T) {
	for _, cmd := range tipsExampleTargets {
		sc := shortcutByCommand(t, cmd)
		examples := exampleCommands(sc)
		if len(examples) == 0 {
			continue // reported by TestIMTipsExamplesPresent
		}
		for _, f := range sc.Flags {
			if !f.Required {
				continue
			}
			if !strings.Contains(examples[0], "--"+f.Name) {
				t.Errorf("%s: first example must cover required flag --%s\nexample: %s",
					cmd, f.Name, examples[0])
			}
		}
	}
}

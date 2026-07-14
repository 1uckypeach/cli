// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"context"
	"strings"
	"testing"
	"time"

	imshortcuts "github.com/larksuite/cli/shortcuts/im"
	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
)

// Placeholder substitutions turning copyable help examples into syntactically
// valid dry-run invocations. IDs are obvious fakes; --dry-run never hits the API.
var tipsPlaceholderValues = map[string]string{
	"<chat_id>":       "oc_e2etest000000000000000000",
	"<open_id>":       "ou_e2etest000000000000000000",
	"<message_id>":    "om_e2etest000000000000000000",
	"<thread_id>":     "omt_e2etest00000000000000000",
	"<file_key>":      "file_v3_e2etest0000000000000",
	"<image_key>":     "img_v3_e2etest00000000000000",
	"<open_id1>":      "ou_e2etest000000000000000001",
	"<open_id2>":      "ou_e2etest000000000000000002",
	"<message_id1>":   "om_e2etest000000000000000001",
	"<message_id2>":   "om_e2etest000000000000000002",
	"<feed_group_id>": "ofg_e2etest00000000000000000",
	"<chat_id1>":      "oc_e2etest000000000000000001",
	"<chat_id2>":      "oc_e2etest000000000000000002",
}

// firstExampleArgs extracts the first "Example:" tip of the shortcut, replaces
// placeholders, and returns the argv after "lark-cli".
func firstExampleArgs(t *testing.T, command string) []string {
	t.Helper()
	for _, sc := range imshortcuts.Shortcuts() {
		if sc.Command != command {
			continue
		}
		prefix := "Example: lark-cli "
		for _, tip := range sc.Tips {
			if !strings.HasPrefix(tip, prefix) {
				continue
			}
			line := strings.TrimPrefix(tip, prefix)
			for ph, v := range tipsPlaceholderValues {
				line = strings.ReplaceAll(line, ph, v)
			}
			return splitExampleArgs(t, line)
		}
		t.Fatalf("%s has no Example tip", command)
	}
	t.Fatalf("shortcut %s not found", command)
	return nil
}

// splitExampleArgs splits a shell-like example line on spaces, honoring
// double-quoted segments (the only quoting style used in Tips examples).
func splitExampleArgs(t *testing.T, line string) []string {
	t.Helper()
	var args []string
	var cur strings.Builder
	inQuote := false
	for _, r := range line {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ' ' && !inQuote:
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if inQuote {
		t.Fatalf("unbalanced quotes in example: %s", line)
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}

func runFirstExampleDryRun(t *testing.T, command string, wantAPIPath string) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "im_tips_dryrun_test")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "im_tips_dryrun_secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := append(firstExampleArgs(t, command), "--dry-run")
	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args:      args,
		DefaultAs: "bot",
		WorkDir:   t.TempDir(),
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	require.Contains(t, result.Stdout, wantAPIPath,
		"dry-run output should reference the upstream API path")
}

func TestIMTipsFirstExampleDryRunMessagesSend(t *testing.T) {
	runFirstExampleDryRun(t, "+messages-send", "/open-apis/im/v1/messages")
}

func TestIMTipsFirstExampleDryRunChatMessagesList(t *testing.T) {
	runFirstExampleDryRun(t, "+chat-messages-list", "/open-apis/im/v1/messages")
}

func TestIMTipsFirstExampleDryRunResourcesDownload(t *testing.T) {
	runFirstExampleDryRun(t, "+messages-resources-download", "/open-apis/im/v1/messages/")
}

// tipsExampleAllTargets mirrors shortcuts/im/tips_examples_test.go's
// tipsExampleTargets: the 12 high-frequency + 6 feed/flag shortcuts whose
// help carries a locked copyable "Example:" tip. Kept as a literal copy here
// because that list lives in an internal _test.go file not visible outside
// the shortcuts/im package.
var tipsExampleAllTargets = []string{
	"+messages-send", "+messages-search", "+chat-messages-list", "+messages-reply",
	"+chat-search", "+chat-list", "+messages-mget", "+threads-messages-list",
	"+messages-resources-download", "+chat-create", "+chat-update", "+chat-members-list",
	"+feed-shortcut-create", "+feed-shortcut-remove",
	"+feed-group-list-item", "+feed-group-query-item",
	"+flag-create", "+flag-cancel",
}

// defaultAsForCommand picks the identity to run the dry-run under by reading
// the shortcut's own AuthTypes: "bot" when the shortcut supports bot identity
// (matching the 3 pre-existing path-assertion tests above), otherwise "user"
// for user-only shortcuts (+messages-search and the whole feed/flag series).
func defaultAsForCommand(t *testing.T, command string) string {
	t.Helper()
	for _, sc := range imshortcuts.Shortcuts() {
		if sc.Command != command {
			continue
		}
		for _, a := range sc.AuthTypes {
			if a == "bot" {
				return "bot"
			}
		}
		return "user"
	}
	t.Fatalf("shortcut %s not found", command)
	return ""
}

// TestIMTipsFirstExampleDryRunAll extends the executability lock from the 3
// path-assertion tests above (messages-send, chat-messages-list,
// resources-download) to every one of the 18 shortcuts carrying a locked
// Example tip: the first example, with placeholders substituted and
// --dry-run appended, must exit 0. This only asserts exit code, not the API
// path — the 3 tests above keep that stronger assertion for their targets.
func TestIMTipsFirstExampleDryRunAll(t *testing.T) {
	for _, cmd := range tipsExampleAllTargets {
		t.Run(cmd, func(t *testing.T) {
			t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
			t.Setenv("LARKSUITE_CLI_APP_ID", "im_tips_dryrun_test")
			t.Setenv("LARKSUITE_CLI_APP_SECRET", "im_tips_dryrun_secret")
			t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			as := defaultAsForCommand(t, cmd)
			args := append(firstExampleArgs(t, cmd), "--dry-run")
			result, err := clie2e.RunCmd(ctx, clie2e.Request{
				Args:      args,
				DefaultAs: as,
				WorkDir:   t.TempDir(),
			})
			require.NoError(t, err)
			result.AssertExitCode(t, 0)
		})
	}
}

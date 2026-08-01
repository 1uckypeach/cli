// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
)

func TestIMFlagAliasesDryRun(t *testing.T) {
	setFlagAliasDryRunEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	tests := []struct {
		name          string
		aliasArgs     []string
		canonicalArgs []string
		defaultAs     string
		notes         []string
	}{
		{
			name: "chat messages",
			aliasArgs: []string{
				"im", "+chat-messages-list", "--chat-id", "oc_dryrun",
				"--start-time", "2026-07-27 00:00:00 +08:00",
				"--end-time", "1785254400",
				"--sort-order", "asc", "--limit", "25", "--no-reactions", "--dry-run",
			},
			canonicalArgs: []string{
				"im", "+chat-messages-list", "--chat-id", "oc_dryrun",
				"--start", "2026-07-27 00:00:00 +08:00",
				"--end", "1785254400",
				"--order", "asc", "--page-size", "25", "--no-reactions", "--dry-run",
			},
			defaultAs: "bot",
			notes: []string{
				"note: --start-time is an alias for --start",
				"note: --end-time is an alias for --end",
				"note: --sort-order is an alias for --order",
				"note: --limit is an alias for --page-size",
			},
		},
		{
			name:          "thread id",
			aliasArgs:     []string{"im", "+threads-messages-list", "--thread-id", "omt_dryrun", "--no-reactions", "--dry-run"},
			canonicalArgs: []string{"im", "+threads-messages-list", "--thread", "omt_dryrun", "--no-reactions", "--dry-run"},
			defaultAs:     "bot",
			notes:         []string{"note: --thread-id is an alias for --thread"},
		},
		{
			name:          "message id",
			aliasArgs:     []string{"im", "+messages-mget", "--message-id", "om_dryrun", "--no-reactions", "--dry-run"},
			canonicalArgs: []string{"im", "+messages-mget", "--message-ids", "om_dryrun", "--no-reactions", "--dry-run"},
			defaultAs:     "bot",
			notes:         []string{"note: --message-id is an alias for --message-ids"},
		},
		{
			name:          "message search",
			aliasArgs:     []string{"im", "+messages-search", "--keyword", "project", "--limit", "30", "--no-reactions", "--dry-run"},
			canonicalArgs: []string{"im", "+messages-search", "--query", "project", "--page-size", "30", "--no-reactions", "--dry-run"},
			defaultAs:     "user",
			notes: []string{
				"note: --keyword is an alias for --query",
				"note: --limit is an alias for --page-size",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aliasResult, err := clie2e.RunCmd(ctx, clie2e.Request{Args: tt.aliasArgs, DefaultAs: tt.defaultAs})
			require.NoError(t, err)
			aliasResult.AssertExitCode(t, 0)

			canonicalResult, err := clie2e.RunCmd(ctx, clie2e.Request{Args: tt.canonicalArgs, DefaultAs: tt.defaultAs})
			require.NoError(t, err)
			canonicalResult.AssertExitCode(t, 0)

			require.JSONEq(t, canonicalResult.Stdout, aliasResult.Stdout)
			require.NotContains(t, aliasResult.Stdout, "is an alias for")
			for _, note := range tt.notes {
				require.Equal(t, 1, strings.Count(aliasResult.Stderr, note), "stderr:\n%s", aliasResult.Stderr)
			}
		})
	}
}

func TestIMFlagAliasesHiddenFromHelp(t *testing.T) {
	setFlagAliasDryRunEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	tests := []struct {
		command string
		aliases []string
	}{
		{"+chat-messages-list", []string{"start-time", "end-time", "sort-order", "limit"}},
		{"+threads-messages-list", []string{"thread-id"}},
		{"+messages-mget", []string{"message-id"}},
		{"+messages-search", []string{"keyword", "limit"}},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result, err := clie2e.RunCmd(ctx, clie2e.Request{Args: []string{"im", tt.command, "--help"}})
			require.NoError(t, err)
			result.AssertExitCode(t, 0)

			for _, alias := range tt.aliases {
				pattern := regexp.MustCompile(`(?m)^\s+--` + regexp.QuoteMeta(alias) + `(?:\s|$)`)
				require.False(t, pattern.MatchString(result.Stdout), "--%s leaked into help:\n%s", alias, result.Stdout)
			}
		})
	}
}

func setFlagAliasDryRunEnv(t *testing.T) {
	t.Helper()
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "alias_dryrun_test")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "alias_dryrun_secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")
	t.Setenv("LARKSUITE_CLI_NO_UPDATE_NOTIFIER", "1")
	t.Setenv("LARKSUITE_CLI_NO_SKILLS_NOTIFIER", "1")
}

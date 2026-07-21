// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"context"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestIM_ChatMembersAddDryRun(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "app")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	tests := []struct {
		name     string
		args     []string
		requests []chatMembersAddDryRunRequest
	}{
		{
			name: "users only",
			args: []string{"--users", "ou_user_b,ou_user_a,ou_user_b"},
			requests: []chatMembersAddDryRunRequest{
				{memberIDType: "open_id", ids: []string{"ou_user_b", "ou_user_a"}},
			},
		},
		{
			name: "bots only",
			args: []string{"--bots", "cli_bot_b,cli_bot_a,cli_bot_b"},
			requests: []chatMembersAddDryRunRequest{
				{memberIDType: "app_id", ids: []string{"cli_bot_b", "cli_bot_a"}},
			},
		},
		{
			name: "users before bots",
			args: []string{
				"--users", "ou_user_b,ou_user_a,ou_user_b",
				"--bots", "cli_bot_b,cli_bot_a,cli_bot_b",
			},
			requests: []chatMembersAddDryRunRequest{
				{memberIDType: "open_id", ids: []string{"ou_user_b", "ou_user_a"}},
				{memberIDType: "app_id", ids: []string{"cli_bot_b", "cli_bot_a"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{
				"im", "+chat-members-add",
				"--chat-id", "oc_e2e_chat",
			}
			args = append(args, tt.args...)
			args = append(args, "--dry-run")

			result, err := clie2e.RunCmd(ctx, clie2e.Request{
				Args:      args,
				DefaultAs: "bot",
			})
			require.NoError(t, err)
			result.AssertExitCode(t, 0)
			result.AssertStdoutStatus(t, true)
			require.True(t, gjson.Get(result.Stdout, "dry_run").Bool(), "expected a dry-run response")

			requests := clie2e.DryRunGet(result.Stdout, "api").Array()
			require.Len(t, requests, len(tt.requests), "unexpected dry-run request count")
			for i, expected := range tt.requests {
				assertChatMembersAddDryRunRequest(t, requests[i], expected)
			}
		})
	}
}

type chatMembersAddDryRunRequest struct {
	memberIDType string
	ids          []string
}

func assertChatMembersAddDryRunRequest(t *testing.T, request gjson.Result, expected chatMembersAddDryRunRequest) {
	t.Helper()

	require.Equal(t, "POST", request.Get("method").String())
	require.Equal(t, "/open-apis/im/v1/chats/oc_e2e_chat/members", request.Get("url").String())
	require.Equal(t, expected.memberIDType, request.Get("params.member_id_type").String())
	require.Equal(t, "1", request.Get("params.succeed_type").String())

	actualIDs := make([]string, 0, len(request.Get("body.id_list").Array()))
	for _, item := range request.Get("body.id_list").Array() {
		actualIDs = append(actualIDs, item.String())
	}
	require.Equal(t, expected.ids, actualIDs)
}

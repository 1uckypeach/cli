// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agents

import (
	"context"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBaseAgentSendDryRun(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "agents_dryrun_test")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "agents_dryrun_secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"agents", "send", "base:assistant",
			"--text", "汇总当前表格",
			"--param", "base_token=basc_dryrun",
			"--param", "active_table_id=tbl_dryrun",
			"--as", "user",
			"--dry-run",
		},
		DefaultAs: "user",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)

	out := result.Stdout
	require.True(t, gjson.Get(out, "data.dry_run").Bool(), out)
	require.Equal(t, "base:assistant", gjson.Get(out, "data.would_send.agent_ref").String(), out)
	require.Equal(t, "汇总当前表格", gjson.Get(out, "data.would_send.text").String(), out)
	require.Equal(t, "basc_dryrun", gjson.Get(out, "data.would_send.params.base_token").String(), out)
	require.Equal(t, "tbl_dryrun", gjson.Get(out, "data.would_send.params.active_table_id").String(), out)
}

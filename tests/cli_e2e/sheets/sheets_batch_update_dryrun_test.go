// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package sheets

import (
	"context"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
)

func TestSheetsBatchUpdateDryRun(t *testing.T) {
	setSheetsDryRunEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{"sheets", "+batch-update", "--spreadsheet-token", "shtDryRun", "--operations",
			`[{"shortcut":"+cells-clear","input":{"sheet_id":"sh1","range":"A1:B2"}}]`, "--continue-on-error", "--dry-run"},
		DefaultAs: "user",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	out := result.Stdout
	require.Equal(t, "POST", clie2e.DryRunGet(out, "api.0.method").String(), out)
	require.Equal(t, "/open-apis/sheet_ai/v2/spreadsheets/shtDryRun/tools/invoke_write", clie2e.DryRunGet(out, "api.0.url").String(), out)
	require.Equal(t, "batch_update", clie2e.DryRunGet(out, "api.0.body.tool_name").String(), out)
	input := clie2e.DryRunGet(out, "api.0.body.input").String()
	require.Contains(t, input, `"continue_on_error":true`)
	require.Contains(t, input, `"tool_name":"clear_cell_range"`)
}

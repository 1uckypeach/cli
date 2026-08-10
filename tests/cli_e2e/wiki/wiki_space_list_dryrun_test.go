// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package wiki

import (
	"context"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
)

func TestWikiSpaceListDryRun(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "wiki_space_list_dryrun_test")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "wiki_space_list_dryrun_secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args:      []string{"wiki", "+space-list", "--page-size", "25", "--page-token", "next_page", "--dry-run"},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	out := result.Stdout
	require.Equal(t, "GET", clie2e.DryRunGet(out, "api.0.method").String(), out)
	require.Equal(t, "/open-apis/wiki/v2/spaces", clie2e.DryRunGet(out, "api.0.url").String(), out)
	require.Equal(t, int64(25), clie2e.DryRunGet(out, "api.0.params.page_size").Int(), out)
	require.Equal(t, "next_page", clie2e.DryRunGet(out, "api.0.params.page_token").String(), out)
}

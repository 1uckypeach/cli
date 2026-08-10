// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package wiki

import (
	"context"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	wikiNodeGetDryRunToken       = "Abcdw_EXAMPLE_WIKI_TOKEN_27"
	wikiNodeGetDryRunObjectToken = "Abcdd_EXAMPLE_DOCX_TOKEN_27"
	wikiNodeGetDryRunLegacyToken = "Abcdl_EXAMPLE_OLD_TOKEN_0027"
)

func TestWikiNodeGetDryRunRejectsTruncatedToken(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "wiki_node_get_dryrun_test")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "wiki_node_get_dryrun_secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args:      []string{"wiki", "+node-get", "--node-token", "PImXw", "--dry-run"},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 2)
	require.Empty(t, result.Stdout)
	require.Equal(t, "validation", gjson.Get(result.Stderr, "error.type").String(), result.Stderr)
	require.Equal(t, "invalid_argument", gjson.Get(result.Stderr, "error.subtype").String(), result.Stderr)
	require.Equal(t, "--node-token", gjson.Get(result.Stderr, "error.param").String(), result.Stderr)
	require.Contains(t, gjson.Get(result.Stderr, "error.hint").String(), "complete token", result.Stderr)
}

func TestWikiNodeGetDryRunMissingTokenPreservesLegacyMessage(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "wiki_node_get_dryrun_test")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "wiki_node_get_dryrun_secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args:      []string{"wiki", "+node-get", "--dry-run"},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 2)
	require.Empty(t, result.Stdout)
	require.Equal(t, "validation", gjson.Get(result.Stderr, "error.type").String(), result.Stderr)
	require.Equal(t, "invalid_argument", gjson.Get(result.Stderr, "error.subtype").String(), result.Stderr)
	require.Equal(t, "--node-token", gjson.Get(result.Stderr, "error.param").String(), result.Stderr)
	require.Equal(t, "--node-token is required", gjson.Get(result.Stderr, "error.message").String(), result.Stderr)
}

func TestWikiNodeGetDryRun(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "wiki_node_get_dryrun_test")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "wiki_node_get_dryrun_secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

	t.Run("raw node token", func(t *testing.T) {
		result := runWikiNodeGetDryRun(t, "--node-token", wikiNodeGetDryRunToken)
		assertWikiNodeGetRequest(t, result, wikiNodeGetDryRunToken, "")
	})

	t.Run("document URL infers object type", func(t *testing.T) {
		result := runWikiNodeGetDryRun(t, "--node-token", "https://feishu.cn/docx/"+wikiNodeGetDryRunObjectToken)
		assertWikiNodeGetRequest(t, result, wikiNodeGetDryRunObjectToken, "docx")
	})

	t.Run("legacy token alias", func(t *testing.T) {
		result := runWikiNodeGetDryRun(t, "--token", wikiNodeGetDryRunLegacyToken)
		assertWikiNodeGetRequest(t, result, wikiNodeGetDryRunLegacyToken, "")
	})
}

func runWikiNodeGetDryRun(t *testing.T, args ...string) *clie2e.Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args:      append([]string{"wiki", "+node-get"}, append(args, "--dry-run")...),
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	return result
}

func assertWikiNodeGetRequest(t *testing.T, result *clie2e.Result, token, objType string) {
	t.Helper()
	assert.Equal(t, "GET", clie2e.DryRunGet(result.Stdout, "api.0.method").String())
	assert.Equal(t, "/open-apis/wiki/v2/spaces/get_node", clie2e.DryRunGet(result.Stdout, "api.0.url").String())
	assert.Equal(t, token, clie2e.DryRunGet(result.Stdout, "api.0.params.token").String())
	gotObjType := clie2e.DryRunGet(result.Stdout, "api.0.params.obj_type")
	if objType == "" {
		assert.False(t, gotObjType.Exists(), "obj_type should be omitted: %s", result.Stdout)
		return
	}
	assert.Equal(t, objType, gotObjType.String())
}

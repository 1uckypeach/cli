// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package docs

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
)

func TestDocsFetchDryRunXMLIncludesCommentsForUserAndBot(t *testing.T) {
	setDocsDryRunEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	type scopeCase struct {
		name string
		args []string
	}
	scopes := []scopeCase{
		{name: "full"},
		{name: "outline", args: []string{"--scope", "outline"}},
		{name: "partial", args: []string{"--scope", "keyword", "--keyword", "commented"}},
	}
	bodies := make(map[string]map[string]map[string]interface{}, len(scopes))
	for _, scope := range scopes {
		bodies[scope.name] = make(map[string]map[string]interface{}, 2)
		for _, identity := range []string{"user", "bot"} {
			t.Run(identity+"/"+scope.name, func(t *testing.T) {
				args := []string{
					"docs", "+fetch",
					"--doc", "doxcnDryRunComments",
					"--doc-format", "xml",
					"--dry-run",
				}
				args = append(args, scope.args...)
				result, err := clie2e.RunCmd(ctx, clie2e.Request{Args: args, DefaultAs: identity})
				require.NoError(t, err)
				result.AssertExitCode(t, 0)

				raw := clie2e.DryRunGet(result.Stdout, "api.0.body.extra_param").String()
				var extra map[string]bool
				require.NoError(t, json.Unmarshal([]byte(raw), &extra))
				require.Equal(t, map[string]bool{
					"enable_user_cite_reference_map": true,
					"include_comments":               true,
					"return_html5_block_data":        true,
				}, extra, "stdout:\n%s", result.Stdout)

				var body map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(clie2e.DryRunGet(result.Stdout, "api.0.body").Raw), &body))
				bodies[scope.name][identity] = body
			})
		}
		require.Equal(t, bodies[scope.name]["user"], bodies[scope.name]["bot"], "user and bot request bodies must match for %s", scope.name)
	}
}

func TestDocsFetchDryRunMarkdownFormatsOmitComments(t *testing.T) {
	setDocsDryRunEnv(t)

	for _, identity := range []string{"user", "bot"} {
		for _, docFormat := range []string{"markdown", "im-markdown"} {
			t.Run(identity+"/"+docFormat, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				t.Cleanup(cancel)
				result, err := clie2e.RunCmd(ctx, clie2e.Request{
					Args: []string{
						"docs", "+fetch",
						"--doc", "doxcnDryRunComments",
						"--doc-format", docFormat,
						"--dry-run",
					},
					DefaultAs: identity,
				})
				require.NoError(t, err)
				result.AssertExitCode(t, 0)
				require.Equal(t, "markdown", clie2e.DryRunGet(result.Stdout, "api.0.body.format").String(), "stdout:\n%s", result.Stdout)

				raw := clie2e.DryRunGet(result.Stdout, "api.0.body.extra_param").String()
				var extra map[string]bool
				require.NoError(t, json.Unmarshal([]byte(raw), &extra))
				require.Equal(t, map[string]bool{
					"enable_user_cite_reference_map": true,
					"return_html5_block_data":        true,
				}, extra, "stdout:\n%s", result.Stdout)
			})
		}
	}
}

func TestDocsFetchCommentsFlagIsRemovedFromHelpAndRejected(t *testing.T) {
	setDocsDryRunEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	help, err := clie2e.RunCmd(ctx, clie2e.Request{Args: []string{"docs", "+fetch", "--help"}, DefaultAs: "bot"})
	require.NoError(t, err)
	help.AssertExitCode(t, 0)
	require.NotContains(t, help.Stdout, "--comments")
	require.NotContains(t, help.Stdout, "docs:document.comment:read")

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args:      []string{"docs", "+fetch", "--doc", "doxcnDryRunComments", "--comments", "--dry-run"},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 2)
	require.Empty(t, result.Stdout)
	require.Contains(t, strings.ToLower(result.Stderr), "unknown flag")
	require.Contains(t, result.Stderr, "--comments")
}

func TestDocsFetchDryRunIgnoresAPIVersionCompatFlag(t *testing.T) {
	setDocsDryRunEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"docs", "+fetch",
			"--doc", "doxcnDryRunCompat",
			"--api-version", "v1",
			"--dry-run",
		},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)

	out := result.Stdout
	if got := clie2e.DryRunGet(out, "api.0.method").String(); got != "POST" {
		t.Fatalf("method=%q, want POST\nstdout:\n%s", got, out)
	}
	if got := clie2e.DryRunGet(out, "api.0.url").String(); got != "/open-apis/docs_ai/v1/documents/doxcnDryRunCompat/fetch" {
		t.Fatalf("url=%q, want docs fetch endpoint\nstdout:\n%s", got, out)
	}
	if got := clie2e.DryRunGet(out, "api.0.body.format").String(); got != "xml" {
		t.Fatalf("format=%q, want xml\nstdout:\n%s", got, out)
	}
}

func TestDocsFetchDryRunSelectionAnchorFragmentBecomesRangeStart(t *testing.T) {
	setDocsDryRunEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"docs", "+fetch",
			"--doc", "https://example.larksuite.com/wiki/wikcnDryRun#share-CUE3d6Ykno2fkexEvt8cGF8Wnse",
			"--dry-run",
		},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)

	out := result.Stdout
	if got := clie2e.DryRunGet(out, "api.0.url").String(); got != "/open-apis/docs_ai/v1/documents/wikcnDryRun/fetch" {
		t.Fatalf("url=%q, want docs fetch endpoint\nstdout:\n%s", got, out)
	}
	if got := clie2e.DryRunGet(out, "api.0.body.read_option.read_mode").String(); got != "range" {
		t.Fatalf("read_mode=%q, want range\nstdout:\n%s", got, out)
	}
	if got := clie2e.DryRunGet(out, "api.0.body.read_option.start_block_id").String(); got != "share-CUE3d6Ykno2fkexEvt8cGF8Wnse" {
		t.Fatalf("start_block_id=%q, want selection anchor\nstdout:\n%s", got, out)
	}
}

func TestDocsFetchDryRunUnsupportedSelectionAnchorFragmentStaysFull(t *testing.T) {
	setDocsDryRunEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"docs", "+fetch",
			"--doc", "https://example.larksuite.com/wiki/wikcnDryRun#part-CUE3d6Ykno2fkexEvt8cGF8Wnse",
			"--dry-run",
		},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)

	out := result.Stdout
	if got := clie2e.DryRunGet(out, "api.0.body.read_option").Raw; got != "" {
		t.Fatalf("read_option=%s, want omitted for unsupported selection anchor\nstdout:\n%s", got, out)
	}
}

func setDocsDryRunEnv(t *testing.T) {
	t.Helper()
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "docs_dryrun_test")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "docs_dryrun_secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")
}

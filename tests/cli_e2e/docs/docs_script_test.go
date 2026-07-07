// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package docs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestDocsScriptParseXMLFromFile(t *testing.T) {
	workDir := t.TempDir()
	input := `<title>标题</title><p>一个苹果是 an apple。</p>`
	if err := os.WriteFile(filepath.Join(workDir, "draft.xml"), []byte(input), 0o600); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"docs", "+script",
			"--command", "parse",
			"--content", "@draft.xml",
		},
		DefaultAs: "bot",
		WorkDir:   workDir,
		Env:       docsScriptE2EEnv(),
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	result.AssertStdoutStatus(t, true)
	require.Equal(t, int64(10), gjson.Get(result.Stdout, "data.profile.word_count").Int())
	require.Equal(t, int64(15), gjson.Get(result.Stdout, "data.profile.char_count").Int())
	require.Equal(t, int64(2), gjson.Get(result.Stdout, "data.profile.block_count").Int())
	require.False(t, gjson.Get(result.Stdout, "data.xml").Exists())
	require.False(t, gjson.Get(result.Stdout, "data.input_format").Exists())
	require.False(t, gjson.Get(result.Stdout, "data.command").Exists())
	require.False(t, gjson.Get(result.Stdout, "data.profile.breakdown").Exists())
	require.False(t, gjson.Get(result.Stdout, "data.profile.compatibility").Exists())
}

func TestDocsScriptParseMarkdownFromFile(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "draft.md"), []byte("# 标题\n\n- item"), 0o600); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"docs", "+script",
			"--command", "parse",
			"--content", "@draft.md",
		},
		DefaultAs: "bot",
		WorkDir:   workDir,
		Env:       docsScriptE2EEnv(),
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	result.AssertStdoutStatus(t, true)
	require.Equal(t, int64(3), gjson.Get(result.Stdout, "data.profile.block_count").Int())
	require.False(t, gjson.Get(result.Stdout, "data.xml").Exists())
	require.False(t, gjson.Get(result.Stdout, "data.profile.breakdown").Exists())
}

func TestDocsScriptConvertsMarkdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"docs", "+script",
			"--command", "markdown-to-xml",
			"--content", "# 标题\n\n- item",
		},
		DefaultAs: "bot",
		Env:       docsScriptE2EEnv(),
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	result.AssertStdoutStatus(t, true)
	require.Equal(t, `<h1>标题</h1><ul><li>item</li></ul>`, gjson.Get(result.Stdout, "data.xml").String())
	require.False(t, gjson.Get(result.Stdout, "data.profile").Exists())
	require.False(t, gjson.Get(result.Stdout, "data.input_format").Exists())
	require.False(t, gjson.Get(result.Stdout, "data.command").Exists())
}

func TestDocsScriptDryRunIsLocal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"docs", "+script",
			"--command", "parse",
			"--content", `<p>text</p>`,
			"--dry-run",
		},
		DefaultAs: "bot",
		Env:       docsScriptE2EEnv(),
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	require.Equal(t, int64(0), gjson.Get(result.Stdout, "api.#").Int())
	require.False(t, gjson.Get(result.Stdout, "network").Bool())
	require.Equal(t, "parse", gjson.Get(result.Stdout, "command").String())
}

func docsScriptE2EEnv() map[string]string {
	return map[string]string{
		"LARKSUITE_CLI_APP_ID":     "docs-script-e2e",
		"LARKSUITE_CLI_APP_SECRET": "secret",
		"LARKSUITE_CLI_BRAND":      "feishu",
	}
}

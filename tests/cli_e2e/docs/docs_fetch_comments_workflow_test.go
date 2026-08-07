// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package docs

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/larksuite/cli/tests/cli_e2e/drive"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestDocsFetchCommentsWorkflow creates its own document and comments so the
// opt-in fetch contract can be exercised without a long-lived shared fixture.
// It is gated because the live credential needs both comment read and write
// scopes, which are intentionally absent from the default test app.
func TestDocsFetchCommentsWorkflow(t *testing.T) {
	clie2e.SkipWithoutTenantAccessToken(t)
	if os.Getenv("LARK_DOCS_FETCH_COMMENTS_E2E") == "" {
		t.Skip("set LARK_DOCS_FETCH_COMMENTS_E2E=1 to run the document comment fetch workflow")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)
	parentT := t
	const defaultAs = "bot"
	suffix := clie2e.GenerateSuffix()
	anchorText := "comment anchor " + suffix
	localText := "local review " + suffix
	wholeText := "whole document review " + suffix

	folderToken := drive.CreateDriveFolder(t, parentT, ctx, "lark-cli-e2e-fetch-comments-"+suffix, defaultAs, "")
	docToken := createDocWithRetry(t, parentT, ctx, folderToken, "fetch comments "+suffix, anchorText+"\n\nsecondary block", defaultAs)

	addDocComment(t, ctx, defaultAs, docToken, localText, "--selection-with-ellipsis", anchorText)
	addDocComment(t, ctx, defaultAs, docToken, wholeText, "--full-comment")

	var fetched *clie2e.Result
	t.Run("xml full", func(t *testing.T) {
		var err error
		fetched, err = clie2e.RunCmdWithRetry(ctx, clie2e.Request{
			Args:      []string{"docs", "+fetch", "--doc", docToken, "--comments", "--doc-format", "xml"},
			DefaultAs: defaultAs,
		}, clie2e.RetryOptions{ShouldRetry: func(result *clie2e.Result) bool {
			return result == nil || result.ExitCode != 0 ||
				!strings.Contains(gjson.Get(result.Stdout, "data.document.content").String(), "comment-refs=") ||
				!docsFetchReferenceGroupContains(result.Stdout, "comment", localText) ||
				!docsFetchReferenceGroupContains(result.Stdout, "document-comment", wholeText)
		}})
		require.NoError(t, err)
		fetched.AssertExitCode(t, 0)
		fetched.AssertStdoutStatus(t, true)
		if !strings.Contains(gjson.Get(fetched.Stdout, "data.document.content").String(), "comment-refs=") {
			t.Fatalf("local comment marker missing:\n%s", fetched.Stdout)
		}
		if !docsFetchReferenceGroupContains(fetched.Stdout, "comment", localText) ||
			!docsFetchReferenceGroupContains(fetched.Stdout, "document-comment", wholeText) {
			t.Fatalf("comment reference groups missing:\n%s", fetched.Stdout)
		}
	})

	t.Run("default remains comment free", func(t *testing.T) {
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args:      []string{"docs", "+fetch", "--doc", docToken, "--doc-format", "xml"},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		content := gjson.Get(result.Stdout, "data.document.content").String()
		if strings.Contains(content, "comment-refs=") || docsFetchReferenceGroupExists(result.Stdout, "comment") || docsFetchReferenceGroupExists(result.Stdout, "document-comment") {
			t.Fatalf("comments must remain opt-in:\n%s", result.Stdout)
		}
	})

	t.Run("partial returns only intersecting local discussion", func(t *testing.T) {
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args: []string{
				"docs", "+fetch", "--doc", docToken, "--comments", "--doc-format", "xml",
				"--scope", "keyword", "--keyword", anchorText,
			},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		if !docsFetchReferenceGroupContains(result.Stdout, "comment", localText) || docsFetchReferenceGroupExists(result.Stdout, "document-comment") {
			t.Fatalf("partial comment filtering mismatch:\n%s", result.Stdout)
		}
	})

	t.Run("markdown uses precise reference shells", func(t *testing.T) {
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args:      []string{"docs", "+fetch", "--doc", docToken, "--comments", "--doc-format", "markdown"},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		content := gjson.Get(result.Stdout, "data.document.content").String()
		if strings.Contains(content, "comment-refs") || !strings.Contains(content, `<comment-ref refs="`) ||
			!docsFetchReferenceGroupContains(result.Stdout, "comment", localText) ||
			!docsFetchReferenceGroupContains(result.Stdout, "document-comment", wholeText) {
			t.Fatalf("markdown comment sidecar mismatch:\n%s", result.Stdout)
		}
	})
}

func addDocComment(t *testing.T, ctx context.Context, defaultAs, docToken, text string, locationArgs ...string) {
	t.Helper()
	content, err := json.Marshal([]map[string]string{{"type": "text", "text": text}})
	require.NoError(t, err)
	args := []string{
		"drive", "+add-comment",
		"--doc", docToken,
		"--type", "docx",
		"--content", string(content),
	}
	args = append(args, locationArgs...)
	result, err := clie2e.RunCmdWithRetry(ctx, clie2e.Request{Args: args, DefaultAs: defaultAs}, clie2e.RetryOptions{})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	result.AssertStdoutStatus(t, true)
}

func docsFetchReferenceGroupExists(stdout, group string) bool {
	return gjson.Get(stdout, "data.document.reference_map").Get(group).Exists()
}

func docsFetchReferenceGroupContains(stdout, group, text string) bool {
	for _, entry := range gjson.Get(stdout, "data.document.reference_map").Get(group).Map() {
		if strings.Contains(entry.Get("data").String(), text) {
			return true
		}
	}
	return false
}

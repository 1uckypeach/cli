// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package docs

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"regexp"
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

	// Both creation helpers register parentT cleanup immediately. Cleanup is
	// LIFO, so the document is deleted before its containing folder.
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
				!docsFetchReferenceGroupContains(result.Stdout, "comments", localText) ||
				!docsFetchReferenceGroupContains(result.Stdout, "comments", wholeText)
		}})
		require.NoError(t, err)
		fetched.AssertExitCode(t, 0)
		fetched.AssertStdoutStatus(t, true)
		if !strings.Contains(gjson.Get(fetched.Stdout, "data.document.content").String(), "comment-refs=") {
			t.Fatalf("local comment marker missing:\n%s", fetched.Stdout)
		}
		if !docsFetchReferenceGroupContains(fetched.Stdout, "comments", localText) ||
			!docsFetchReferenceGroupContains(fetched.Stdout, "comments", wholeText) {
			t.Fatalf("comment reference groups missing:\n%s", fetched.Stdout)
		}
		assertDocsFetchCommentContract(t, fetched.Stdout, localText, wholeText)
	})

	t.Run("default remains comment free", func(t *testing.T) {
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args:      []string{"docs", "+fetch", "--doc", docToken, "--doc-format", "xml"},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		content := gjson.Get(result.Stdout, "data.document.content").String()
		if strings.Contains(content, "comment-refs=") || strings.Contains(content, "comment-ids=") || docsFetchReferenceGroupExists(result.Stdout, "comments") {
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
		if !docsFetchReferenceGroupContains(result.Stdout, "comments", localText) || docsFetchReferenceGroupContains(result.Stdout, "comments", wholeText) {
			t.Fatalf("partial comment filtering mismatch:\n%s", result.Stdout)
		}
		assertDocsFetchCommentContract(t, result.Stdout, localText, "")
	})
}

type docsFetchCommentEnvelope struct {
	Data struct {
		Document struct {
			Content      string                                        `json:"content"`
			ReferenceMap map[string]map[string]docsFetchReferenceEntry `json:"reference_map"`
		} `json:"document"`
	} `json:"data"`
}

type docsFetchReferenceEntry struct {
	Data string `json:"data"`
}

func assertDocsFetchCommentContract(t *testing.T, stdout, localText, wholeText string) {
	t.Helper()

	var envelope docsFetchCommentEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &envelope))
	document := envelope.Data.Document
	entries := document.ReferenceMap["comments"]
	wantEntries := 1
	if wholeText != "" {
		wantEntries = 2
	}
	require.Len(t, entries, wantEntries, "comments sidecar must contain only the fixture discussions relevant to this fetch")
	require.False(t, docsFetchReferenceGroupExists(stdout, "comment"), "legacy local-comment group must be absent")
	require.False(t, docsFetchReferenceGroupExists(stdout, "document-comment"), "legacy whole-comment group must be absent")

	localKeys := make(map[string]struct{}, 1)
	wholeKeys := make(map[string]struct{}, 1)
	for key, entry := range entries {
		if !docsFetchCommentRefPattern.MatchString(key) {
			t.Fatalf("comment key %q is not an opaque cN surrogate", key)
		}
		shape := assertDiscussionXML(t, entry.Data)
		switch {
		case strings.Contains(entry.Data, localText):
			require.True(t, shape.hasQuote, "local discussion must include its quoted anchor")
			localKeys[key] = struct{}{}
		case wholeText != "" && strings.Contains(entry.Data, wholeText):
			require.False(t, shape.hasQuote, "whole-document discussion must omit quote")
			wholeKeys[key] = struct{}{}
		default:
			t.Fatalf("unexpected discussion %q in comments sidecar: %s", key, entry.Data)
		}
	}
	require.Len(t, localKeys, 1, "fixture creates exactly one local comment")
	if wholeText == "" {
		require.Empty(t, wholeKeys, "partial fetch must omit whole-document comments")
	} else {
		require.Len(t, wholeKeys, 1, "fixture creates exactly one whole-document comment")
	}

	if strings.Contains(document.Content, "comment-ids=") {
		t.Fatal("raw Engine comment IDs must never reach the public document content")
	}
	if strings.Contains(document.Content, "<comment-ref") {
		t.Fatal("DocxXML output must use comment-refs attributes, not Markdown shells")
	}
	bodyRefs := collectCommentRefsFromXML(t, document.Content)
	require.Equal(t, localKeys, bodyRefs, "body refs and local sidecar keys must form an exact closure")
	for ref := range wholeKeys {
		if _, exists := bodyRefs[ref]; exists {
			t.Fatalf("whole-document discussion %q must not be attached to a body block", ref)
		}
	}
}

var (
	docsFetchCommentRefPattern = regexp.MustCompile(`^c[1-9][0-9]*$`)
	docsFetchCommentIDPattern  = regexp.MustCompile(`^[0-9]+$`)
)

func collectCommentRefsFromXML(t *testing.T, fragment string) map[string]struct{} {
	t.Helper()

	refs := make(map[string]struct{})
	decoder := xml.NewDecoder(strings.NewReader("<root>" + fragment + "</root>"))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "comment marker XML must be well-formed")
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local != "comment-refs" {
				continue
			}
			for _, ref := range strings.Fields(attr.Value) {
				if !docsFetchCommentRefPattern.MatchString(ref) {
					t.Fatalf("body comment ref %q is not an opaque cN surrogate", ref)
				}
				refs[ref] = struct{}{}
			}
		}
	}
	return refs
}

type docsFetchDiscussionShape struct {
	hasQuote bool
}

func assertDiscussionXML(t *testing.T, data string) docsFetchDiscussionShape {
	t.Helper()

	allowedAttrs := map[string]map[string]bool{
		"discussion": {"timezone": true, "comment-id": true},
		"quote":      {},
		"message":    {"time": true, "user": true},
		"img":        {},
		"reaction":   {},
	}
	decoder := xml.NewDecoder(strings.NewReader(data))
	stack := make([]string, 0, 3)
	sawRoot := false
	sawQuote := false
	messageCount := 0
	timezone := ""
	commentID := ""
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "discussion sidecar must be well-formed XML")
		switch typed := token.(type) {
		case xml.StartElement:
			parent := ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			switch typed.Name.Local {
			case "discussion":
				require.Empty(t, parent, "discussion must be the single root")
				require.False(t, sawRoot, "discussion sidecar must have one root")
				sawRoot = true
			case "quote", "message":
				require.Equal(t, "discussion", parent, "%s must be a direct discussion child", typed.Name.Local)
			case "img", "reaction":
				require.Equal(t, "message", parent, "%s must belong to one message", typed.Name.Local)
			default:
				t.Fatalf("unexpected discussion element <%s>", typed.Name.Local)
			}
			for _, attr := range typed.Attr {
				if !allowedAttrs[typed.Name.Local][attr.Name.Local] {
					t.Fatalf("unexpected %s attribute %q", typed.Name.Local, attr.Name.Local)
				}
				if typed.Name.Local == "discussion" && attr.Name.Local == "timezone" {
					timezone = attr.Value
				}
				if typed.Name.Local == "discussion" && attr.Name.Local == "comment-id" {
					commentID = attr.Value
				}
				if typed.Name.Local == "message" && (attr.Name.Local == "time" || attr.Name.Local == "user") {
					require.NotEmpty(t, attr.Value, "message %s must be non-empty", attr.Name.Local)
				}
			}
			if typed.Name.Local == "quote" {
				sawQuote = true
			}
			if typed.Name.Local == "message" {
				messageCount++
			}
			stack = append(stack, typed.Name.Local)
		case xml.EndElement:
			require.NotEmpty(t, stack, "unexpected closing element </%s>", typed.Name.Local)
			require.Equal(t, stack[len(stack)-1], typed.Name.Local, "discussion elements must close in order")
			stack = stack[:len(stack)-1]
		}
	}
	require.True(t, sawRoot)
	require.Empty(t, stack)
	require.Equal(t, "Asia/Shanghai", timezone)
	require.True(t, docsFetchCommentIDPattern.MatchString(commentID), "discussion must expose the first visible numeric comment-id")
	require.Positive(t, messageCount, "discussion must contain at least one message")
	return docsFetchDiscussionShape{hasQuote: sawQuote}
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

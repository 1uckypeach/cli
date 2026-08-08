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
		assertDocsFetchCommentContract(t, fetched.Stdout, "xml", localText, wholeText)
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
		assertDocsFetchCommentContract(t, result.Stdout, "xml", localText, "")
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
		assertDocsFetchCommentContract(t, result.Stdout, "markdown", localText, wholeText)
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

var docsFetchCommentShellPattern = regexp.MustCompile(`<comment-ref\b[^>]*?/>`)

func assertDocsFetchCommentContract(t *testing.T, stdout, markerMode, localText, wholeText string) {
	t.Helper()

	var envelope docsFetchCommentEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &envelope))
	document := envelope.Data.Document
	localEntries := document.ReferenceMap["comment"]
	documentEntries := document.ReferenceMap["document-comment"]
	require.Len(t, localEntries, 1, "fixture creates exactly one local comment")
	if wholeText == "" {
		require.Empty(t, documentEntries, "partial fetch must omit whole-document comments")
	} else {
		require.Len(t, documentEntries, 1, "fixture creates exactly one whole-document comment")
	}

	localKeys := make(map[string]struct{}, len(localEntries))
	for key, entry := range localEntries {
		if !regexp.MustCompile(`^c[1-9][0-9]*$`).MatchString(key) {
			t.Fatalf("local comment key %q is not an opaque cN surrogate", key)
		}
		localKeys[key] = struct{}{}
		assertDiscussionXML(t, entry.Data, false)
	}
	if !docsFetchReferenceGroupContains(stdout, "comment", localText) {
		t.Fatalf("local discussion does not contain %q", localText)
	}
	for key, entry := range documentEntries {
		if !regexp.MustCompile(`^d[1-9][0-9]*$`).MatchString(key) {
			t.Fatalf("document comment key %q is not an opaque dN surrogate", key)
		}
		assertDiscussionXML(t, entry.Data, true)
	}
	if wholeText != "" && !docsFetchReferenceGroupContains(stdout, "document-comment", wholeText) {
		t.Fatalf("whole-document discussion does not contain %q", wholeText)
	}

	var bodyRefs map[string]struct{}
	switch markerMode {
	case "xml":
		bodyRefs = collectCommentRefsFromXML(t, document.Content, true)
		if strings.Contains(document.Content, "<comment-ref") {
			t.Fatal("DocxXML output must use comment-refs attributes, not Markdown shells")
		}
	case "markdown":
		if strings.Contains(document.Content, "comment-refs=") {
			t.Fatal("Markdown output must not contain DocxXML comment-refs attributes")
		}
		shells := docsFetchCommentShellPattern.FindAllString(document.Content, -1)
		bodyRefs = collectCommentRefsFromXML(t, strings.Join(shells, ""), false)
	default:
		t.Fatalf("unsupported marker mode %q", markerMode)
	}
	require.Equal(t, localKeys, bodyRefs, "body refs and local sidecar keys must form an exact closure")
}

func collectCommentRefsFromXML(t *testing.T, fragment string, includeBlockAttrs bool) map[string]struct{} {
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
			isRefAttr := start.Name.Local == "comment-ref" && attr.Name.Local == "refs"
			isRefAttr = isRefAttr || (includeBlockAttrs && attr.Name.Local == "comment-refs")
			if !isRefAttr {
				continue
			}
			for _, ref := range strings.Fields(attr.Value) {
				refs[ref] = struct{}{}
			}
		}
	}
	return refs
}

func assertDiscussionXML(t *testing.T, data string, documentScope bool) {
	t.Helper()

	allowedChildren := map[string]bool{"quote": true, "message": true, "img": true, "reaction": true}
	allowedAttrs := map[string]map[string]bool{
		"discussion": {"scope": true, "timezone": true},
		"quote":      {},
		"message":    {"t": true, "u": true},
		"img":        {},
		"reaction":   {},
	}
	decoder := xml.NewDecoder(strings.NewReader(data))
	depth := 0
	sawRoot := false
	sawQuote := false
	scope := ""
	timezone := ""
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "discussion sidecar must be well-formed XML")
		switch typed := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				require.False(t, sawRoot, "discussion sidecar must have one root")
				require.Equal(t, "discussion", typed.Name.Local)
				sawRoot = true
			} else if !allowedChildren[typed.Name.Local] {
				t.Fatalf("unexpected discussion element <%s>", typed.Name.Local)
			}
			for _, attr := range typed.Attr {
				if !allowedAttrs[typed.Name.Local][attr.Name.Local] {
					t.Fatalf("unexpected %s attribute %q", typed.Name.Local, attr.Name.Local)
				}
				if typed.Name.Local == "discussion" && attr.Name.Local == "scope" {
					scope = attr.Value
				}
				if typed.Name.Local == "discussion" && attr.Name.Local == "timezone" {
					timezone = attr.Value
				}
			}
			if typed.Name.Local == "quote" {
				sawQuote = true
			}
			depth++
		case xml.EndElement:
			depth--
			require.GreaterOrEqual(t, depth, 0)
		}
	}
	require.True(t, sawRoot)
	require.Zero(t, depth)
	require.Equal(t, "Asia/Shanghai", timezone)
	if documentScope {
		require.Equal(t, "document", scope)
		require.False(t, sawQuote, "whole-document discussions must not repeat a quote")
	} else {
		require.Empty(t, scope)
	}
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

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

// By default the live workflows create their own documents and comments. A
// read-only document fixture can be supplied when the environment does not
// permit comment creation (for example, a BOE document with a large corpus of
// real comments).
func TestDocsFetchCommentsWorkflowAsBot(t *testing.T) {
	clie2e.SkipWithoutTenantAccessToken(t)
	testDocsFetchCommentsWorkflow(t, "bot")
}

func TestDocsFetchCommentsWorkflowAsUser(t *testing.T) {
	clie2e.SkipWithoutUserToken(t)
	testDocsFetchCommentsWorkflow(t, "user")
}

func testDocsFetchCommentsWorkflow(t *testing.T, defaultAs string) {
	if os.Getenv("LARK_DOCS_FETCH_COMMENTS_E2E") == "" {
		t.Skip("set LARK_DOCS_FETCH_COMMENTS_E2E=1 to run the document comment fetch workflow")
	}
	if docToken := strings.TrimSpace(os.Getenv("LARK_DOCS_FETCH_COMMENTS_E2E_DOC")); docToken != "" {
		testDocsFetchCommentsReadOnlyFixture(t, defaultAs, docToken)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)
	parentT := t
	suffix := clie2e.GenerateSuffix()
	anchorText := "comment anchor " + suffix
	secondaryText := "secondary block " + suffix
	localText := "local review " + suffix
	secondaryLocalText := "secondary review " + suffix
	wholeText := "whole document review " + suffix

	// Both creation helpers register parentT cleanup immediately. Cleanup is
	// LIFO, so the document is deleted before its containing folder.
	folderToken := drive.CreateDriveFolder(t, parentT, ctx, "lark-cli-e2e-fetch-comments-"+suffix, defaultAs, "")
	docToken := createDocWithRetry(t, parentT, ctx, folderToken, "fetch comments "+suffix, anchorText+"\n\n"+secondaryText, defaultAs)

	localCommentID := addDocComment(t, ctx, defaultAs, docToken, localText, "--selection-with-ellipsis", anchorText)
	secondaryCommentID := addDocComment(t, ctx, defaultAs, docToken, secondaryLocalText, "--selection-with-ellipsis", secondaryText)
	wholeCommentID := addDocComment(t, ctx, defaultAs, docToken, wholeText, "--full-comment")

	var fetched *clie2e.Result
	t.Run("xml full", func(t *testing.T) {
		var err error
		fetched, err = clie2e.RunCmdWithRetry(ctx, clie2e.Request{
			Args:      []string{"docs", "+fetch", "--doc", docToken, "--doc-format", "xml"},
			DefaultAs: defaultAs,
		}, clie2e.RetryOptions{ShouldRetry: func(result *clie2e.Result) bool {
			return result == nil || result.ExitCode != 0 ||
				!strings.Contains(gjson.Get(result.Stdout, "data.document.content").String(), "comment-refs=") ||
				!docsFetchReferenceGroupContains(result.Stdout, "comments", localText) ||
				!docsFetchReferenceGroupContains(result.Stdout, "comments", secondaryLocalText) ||
				!docsFetchReferenceGroupContains(result.Stdout, "comments", wholeText)
		}})
		require.NoError(t, err)
		fetched.AssertExitCode(t, 0)
		fetched.AssertStdoutStatus(t, true)
		if !strings.Contains(gjson.Get(fetched.Stdout, "data.document.content").String(), "comment-refs=") {
			t.Fatalf("local comment marker missing:\n%s", fetched.Stdout)
		}
		if !docsFetchReferenceGroupContains(fetched.Stdout, "comments", localText) ||
			!docsFetchReferenceGroupContains(fetched.Stdout, "comments", secondaryLocalText) ||
			!docsFetchReferenceGroupContains(fetched.Stdout, "comments", wholeText) {
			t.Fatalf("comment reference groups missing:\n%s", fetched.Stdout)
		}
		assertDocsFetchCommentContract(t, fetched.Stdout, []docsFetchExpectedComment{
			{text: localText, commentID: localCommentID, local: true},
			{text: secondaryLocalText, commentID: secondaryCommentID, local: true},
			{text: wholeText, commentID: wholeCommentID},
		})
	})

	t.Run("Markdown protocols do not expose XML comment sidecars", func(t *testing.T) {
		for _, docFormat := range []string{"markdown", "im-markdown"} {
			t.Run(docFormat, func(t *testing.T) {
				result, err := clie2e.RunCmd(ctx, clie2e.Request{
					Args:      []string{"docs", "+fetch", "--doc", docToken, "--doc-format", docFormat},
					DefaultAs: defaultAs,
				})
				require.NoError(t, err)
				result.AssertExitCode(t, 0)
				content := gjson.Get(result.Stdout, "data.document.content").String()
				if strings.Contains(content, "comment-refs=") || docsFetchReferenceGroupExists(result.Stdout, "comments") {
					t.Fatalf("%s fetch must not carry XML comment protocol:\n%s", docFormat, result.Stdout)
				}
			})
		}
	})

	t.Run("partial returns only intersecting local comment", func(t *testing.T) {
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args: []string{
				"docs", "+fetch", "--doc", docToken, "--doc-format", "xml",
				"--scope", "keyword", "--keyword", anchorText,
			},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		if !docsFetchReferenceGroupContains(result.Stdout, "comments", localText) ||
			docsFetchReferenceGroupContains(result.Stdout, "comments", secondaryLocalText) ||
			docsFetchReferenceGroupContains(result.Stdout, "comments", wholeText) {
			t.Fatalf("partial comment filtering mismatch:\n%s", result.Stdout)
		}
		assertDocsFetchCommentContract(t, result.Stdout, []docsFetchExpectedComment{
			{text: localText, commentID: localCommentID, local: true},
		})
	})

	t.Run("pretty remains body only", func(t *testing.T) {
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args:      []string{"docs", "+fetch", "--doc", docToken, "--doc-format", "xml", "--format", "pretty"},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		require.Contains(t, result.Stdout, anchorText)
		require.NotContains(t, result.Stdout, `"reference_map"`)
		require.NotContains(t, result.Stdout, localText)
		require.NotContains(t, result.Stdout, wholeText)
	})
}

func testDocsFetchCommentsReadOnlyFixture(t *testing.T, defaultAs, docToken string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)

	var fetched *clie2e.Result
	t.Run("xml full read only", func(t *testing.T) {
		var err error
		fetched, err = clie2e.RunCmdWithRetry(ctx, clie2e.Request{
			Args:      []string{"docs", "+fetch", "--doc", docToken, "--doc-format", "xml"},
			DefaultAs: defaultAs,
		}, clie2e.RetryOptions{ShouldRetry: func(result *clie2e.Result) bool {
			return result == nil || result.ExitCode != 0 ||
				!docsFetchReferenceGroupExists(result.Stdout, "comments")
		}})
		require.NoError(t, err)
		fetched.AssertExitCode(t, 0)
		fetched.AssertStdoutStatus(t, true)
		summary := assertDocsFetchReadOnlyContract(t, fetched.Stdout, true)
		require.Positive(t, summary.localCount, "the shared fixture must contain local comments")
		require.Positive(t, summary.wholeCount, "the shared fixture must contain whole-document comments")
	})

	t.Run("Markdown protocols do not expose XML comment sidecars", func(t *testing.T) {
		for _, docFormat := range []string{"markdown", "im-markdown"} {
			t.Run(docFormat, func(t *testing.T) {
				result, err := clie2e.RunCmd(ctx, clie2e.Request{
					Args:      []string{"docs", "+fetch", "--doc", docToken, "--doc-format", docFormat},
					DefaultAs: defaultAs,
				})
				require.NoError(t, err)
				result.AssertExitCode(t, 0)
				result.AssertStdoutStatus(t, true)
				content := gjson.Get(result.Stdout, "data.document.content").String()
				if strings.Contains(content, "comment-refs=") || docsFetchReferenceGroupExists(result.Stdout, "comments") {
					t.Fatalf("%s fetch must not carry XML comment protocol:\n%s", docFormat, result.Stdout)
				}
			})
		}
	})

	t.Run("partial returns only intersecting local comments", func(t *testing.T) {
		require.NotNil(t, fetched)
		keyword := firstLocalCommentQuote(t, fetched.Stdout)
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args: []string{
				"docs", "+fetch", "--doc", docToken, "--doc-format", "xml",
				"--scope", "keyword", "--keyword", keyword,
			},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		result.AssertStdoutStatus(t, true)
		summary := assertDocsFetchReadOnlyContract(t, result.Stdout, false)
		require.Positive(t, summary.localCount, "keyword fetch must retain at least one intersecting local comment")
		require.Zero(t, summary.wholeCount, "keyword fetch must omit whole-document comments")
	})

	t.Run("outline remains comment free", func(t *testing.T) {
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args:      []string{"docs", "+fetch", "--doc", docToken, "--doc-format", "xml", "--scope", "outline"},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		result.AssertStdoutStatus(t, true)
		require.False(t, docsFetchReferenceGroupExists(result.Stdout, "comments"))
		require.NotContains(t, gjson.Get(result.Stdout, "data.document.content").String(), "comment-refs=")
	})

	t.Run("pretty remains body only", func(t *testing.T) {
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args:      []string{"docs", "+fetch", "--doc", docToken, "--doc-format", "xml", "--format", "pretty"},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		require.NotEmpty(t, strings.TrimSpace(result.Stdout))
		require.NotContains(t, result.Stdout, `"reference_map"`)
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

type docsFetchExpectedComment struct {
	text      string
	commentID string
	local     bool
}

type docsFetchReadOnlySummary struct {
	localCount int
	wholeCount int
}

func assertDocsFetchReadOnlyContract(t *testing.T, stdout string, allowWhole bool) docsFetchReadOnlySummary {
	t.Helper()

	var envelope docsFetchCommentEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &envelope))
	document := envelope.Data.Document
	entries := document.ReferenceMap["comments"]
	require.NotEmpty(t, entries, "comments sidecar must not be empty")
	require.False(t, docsFetchReferenceGroupExists(stdout, "comment"), "legacy local-comment group must be absent")
	require.False(t, docsFetchReferenceGroupExists(stdout, "document-comment"), "legacy whole-comment group must be absent")

	localKeys := make(map[string]struct{})
	wholeKeys := make(map[string]struct{})
	for key, entry := range entries {
		if key == "tips" {
			require.Equal(t, "Comments are truncated. Use the comment API to fetch complete content.", entry.Data)
			continue
		}
		if !docsFetchCommentRefPattern.MatchString(key) {
			t.Fatalf("comment key %q is not an opaque cN surrogate", key)
		}
		shape := assertCommentXML(t, entry.Data)
		require.NotContains(t, entry.Data, "A-1(", "reaction users must expose compact names, never name(display)")
		if shape.isWhole {
			require.True(t, allowWhole, "partial fetch must not return whole-document comments")
			require.False(t, shape.hasQuote, "whole-document comments must not carry a quote")
			wholeKeys[key] = struct{}{}
			continue
		}
		require.True(t, shape.hasQuote, "local comments must carry a quote")
		localKeys[key] = struct{}{}
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
			t.Fatalf("whole-document comment %q must not be attached to a body block", ref)
		}
	}
	return docsFetchReadOnlySummary{localCount: len(localKeys), wholeCount: len(wholeKeys)}
}

func firstLocalCommentQuote(t *testing.T, stdout string) string {
	t.Helper()

	var envelope docsFetchCommentEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &envelope))
	for key, entry := range envelope.Data.Document.ReferenceMap["comments"] {
		if key == "tips" {
			continue
		}
		decoder := xml.NewDecoder(strings.NewReader(entry.Data))
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			require.NoError(t, err, "comment sidecar must be well-formed XML")
			start, ok := token.(xml.StartElement)
			if !ok || start.Name.Local != "quote" {
				continue
			}
			var quote string
			require.NoError(t, decoder.DecodeElement(&quote, &start))
			quote = strings.TrimSpace(quote)
			if quote == "" {
				break
			}
			runes := []rune(quote)
			if len(runes) > 80 {
				quote = string(runes[:80])
			}
			return quote
		}
	}
	t.Fatal("the shared fixture must contain a non-empty local comment quote")
	return ""
}

func assertDocsFetchCommentContract(t *testing.T, stdout string, expected []docsFetchExpectedComment) {
	t.Helper()

	var envelope docsFetchCommentEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &envelope))
	document := envelope.Data.Document
	entries := document.ReferenceMap["comments"]
	require.Len(t, entries, len(expected), "comments sidecar must contain only the fixture comments relevant to this fetch")
	require.False(t, docsFetchReferenceGroupExists(stdout, "comment"), "legacy local-comment group must be absent")
	require.False(t, docsFetchReferenceGroupExists(stdout, "document-comment"), "legacy whole-comment group must be absent")

	localKeys := make(map[string]struct{}, len(expected))
	wholeKeys := make(map[string]struct{}, len(expected))
	found := make(map[string]struct{}, len(expected))
	for key, entry := range entries {
		if !docsFetchCommentRefPattern.MatchString(key) {
			t.Fatalf("comment key %q is not an opaque cN surrogate", key)
		}
		shape := assertCommentXML(t, entry.Data)
		matched := false
		for _, want := range expected {
			if !strings.Contains(entry.Data, want.text) {
				continue
			}
			require.Equal(t, want.commentID, shape.commentID, "comment must expose the root ID returned by comment creation")
			require.Equal(t, want.local, shape.hasQuote, "only local comments carry a quote")
			require.Equal(t, !want.local, shape.isWhole, "only whole-document comments carry is_whole=true")
			found[want.text] = struct{}{}
			if want.local {
				localKeys[key] = struct{}{}
			} else {
				wholeKeys[key] = struct{}{}
			}
			matched = true
			break
		}
		if !matched {
			t.Fatalf("unexpected comment %q in comments sidecar: %s", key, entry.Data)
		}
	}
	for _, want := range expected {
		if _, ok := found[want.text]; !ok {
			t.Fatalf("expected comment containing %q was not returned", want.text)
		}
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
			t.Fatalf("whole-document comment %q must not be attached to a body block", ref)
		}
	}
}

var (
	docsFetchCommentRefPattern = regexp.MustCompile(`^c[1-9][0-9]*$`)
	docsFetchCommentIDPattern  = regexp.MustCompile(`^[1-9][0-9]*$`)
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

type docsFetchCommentShape struct {
	hasQuote  bool
	commentID string
	isWhole   bool
}

func assertCommentXML(t *testing.T, data string) docsFetchCommentShape {
	t.Helper()

	allowedAttrOrder := map[string][]string{
		"comment":  {"id", "is_whole"},
		"quote":    {},
		"msg":      {"user"},
		"img":      {"src"},
		"cite":     {"type", "doc-id"},
		"reaction": {"key", "users", "count", "partial"},
	}
	decoder := xml.NewDecoder(strings.NewReader(data))
	stack := make([]string, 0, 3)
	sawRoot := false
	sawQuote := false
	messageCount := 0
	commentID := ""
	isWhole := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "comment sidecar must be well-formed XML")
		switch typed := token.(type) {
		case xml.StartElement:
			parent := ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			switch typed.Name.Local {
			case "comment":
				require.Empty(t, parent, "comment must be the single root")
				require.False(t, sawRoot, "comment sidecar must have one root")
				sawRoot = true
			case "quote", "msg":
				require.Equal(t, "comment", parent, "%s must be a direct comment child", typed.Name.Local)
			case "img", "cite", "reaction":
				require.Equal(t, "msg", parent, "%s must belong to one msg", typed.Name.Local)
			default:
				t.Fatalf("unexpected comment element <%s>", typed.Name.Local)
			}

			allowed := allowedAttrOrder[typed.Name.Local]
			lastPosition := -1
			for _, attr := range typed.Attr {
				position := -1
				for i, name := range allowed {
					if attr.Name.Local == name {
						position = i
						break
					}
				}
				if position < 0 {
					t.Fatalf("unexpected %s attribute %q", typed.Name.Local, attr.Name.Local)
				}
				if position <= lastPosition {
					t.Fatalf("%s attributes are out of contract order: %#v", typed.Name.Local, typed.Attr)
				}
				lastPosition = position
				if typed.Name.Local == "comment" && attr.Name.Local == "id" {
					commentID = attr.Value
				}
				if typed.Name.Local == "comment" && attr.Name.Local == "is_whole" {
					require.Equal(t, "true", attr.Value, "is_whole must use the true literal when present")
					isWhole = true
				}
				if typed.Name.Local == "cite" && attr.Name.Local == "type" {
					require.Equal(t, "doc", attr.Value, "cite type must be doc")
				}
			}
			if typed.Name.Local == "comment" {
				require.NotEmpty(t, typed.Attr, "comment must include id")
				require.Equal(t, "id", typed.Attr[0].Name.Local, "comment id must be the first attribute")
			}
			if typed.Name.Local == "quote" {
				require.False(t, sawQuote, "comment may contain at most one quote")
				sawQuote = true
			}
			if typed.Name.Local == "msg" {
				require.Len(t, typed.Attr, 1, "msg must include only user")
				require.Equal(t, "user", typed.Attr[0].Name.Local, "msg must include user even when the name is empty")
				messageCount++
			}
			if typed.Name.Local == "img" {
				require.Len(t, typed.Attr, 1, "img must include only src")
				require.Equal(t, "src", typed.Attr[0].Name.Local)
				require.NotEmpty(t, typed.Attr[0].Value)
			}
			if typed.Name.Local == "cite" {
				require.Len(t, typed.Attr, 2, "cite must include type and doc-id")
				require.Equal(t, "doc-id", typed.Attr[1].Name.Local)
				require.NotEmpty(t, typed.Attr[1].Value)
			}
			if typed.Name.Local == "reaction" {
				require.NotEmpty(t, typed.Attr, "reaction must include key")
				require.Equal(t, "key", typed.Attr[0].Name.Local, "reaction key must be the first attribute")
				require.NotEmpty(t, strings.TrimSpace(typed.Attr[0].Value))
			}
			stack = append(stack, typed.Name.Local)
		case xml.EndElement:
			require.NotEmpty(t, stack, "unexpected closing element </%s>", typed.Name.Local)
			require.Equal(t, stack[len(stack)-1], typed.Name.Local, "comment elements must close in order")
			stack = stack[:len(stack)-1]
		}
	}
	require.True(t, sawRoot)
	require.Empty(t, stack)
	require.True(t, docsFetchCommentIDPattern.MatchString(commentID), "comment must expose a positive numeric id")
	require.Positive(t, messageCount, "comment must contain at least one msg")
	return docsFetchCommentShape{hasQuote: sawQuote, commentID: commentID, isWhole: isWhole}
}

func addDocComment(t *testing.T, ctx context.Context, defaultAs, docToken, text string, locationArgs ...string) string {
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
	commentID := gjson.Get(result.Stdout, "data.comment_id").String()
	require.NotEmpty(t, commentID, "comment creation must return data.comment_id: %s", result.Stdout)
	return commentID
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

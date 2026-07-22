// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

// v2CreateFlags returns the flag definitions for the v2 (OpenAPI) create path.
func v2CreateFlags() []common.Flag {
	return []common.Flag{
		{Name: "title", Desc: "document title; the CLI prepends it to --content as <title>...</title>. In XML mode, top-level <title> elements in --content are removed so this flag wins without duplicate-title warnings"},
		{Name: "content", Desc: "document body; XML by default or Markdown when --doc-format markdown. " + docsContentSkillHelp + "; use --help for the latest command flags", Input: []string{common.File, common.Stdin}},
		{Name: "reference-map", Desc: docsReferenceMapFlagDesc, Input: []string{common.File, common.Stdin}},
		{Name: "doc-format", Desc: "content format; xml is default and supports richer DocxXML blocks, markdown imports plain Markdown", Default: "xml", Enum: []string{"xml", "markdown"}},
		{Name: "parent-token", Desc: "parent folder token or wiki node token; mutually exclusive with --parent-position"},
		{Name: "parent-position", Desc: "parent position such as my_library; mutually exclusive with --parent-token"},
	}
}

func validateCreateV2(_ context.Context, runtime *common.RuntimeContext) error {
	if err := validateDocsV2Only(runtime, "+create", docsCreateLegacyFlags()); err != nil {
		return err
	}
	title := strings.TrimSpace(runtime.Str("title"))
	if runtime.Changed("title") && title == "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--title must not be empty").WithParam("--title")
	}
	if err := validateDocsV2ReferenceMapFlags(runtime); err != nil {
		return err
	}
	if runtime.Str("parent-token") != "" && runtime.Str("parent-position") != "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--parent-token and --parent-position are mutually exclusive").WithParams(
			errs.InvalidParam{Name: "--parent-token", Reason: "mutually exclusive with --parent-position"},
			errs.InvalidParam{Name: "--parent-position", Reason: "mutually exclusive with --parent-token"},
		)
	}
	if runtime.Str("content") == "" && title == "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--content is required unless --title is provided").WithParam("--content")
	}
	if runtime.Str("content") != "" {
		_, err := resolveDocsV2ContentReferenceMap(runtime)
		return err
	}
	return nil
}

func dryRunCreateV2(_ context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
	body, err := buildCreateBodyWithHTML5ReferenceMap(runtime)
	if err != nil {
		return common.NewDryRunAPI().Set("error", err.Error())
	}
	desc := "OpenAPI: create document"
	if runtime.IsBot() {
		desc += ". After document creation succeeds in bot mode, the CLI will also try to grant the current CLI user full_access on the new document."
	}
	return common.NewDryRunAPI().
		POST("/open-apis/docs_ai/v1/documents").
		Desc(desc).
		Body(body)
}

func executeCreateV2(_ context.Context, runtime *common.RuntimeContext) error {
	body, err := buildCreateBodyWithHTML5ReferenceMap(runtime)
	if err != nil {
		return err
	}

	data, err := doDocAPI(runtime, "POST", "/open-apis/docs_ai/v1/documents", body)
	if err != nil {
		return err
	}

	augmentDocsCreatePermission(runtime, data)
	fallbackDocsCreateURLV2(runtime, data)
	runtime.OutRaw(data, nil)
	return nil
}

func buildCreateBody(runtime *common.RuntimeContext) map[string]interface{} {
	body := map[string]interface{}{
		"format":  runtime.Str("doc-format"),
		"content": buildCreateContent(runtime),
	}
	if v := runtime.Str("parent-token"); v != "" {
		body["parent_token"] = v
	}
	if v := runtime.Str("parent-position"); v != "" {
		body["parent_position"] = v
	}
	injectDocsScene(runtime, body)
	return body
}

func buildCreateContent(runtime *common.RuntimeContext) string {
	return buildCreateContentWithBody(runtime, runtime.Str("content"))
}

func buildCreateContentWithBody(runtime *common.RuntimeContext, content string) string {
	title := strings.TrimSpace(runtime.Str("title"))
	if title == "" {
		return content
	}
	if runtime.Str("doc-format") == "xml" {
		content = stripTopLevelXMLTitles(content)
	}

	titleTag := "<title>" + escapeDocTitleText(title) + "</title>"
	if content == "" {
		return titleTag
	}
	return titleTag + "\n" + content
}

type docContentRange struct {
	start int64
	end   int64
}

// stripTopLevelXMLTitles preserves the established --title-wins contract while
// avoiding duplicate-title warnings from XML content. If the fragment is not
// well-formed XML, it is left untouched for the service to diagnose.
func stripTopLevelXMLTitles(content string) string {
	const wrapperStart = "<root>"
	wrapped := wrapperStart + content + "</root>"
	decoder := xml.NewDecoder(strings.NewReader(wrapped))
	wrapperLen := int64(len(wrapperStart))
	depth := 0
	activeStart := int64(-1)
	ranges := make([]docContentRange, 0, 1)

	for {
		tokenStart := decoder.InputOffset()
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return content
		}

		switch value := token.(type) {
		case xml.StartElement:
			if depth == 1 && value.Name.Space == "" && value.Name.Local == "title" {
				activeStart = tokenStart - wrapperLen
			}
			depth++
		case xml.EndElement:
			depth--
			if activeStart >= 0 && depth == 1 && value.Name.Space == "" && value.Name.Local == "title" {
				ranges = append(ranges, docContentRange{start: activeStart, end: decoder.InputOffset() - wrapperLen})
				activeStart = -1
			}
		}
	}

	if len(ranges) == 0 {
		return content
	}

	var result strings.Builder
	cursor := int64(0)
	for _, item := range ranges {
		result.WriteString(content[int(cursor):int(item.start)])
		cursor = item.end
	}
	result.WriteString(content[int(cursor):])
	return strings.TrimSpace(result.String())
}

func escapeDocTitleText(title string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(title))
	return buf.String()
}

// augmentDocsCreatePermission grants full_access to the current CLI user when
// the document was created with bot identity.
func augmentDocsCreatePermission(runtime *common.RuntimeContext, data map[string]interface{}) {
	doc, _ := data["document"].(map[string]interface{})
	if doc == nil {
		return
	}
	docID := strings.TrimSpace(common.GetString(doc, "document_id"))
	if docID == "" {
		return
	}
	if grant := common.AutoGrantCurrentUserDrivePermission(runtime, docID, "docx"); grant != nil {
		data["permission_grant"] = grant
	}
}

// fallbackDocsCreateURLV2 fills data.document.url with a brand-standard URL
// when the OpenAPI response did not include one. Backfills only when missing,
// so any tenant-specific URL the backend returned is preserved.
func fallbackDocsCreateURLV2(runtime *common.RuntimeContext, data map[string]interface{}) {
	doc, _ := data["document"].(map[string]interface{})
	if doc == nil {
		return
	}
	if strings.TrimSpace(common.GetString(doc, "url")) != "" {
		return
	}
	docID := strings.TrimSpace(common.GetString(doc, "document_id"))
	if docID == "" {
		return
	}
	if u := common.BuildResourceURL(runtime.Config.Brand, "docx", docID); u != "" {
		doc["url"] = u
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

const (
	localDocFormat           = "xml"
	localOOXMLFetchToolName  = "LocalOOXMLFetch"
	localOOXMLUpdateToolName = "LocalOOXMLUpdate"
)

var validLocalUpdateCommands = map[string]bool{
	"str_replace":           true,
	"block_replace":         true,
	"block_delete":          true,
	"block_insert_after":    true,
	"append":                true,
	"table_insert_rows":     true,
	"table_insert_cols":     true,
	"table_delete_rows":     true,
	"table_delete_cols":     true,
	"table_merge_cells":     true,
	"table_unmerge_cells":   true,
	"table_update_property": true,
}

var DocsLocalFetch = common.Shortcut{
	Service:     "docs",
	Command:     "+local_fetch",
	Description: "Fetch a host-dispatched local Word document as DocxXML",
	Risk:        "read",
	Scopes:      []string{"docx:document:readonly"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags: []common.Flag{
		{Name: "doc", Desc: "host-provided local Word document_id (not a URL or file path)", Required: true},
		{Name: "detail", Desc: "detail level: simple for reading, with-ids for block references, full for styles and layout metadata", Default: "simple", Enum: []string{"simple", "with-ids", "full"}},
		{Name: "scope", Desc: "read scope: full | outline | section | range | layout | page", Default: "full", Enum: []string{"full", "outline", "section", "range", "layout", "page"}},
		{Name: "start-block-id", Desc: "section anchor or range start block id"},
		{Name: "end-block-id", Desc: "range end block id; -1 means through document end"},
		{Name: "max-depth", Desc: "outline heading depth or section/range subtree depth; -1 means unlimited", Type: "int", Default: "-1"},
		{Name: "start-page-index", Desc: "first page index for --scope page", Type: "int"},
		{Name: "end-page-index", Desc: "last page index for --scope page", Type: "int"},
	},
	Validate: validateLocalFetch,
	DryRun:   dryRunLocalFetch,
	Execute:  executeLocalFetch,
}

var DocsLocalUpdate = common.Shortcut{
	Service:     "docs",
	Command:     "+local_update",
	Description: "Update a host-dispatched local Word document with DocxXML",
	Risk:        "write",
	Scopes:      []string{"docx:document:write_only", "docx:document:readonly"},
	AuthTypes:   []string{"user", "bot"},
	Flags: []common.Flag{
		{Name: "doc", Desc: "host-provided local Word document_id (not a URL or file path)", Required: true},
		{Name: "command", Desc: "local Word update operation", Required: true, Enum: localUpdateCommandKeys()},
		{Name: "content", Desc: "DocxXML content; explicitly pass an empty value where the command contract requires empty content", Input: []string{common.File, common.Stdin}},
		{Name: "pattern", Desc: "unique text matched by str_replace"},
		{Name: "block-id", Desc: "target block id; table commands require the table block id"},
		{Name: "table-option", Desc: "single JSON object for table_* commands; the CLI serializes it into extra_param.table_option", Input: []string{common.File, common.Stdin}},
	},
	Validate: validateLocalUpdate,
	DryRun:   dryRunLocalUpdate,
	Execute:  executeLocalUpdate,
}

var DocsOOXMLFetch = common.Shortcut{
	Service:     "docs",
	Command:     "+ooxml_fetch",
	Description: "Fetch the editable OOXML file path for a host-dispatched local Word document",
	Risk:        "read",
	Scopes:      []string{"docx:document:readonly"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags: []common.Flag{
		{Name: "doc", Desc: "host-provided local Word document_id (not a URL or file path)", Required: true},
	},
	Validate: validateOOXMLFetch,
	DryRun:   dryRunOOXMLFetch,
	Execute:  executeOOXMLFetch,
}

var DocsOOXMLUpdate = common.Shortcut{
	Service:     "docs",
	Command:     "+ooxml_update",
	Description: "Notify the host to refresh a local Word document from an edited OOXML file",
	Risk:        "write",
	Scopes:      []string{"docx:document:write_only", "docx:document:readonly"},
	AuthTypes:   []string{"user", "bot"},
	Flags: []common.Flag{
		{Name: "doc", Desc: "host-provided local Word document_id (not a URL or file path)", Required: true},
		{Name: "file-path", Desc: "edited OOXML .docx path returned by the local OOXML workflow", Required: true},
	},
	Validate: validateOOXMLUpdate,
	DryRun:   dryRunOOXMLUpdate,
	Execute:  executeOOXMLUpdate,
}

func localUpdateCommandKeys() []string {
	return []string{
		"str_replace",
		"block_replace",
		"block_delete",
		"block_insert_after",
		"append",
		"table_insert_rows",
		"table_insert_cols",
		"table_delete_rows",
		"table_delete_cols",
		"table_merge_cells",
		"table_unmerge_cells",
		"table_update_property",
	}
}

func localDocumentID(runtime *common.RuntimeContext) (string, error) {
	documentID := strings.TrimSpace(runtime.Str("doc"))
	if documentID == "" {
		return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "--doc cannot be empty").WithParam("--doc")
	}
	if strings.Contains(documentID, "://") || strings.ContainsAny(documentID, "/\\?#") ||
		strings.IndexFunc(documentID, unicode.IsSpace) >= 0 {
		return "", errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"--doc must be a host-provided local Word document_id, not a URL or file path",
		).WithParam("--doc")
	}
	return documentID, nil
}

func localDocumentAPIPath(documentID, suffix string) string {
	return fmt.Sprintf("/open-apis/docs_ai/v1/documents/%s%s", documentID, suffix)
}

func validateLocalFetch(_ context.Context, runtime *common.RuntimeContext) error {
	if _, err := localDocumentID(runtime); err != nil {
		return err
	}
	if runtime.Int("max-depth") < -1 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--max-depth must be >= -1").WithParam("--max-depth")
	}

	scope := runtime.Str("scope")
	switch scope {
	case "full":
		if err := rejectLocalFetchBlockFlags(runtime); err != nil {
			return err
		}
		if err := rejectLocalFetchPageFlags(runtime); err != nil {
			return err
		}
		return rejectLocalFetchMaxDepth(runtime)
	case "outline":
		if err := rejectLocalFetchBlockFlags(runtime); err != nil {
			return err
		}
		return rejectLocalFetchPageFlags(runtime)
	case "section":
		if strings.TrimSpace(runtime.Str("start-block-id")) == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--scope section requires --start-block-id").WithParam("--start-block-id")
		}
		if runtime.Changed("end-block-id") {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--end-block-id is only supported with --scope range").WithParam("--end-block-id")
		}
		return rejectLocalFetchPageFlags(runtime)
	case "range":
		if strings.TrimSpace(runtime.Str("start-block-id")) == "" && strings.TrimSpace(runtime.Str("end-block-id")) == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--scope range requires --start-block-id or --end-block-id").WithParams(
				errs.InvalidParam{Name: "--start-block-id", Reason: "provide a range start or end block id"},
				errs.InvalidParam{Name: "--end-block-id", Reason: "provide a range start or end block id"},
			)
		}
		return rejectLocalFetchPageFlags(runtime)
	case "layout":
		if err := rejectLocalFetchBlockFlags(runtime); err != nil {
			return err
		}
		if err := rejectLocalFetchPageFlags(runtime); err != nil {
			return err
		}
		if err := rejectLocalFetchMaxDepth(runtime); err != nil {
			return err
		}
		if runtime.Str("detail") != "full" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--scope layout requires --detail full").WithParam("--detail")
		}
		return nil
	case "page":
		if err := rejectLocalFetchBlockFlags(runtime); err != nil {
			return err
		}
		if runtime.Changed("max-depth") {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--max-depth is not supported with --scope page").WithParam("--max-depth")
		}
		if runtime.Str("detail") != "full" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--scope page requires --detail full").WithParam("--detail")
		}
		start, end := runtime.Int("start-page-index"), runtime.Int("end-page-index")
		if start <= 0 {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--scope page requires a positive --start-page-index").WithParam("--start-page-index")
		}
		if end <= 0 {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--scope page requires a positive --end-page-index").WithParam("--end-page-index")
		}
		if start > end {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--start-page-index must be <= --end-page-index").WithParam("--start-page-index")
		}
		return nil
	default:
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "invalid --scope %q", scope).WithParam("--scope")
	}
}

func rejectLocalFetchBlockFlags(runtime *common.RuntimeContext) error {
	if runtime.Changed("start-block-id") {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--start-block-id is only supported with --scope section or range").WithParam("--start-block-id")
	}
	if runtime.Changed("end-block-id") {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--end-block-id is only supported with --scope range").WithParam("--end-block-id")
	}
	return nil
}

func rejectLocalFetchPageFlags(runtime *common.RuntimeContext) error {
	if runtime.Changed("start-page-index") {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--start-page-index is only supported with --scope page").WithParam("--start-page-index")
	}
	if runtime.Changed("end-page-index") {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--end-page-index is only supported with --scope page").WithParam("--end-page-index")
	}
	return nil
}

func rejectLocalFetchMaxDepth(runtime *common.RuntimeContext) error {
	if runtime.Changed("max-depth") {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--max-depth is only supported with --scope outline, section, or range").WithParam("--max-depth")
	}
	return nil
}

func buildLocalFetchBody(runtime *common.RuntimeContext) map[string]interface{} {
	exportOption := map[string]interface{}{
		"export_block_id":    false,
		"export_style_attrs": false,
	}
	switch runtime.Str("detail") {
	case "with-ids":
		exportOption["export_block_id"] = true
	case "full":
		exportOption["export_block_id"] = true
		exportOption["export_style_attrs"] = true
	}
	if runtime.Str("scope") == "layout" && runtime.Str("detail") == "full" {
		exportOption["export_rendered_page"] = true
	}

	body := map[string]interface{}{
		"format":        localDocFormat,
		"export_option": exportOption,
	}
	if readOption := buildLocalReadOption(runtime); readOption != nil {
		body["read_option"] = readOption
	}
	injectDocsScene(runtime, body)
	return body
}

func buildLocalReadOption(runtime *common.RuntimeContext) map[string]interface{} {
	scope := runtime.Str("scope")
	if scope == "" || scope == "full" {
		return nil
	}
	readOption := map[string]interface{}{"read_mode": scope}
	if value := strings.TrimSpace(runtime.Str("start-block-id")); value != "" {
		readOption["start_block_id"] = value
	}
	if value := strings.TrimSpace(runtime.Str("end-block-id")); value != "" {
		readOption["end_block_id"] = value
	}
	if depth := runtime.Int("max-depth"); depth >= 0 {
		readOption["max_depth"] = strconv.Itoa(depth)
	}
	if scope == "page" {
		readOption["start_page_index"] = strconv.Itoa(runtime.Int("start-page-index"))
		readOption["end_page_index"] = strconv.Itoa(runtime.Int("end-page-index"))
	}
	return readOption
}

func dryRunLocalFetch(_ context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
	documentID, _ := localDocumentID(runtime)
	return common.NewDryRunAPI().
		POST(localDocumentAPIPath(documentID, "/fetch")).
		Desc("docs_ai: fetch local Word document").
		Body(buildLocalFetchBody(runtime)).
		Set("document_id", documentID)
}

func executeLocalFetch(_ context.Context, runtime *common.RuntimeContext) error {
	documentID, _ := localDocumentID(runtime)
	data, err := doDocAPI(runtime, "POST", localDocumentAPIPath(documentID, "/fetch"), buildLocalFetchBody(runtime))
	if err != nil {
		return err
	}
	outputLocalFetchResult(runtime, data)
	return nil
}

func outputLocalFetchResult(runtime *common.RuntimeContext, data map[string]interface{}) {
	runtime.OutFormatRaw(data, nil, func(w io.Writer) {
		if document, ok := data["document"].(map[string]interface{}); ok {
			if content, ok := document["content"].(string); ok {
				fmt.Fprintln(w, content)
			}
		}
	})
}

func validateLocalUpdate(_ context.Context, runtime *common.RuntimeContext) error {
	if _, err := localDocumentID(runtime); err != nil {
		return err
	}
	command := runtime.Str("command")
	if !validLocalUpdateCommands[command] {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "invalid --command %q for local Word", command).WithParam("--command")
	}

	contentChanged := runtime.Changed("content")
	content := runtime.Str("content")
	pattern := runtime.Str("pattern")
	blockID := strings.TrimSpace(runtime.Str("block-id"))
	tableCommand := strings.HasPrefix(command, "table_")
	if command != "str_replace" && runtime.Changed("pattern") {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--pattern is only supported with --command str_replace").WithParam("--pattern")
	}
	if (command == "str_replace" || command == "append") && runtime.Changed("block-id") {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--block-id is not supported with --command %s", command).WithParam("--block-id")
	}

	if tableCommand {
		if blockID == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--command %s requires --block-id", command).WithParam("--block-id")
		}
		if !runtime.Changed("table-option") {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--command %s requires --table-option", command).WithParam("--table-option")
		}
		if _, err := parseLocalTableOption(command, runtime.Str("table-option")); err != nil {
			return err
		}
	} else if runtime.Changed("table-option") {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--table-option is only supported with table_* commands").WithParam("--table-option")
	}

	switch command {
	case "str_replace":
		if strings.TrimSpace(pattern) == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--command str_replace requires --pattern").WithParam("--pattern")
		}
		if !contentChanged {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--command str_replace requires --content; pass an explicit empty value to delete the match").WithParam("--content")
		}
	case "block_replace", "block_insert_after":
		if blockID == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--command %s requires --block-id", command).WithParam("--block-id")
		}
		if !contentChanged || content == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--command %s requires non-empty --content", command).WithParam("--content")
		}
	case "block_delete":
		if blockID == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--command block_delete requires --block-id").WithParam("--block-id")
		}
		if contentChanged {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--command block_delete does not accept --content").WithParam("--content")
		}
	case "append":
		if !contentChanged || content == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--command append requires non-empty --content").WithParam("--content")
		}
	case "table_insert_rows", "table_insert_cols":
		if !contentChanged {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--command %s requires --content; pass an explicit empty value when no cell content is inserted", command).WithParam("--content")
		}
	case "table_delete_rows", "table_delete_cols", "table_merge_cells", "table_unmerge_cells", "table_update_property":
		if contentChanged {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--command %s does not accept --content", command).WithParam("--content")
		}
	}
	return nil
}

func parseLocalTableOption(command, raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--table-option must be a non-empty JSON object").WithParam("--table-option")
	}
	var option map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &option); err != nil {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--table-option must be a valid JSON object: %v", err).WithParam("--table-option").WithCause(err)
	}
	if len(option) == 0 {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--table-option must not be empty").WithParam("--table-option")
	}

	required := map[string][]string{
		"table_insert_rows":     {"row_index"},
		"table_insert_cols":     {"column_index"},
		"table_delete_rows":     {"row_start_index", "row_end_index"},
		"table_delete_cols":     {"column_start_index", "column_end_index"},
		"table_merge_cells":     {"range"},
		"table_unmerge_cells":   {"cell"},
		"table_update_property": {"cell"},
	}[command]
	for _, key := range required {
		if _, ok := option[key]; !ok {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--table-option for %s requires %q", command, key).WithParam("--table-option")
		}
	}
	if command == "table_update_property" {
		_, hasBackground := option["background_color"]
		_, hasAlign := option["vertical_align"]
		if !hasBackground && !hasAlign {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--table-option for table_update_property requires background_color or vertical_align").WithParam("--table-option")
		}
	}
	return option, nil
}

func buildLocalUpdateBody(runtime *common.RuntimeContext) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"format":  localDocFormat,
		"command": runtime.Str("command"),
	}
	if runtime.Changed("content") {
		body["content"] = runtime.Str("content")
	}
	if runtime.Changed("pattern") {
		body["pattern"] = runtime.Str("pattern")
	}
	if value := strings.TrimSpace(runtime.Str("block-id")); value != "" {
		body["block_id"] = value
	}
	if runtime.Changed("table-option") {
		option, err := parseLocalTableOption(runtime.Str("command"), runtime.Str("table-option"))
		if err != nil {
			return nil, err
		}
		extraParam, err := json.Marshal(map[string]interface{}{"table_option": option})
		if err != nil {
			return nil, errs.NewInternalError(errs.SubtypeUnknown, "failed to serialize --table-option: %v", err).WithCause(err)
		}
		body["extra_param"] = string(extraParam)
	}
	injectDocsScene(runtime, body)
	return body, nil
}

func dryRunLocalUpdate(_ context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
	documentID, _ := localDocumentID(runtime)
	body, err := buildLocalUpdateBody(runtime)
	if err != nil {
		return common.NewDryRunAPI().Set("error", err.Error())
	}
	return common.NewDryRunAPI().
		PUT(localDocumentAPIPath(documentID, "")).
		Desc("docs_ai: update local Word document").
		Body(body).
		Set("document_id", documentID)
}

func executeLocalUpdate(_ context.Context, runtime *common.RuntimeContext) error {
	documentID, _ := localDocumentID(runtime)
	body, err := buildLocalUpdateBody(runtime)
	if err != nil {
		return err
	}
	data, err := doDocAPI(runtime, "PUT", localDocumentAPIPath(documentID, ""), body)
	if err != nil {
		return err
	}
	runtime.OutRaw(data, nil)
	return nil
}

func validateOOXMLFetch(_ context.Context, runtime *common.RuntimeContext) error {
	_, err := localDocumentID(runtime)
	return err
}

func validateOOXMLUpdate(_ context.Context, runtime *common.RuntimeContext) error {
	if _, err := localDocumentID(runtime); err != nil {
		return err
	}
	if strings.TrimSpace(runtime.Str("file-path")) == "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--file-path cannot be empty").WithParam("--file-path")
	}
	return nil
}

func buildOOXMLBody(toolName string, fields map[string]interface{}) (map[string]interface{}, error) {
	extra := map[string]interface{}{"ToolName": toolName}
	for key, value := range fields {
		extra[key] = value
	}
	raw, err := json.Marshal(extra)
	if err != nil {
		return nil, errs.NewInternalError(errs.SubtypeUnknown, "failed to serialize OOXML request: %v", err).WithCause(err)
	}
	return map[string]interface{}{"extra_param": string(raw)}, nil
}

func dryRunOOXMLFetch(_ context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
	documentID, _ := localDocumentID(runtime)
	body, err := buildOOXMLBody(localOOXMLFetchToolName, nil)
	if err != nil {
		return common.NewDryRunAPI().Set("error", err.Error())
	}
	return common.NewDryRunAPI().
		POST(localDocumentAPIPath(documentID, "/fetch")).
		Desc("docs_ai: fetch local Word OOXML path").
		Body(body).
		Set("document_id", documentID)
}

func executeOOXMLFetch(_ context.Context, runtime *common.RuntimeContext) error {
	documentID, _ := localDocumentID(runtime)
	body, err := buildOOXMLBody(localOOXMLFetchToolName, nil)
	if err != nil {
		return err
	}
	data, err := doDocAPI(runtime, "POST", localDocumentAPIPath(documentID, "/fetch"), body)
	if err != nil {
		return err
	}
	outputLocalFetchResult(runtime, data)
	return nil
}

func dryRunOOXMLUpdate(_ context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
	documentID, _ := localDocumentID(runtime)
	body, err := buildOOXMLBody(localOOXMLUpdateToolName, map[string]interface{}{
		"file_path": runtime.Str("file-path"),
	})
	if err != nil {
		return common.NewDryRunAPI().Set("error", err.Error())
	}
	return common.NewDryRunAPI().
		PUT(localDocumentAPIPath(documentID, "")).
		Desc("docs_ai: refresh local Word from OOXML path").
		Body(body).
		Set("document_id", documentID)
}

func executeOOXMLUpdate(_ context.Context, runtime *common.RuntimeContext) error {
	documentID, _ := localDocumentID(runtime)
	body, err := buildOOXMLBody(localOOXMLUpdateToolName, map[string]interface{}{
		"file_path": runtime.Str("file-path"),
	})
	if err != nil {
		return err
	}
	data, err := doDocAPI(runtime, "PUT", localDocumentAPIPath(documentID, ""), body)
	if err != nil {
		return err
	}
	runtime.OutRaw(data, nil)
	return nil
}

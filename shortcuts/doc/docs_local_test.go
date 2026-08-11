// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"context"
	"errors"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
	"github.com/spf13/cobra"
)

func newLocalFetchTestRuntime(t *testing.T, values map[string]string) *common.RuntimeContext {
	t.Helper()
	cmd := &cobra.Command{Use: "docs +local_fetch"}
	cmd.Flags().String("doc", "doccn_local", "")
	cmd.Flags().String("detail", "simple", "")
	cmd.Flags().String("scope", "full", "")
	cmd.Flags().String("start-block-id", "", "")
	cmd.Flags().String("end-block-id", "", "")
	cmd.Flags().Int("max-depth", -1, "")
	cmd.Flags().Int("start-page-index", 0, "")
	cmd.Flags().Int("end-page-index", 0, "")
	for name, value := range values {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s: %v", name, err)
		}
	}
	return common.TestNewRuntimeContext(cmd, nil)
}

func newLocalUpdateTestRuntime(t *testing.T, values map[string]string) *common.RuntimeContext {
	t.Helper()
	cmd := &cobra.Command{Use: "docs +local_update"}
	cmd.Flags().String("doc", "doccn_local", "")
	cmd.Flags().String("command", "", "")
	cmd.Flags().String("content", "", "")
	cmd.Flags().String("pattern", "", "")
	cmd.Flags().String("block-id", "", "")
	cmd.Flags().String("table-option", "", "")
	for name, value := range values {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s: %v", name, err)
		}
	}
	return common.TestNewRuntimeContext(cmd, nil)
}

func TestBuildLocalFetchBodyKeepsOnlineFieldsOut(t *testing.T) {
	t.Parallel()
	runtime := newLocalFetchTestRuntime(t, map[string]string{
		"scope":  "layout",
		"detail": "full",
	})
	if err := validateLocalFetch(context.Background(), runtime); err != nil {
		t.Fatalf("validateLocalFetch() error = %v", err)
	}
	body := buildLocalFetchBody(runtime)
	if got := body["edit_mode"]; got != localEditModeDocxXML {
		t.Fatalf("edit_mode = %#v, want %q", got, localEditModeDocxXML)
	}
	if _, ok := body["extra_param"]; ok {
		t.Fatalf("local fetch must not include online extra_param: %#v", body)
	}
	exportOption := body["export_option"].(map[string]interface{})
	if got := exportOption["export_rendered_page"]; got != true {
		t.Fatalf("export_rendered_page = %#v, want true", got)
	}
	readOption := body["read_option"].(map[string]interface{})
	if got := readOption["read_mode"]; got != "layout" {
		t.Fatalf("read_mode = %#v, want layout", got)
	}
}

func TestBuildLocalFetchBodyPageRange(t *testing.T) {
	t.Parallel()
	runtime := newLocalFetchTestRuntime(t, map[string]string{
		"scope":            "page",
		"detail":           "full",
		"start-page-index": "1",
		"end-page-index":   "2",
	})
	if err := validateLocalFetch(context.Background(), runtime); err != nil {
		t.Fatalf("validateLocalFetch() error = %v", err)
	}
	readOption := buildLocalFetchBody(runtime)["read_option"].(map[string]interface{})
	if got := readOption["start_page_index"]; got != 1 {
		t.Fatalf("start_page_index = %#v, want 1", got)
	}
	if got := readOption["end_page_index"]; got != 2 {
		t.Fatalf("end_page_index = %#v, want 2", got)
	}
}

func TestBuildLocalUpdateBodyUsesTopLevelTableOption(t *testing.T) {
	t.Parallel()
	runtime := newLocalUpdateTestRuntime(t, map[string]string{
		"command":      "table_delete_cols",
		"block-id":     "table_block",
		"table-option": `{"column_start_index":"B","column_end_index":"D"}`,
	})
	if err := validateLocalUpdate(context.Background(), runtime); err != nil {
		t.Fatalf("validateLocalUpdate() error = %v", err)
	}
	body, err := buildLocalUpdateBody(runtime)
	if err != nil {
		t.Fatalf("buildLocalUpdateBody() error = %v", err)
	}
	tableOption := body["table_option"].(map[string]interface{})
	if tableOption["column_start_index"] != "B" || tableOption["column_end_index"] != "D" {
		t.Fatalf("table_option = %#v", tableOption)
	}
	if _, ok := body["extra_param"]; ok {
		t.Fatalf("table_option must not be packed into extra_param: %#v", body)
	}
}

func TestBuildLocalUpdateBodyPreservesAppendCommand(t *testing.T) {
	t.Parallel()
	runtime := newLocalUpdateTestRuntime(t, map[string]string{
		"command": "append",
		"content": "<p>local Word</p>",
	})
	if err := validateLocalUpdate(context.Background(), runtime); err != nil {
		t.Fatalf("validateLocalUpdate() error = %v", err)
	}
	body, err := buildLocalUpdateBody(runtime)
	if err != nil {
		t.Fatalf("buildLocalUpdateBody() error = %v", err)
	}
	if got := body["command"]; got != "append" {
		t.Fatalf("command = %#v, want append", got)
	}
	if got := body["edit_mode"]; got != localEditModeDocxXML {
		t.Fatalf("edit_mode = %#v, want %q", got, localEditModeDocxXML)
	}
	if _, ok := body["revision_id"]; ok {
		t.Fatalf("local update must not include online revision_id: %#v", body)
	}
}

func TestBuildLocalUpdateBodyPreservesPatternWhitespace(t *testing.T) {
	t.Parallel()
	runtime := newLocalUpdateTestRuntime(t, map[string]string{
		"command": "str_replace",
		"pattern": " old text ",
		"content": "new text",
	})
	if err := validateLocalUpdate(context.Background(), runtime); err != nil {
		t.Fatalf("validateLocalUpdate() error = %v", err)
	}
	body, err := buildLocalUpdateBody(runtime)
	if err != nil {
		t.Fatalf("buildLocalUpdateBody() error = %v", err)
	}
	if got := body["pattern"]; got != " old text " {
		t.Fatalf("pattern = %#v, want exact input", got)
	}
}

func TestBuildOOXMLBodiesUseModeAndTopLevelFilePath(t *testing.T) {
	t.Parallel()
	fetchBody := buildOOXMLFetchBody()
	updateBody := buildOOXMLUpdateBody("/tmp/edited.docx")
	if fetchBody["edit_mode"] != localEditModeOOXML || fetchBody["format"] != localDocFormat {
		t.Fatalf("fetch body = %#v", fetchBody)
	}
	if updateBody["edit_mode"] != localEditModeOOXML || updateBody["file_path"] != "/tmp/edited.docx" {
		t.Fatalf("update body = %#v", updateBody)
	}
	if _, ok := fetchBody["extra_param"]; ok {
		t.Fatalf("fetch body must not include extra_param: %#v", fetchBody)
	}
	if _, ok := updateBody["extra_param"]; ok {
		t.Fatalf("update body must not include extra_param: %#v", updateBody)
	}
}

func TestLocalShortcutRejectsOnlineURL(t *testing.T) {
	t.Parallel()
	runtime := newLocalFetchTestRuntime(t, map[string]string{
		"doc": "https://example.com/docx/doxcnOnline",
	})
	err := validateLocalFetch(context.Background(), runtime)
	if err == nil {
		t.Fatal("validateLocalFetch() succeeded, want error")
	}
	problem, ok := errs.ProblemOf(err)
	var validationErr *errs.ValidationError
	if !ok || problem.Subtype != errs.SubtypeInvalidArgument || !errors.As(err, &validationErr) || validationErr.Param != "--doc" {
		t.Fatalf("problem = %#v, validation = %#v, ok = %v", problem, validationErr, ok)
	}
}

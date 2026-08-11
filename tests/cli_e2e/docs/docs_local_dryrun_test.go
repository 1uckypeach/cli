// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package docs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
)

func TestDocsLocalShortcutsDryRun(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "app")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	tests := []struct {
		name       string
		args       []string
		wantMethod string
		wantURL    string
		assertBody func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "local fetch page",
			args: []string{
				"docs", "+local_fetch",
				"--doc", "doccnLocalDryRun",
				"--scope", "page",
				"--detail", "full",
				"--start-page-index", "1",
				"--end-page-index", "2",
				"--dry-run",
			},
			wantMethod: "POST",
			wantURL:    "/open-apis/docs_ai/v1/local_documents/doccnLocalDryRun/fetch",
			assertBody: func(t *testing.T, body map[string]interface{}) {
				t.Helper()
				require.Equal(t, "xml", body["format"])
				require.Equal(t, "docx_xml", body["edit_mode"])
				require.NotContains(t, body, "extra_param")
				readOption := body["read_option"].(map[string]interface{})
				require.Equal(t, "page", readOption["read_mode"])
				require.EqualValues(t, 1, readOption["start_page_index"])
				require.EqualValues(t, 2, readOption["end_page_index"])
			},
		},
		{
			name: "local update table",
			args: []string{
				"docs", "+local_update",
				"--doc", "doccnLocalDryRun",
				"--command", "table_merge_cells",
				"--block-id", "table_block",
				"--table-option", `{"range":"A1:C3"}`,
				"--dry-run",
			},
			wantMethod: "PUT",
			wantURL:    "/open-apis/docs_ai/v1/local_documents/doccnLocalDryRun",
			assertBody: func(t *testing.T, body map[string]interface{}) {
				t.Helper()
				require.Equal(t, "xml", body["format"])
				require.Equal(t, "docx_xml", body["edit_mode"])
				require.Equal(t, "table_merge_cells", body["command"])
				require.Equal(t, map[string]interface{}{"range": "A1:C3"}, body["table_option"])
				require.NotContains(t, body, "extra_param")
			},
		},
		{
			name:       "ooxml fetch",
			args:       []string{"docs", "+ooxml_fetch", "--doc", "doccnLocalDryRun", "--dry-run"},
			wantMethod: "POST",
			wantURL:    "/open-apis/docs_ai/v1/local_documents/doccnLocalDryRun/fetch",
			assertBody: func(t *testing.T, body map[string]interface{}) {
				t.Helper()
				require.Equal(t, "xml", body["format"])
				require.Equal(t, "ooxml", body["edit_mode"])
				require.NotContains(t, body, "extra_param")
			},
		},
		{
			name: "ooxml update",
			args: []string{
				"docs", "+ooxml_update",
				"--doc", "doccnLocalDryRun",
				"--file-path", "/tmp/edited.docx",
				"--dry-run",
			},
			wantMethod: "PUT",
			wantURL:    "/open-apis/docs_ai/v1/local_documents/doccnLocalDryRun",
			assertBody: func(t *testing.T, body map[string]interface{}) {
				t.Helper()
				require.Equal(t, "xml", body["format"])
				require.Equal(t, "ooxml", body["edit_mode"])
				require.Equal(t, "/tmp/edited.docx", body["file_path"])
				require.NotContains(t, body, "extra_param")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := clie2e.RunCmd(ctx, clie2e.Request{
				Args:      tt.args,
				DefaultAs: "bot",
			})
			require.NoError(t, err)
			result.AssertExitCode(t, 0)
			require.Equal(t, tt.wantMethod, clie2e.DryRunGet(result.Stdout, "api.0.method").String())
			require.Equal(t, tt.wantURL, clie2e.DryRunGet(result.Stdout, "api.0.url").String())

			bodyRaw := clie2e.DryRunGet(result.Stdout, "api.0.body").Raw
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(bodyRaw), &body))
			tt.assertBody(t, body)
		})
	}
}

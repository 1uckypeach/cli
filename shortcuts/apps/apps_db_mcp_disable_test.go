// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/httpmock"
)

// TestAppsDBMcpDisable_BothBranches：停用两支路径的 URL 与摘要行。
func TestAppsDBMcpDisable_BothBranches(t *testing.T) {
	for _, tc := range []struct {
		name, url, idKey, idVal string
		args                    []string
	}{
		{"database", "/open-apis/spark/v1/databases/db_1/mcp/disable", "database_id", "db_1", []string{"--database-id", "db_1"}},
		{"app", "/open-apis/spark/v1/apps/app_1/db/mcp/disable", "app_id", "app_1", []string{"--app-id", "app_1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory, stdout, reg := newAppsExecuteFactory(t)
			reg.Register(&httpmock.Stub{
				Method: "POST", URL: tc.url,
				Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"enabled": false}},
			})
			args := append([]string{"+db-mcp-disable", "--format", "pretty", "--as", "user"}, tc.args...)
			if err := runAppsShortcut(t, AppsDBMcpDisable, args, factory, stdout); err != nil {
				t.Fatalf("execute err=%v", err)
			}
			if got := strings.TrimSpace(stdout.String()); got != "✓ DB MCP disabled: "+tc.idVal {
				t.Fatalf("pretty = %q", got)
			}
		})
	}
}

func TestAppsDBMcpDisable_Contract(t *testing.T) {
	assertDBResourceContract(t, AppsDBMcpDisable, "write", "spark:app:write")
}

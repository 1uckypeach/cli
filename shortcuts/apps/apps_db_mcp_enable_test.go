// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/httpmock"
)

// TestAppsDBMcpEnable_BothBranches：两支路径各自的 URL、回显字段名（app_id / database_id）与摘要行。
func TestAppsDBMcpEnable_BothBranches(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		url   string
		idKey string
		idVal string
	}{
		{"database", []string{"--database-id", "db_1"}, "/open-apis/spark/v1/databases/db_1/mcp/enable", "database_id", "db_1"},
		{"app", []string{"--app-id", "app_1"}, "/open-apis/spark/v1/apps/app_1/db/mcp/enable", "app_id", "app_1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory, stdout, reg := newAppsExecuteFactory(t)
			reg.Register(&httpmock.Stub{
				Method: "POST", URL: tc.url,
				Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"enabled": true}},
			})
			args := append([]string{"+db-mcp-enable", "--as", "user"}, tc.args...)
			if err := runAppsShortcut(t, AppsDBMcpEnable, args, factory, stdout); err != nil {
				t.Fatalf("execute err=%v", err)
			}
			d := parseEnvelopeData(t, stdout)
			if d[tc.idKey] != tc.idVal {
				t.Errorf("data must echo %s=%s, got %v", tc.idKey, tc.idVal, d)
			}
			if d["enabled"] != true {
				t.Errorf("enabled=%v want true", d["enabled"])
			}
		})
	}
}

// TestAppsDBMcpEnable_DoesNotFabricateEnabled：enabled 取服务端结果，不按动作词伪造。
// 这是开关命令，「以为开了其实没开」在使用时查不出来。
func TestAppsDBMcpEnable_DoesNotFabricateEnabled(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "POST", URL: "/open-apis/spark/v1/databases/db_1/mcp/enable",
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"enabled": false}},
	})
	if err := runAppsShortcut(t, AppsDBMcpEnable,
		[]string{"+db-mcp-enable", "--database-id", "db_1", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	if d := parseEnvelopeData(t, stdout); d["enabled"] != false {
		t.Fatalf("enabled must mirror the server, got %v", d["enabled"])
	}
}

// TestAppsDBMcpEnable_Pretty：单行 ✓ 摘要，回显传入的那个标识。
func TestAppsDBMcpEnable_Pretty(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "POST", URL: "/open-apis/spark/v1/databases/db_1/mcp/enable",
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"enabled": true}},
	})
	if err := runAppsShortcut(t, AppsDBMcpEnable,
		[]string{"+db-mcp-enable", "--database-id", "db_1", "--format", "pretty", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "✓ DB MCP enabled: db_1" {
		t.Fatalf("pretty = %q", got)
	}
}

func TestAppsDBMcpEnable_Contract(t *testing.T) {
	assertDBResourceContract(t, AppsDBMcpEnable, "write", "spark:app:write")
}

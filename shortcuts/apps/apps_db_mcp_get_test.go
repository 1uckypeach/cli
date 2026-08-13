// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/httpmock"
)

const dbMcpGetURL = "/open-apis/spark/v1/databases/db_1/mcp"

// TestAppsDBMcpGet_Enabled：已启用时透出 url / transport，标识字段随资源类型变化。
func TestAppsDBMcpGet_Enabled(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: dbMcpGetURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"name": "产品反馈库", "enabled": true,
			"url": "https://mcp.feishu.cn/miaoda_database/db_1", "transport": "streamable-http",
		}},
	})
	if err := runAppsShortcut(t, AppsDBMcpGet,
		[]string{"+db-mcp-get", "--database-id", "db_1", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	d := parseEnvelopeData(t, stdout)
	if d["database_id"] != "db_1" || d["name"] != "产品反馈库" || d["enabled"] != true {
		t.Fatalf("get data=%v", d)
	}
	if d["url"] != "https://mcp.feishu.cn/miaoda_database/db_1" || d["transport"] != "streamable-http" {
		t.Fatalf("connection config missing: %v", d)
	}
}

// TestAppsDBMcpGet_DisabledOmitsConfig：「关着」是正常返回（ok:true），不是错误；
// 未启用时 url / transport 按 omit-empty 省略、不补空串。
func TestAppsDBMcpGet_DisabledOmitsConfig(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: dbMcpGetURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"name": "产品反馈库", "enabled": false,
		}},
	})
	if err := runAppsShortcut(t, AppsDBMcpGet,
		[]string{"+db-mcp-get", "--database-id", "db_1", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("disabled must not be an error, got %v", err)
	}
	d := parseEnvelopeData(t, stdout)
	if d["enabled"] != false {
		t.Fatalf("enabled=%v want false", d["enabled"])
	}
	for _, k := range []string{"url", "transport"} {
		if _, present := d[k]; present {
			t.Errorf("%s must be omitted when disabled, got %v", k, d[k])
		}
	}
}

// TestAppsDBMcpGet_PrettyKeyValue：左对齐 key:value；未启用时缺席字段不占行，末尾给下一步。
func TestAppsDBMcpGet_PrettyKeyValue(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: dbMcpGetURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"name": "产品反馈库", "enabled": false,
		}},
	})
	if err := runAppsShortcut(t, AppsDBMcpGet,
		[]string{"+db-mcp-get", "--database-id", "db_1", "--format", "pretty", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	got := stdout.String()
	for _, want := range []string{"database_id:", "name:", "enabled:", "false"} {
		if !strings.Contains(got, want) {
			t.Errorf("pretty missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"url:", "transport:"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("pretty must not print absent field %q:\n%s", unwanted, got)
		}
	}
	// 「关着」这个正常返回里唯一有价值的可执行信息就是下一步，且 flag 名要跟着资源类型走。
	if !strings.Contains(got, "+db-mcp-enable --database-id db_1") {
		t.Errorf("disabled pretty must end with an actionable next step:\n%s", got)
	}
}

// TestAppsDBMcpGet_AppBranchNextStepUsesAppFlag：App 支路的下一步提示要用 --app-id，
// 不能照抄 --database-id —— 照抄会给出一条这条支路上执行不了的命令。
func TestAppsDBMcpGet_AppBranchNextStepUsesAppFlag(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: "/open-apis/spark/v1/apps/app_1/db/mcp",
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"name": "订单管理", "enabled": false,
		}},
	})
	if err := runAppsShortcut(t, AppsDBMcpGet,
		[]string{"+db-mcp-get", "--app-id", "app_1", "--format", "pretty", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "app_id:") {
		t.Errorf("app branch must echo app_id:\n%s", got)
	}
	if !strings.Contains(got, "+db-mcp-enable --app-id app_1") {
		t.Errorf("app branch next step must use --app-id:\n%s", got)
	}
}

func TestAppsDBMcpGet_Contract(t *testing.T) {
	assertDBResourceContract(t, AppsDBMcpGet, "read", "spark:app:read")
}

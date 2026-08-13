// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/httpmock"
)

const dbDeleteURL = "/open-apis/spark/v1/databases/db_7f3a9c21e0b84d55"

// TestAppsDBDelete_JSON：database_id 回显调用方传入值，deleted 取服务端结果。
func TestAppsDBDelete_JSON(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "DELETE", URL: dbDeleteURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"database_id": "db_7f3a9c21e0b84d55", "deleted": true,
		}},
	})
	if err := runAppsShortcut(t, AppsDBDelete,
		[]string{"+db-delete", "--database-id", "db_7f3a9c21e0b84d55", "--yes", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	d := parseEnvelopeData(t, stdout)
	if d["database_id"] != "db_7f3a9c21e0b84d55" || d["deleted"] != true {
		t.Fatalf("delete data=%v", d)
	}
}

// TestAppsDBDelete_DoesNotFabricateDeleted：服务端没回 deleted 时 CLI 不得伪造 true。
// 删库不可恢复，输出必须反映服务端实际说了什么。
func TestAppsDBDelete_DoesNotFabricateDeleted(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "DELETE", URL: dbDeleteURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"database_id": "db_7f3a9c21e0b84d55",
		}},
	})
	if err := runAppsShortcut(t, AppsDBDelete,
		[]string{"+db-delete", "--database-id", "db_7f3a9c21e0b84d55", "--yes", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	d := parseEnvelopeData(t, stdout)
	if d["deleted"] == true {
		t.Fatalf("deleted must not be fabricated when the server omits it, got %v", d)
	}
}

// TestAppsDBDelete_Pretty：单行 ✓ 摘要，只带 id（一期没有单库详情接口，回显不出库名）。
func TestAppsDBDelete_Pretty(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "DELETE", URL: dbDeleteURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"database_id": "db_7f3a9c21e0b84d55", "deleted": true,
		}},
	})
	if err := runAppsShortcut(t, AppsDBDelete,
		[]string{"+db-delete", "--database-id", "db_7f3a9c21e0b84d55", "--yes", "--format", "pretty", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != "✓ Database deleted: db_7f3a9c21e0b84d55" {
		t.Fatalf("pretty = %q", got)
	}
}

// TestAppsDBDelete_BlankDatabaseID：--database-id 传空串（cobra 的 Required 拦不住）应在
// Validate 阶段拒，不得拼出 /databases/ 这种空标识 URL 打到集合接口上。
func TestAppsDBDelete_BlankDatabaseID(t *testing.T) {
	factory, stdout, _ := newAppsExecuteFactory(t)
	err := runAppsShortcut(t, AppsDBDelete,
		[]string{"+db-delete", "--database-id", "  ", "--yes", "--as", "user"}, factory, stdout)
	if err == nil {
		t.Fatal("blank --database-id must be rejected")
	}
	if !strings.Contains(err.Error(), "--database-id") {
		t.Fatalf("error must point at --database-id, got %v", err)
	}
}

// TestAppsDBDelete_Contract 锁住命令面：high-risk-write（框架据此注入 --yes 关卡）/
// spark:app:write / user-only / 不收 --app-id。
func TestAppsDBDelete_Contract(t *testing.T) {
	assertStandaloneDBContract(t, AppsDBDelete, "high-risk-write", "spark:app:write", []string{"database-id"})
}

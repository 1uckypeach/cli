// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/httpmock"
)

const dbPermissionRevokeURL = "/open-apis/spark/v1/databases/db_7f3a9c21e0b84d55/permissions"

// runDBPermissionRevoke 跑一次撤销，返回原始 stdout。pretty=true 时走 --format pretty
// （此时 stdout 不是 JSON，调用方只看文本）。
func runDBPermissionRevoke(t *testing.T, removed interface{}, pretty bool) string {
	t.Helper()
	factory, stdout, reg := newAppsExecuteFactory(t)
	data := map[string]interface{}{}
	if removed != nil {
		data["removed"] = removed
	}
	reg.Register(&httpmock.Stub{
		Method: "DELETE", URL: dbPermissionRevokeURL,
		Body: map[string]interface{}{"code": 0, "data": data},
	})
	args := []string{"+db-permission-revoke", "--database-id", "db_7f3a9c21e0b84d55",
		"--user-id", "2233445566", "--as", "user"}
	if pretty {
		args = append(args, "--format", "pretty")
	}
	if err := runAppsShortcut(t, AppsDBPermissionRevoke, args, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	return stdout.String()
}

// TestAppsDBPermissionRevoke_JSON：permission 恒为显式 null，removed 取服务端结果。
func TestAppsDBPermissionRevoke_JSON(t *testing.T) {
	raw := runDBPermissionRevoke(t, true, false)
	var env struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("decode envelope: %v (raw=%q)", err, raw)
	}
	d := env.Data
	if d["database_id"] != "db_7f3a9c21e0b84d55" || d["user_id"] != "2233445566" || d["removed"] != true {
		t.Fatalf("revoke data=%v", d)
	}
	// 撤销后已无级别可言，显式 null 而不是省略该键 —— 与 +db-permission-set 的三字段形态对齐。
	if !strings.Contains(raw, `"permission": null`) {
		t.Errorf("permission must be explicit null:\n%s", raw)
	}
}

// TestAppsDBPermissionRevoke_PrettyDistinguishesIdempotentMiss：removed 区分两种成功的措辞。
// 能区分是因为服务端给了 removed 字段；+db-permission-set 没有，所以那边不能区分。
func TestAppsDBPermissionRevoke_PrettyDistinguishesIdempotentMiss(t *testing.T) {
	hit := runDBPermissionRevoke(t, true, true)
	if got := strings.TrimSpace(hit); got != "✓ Permission revoked: 2233445566" {
		t.Fatalf("revoked pretty = %q", got)
	}
	miss := runDBPermissionRevoke(t, false, true)
	if got := strings.TrimSpace(miss); got != "✓ No permission to revoke: 2233445566" {
		t.Fatalf("idempotent-miss pretty = %q", got)
	}
	// 两种都是成功：都打 ✓，不是错误。
	if !strings.HasPrefix(strings.TrimSpace(miss), "✓") {
		t.Errorf("idempotent miss is still a success: %q", miss)
	}
}

// TestAppsDBPermissionRevoke_Contract：write（不是 high-risk-write —— 撤权限可由任一 manage
// 立刻用 +db-permission-set 复原，不可逆的那一支已被服务端的「最后一名 manage」护栏挡住）。
func TestAppsDBPermissionRevoke_Contract(t *testing.T) {
	assertStandaloneDBContract(t, AppsDBPermissionRevoke, "write", "spark:app:write",
		[]string{"database-id", "user-id"})
}

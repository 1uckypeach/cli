// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/httpmock"
)

const dbPermissionSetURL = "/open-apis/spark/v1/databases/db_7f3a9c21e0b84d55/permissions"

// TestAppsDBPermissionSet_JSONAndWire：三字段回显调用方传入值；body 是 {user_id, permission}。
func TestAppsDBPermissionSet_JSONAndWire(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	stub := &httpmock.Stub{
		Method: "POST", URL: dbPermissionSetURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{}},
	}
	reg.Register(stub)
	if err := runAppsShortcut(t, AppsDBPermissionSet,
		[]string{"+db-permission-set", "--database-id", "db_7f3a9c21e0b84d55",
			"--user-id", "2233445566", "--permission", "runtime", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	d := parseEnvelopeData(t, stdout)
	if d["database_id"] != "db_7f3a9c21e0b84d55" || d["user_id"] != "2233445566" || d["permission"] != "runtime" {
		t.Fatalf("set data=%v", d)
	}
	var sent map[string]interface{}
	if err := json.Unmarshal(stub.CapturedBody, &sent); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if sent["user_id"] != "2233445566" || sent["permission"] != "runtime" {
		t.Fatalf("wire body=%s", stub.CapturedBody)
	}
	// database_id 在路径里，不该再进 body。
	if _, dup := sent["database_id"]; dup {
		t.Errorf("database_id belongs in the path, not the body: %s", stub.CapturedBody)
	}
}

// TestAppsDBPermissionSet_PrettyIdenticalOnRepeat：幂等重复设置的 pretty 与首次【逐字相同】。
// 服务端返回体不带变更标记，CLI 无从判断是否真改了，措辞不能自作主张区分。
func TestAppsDBPermissionSet_PrettyIdenticalOnRepeat(t *testing.T) {
	var outputs []string
	for i := 0; i < 2; i++ {
		factory, stdout, reg := newAppsExecuteFactory(t)
		reg.Register(&httpmock.Stub{
			Method: "POST", URL: dbPermissionSetURL,
			Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{}},
		})
		if err := runAppsShortcut(t, AppsDBPermissionSet,
			[]string{"+db-permission-set", "--database-id", "db_7f3a9c21e0b84d55",
				"--user-id", "2233445566", "--permission", "runtime", "--format", "pretty", "--as", "user"}, factory, stdout); err != nil {
			t.Fatalf("execute err=%v", err)
		}
		outputs = append(outputs, strings.TrimSpace(stdout.String()))
	}
	if outputs[0] != "✓ Permission set: 2233445566 → runtime" {
		t.Fatalf("pretty = %q", outputs[0])
	}
	if outputs[0] != outputs[1] {
		t.Fatalf("idempotent repeat must render identically: %q vs %q", outputs[0], outputs[1])
	}
}

// TestAppsDBPermissionSet_RejectsInvalidLevel：--permission 只收 manage / runtime，
// 由框架 Enum 在发请求前拦掉。
func TestAppsDBPermissionSet_RejectsInvalidLevel(t *testing.T) {
	factory, stdout, _ := newAppsExecuteFactory(t)
	err := runAppsShortcut(t, AppsDBPermissionSet,
		[]string{"+db-permission-set", "--database-id", "db_1", "--user-id", "u1", "--permission", "develop", "--as", "user"}, factory, stdout)
	if err == nil {
		t.Fatal("invalid --permission must be rejected before the request")
	}
	if !strings.Contains(err.Error(), "permission") {
		t.Fatalf("error must point at --permission, got %v", err)
	}
}

// TestAppsDBPermissionSet_BlankUserID：--user-id 传空白应在 Validate 阶段拒。
func TestAppsDBPermissionSet_BlankUserID(t *testing.T) {
	factory, stdout, _ := newAppsExecuteFactory(t)
	err := runAppsShortcut(t, AppsDBPermissionSet,
		[]string{"+db-permission-set", "--database-id", "db_1", "--user-id", "  ", "--permission", "runtime", "--as", "user"}, factory, stdout)
	if err == nil || !strings.Contains(err.Error(), "--user-id") {
		t.Fatalf("blank --user-id must be rejected with a --user-id error, got %v", err)
	}
}

// TestAppsDBPermissionSet_Contract：write / spark:app:write / 只认 --database-id。
func TestAppsDBPermissionSet_Contract(t *testing.T) {
	assertStandaloneDBContract(t, AppsDBPermissionSet, "write", "spark:app:write",
		[]string{"database-id", "user-id", "permission"})
}

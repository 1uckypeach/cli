// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/httpmock"
)

const dbCreateURL = "/open-apis/spark/v1/databases"

// TestAppsDBCreate_JSON：data 只出 database_id 与 name（PRD 形态），不透传服务端多余字段。
func TestAppsDBCreate_JSON(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "POST", URL: dbCreateURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"database_id": "db_7f3a9c21e0b84d55", "name": "产品反馈库",
			// 服务端将来多回字段也不应泄漏进 CLI 输出（白名单组装，不是黑名单删除）
			"schema_name": "workspace_baas_xxx",
		}},
	})
	if err := runAppsShortcut(t, AppsDBCreate,
		[]string{"+db-create", "--name", "产品反馈库", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	d := parseEnvelopeData(t, stdout)
	if d["database_id"] != "db_7f3a9c21e0b84d55" || d["name"] != "产品反馈库" {
		t.Fatalf("create data=%v", d)
	}
	if _, leaked := d["schema_name"]; leaked {
		t.Fatalf("server-only field leaked into output: %v", d)
	}
}

// TestAppsDBCreate_Pretty：写操作单行 ✓ 摘要，含 name 与 id，不含 description。
func TestAppsDBCreate_Pretty(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "POST", URL: dbCreateURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"database_id": "db_7f3a9c21e0b84d55", "name": "产品反馈库",
		}},
	})
	if err := runAppsShortcut(t, AppsDBCreate,
		[]string{"+db-create", "--name", "产品反馈库", "--description", "群反馈沉淀", "--format", "pretty", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != "✓ Database created: 产品反馈库 (db_7f3a9c21e0b84d55)" {
		t.Fatalf("pretty = %q", got)
	}
	if strings.Contains(got, "群反馈沉淀") {
		t.Fatalf("summary line must not echo description: %q", got)
	}
}

// TestAppsDBCreate_BodyOmitsBlankDescription：--description 缺省 / 空白时 wire 上不带该键，
// 不发 "description": ""。断言的是 httpmock 捕获的真实请求体，而不是 body 构造函数的返回值 ——
// 后者过不了「DryRun 与 Execute 是否真的走同一条构造」这一关。
func TestAppsDBCreate_BodyOmitsBlankDescription(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		wantKey bool
	}{
		{"omitted", []string{"+db-create", "--name", "n", "--as", "user"}, false},
		{"blank", []string{"+db-create", "--name", "n", "--description", "   ", "--as", "user"}, false},
		{"set", []string{"+db-create", "--name", "n", "--description", "d", "--as", "user"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory, stdout, reg := newAppsExecuteFactory(t)
			stub := &httpmock.Stub{
				Method: "POST", URL: dbCreateURL,
				Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
					"database_id": "db_1", "name": "n",
				}},
			}
			reg.Register(stub)
			if err := runAppsShortcut(t, AppsDBCreate, tc.args, factory, stdout); err != nil {
				t.Fatalf("execute err=%v", err)
			}
			var sent map[string]interface{}
			if err := json.Unmarshal(stub.CapturedBody, &sent); err != nil {
				t.Fatalf("decode captured body: %v (raw=%q)", err, stub.CapturedBody)
			}
			if _, has := sent["description"]; has != tc.wantKey {
				t.Errorf("wire body has description = %v, want %v (body=%s)", has, tc.wantKey, stub.CapturedBody)
			}
			if sent["name"] != "n" {
				t.Errorf("wire body name = %v, want n", sent["name"])
			}
		})
	}
}

// TestAppsDBCreate_BlankName：--name 传空串（cobra 的 Required 拦不住）应在 Validate 阶段拒。
func TestAppsDBCreate_BlankName(t *testing.T) {
	factory, stdout, _ := newAppsExecuteFactory(t)
	err := runAppsShortcut(t, AppsDBCreate,
		[]string{"+db-create", "--name", "   ", "--as", "user"}, factory, stdout)
	if err == nil {
		t.Fatal("blank --name must be rejected")
	}
	if !strings.Contains(err.Error(), "--name") {
		t.Fatalf("error must point at --name, got %v", err)
	}
}

// TestAppsDBCreate_Contract 锁住命令面：write / spark:app:write / user-only / 不收 --app-id。
func TestAppsDBCreate_Contract(t *testing.T) {
	assertStandaloneDBContract(t, AppsDBCreate, "write", "spark:app:write", []string{"name", "description"})
}

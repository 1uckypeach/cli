// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"net/http"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/httpmock"
)

const dbPermissionsURL = "/open-apis/spark/v1/databases/db_7f3a9c21e0b84d55/permissions"

func dbPermissionItem(userID, permission string) map[string]interface{} {
	return map[string]interface{}{"user_id": userID, "permission": permission}
}

// TestAppsDBPermissionList_JSON：items 原样透传，分页两键补齐。
func TestAppsDBPermissionList_JSON(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: dbPermissionsURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"items": []interface{}{
				dbPermissionItem("1234567890", "manage"),
				dbPermissionItem("2233445566", "runtime"),
			},
		}},
	})
	if err := runAppsShortcut(t, AppsDBPermissionList,
		[]string{"+db-permission-list", "--database-id", "db_7f3a9c21e0b84d55", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	raw := stdout.String()
	if !strings.Contains(raw, `"page_token": null`) {
		t.Errorf("must emit explicit page_token:null, got:\n%s", raw)
	}
	d := parseEnvelopeData(t, stdout)
	if d["has_more"] != false {
		t.Errorf("has_more must default to false, got %v", d["has_more"])
	}
	if items, _ := d["items"].([]interface{}); len(items) != 2 {
		t.Errorf("items len=%d want 2", len(items))
	}
}

// TestAppsDBPermissionList_Pretty：两列表格；单行结果仍走表格、不退化成 key:value。
func TestAppsDBPermissionList_Pretty(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: dbPermissionsURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"items": []interface{}{dbPermissionItem("2233445566", "runtime")},
		}},
	})
	if err := runAppsShortcut(t, AppsDBPermissionList,
		[]string{"+db-permission-list", "--database-id", "db_7f3a9c21e0b84d55", "--user-id", "2233445566", "--format", "pretty", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	got := stdout.String()
	// 表头必须在：一条结果也是表格，否则解析它的人要写两套。
	if !strings.Contains(got, "user_id") || !strings.Contains(got, "permission") {
		t.Errorf("single-row pretty must still print the table header:\n%s", got)
	}
	if !strings.Contains(got, "2233445566") || !strings.Contains(got, "runtime") {
		t.Errorf("pretty missing row:\n%s", got)
	}
}

// TestAppsDBPermissionList_EmptyPretty：筛不到不是错误，打一行提示。
func TestAppsDBPermissionList_EmptyPretty(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: dbPermissionsURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"items": []interface{}{}}},
	})
	if err := runAppsShortcut(t, AppsDBPermissionList,
		[]string{"+db-permission-list", "--database-id", "db_7f3a9c21e0b84d55", "--user-id", "9999999999", "--format", "pretty", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("filtering to nothing must not be an error, got %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "No permissions found." {
		t.Fatalf("empty pretty = %q", got)
	}
}

// TestAppsDBPermissionList_ParamsOmitBlank：user-id / page-token 缺省时 wire 上不带该键。
func TestAppsDBPermissionList_ParamsOmitBlank(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	var gotQuery string
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: dbPermissionsURL,
		OnMatch: func(req *http.Request) { gotQuery = req.URL.RawQuery },
		Body:    map[string]interface{}{"code": 0, "data": map[string]interface{}{"items": []interface{}{}}},
	})
	if err := runAppsShortcut(t, AppsDBPermissionList,
		[]string{"+db-permission-list", "--database-id", "db_7f3a9c21e0b84d55", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	if !strings.Contains(gotQuery, "page_size=20") {
		t.Errorf("query %q missing page_size", gotQuery)
	}
	for _, unwanted := range []string{"user_id=", "page_token="} {
		if strings.Contains(gotQuery, unwanted) {
			t.Errorf("query %q must not carry %q", gotQuery, unwanted)
		}
	}
}

// TestAppsDBPermissionList_Contract：read / spark:app:read / 只认 --database-id。
func TestAppsDBPermissionList_Contract(t *testing.T) {
	assertStandaloneDBContract(t, AppsDBPermissionList, "read", "spark:app:read",
		[]string{"database-id", "user-id", "page-size", "page-token"})
}

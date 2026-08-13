// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"net/http"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/httpmock"
)

const dbListURL = "/open-apis/spark/v1/databases"

func dbListItem(id, name, creator, createdAt string) map[string]interface{} {
	return map[string]interface{}{
		"database_id": id, "name": name, "creator_user_id": creator,
		"permission": "manage", "created_at": createdAt,
	}
}

// TestAppsDBList_JSONPaging：首页带游标时 page_token / has_more 原样透传。
func TestAppsDBList_JSONPaging(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: dbListURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"items": []interface{}{
				dbListItem("db_7f3a9c21e0b84d55", "产品反馈库", "1234567890", "2026-08-03T02:00:00Z"),
				dbListItem("db_5b18ee0742cd91af", "客户风险库", "2233445566", "2026-08-08T06:02:11Z"),
			},
			"page_token": "cursor-abc", "has_more": true,
		}},
	})
	if err := runAppsShortcut(t, AppsDBList,
		[]string{"+db-list", "--page-size", "2", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	d := parseEnvelopeData(t, stdout)
	if d["page_token"] != "cursor-abc" || d["has_more"] != true {
		t.Fatalf("paging fields=%v", d)
	}
	items, _ := d["items"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("items len=%d", len(items))
	}
}

// TestAppsDBList_NormalizesPagingOnLastPage：末页服务端省略 page_token 时，CLI 必须补成
// 显式 null 并给出 has_more=false —— agent 不该去区分「键缺席」与「键为 null」，
// 而 has_more 缺席会让翻页判据消失。
func TestAppsDBList_NormalizesPagingOnLastPage(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: dbListURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"items": []interface{}{dbListItem("db_9c02a4f1387b6e20", "项目风险库", "3344556677", "2026-08-09T01:12:40Z")},
		}},
	})
	if err := runAppsShortcut(t, AppsDBList,
		[]string{"+db-list", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	raw := stdout.String()
	if !strings.Contains(raw, `"page_token": null`) {
		t.Errorf("last page must emit explicit page_token:null, got:\n%s", raw)
	}
	d := parseEnvelopeData(t, stdout)
	if d["has_more"] != false {
		t.Errorf("has_more must default to false, got %v", d["has_more"])
	}
}

// TestAppsDBList_ParamsOmitBlankFilters：keyword / page-token 缺省时 wire 上不带该键。
// 断言 httpmock 捕获的真实 URL，而不是 params 构造函数的返回值。
func TestAppsDBList_ParamsOmitBlankFilters(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		wantIn    []string
		wantNotIn []string
	}{
		{"defaults", []string{"+db-list", "--as", "user"},
			[]string{"page_size=20"}, []string{"keyword=", "page_token="}},
		{"filters", []string{"+db-list", "--keyword", "反馈", "--page-token", "cur", "--page-size", "2", "--as", "user"},
			[]string{"page_size=2", "keyword=", "page_token=cur"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory, stdout, reg := newAppsExecuteFactory(t)
			var gotQuery string
			reg.Register(&httpmock.Stub{
				Method: "GET", URL: dbListURL,
				OnMatch: func(req *http.Request) { gotQuery = req.URL.RawQuery },
				Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
					"items": []interface{}{}, "has_more": false,
				}},
			})
			if err := runAppsShortcut(t, AppsDBList, tc.args, factory, stdout); err != nil {
				t.Fatalf("execute err=%v", err)
			}
			got := gotQuery
			for _, want := range tc.wantIn {
				if !strings.Contains(got, want) {
					t.Errorf("query %q missing %q", got, want)
				}
			}
			for _, unwanted := range tc.wantNotIn {
				if strings.Contains(got, unwanted) {
					t.Errorf("query %q must not carry %q", got, unwanted)
				}
			}
		})
	}
}

// TestAppsDBList_Pretty：5 列表格；created_at 由 UTC 本地化（不再带 Z）；不输出游标字段。
func TestAppsDBList_Pretty(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: dbListURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"items":      []interface{}{dbListItem("db_7f3a9c21e0b84d55", "产品反馈库", "1234567890", "2026-08-03T02:00:00Z")},
			"page_token": "cursor-abc", "has_more": true,
		}},
	})
	if err := runAppsShortcut(t, AppsDBList,
		[]string{"+db-list", "--format", "pretty", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("execute err=%v", err)
	}
	got := stdout.String()
	for _, want := range []string{"database_id", "name", "creator_user_id", "permission", "created_at", "db_7f3a9c21e0b84d55", "manage"} {
		if !strings.Contains(got, want) {
			t.Errorf("pretty missing %q:\n%s", want, got)
		}
	}
	// 游标是给程序用的，pretty 不输出
	for _, unwanted := range []string{"page_token", "has_more", "cursor-abc"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("pretty must not expose %q:\n%s", unwanted, got)
		}
	}
	if strings.Contains(got, "2026-08-03T02:00:00Z") {
		t.Errorf("created_at must be localized in pretty, got:\n%s", got)
	}
}

// TestAppsDBList_EmptyPretty：无可管理的库（含只有 runtime 的用户）是正常返回，
// pretty 打一行提示、不打表头。
func TestAppsDBList_EmptyPretty(t *testing.T) {
	factory, stdout, reg := newAppsExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "GET", URL: dbListURL,
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"items": []interface{}{}, "has_more": false,
		}},
	})
	if err := runAppsShortcut(t, AppsDBList,
		[]string{"+db-list", "--format", "pretty", "--as", "user"}, factory, stdout); err != nil {
		t.Fatalf("empty list must not be an error, got %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != "No databases found." {
		t.Fatalf("empty pretty = %q", got)
	}
}

// TestAppsDBList_PageSizeBounds：越界的 --page-size 在 Validate 阶段拒，不发请求。
func TestAppsDBList_PageSizeBounds(t *testing.T) {
	for _, size := range []string{"0", "101"} {
		factory, stdout, _ := newAppsExecuteFactory(t)
		err := runAppsShortcut(t, AppsDBList,
			[]string{"+db-list", "--page-size", size, "--as", "user"}, factory, stdout)
		if err == nil {
			t.Errorf("--page-size %s must be rejected", size)
			continue
		}
		if !strings.Contains(err.Error(), "--page-size") {
			t.Errorf("--page-size %s error must point at the flag, got %v", size, err)
		}
	}
}

// TestFormatDBLocalTime：只在 pretty 做本地化；解析不了的时间戳原样透出，不静默留空。
func TestFormatDBLocalTime(t *testing.T) {
	if got := formatDBLocalTime("not-a-time"); got != "not-a-time" {
		t.Fatalf("unparsable timestamp must pass through, got %q", got)
	}
	if got := formatDBLocalTime(nil); got != "" {
		t.Fatalf("nil timestamp = %q, want empty", got)
	}
	if got := formatDBLocalTime("   "); got != "" {
		t.Fatalf("blank timestamp = %q, want empty", got)
	}
}

// TestAppsDBList_Contract 锁住命令面：read / spark:app:read / user-only / 不收 --app-id。
func TestAppsDBList_Contract(t *testing.T) {
	assertStandaloneDBContract(t, AppsDBList, "read", "spark:app:read", []string{"keyword", "page-size", "page-token"})
}

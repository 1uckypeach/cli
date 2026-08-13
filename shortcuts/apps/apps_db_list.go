// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/larksuite/cli/shortcuts/common"
)

const dbListHint = "this lists standalone databases only; use `lark-cli apps +list` for Miaoda apps"

// AppsDBList lists the standalone databases the caller can manage.
//
// GET /databases（cursor 分页），params keyword / page_size / page_token。
//
// 只返回调用者具有 manage 的库：无权限的库不透出，只持有 runtime 的用户拿到空列表而不是权限
// 错误 —— 「关着的门」和「没有门」对使用者是同一件事，报错反而让 agent 每次都要先 catch 一次。
// 因此 items[].permission 这一列恒为 "manage"；字段保留是为了将来放开 runtime 可见性时不改契约。
//
// 【不收 --app-id】—— 妙搭 App 的库用 apps +list，误传由 cobra 拒为 unknown flag。
var AppsDBList = common.Shortcut{
	Service:     appsService,
	Command:     "+db-list",
	Description: "List standalone databases you can manage (cursor pagination)",
	Risk:        "read",
	Tips: []string{
		"Example: lark-cli apps +db-list",
		"Names may repeat: --keyword can match several databases; never pick one for the user — surface the candidates and let them choose. database_id is the only reliable identifier.",
		"pretty output omits page_token/has_more; page through with the default JSON output.",
	},
	Scopes:    []string{"spark:app:read"},
	AuthTypes: []string{"user"},
	HasFormat: true,
	Flags: []common.Flag{
		{Name: "keyword", Desc: "filter by name keyword (may match several databases)"},
		{Name: "page-size", Type: "int", Default: "20", Desc: "page size"},
		{Name: "page-token", Desc: "pagination cursor from previous response"},
	},
	Validate: func(ctx context.Context, rctx *common.RuntimeContext) error {
		return validateAppsPageSize(rctx.Int("page-size"))
	},
	DryRun: func(ctx context.Context, rctx *common.RuntimeContext) *common.DryRunAPI {
		return common.NewDryRunAPI().
			GET(databasesPath()).
			Desc("List standalone databases").
			Params(buildDBListParams(rctx))
	},
	Execute: func(ctx context.Context, rctx *common.RuntimeContext) error {
		data, err := rctx.CallAPITyped("GET", databasesPath(), buildDBListParams(rctx), nil)
		if err != nil {
			return withAppsHint(err, dbListHint)
		}
		normalizeDBListPaging(data)
		rctx.OutFormat(data, nil, func(w io.Writer) {
			renderDBListPretty(w, data["items"])
		})
		return nil
	},
}

// buildDBListParams 组装列表 query：keyword / page-token 为空时省略该键，不发空值。
// DryRun 与 Execute 共用，保证预览与真实请求一致。
func buildDBListParams(rctx *common.RuntimeContext) map[string]interface{} {
	params := map[string]interface{}{"page_size": rctx.Int("page-size")}
	if kw := strings.TrimSpace(rctx.Str("keyword")); kw != "" {
		params["keyword"] = kw
	}
	if token := strings.TrimSpace(rctx.Str("page-token")); token != "" {
		params["page_token"] = token
	}
	return params
}

// normalizeDBListPaging 补齐分页两键，使 JSON 输出的形状与是否末页无关。
//
// 服务端在末页会省略 page_token（thrift optional 的默认行为）。若直接透传，agent 就要区分
// 「键不存在」和「键为 null」两种情况；而 has_more 缺席更危险 —— 它是翻页的唯一判据，
// 缺了会被读成 false 还是漏判取决于调用方语言。这里统一成 page_token:null + has_more:false。
func normalizeDBListPaging(data map[string]interface{}) {
	if _, ok := data["page_token"]; !ok {
		data["page_token"] = nil
	}
	if _, ok := data["has_more"]; !ok {
		data["has_more"] = false
	}
}

// renderDBListPretty 5 列对齐表格：database_id / name / creator_user_id / permission / created_at。
//
// 空列表打一行提示、不打表头（对齐现网 "No files found." / "No DDL changes found."）。
// 【不输出 page_token 与 has_more】—— pretty 是给人看的，游标是给程序用的；要翻页就用 JSON。
// 直接后果是 pretty 下末页与首页无法区分，这是刻意取舍。
func renderDBListPretty(w io.Writer, raw interface{}) {
	arr, _ := raw.([]interface{})
	if len(arr) == 0 {
		io.WriteString(w, "No databases found.\n")
		return
	}
	headers := []string{"database_id", "name", "creator_user_id", "permission", "created_at"}
	rows := make([][]string, 0, len(arr))
	for _, it := range arr {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		rows = append(rows, []string{
			common.GetString(m, "database_id"),
			common.GetString(m, "name"),
			common.GetString(m, "creator_user_id"),
			common.GetString(m, "permission"),
			formatDBLocalTime(m["created_at"]),
		})
	}
	renderAlignedTable(w, headers, rows)
}

// formatDBLocalTime 把服务端的 RFC3339 UTC 时间戳渲染成本地时间 "2006-01-02 15:04:05"。
//
// 只在 pretty 里做本地化，JSON 保持服务端原样的 UTC（带 Z）—— 机器读的输出不应随运行机器的
// 时区变化。解析失败时原样透出而不是留空：宁可让人看到一个没见过的格式，也不要静默丢掉服务端值。
func formatDBLocalTime(v interface{}) string {
	if v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

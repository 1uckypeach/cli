// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"io"
	"strings"

	"github.com/larksuite/cli/shortcuts/common"
)

const dbPermissionHint = "check the --database-id; run `lark-cli apps +db-list` to see the databases you can access"

// AppsDBPermissionList lists every member permission on a standalone database.
//
// GET /databases/{database_id}/permissions（cursor 分页），params user_id / page_size / page_token。
//
// 【始终返回全量成员】—— CLI 只有 manage 能调用，不存在「只看自己」的降级分支；
// `--user-id` 是查询过滤器而非权限降级，筛不到就是空 items，不是错误。

// 【只认 --database-id】—— 妙搭 App 的协作者归 apps +member-* 那一套管（三档 view/edit/full_access，
// 主体可以是人/群/部门），主体模型与后端存储都和独立库的两级权限不同，不并入二选一。
// 误传 --app-id 由 cobra 拒为 unknown flag。
var AppsDBPermissionList = common.Shortcut{
	Service:     appsService,
	Command:     "+db-permission-list",
	Description: "List member permissions on a standalone database (cursor pagination)",
	Risk:        "read",
	Tips: []string{
		"Example: lark-cli apps +db-permission-list --database-id <database_id>",
		"--user-id is a filter, not a scope reducer: this command always returns every member.",
	},
	Scopes:    []string{"spark:app:read"},
	AuthTypes: []string{"user"},
	HasFormat: true,
	Flags: []common.Flag{
		{Name: "database-id", Desc: "standalone database id", Required: true},
		{Name: "user-id", Desc: "filter by a single Lark user id"},
		{Name: "page-size", Type: "int", Default: "20", Desc: "page size"},
		{Name: "page-token", Desc: "pagination cursor from previous response"},
	},
	Validate: func(ctx context.Context, rctx *common.RuntimeContext) error {
		if _, err := requireDatabaseID(rctx.Str("database-id")); err != nil {
			return err
		}
		return validateAppsPageSize(rctx.Int("page-size"))
	},
	DryRun: func(ctx context.Context, rctx *common.RuntimeContext) *common.DryRunAPI {
		databaseID, _ := requireDatabaseID(rctx.Str("database-id"))
		return common.NewDryRunAPI().
			GET(databasePermissionsPath(databaseID)).
			Desc("List database permissions").
			Params(buildDBPermissionListParams(rctx))
	},
	Execute: func(ctx context.Context, rctx *common.RuntimeContext) error {
		databaseID, err := requireDatabaseID(rctx.Str("database-id"))
		if err != nil {
			return err
		}
		data, err := rctx.CallAPITyped("GET", databasePermissionsPath(databaseID), buildDBPermissionListParams(rctx), nil)
		if err != nil {
			return withAppsHint(err, dbPermissionHint)
		}
		normalizeDBListPaging(data)
		rctx.OutFormat(data, nil, func(w io.Writer) {
			renderDBPermissionListPretty(w, data["items"])
		})
		return nil
	},
}

// buildDBPermissionListParams 组装 query：user-id / page-token 为空时省略该键。
// DryRun 与 Execute 共用，保证预览与真实请求一致。
func buildDBPermissionListParams(rctx *common.RuntimeContext) map[string]interface{} {
	params := map[string]interface{}{"page_size": rctx.Int("page-size")}
	if uid := strings.TrimSpace(rctx.Str("user-id")); uid != "" {
		params["user_id"] = uid
	}
	if token := strings.TrimSpace(rctx.Str("page-token")); token != "" {
		params["page_token"] = token
	}
	return params
}

// renderDBPermissionListPretty 两列对齐表格：user_id / permission。
//
// 单行结果仍走表格、不退化成 key:value —— 同一条命令的输出形态不应随结果条数变化，
// 否则解析它的人要写两套。空结果打一行提示，不打表头。
func renderDBPermissionListPretty(w io.Writer, raw interface{}) {
	arr, _ := raw.([]interface{})
	if len(arr) == 0 {
		io.WriteString(w, "No permissions found.\n")
		return
	}
	rows := make([][]string, 0, len(arr))
	for _, it := range arr {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		rows = append(rows, []string{common.GetString(m, "user_id"), common.GetString(m, "permission")})
	}
	renderAlignedTable(w, []string{"user_id", "permission"}, rows)
}

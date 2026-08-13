// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"io"
	"strings"

	"github.com/larksuite/cli/shortcuts/common"
)

const dbTableGetHint = "verify --app-id and --table are correct; list tables with `lark-cli apps +db-table-list --app-id <app_id>`; if targeting --environment dev, create it first with `lark-cli apps +db-env-create --app-id <app_id> --environment dev`"

// 独立 DB 没有 app、没有环境，也没有 +db-env-create，hint 另写一份。
const dbTableGetDatabaseHint = "verify --database-id and --table are correct; list tables with `lark-cli apps +db-table-list --database-id <database_id>`"

// AppsDBTableGet gets one table's structure (动词对齐 +db-table-list)。
//
// GET /apps/{app_id}/tables/{table_name}。
//
// `--format` 同时驱动 CLI 渲染和 server 请求形态：
//   - `--format json`（默认）/ table / ndjson / csv：CLI 不传 format query，response 含结构化
//     columns / indexes / constraints / stats，envelope 化输出。
//   - `--format pretty`：CLI 给 server 带 ?format=ddl，response 含 ddl 字符串，stdout 直接打
//     ddl 内容（无 envelope / 无表格包装）。
var AppsDBTableGet = common.Shortcut{
	Service:     appsService,
	Command:     "+db-table-get",
	Description: "Get a table's structure: columns, indexes and constraints",
	Risk:        "read",
	Tips: []string{
		"Example: lark-cli apps +db-table-get --app-id <app_id> --table <table>",
		"Tip: filter fields with --jq (json format), e.g. -q '.data.columns[].name'",
	},
	Scopes:    []string{"spark:app:read"},
	AuthTypes: []string{"user"},
	HasFormat: true,
	Flags: append(append(
		dbResourceFlags("app id (mutually exclusive with --database-id)", "standalone database id (mutually exclusive with --app-id)"),
		[]common.Flag{
			{Name: "table", Desc: "table name", Required: true},
		}...), dbEnvFlags("", []string{"dev", "online"}, "target db environment (Miaoda app only; a standalone database has none); leave unset to auto-select (multi-env app uses dev, single-env uses online), or pass dev/online")...),
	Validate: func(ctx context.Context, rctx *common.RuntimeContext) error {
		if _, _, err := requireDBResource(rctx); err != nil {
			return err
		}
		if err := rejectLegacyEnvFlag(rctx); err != nil {
			return err
		}
		if strings.TrimSpace(rctx.Str("table")) == "" {
			return appsValidationParamError("--table", "--table is required")
		}
		return nil
	},
	DryRun: func(ctx context.Context, rctx *common.RuntimeContext) *common.DryRunAPI {
		kind, url, params := dbTableGetTarget(rctx)
		return common.NewDryRunAPI().
			GET(url).
			Desc(dbHintFor(kind, "Get app db table schema", "Get standalone database table schema")).
			Params(params)
	},
	Execute: func(ctx context.Context, rctx *common.RuntimeContext) error {
		if _, _, err := requireDBResource(rctx); err != nil {
			return err
		}
		kind, url, params := dbTableGetTarget(rctx)
		data, err := rctx.CallAPITyped("GET", url, params, nil)
		if err != nil {
			return withAppsHint(err, dbHintFor(kind, dbTableGetHint, dbTableGetDatabaseHint))
		}
		rctx.OutFormat(data, nil, func(w io.Writer) {
			// pretty 模式：stdout 直接打 ddl 文本（无 trailing newline，由 server 返回的字符串决定）。
			io.WriteString(w, common.GetString(data, "ddl"))
		})
		return nil
	},
}

// buildDBTableGetParams 构造 schema 接口的 query。
//
// CLI 检测 rctx.Format == "pretty" 时给 server 带 format=ddl，要求返 CREATE 语句文本；
// 其他 format（含默认 json）不传该参数，让 server 返默认结构化字段。
// dbTableGetTarget 按标识分派，DryRun 与 Execute 共用。
//
// 【表名的位置两支不同】：App 支路沿用存量的 /apps/{id}/tables/{table}（表名在路径段，属存量债）；
// 独立 DB 支路是 /databases/{id}/table?table=xxx —— 表名是用户自定义值，进路径段会被
// url.PathEscape 漏掉的 ".." 之类规范化掉、打到别的接口上。
func dbTableGetTarget(rctx *common.RuntimeContext) (dbResourceKind, string, map[string]interface{}) {
	kind, id, _ := requireDBResource(rctx)
	table := strings.TrimSpace(rctx.Str("table"))
	params := map[string]interface{}{}
	if rctx.Format == "pretty" {
		params["format"] = "ddl"
	}
	if kind == dbResourceDatabase {
		params["table"] = table
		return kind, databaseTablePath(id), params
	}
	return kind, appTablePath(id, table), dbEnvParams(rctx, params)
}

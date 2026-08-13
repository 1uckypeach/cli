// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/larksuite/cli/shortcuts/common"
)

// DB MCP 三条命令（+db-mcp-enable / -disable / -get）共用的接线。
//
// 三条都收「--app-id 或 --database-id」二选一。
// 【两支路径的域段不对称】：独立 DB 是 /databases/{id}/mcp[/action]，妙搭 App 是
// /apps/{id}/db/mcp[/action] —— 多一段 db/。所以分派靠 dbMcpPath 而不是拼一个通用 suffix。

const (
	dbMcpAppIDDesc      = "Miaoda app id (mutually exclusive with --database-id)"
	dbMcpDatabaseIDDesc = "standalone database id (mutually exclusive with --app-id)"
	dbMcpHint           = "verify the identifier: --app-id for a Miaoda app, --database-id for a standalone database (`lark-cli apps +db-list`)"
)

// dbMcpTarget 把二选一的解析结果收敛成「URL + 回显字段名 + 回显值」。
// 回显字段名随资源类型变化（app_id / database_id），这是设计文档要求的输出形态。
//
// DryRun 与 Execute 都经这一个函数取 URL —— dry-run 是 agent 唯一的自检手段，
// 预览与真实请求不一致会让这个手段失效。
func dbMcpTarget(rctx *common.RuntimeContext, action string) (url, idKey, idValue string, err error) {
	kind, id, err := requireDBResource(rctx)
	if err != nil {
		return "", "", "", err
	}
	if kind == dbResourceDatabase {
		return databaseMcpPath(id, action), "database_id", id, nil
	}
	return appDbMcpPath(id, action), "app_id", id, nil
}

// dbMcpSwitchShortcut 造 +db-mcp-enable / +db-mcp-disable —— 两条命令除动作词与文案外完全同构，
// 各自手写只会让它们随时间长歪（一边改了 hint / 输出形态另一边忘改）。
func dbMcpSwitchShortcut(action, pastTense string) common.Shortcut {
	verb := capitalizeFirst(action)
	return common.Shortcut{
		Service:     appsService,
		Command:     "+db-mcp-" + action,
		Description: verb + " DB MCP for a Miaoda app or a standalone database",
		Risk:        "write",
		Tips: []string{
			fmt.Sprintf("Example: lark-cli apps +db-mcp-%s --database-id <database_id>", action),
			"Switch semantics are idempotent: repeating the call returns the current state.",
		},
		Scopes:    []string{"spark:app:write"},
		AuthTypes: []string{"user"},
		HasFormat: true,
		Flags:     dbResourceFlags(dbMcpAppIDDesc, dbMcpDatabaseIDDesc),
		Validate: func(ctx context.Context, rctx *common.RuntimeContext) error {
			_, _, _, err := dbMcpTarget(rctx, action)
			return err
		},
		DryRun: func(ctx context.Context, rctx *common.RuntimeContext) *common.DryRunAPI {
			url, idKey, _, _ := dbMcpTarget(rctx, action)
			return common.NewDryRunAPI().POST(url).Desc(verb + " DB MCP for " + dbResourceLabel(idKey))
		},
		Execute: func(ctx context.Context, rctx *common.RuntimeContext) error {
			url, idKey, idValue, err := dbMcpTarget(rctx, action)
			if err != nil {
				return err
			}
			data, err := rctx.CallAPITyped("POST", url, nil, nil)
			if err != nil {
				return withAppsHint(err, dbMcpHint)
			}
			// enabled 取服务端结果，不按动作词伪造：这是开关命令，输出必须反映服务端实际状态，
			// 否则「以为开了其实没开」在使用时查不出来。
			out := map[string]interface{}{idKey: idValue, "enabled": data["enabled"]}
			rctx.OutFormat(out, nil, func(w io.Writer) {
				fmt.Fprintf(w, "✓ DB MCP %s: %s\n", pastTense, idValue)
			})
			return nil
		},
	}
}

// dbResourceLabel 供 dry-run 的 desc 用，措辞随资源类型分支。
func dbResourceLabel(idKey string) string {
	if idKey == "app_id" {
		return "a Miaoda app database"
	}
	return "a standalone database"
}

// flagNameForIDKey 把回显字段名换回 flag 名（app_id → app-id），供 hint 给出可直接执行的命令。
func flagNameForIDKey(idKey string) string {
	if idKey == "app_id" {
		return "app-id"
	}
	return "database-id"
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

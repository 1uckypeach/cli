// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"fmt"
	"io"

	"github.com/larksuite/cli/shortcuts/common"
)

// AppsDBMcpGet reads the DB MCP state and connection config.
//
// GET /databases/{id}/mcp 或 /apps/{id}/db/mcp（二选一），接口返回原样透传。
//
// 【「关着」是正常返回】—— enabled:false 配 ok:true，不是错误。若返错误，agent 每次探测状态
// 都要先 catch 一次，而且分不清「关着」与「查不到」。
// 未启用时 url / transport 按现网 omit-empty 约定省略，不补空串
// （先例：+db-quota-get 未对接配额时省略 storage_quota_bytes / usage_percent）。
var AppsDBMcpGet = common.Shortcut{
	Service:     appsService,
	Command:     "+db-mcp-get",
	Description: "Get DB MCP state and connection config for a Miaoda app or a standalone database",
	Risk:        "read",
	Tips: []string{
		"Example: lark-cli apps +db-mcp-get --database-id <database_id>",
		"Tip: hand the endpoint straight to an agent with -q '.data.url'",
		"enabled:false is a normal response, not an error.",
	},
	Scopes:    []string{"spark:app:read"},
	AuthTypes: []string{"user"},
	HasFormat: true,
	Flags:     dbResourceFlags(dbMcpAppIDDesc, dbMcpDatabaseIDDesc),
	Validate: func(ctx context.Context, rctx *common.RuntimeContext) error {
		_, _, _, err := dbMcpTarget(rctx, "")
		return err
	},
	DryRun: func(ctx context.Context, rctx *common.RuntimeContext) *common.DryRunAPI {
		url, idKey, _, _ := dbMcpTarget(rctx, "")
		return common.NewDryRunAPI().GET(url).Desc("Get DB MCP state and config for " + dbResourceLabel(idKey))
	},
	Execute: func(ctx context.Context, rctx *common.RuntimeContext) error {
		url, idKey, idValue, err := dbMcpTarget(rctx, "")
		if err != nil {
			return err
		}
		data, err := rctx.CallAPITyped("GET", url, nil, nil)
		if err != nil {
			return withAppsHint(err, dbMcpHint)
		}
		out := map[string]interface{}{
			idKey:     idValue,
			"name":    common.GetString(data, "name"),
			"enabled": data["enabled"],
		}
		for _, k := range []string{"url", "transport"} {
			if v := common.GetString(data, k); v != "" {
				out[k] = v
			}
		}
		rctx.OutFormat(out, nil, func(w io.Writer) {
			renderDBMcpGetPretty(w, out, idKey, idValue)
		})
		return nil
	},
}

// renderDBMcpGetPretty 左对齐 key:value 明细块（key 名与 JSON 字段同名）；缺席字段不占行。
// 未启用时末尾补一行下一步 —— 「关着」这个正常返回里唯一有价值的可执行信息就是它。
func renderDBMcpGetPretty(w io.Writer, out map[string]interface{}, idKey, idValue string) {
	pairs := [][2]string{
		{idKey, idValue},
		{"name", common.GetString(out, "name")},
		{"enabled", fmt.Sprintf("%v", out["enabled"])},
	}
	for _, k := range []string{"url", "transport"} {
		if v := common.GetString(out, k); v != "" {
			pairs = append(pairs, [2]string{k, v})
		}
	}
	renderKeyValuePairs(w, pairs)
	if enabled, ok := out["enabled"].(bool); ok && !enabled {
		fmt.Fprintf(w, "Enable it with `lark-cli apps +db-mcp-enable --%s %s`.\n", flagNameForIDKey(idKey), idValue)
	}
}

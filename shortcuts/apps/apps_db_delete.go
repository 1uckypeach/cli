// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"fmt"
	"io"

	"github.com/larksuite/cli/shortcuts/common"
)

const dbDeleteHint = "check the --database-id; run `lark-cli apps +db-list` to see the databases you can access"

// AppsDBDelete hard-deletes a standalone database（high-risk-write，框架自动注入 --yes 确认）。
//
// DELETE /databases/{database_id}。硬删、不可恢复（一期不做备份恢复），仅 manage 可调用。
// 不传 --yes 时框架返 confirmation_required / exit 10，由 agent 展示目标库、取得用户确认后重试。
//
// 摘要行只回显 database_id，回显不出库名 —— 一期没有「单库详情」接口，CLI 手上只有调用方传进来的
// id。agent 需要更多上下文时自行用 +db-list 比对，不要在这里替它猜。
//
// 【不收 --app-id】—— 删妙搭 App 用 apps 域既有能力，本命令只删独立 DB，误传由 cobra 拒为 unknown flag。
var AppsDBDelete = common.Shortcut{
	Service:     appsService,
	Command:     "+db-delete",
	Description: "Delete a standalone database (hard delete, not recoverable)",
	Risk:        "high-risk-write",
	Tips: []string{
		"Example: lark-cli apps +db-delete --database-id <database_id> --yes",
		"Hard delete with no recovery path: confirm the target with +db-list before passing --yes.",
	},
	Scopes:    []string{"spark:app:write"},
	AuthTypes: []string{"user"},
	HasFormat: true,
	Flags: []common.Flag{
		{Name: "database-id", Desc: "standalone database id", Required: true},
	},
	Validate: func(ctx context.Context, rctx *common.RuntimeContext) error {
		_, err := requireDatabaseID(rctx.Str("database-id"))
		return err
	},
	DryRun: func(ctx context.Context, rctx *common.RuntimeContext) *common.DryRunAPI {
		databaseID, _ := requireDatabaseID(rctx.Str("database-id"))
		return common.NewDryRunAPI().
			DELETE(databasePath(databaseID)).
			Desc("Delete a standalone database")
	},
	Execute: func(ctx context.Context, rctx *common.RuntimeContext) error {
		databaseID, err := requireDatabaseID(rctx.Str("database-id"))
		if err != nil {
			return err
		}
		data, err := rctx.CallAPITyped("DELETE", databasePath(databaseID), nil, nil)
		if err != nil {
			return withAppsHint(err, dbDeleteHint)
		}
		// database_id 回显调用方传入的值而不是服务端回包：删除是幂等语义上的终态操作，
		// 输出必须让人确认「删掉的正是我指定的那个」。deleted 取服务端结果，不自己伪造 true。
		out := map[string]interface{}{
			"database_id": databaseID,
			"deleted":     data["deleted"],
		}
		rctx.OutFormat(out, nil, func(w io.Writer) {
			renderDBDeletePretty(w, out)
		})
		return nil
	},
}

// renderDBDeletePretty 单行 ✓ 摘要，只带 database_id（没有库名可回显，见类型注释）。
func renderDBDeletePretty(w io.Writer, out map[string]interface{}) {
	fmt.Fprintf(w, "✓ Database deleted: %s\n", common.GetString(out, "database_id"))
}

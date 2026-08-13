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

// AppsDBPermissionRevoke revokes a user's permission on a standalone database.
//
// DELETE /databases/{database_id}/permissions?user_id=。撤销目标用户在该库的【全部】权限。
//
// 【幂等且输出能区分】—— 目标本无权限时服务端返 removed:false，CLI 据此换措辞
// （对齐 +cache-delete 的 "✓ cache deleted" / "✓ cache already absent"）。
// 能区分是因为 PRD 给了 removed 字段；+db-permission-set 没有，所以那边区分不了。
//
// 【创建者不特殊】—— 对外没有 owner 概念，创建者就是一条普通 manage，可以被其他 manage 撤销。
// 唯一护栏是「不得使 DB 失去最后一名 manage」，由服务端判定并返 400002903（exit 2）。
//
// Risk 定 write 而非 high-risk-write：撤权限可由任一 manage 立刻用 +db-permission-set 复原，
// 不像删库那样不可逆；「最后一名 manage」这一不可逆分支已被服务端护栏挡住。
var AppsDBPermissionRevoke = common.Shortcut{
	Service:     appsService,
	Command:     "+db-permission-revoke",
	Description: "Revoke a user's permission on a standalone database",
	Risk:        "write",
	Tips: []string{
		"Example: lark-cli apps +db-permission-revoke --database-id <database_id> --user-id <user_id>",
		"Idempotent: revoking a user who has no permission succeeds with removed:false.",
	},
	Scopes:    []string{"spark:app:write"},
	AuthTypes: []string{"user"},
	HasFormat: true,
	Flags: []common.Flag{
		{Name: "database-id", Desc: "standalone database id", Required: true},
		{Name: "user-id", Desc: "target Lark user id", Required: true},
	},
	Validate: func(ctx context.Context, rctx *common.RuntimeContext) error {
		if _, err := requireDatabaseID(rctx.Str("database-id")); err != nil {
			return err
		}
		return requireDBTargetUserID(rctx)
	},
	DryRun: func(ctx context.Context, rctx *common.RuntimeContext) *common.DryRunAPI {
		databaseID, _ := requireDatabaseID(rctx.Str("database-id"))
		return common.NewDryRunAPI().
			DELETE(databasePermissionsPath(databaseID)).
			Desc("Revoke a user's database permission").
			Params(buildDBPermissionRevokeParams(rctx))
	},
	Execute: func(ctx context.Context, rctx *common.RuntimeContext) error {
		databaseID, err := requireDatabaseID(rctx.Str("database-id"))
		if err != nil {
			return err
		}
		params := buildDBPermissionRevokeParams(rctx)
		data, err := rctx.CallAPITyped("DELETE", databasePermissionsPath(databaseID), params, nil)
		if err != nil {
			return withAppsHint(err, dbPermissionHint)
		}
		// permission 恒为 null：撤销后已无级别可言，显式给出 null 而不是省略该键，
		// 让「撤销成功」的返回体与 +db-permission-set 的三字段形态对齐、agent 不用判键是否存在。
		out := map[string]interface{}{
			"database_id": databaseID,
			"user_id":     params["user_id"],
			"permission":  nil,
			"removed":     data["removed"],
		}
		rctx.OutFormat(out, nil, func(w io.Writer) {
			renderDBPermissionRevokePretty(w, out)
		})
		return nil
	},
}

func buildDBPermissionRevokeParams(rctx *common.RuntimeContext) map[string]interface{} {
	return map[string]interface{}{"user_id": strings.TrimSpace(rctx.Str("user-id"))}
}

// renderDBPermissionRevokePretty 两种成功都打 ✓，只换措辞：真撤销 vs 本就没有权限。
func renderDBPermissionRevokePretty(w io.Writer, out map[string]interface{}) {
	userID := common.GetString(out, "user_id")
	if removed, ok := out["removed"].(bool); ok && removed {
		fmt.Fprintf(w, "✓ Permission revoked: %s\n", userID)
		return
	}
	fmt.Fprintf(w, "✓ No permission to revoke: %s\n", userID)
}

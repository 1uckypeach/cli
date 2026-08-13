// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

// dbPermissionLevels 是对外的两级权限，写进 Enum 后框架在发请求前就会拦掉非法值。
var dbPermissionLevels = []string{"manage", "runtime"}

// AppsDBPermissionSet grants or changes a user's permission on a standalone database.
//
// POST /databases/{database_id}/permissions，body {user_id, permission}。
// 仅 manage 可调用；目标用户已有权限时覆盖为本次传入的值。
//
// 【幂等，但输出无法区分是否真改了】—— 服务端返回体不带变更标记，CLI 无从判断本次是覆盖还是
// 原样，所以 pretty 措辞【不能】像 +cache-delete 那样分「已删 / 本就不存在」。
// 对比 +db-permission-revoke：它有 removed 字段，所以那边能区分。
//
// 【护栏在服务端】—— 「不得使 DB 失去最后一名 manage」由服务端判定并返 400002903
// （validation / failed_precondition, exit 2）。CLI 不预判：它没有成员全貌，抢着判会在并发下判错。
var AppsDBPermissionSet = common.Shortcut{
	Service:     appsService,
	Command:     "+db-permission-set",
	Description: "Grant or change a user's permission on a standalone database",
	Risk:        "write",
	Tips: []string{
		"Example: lark-cli apps +db-permission-set --database-id <database_id> --user-id <user_id> --permission runtime",
		"Idempotent: re-setting the same level succeeds with an identical response.",
	},
	Scopes:    []string{"spark:app:write"},
	AuthTypes: []string{"user"},
	HasFormat: true,
	Flags: []common.Flag{
		{Name: "database-id", Desc: "standalone database id", Required: true},
		{Name: "user-id", Desc: "target Lark user id", Required: true},
		{Name: "permission", Enum: dbPermissionLevels, Desc: "permission level to grant", Required: true},
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
			POST(databasePermissionsPath(databaseID)).
			Desc("Set a user's database permission").
			Body(buildDBPermissionSetBody(rctx))
	},
	Execute: func(ctx context.Context, rctx *common.RuntimeContext) error {
		databaseID, err := requireDatabaseID(rctx.Str("database-id"))
		if err != nil {
			return err
		}
		body := buildDBPermissionSetBody(rctx)
		if _, err := rctx.CallAPITyped("POST", databasePermissionsPath(databaseID), nil, body); err != nil {
			return withAppsHint(err, dbPermissionHint)
		}
		// 三个字段都回显调用方传入值：这是一条「设成什么」的写操作，输出要让人确认
		// 「设的正是我要的那个人、那个级别」，而不是服务端复述了什么。
		out := map[string]interface{}{
			"database_id": databaseID,
			"user_id":     body["user_id"],
			"permission":  body["permission"],
		}
		rctx.OutFormat(out, nil, func(w io.Writer) {
			fmt.Fprintf(w, "✓ Permission set: %s → %s\n",
				common.GetString(out, "user_id"), common.GetString(out, "permission"))
		})
		return nil
	},
}

// buildDBPermissionSetBody 组装 body，DryRun 与 Execute 共用。
func buildDBPermissionSetBody(rctx *common.RuntimeContext) map[string]interface{} {
	return map[string]interface{}{
		"user_id":    strings.TrimSpace(rctx.Str("user-id")),
		"permission": strings.TrimSpace(rctx.Str("permission")),
	}
}

// requireDBTargetUserID 拦掉 --user-id 传空白：cobra 的 Required 只保证「传了这个 flag」。
func requireDBTargetUserID(rctx *common.RuntimeContext) error {
	if strings.TrimSpace(rctx.Str("user-id")) == "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--user-id must not be empty").WithParam("--user-id")
	}
	return nil
}

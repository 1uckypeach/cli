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

const dbCreateHint = "retry `lark-cli apps +db-create`; a failed attempt leaves no partial database behind"

// AppsDBCreate creates a standalone database owned by the calling Lark user.
//
// POST /databases，body {name, description?}。创建者自动获得 manage 权限。
//
// 【不收 --app-id】—— 独立 DB 不挂在任何妙搭 App 下，误传由 cobra 拒为 unknown flag。
// 这不是遗漏：命令面上不给这个 flag，才能让「独立 DB 与 App 无关」这件事在使用时就暴露，
// 而不是接受了再在 Validate 里报一个语义模糊的错。
//
// 【本命令不幂等】—— 服务端不做同名校验，同名再建一次会得到一个新的 database_id、各自占资源。
// agent 不能把它当幂等操作重试。这一点与「建库失败可安全重试」不矛盾：后者是失败路径，
// 服务端保证失败不留半成品（见 dbCreateHint 与 500002901 的 retryable 语义）。
var AppsDBCreate = common.Shortcut{
	Service:     appsService,
	Command:     "+db-create",
	Description: "Create a standalone database (the creator gets manage permission)",
	Risk:        "write",
	Tips: []string{
		"Example: lark-cli apps +db-create --name <name>",
		"Not idempotent: creating with the same --name again yields a NEW database_id; never retry it as an idempotent call.",
		"Tip: grab the id with --jq, e.g. -q '.data.database_id'",
	},
	Scopes:    []string{"spark:app:write"},
	AuthTypes: []string{"user"},
	HasFormat: true,
	Flags: []common.Flag{
		{Name: "name", Desc: "database display name (duplicates are allowed; database_id is the only reliable identifier)", Required: true},
		{Name: "description", Desc: "database description"},
	},
	Validate: func(ctx context.Context, rctx *common.RuntimeContext) error {
		// cobra 的 Required 只保证「传了这个 flag」，--name "" 仍会过；空名字建库无意义，在此拦掉。
		if strings.TrimSpace(rctx.Str("name")) == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--name must not be empty").WithParam("--name")
		}
		return nil
	},
	DryRun: func(ctx context.Context, rctx *common.RuntimeContext) *common.DryRunAPI {
		return common.NewDryRunAPI().
			POST(databasesPath()).
			Desc("Create a standalone database").
			Body(buildDBCreateBody(rctx))
	},
	Execute: func(ctx context.Context, rctx *common.RuntimeContext) error {
		data, err := rctx.CallAPITyped("POST", databasesPath(), nil, buildDBCreateBody(rctx))
		if err != nil {
			return withAppsHint(err, dbCreateHint)
		}
		out := map[string]interface{}{
			"database_id": common.GetString(data, "database_id"),
			"name":        common.GetString(data, "name"),
		}
		rctx.OutFormat(out, nil, func(w io.Writer) {
			renderDBCreatePretty(w, out)
		})
		return nil
	},
}

// buildDBCreateBody 组装建库 body：description 为空时省略该键，不发 "description": ""。
// DryRun 与 Execute 共用本函数，保证 --dry-run 预览与真实请求一字不差 —— 这正是 agent 的自检手段。
func buildDBCreateBody(rctx *common.RuntimeContext) map[string]interface{} {
	body := map[string]interface{}{"name": strings.TrimSpace(rctx.Str("name"))}
	if desc := strings.TrimSpace(rctx.Str("description")); desc != "" {
		body["description"] = desc
	}
	return body
}

// renderDBCreatePretty 单行 ✓ 摘要：name 与 id 都装得进一行，不另起 key:value 明细块。
// 不回显 description —— 摘要行只放定位这个库需要的信息。
func renderDBCreatePretty(w io.Writer, out map[string]interface{}) {
	fmt.Fprintf(w, "✓ Database created: %s (%s)\n",
		common.GetString(out, "name"), common.GetString(out, "database_id"))
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

// ── db 环境 flag：--environment 是唯一受理名；旧名 --env 已移除 ──
//
// 硬改名：标准名 --environment（带默认/枚举）正常注册并受理；旧名 --env 仅注册为隐藏 flag，
// 目的是「传了能被识别并给出清晰报错」而非继续受理——一旦显式传 --env，在 Validate 阶段直接
// 返回 validation 错、指向 --environment。所有 DryRun/Execute 经 dbEnv() 只读 --environment。

// dbEnvFlags 返回环境 flag 对，供各 db 命令 append 进自己的 Flags。
func dbEnvFlags(def string, enum []string, desc string) []common.Flag {
	return []common.Flag{
		{Name: "environment", Default: def, Enum: enum, Desc: desc},
		{Name: "env", Hidden: true, Desc: "removed: use --environment"},
	}
}

// dbEnv 取环境值：只认标准 --environment（含其默认值）；旧名 --env 不再受理（见 rejectLegacyEnvFlag）。
func dbEnv(rctx *common.RuntimeContext) string {
	return rctx.Str("environment")
}

// dbEnvParams 把 env 并入 params：仅当显式指定了环境（非空）才带 env 键；未指定（空）时
// 省略该键，由服务端按应用多环境状态自动选分支（多环境→dev，单环境→online）。与家族对
// 空可选参数的 omit-empty 约定一致——不发空串，wire 上真正不带 env。原样返回同一个 map 便于链式。
func dbEnvParams(rctx *common.RuntimeContext, params map[string]interface{}) map[string]interface{} {
	if env := dbEnv(rctx); env != "" {
		params["env"] = env
	}
	return params
}

// dbEnvBody 把 env 并入请求体顶层：sync_create/sync_update 从 body 读 env（与 config/preview 平级），
// 不是从 query。语义与 dbEnvParams 一致——仅在显式指定环境（非空）时带 env 键，未指定时省略、
// 由服务端按应用多环境状态自动选分支。原样返回同一个 map 便于链式。
func dbEnvBody(rctx *common.RuntimeContext, body map[string]interface{}) map[string]interface{} {
	if env := dbEnv(rctx); env != "" {
		body["env"] = env
	}
	return body
}

// rejectLegacyEnvFlag 在 Validate 阶段拦截已移除的 --env：显式传了就报清晰的 validation 错，指向 --environment。
func rejectLegacyEnvFlag(rctx *common.RuntimeContext) error {
	if rctx.Changed("env") {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"--env is no longer supported; use --environment instead").WithParam("--env")
	}
	return nil
}

// pollUntil 轮询异步任务直到 check 判定终态。async migrate/recovery 用：dataloom 立即返
// task_id/preview_request_id，CLI 自己 poll（避免单连接长挂被网关/SDK 30s 中断）。
// 首次立即 fetch（不睡）；check 返 done→返回；返 err→透传（失败终态）；否则按 interval 间隔重试至 maxWait。
func pollUntil(ctx context.Context, interval, maxWait time.Duration,
	fetch func() (map[string]interface{}, error),
	check func(map[string]interface{}) (done bool, err error)) (map[string]interface{}, error) {
	maxAttempts := int(maxWait / interval)
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	for i := 0; ; i++ {
		data, err := fetch()
		if err != nil {
			return nil, err
		}
		done, cerr := check(data)
		if cerr != nil {
			return nil, cerr
		}
		if done {
			return data, nil
		}
		if i+1 >= maxAttempts {
			// async 任务多半还在服务端推进，poll 超时是可重试的——标 retryable 让 agent 重新轮询而非放弃。
			return nil, errs.NewNetworkError(errs.SubtypeNetworkTimeout, "timed out waiting for completion after %s", maxWait).WithRetryable()
		}
		select {
		case <-ctx.Done():
			return nil, errs.NewNetworkError(errs.SubtypeNetworkTransport, "cancelled while waiting").WithCause(ctx.Err())
		case <-time.After(interval):
		}
	}
}

// URL helpers for the db CLI commands.

// ── 独立数据库（standalone DB）的 URL —— 与 app 维度的 /apps/{app_id}/... 两套并列 ──
//
// 两套路由对应两套 OpenAPI 契约（独立 DB 全程飞书身份、无环境概念），所以路径函数分开写、
// 不在存量 app* 函数里加 if：分开更易 review 与灰度，也避免某一支的改动波及另一支。
//
// 路径段里只有平台发号的 database_id。用户自定义的值（表名等）一律走 query —— EncodePathSegment
// （即 url.PathEscape）挡不住 ".."，让它进路径段会被规范化到上一级、打到别的接口上。

// databasesPath 返回独立数据库集合 URL（POST 建库 / GET 列表）。
func databasesPath() string {
	return apiBasePath + "/databases"
}

// databasePath 返回单个独立数据库 URL（DELETE 删库）。
func databasePath(databaseID string) string {
	return fmt.Sprintf("%s/databases/%s", apiBasePath, validate.EncodePathSegment(databaseID))
}

// databaseTablesPath / databaseTablePath 返回独立数据库的表 URL。
//
// 【表列表是复数 tables，单表详情是单数 table + query】—— 不对称是有意的：单表详情的表名
// 是用户自定义值，不能进路径段（见文件上方说明），所以走 GET /table?table=xxx。
func databaseTablesPath(databaseID string) string {
	return databasePath(databaseID) + "/tables"
}

func databaseTablePath(databaseID string) string {
	return databasePath(databaseID) + "/table"
}

// databaseSQLPath 返回独立数据库 SQL 执行 URL。
func databaseSQLPath(databaseID string) string {
	return databasePath(databaseID) + "/sql_commands"
}

// databaseDataImportPath / databaseDataExportPath 返回独立数据库的数据导入导出 URL。
// 注意与 App 支路的域段差异：App 是 /apps/{id}/db/data_import，独立 DB 没有 db/ 这一段。
func databaseDataImportPath(databaseID string) string {
	return databasePath(databaseID) + "/data_import"
}

func databaseDataExportPath(databaseID string) string {
	return databasePath(databaseID) + "/data_export"
}

// databasePermissionsPath 返回独立数据库权限 URL（GET 列表 / POST 授权 / DELETE 撤销，同路径三方法）。
func databasePermissionsPath(databaseID string) string {
	return databasePath(databaseID) + "/permissions"
}

// databaseMcpPath / appDbMcpPath 返回 DB MCP 的两支路径。
//
// 【域段不对称，故意分成两个函数】：独立 DB 是 /databases/{id}/mcp[/action]，
// 妙搭 App 是 /apps/{id}/db/mcp[/action] —— 多一段 db/。用一个通用 suffix 拼接会拼错 App 支路，
// 这是设计文档专门标出来的坑。
func databaseMcpPath(databaseID, action string) string {
	return databasePath(databaseID) + mcpSuffix(action)
}

func appDbMcpPath(appID, action string) string {
	return fmt.Sprintf("%s/apps/%s/db%s", apiBasePath, validate.EncodePathSegment(appID), mcpSuffix(action))
}

// mcpSuffix 把动作拼成 /mcp 或 /mcp/<action>；action 为空即「取状态」。
func mcpSuffix(action string) string {
	if action == "" {
		return "/mcp"
	}
	return "/mcp/" + action
}

// appTablesPath 返回 app db 表列表 URL（复用存量「获取数据表列表」接口）。
func appTablesPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/tables", apiBasePath, validate.EncodePathSegment(appID))
}

// appTablePath 返回单个 app db 表详情 URL（复用存量「获取数据表详细信息」接口）。
func appTablePath(appID, table string) string {
	return appTablesPath(appID) + "/" + validate.EncodePathSegment(table)
}

// appSQLPath 返回 app db SQL 执行 URL（复用存量「执行 SQL」接口）。
func appSQLPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/sql_commands", apiBasePath, validate.EncodePathSegment(appID))
}

// appDbEnvCreatePath 返回 app db 环境创建 URL（服务端接口名仍为 db_dev_init）。
func appDbEnvCreatePath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db_dev_init", apiBasePath, validate.EncodePathSegment(appID))
}

// ── 多环境发布（env diff/migrate）/ 数据恢复（recovery）/ 配额 路由 ──

// appEnvMigratePath 返回 dev→online 发布（预览/落地共用）URL：db/env_migrate。
func appEnvMigratePath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/env_migrate", apiBasePath, validate.EncodePathSegment(appID))
}

// appEnvMigrateStatusPath 返回发布异步任务状态查询 URL：db/env_migrate_status。
func appEnvMigrateStatusPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/env_migrate_status", apiBasePath, validate.EncodePathSegment(appID))
}

// appRecoveryPath 返回 PITR 数据恢复（预览/落地共用）URL：db/env_recovery。
func appRecoveryPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/env_recovery", apiBasePath, validate.EncodePathSegment(appID))
}

// appRecoveryDiffStatusPath 返回恢复预览（diff）异步状态查询 URL：db/env_recovery_diff_status。
func appRecoveryDiffStatusPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/env_recovery_diff_status", apiBasePath, validate.EncodePathSegment(appID))
}

// appRecoveryApplyStatusPath 返回恢复落地异步状态查询 URL：db/env_recovery_apply_status。
func appRecoveryApplyStatusPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/env_recovery_apply_status", apiBasePath, validate.EncodePathSegment(appID))
}

// appDbQuotaPath 返回 db 配额查询 URL：db/quota。
func appDbQuotaPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/quota", apiBasePath, validate.EncodePathSegment(appID))
}

// appDbSyncCreatePath returns the Base data sync task create/preview URL.
func appDbSyncCreatePath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/sync_create", apiBasePath, validate.EncodePathSegment(appID))
}

// appDbSyncListPath returns the Base data sync task list URL.
func appDbSyncListPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/sync_list", apiBasePath, validate.EncodePathSegment(appID))
}

// appDbSyncTaskPath returns the Base data sync task detail URL.
func appDbSyncTaskPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/sync_task", apiBasePath, validate.EncodePathSegment(appID))
}

// appDbSyncUpdatePath returns the Base data sync task update URL.
func appDbSyncUpdatePath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/sync_update", apiBasePath, validate.EncodePathSegment(appID))
}

// appDbSyncDeletePath returns the Base data sync task delete URL.
func appDbSyncDeletePath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/sync_del", apiBasePath, validate.EncodePathSegment(appID))
}

// appDbSyncActionPath returns a Base data sync task action URL.
func appDbSyncActionPath(appID, action string) string {
	return fmt.Sprintf("%s/apps/%s/db/sync_%s", apiBasePath, validate.EncodePathSegment(appID), validate.EncodePathSegment(action))
}

// ── 变更追溯（changelog / audit）路由 ──

// appChangelogListPath 返回 DDL 变更记录列表 URL：db/changelog_list。
func appChangelogListPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/changelog_list", apiBasePath, validate.EncodePathSegment(appID))
}

// appAuditStatusPath 返回表审计开关状态查询 URL：db/audit_status。
func appAuditStatusPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/audit_status", apiBasePath, validate.EncodePathSegment(appID))
}

// appAuditSetPath 返回表审计开关设置 URL：db/audit_set。
func appAuditSetPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/audit_set", apiBasePath, validate.EncodePathSegment(appID))
}

// appAuditListPath 返回行级审计事件列表 URL：db/audit_list。
func appAuditListPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/audit_list", apiBasePath, validate.EncodePathSegment(appID))
}

// operatorRef 是 operator 的 {id,name}。后端用 JSON 字符串内嵌透传，CLI parse：
// json 输出还原成对象（下游能区分同名用户），pretty 只取 name。
type operatorRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// parseOperator 解析 operator 字符串：空→nil；非 JSON→{raw,raw}；JSON→{id,name}（name 空兜底 id）。
func parseOperator(raw string) *operatorRef {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	if !strings.HasPrefix(s, "{") {
		return &operatorRef{ID: s, Name: s}
	}
	var o operatorRef
	if json.Unmarshal([]byte(s), &o) != nil {
		return &operatorRef{ID: s, Name: s}
	}
	if o.Name == "" {
		o.Name = o.ID
	}
	return &o
}

// operatorName 取 operator 的展示名（pretty），空用 "—"。
func operatorName(op *operatorRef) string {
	if op == nil || op.Name == "" {
		return "—"
	}
	return op.Name
}

// safeParseJSON 把 before/after 的 JSON 字符串还原成结构化对象供下游消费；失败时透传原始串。
func safeParseJSON(s string) interface{} {
	var v interface{}
	if json.Unmarshal([]byte(s), &v) == nil {
		return v
	}
	return s
}

// appDataImportPath 返回 db 数据导入 URL（新增 db/ 域段路由）。
func appDataImportPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/data_import", apiBasePath, validate.EncodePathSegment(appID))
}

// appDataExportPath 返回 db 数据导出 URL（返原始字节）。
func appDataExportPath(appID string) string {
	return fmt.Sprintf("%s/apps/%s/db/data_export", apiBasePath, validate.EncodePathSegment(appID))
}

// appTableRecordsPath 返回数据表记录列表 URL（复用 GetAppTableRecordList，其 total 即符合条件的记录总数）。
func appTableRecordsPath(appID, table string) string {
	return appTablePath(appID, table) + "/records"
}

// resolveDataFormat 由文件扩展名推断数据格式。lark-cli 的 --format 已被框架占用（输出渲染），
// 故数据格式从文件名推断：import 接受 csv/json，export 还接受 sql。
func resolveDataFormat(ext string, allowSQL bool) (string, error) {
	raw := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), ".")
	switch raw {
	case "csv", "json":
		return raw, nil
	case "sql":
		if allowSQL {
			return "sql", nil
		}
	}
	if allowSQL {
		return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "unsupported data format %q (file must end in .csv, .json or .sql)", raw)
	}
	return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "unsupported data format %q (file must end in .csv or .json)", raw)
}

// countDataRows 粗估数据行数（用于导入上限校验、导出兜底计数）。
// csv：非空行数 - 1（表头）；json：顶层数组长度，非数组算 1，解析失败算 0。
func countDataRows(body []byte, format string) int {
	if format == "csv" {
		lines := 0
		for _, ln := range strings.Split(string(body), "\n") {
			if strings.TrimRight(ln, "\r") != "" {
				lines++
			}
		}
		if lines > 0 {
			return lines - 1
		}
		return 0
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err == nil {
		return len(arr)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err == nil {
		return 1
	}
	return 0
}

// requireAppID trims --app-id and rejects blank, returning a uniform validation error.
func requireAppID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "--app-id is required").WithParam("--app-id")
	}
	return id, nil
}

// dbResourceKind 标识一条 db 命令这次作用在哪种资源上。
type dbResourceKind string

const (
	dbResourceApp      dbResourceKind = "app"
	dbResourceDatabase dbResourceKind = "database"
)

// dbResourceFlags 返回「二选一」的标识 flag 对，供支持两种资源的 db 命令 append 进 Flags。
//
// 两个都不带 Required：谁都不是必填，但必须【恰好一个】——这条由 requireDBResource 在
// Validate 阶段用 common.ExactlyOneTyped 保证，报错文案与 params 由框架统一产出。
// 若给 --app-id 挂 Required，cobra 会在二选一判定之前就把只传 --database-id 的调用拒掉。
func dbResourceFlags(appIDDesc, databaseIDDesc string) []common.Flag {
	return []common.Flag{
		{Name: "app-id", Desc: appIDDesc},
		{Name: "database-id", Desc: databaseIDDesc},
	}
}

// requireDBResource 解析「这次作用在 app 还是独立 DB 上」，是存量 db 命令支持独立 DB 的统一入口。
//
// 不改造 requireAppID：它被 30+ 个 app 维度命令引用，只有本次涉及的命令换用本函数，
// 其余命令行为逐字不变。
// 文案与 params 一律由 common.ExactlyOneTyped 产出，不自造 —— 二选一的两种失败
// （都不传 / 都传）在整个 CLI 里必须是同一套信封。
func requireDBResource(rctx *common.RuntimeContext) (dbResourceKind, string, error) {
	if err := common.ExactlyOneTyped(rctx, "app-id", "database-id"); err != nil {
		return "", "", err
	}
	if id := strings.TrimSpace(rctx.Str("database-id")); id != "" {
		// 独立 DB 没有环境概念，显式传 --environment 属于用错了心智模型，早失败好过静默忽略。
		if err := rejectEnvironmentForDatabase(rctx); err != nil {
			return "", "", err
		}
		return dbResourceDatabase, id, nil
	}
	id, err := requireAppID(rctx.Str("app-id"))
	if err != nil {
		return "", "", err
	}
	return dbResourceApp, id, nil
}

// rejectEnvironmentForDatabase 在 --database-id 与 --environment 同时出现时报错。
//
// 判据是 Changed() 而不是 Str() != ""：--environment 在部分命令上带默认值，用取值判断会把
// 「没传、吃到默认值」误判成「显式传了」，于是所有独立 DB 调用都会被拒。
func rejectEnvironmentForDatabase(rctx *common.RuntimeContext) error {
	for _, name := range []string{"environment", "env"} {
		if rctx.Changed(name) {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"--%s is not supported with --database-id: a standalone database has no environments", name).
				WithParam("--" + name)
		}
	}
	return nil
}

// dbEnvParamsFor 按资源类型决定要不要带 env 键。
//
// App 支路行为逐字不变（沿用 dbEnvParams：显式指定才带 env，未指定由服务端自动选分支）；
// 独立 DB 支路【一律不带 env 键】—— 它只有一个库，没有环境维度。
// 显式传 --environment 已在 requireDBResource 里拒掉，这里只负责不把默认值漏出去。
func dbEnvParamsFor(kind dbResourceKind, rctx *common.RuntimeContext, params map[string]interface{}) map[string]interface{} {
	if kind == dbResourceDatabase {
		return params
	}
	return dbEnvParams(rctx, params)
}

// dbHintFor 按资源类型选 hint。现有 hint 文案全是 app 口径（提 --app-id、--environment dev、
// +db-env-create），对独立 DB 逐字都是错的，照抄会把人指向不存在的命令。
func dbHintFor(kind dbResourceKind, appHint, databaseHint string) string {
	if kind == dbResourceDatabase {
		return databaseHint
	}
	return appHint
}

// requireDatabaseID trims --database-id and rejects blank, mirroring requireAppID.
//
// 与 requireAppID 并列而非改造它：requireAppID 被 30+ 个 app 维度命令引用，独立 DB 的命令
// 只认 database_id，两者不共用一个校验器，改错一处会波及整个 apps 域。
// 完全没传该 flag 时由 cobra 的 Required 拦在更前面（信封里没有 identity），这里兜的是
// 显式传了空串 / 纯空格的情况。
func requireDatabaseID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "--database-id is required").WithParam("--database-id")
	}
	return id, nil
}

var dbSyncCodeHints = map[int]string{
	400002477: "Map 'Base 表记录 ID' to a text, single-value, unique target column. " +
		"If the use_existing target table has no such column, first add one with " +
		"+db-execute (e.g. ALTER TABLE <table> ADD COLUMN base_record_id varchar UNIQUE), then rerun preview. " +
		"For create, rerun preview; for update, resubmit the corrected configuration.",
	400002478: "Run +log-list with --keyword <table> to inspect logs. Fix the target with +db-execute or update the streaming task mapping with +db-sync-update, then query the same task again.",
	400002479: "Run +db-sync-get --task-id <task_id> to inspect the completed task, or create a new task with the required mode.",
	400002480: "Verify --task-id and list tasks with +db-sync-list.",
	400002481: "Use a task_id returned by +db-sync-create or +db-sync-list, such as streaming_<id> or batch_<id>.",
	400002482: "Correct source.table in the config, then resubmit: rerun +db-sync-create --preview for a new task, or resubmit the same task_id with +db-sync-update.",
	400002483: "Set target.table.action to 'create', or create the table with +db-execute, then rerun preview.",
}

// dbSyncOnlineDDLCode / dbSyncOnlineDDLSubcode identify the online-branch DDL ban.
// Code 500002776 is generic (many db-sync failures share it), so the online-DDL
// case is only recognized when the stable subcode k_dl_4000001 is also present in
// the server msg — matching the subcode (a backend contract), not the free-text
// English message that follows it. This ban is multi-env-only: shared/single-env
// apps create tables on online fine and never return it.
const (
	dbSyncOnlineDDLCode    = 500002776
	dbSyncOnlineDDLSubcode = "k_dl_4000001"
)

// dbSyncOnlineDDLHint states the product fact rather than framing dev as a retry
// trick: a multi-env app's online branch does not allow direct table creation, so
// table creation must target the dev branch.
const dbSyncOnlineDDLHint = "The online branch does not allow creating tables (DDL) directly — this is how multi-env apps work, not a CLI limit. " +
	"Rerun +db-sync-create with --environment dev to create the table on the dev branch."

// withDBSyncHint attaches data-sync recovery guidance to typed API errors
// without changing the original category, subtype, code, log_id, or cause.
func withDBSyncHint(err error, fallback string) error {
	if err == nil {
		return nil
	}
	p, ok := errs.ProblemOf(err)
	if !ok {
		return err
	}
	if strings.TrimSpace(p.Hint) != "" {
		return err
	}
	// Subcode-specific hint takes precedence over the generic code table: the
	// online-DDL ban shares code 500002776 with unrelated failures, so match the
	// stable subcode in the msg before falling back to per-code guidance.
	if p.Code == dbSyncOnlineDDLCode && strings.Contains(p.Message, dbSyncOnlineDDLSubcode) {
		p.Hint = dbSyncOnlineDDLHint
		return err
	}
	if hint := dbSyncCodeHints[p.Code]; hint != "" {
		p.Hint = hint
		return err
	}
	if fallback != "" {
		p.Hint = fallback
	}
	return err
}

// parseDBSyncConfigFlag parses and validates the local, recoverable portion of
// the db sync config contract. Service-owned checks such as table existence and
// field compatibility are intentionally left to the OpenAPI endpoint.
//
// allowFieldMapsAutoMatch relaxes the field_maps requirement for create-commit:
// when field_maps is absent or an empty array, the server auto-matches fields and
// creates the task (same matching as preview), so the caller may omit them. Update
// passes false — it requires an explicit mapping. Either way, a field_maps array
// that is present but has every mapping disabled is rejected as a suspected mistake.
func parseDBSyncConfigFlag(raw string, requireFieldMaps, allowFieldMapsAutoMatch bool) (map[string]interface{}, error) {
	var cfg map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&cfg); err != nil {
		return nil, dbSyncConfigError("invalid JSON object for --config").WithCause(err)
	}
	if cfg == nil {
		return nil, dbSyncConfigError("--config must be a JSON object")
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, dbSyncConfigError("--config must contain one JSON object")
	}

	if _, ok := cfg["field_map"]; ok {
		return nil, dbSyncConfigError("unsupported key field_map in --config; use field_maps instead").
			WithHint("use field_maps instead")
	}
	if _, ok := cfg["option_mapping"]; ok {
		return nil, dbSyncConfigError("unsupported key option_mapping in --config; use option_mappings instead").
			WithHint("use option_mappings instead")
	}

	mode, ok := stringField(cfg, "mode")
	if !ok || (mode != "batch" && mode != "streaming") {
		return nil, dbSyncConfigError("config.mode must be batch or streaming")
	}

	source, ok := objectField(cfg, "source")
	if !ok {
		return nil, dbSyncConfigError("config.source is required")
	}
	sourceType, ok := stringField(source, "type")
	if !ok || sourceType != "base" {
		return nil, dbSyncConfigError("config.source.type must be base")
	}

	target, ok := objectField(cfg, "target")
	if !ok {
		return nil, dbSyncConfigError("config.target is required")
	}
	targetType, ok := stringField(target, "type")
	if !ok || targetType != "postgresql" {
		return nil, dbSyncConfigError("config.target.type must be postgresql")
	}
	table, ok := objectField(target, "table")
	if !ok {
		return nil, dbSyncConfigError("config.target.table is required")
	}
	tableName, ok := stringField(table, "name")
	if !ok || strings.TrimSpace(tableName) == "" {
		return nil, dbSyncConfigError("config.target.table.name is required")
	}
	action, ok := stringField(table, "action")
	if !ok || (action != "create" && action != "use_existing") {
		return nil, dbSyncConfigError("config.target.table.action must be create or use_existing")
	}
	if schemaOnly, ok := boolField(cfg, "schema_only"); ok && schemaOnly && (mode != "batch" || action != "create") {
		return nil, dbSyncConfigError("config.schema_only=true requires mode=batch and target.table.action=create")
	}
	fieldMaps, fieldMapsPresent := cfg["field_maps"]
	fieldMapsState := classifyFieldMaps(fieldMaps, fieldMapsPresent)
	// A non-array field_maps is a structural error regardless of preview/commit:
	// preview would otherwise forward the malformed shape to the backend.
	if fieldMapsState == dbSyncFieldMapsInvalid {
		return nil, dbSyncConfigError("config.field_maps must be an array")
	}
	if requireFieldMaps {
		switch fieldMapsState {
		case dbSyncFieldMapsDisabled:
			// Mappings present but every one disabled — a suspected mistake, not an
			// intent to auto-match. Reject for both create and update.
			if !allowFieldMapsAutoMatch {
				return nil, dbSyncConfigError("config.field_maps has mappings but all are disabled; enable at least one")
			}
			return nil, dbSyncConfigError("config.field_maps has mappings but all are disabled; enable at least one, or omit field_maps to let the server auto-match")
		case dbSyncFieldMapsAbsent:
			if !allowFieldMapsAutoMatch {
				return nil, dbSyncConfigError("config.field_maps must contain at least one enabled mapping")
			}
			// create-commit: allowed — the server auto-matches and creates the task.
		}
	}
	if err := rejectSingularOptionMappings(fieldMaps); err != nil {
		return nil, err
	}
	return cfg, nil
}

// requireDBSyncSourceTableIdentifiable pre-empts the backend's "source.table.name
// is required when source.base_url has no table" rejection. The source table is
// identifiable when either source.table.name is set (name takes precedence) or the
// base_url carries a ?table=<table_id> query parameter (the only URL form that
// carries a specific table). Both signals are local, so create is blocked before
// submission rather than round-tripping to the backend. Used only by
// +db-sync-create — update reuses the original task's source and is unaffected.
func requireDBSyncSourceTableIdentifiable(cfg map[string]interface{}) error {
	source, _ := objectField(cfg, "source")
	if table, ok := objectField(source, "table"); ok {
		if name, _ := stringField(table, "name"); strings.TrimSpace(name) != "" {
			return nil
		}
	}
	if baseURL, _ := stringField(source, "base_url"); baseURLHasTable(baseURL) {
		return nil
	}
	return dbSyncConfigError("source.table.name is required when source.base_url has no table").
		WithHint("append the table id to base_url as ?table=<table_id>, " +
			"set source.table.name to the Base table name, " +
			"or list tables with `lark-cli base +table-list --base-token <token>` and pick one")
}

// baseURLHasTable reports whether a Base URL carries a specific table via the
// ?table=<table_id> query parameter — the only form that identifies a table
// (confirmed with the backend). A URL that fails to parse is treated as
// carrying no table, leaving the final call to the backend.
func baseURLHasTable(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return strings.TrimSpace(parsed.Query().Get("table")) != ""
}

func dbSyncConfigError(msg string) *errs.ValidationError {
	return errs.NewValidationError(errs.SubtypeInvalidArgument, msg).WithParam("--config")
}

func objectField(m map[string]interface{}, key string) (map[string]interface{}, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	obj, ok := v.(map[string]interface{})
	return obj, ok
}

func stringField(m map[string]interface{}, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func boolField(m map[string]interface{}, key string) (bool, bool) {
	v, ok := m[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// dbSyncFieldMapsState classifies a config's field_maps for validation:
//   - absent:   no field_maps key, or an empty array → server auto-matches
//   - enabled:  at least one mapping not explicitly enabled:false
//   - disabled: mappings present but every one is enabled:false (suspected mistake)
//   - invalid:  field_maps key is present but is not an array
type dbSyncFieldMapsState int

const (
	dbSyncFieldMapsAbsent dbSyncFieldMapsState = iota
	dbSyncFieldMapsEnabled
	dbSyncFieldMapsDisabled
	dbSyncFieldMapsInvalid
)

func classifyFieldMaps(v interface{}, present bool) dbSyncFieldMapsState {
	if !present {
		return dbSyncFieldMapsAbsent
	}
	items, ok := v.([]interface{})
	if !ok {
		return dbSyncFieldMapsInvalid
	}
	if len(items) == 0 {
		return dbSyncFieldMapsAbsent
	}
	for _, item := range items {
		mapping, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if enabled, ok := mapping["enabled"].(bool); ok && !enabled {
			continue
		}
		return dbSyncFieldMapsEnabled
	}
	return dbSyncFieldMapsDisabled
}

func rejectSingularOptionMappings(v interface{}) error {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	for _, item := range items {
		mapping, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := mapping["option_mapping"]; ok {
			return dbSyncConfigError("unsupported key option_mapping in --config field_maps; use option_mappings instead").
				WithHint("use option_mappings instead")
		}
	}
	return nil
}

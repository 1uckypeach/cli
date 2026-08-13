// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/extension/fileio"
	"github.com/larksuite/cli/shortcuts/common"
)

const dbDataExportMaxRows = 5000
const dbDataExportMaxBytes = 1 * 1024 * 1024 // 1 MB

const dbDataExportHint = "verify --app-id and --table; if too large, filter rows with +db-execute (WHERE/LIMIT) and export smaller subsets"

const dbDataExportDatabaseHint = "verify --database-id and --table; if too large, filter rows with +db-execute (WHERE/LIMIT) and export smaller subsets"

// dbExportRecordCountHeader 是独立 DB 支路的行数来源。
// App 支路要先打一次 records 接口取 total（两次请求）；独立 DB 的导出接口把行数放在这个
// response header 里，一次请求同时拿到数据与行数。
const dbExportRecordCountHeader = "x-apaas-record-count"

// AppsDBDataExport 把应用数据表导出到本地文件（csv/json/sql）。
//
// GET /apps/{app_id}/db/data_export，返回原始字节（非 JSON 信封）。
// 行数不随导出文件返回：CLI 原子编排——先查 GetAppTableRecordList 的 total，再导出文件。
// 数据格式由 --output 扩展名推断（默认 csv，缺省输出 <table>.csv）；上限 5000 行 / 1 MB。
var AppsDBDataExport = common.Shortcut{
	Service:     appsService,
	Command:     "+db-data-export",
	Description: "Export rows from a Miaoda app table to a local file (csv/json/sql)",
	Risk:        "read",
	Tips: []string{
		"Example: lark-cli apps +db-data-export --app-id <app_id> --table orders --output ./orders.csv",
		"Format follows the --output extension: .csv / .json / .sql (default csv).",
	},
	Scopes:    []string{"spark:app:read"},
	AuthTypes: []string{"user"},
	HasFormat: true,
	Flags: append(append(
		dbResourceFlags("Miaoda app id (mutually exclusive with --database-id)", "standalone database id (mutually exclusive with --app-id)"),
		common.Flag{Name: "table", Desc: "source table", Required: true},
		common.Flag{Name: "output", Desc: "local output path; extension picks format .csv/.json/.sql (default: <table>.csv)"},
		common.Flag{Name: "limit", Type: "int", Default: "5000", Desc: "max rows to export (1..5000)"},
	), dbEnvFlags("", []string{"dev", "online"}, "source db environment (Miaoda app only; a standalone database has none); leave unset to auto-select (multi-env app uses dev, single-env uses online), or pass dev/online")...),
	Validate: func(ctx context.Context, rctx *common.RuntimeContext) error {
		if _, _, err := requireDBResource(rctx); err != nil {
			return err
		}
		if err := rejectLegacyEnvFlag(rctx); err != nil {
			return err
		}
		if strings.TrimSpace(rctx.Str("table")) == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--table is required").WithParam("--table")
		}
		if n := rctx.Int("limit"); n <= 0 || n > dbDataExportMaxRows {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--limit must be a positive integer ≤ %d", dbDataExportMaxRows).WithParam("--limit")
		}
		if err := rejectOutputTraversal(rctx.Str("output")); err != nil {
			return err
		}
		if _, _, err := exportFormatAndOutput(rctx); err != nil {
			return err
		}
		return nil
	},
	DryRun: func(ctx context.Context, rctx *common.RuntimeContext) *common.DryRunAPI {
		kind, url, params := dbDataExportTarget(rctx)
		return common.NewDryRunAPI().
			GET(url).
			Desc(dbHintFor(kind, "Export Miaoda app table data (raw bytes)", "Export standalone database table data (raw bytes)")).
			Params(params)
	},
	Execute: func(ctx context.Context, rctx *common.RuntimeContext) error {
		kind, id, err := requireDBResource(rctx)
		if err != nil {
			return err
		}
		table := strings.TrimSpace(rctx.Str("table"))
		format, out, err := exportFormatAndOutput(rctx)
		if err != nil {
			return err
		}

		// 行数来源两支不同：
		//   App 支路   —— 先打一次 records 接口取 total（存量行为，两次请求）；
		//   独立 DB 支路 —— 导出响应的 x-apaas-record-count header 带行数，不再多打一次。
		// 两支都保留「取不到就按导出内容数行」的兜底：行数是附带信息，不该让导出本身失败。
		total, totalErr := 0, error(nil)
		if kind == dbResourceApp {
			total, totalErr = queryExportTotal(rctx, id, dbEnv(rctx), table)
		} else {
			totalErr = errExportTotalFromHeader
		}

		_, exportURL, exportParams := dbDataExportTarget(rctx)
		exportQuery := larkcore.QueryParams{}
		for k, v := range exportParams {
			exportQuery[k] = []string{fmt.Sprintf("%v", v)}
		}
		resp, err := rctx.DoAPI(&larkcore.ApiReq{
			HttpMethod:  http.MethodGet,
			ApiPath:     exportURL,
			QueryParams: exportQuery,
		})
		if err != nil {
			return withAppsHint(errs.NewNetworkError(errs.SubtypeNetworkTransport, "export request failed").WithCause(err).WithRetryable(), dbHintFor(kind, dbDataExportHint, dbDataExportDatabaseHint))
		}
		// 成功是原始字节；业务错误网关以 JSON 信封 {code,msg} 返回（以 '{' 开头）。
		if b := bytes.TrimSpace(resp.RawBody); len(b) > 0 && b[0] == '{' {
			if _, cerr := rctx.ClassifyAPIResponse(resp); cerr != nil {
				return withAppsHint(cerr, dbHintFor(kind, dbDataExportHint, dbDataExportDatabaseHint))
			}
		}
		if resp.StatusCode >= 400 {
			return withAppsHint(errs.NewNetworkError(errs.SubtypeNetworkServer, "export failed: HTTP %d", resp.StatusCode).WithRetryable(), dbHintFor(kind, dbDataExportHint, dbDataExportDatabaseHint))
		}
		body := resp.RawBody
		if len(body) > dbDataExportMaxBytes {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "export exceeds 1 MB limit (%d bytes); filter rows with +db-execute (WHERE/LIMIT) and export smaller subsets", len(body))
		}

		saved, err := rctx.FileIO().Save(out, fileio.SaveOptions{
			ContentType:   resp.Header.Get("Content-Type"),
			ContentLength: int64(len(body)),
		}, bytes.NewReader(body))
		if err != nil {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--output: %v", err).WithParam("--output")
		}
		// 独立 DB 支路优先读 header 里的行数；读不到再落到「数导出内容」的兜底。
		if kind == dbResourceDatabase {
			if n, ok := parseExportRecordCount(resp.Header.Get(dbExportRecordCountHeader)); ok {
				total, totalErr = n, nil
			}
		}
		// 行数取自 total（导出最多 limit 行，故取 min）；取不到时按导出内容数行兜底。
		rows := 0
		if totalErr == nil {
			rows = total
			if lim := rctx.Int("limit"); rows > lim {
				rows = lim
			}
		} else {
			rows = countDataRows(body, format)
		}
		resolved, perr := rctx.FileIO().ResolvePath(out)
		if perr != nil || resolved == "" {
			resolved = out
		}
		result := map[string]interface{}{
			"table": table, "output": resolved, "format": format,
			"rows": rows, "size_bytes": saved.Size(),
		}
		rctx.OutFormat(result, nil, func(w io.Writer) {
			fmt.Fprintf(w, "✓ Exported %s → %s (%d rows)\n", table, resolved, rows)
		})
		return nil
	},
}

// queryExportTotal 调 GetAppTableRecordList（page_size=1）取 total（符合条件的记录总数）。
// 该接口与 +db-data-export 同为 spark:app:read scope，避免导出命令被迫升级到写权限。
func queryExportTotal(rctx *common.RuntimeContext, appID, env, table string) (int, error) {
	params := map[string]interface{}{"page_size": 1}
	if env != "" {
		params["env"] = env
	}
	raw, err := rctx.CallAPITyped("GET", appTableRecordsPath(appID, table), params, nil)
	if err != nil {
		return 0, err
	}
	return totalAsInt(raw["total"]), nil
}

// totalAsInt 把 total 解析成 int，兼容 JSON number 与 i64-as-string 两种 wire 形态。
func totalAsInt(v interface{}) int {
	if f, ok := numericAsFloat(v); ok {
		return int(f)
	}
	if s, ok := v.(string); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return n
		}
	}
	return 0
}

// exportFormatAndOutput 由 --output 推断数据格式与落盘路径：
// 给了 --output → 取其扩展名定 format（csv/json/sql）；未给 → 默认 csv、输出 <table>.csv。
func exportFormatAndOutput(rctx *common.RuntimeContext) (format, outPath string, err error) {
	table := strings.TrimSpace(rctx.Str("table"))
	out := strings.TrimSpace(rctx.Str("output"))
	if out == "" {
		return "csv", table + ".csv", nil
	}
	f, ferr := resolveDataFormat(filepath.Ext(out), true)
	if ferr != nil {
		return "", "", ferr
	}
	return f, out, nil
}

// errExportTotalFromHeader 标记「行数还没拿到，等响应 header」——不是错误，只是让下面的
// 兜底分支在 header 也读不到时接管。用哨兵而不是 bool，是为了和 App 支路的 totalErr 同型。
var errExportTotalFromHeader = errors.New("record count pending response header")

// dbDataExportTarget 按标识分派，DryRun 与 Execute 共用。表名两支都走 query。
func dbDataExportTarget(rctx *common.RuntimeContext) (dbResourceKind, string, map[string]interface{}) {
	kind, id, _ := requireDBResource(rctx)
	format, _, _ := exportFormatAndOutput(rctx)
	params := map[string]interface{}{
		"table":  strings.TrimSpace(rctx.Str("table")),
		"format": format,
		"limit":  rctx.Int("limit"),
	}
	if kind == dbResourceDatabase {
		return kind, databaseDataExportPath(id), params
	}
	return kind, appDataExportPath(id), dbEnvParams(rctx, params)
}

// parseExportRecordCount 解析行数 header；缺席或不是非负整数都返回 false 走兜底，
// 不把脏值当行数报出去。
func parseExportRecordCount(raw string) (int, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

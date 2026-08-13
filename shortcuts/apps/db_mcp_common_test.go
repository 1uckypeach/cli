// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"strings"
	"testing"

	"github.com/larksuite/cli/shortcuts/common"
)

// TestDBMcpPaths_DomainSegmentAsymmetry 锁住两支路径的【域段不对称】：
// 独立 DB 是 /databases/{id}/mcp[...]，妙搭 App 多一段 db/。
// 用一个通用 suffix 拼两支会拼错 App 支路，这是设计文档专门标出来的坑。
func TestDBMcpPaths_DomainSegmentAsymmetry(t *testing.T) {
	for _, tc := range []struct{ action, database, app string }{
		{"", "/open-apis/spark/v1/databases/db_1/mcp", "/open-apis/spark/v1/apps/app_1/db/mcp"},
		{"enable", "/open-apis/spark/v1/databases/db_1/mcp/enable", "/open-apis/spark/v1/apps/app_1/db/mcp/enable"},
		{"disable", "/open-apis/spark/v1/databases/db_1/mcp/disable", "/open-apis/spark/v1/apps/app_1/db/mcp/disable"},
	} {
		if got := databaseMcpPath("db_1", tc.action); got != tc.database {
			t.Errorf("databaseMcpPath(%q) = %q want %q", tc.action, got, tc.database)
		}
		if got := appDbMcpPath("app_1", tc.action); got != tc.app {
			t.Errorf("appDbMcpPath(%q) = %q want %q", tc.action, got, tc.app)
		}
	}
}

// assertDBResourceTwoOfOne 是所有「--app-id 或 --database-id 二选一」命令共用的断言：
// 都不传 → at-least-one；都传 → mutually-exclusive。两种失败在整个 CLI 里必须是同一套信封，
// 所以这里只验错误文案的判别词，不各自造断言。
func assertDBResourceTwoOfOne(t *testing.T, sc common.Shortcut, baseArgs []string) {
	t.Helper()
	factory, stdout, _ := newAppsExecuteFactory(t)
	err := runAppsShortcut(t, sc, append([]string{sc.Command, "--as", "user"}, baseArgs...), factory, stdout)
	if err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Errorf("%s: neither identifier must fail with at-least-one, got %v", sc.Command, err)
	}

	factory, stdout, _ = newAppsExecuteFactory(t)
	both := append([]string{sc.Command, "--app-id", "app_1", "--database-id", "db_1", "--as", "user"}, baseArgs...)
	err = runAppsShortcut(t, sc, both, factory, stdout)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("%s: both identifiers must fail with mutually-exclusive, got %v", sc.Command, err)
	}
}

// TestDBResourceCommands_TwoOfOne 覆盖全部 8 条收二选一的命令（MCP 3 + 存量 5）。
func TestDBResourceCommands_TwoOfOne(t *testing.T) {
	for _, tc := range []struct {
		sc   common.Shortcut
		args []string
	}{
		{AppsDBMcpEnable, nil},
		{AppsDBMcpDisable, nil},
		{AppsDBMcpGet, nil},
		{AppsDBExecute, []string{"--sql", "SELECT 1", "--yes"}},
		{AppsDBTableList, nil},
		{AppsDBTableGet, []string{"--table", "t"}},
		{AppsDBDataImport, []string{"--file", "x.csv", "--yes"}},
		{AppsDBDataExport, []string{"--table", "t"}},
	} {
		t.Run(tc.sc.Command, func(t *testing.T) {
			assertDBResourceTwoOfOne(t, tc.sc, tc.args)
		})
	}
}

// TestDBResourceCommands_RejectEnvironmentWithDatabase：独立 DB 没有环境概念，
// --database-id 与 --environment 同时出现必须在 Validate 阶段拒。
//
// 判据是 Changed() 而非取值：--environment 在这些命令上注册了默认值，用取值判断会把
// 「没传、吃到默认值」误判成显式传了，于是所有独立 DB 调用都会被误拒。
func TestDBResourceCommands_RejectEnvironmentWithDatabase(t *testing.T) {
	for _, tc := range []struct {
		sc   common.Shortcut
		args []string
	}{
		{AppsDBExecute, []string{"--sql", "SELECT 1", "--yes"}},
		{AppsDBTableList, nil},
		{AppsDBTableGet, []string{"--table", "t"}},
		{AppsDBDataExport, []string{"--table", "t"}},
	} {
		t.Run(tc.sc.Command, func(t *testing.T) {
			// 不传 --environment：不该因为默认值被误拒（走到后面才因没 mock 而失败，这里只看 Validate）
			factory, stdout, _ := newAppsExecuteFactory(t)
			args := append([]string{tc.sc.Command, "--database-id", "db_1", "--dry-run", "--as", "user"}, tc.args...)
			if err := runAppsShortcut(t, tc.sc, args, factory, stdout); err != nil {
				t.Fatalf("plain --database-id must pass validation, got %v", err)
			}
			// 显式传 --environment：必须拒
			factory, stdout, _ = newAppsExecuteFactory(t)
			args = append([]string{tc.sc.Command, "--database-id", "db_1", "--environment", "dev", "--dry-run", "--as", "user"}, tc.args...)
			err := runAppsShortcut(t, tc.sc, args, factory, stdout)
			if err == nil || !strings.Contains(err.Error(), "--environment") {
				t.Fatalf("--environment with --database-id must be rejected, got %v", err)
			}
		})
	}
}

// assertDBResourceContract 是「--app-id 或 --database-id 二选一」命令的命令面断言。
//
// 与 assertStandaloneDBContract（只认 --database-id）互补：那边断言【不得出现】--app-id，
// 这边断言两个标识【都在、且都不是 Required】——给任一个挂 Required，cobra 会在二选一
// 判定之前就把只传另一个的调用拒掉，二选一直接失效。
func assertDBResourceContract(t *testing.T, sc common.Shortcut, risk, scope string) {
	t.Helper()
	cmd := sc.Command
	if sc.Service != appsService {
		t.Errorf("%s Service=%q want %q", cmd, sc.Service, appsService)
	}
	if sc.Risk != risk {
		t.Errorf("%s Risk=%q want %q", cmd, sc.Risk, risk)
	}
	if len(sc.Scopes) != 1 || sc.Scopes[0] != scope {
		t.Errorf("%s Scopes=%v want [%s]", cmd, sc.Scopes, scope)
	}
	if len(sc.AuthTypes) != 1 || sc.AuthTypes[0] != "user" {
		t.Errorf("%s AuthTypes=%v want [user]", cmd, sc.AuthTypes)
	}
	seen := map[string]bool{}
	for _, f := range sc.Flags {
		if f.Name == "app-id" || f.Name == "database-id" {
			seen[f.Name] = true
			if f.Required {
				t.Errorf("%s --%s must not be Required: cobra would reject the other branch before the two-of-one check runs", cmd, f.Name)
			}
		}
	}
	for _, name := range []string{"app-id", "database-id"} {
		if !seen[name] {
			t.Errorf("%s must accept --%s", cmd, name)
		}
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package sheets

import (
	"os"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/cmdutil"
	_ "github.com/larksuite/cli/internal/vfs/localfileio"
	"github.com/larksuite/cli/shortcuts/common"
	"github.com/spf13/cobra"
)

func newCSVGuardRuntime(csvVal string) *common.RuntimeContext {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("csv", "", "")
	cmd.ParseFlags(nil)
	cmd.Flags().Set("csv", csvVal)
	return &common.RuntimeContext{Cmd: cmd}
}

// TestGuardCSVValueIsNotFilePath covers the existing-file tier: a bare --csv
// value naming a real file is a forgotten "@", and the fix is inlined.
func TestGuardCSVValueIsNotFilePath(t *testing.T) {
	dir := t.TempDir()
	cmdutil.TestChdir(t, dir)
	if err := os.WriteFile("data.csv", []byte("a,b\n1,2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := guardCSVValueIsNotFilePath(newCSVGuardRuntime("data.csv"))
	ve := requireValidation(t, err, "existing file")
	if !strings.Contains(ve.Message, "@data.csv") {
		t.Errorf("message should suggest @data.csv, got: %q", ve.Message)
	}
	if ve.Param != "--csv" {
		t.Errorf("param = %q, want --csv", ve.Param)
	}
}

// TestGuardCSVValueIsNotFilePath_MissingButPathShaped covers the second tier.
// A path that doesn't resolve used to pass through and be written into the
// cell verbatim — a wrong value with a success exit code. The common source is
// an absolute path: `@` rejects those, so the caller drops the `@` and retries.
// Since the file can't be read from cwd, the prescription is stdin.
func TestGuardCSVValueIsNotFilePath_MissingButPathShaped(t *testing.T) {
	dir := t.TempDir()
	cmdutil.TestChdir(t, dir)

	for _, v := range []string{
		"nope.csv",          // relative path from another working directory
		"./missing.csv",     // explicit relative prefix
		"../sibling/x.tsv",  // parent-relative
		"/tmp/nope.csv",     // absolute — the `@`-rejected case
		"~/data.tsv",        // home-relative
		"/var/tmp/export",   // no extension, but an unmistakable path prefix
		"C:/Users/me/a.csv", // windows-style, still ASCII path shape
	} {
		err := guardCSVValueIsNotFilePath(newCSVGuardRuntime(v))
		ve := requireValidation(t, err, "looks like a file path")
		if !strings.Contains(ve.Hint, "--csv @") || !strings.Contains(ve.Hint, "--csv - <") {
			t.Errorf("value %q: hint should offer both @file and stdin, got: %q", v, ve.Hint)
		}
		if !strings.Contains(ve.Hint, v) {
			t.Errorf("value %q: hint should echo the path in the stdin example, got: %q", v, ve.Hint)
		}
	}
}

// TestGuardCSVValueIsNotFilePath_SkipsResolvedInput pins the origin rule that
// makes the shape heuristic safe: a value that arrived via @file / stdin is
// never inspected, however path-shaped its content — so the hint's promise
// that stdin writes such text verbatim actually holds, and a correct
// `--csv @file` invocation can't be re-rejected for its content.
func TestGuardCSVValueIsNotFilePath_SkipsResolvedInput(t *testing.T) {
	dir := t.TempDir()
	cmdutil.TestChdir(t, dir)
	if err := os.WriteFile("data.csv", []byte("a,b\n1,2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, v := range []string{
		"nope.csv", // path-shaped, missing — rejected when inline
		"data.csv", // names an existing file — rejected when inline
	} {
		rctx := newCSVGuardRuntime(v)
		common.TestMarkInputResolved(rctx, "csv")
		if err := guardCSVValueIsNotFilePath(rctx); err != nil {
			t.Errorf("resolved value %q must skip the guard, got: %v", v, err)
		}
	}
}

// TestGuardCSVValueIsNotFilePath_PassesThrough pins what must still reach the
// sheet untouched. The prose cases are why the guard checks a narrow shape
// instead of "contains a filename": an earlier name-shape heuristic rejected
// them. "N/A" and "README.md" pin the two narrowing rules — a slash alone is
// not a path, and a filename alone is not a CSV path.
func TestGuardCSVValueIsNotFilePath_PassesThrough(t *testing.T) {
	dir := t.TempDir()
	cmdutil.TestChdir(t, dir)

	for _, v := range []string{
		"改完记得更新config.json",           // CJK prose ending in a filename
		"remember to update data.csv", // prose mentioning a file
		"a,b\n1,2",                    // multi-cell CSV
		"hello world",
		"N/A",             // slash, but no CSV extension and no path prefix
		"README.md",       // filename shape, not a CSV one
		"report 2026.csv", // has a space: content, not a path
		"",
	} {
		if err := guardCSVValueIsNotFilePath(newCSVGuardRuntime(v)); err != nil {
			t.Errorf("content %q must pass through, got: %v", v, err)
		}
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package sheets

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/credential"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/spf13/cobra"
)

type csvPutScopeResolver struct{ scopes string }

func (r csvPutScopeResolver) ResolveToken(context.Context, credential.TokenSpec) (*credential.TokenResult, error) {
	return &credential.TokenResult{Token: "token", Scopes: r.scopes}, nil
}

func TestCSVPutAuthorizationContract(t *testing.T) {
	for _, identity := range []string{"user", "bot"} {
		if got := CsvPut.ScopesForIdentity(identity); !reflect.DeepEqual(got, []string{"sheets:spreadsheet:write_only"}) {
			t.Fatalf("%s unconditional scopes = %#v", identity, got)
		}
		wantDeclared := []string{"sheets:spreadsheet:write_only", "wiki:node:read"}
		if got := CsvPut.DeclaredScopesForIdentity(identity); !reflect.DeepEqual(got, wantDeclared) {
			t.Fatalf("%s declared scopes = %#v, want %#v", identity, got, wantDeclared)
		}
	}

	parent, _, _, _ := newTestRig(t, CsvPut)
	cmd, _, err := parent.Find([]string{CsvPut.Command})
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Help(); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	for _, want := range []string{"wiki:node:read", "when: --url resolves to a Wiki node", "related parameters: --url"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Help missing %q:\n%s", want, got)
		}
	}
}

func TestCSVPutChecksWikiScopeOnlyOnWikiExecutePath(t *testing.T) {
	t.Run("direct spreadsheet token", func(t *testing.T) {
		factory, stdout, _, registry := cmdutil.TestFactory(t, testConfig(t))
		factory.Credential = credential.NewCredentialProvider(nil, nil, csvPutScopeResolver{scopes: "sheets:spreadsheet:write_only"}, nil)
		registry.Register(toolOutputStub(testToken, "write", `{"updated_cells":1}`))
		parent := &cobra.Command{Use: "sheets", SilenceErrors: true, SilenceUsage: true}
		CsvPut.Mount(parent, factory)
		parent.SetArgs([]string{CsvPut.Command, "--url", testURL, "--sheet-id", testSheetID, "--csv", "a", "--start-cell", "A1", "--as", "user"})
		if err := parent.Execute(); err != nil {
			t.Fatalf("direct execute: %v", err)
		}
		if got := decodeEnvelopeData(t, stdout.String())["updated_cells"]; got != float64(1) {
			t.Fatalf("updated_cells = %#v", got)
		}
	})

	t.Run("wiki URL", func(t *testing.T) {
		factory, _, _, _ := cmdutil.TestFactory(t, testConfig(t))
		factory.Credential = credential.NewCredentialProvider(nil, nil, csvPutScopeResolver{scopes: "sheets:spreadsheet:write_only"}, nil)
		parent := &cobra.Command{Use: "sheets", SilenceErrors: true, SilenceUsage: true}
		CsvPut.Mount(parent, factory)
		parent.SetArgs([]string{CsvPut.Command, "--url", "https://example.feishu.cn/wiki/wikCSVPut", "--sheet-id", testSheetID, "--csv", "a", "--start-cell", "A1", "--as", "user"})
		err := parent.Execute()
		var permission *errs.PermissionError
		if !errors.As(err, &permission) || permission.Identity != "user" || !reflect.DeepEqual(permission.MissingScopes, []string{"wiki:node:read"}) {
			t.Fatalf("wiki execute error = %#v (%v)", permission, err)
		}
	})
}

func TestCSVPutMountedFlagContract(t *testing.T) {
	parent, _, _, _ := newTestRig(t, CsvPut)
	cmd, _, err := parent.Find([]string{CsvPut.Command})
	if err != nil || cmd == nil {
		t.Fatalf("find %s: cmd=%v err=%v", CsvPut.Command, cmd, err)
	}

	startCell := cmd.Flags().Lookup("start-cell")
	rangeFlag := cmd.Flags().Lookup("range")
	csvFlag := cmd.Flags().Lookup("csv")
	allowOverwrite := cmd.Flags().Lookup("allow-overwrite")
	if startCell == nil {
		t.Fatal("missing --start-cell")
	}
	if rangeFlag == nil {
		t.Fatal("missing --range")
	}
	if csvFlag == nil {
		t.Fatal("missing --csv")
	}
	if allowOverwrite == nil {
		t.Fatal("missing --allow-overwrite")
	}
	if startCell.DefValue != "A1" || startCell.Value.Type() != "string" {
		t.Fatalf("--start-cell default/type = %q/%q", startCell.DefValue, startCell.Value.Type())
	}
	if !rangeFlag.Hidden || rangeFlag.DefValue != "" || rangeFlag.Value.Type() != "string" {
		t.Fatalf("--range hidden/default/type = %v/%q/%q", rangeFlag.Hidden, rangeFlag.DefValue, rangeFlag.Value.Type())
	}
	if csvFlag.Value.Type() != "string" {
		t.Fatalf("--csv type = %q", csvFlag.Value.Type())
	}
	if allowOverwrite.DefValue != "true" || allowOverwrite.Value.Type() != "bool" {
		t.Fatalf("--allow-overwrite default/type = %q/%q", allowOverwrite.DefValue, allowOverwrite.Value.Type())
	}

	const (
		oneRequiredAnnotation       = "cobra_annotation_one_required"
		mutuallyExclusiveAnnotation = "cobra_annotation_mutually_exclusive"
	)
	oneStart := startCell.Annotations[oneRequiredAnnotation]
	oneRange := rangeFlag.Annotations[oneRequiredAnnotation]
	if len(oneStart) == 0 || !reflect.DeepEqual(oneStart, oneRange) {
		t.Fatalf("one-required annotations start=%#v range=%#v", oneStart, oneRange)
	}
	mutexStart := startCell.Annotations[mutuallyExclusiveAnnotation]
	mutexRange := rangeFlag.Annotations[mutuallyExclusiveAnnotation]
	if len(mutexStart) == 0 || !reflect.DeepEqual(mutexStart, mutexRange) {
		t.Fatalf("mutually-exclusive annotations start=%#v range=%#v", mutexStart, mutexRange)
	}
	if len(csvFlag.Annotations[cobra.BashCompOneRequiredFlag]) == 0 {
		t.Fatalf("--csv required annotation missing: %#v", csvFlag.Annotations)
	}
}

func TestCSVPutStandaloneRangeAliasAndOverwritePresence(t *testing.T) {
	tests := []struct {
		name               string
		args               []string
		wantAnchor         string
		wantAllowOverwrite any
	}{
		{
			name:       "range alias collapses to top-left and default true is omitted",
			args:       []string{"--range", "Sheet1!B2:H17"},
			wantAnchor: "B2",
		},
		{
			name:               "explicit false is sent",
			args:               []string{"--start-cell", "C3", "--allow-overwrite=false"},
			wantAnchor:         "C3",
			wantAllowOverwrite: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"--url", testURL, "--sheet-id", testSheetID, "--csv", "a,b\n1,2"}
			args = append(args, tt.args...)
			body := parseDryRunBody(t, CsvPut, args)
			input := decodeToolInput(t, body, "set_range_from_csv")
			if input["start_cell"] != tt.wantAnchor {
				t.Fatalf("start_cell = %#v, want %q", input["start_cell"], tt.wantAnchor)
			}
			got, present := input["allow_overwrite"]
			if tt.wantAllowOverwrite == nil {
				if present {
					t.Fatalf("allow_overwrite = %#v, want omitted", got)
				}
			} else if !present || got != tt.wantAllowOverwrite {
				t.Fatalf("allow_overwrite = %#v present=%v, want %#v", got, present, tt.wantAllowOverwrite)
			}
		})
	}
}

func TestCSVPutPresenceNonZeroIgnoresExplicitEmptyAlternative(t *testing.T) {
	body := parseDryRunBody(t, CsvPut, []string{
		"--url", "", "--spreadsheet-token", testToken,
		"--sheet-id", testSheetID, "--sheet-name", "",
		"--csv", "a", "--start-cell", "A1",
	})
	input := decodeToolInput(t, body, "set_range_from_csv")
	if input["excel_id"] != testToken || input["sheet_id"] != testSheetID {
		t.Fatalf("input = %#v, explicit empty alternatives must remain absent", input)
	}
}

func TestCSVPutInputSourcesReachToolInput(t *testing.T) {
	t.Run("file strips BOM", func(t *testing.T) {
		dir := t.TempDir()
		cmdutil.TestChdir(t, dir)
		if err := os.WriteFile("data.csv", []byte("\uFEFFa,b\n1,2"), 0o644); err != nil {
			t.Fatal(err)
		}
		body := parseDryRunBody(t, CsvPut, []string{
			"--url", testURL, "--sheet-id", testSheetID,
			"--csv", "@data.csv", "--start-cell", "A1",
		})
		input := decodeToolInput(t, body, "set_range_from_csv")
		if input["csv"] != "a,b\n1,2" {
			t.Fatalf("csv = %q", input["csv"])
		}
	})

	t.Run("file content that looks like a path is not re-rejected", func(t *testing.T) {
		dir := t.TempDir()
		cmdutil.TestChdir(t, dir)
		if err := os.WriteFile("path-shaped.csv", []byte("nope.csv"), 0o644); err != nil {
			t.Fatal(err)
		}
		body := parseDryRunBody(t, CsvPut, []string{
			"--url", testURL, "--sheet-id", testSheetID,
			"--csv", "@path-shaped.csv", "--start-cell", "A1",
		})
		input := decodeToolInput(t, body, "set_range_from_csv")
		if input["csv"] != "nope.csv" {
			t.Fatalf("csv = %q, want source content verbatim", input["csv"])
		}
	})

	t.Run("stdin strips BOM", func(t *testing.T) {
		f, stdout, _, _ := cmdutil.TestFactory(t, testConfig(t))
		f.IOStreams.In = strings.NewReader("\uFEFFa,b\n3,4")
		parent := &cobra.Command{Use: "sheets"}
		CsvPut.Mount(parent, f)
		parent.SilenceErrors = true
		parent.SilenceUsage = true
		parent.SetArgs([]string{
			CsvPut.Command,
			"--url", testURL, "--sheet-id", testSheetID,
			"--csv", "-", "--start-cell", "A1", "--dry-run",
		})
		if err := parent.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		body := decodeDryRunFirstCall(t, stdout.String())
		input := decodeToolInput(t, body, "set_range_from_csv")
		if input["csv"] != "a,b\n3,4" {
			t.Fatalf("csv = %q", input["csv"])
		}
	})

	t.Run("double-at escapes file syntax", func(t *testing.T) {
		body := parseDryRunBody(t, CsvPut, []string{
			"--url", testURL, "--sheet-id", testSheetID,
			"--csv", "@@literal", "--start-cell", "A1",
		})
		input := decodeToolInput(t, body, "set_range_from_csv")
		if input["csv"] != "@literal" {
			t.Fatalf("csv = %q, want literal @ prefix", input["csv"])
		}
	})
}

func TestCSVPutTypedToolErrorPreservesUpstreamClassification(t *testing.T) {
	stub := &httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/sheet_ai/v2/spreadsheets/" + testToken + "/tools/invoke_write",
		Body: map[string]interface{}{
			"code": 99991400,
			"msg":  "rate limited",
			"data": map[string]interface{}{},
		},
	}
	stdout, err := runShortcutWithStubs(t, CsvPut, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--csv", "a",
		"--start-cell", "A1",
		"--as", "user",
	}, stub)
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty on tool error", stdout)
	}
	problem := requireProblem(t, err, errs.CategoryAPI, errs.SubtypeRateLimit, "set_range_from_csv")
	if !problem.Retryable || problem.Code != 99991400 {
		t.Fatalf("problem = %#v, want retryable code 99991400", problem)
	}
}

func TestCSVPutExecuteRequestAndOutputContract(t *testing.T) {
	stub := toolOutputStub(testToken, "write", `{"updated_cells":4}`)
	out, err := runShortcutWithStubs(t, CsvPut, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--csv", "a,b\n1,2",
		"--start-cell", "B2",
		"--format", "table", // Legacy Out accepted --format but still emitted JSON.
		"--as", "user",
	}, stub)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	var wireBody struct {
		ToolName string `json:"tool_name"`
		Input    string `json:"input"`
	}
	if err := json.Unmarshal(stub.CapturedBody, &wireBody); err != nil {
		t.Fatalf("decode wire body: %v", err)
	}
	if wireBody.ToolName != "set_range_from_csv" {
		t.Fatalf("tool_name = %q", wireBody.ToolName)
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(wireBody.Input), &input); err != nil {
		t.Fatalf("decode tool input: %v", err)
	}
	wantInput := map[string]any{
		"excel_id": testToken, "sheet_id": testSheetID,
		"csv": "a,b\n1,2", "start_cell": "B2",
	}
	if !reflect.DeepEqual(input, wantInput) {
		t.Fatalf("input = %#v, want %#v", input, wantInput)
	}

	data := decodeEnvelopeData(t, out)
	if data["updated_cells"] != float64(4) || data["writes_range"] != "B2:C3" {
		t.Fatalf("data = %#v", data)
	}
}

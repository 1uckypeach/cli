// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package application

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/spf13/cobra"
)

func deleteOKStub(id string) *httpmock.Stub {
	return &httpmock.Stub{
		Method: "DELETE",
		URL:    slashCommandBasePath + "/" + id,
		Body:   map[string]interface{}{"code": 0, "msg": "success", "data": map[string]interface{}{}},
	}
}

func TestSlashCommandDelete_DeclaredScopesKeepReadConditional(t *testing.T) {
	for _, identity := range []string{"user", "bot"} {
		if got := SlashCommandDelete.ScopesForIdentity(identity); !reflect.DeepEqual(got, []string{"application:app_slash_command:write"}) {
			t.Fatalf("%s unconditional scopes = %#v", identity, got)
		}
		wantDeclared := []string{"application:app_slash_command:write", "application:app_slash_command:read"}
		if got := SlashCommandDelete.DeclaredScopesForIdentity(identity); !reflect.DeepEqual(got, wantDeclared) {
			t.Fatalf("%s declared scopes = %#v, want %#v", identity, got, wantDeclared)
		}
	}
}

func TestSlashCommandDelete_AuthorizationHelp(t *testing.T) {
	factory, _, _, _ := cmdutil.TestFactory(t, appTestConfig())
	service := &cobra.Command{Use: "application"}
	SlashCommandDelete.Mount(service, factory)
	command, _, err := service.Find([]string{SlashCommandDelete.Command})
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	command.SetOut(&output)
	command.SetErr(&output)
	if err := command.Help(); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	for _, want := range []string{"application:app_slash_command:write", "application:app_slash_command:read", "when: --command is used and the command must be resolved by name", "related parameters: --command"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Help missing %q:\n%s", want, got)
		}
	}
}

func TestSlashCommandDelete_RequiresYes(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, appTestConfig())
	err := mountAndRun(t, SlashCommandDelete, []string{"+slash-command-delete",
		"--command-id", "id1", "--as", "bot"}, f, stdout)
	if err == nil {
		t.Fatal("expected confirmation_required without --yes")
	}
	if errs.CategoryOf(err) != errs.CategoryConfirmation {
		t.Fatalf("expected confirmation category, got %v (%v)", errs.CategoryOf(err), err)
	}
}

func TestSlashCommandDelete_ByIDWithYes(t *testing.T) {
	f, stdout, _, reg := cmdutil.TestFactory(t, appTestConfig())
	reg.Register(deleteOKStub("id1"))

	err := mountAndRun(t, SlashCommandDelete, []string{"+slash-command-delete",
		"--command-id", "id1", "--yes", "--format", "json", "--as", "bot"}, f, stdout)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	data := got["data"].(map[string]interface{})
	// 上游 DELETE 返回空对象；CLI 必须补 action/command_id（写操作返回资源 ID）
	if data["action"] != "deleted" || data["command_id"] != "id1" {
		t.Fatalf("data = %v", data)
	}
}

func TestSlashCommandDelete_PrettyOutputAndStderrContract(t *testing.T) {
	f, stdout, stderr, reg := cmdutil.TestFactory(t, appTestConfig())
	reg.Register(deleteOKStub("id1"))

	err := mountAndRun(t, SlashCommandDelete, []string{"+slash-command-delete",
		"--command-id", "id1", "--yes", "--format", "pretty", "--as", "bot"}, f, stdout)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if stdout.String() != "deleted command_id id1\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	wantStderr := clientCacheHint + "\n" + "note: recreating the same command name will yield a NEW command_id.\n"
	if stderr.String() != wantStderr {
		t.Fatalf("stderr = %q, want %q", stderr.String(), wantStderr)
	}
}

func TestSlashCommandDelete_ByNameWithYes(t *testing.T) {
	f, stdout, _, reg := cmdutil.TestFactory(t, appTestConfig())
	reg.Register(listStub([]interface{}{sampleItem("greet", "id7")}))
	reg.Register(deleteOKStub("id7"))

	err := mountAndRun(t, SlashCommandDelete, []string{"+slash-command-delete",
		"--command", "greet", "--yes", "--format", "json", "--as", "bot"}, f, stdout)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	data := got["data"].(map[string]interface{})
	if data["command"] != "greet" || data["command_id"] != "id7" {
		t.Fatalf("data = %v", data)
	}
}

func TestSlashCommandDelete_ByNameDryRun(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, appTestConfig())

	err := mountAndRun(t, SlashCommandDelete, []string{"+slash-command-delete",
		"--command", "greet", "--dry-run", "--as", "bot"}, f, stdout)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var envlp struct {
		Data struct {
			Description string `json:"description"`
			API         []struct {
				Desc   string `json:"desc"`
				Method string `json:"method"`
			} `json:"api"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envlp); err != nil {
		t.Fatalf("json: %v", err)
	}
	got := envlp.Data
	if !strings.Contains(got.Description, "HIGH-RISK") || strings.Contains(got.Description, "resolve command_id") {
		t.Fatalf("top-level description must contain only the risk context: %q", got.Description)
	}
	if len(got.API) != 2 || got.API[0].Method != "GET" || !strings.Contains(got.API[0].Desc, "resolve command_id") {
		t.Fatalf("first call must describe name resolution: %#v", got.API)
	}
	if got.API[1].Method != "DELETE" || strings.Contains(got.API[1].Desc, "resolve command_id") {
		t.Fatalf("second call must be the delete without the resolve description: %#v", got.API)
	}
}

func TestSlashCommandDelete_Validate(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, appTestConfig())
	for _, args := range [][]string{
		// The missing-target validation must win over high-risk confirmation.
		{"+slash-command-delete", "--as", "bot"},
		{"+slash-command-delete", "--command-id", "id1", "--command", "greet", "--yes", "--as", "bot"},
	} {
		err := mountAndRun(t, SlashCommandDelete, args, f, stdout)
		if err == nil {
			t.Errorf("%v: expected validation error", args)
			continue
		}
		p, ok := errs.ProblemOf(err)
		if !ok || p.Category != errs.CategoryValidation || p.Subtype != errs.SubtypeInvalidArgument {
			t.Errorf("%v: expected validation problem, got %v", args, err)
		}
	}
}

func TestSlashCommandDelete_ByIDEncodesTrimmedPathSegment(t *testing.T) {
	f, stdout, _, reg := cmdutil.TestFactory(t, appTestConfig())
	reg.Register(deleteOKStub("id%2Fwith%20space%3Fx"))

	err := mountAndRun(t, SlashCommandDelete, []string{"+slash-command-delete",
		"--command-id", " id/with space?x ", "--yes", "--as", "bot"}, f, stdout)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package base

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/shortcuts/common"
)

func TestBaseFormDeleteTypedContractPreservesMetadata(t *testing.T) {
	if BaseFormDelete.Execute != nil || BaseFormDelete.DryRun != nil {
		t.Fatal("form-delete must keep Execute and DryRun in the compiled Typed hooks")
	}
	if got := strings.Join(BaseFormDelete.AuthTypes, ","); got != "user,bot" {
		t.Fatalf("auth types=%q, want %q", got, "user,bot")
	}
	for _, identity := range []string{"user", "bot"} {
		if got := strings.Join(BaseFormDelete.ScopesForIdentity(identity), ","); got != "base:form:delete" {
			t.Fatalf("%s scopes=%q, want %q", identity, got, "base:form:delete")
		}
	}
	if BaseFormDelete.Risk != "high-risk-write" {
		t.Fatalf("risk=%q, want high-risk-write", BaseFormDelete.Risk)
	}
	if len(BaseFormDelete.Flags) != 3 {
		t.Fatalf("flags=%#v", BaseFormDelete.Flags)
	}
	for index, name := range []string{"base-token", "table-id", "form-id"} {
		if BaseFormDelete.Flags[index].Name != name || !BaseFormDelete.Flags[index].Required {
			t.Fatalf("flag[%d]=%#v, want required --%s", index, BaseFormDelete.Flags[index], name)
		}
	}
}

func TestBaseV3TypedRequestOptionsPreserveLegacyHeaderOverwrite(t *testing.T) {
	apply := func(options []larkcore.RequestOptionFunc) *larkcore.RequestOption {
		result := &larkcore.RequestOption{}
		for _, option := range options {
			option(result)
		}
		return result
	}

	withoutShortcut := apply(baseV3TypedRequestOptions(context.Background(), "cli_test"))
	if got := withoutShortcut.Header.Get("X-App-Id"); got != "cli_test" {
		t.Fatalf("without shortcut metadata X-App-Id=%q, want %q", got, "cli_test")
	}

	ctx := cmdutil.ContextWithShortcut(context.Background(), "base:+form-delete", "exec-1")
	withShortcut := apply(baseV3TypedRequestOptions(ctx, "cli_test"))
	if got := withShortcut.Header.Get("X-App-Id"); got != "" {
		t.Fatalf("with shortcut metadata X-App-Id=%q, want absent to preserve Legacy wire behavior", got)
	}
	if got := withShortcut.Header.Get(cmdutil.HeaderShortcut); got != "base:+form-delete" {
		t.Fatalf("%s=%q, want %q", cmdutil.HeaderShortcut, got, "base:+form-delete")
	}
	if got := withShortcut.Header.Get(cmdutil.HeaderExecutionId); got != "exec-1" {
		t.Fatalf("%s=%q, want %q", cmdutil.HeaderExecutionId, got, "exec-1")
	}
}

func TestBaseV3TypedClassifyContextIsComplete(t *testing.T) {
	got := baseV3TypedClassifyContext(core.CliConfig{Brand: core.BrandLark, AppID: "cli_test"}, common.IdentityBot, "base +form-delete")
	if got.Brand != "lark" || got.AppID != "cli_test" || got.Identity != "bot" || got.LarkCmd != "base +form-delete" {
		t.Fatalf("classify context=%#v", got)
	}
}

func TestClassifyAndDecodeBaseV3ResponsePreservesJSONNumbers(t *testing.T) {
	resp := &larkcore.ApiResp{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		RawBody:    []byte(`{"code":0,"data":{"large":9007199254740993}}`),
	}
	result, err := classifyAndDecodeBaseV3Response(resp, baseV3TypedClassifyContext(core.CliConfig{}, common.IdentityBot, "base +form-delete"))
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	data, _ := result["data"].(map[string]interface{})
	large, ok := data["large"].(json.Number)
	if !ok || large.String() != "9007199254740993" {
		t.Fatalf("large=%#v (%T), want exact json.Number", data["large"], data["large"])
	}
}

func TestBaseFormDeletePermissionClassificationUsesTypedContext(t *testing.T) {
	factory, stdout, reg := newExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "DELETE",
		URL:    "/open-apis/base/v3/bases/app_x/tables/tbl_x/forms/vew_form1",
		Body: map[string]interface{}{
			"code": 99991672,
			"msg":  "app scope not applied",
			"error": map[string]interface{}{
				"permission_violations": []interface{}{map[string]interface{}{"subject": "base:form:delete"}},
			},
		},
	})
	err := runShortcut(t, BaseFormDelete, []string{"+form-delete", "--base-token", "app_x", "--table-id", "tbl_x", "--form-id", "vew_form1", "--yes"}, factory, stdout)
	var permissionErr *errs.PermissionError
	if !errors.As(err, &permissionErr) {
		t.Fatalf("expected PermissionError, got %T: %v", err, err)
	}
	if permissionErr.Subtype != errs.SubtypeAppScopeNotApplied || permissionErr.Identity != "bot" {
		t.Fatalf("permission error=%#v", permissionErr)
	}
	if len(permissionErr.MissingScopes) != 1 || permissionErr.MissingScopes[0] != "base:form:delete" {
		t.Fatalf("missing scopes=%v", permissionErr.MissingScopes)
	}
	if !strings.Contains(permissionErr.ConsoleURL, "open.feishu.cn/page/scope-apply") || !strings.Contains(permissionErr.ConsoleURL, "clientID=test-app-") {
		t.Fatalf("console URL=%q", permissionErr.ConsoleURL)
	}
}

func TestBaseFormDeletePreservesBaseErrorEnrichment(t *testing.T) {
	factory, stdout, reg := newExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method: "DELETE",
		URL:    "/open-apis/base/v3/bases/app_x/tables/tbl_x/forms/vew_form1",
		Body: map[string]interface{}{
			"code": 1254001,
			"msg":  "fallback message",
			"data": map[string]interface{}{"error": map[string]interface{}{
				"message": "precise Base failure", "hint": "retry with an existing form ID",
			}},
		},
		Headers: http.Header{"X-Tt-Logid": []string{"base-log-1"}},
	})
	err := runShortcut(t, BaseFormDelete, []string{"+form-delete", "--base-token", "app_x", "--table-id", "tbl_x", "--form-id", "vew_form1", "--yes"}, factory, stdout)
	problem, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("expected typed error, got %T: %v", err, err)
	}
	if problem.Message != "precise Base failure" || problem.Hint != "retry with an existing form ID" || problem.LogID != "base-log-1" {
		t.Fatalf("problem=%#v", problem)
	}
}

func TestBaseFormDeletePreservesHTTPServerClassification(t *testing.T) {
	factory, stdout, reg := newExecuteFactory(t)
	reg.Register(&httpmock.Stub{
		Method:  "DELETE",
		URL:     "/open-apis/base/v3/bases/app_x/tables/tbl_x/forms/vew_form1",
		Status:  http.StatusServiceUnavailable,
		RawBody: []byte("temporarily unavailable"),
		Headers: http.Header{"Content-Type": []string{"text/plain"}, "X-Tt-Logid": []string{"base-log-503"}},
	})
	err := runShortcut(t, BaseFormDelete, []string{"+form-delete", "--base-token", "app_x", "--table-id", "tbl_x", "--form-id", "vew_form1", "--yes"}, factory, stdout)
	problem, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("expected typed error, got %T: %v", err, err)
	}
	if problem.Category != errs.CategoryNetwork || problem.Subtype != errs.SubtypeNetworkServer || !problem.Retryable || problem.LogID != "base-log-503" {
		t.Fatalf("problem=%#v", problem)
	}
}

func TestBaseFormDeleteConfirmationRunsBeforeAPI(t *testing.T) {
	factory, stdout, _ := newExecuteFactory(t)
	err := runShortcut(t, BaseFormDelete, []string{"+form-delete", "--base-token", "app_x", "--table-id", "tbl_x", "--form-id", "vew_form1"}, factory, stdout)
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Subtype != errs.SubtypeConfirmationRequired {
		t.Fatalf("expected confirmation_required, got %T: %v", err, err)
	}
}

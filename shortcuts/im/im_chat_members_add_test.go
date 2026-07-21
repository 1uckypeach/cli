// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/shortcuts/common"
	"github.com/spf13/cobra"
)

func TestCollectMemberAddIDs(t *testing.T) {
	cases := []struct {
		name    string
		flag    string
		prefix  string
		max     int
		raw     []string
		want    []string
		wantErr bool
	}{
		{"empty is ok", "users", "ou_", chatMembersAddMaxUsers, nil, []string{}, false},
		{"dedupes", "users", "ou_", chatMembersAddMaxUsers, []string{"ou_a", "ou_a", "ou_b"}, []string{"ou_a", "ou_b"}, false},
		{"trims whitespace", "users", "ou_", chatMembersAddMaxUsers, []string{" ou_a ", ""}, []string{"ou_a"}, false},
		{"wrong prefix", "users", "ou_", chatMembersAddMaxUsers, []string{"cli_a"}, nil, true},
		{"over limit", "bots", "cli_", 2, []string{"cli_a", "cli_b", "cli_c"}, nil, true},
	}
	for _, c := range cases {
		got, err := collectMemberAddIDs(newBareTestRuntime(t, map[string][]string{c.flag: c.raw}), c.flag, c.prefix, c.max)
		if c.wantErr {
			var ve *errs.ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("%s: want *errs.ValidationError, got %T (%v)", c.name, err, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error %v", c.name, err)
		}
		if !equalStringSlices(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestValidateChatMembersAdd(t *testing.T) {
	cases := []struct {
		name      string
		chatID    string
		users     []string
		bots      []string
		wantErr   bool
		wantParam string
	}{
		{"valid users only", "oc_x", []string{"ou_a"}, nil, false, ""},
		{"valid bots only", "oc_x", nil, []string{"cli_a"}, false, ""},
		{"valid both", "oc_x", []string{"ou_a"}, []string{"cli_a"}, false, ""},
		{"missing chat-id", "", []string{"ou_a"}, nil, true, "--chat-id"},
		{"bad chat-id prefix", "abc", []string{"ou_a"}, nil, true, "--chat-id"},
		{"neither users nor bots", "oc_x", nil, nil, true, ""},
	}
	for _, c := range cases {
		rt := newChatMembersAddTestRuntime(t, nil, map[string]string{"chat-id": c.chatID}, map[string][]string{"users": c.users, "bots": c.bots})
		err := validateChatMembersAdd(rt)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: want error, got nil", c.name)
				continue
			}
			if c.wantParam != "" {
				assertValidationError(t, c.name, err, c.wantParam)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
		}
	}
}

// newBareTestRuntime builds a runtime with only string_slice flags registered
// (no HTTP transport needed) for pure-function tests like collectMemberAddIDs.
func newBareTestRuntime(t *testing.T, slices map[string][]string) *common.RuntimeContext {
	t.Helper()
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
	}))
	cmd := &cobra.Command{Use: "test"}
	for flag := range slices {
		cmd.Flags().StringSlice(flag, nil, "")
	}
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	for flag, vals := range slices {
		for _, v := range vals {
			if err := cmd.Flags().Set(flag, v); err != nil {
				t.Fatalf("set %s: %v", flag, err)
			}
		}
	}
	rt.Cmd = cmd
	return rt
}

// newChatMembersAddTestRuntime wires --chat-id/--users/--bots for the full
// Validate/DryRun/Execute surface.
func newChatMembersAddTestRuntime(t *testing.T, rtRoundTripper http.RoundTripper, str map[string]string, slices map[string][]string) *common.RuntimeContext {
	t.Helper()
	if rtRoundTripper == nil {
		rtRoundTripper = shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
		})
	}
	runtime := newUserShortcutRuntime(t, rtRoundTripper)
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("chat-id", "", "")
	cmd.Flags().StringSlice("users", nil, "")
	cmd.Flags().StringSlice("bots", nil, "")
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	for k, v := range str {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}
	for flag, vals := range slices {
		for _, v := range vals {
			if err := cmd.Flags().Set(flag, v); err != nil {
				t.Fatalf("set %s: %v", flag, err)
			}
		}
	}
	runtime.Cmd = cmd
	return runtime
}

func TestAddChatMembersBatch_AllSucceed(t *testing.T) {
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return shortcutJSONResponse(200, map[string]interface{}{"code": 0, "data": map[string]interface{}{}}), nil
	}))
	res := newChatMembersAddResult()
	addChatMembersBatch(rt, "oc_x", "user", "open_id", []string{"ou_a", "ou_b"}, res)

	if !equalStringSlices(res.succeeded, []string{"ou_a", "ou_b"}) {
		t.Errorf("succeeded = %v, want [ou_a ou_b]", res.succeeded)
	}
	if len(res.invalid) != 0 || len(res.notExisted) != 0 || len(res.pendingApproval) != 0 || len(res.callErrors) != 0 {
		t.Errorf("expected no failures, got %+v", res)
	}
}

func TestAddChatMembersBatch_PartialFailure(t *testing.T) {
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return shortcutJSONResponse(200, map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"invalid_id_list":          []interface{}{"ou_c"},
				"not_existed_id_list":      []interface{}{"ou_d"},
				"pending_approval_id_list": []interface{}{"ou_e"},
			},
		}), nil
	}))
	res := newChatMembersAddResult()
	addChatMembersBatch(rt, "oc_x", "user", "open_id", []string{"ou_a", "ou_b", "ou_c", "ou_d", "ou_e"}, res)

	if !equalStringSlices(res.succeeded, []string{"ou_a", "ou_b"}) {
		t.Errorf("succeeded = %v, want [ou_a ou_b]", res.succeeded)
	}
	if !equalStringSlices(res.invalid, []string{"ou_c"}) {
		t.Errorf("invalid = %v, want [ou_c]", res.invalid)
	}
	if !equalStringSlices(res.notExisted, []string{"ou_d"}) {
		t.Errorf("notExisted = %v, want [ou_d]", res.notExisted)
	}
	if !equalStringSlices(res.pendingApproval, []string{"ou_e"}) {
		t.Errorf("pendingApproval = %v, want [ou_e]", res.pendingApproval)
	}
}

func TestAddChatMembersBatch_CallLevelFailure(t *testing.T) {
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return shortcutJSONResponse(400, map[string]interface{}{"code": 123, "msg": "bot count exceeds chat limit"}), nil
	}))
	res := newChatMembersAddResult()
	addChatMembersBatch(rt, "oc_x", "bot", "app_id", []string{"cli_y"}, res)

	if len(res.succeeded) != 0 {
		t.Errorf("succeeded = %v, want empty (call failed)", res.succeeded)
	}
	if len(res.callErrors) != 1 {
		t.Fatalf("callErrors = %v, want 1 entry", res.callErrors)
	}
	ce := res.callErrors[0]
	if ce["member_type"] != "bot" {
		t.Errorf("call_errors[0].member_type = %v, want bot", ce["member_type"])
	}
	ids, _ := ce["id_list"].([]string)
	if !equalStringSlices(ids, []string{"cli_y"}) {
		t.Errorf("call_errors[0].id_list = %v, want [cli_y]", ids)
	}
}

func TestImChatMembersAddExecute_AllSucceed(t *testing.T) {
	var gotPaths []string
	rt := newChatMembersAddTestRuntime(t,
		shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotPaths = append(gotPaths, req.URL.Path+"?"+req.URL.RawQuery)
			return shortcutJSONResponse(200, map[string]interface{}{"code": 0, "data": map[string]interface{}{}}), nil
		}),
		map[string]string{"chat-id": "oc_x"},
		map[string][]string{"users": {"ou_a"}, "bots": {"cli_x"}},
	)

	err := ImChatMembersAdd.Execute(context.Background(), rt)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(gotPaths) != 2 {
		t.Fatalf("want 2 API calls (users + bots), got %d: %v", len(gotPaths), gotPaths)
	}

	out := rt.Factory.IOStreams.Out.(interface{ String() string }).String()
	if !strings.Contains(out, `"ok": true`) {
		t.Errorf("output = %s, want ok:true", out)
	}
	if !strings.Contains(out, `"success_count": 2`) {
		t.Errorf("output = %s, want success_count 2", out)
	}
}

func TestImChatMembersAddExecute_PartialFailure(t *testing.T) {
	rt := newChatMembersAddTestRuntime(t,
		shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Query().Get("member_id_type") == "open_id" {
				return shortcutJSONResponse(200, map[string]interface{}{
					"code": 0,
					"data": map[string]interface{}{"invalid_id_list": []interface{}{"ou_c"}},
				}), nil
			}
			return shortcutJSONResponse(200, map[string]interface{}{"code": 0, "data": map[string]interface{}{}}), nil
		}),
		map[string]string{"chat-id": "oc_x"},
		map[string][]string{"users": {"ou_a", "ou_c"}, "bots": {"cli_x"}},
	)

	err := ImChatMembersAdd.Execute(context.Background(), rt)
	var pfErr *output.PartialFailureError
	if !errors.As(err, &pfErr) {
		t.Fatalf("Execute() error = %T %v, want partial failure", err, err)
	}
	if pfErr.Code != output.ExitAPI {
		t.Fatalf("partial failure exit code = %d, want %d (ExitAPI)", pfErr.Code, output.ExitAPI)
	}

	out := rt.Factory.IOStreams.Out.(interface{ String() string }).String()
	if !strings.Contains(out, `"ok": false`) {
		t.Errorf("output = %s, want ok:false", out)
	}
	if !strings.Contains(out, `"failure_count": 1`) {
		t.Errorf("output = %s, want failure_count 1", out)
	}
	if !strings.Contains(out, `"success_count": 2`) {
		t.Errorf("output = %s, want success_count 2", out)
	}
}

func TestImChatMembersAddDryRun_TwoCallsWhenBothFlags(t *testing.T) {
	rt := newChatMembersAddTestRuntime(t, nil,
		map[string]string{"chat-id": "oc_x"},
		map[string][]string{"users": {"ou_a"}, "bots": {"cli_x"}},
	)
	dry := ImChatMembersAdd.DryRun(context.Background(), rt)
	b, err := json.Marshal(dry)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Count(string(b), `"method":"POST"`) != 2 {
		t.Errorf("dry-run json = %s, want 2 POST calls", b)
	}
}

// TestImChatMembersAddExecute_BothCallsAuthFailure_ReturnsTypedError covers
// the spec rule: when BOTH the users-call and the bots-call fail at the
// auth/permission classification layer, Execute must return that typed error
// directly (no ledger, no OutPartialFailure) instead of folding it into
// call_errors. Lark error code 99991672 ("app_missing_scope") is the same
// fixture code internal/errclass/classify_test.go uses to assert
// CategoryAuthorization — using it here means DoAPIJSONTyped's real
// classifier produces the *errs.PermissionError, not a hand-built stand-in.
func TestImChatMembersAddExecute_BothCallsAuthFailure_ReturnsTypedError(t *testing.T) {
	rt := newChatMembersAddTestRuntime(t,
		shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return shortcutJSONResponse(200, map[string]interface{}{"code": 99991672, "msg": "app_missing_scope"}), nil
		}),
		map[string]string{"chat-id": "oc_x"},
		map[string][]string{"users": {"ou_a"}, "bots": {"cli_x"}},
	)

	err := ImChatMembersAdd.Execute(context.Background(), rt)
	var permErr *errs.PermissionError
	if !errors.As(err, &permErr) {
		t.Fatalf("Execute() error = %T %v, want *errs.PermissionError (both calls failed at auth layer)", err, err)
	}

	out := rt.Factory.IOStreams.Out.(interface{ String() string }).String()
	if out != "" {
		t.Errorf("Execute() must not write a ledger when returning the typed error directly, got stdout = %s", out)
	}
}

// TestImChatMembersAddExecute_SingleCallAuthFailure_StillBuildsLedger covers
// the companion rule: when only ONE call was attempted (only --users given)
// and it fails at the auth layer, spec does not apply the "both failed"
// short-circuit — it still builds a ledger (call_errors + OutPartialFailure).
func TestImChatMembersAddExecute_SingleCallAuthFailure_StillBuildsLedger(t *testing.T) {
	rt := newChatMembersAddTestRuntime(t,
		shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return shortcutJSONResponse(200, map[string]interface{}{"code": 99991672, "msg": "app_missing_scope"}), nil
		}),
		map[string]string{"chat-id": "oc_x"},
		map[string][]string{"users": {"ou_a"}},
	)

	err := ImChatMembersAdd.Execute(context.Background(), rt)
	var pfErr *output.PartialFailureError
	if !errors.As(err, &pfErr) {
		t.Fatalf("Execute() error = %T %v, want partial failure (single call, not the both-failed short-circuit)", err, err)
	}

	out := rt.Factory.IOStreams.Out.(interface{ String() string }).String()
	if !strings.Contains(out, `"failure_count": 1`) {
		t.Errorf("output = %s, want failure_count 1 (call_errors ledger entry)", out)
	}
}

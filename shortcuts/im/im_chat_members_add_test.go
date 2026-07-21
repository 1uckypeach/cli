// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/larksuite/cli/errs"
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

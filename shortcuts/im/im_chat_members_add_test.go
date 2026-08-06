// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/output"
	"github.com/spf13/cobra"
)

// newChatMembersAddCmd builds a cobra.Command pre-wired with the flags
// ImChatMembersAdd registers at runtime, mirroring newFeedShortcutCreateCmd.
func newChatMembersAddCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("chat-id", "", "")
	cmd.Flags().String("users", "", "")
	cmd.Flags().String("bots", "", "")
	cmd.Flags().Int("succeed-type", 1, "")
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}
	return cmd
}

func TestCollectChatMembersToAddRequiresAtLeastOne(t *testing.T) {
	cmd := newChatMembersAddCmd(t)
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("must not call API")
		return nil, nil
	}))
	setRuntimeField(t, rt, "Cmd", cmd)

	_, _, err := collectChatMembersToAdd(rt)
	var vErr *errs.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("collectChatMembersToAdd() error = %T %v, want *errs.ValidationError", err, err)
	}
	if vErr.Param != "--users" {
		t.Fatalf("err.Param = %q, want --users", vErr.Param)
	}
}

func TestCollectChatMembersToAddValidatesUserPrefix(t *testing.T) {
	cmd := newChatMembersAddCmd(t)
	if err := cmd.Flags().Set("users", "ou_ok,bad_id"); err != nil {
		t.Fatalf("Set users error = %v", err)
	}
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("must not call API")
		return nil, nil
	}))
	setRuntimeField(t, rt, "Cmd", cmd)

	_, _, err := collectChatMembersToAdd(rt)
	var vErr *errs.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("collectChatMembersToAdd() error = %T %v, want *errs.ValidationError", err, err)
	}
	if vErr.Param != "--users" {
		t.Fatalf("err.Param = %q, want --users", vErr.Param)
	}
}

func TestCollectChatMembersToAddValidatesBotPrefix(t *testing.T) {
	cmd := newChatMembersAddCmd(t)
	if err := cmd.Flags().Set("bots", "cli_ok,ou_notabot"); err != nil {
		t.Fatalf("Set bots error = %v", err)
	}
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("must not call API")
		return nil, nil
	}))
	setRuntimeField(t, rt, "Cmd", cmd)

	_, _, err := collectChatMembersToAdd(rt)
	var vErr *errs.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("collectChatMembersToAdd() error = %T %v, want *errs.ValidationError", err, err)
	}
	if vErr.Param != "--bots" {
		t.Fatalf("err.Param = %q, want --bots", vErr.Param)
	}
}

func TestCollectChatMembersToAddRejectsTooManyUsers(t *testing.T) {
	ids := make([]string, 0, 51)
	for i := 0; i < 51; i++ {
		ids = append(ids, "ou_"+string(rune('a'+i%26)))
	}
	cmd := newChatMembersAddCmd(t)
	if err := cmd.Flags().Set("users", joinComma(ids)); err != nil {
		t.Fatalf("Set users error = %v", err)
	}
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("must not call API")
		return nil, nil
	}))
	setRuntimeField(t, rt, "Cmd", cmd)

	_, _, err := collectChatMembersToAdd(rt)
	var vErr *errs.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("collectChatMembersToAdd() error = %T %v, want *errs.ValidationError", err, err)
	}
	if vErr.Param != "--users" {
		t.Fatalf("err.Param = %q, want --users", vErr.Param)
	}
}

func TestCollectChatMembersToAddReturnsBothLists(t *testing.T) {
	cmd := newChatMembersAddCmd(t)
	if err := cmd.Flags().Set("users", "ou_a, ou_b"); err != nil {
		t.Fatalf("Set users error = %v", err)
	}
	if err := cmd.Flags().Set("bots", "cli_c"); err != nil {
		t.Fatalf("Set bots error = %v", err)
	}
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("must not call API")
		return nil, nil
	}))
	setRuntimeField(t, rt, "Cmd", cmd)

	users, bots, err := collectChatMembersToAdd(rt)
	if err != nil {
		t.Fatalf("collectChatMembersToAdd() error = %v", err)
	}
	if len(users) != 2 || users[0] != "ou_a" || users[1] != "ou_b" {
		t.Fatalf("users = %v, want [ou_a ou_b]", users)
	}
	if len(bots) != 1 || bots[0] != "cli_c" {
		t.Fatalf("bots = %v, want [cli_c]", bots)
	}
}

func TestEmitChatMembersAddResultAllSucceed(t *testing.T) {
	cmd := newChatMembersAddCmd(t)
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("must not call API")
		return nil, nil
	}))
	setRuntimeField(t, rt, "Cmd", cmd)

	err := emitChatMembersAddResult(rt, "oc_x", []string{"ou_a", "cli_b"}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("emitChatMembersAddResult() error = %v, want nil", err)
	}
	out := rt.Factory.IOStreams.Out.(interface{ String() string }).String()
	for _, want := range []string{
		`"chat_id": "oc_x"`,
		`"total": 2`,
		`"success_count": 2`,
		`"failure_count": 0`,
		`"succeeded_ids"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %s, want %q", out, want)
		}
	}
}

func TestEmitChatMembersAddResultPartialFailure(t *testing.T) {
	cmd := newChatMembersAddCmd(t)
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("must not call API")
		return nil, nil
	}))
	setRuntimeField(t, rt, "Cmd", cmd)

	err := emitChatMembersAddResult(rt, "oc_x", []string{"ou_a", "ou_bad"}, map[string]interface{}{
		"invalid_id_list": []interface{}{"ou_bad"},
	})
	var pfErr *output.PartialFailureError
	if !errors.As(err, &pfErr) {
		t.Fatalf("emitChatMembersAddResult() error = %T %v, want partial failure", err, err)
	}
	if pfErr.Code != output.ExitAPI {
		t.Fatalf("partial failure exit code = %d, want %d (ExitAPI)", pfErr.Code, output.ExitAPI)
	}
	out := rt.Factory.IOStreams.Out.(interface{ String() string }).String()
	for _, want := range []string{
		`"ok": false`,
		`"success_count": 1`,
		`"failure_count": 1`,
		`"invalid_id_list"`,
		`ou_bad`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %s, want %q", out, want)
		}
	}
	errOut := rt.Factory.IOStreams.ErrOut.(interface{ String() string }).String()
	if !strings.Contains(errOut, "warning: 1 member(s) could not be added: ou_bad") {
		t.Fatalf("stderr = %s, want warning line", errOut)
	}
}

func TestEmitChatMembersAddResultInvariantHoldsUnderServerAnomalies(t *testing.T) {
	cases := []struct {
		name      string
		requested []string
		data      map[string]interface{}
	}{
		{
			name:      "duplicate requested id echoed once in invalid_id_list",
			requested: []string{"ou_a", "ou_a"},
			data: map[string]interface{}{
				"invalid_id_list": []interface{}{"ou_a"},
			},
		},
		{
			name:      "invalid_id_list contains an id outside requested",
			requested: []string{"ou_a", "ou_bad"},
			data: map[string]interface{}{
				"invalid_id_list": []interface{}{"ou_bad", "ou_not_requested"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newChatMembersAddCmd(t)
			rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("must not call API")
				return nil, nil
			}))
			setRuntimeField(t, rt, "Cmd", cmd)

			err := emitChatMembersAddResult(rt, "oc_x", tc.requested, tc.data)
			var pfErr *output.PartialFailureError
			if !errors.As(err, &pfErr) {
				t.Fatalf("emitChatMembersAddResult() error = %T %v, want partial failure", err, err)
			}

			out := rt.Factory.IOStreams.Out.(interface{ String() string }).String()
			total := len(tc.requested)
			wantTotal := fmt.Sprintf(`"total": %d`, total)
			if !strings.Contains(out, wantTotal) {
				t.Fatalf("stdout = %s, want %q", out, wantTotal)
			}

			successCount := extractIntField(t, out, "success_count")
			failureCount := extractIntField(t, out, "failure_count")
			if successCount+failureCount != total {
				t.Fatalf("success_count(%d) + failure_count(%d) = %d, want total %d", successCount, failureCount, successCount+failureCount, total)
			}
		})
	}
}

// extractIntField pulls the integer value of a `"field": N` pair out of the
// JSON output produced by runtime.Out/OutPartialFailure for assertions above.
func extractIntField(t *testing.T, out, field string) int {
	t.Helper()
	marker := fmt.Sprintf(`"%s": `, field)
	idx := strings.Index(out, marker)
	if idx == -1 {
		t.Fatalf("stdout = %s, missing field %q", out, field)
	}
	rest := out[idx+len(marker):]
	end := strings.IndexAny(rest, ",\n}")
	if end == -1 {
		t.Fatalf("stdout = %s, could not parse field %q", out, field)
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(rest[:end]), "%d", &n); err != nil {
		t.Fatalf("failed to parse int for field %q: %v", field, err)
	}
	return n
}

func joinComma(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += id
	}
	return out
}

func TestImChatMembersAddExecuteCallsAPI(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	var gotBody []byte
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotQuery = req.URL.Query()
		body, _ := io.ReadAll(req.Body)
		gotBody = body
		return shortcutJSONResponse(200, map[string]any{
			"code": 0,
			"data": map[string]any{},
		}), nil
	}))
	cmd := newChatMembersAddCmd(t)
	if err := cmd.Flags().Set("chat-id", "oc_x"); err != nil {
		t.Fatalf("Set chat-id error = %v", err)
	}
	if err := cmd.Flags().Set("users", "ou_a"); err != nil {
		t.Fatalf("Set users error = %v", err)
	}
	if err := cmd.Flags().Set("bots", "cli_b"); err != nil {
		t.Fatalf("Set bots error = %v", err)
	}
	setRuntimeField(t, rt, "Cmd", cmd)

	if err := ImChatMembersAdd.Execute(context.Background(), rt); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.HasSuffix(gotPath, "/open-apis/im/v1/chats/oc_x/members") {
		t.Fatalf("Execute() path = %q, want .../chats/oc_x/members", gotPath)
	}
	if gotQuery.Get("member_id_type") != "open_id" {
		t.Fatalf("member_id_type query = %q, want open_id", gotQuery.Get("member_id_type"))
	}
	if gotQuery.Get("succeed_type") != "1" {
		t.Fatalf("succeed_type query = %q, want 1 (default)", gotQuery.Get("succeed_type"))
	}
	if !strings.Contains(string(gotBody), `"id_list":["ou_a","cli_b"]`) {
		t.Fatalf("Execute() body = %s, want id_list [ou_a cli_b]", gotBody)
	}
	out := rt.Factory.IOStreams.Out.(interface{ String() string }).String()
	if !strings.Contains(out, `"success_count": 2`) {
		t.Fatalf("stdout = %s, want success_count 2", out)
	}
}

func TestImChatMembersAddExecutePassesSucceedType(t *testing.T) {
	var gotQuery url.Values
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotQuery = req.URL.Query()
		return shortcutJSONResponse(200, map[string]any{"code": 0, "data": map[string]any{}}), nil
	}))
	cmd := newChatMembersAddCmd(t)
	if err := cmd.Flags().Set("chat-id", "oc_x"); err != nil {
		t.Fatalf("Set chat-id error = %v", err)
	}
	if err := cmd.Flags().Set("users", "ou_a"); err != nil {
		t.Fatalf("Set users error = %v", err)
	}
	if err := cmd.Flags().Set("succeed-type", "0"); err != nil {
		t.Fatalf("Set succeed-type error = %v", err)
	}
	setRuntimeField(t, rt, "Cmd", cmd)

	if err := ImChatMembersAdd.Execute(context.Background(), rt); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotQuery.Get("succeed_type") != "0" {
		t.Fatalf("succeed_type query = %q, want 0", gotQuery.Get("succeed_type"))
	}
}

func TestImChatMembersAddValidateRejectsBadChatID(t *testing.T) {
	cmd := newChatMembersAddCmd(t)
	if err := cmd.Flags().Set("users", "ou_a"); err != nil {
		t.Fatalf("Set users error = %v", err)
	}
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("must not call API")
		return nil, nil
	}))
	setRuntimeField(t, rt, "Cmd", cmd)

	err := ImChatMembersAdd.Validate(context.Background(), rt)
	var vErr *errs.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("Validate() error = %T %v, want *errs.ValidationError", err, err)
	}
	if vErr.Param != "--chat-id" {
		t.Fatalf("err.Param = %q, want --chat-id", vErr.Param)
	}
}

func TestImChatMembersAddValidateRejectsBadSucceedType(t *testing.T) {
	cmd := newChatMembersAddCmd(t)
	if err := cmd.Flags().Set("chat-id", "oc_x"); err != nil {
		t.Fatalf("Set chat-id error = %v", err)
	}
	if err := cmd.Flags().Set("users", "ou_a"); err != nil {
		t.Fatalf("Set users error = %v", err)
	}
	if err := cmd.Flags().Set("succeed-type", "2"); err != nil {
		t.Fatalf("Set succeed-type error = %v", err)
	}
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("must not call API")
		return nil, nil
	}))
	setRuntimeField(t, rt, "Cmd", cmd)

	err := ImChatMembersAdd.Validate(context.Background(), rt)
	var vErr *errs.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("Validate() error = %T %v, want *errs.ValidationError", err, err)
	}
	if vErr.Param != "--succeed-type" {
		t.Fatalf("err.Param = %q, want --succeed-type", vErr.Param)
	}
}

func TestImChatMembersAddDryRunRendersBody(t *testing.T) {
	rt := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("must not call API")
		return nil, nil
	}))
	cmd := newChatMembersAddCmd(t)
	if err := cmd.Flags().Set("chat-id", "oc_x"); err != nil {
		t.Fatalf("Set chat-id error = %v", err)
	}
	if err := cmd.Flags().Set("users", "ou_a"); err != nil {
		t.Fatalf("Set users error = %v", err)
	}
	setRuntimeField(t, rt, "Cmd", cmd)

	dry := ImChatMembersAdd.DryRun(context.Background(), rt)
	if dry == nil {
		t.Fatal("DryRun() = nil")
	}
}

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

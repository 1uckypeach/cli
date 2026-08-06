// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"errors"
	"net/http"
	"testing"

	"github.com/larksuite/cli/errs"
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

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
	"github.com/spf13/cobra"
)

func TestReadChatMembersAddSpec(t *testing.T) {
	userIDsOverLimit := make([]string, imChatMembersAddUserLimit+1)
	for i := range userIDsOverLimit {
		userIDsOverLimit[i] = "ou_user_" + strings.Repeat("a", i+1)
	}
	botIDsOverLimit := make([]string, imChatMembersAddBotLimit+1)
	for i := range botIDsOverLimit {
		botIDsOverLimit[i] = "cli_bot_" + strings.Repeat("a", i+1)
	}

	tests := []struct {
		name       string
		chatID     string
		users      string
		bots       string
		wantParam  string
		wantParams []string
	}{
		{
			name:      "missing chat",
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:       "missing members",
			chatID:     "oc_chat_a",
			wantParams: []string{"--users", "--bots"},
		},
		{
			name:      "invalid user prefix",
			chatID:    "oc_chat_a",
			users:     "user_a",
			wantParam: "--users",
		},
		{
			name:      "empty user suffix",
			chatID:    "oc_chat_a",
			users:     "ou_",
			wantParam: "--users",
		},
		{
			name:      "invalid bot prefix",
			chatID:    "oc_chat_a",
			bots:      "bot_a",
			wantParam: "--bots",
		},
		{
			name:      "empty bot suffix",
			chatID:    "oc_chat_a",
			bots:      "cli_",
			wantParam: "--bots",
		},
		{
			name:      "user control character",
			chatID:    "oc_chat_a",
			users:     "ou_user\x00a",
			wantParam: "--users",
		},
		{
			name:      "bot control character",
			chatID:    "oc_chat_a",
			bots:      "cli_bot\x1fa",
			wantParam: "--bots",
		},
		{
			name:      "user identifier too long",
			chatID:    "oc_chat_a",
			users:     "ou_" + strings.Repeat("a", imChatMembersAddIDMaxBytes-2),
			wantParam: "--users",
		},
		{
			name:      "bot identifier too long",
			chatID:    "oc_chat_a",
			bots:      "cli_" + strings.Repeat("a", imChatMembersAddIDMaxBytes-3),
			wantParam: "--bots",
		},
		{
			name:      "too many users",
			chatID:    "oc_chat_a",
			users:     strings.Join(userIDsOverLimit, ","),
			wantParam: "--users",
		},
		{
			name:      "too many bots",
			chatID:    "oc_chat_a",
			bots:      strings.Join(botIDsOverLimit, ","),
			wantParam: "--bots",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := newChatMembersAddTestRuntime(t, tt.chatID, tt.users, tt.bots)
			_, err := readChatMembersAddSpec(runtime)
			assertChatMembersAddValidationError(t, err, tt.wantParam, tt.wantParams)
		})
	}
}

func TestReadChatMembersAddSpecDeduplicatesInFirstSeenOrder(t *testing.T) {
	runtime := newChatMembersAddTestRuntime(
		t,
		"oc_chat_a",
		"ou_b,ou_a,ou_b",
		"cli_b,cli_a,cli_b",
	)

	got, err := readChatMembersAddSpec(runtime)
	if err != nil {
		t.Fatalf("readChatMembersAddSpec() error = %v", err)
	}

	want := chatMembersAddSpec{
		ChatID: "oc_chat_a",
		Users:  []string{"ou_b", "ou_a"},
		Bots:   []string{"cli_b", "cli_a"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readChatMembersAddSpec() = %#v, want %#v", got, want)
	}
}

func TestDedupeChatMemberIDs(t *testing.T) {
	got := dedupeChatMemberIDs([]string{"ou_b", "ou_a", "ou_b", "ou_a", "ou_c"})
	want := []string{"ou_b", "ou_a", "ou_c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dedupeChatMemberIDs() = %#v, want %#v", got, want)
	}
}

func newChatMembersAddTestRuntime(t *testing.T, chatID, users, bots string) *common.RuntimeContext {
	t.Helper()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("chat-id", "", "")
	cmd.Flags().String("users", "", "")
	cmd.Flags().String("bots", "", "")
	for name, value := range map[string]string{
		"chat-id": chatID,
		"users":   users,
		"bots":    bots,
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	return &common.RuntimeContext{Cmd: cmd}
}

func assertChatMembersAddValidationError(t *testing.T, err error, wantParam string, wantParams []string) {
	t.Helper()

	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %T (%v), want *errs.ValidationError", err, err)
	}
	problem, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("ProblemOf(%v) returned ok=false", err)
	}
	if problem.Category != errs.CategoryValidation || problem.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf(
			"problem = category %q subtype %q, want category %q subtype %q",
			problem.Category,
			problem.Subtype,
			errs.CategoryValidation,
			errs.SubtypeInvalidArgument,
		)
	}
	if validationErr.Param != wantParam {
		t.Fatalf("Param = %q, want %q", validationErr.Param, wantParam)
	}
	var gotParams []string
	for _, param := range validationErr.Params {
		gotParams = append(gotParams, param.Name)
	}
	if !reflect.DeepEqual(gotParams, wantParams) {
		t.Fatalf("Params = %#v, want %#v", gotParams, wantParams)
	}
}

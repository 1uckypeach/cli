// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"errors"
	"fmt"
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
		name        string
		chatID      string
		users       string
		bots        string
		wantParam   string
		wantParams  []string
		wantMessage string
	}{
		{
			name:      "missing chat",
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:        "missing members",
			chatID:      "oc_chat_a",
			wantParams:  []string{"--users", "--bots"},
			wantMessage: "specify at least one of --users or --bots",
		},
		{
			name:      "chat control character",
			chatID:    "oc_chat\x00a",
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:      "chat unicode suffix",
			chatID:    "oc_群聊",
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:      "chat URL metacharacter",
			chatID:    "oc_chat?mode=admin",
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:      "chat identifier too long",
			chatID:    "oc_" + strings.Repeat("a", imChatMembersAddIDMaxBytes-2),
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:      "chat URL token receives local validation",
			chatID:    "https://tenant.feishu.cn/messenger/chat/oc_chat?mode=admin",
			users:     "ou_user_a",
			wantParam: "--chat-id",
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
			name:        "too many users",
			chatID:      "oc_chat_a",
			users:       strings.Join(userIDsOverLimit, ","),
			wantParam:   "--users",
			wantMessage: "--users accepts at most 50 unique IDs",
		},
		{
			name:        "too many bots",
			chatID:      "oc_chat_a",
			bots:        strings.Join(botIDsOverLimit, ","),
			wantParam:   "--bots",
			wantMessage: "--bots accepts at most 5 unique IDs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := newChatMembersAddTestRuntime(t, tt.chatID, tt.users, tt.bots)
			_, err := readChatMembersAddSpec(runtime)
			assertChatMembersAddValidationError(t, err, tt.wantParam, tt.wantParams, tt.wantMessage)
		})
	}
}

func TestReadChatMembersAddSpecAcceptsNormalizedChatURL(t *testing.T) {
	runtime := newChatMembersAddTestRuntime(
		t,
		"https://tenant.feishu.cn/messenger/chat/oc_chat_a",
		"ou_user_a",
		"",
	)

	got, err := readChatMembersAddSpec(runtime)
	if err != nil {
		t.Fatalf("readChatMembersAddSpec() error = %v", err)
	}
	if got.ChatID != "oc_chat_a" {
		t.Fatalf("ChatID = %q, want %q", got.ChatID, "oc_chat_a")
	}
}

func TestReadChatMembersAddSpecCountsUniqueIDs(t *testing.T) {
	usersAtLimit := makeChatMembersAddIDs("ou_user_", imChatMembersAddUserLimit)
	botsAtLimit := makeChatMembersAddIDs("cli_bot_", imChatMembersAddBotLimit)

	tests := []struct {
		name      string
		users     []string
		bots      []string
		wantUsers int
		wantBots  int
	}{
		{
			name:      "51 user entries with one duplicate",
			users:     append(append([]string(nil), usersAtLimit...), usersAtLimit[0]),
			wantUsers: imChatMembersAddUserLimit,
		},
		{
			name:     "6 bot entries with one duplicate",
			bots:     append(append([]string(nil), botsAtLimit...), botsAtLimit[0]),
			wantBots: imChatMembersAddBotLimit,
		},
		{
			name:      "exact user and bot limits",
			users:     usersAtLimit,
			bots:      botsAtLimit,
			wantUsers: imChatMembersAddUserLimit,
			wantBots:  imChatMembersAddBotLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := newChatMembersAddTestRuntime(
				t,
				"oc_chat_a",
				strings.Join(tt.users, ","),
				strings.Join(tt.bots, ","),
			)
			got, err := readChatMembersAddSpec(runtime)
			if err != nil {
				t.Fatalf("readChatMembersAddSpec() error = %v", err)
			}
			if len(got.Users) != tt.wantUsers || len(got.Bots) != tt.wantBots {
				t.Fatalf(
					"member counts = users %d bots %d, want users %d bots %d",
					len(got.Users),
					len(got.Bots),
					tt.wantUsers,
					tt.wantBots,
				)
			}
		})
	}
}

func TestReadChatMembersAddSpecErrorsDoNotEchoMemberIDs(t *testing.T) {
	tests := []struct {
		name  string
		users string
		bots  string
		rawID string
	}{
		{
			name:  "invalid user characters",
			users: "ou_private@example.com",
			rawID: "ou_private@example.com",
		},
		{
			name:  "invalid bot characters",
			bots:  "cli_private@example.com",
			rawID: "cli_private@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := newChatMembersAddTestRuntime(t, "oc_chat_a", tt.users, tt.bots)
			_, err := readChatMembersAddSpec(runtime)
			if err == nil {
				t.Fatal("readChatMembersAddSpec() error = nil, want validation error")
			}
			if strings.Contains(err.Error(), tt.rawID) {
				t.Fatalf("error message contains the member identifier: %q", err.Error())
			}
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

func makeChatMembersAddIDs(prefix string, count int) []string {
	ids := make([]string, count)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s%d", prefix, i)
	}
	return ids
}

func assertChatMembersAddValidationError(t *testing.T, err error, wantParam string, wantParams []string, wantMessage string) {
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
	if wantMessage != "" && validationErr.Error() != wantMessage {
		t.Fatalf("error message = %q, want %q", validationErr.Error(), wantMessage)
	}
}

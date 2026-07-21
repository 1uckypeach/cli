// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

const imChatMembersAddPathFmt = "/open-apis/im/v1/chats/%s/members"

const (
	chatMembersAddMaxUsers = 50
	chatMembersAddMaxBots  = 5
)

// collectMemberAddIDs reads a string_slice flag, trims/dedupes its values,
// and enforces the ID prefix and the per-call count limit dictated by
// chat.members.create (50 users / 5 bots per request).
func collectMemberAddIDs(runtime *common.RuntimeContext, flag, prefix string, max int) ([]string, error) {
	raw := runtime.StrSlice(flag)
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if !strings.HasPrefix(v, prefix) {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
				"invalid --%s value %q: must start with %q", flag, v, prefix).WithParam("--" + flag)
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) > max {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"--%s exceeds the maximum of %d (got %d)", flag, max, len(out)).WithParam("--" + flag)
	}
	return out, nil
}

// validateChatMembersAdd checks --chat-id format and that --users/--bots
// each satisfy their prefix and count-limit rules, and that at least one of
// them is non-empty. All checks happen locally so bad input never reaches
// the API layer.
func validateChatMembersAdd(runtime *common.RuntimeContext) error {
	chatID := strings.TrimSpace(runtime.Str("chat-id"))
	if !strings.HasPrefix(chatID, "oc_") {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"invalid --chat-id %q: must be an open_chat_id starting with oc_", chatID).WithParam("--chat-id")
	}

	users, err := collectMemberAddIDs(runtime, "users", "ou_", chatMembersAddMaxUsers)
	if err != nil {
		return err
	}
	bots, err := collectMemberAddIDs(runtime, "bots", "cli_", chatMembersAddMaxBots)
	if err != nil {
		return err
	}
	if len(users) == 0 && len(bots) == 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "at least one of --users or --bots is required")
	}
	return nil
}

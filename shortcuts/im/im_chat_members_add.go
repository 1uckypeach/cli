// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

const (
	chatMembersAddMaxUsers = 50
	chatMembersAddMaxBots  = 5
)

// collectChatMembersToAdd parses --users/--bots into validated ID slices.
// Returns a validation error when both are empty, any user ID lacks the ou_
// prefix, any bot ID lacks the cli_ prefix, or either list exceeds its cap —
// mirroring the --users/--bots validation +chat-create already applies at
// chat-creation time (shortcuts/im/im_chat_create.go).
func collectChatMembersToAdd(runtime *common.RuntimeContext) ([]string, []string, error) {
	var users, bots []string

	if raw := runtime.Str("users"); raw != "" {
		ids := common.SplitCSV(raw)
		if len(ids) > chatMembersAddMaxUsers {
			return nil, nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--users exceeds the maximum of %d (got %d)", chatMembersAddMaxUsers, len(ids)).WithParam("--users")
		}
		for _, id := range ids {
			normalized, err := common.ValidateUserIDTyped("--users", id)
			if err != nil {
				return nil, nil, err
			}
			users = append(users, normalized)
		}
	}

	if raw := runtime.Str("bots"); raw != "" {
		ids := common.SplitCSV(raw)
		if len(ids) > chatMembersAddMaxBots {
			return nil, nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--bots exceeds the maximum of %d (got %d)", chatMembersAddMaxBots, len(ids)).WithParam("--bots")
		}
		for _, id := range ids {
			if !strings.HasPrefix(id, "cli_") {
				return nil, nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "invalid bot id %q: expected app ID (cli_xxx)", id).WithParam("--bots")
			}
			bots = append(bots, id)
		}
	}

	if len(users) == 0 && len(bots) == 0 {
		return nil, nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--users or --bots is required (at least one)").WithParam("--users")
	}
	return users, bots, nil
}

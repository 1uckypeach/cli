// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"fmt"
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

// emitChatMembersAddResult builds the ledger envelope (total/success/failure
// counts plus succeeded_ids) from the requested id list and the server's
// invalid_id_list, then writes it via the ok:false partial-failure path when
// any ID failed — the same convention +feed-shortcut-create already
// established for batch writes in this domain (see addFeedShortcutWriteLedger
// in shortcuts/im/helpers.go).
func emitChatMembersAddResult(runtime *common.RuntimeContext, chatID string, requested []string, data map[string]interface{}) error {
	if data == nil {
		data = map[string]interface{}{}
	}
	invalid := stringsFromAny(data["invalid_id_list"])

	invalidSet := make(map[string]struct{}, len(invalid))
	for _, id := range invalid {
		invalidSet[id] = struct{}{}
	}
	succeeded := make([]string, 0, len(requested))
	for _, id := range requested {
		if _, failed := invalidSet[id]; failed {
			continue
		}
		succeeded = append(succeeded, id)
	}

	outData := map[string]interface{}{
		"chat_id":         chatID,
		"total":           len(requested),
		"success_count":   len(succeeded),
		"failure_count":   len(requested) - len(succeeded),
		"succeeded_ids":   succeeded,
		"invalid_id_list": invalid,
	}

	if len(invalid) > 0 {
		fmt.Fprintf(runtime.IO().ErrOut, "warning: %d member(s) could not be added: %s\n", len(invalid), strings.Join(invalid, ", "))
		return runtime.OutPartialFailure(outData, nil)
	}
	runtime.Out(outData, nil)
	return nil
}

// stringsFromAny coerces a JSON-decoded []interface{} of strings (the shape
// invalid_id_list arrives in after generic unmarshal) into []string,
// tolerating a nil/missing field as an empty slice.
func stringsFromAny(v interface{}) []string {
	raw, ok := v.([]interface{})
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

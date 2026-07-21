// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
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

// chatMembersAddResult is the merged ledger across the users-call and the
// bots-call.
type chatMembersAddResult struct {
	succeeded       []string
	invalid         []string
	notExisted      []string
	pendingApproval []string
	callErrors      []map[string]interface{}
}

func newChatMembersAddResult() *chatMembersAddResult {
	return &chatMembersAddResult{
		succeeded:       []string{},
		invalid:         []string{},
		notExisted:      []string{},
		pendingApproval: []string{},
		callErrors:      []map[string]interface{}{},
	}
}

// addChatMembersBatch issues one chat.members.create call for a single
// member_id_type and folds the outcome into res. A full-call error (e.g.
// missing scope, chat-wide bot cap exceeded) is recorded as a call_errors
// entry carrying the affected id_list, rather than aborting the other call.
func addChatMembersBatch(runtime *common.RuntimeContext, chatID, memberType, memberIDType string, ids []string, res *chatMembersAddResult) {
	path := fmt.Sprintf(imChatMembersAddPathFmt, validate.EncodePathSegment(chatID))
	data, err := runtime.DoAPIJSONTyped(http.MethodPost, path,
		larkcore.QueryParams{
			"member_id_type": []string{memberIDType},
			"succeed_type":   []string{"1"},
		},
		map[string]interface{}{"id_list": ids},
	)
	if err != nil {
		res.callErrors = append(res.callErrors, map[string]interface{}{
			"member_type": memberType,
			"id_list":     ids,
			"error":       err.Error(),
		})
		return
	}

	invalid := stringsFromAny(data["invalid_id_list"])
	notExisted := stringsFromAny(data["not_existed_id_list"])
	pending := stringsFromAny(data["pending_approval_id_list"])
	res.invalid = append(res.invalid, invalid...)
	res.notExisted = append(res.notExisted, notExisted...)
	res.pendingApproval = append(res.pendingApproval, pending...)

	failed := make(map[string]struct{}, len(invalid)+len(notExisted)+len(pending))
	for _, id := range invalid {
		failed[id] = struct{}{}
	}
	for _, id := range notExisted {
		failed[id] = struct{}{}
	}
	for _, id := range pending {
		failed[id] = struct{}{}
	}
	for _, id := range ids {
		if _, isFailed := failed[id]; !isFailed {
			res.succeeded = append(res.succeeded, id)
		}
	}
}

// stringsFromAny converts a JSON-decoded []interface{} of strings to []string,
// skipping any non-string entries defensively.
func stringsFromAny(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

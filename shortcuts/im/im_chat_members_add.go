// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
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
// invalid_id_list/not_existed_id_list/pending_approval_id_list, then writes
// it via the ok:false partial-failure path when any ID failed — the same
// convention +feed-shortcut-create already established for batch writes in
// this domain (see addFeedShortcutWriteLedger in shortcuts/im/helpers.go).
//
// All three server lists represent an id that did NOT become a real member
// of the chat: invalid_id_list (departed/invisible/app-not-activated),
// not_existed_id_list (id does not exist), and pending_approval_id_list
// (awaiting owner/admin approval — not yet a member). An id present in any
// of them must not be counted as succeeded.
func emitChatMembersAddResult(runtime *common.RuntimeContext, chatID string, requested []string, data map[string]interface{}) error {
	if data == nil {
		data = map[string]interface{}{}
	}
	invalid := stringsFromAny(data["invalid_id_list"])
	notExisted := stringsFromAny(data["not_existed_id_list"])
	pendingApproval := stringsFromAny(data["pending_approval_id_list"])

	failedSet := make(map[string]struct{}, len(invalid)+len(notExisted)+len(pendingApproval))
	var failedIDs []string
	for _, list := range [][]string{invalid, notExisted, pendingApproval} {
		for _, id := range list {
			if _, seen := failedSet[id]; !seen {
				failedSet[id] = struct{}{}
				failedIDs = append(failedIDs, id)
			}
		}
	}
	succeeded := make([]string, 0, len(requested))
	for _, id := range requested {
		if _, failed := failedSet[id]; failed {
			continue
		}
		succeeded = append(succeeded, id)
	}

	outData := map[string]interface{}{
		"chat_id":              chatID,
		"total":                len(requested),
		"success_count":        len(succeeded),
		"failure_count":        len(requested) - len(succeeded),
		"succeeded_ids":        succeeded,
		"invalid_id_list":      invalid,
		"not_existed_id_list":  notExisted,
		"pending_approval_ids": pendingApproval,
	}

	if len(invalid) > 0 || len(notExisted) > 0 || len(pendingApproval) > 0 {
		fmt.Fprintf(runtime.IO().ErrOut, "warning: %d member(s) could not be added: %s\n", len(failedIDs), strings.Join(failedIDs, ", "))
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

const imChatMembersAddPathFmt = "/open-apis/im/v1/chats/%s/members"

// ImChatMembersAdd is the +chat-members-add shortcut: it adds users and/or
// bots to an existing chat via a single POST
// /open-apis/im/v1/chats/:chat_id/members call. It collapses the two inputs
// the raw chat.members create command requires (a --params JSON blob for
// chat_id/member_id_type/succeed_type, a --data JSON blob for id_list) into
// two plain flags, and fixes member_id_type to open_id so callers never have
// to keep a user ID's format in sync with a separate query parameter — bot
// IDs are always recognized by their cli_ prefix regardless of
// member_id_type.
var ImChatMembersAdd = common.Shortcut{
	Service:     "im",
	Command:     "+chat-members-add",
	Description: "Add users and/or bots to a chat; user/bot; --users (ou_xxx, max 50) and/or --bots (cli_xxx, max 5); --succeed-type controls whether unreachable IDs fail the whole call or are reported via invalid_id_list",
	Risk:        "write",
	Scopes:      []string{"im:chat.members:write_only"},
	AuthTypes:   []string{"user", "bot"},
	Flags: []common.Flag{
		{Name: "chat-id", Required: true, Desc: "chat ID (oc_xxx)"},
		{Name: "users", Desc: "comma-separated user open_ids (ou_xxx) to add, max 50"},
		{Name: "bots", Desc: "comma-separated bot app IDs (cli_xxx) to add, max 5"},
		{Name: "succeed-type", Type: "int", Default: "1", Desc: "0 = fail the entire request if any member is invalid; 1 (default) = add valid members and report invalid ones via invalid_id_list without failing the call"},
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		chatID := strings.TrimSpace(runtime.Str("chat-id"))
		if chatID == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--chat-id is required (oc_xxx)").WithParam("--chat-id")
		}
		if !strings.HasPrefix(chatID, "oc_") {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "invalid --chat-id %q: must be an open_chat_id starting with oc_", chatID).WithParam("--chat-id")
		}
		if _, _, err := collectChatMembersToAdd(runtime); err != nil {
			return err
		}
		if st := runtime.Int("succeed-type"); st != 0 && st != 1 {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--succeed-type must be 0 or 1 (got %d)", st).WithParam("--succeed-type")
		}
		return nil
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		chatID := strings.TrimSpace(runtime.Str("chat-id"))
		users, bots, err := collectChatMembersToAdd(runtime)
		if err != nil {
			return common.NewDryRunAPI().Set("error", err.Error())
		}
		idList := append(append([]string{}, users...), bots...)
		return common.NewDryRunAPI().
			POST(fmt.Sprintf(imChatMembersAddPathFmt, validate.EncodePathSegment(chatID))).
			Params(map[string]interface{}{
				"member_id_type": "open_id",
				"succeed_type":   runtime.Int("succeed-type"),
			}).
			Body(map[string]interface{}{"id_list": idList})
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		chatID := strings.TrimSpace(runtime.Str("chat-id"))
		users, bots, err := collectChatMembersToAdd(runtime)
		if err != nil {
			return err
		}
		idList := append(append([]string{}, users...), bots...)
		succeedType := runtime.Int("succeed-type")

		qp := larkcore.QueryParams{
			"member_id_type": []string{"open_id"},
			"succeed_type":   []string{strconv.Itoa(succeedType)},
		}
		resData, err := runtime.DoAPIJSONTyped(http.MethodPost,
			fmt.Sprintf(imChatMembersAddPathFmt, validate.EncodePathSegment(chatID)),
			qp, map[string]interface{}{"id_list": idList})
		if err != nil {
			return err
		}
		return emitChatMembersAddResult(runtime, chatID, idList, resData)
	},
}

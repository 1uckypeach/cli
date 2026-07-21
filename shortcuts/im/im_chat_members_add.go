// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"context"
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
// bots-call. rawCallErrors is parallel to callErrors (same append order) and
// carries the original typed error instead of its stringified form, so the
// caller can classify it (e.g. auth/permission) without re-parsing error
// text; it never enters the JSON output.
type chatMembersAddResult struct {
	succeeded       []string
	invalid         []string
	notExisted      []string
	pendingApproval []string
	callErrors      []map[string]interface{}
	rawCallErrors   []error
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
		res.rawCallErrors = append(res.rawCallErrors, err)
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

// ImChatMembersAdd is the +chat-members-add shortcut: adds users and/or bots
// to a group chat. --users (open_id) and --bots (app_id) map to two
// independent underlying calls to POST /open-apis/im/v1/chats/{chat_id}/members
// (member_id_type differs per call — a single call cannot mix open_id and
// app_id, because chat.members.create only accepts one member_id_type per
// request). Both calls use succeed_type=1 (best effort): valid IDs are added
// even if others are invalid/nonexistent/pending approval, so a single
// resigned user does not block everyone else. Results are merged into one
// ledger so the caller doesn't have to reconcile two API responses by hand.
var ImChatMembersAdd = common.Shortcut{
	Service:     "im",
	Command:     "+chat-members-add",
	Description: "Add users and/or bots to a group chat; user/bot; batches --users (open_id) and --bots (app_id) into up to 2 API calls under best-effort semantics; returns a merged succeeded/invalid/not_existed/pending_approval ledger",
	Risk:        "write",
	Scopes:      []string{"im:chat", "im:chat.members:write_only"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags: []common.Flag{
		{Name: "chat-id", Required: true, Desc: "chat ID to add members to (oc_xxx)"},
		{Name: "users", Type: "string_slice", Desc: "user open_ids to invite (ou_xxx); comma-separated or repeat the flag; max 50"},
		{Name: "bots", Type: "string_slice", Desc: "bot app_ids to invite (cli_xxx); comma-separated or repeat the flag; max 5"},
	},
	Tips: []string{
		"lark-cli im +chat-members-add --chat-id oc_xxx --users ou_a,ou_b --bots cli_x",
		"--users and --bots are independent; at least one is required, but providing both issues two API calls internally and merges the results into one ledger.",
		"Partial failures (resigned users, nonexistent IDs, pending approval) don't block the other valid IDs from being added — check failure_count and the per-reason lists in the result.",
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		return validateChatMembersAdd(runtime)
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		chatID := strings.TrimSpace(runtime.Str("chat-id"))
		users, _ := collectMemberAddIDs(runtime, "users", "ou_", chatMembersAddMaxUsers)
		bots, _ := collectMemberAddIDs(runtime, "bots", "cli_", chatMembersAddMaxBots)
		path := fmt.Sprintf(imChatMembersAddPathFmt, validate.EncodePathSegment(chatID))

		dry := common.NewDryRunAPI()
		if len(users) > 0 {
			dry.POST(path).
				Params(map[string]interface{}{"member_id_type": "open_id", "succeed_type": 1}).
				Body(map[string]interface{}{"id_list": users})
		}
		if len(bots) > 0 {
			dry.POST(path).
				Params(map[string]interface{}{"member_id_type": "app_id", "succeed_type": 1}).
				Body(map[string]interface{}{"id_list": bots})
		}
		return dry
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		return executeChatMembersAdd(runtime)
	},
}

// executeChatMembersAdd issues one call per non-empty member list and merges
// the results. The two calls are independent: a full-call failure on one
// (e.g. bot count over the chat-wide cap) does not prevent the other from
// running or from having its successes reported.
func executeChatMembersAdd(runtime *common.RuntimeContext) error {
	chatID := strings.TrimSpace(runtime.Str("chat-id"))
	users, err := collectMemberAddIDs(runtime, "users", "ou_", chatMembersAddMaxUsers)
	if err != nil {
		return err
	}
	bots, err := collectMemberAddIDs(runtime, "bots", "cli_", chatMembersAddMaxBots)
	if err != nil {
		return err
	}

	res := newChatMembersAddResult()
	attempted := 0
	if len(users) > 0 {
		attempted++
		addChatMembersBatch(runtime, chatID, "user", "open_id", users, res)
	}
	if len(bots) > 0 {
		attempted++
		addChatMembersBatch(runtime, chatID, "bot", "app_id", bots, res)
	}

	// Per spec: only when BOTH attempted calls failed, and both failures
	// classify as auth/permission errors, surface the typed error directly
	// instead of building a ledger. A single attempted call failing this way
	// still falls through to the normal ledger path below.
	if attempted == 2 && len(res.rawCallErrors) == 2 && allAuthClassified(res.rawCallErrors) {
		return res.rawCallErrors[0]
	}

	total := len(users) + len(bots)
	failureCount := len(res.invalid) + len(res.notExisted) + len(res.pendingApproval)
	for _, ce := range res.callErrors {
		if ids, ok := ce["id_list"].([]string); ok {
			failureCount += len(ids)
		}
	}
	successCount := total - failureCount

	outData := map[string]interface{}{
		"chat_id":                  chatID,
		"succeeded_id_list":        res.succeeded,
		"invalid_id_list":          res.invalid,
		"not_existed_id_list":      res.notExisted,
		"pending_approval_id_list": res.pendingApproval,
		"call_errors":              res.callErrors,
		"success_count":            successCount,
		"failure_count":            failureCount,
		"total":                    total,
	}

	if failureCount > 0 {
		return runtime.OutPartialFailure(outData, nil)
	}
	runtime.Out(outData, nil)
	return nil
}

// allAuthClassified reports whether every error in errsList classifies as an
// auth/permission-category typed error (errs.CategoryAuthentication or
// errs.CategoryAuthorization). An empty slice is not "all classified" — the
// caller only calls this when len(errsList) == 2, but the explicit false
// guards against a future call site passing an empty slice by mistake.
func allAuthClassified(errsList []error) bool {
	if len(errsList) == 0 {
		return false
	}
	for _, e := range errsList {
		cat := errs.CategoryOf(e)
		if cat != errs.CategoryAuthentication && cat != errs.CategoryAuthorization {
			return false
		}
	}
	return true
}

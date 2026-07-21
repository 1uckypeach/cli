// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"regexp"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

const (
	imChatMembersAddPathFormat = "/open-apis/im/v1/chats/%s/members"
	imChatMembersAddUserLimit  = 50
	imChatMembersAddBotLimit   = 5
	imChatMembersAddIDMaxBytes = 256
)

var imChatMembersAddIDSuffix = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type chatMembersAddSpec struct {
	ChatID string
	Users  []string
	Bots   []string
}

func readChatMembersAddSpec(runtime *common.RuntimeContext) (chatMembersAddSpec, error) {
	chatID, err := common.ValidateChatIDTyped("--chat-id", runtime.Str("chat-id"))
	if err != nil {
		return chatMembersAddSpec{}, err
	}
	if err := validateChatMembersAddID("--chat-id", chatID, "oc_"); err != nil {
		return chatMembersAddSpec{}, err
	}

	spec := chatMembersAddSpec{
		ChatID: chatID,
		Users:  dedupeChatMemberIDs(common.SplitCSV(runtime.Str("users"))),
		Bots:   dedupeChatMemberIDs(common.SplitCSV(runtime.Str("bots"))),
	}
	if len(spec.Users) == 0 && len(spec.Bots) == 0 {
		return chatMembersAddSpec{}, errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"specify at least one of --users or --bots",
		).WithParams(
			errs.InvalidParam{Name: "--users", Reason: "required; specify at least one"},
			errs.InvalidParam{Name: "--bots", Reason: "required; specify at least one"},
		)
	}
	if len(spec.Users) > imChatMembersAddUserLimit {
		return chatMembersAddSpec{}, errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"--users accepts at most %d unique IDs",
			imChatMembersAddUserLimit,
		).WithParam("--users")
	}
	if len(spec.Bots) > imChatMembersAddBotLimit {
		return chatMembersAddSpec{}, errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"--bots accepts at most %d unique IDs",
			imChatMembersAddBotLimit,
		).WithParam("--bots")
	}

	for _, id := range spec.Users {
		if err := validateChatMembersAddID("--users", id, "ou_"); err != nil {
			return chatMembersAddSpec{}, err
		}
		if _, err := common.ValidateUserIDTyped("--users", id); err != nil {
			return chatMembersAddSpec{}, err
		}
	}
	for _, id := range spec.Bots {
		if err := validateChatMembersAddID("--bots", id, "cli_"); err != nil {
			return chatMembersAddSpec{}, err
		}
	}

	return spec, nil
}

func dedupeChatMemberIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func validateChatMembersAddID(param, id, prefix string) error {
	if !strings.HasPrefix(id, prefix) {
		return errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"invalid %s: identifier must start with %s",
			param,
			prefix,
		).WithParam(param)
	}
	if len(id) > imChatMembersAddIDMaxBytes {
		return errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"invalid %s: identifier must not exceed %d bytes",
			param,
			imChatMembersAddIDMaxBytes,
		).WithParam(param)
	}

	suffix := strings.TrimPrefix(id, prefix)
	if suffix == "" {
		return errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"invalid %s: identifier suffix cannot be empty",
			param,
		).WithParam(param)
	}
	if !imChatMembersAddIDSuffix.MatchString(suffix) {
		return errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"invalid %s: identifier suffix must use only ASCII letters, digits, underscores, or hyphens",
			param,
		).WithParam(param)
	}
	return nil
}

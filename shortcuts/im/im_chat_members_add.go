// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/validate"
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

type chatMembersAddResponse struct {
	InvalidIDList         []string
	NotExistedIDList      []string
	PendingApprovalIDList []string
}

type chatMembersAddResult struct {
	ChatID                string               `json:"chat_id"`
	SuccessCount          int                  `json:"success_count"`
	InvalidIDList         []string             `json:"invalid_id_list"`
	NotExistedIDList      []string             `json:"not_existed_id_list"`
	PendingApprovalIDList []string             `json:"pending_approval_id_list"`
	FailedMemberType      string               `json:"failed_member_type,omitempty"`
	OutcomeUnknown        bool                 `json:"outcome_unknown,omitempty"`
	Error                 *chatMembersAddError `json:"error,omitempty"`
}

type chatMembersAddError struct {
	Type            errs.Category `json:"type"`
	Subtype         errs.Subtype  `json:"subtype,omitempty"`
	Code            int           `json:"code,omitempty"`
	Message         string        `json:"message"`
	Hint            string        `json:"hint,omitempty"`
	LogID           string        `json:"log_id,omitempty"`
	Troubleshooter  string        `json:"troubleshooter,omitempty"`
	Retryable       bool          `json:"retryable"`
	MissingScopes   []string      `json:"missing_scopes,omitempty"`
	RequestedScopes []string      `json:"requested_scopes,omitempty"`
	GrantedScopes   []string      `json:"granted_scopes,omitempty"`
	Identity        string        `json:"identity,omitempty"`
	ConsoleURL      string        `json:"console_url,omitempty"`
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

func buildChatMembersAddDryRun(spec chatMembersAddSpec) *common.DryRunAPI {
	dryRun := common.NewDryRunAPI()
	path := fmt.Sprintf(imChatMembersAddPathFormat, validate.EncodePathSegment(spec.ChatID))
	if len(spec.Users) > 0 {
		dryRun.POST(path).
			Params(chatMembersAddParams("open_id")).
			Body(chatMembersAddBody(spec.Users))
	}
	if len(spec.Bots) > 0 {
		dryRun.POST(path).
			Params(chatMembersAddParams("app_id")).
			Body(chatMembersAddBody(spec.Bots))
	}
	return dryRun
}

func executeChatMembersAdd(runtime *common.RuntimeContext, spec chatMembersAddSpec) error {
	responses := make([]chatMembersAddResponse, 0, 2)
	if len(spec.Users) > 0 {
		response, err := callChatMembersAddBatch(runtime, spec.ChatID, "open_id", spec.Users)
		if err != nil {
			return err
		}
		responses = append(responses, response)
	}
	if len(spec.Bots) > 0 {
		response, err := callChatMembersAddBatch(runtime, spec.ChatID, "app_id", spec.Bots)
		if err != nil {
			return err
		}
		responses = append(responses, response)
	}

	merged := mergeChatMembersAddResponse(responses...)
	result := chatMembersAddResult{
		ChatID:                spec.ChatID,
		SuccessCount:          confirmedChatMembersAddCount(len(spec.Users)+len(spec.Bots), merged),
		InvalidIDList:         merged.InvalidIDList,
		NotExistedIDList:      merged.NotExistedIDList,
		PendingApprovalIDList: merged.PendingApprovalIDList,
	}
	runtime.Out(result, nil)
	return nil
}

func callChatMembersAddBatch(
	runtime *common.RuntimeContext,
	chatID string,
	memberIDType string,
	ids []string,
) (chatMembersAddResponse, error) {
	path := fmt.Sprintf(imChatMembersAddPathFormat, validate.EncodePathSegment(chatID))
	data, err := runtime.CallAPITyped(
		http.MethodPost,
		path,
		chatMembersAddParams(memberIDType),
		chatMembersAddBody(ids),
	)
	if err != nil {
		return chatMembersAddResponse{}, err
	}
	return projectChatMembersAddResponse(data, ids)
}

func chatMembersAddParams(memberIDType string) map[string]interface{} {
	return map[string]interface{}{
		"member_id_type": memberIDType,
		"succeed_type":   "1",
	}
}

func chatMembersAddBody(ids []string) map[string]interface{} {
	return map[string]interface{}{
		"id_list": ids,
	}
}

func projectChatMembersAddResponse(data map[string]interface{}, requestedIDs []string) (chatMembersAddResponse, error) {
	response := chatMembersAddResponse{}
	fields := []struct {
		name   string
		target *[]string
	}{
		{name: "invalid_id_list", target: &response.InvalidIDList},
		{name: "not_existed_id_list", target: &response.NotExistedIDList},
		{name: "pending_approval_id_list", target: &response.PendingApprovalIDList},
	}

	requested := make(map[string]struct{}, len(requestedIDs))
	for _, id := range requestedIDs {
		requested[id] = struct{}{}
	}
	seen := make(map[string]struct{})

	for _, field := range fields {
		values, err := projectChatMembersAddList(data, field.name)
		if err != nil {
			return chatMembersAddResponse{}, err
		}
		for _, id := range values {
			if _, ok := requested[id]; !ok {
				return chatMembersAddResponse{}, newInvalidChatMembersAddResponseError(
					field.name,
					"contains an identifier outside the request",
				)
			}
			if _, ok := seen[id]; ok {
				return chatMembersAddResponse{}, newInvalidChatMembersAddResponseError(
					field.name,
					"contains a duplicate identifier",
				)
			}
			seen[id] = struct{}{}
		}
		*field.target = values
	}

	return response, nil
}

func projectChatMembersAddList(data map[string]interface{}, key string) ([]string, error) {
	raw, exists := data[key]
	if !exists {
		return []string{}, nil
	}

	switch values := raw.(type) {
	case []string:
		return append([]string{}, values...), nil
	case []interface{}:
		result := make([]string, 0, len(values))
		for _, value := range values {
			id, ok := value.(string)
			if !ok {
				return nil, newInvalidChatMembersAddResponseError(key, "contains a non-string element")
			}
			result = append(result, id)
		}
		return result, nil
	default:
		return nil, newInvalidChatMembersAddResponseError(key, "is not an array of strings")
	}
}

func newInvalidChatMembersAddResponseError(field, reason string) error {
	cause := fmt.Errorf("%s %s", field, reason)
	return errs.NewInternalError(
		errs.SubtypeInvalidResponse,
		"API returned invalid %s",
		field,
	).WithCause(cause)
}

func mergeChatMembersAddResponse(responses ...chatMembersAddResponse) chatMembersAddResponse {
	merged := chatMembersAddResponse{
		InvalidIDList:         []string{},
		NotExistedIDList:      []string{},
		PendingApprovalIDList: []string{},
	}
	for _, response := range responses {
		merged.InvalidIDList = append(merged.InvalidIDList, response.InvalidIDList...)
		merged.NotExistedIDList = append(merged.NotExistedIDList, response.NotExistedIDList...)
		merged.PendingApprovalIDList = append(merged.PendingApprovalIDList, response.PendingApprovalIDList...)
	}
	return merged
}

func confirmedChatMembersAddCount(requested int, response chatMembersAddResponse) int {
	if requested <= 0 {
		return 0
	}
	unfinished := len(response.InvalidIDList) + len(response.NotExistedIDList) + len(response.PendingApprovalIDList)
	confirmed := requested - unfinished
	if confirmed < 0 {
		return 0
	}
	if confirmed > requested {
		return requested
	}
	return confirmed
}

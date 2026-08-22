// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package vc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

const (
	meetingBotInvitePath = "/open-apis/vc/v1/bots/invite"

	meetingInviteTypeAllSuggested  = "ALL_SUGGESTED"
	meetingInviteTypeSelected      = "SELECTED"
	meetingInviteeLimit            = 200
	meetingInviteTypeAllValue      = 1
	meetingInviteTypeSelectedValue = 2
)

// VCMeetingInvite invites users through the Agent bot invite path.
var VCMeetingInvite = common.Shortcut{
	Service:     "vc",
	Command:     "+meeting-invite",
	Description: "Invite selected or all eligible users as the app bot",
	Risk:        "write",
	Scopes:      []string{"vc:meeting.bot.join:write"},
	AuthTypes:   []string{"bot"},
	HasFormat:   true,
	Flags: []common.Flag{
		{Name: "meeting-id", Required: true, Desc: "meeting ID"},
		{Name: "type", Required: true, Desc: "invite type", Enum: []string{meetingInviteTypeAllSuggested, meetingInviteTypeSelected}},
		{Name: "open-ids", Type: "string_slice", Desc: "user open_ids for SELECTED (maximum 200)"},
	},
	Normalize: func(_ context.Context, flags *common.FlagContext) error {
		return flags.SetCanonical("type", strings.ToUpper(strings.TrimSpace(flags.Str("type"))))
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		_, err := buildMeetingInviteBody(runtime)
		return err
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		body, err := buildMeetingInviteBody(runtime)
		if err != nil {
			return common.NewDryRunAPI().Set("error", err.Error())
		}
		return common.NewDryRunAPI().POST(meetingBotInvitePath).
			Params(buildMeetingInviteParams()).
			Body(body)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		body, err := buildMeetingInviteBody(runtime)
		if err != nil {
			return err
		}
		data, err := runtime.CallAPITyped(http.MethodPost, meetingBotInvitePath, buildMeetingInviteParams(), body)
		if err != nil {
			return err
		}
		if data == nil {
			data = map[string]interface{}{}
		}
		runtime.OutFormat(data, nil, func(w io.Writer) {
			printMeetingInviteResult(w, data)
		})
		return nil
	},
}

func buildMeetingInviteBody(runtime *common.RuntimeContext) (map[string]interface{}, error) {
	if err := validateMeetingIDFlag(runtime.Str("meeting-id")); err != nil {
		return nil, err
	}
	inviteType := strings.ToUpper(strings.TrimSpace(runtime.Str("type")))
	openIDs := normalizeMeetingInviteOpenIDs(runtime.StrSlice("open-ids"))
	var inviteTypeValue int
	switch inviteType {
	case meetingInviteTypeSelected:
		inviteTypeValue = meetingInviteTypeSelectedValue
		if len(openIDs) == 0 {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--open-ids is required when --type is SELECTED").WithParam("--open-ids")
		}
		if len(openIDs) > meetingInviteeLimit {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--open-ids accepts at most %d users, got %d", meetingInviteeLimit, len(openIDs)).WithParam("--open-ids")
		}
		for _, openID := range openIDs {
			if !strings.HasPrefix(openID, "ou_") {
				return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--open-ids only accepts user open_id values (ou_xxx)").WithParam("--open-ids")
			}
		}
	case meetingInviteTypeAllSuggested:
		inviteTypeValue = meetingInviteTypeAllValue
		if len(openIDs) != 0 {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--open-ids must not be set when --type is ALL_SUGGESTED").WithParam("--open-ids")
		}
	case "":
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--type is required").WithParam("--type")
	default:
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--type must be ALL_SUGGESTED or SELECTED, got %q", runtime.Str("type")).WithParam("--type")
	}

	body := map[string]interface{}{
		"meeting_id":  strings.TrimSpace(runtime.Str("meeting-id")),
		"invite_type": inviteTypeValue,
	}
	if inviteType == meetingInviteTypeSelected {
		body["invitees"] = buildMeetingInviteUsers(openIDs)
	}
	return body, nil
}

func buildMeetingInviteParams() map[string]interface{} {
	return map[string]interface{}{"user_id_type": "open_id"}
}

func printMeetingInviteResult(w io.Writer, data map[string]interface{}) {
	fmt.Fprintln(w, "Invite request sent.")
	if failedCount, ok := common.GetFloatOK(data, "failed_count"); ok {
		fmt.Fprintf(w, "  Failed:   %d\n", int(failedCount))
	}
	if invitedCount, ok := common.GetFloatOK(data, "invited_count"); ok {
		fmt.Fprintf(w, "  Invited:  %d\n", int(invitedCount))
	}
	if common.GetBool(data, "has_more") {
		fmt.Fprintln(w, "  Has more: true")
	}
	results, _ := data["invite_results"].([]interface{})
	if len(results) > 0 {
		fmt.Fprintf(w, "  Results:  %d users\n", len(results))
	}
}

func buildMeetingInviteUsers(openIDs []string) []map[string]interface{} {
	invitees := make([]map[string]interface{}, 0, len(openIDs))
	for _, openID := range openIDs {
		invitees = append(invitees, map[string]interface{}{
			"id":        openID,
			"user_type": 1,
		})
	}
	return invitees
}

func normalizeMeetingInviteOpenIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	openIDs := make([]string, 0, len(values))
	for _, value := range values {
		openID := strings.TrimSpace(value)
		if openID == "" {
			continue
		}
		if _, ok := seen[openID]; ok {
			continue
		}
		seen[openID] = struct{}{}
		openIDs = append(openIDs, openID)
	}
	return openIDs
}

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
	meetingBotStartPath  = "/open-apis/vc/v1/bots/join"
	meetingBotInvitePath = "/open-apis/vc/v1/bots/invite"
	meetingBotEndPath    = "/open-apis/vc/v1/bots/end"

	meetingInviteTypeAllSuggested  = "ALL_SUGGESTED"
	meetingInviteTypeSelected      = "SELECTED"
	meetingInviteeLimit            = 200
	meetingInviteTypeAllValue      = 1
	meetingInviteTypeSelectedValue = 2
)

// VCMeetingStart starts and joins a Calendar meeting as the app bot.
var VCMeetingStart = common.Shortcut{
	Service:     "vc",
	Command:     "+meeting-start",
	Description: "Start and join a Calendar meeting as the app bot",
	Risk:        "write",
	Scopes:      []string{"vc:meeting.bot.join:write"},
	AuthTypes:   []string{"bot"},
	HasFormat:   true,
	Flags: []common.Flag{
		{Name: "meeting-number", Required: true, Desc: "Calendar meeting number to start"},
		{Name: "password", Desc: "meeting password (if required)"},
		{Name: "call-id", Desc: "correlation id forwarded from invite event"},
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		return validateMeetingStart(runtime)
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		body, err := buildMeetingStartBody(runtime)
		if err != nil {
			return common.NewDryRunAPI().Set("error", err.Error())
		}
		return common.NewDryRunAPI().POST(meetingBotStartPath).Body(body)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		body, err := buildMeetingStartBody(runtime)
		if err != nil {
			return err
		}
		data, err := runtime.CallAPITyped(http.MethodPost, meetingBotStartPath, nil, body)
		if err != nil {
			return err
		}
		if data == nil {
			data = map[string]interface{}{}
		}
		runtime.OutFormat(data, nil, func(w io.Writer) {
			fmt.Fprintln(w, "Started and joined meeting.")
			printMeetingInfo(w, data)
		})
		return nil
	},
}

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
		return common.NewDryRunAPI().POST(meetingBotInvitePath).Body(body)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		body, err := buildMeetingInviteBody(runtime)
		if err != nil {
			return err
		}
		data, err := runtime.CallAPITyped(http.MethodPost, meetingBotInvitePath, nil, body)
		if err != nil {
			return err
		}
		if data == nil {
			data = map[string]interface{}{}
		}
		runtime.OutFormat(data, nil, func(w io.Writer) {
			fmt.Fprintln(w, "Invite request sent.")
			if failed := common.GetString(data, "failed_count"); failed != "" {
				fmt.Fprintf(w, "  Failed:  %s\n", failed)
			}
		})
		return nil
	},
}

// VCMeetingEnd ends a meeting as the app bot when that bot is the current host.
var VCMeetingEnd = common.Shortcut{
	Service:     "vc",
	Command:     "+meeting-end",
	Description: "End a meeting as the host app bot",
	Risk:        "write",
	Scopes:      []string{"vc:meeting.bot.manage:write"},
	AuthTypes:   []string{"bot"},
	HasFormat:   true,
	Flags: []common.Flag{
		{Name: "meeting-id", Required: true, Desc: "meeting ID to end"},
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		return validateMeetingIDFlag(runtime.Str("meeting-id"))
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		body, err := buildMeetingEndBody(runtime)
		if err != nil {
			return common.NewDryRunAPI().Set("error", err.Error())
		}
		return common.NewDryRunAPI().POST(meetingBotEndPath).Body(body)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		body, err := buildMeetingEndBody(runtime)
		if err != nil {
			return err
		}
		data, err := runtime.CallAPITyped(http.MethodPost, meetingBotEndPath, nil, body)
		if err != nil {
			return err
		}
		if data == nil {
			data = map[string]interface{}{}
		}
		runtime.OutFormat(data, nil, func(w io.Writer) {
			fmt.Fprintf(w, "Ended meeting %s.\n", strings.TrimSpace(runtime.Str("meeting-id")))
		})
		return nil
	},
}

func buildMeetingStartBody(runtime *common.RuntimeContext) (map[string]interface{}, error) {
	if err := validateMeetingStart(runtime); err != nil {
		return nil, err
	}
	body := buildMeetingJoinBody(runtime)
	body["action"] = 2
	return body, nil
}

func validateMeetingStart(runtime *common.RuntimeContext) error {
	return VCMeetingJoin.Validate(context.Background(), runtime)
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
		"meeting_id": strings.TrimSpace(runtime.Str("meeting-id")),
		"type":       inviteTypeValue,
	}
	if inviteType == meetingInviteTypeSelected {
		body["open_ids"] = openIDs
	}
	return body, nil
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

func buildMeetingEndBody(runtime *common.RuntimeContext) (map[string]interface{}, error) {
	if err := validateMeetingIDFlag(runtime.Str("meeting-id")); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"meeting_id": strings.TrimSpace(runtime.Str("meeting-id")),
	}, nil
}

func validateMeetingIDFlag(value string) error {
	meetingID := strings.TrimSpace(value)
	if meetingID == "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--meeting-id is required").WithParam("--meeting-id")
	}
	if !isDigits(meetingID) {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--meeting-id must be numeric").WithParam("--meeting-id")
	}
	return nil
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func printMeetingInfo(w io.Writer, data map[string]interface{}) {
	meeting, _ := data["meeting"].(map[string]interface{})
	if meeting == nil {
		return
	}
	if id := common.GetString(meeting, "id"); id != "" {
		fmt.Fprintf(w, "  Meeting ID:  %s\n", id)
	}
	if no := common.GetString(meeting, "meeting_no"); no != "" {
		fmt.Fprintf(w, "  Meeting No:  %s\n", no)
	}
	if topic := common.GetString(meeting, "topic"); topic != "" {
		fmt.Fprintf(w, "  Topic:       %s\n", topic)
	}
}

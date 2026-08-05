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

	meetingInviteScopeSelected = "selected"
	meetingInviteScopeAll      = "all"
	meetingInviteeLimit        = 200
)

// VCMeetingStart probes the external START_AND_JOIN API as the app bot.
var VCMeetingStart = common.Shortcut{
	Service:     "vc",
	Command:     "+meeting-start",
	Description: "Probe Calendar START_AND_JOIN as the app bot",
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
		{Name: "scope", Required: true, Desc: "invite scope: SELECTED or ALL"},
		{Name: "invitee-user-ids", Desc: "comma-separated numeric user IDs for SELECTED scope (maximum 200)"},
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
	Scopes:      []string{"vc:meeting.bot.join:write"},
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
	body["meeting_action"] = "start_and_join"
	return body, nil
}

func validateMeetingStart(runtime *common.RuntimeContext) error {
	return VCMeetingJoin.Validate(context.Background(), runtime)
}

func buildMeetingInviteBody(runtime *common.RuntimeContext) (map[string]interface{}, error) {
	if err := validateMeetingIDFlag(runtime.Str("meeting-id")); err != nil {
		return nil, err
	}
	scope := strings.ToLower(strings.TrimSpace(runtime.Str("scope")))
	ids, err := parseMeetingInviteeUserIDs(runtime.Str("invitee-user-ids"))
	if err != nil {
		return nil, err
	}
	switch scope {
	case meetingInviteScopeSelected:
		if len(ids) == 0 {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--invitee-user-ids is required when --scope is SELECTED").WithParam("--invitee-user-ids")
		}
		if len(ids) > meetingInviteeLimit {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--invitee-user-ids accepts at most %d users, got %d", meetingInviteeLimit, len(ids)).WithParam("--invitee-user-ids")
		}
	case meetingInviteScopeAll:
		if len(ids) != 0 {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--invitee-user-ids must not be set when --scope is ALL").WithParam("--invitee-user-ids")
		}
	case "":
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--scope is required").WithParam("--scope")
	default:
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--scope must be SELECTED or ALL, got %q", runtime.Str("scope")).WithParam("--scope")
	}

	invitees := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		if !isDigits(id) {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--invitee-user-ids must contain numeric user IDs").WithParam("--invitee-user-ids")
		}
		invitees = append(invitees, map[string]interface{}{
			"id":        id,
			"user_type": 1,
		})
	}
	body := map[string]interface{}{
		"meeting_id": strings.TrimSpace(runtime.Str("meeting-id")),
		"scope":      scope,
	}
	if scope == meetingInviteScopeSelected {
		body["invitees"] = invitees
	}
	return body, nil
}

func parseMeetingInviteeUserIDs(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--invitee-user-ids must not contain empty values").WithParam("--invitee-user-ids")
		}
		ids = append(ids, id)
	}
	return ids, nil
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

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package vc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

const (
	meetingBotJoinPath      = "/open-apis/vc/v1/bots/join"
	meetingJoinActionJoin   = "join"
	meetingJoinActionStart  = "start"
	meetingJoinStartAPIFlag = 2
)

var meetingNumberRe = regexp.MustCompile(`^\d{9}$`)

type meetingJoinIdentify struct {
	MeetingNo string `json:"meeting_no"`
}

type meetingJoinRequest struct {
	JoinType     int                 `json:"join_type"`
	JoinIdentify meetingJoinIdentify `json:"join_identify"`
	Password     string              `json:"password,omitempty"`
	CallID       string              `json:"call_id,omitempty"`
	Action       *int                `json:"action,omitempty"`
}

// validMeetingNumber checks whether s is a valid 9-digit meeting number.
func validMeetingNumber(s string) bool {
	return meetingNumberRe.MatchString(s)
}

// VCMeetingJoin joins a meeting by meeting number via /vc/v1/bots/join.
var VCMeetingJoin = common.Shortcut{
	Service:     "vc",
	Command:     "+meeting-join",
	Description: "Join a meeting by meeting number (bot join)",
	Risk:        "write",
	Scopes:      []string{"vc:meeting.bot.join:write"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags: []common.Flag{
		{Name: "meeting-number", Required: true, Desc: "meeting number to join"},
		{Name: "password", Desc: "meeting password (if required)"},
		{Name: "call-id", Desc: "correlation id forwarded from invite event"},
		{Name: "action", Default: meetingJoinActionJoin, Desc: "meeting action (default: join; start initiates a Calendar meeting)", Enum: []string{meetingJoinActionJoin, meetingJoinActionStart}},
	},
	Normalize: func(_ context.Context, flags *common.FlagContext) error {
		return flags.SetCanonical("action", strings.ToLower(strings.TrimSpace(flags.Str("action"))))
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		mn := strings.TrimSpace(runtime.Str("meeting-number"))
		if !validMeetingNumber(mn) {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--meeting-number must be exactly 9 digits, got %q", mn).WithParam("--meeting-number")
		}
		if meetingJoinAction(runtime) == meetingJoinActionStart && !runtime.As().IsBot() {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--action start requires --as bot").WithParam("--action")
		}
		return nil
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		return common.NewDryRunAPI().
			POST(meetingBotJoinPath).
			Body(buildMeetingJoinBody(runtime, meetingJoinAction(runtime)))
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		action := meetingJoinAction(runtime)
		data, err := runtime.CallAPITyped("POST", meetingBotJoinPath, nil, buildMeetingJoinBody(runtime, action))
		if err != nil {
			return err
		}
		if data == nil {
			data = map[string]interface{}{}
		}
		runtime.OutFormat(data, nil, func(w io.Writer) {
			meeting, _ := data["meeting"].(map[string]interface{})
			if meeting == nil {
				if action == meetingJoinActionStart {
					fmt.Fprintln(w, "Started Calendar meeting (no meeting info returned).")
				} else {
					fmt.Fprintln(w, "Joined meeting (no meeting info returned).")
				}
				return
			}
			if action == meetingJoinActionStart {
				fmt.Fprintln(w, "Started Calendar meeting.")
			} else {
				fmt.Fprintln(w, "Joined meeting successfully.")
			}
			printMeetingSummary(w, data)
			if startTime := common.GetString(meeting, "start_time"); startTime != "" {
				fmt.Fprintf(w, "  Start Time:  %s\n", startTime)
			}
		})
		return nil
	},
}

func buildMeetingJoinBody(runtime *common.RuntimeContext, action string) meetingJoinRequest {
	body := meetingJoinRequest{
		JoinType:     1,
		JoinIdentify: meetingJoinIdentify{MeetingNo: strings.TrimSpace(runtime.Str("meeting-number"))},
	}
	if pw := strings.TrimSpace(runtime.Str("password")); pw != "" {
		body.Password = pw
	}
	if cid := strings.TrimSpace(runtime.Str("call-id")); cid != "" {
		body.CallID = cid
	}
	if action == meetingJoinActionStart {
		startAction := meetingJoinStartAPIFlag
		body.Action = &startAction
	}
	return body
}

func meetingJoinAction(runtime *common.RuntimeContext) string {
	if runtime.Str("action") == meetingJoinActionStart {
		return meetingJoinActionStart
	}
	return meetingJoinActionJoin
}

func printMeetingSummary(w io.Writer, data map[string]interface{}) {
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

const (
	meetingBotInvitePath = "/open-apis/vc/v1/bots/invite"
	meetingBotEndPath    = "/open-apis/vc/v1/bots/end"

	meetingInviteTypeAllSuggested  = "ALL_SUGGESTED"
	meetingInviteTypeSelected      = "SELECTED"
	meetingInviteeLimit            = 200
	meetingInviteTypeAllValue      = 1
	meetingInviteTypeSelectedValue = 2
	meetingInviteeUserType         = 1
	meetingInviteStatusSucceeded   = 1
	meetingInviteStatusFailed      = 2

	meetingInviteCandidateLimitNotice = "Some eligible candidates were not invited because the service limit is 200."
)

var meetingIDRe = regexp.MustCompile(`^\d+$`)

type meetingInvitee struct {
	ID       string `json:"id"`
	UserType int    `json:"user_type"`
}

type meetingInviteRequest struct {
	MeetingID  string           `json:"meeting_id"`
	InviteType int              `json:"invite_type"`
	Invitees   []meetingInvitee `json:"invitees,omitempty"`
}

type meetingEndRequest struct {
	MeetingID string `json:"meeting_id"`
}

type meetingInviteResult struct {
	ID     string
	Status int
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
		inviteType := strings.ToUpper(strings.TrimSpace(flags.Str("type")))
		if inviteType == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--type is required").WithParam("--type")
		}
		return flags.SetCanonical("type", inviteType)
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		if err := validateMeetingIDFlag(runtime.Str("meeting-id")); err != nil {
			return err
		}
		return validateMeetingInviteFlags(runtime)
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		return common.NewDryRunAPI().POST(meetingBotInvitePath).
			Params(buildMeetingInviteParams()).
			Body(buildMeetingInviteBody(runtime))
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		data, err := runtime.CallAPITyped(http.MethodPost, meetingBotInvitePath, buildMeetingInviteParams(), buildMeetingInviteBody(runtime))
		if err != nil {
			return err
		}
		if data == nil {
			data = map[string]interface{}{}
		}
		output := buildMeetingInviteOutput(data)
		runtime.OutFormat(output, nil, func(w io.Writer) {
			printMeetingInviteResult(w, output)
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
		return common.NewDryRunAPI().POST(meetingBotEndPath).Body(buildMeetingEndBody(runtime))
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		data, err := runtime.CallAPITyped(http.MethodPost, meetingBotEndPath, nil, buildMeetingEndBody(runtime))
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

func validateMeetingInviteFlags(runtime *common.RuntimeContext) error {
	openIDs := normalizeMeetingInviteOpenIDs(runtime.StrSlice("open-ids"))
	switch runtime.Str("type") {
	case meetingInviteTypeSelected:
		if len(openIDs) == 0 {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--open-ids is required when --type is SELECTED").WithParam("--open-ids")
		}
		if len(openIDs) > meetingInviteeLimit {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--open-ids accepts at most %d users, got %d", meetingInviteeLimit, len(openIDs)).WithParam("--open-ids")
		}
		for _, openID := range openIDs {
			if !strings.HasPrefix(openID, "ou_") {
				return errs.NewValidationError(errs.SubtypeInvalidArgument, "--open-ids only accepts user open_id values (ou_xxx)").WithParam("--open-ids")
			}
		}
	case meetingInviteTypeAllSuggested:
		if len(openIDs) != 0 {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--open-ids must not be set when --type is ALL_SUGGESTED").WithParam("--open-ids")
		}
	}
	return nil
}

func buildMeetingInviteBody(runtime *common.RuntimeContext) meetingInviteRequest {
	body := meetingInviteRequest{
		MeetingID: strings.TrimSpace(runtime.Str("meeting-id")),
	}
	if runtime.Str("type") == meetingInviteTypeSelected {
		body.InviteType = meetingInviteTypeSelectedValue
		body.Invitees = buildMeetingInviteUsers(normalizeMeetingInviteOpenIDs(runtime.StrSlice("open-ids")))
	} else {
		body.InviteType = meetingInviteTypeAllValue
	}
	return body
}

func buildMeetingInviteParams() map[string]interface{} {
	return map[string]interface{}{"user_id_type": "open_id"}
}

func buildMeetingInviteOutput(data map[string]interface{}) map[string]interface{} {
	output := make(map[string]interface{}, len(data)+1)
	for key, value := range data {
		output[key] = value
	}
	if common.GetBool(data, "has_more") {
		output["notice"] = meetingInviteCandidateLimitNotice
	}
	delete(output, "has_more")
	return output
}

func printMeetingInviteResult(w io.Writer, data map[string]interface{}) {
	fmt.Fprintln(w, "Invite request sent.")
	if failedCount, ok := common.GetFloatOK(data, "failed_count"); ok {
		fmt.Fprintf(w, "  Failed:   %d\n", int(failedCount))
	}
	if invitedCount, ok := common.GetFloatOK(data, "invited_count"); ok {
		fmt.Fprintf(w, "  Invited:  %d\n", int(invitedCount))
	}
	if notice := common.GetString(data, "notice"); notice != "" {
		fmt.Fprintf(w, "  Note: %s\n", notice)
	}
	results := meetingInviteResults(data)
	if len(results) > 0 {
		fmt.Fprintln(w, "  Invite results:")
		for _, result := range results {
			fmt.Fprintf(w, "    %s: %s\n", result.ID, meetingInviteStatusLabel(result.Status))
		}
	}
}

func meetingInviteResults(data map[string]interface{}) []meetingInviteResult {
	items, _ := data["invite_results"].([]interface{})
	results := make([]meetingInviteResult, 0, len(items))
	common.EachMap(items, func(item map[string]interface{}) {
		status, ok := common.GetFloatOK(item, "status")
		if id := common.GetStringLoose(item, "id"); id != "" && ok {
			results = append(results, meetingInviteResult{ID: id, Status: int(status)})
		}
	})
	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})
	return results
}

func meetingInviteStatusLabel(status int) string {
	switch status {
	case meetingInviteStatusSucceeded:
		return "invited"
	case meetingInviteStatusFailed:
		return "failed"
	default:
		return fmt.Sprintf("unknown (%d)", status)
	}
}

func buildMeetingInviteUsers(openIDs []string) []meetingInvitee {
	invitees := make([]meetingInvitee, 0, len(openIDs))
	for _, openID := range openIDs {
		invitees = append(invitees, meetingInvitee{
			ID:       openID,
			UserType: meetingInviteeUserType,
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

func buildMeetingEndBody(runtime *common.RuntimeContext) meetingEndRequest {
	return meetingEndRequest{MeetingID: strings.TrimSpace(runtime.Str("meeting-id"))}
}

func validateMeetingIDFlag(value string) error {
	meetingID := strings.TrimSpace(value)
	if meetingID == "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--meeting-id is required").WithParam("--meeting-id")
	}
	if !meetingIDRe.MatchString(meetingID) || strings.TrimLeft(meetingID, "0") == "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--meeting-id must be a positive integer").WithParam("--meeting-id")
	}
	if _, err := strconv.ParseInt(meetingID, 10, 64); err != nil {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--meeting-id is out of range").WithParam("--meeting-id")
	}
	return nil
}

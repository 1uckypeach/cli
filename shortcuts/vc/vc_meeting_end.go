// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package vc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

const meetingBotEndPath = "/open-apis/vc/v1/bots/end"

var meetingIDRe = regexp.MustCompile(`^\d+$`)

type meetingEndRequest struct {
	MeetingID string `json:"meeting_id"`
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

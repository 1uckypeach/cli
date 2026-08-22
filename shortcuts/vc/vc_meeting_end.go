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

const meetingBotEndPath = "/open-apis/vc/v1/bots/end"

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

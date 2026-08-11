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
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

const (
	meetingCountdownActionSet          = "set"
	meetingCountdownActionProlong      = "prolong"
	meetingCountdownActionEndInAdvance = "end_in_advance"
	meetingCountdownActionClose        = "close"
)

var meetingCountdownActions = []string{
	meetingCountdownActionSet,
	meetingCountdownActionProlong,
	meetingCountdownActionEndInAdvance,
	meetingCountdownActionClose,
}

// VCMeetingCountdown operates an in-meeting countdown.
var VCMeetingCountdown = common.Shortcut{
	Service:     "vc",
	Command:     "+meeting-countdown",
	Description: "Operate an in-meeting countdown",
	Risk:        "write",
	Scopes:      []string{"vc:meeting"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags: []common.Flag{
		{Name: "meeting-id", Required: true, Desc: "meeting ID to operate"},
		{Name: "action", Required: true, Desc: "countdown action: set, prolong, end_in_advance, or close", Enum: meetingCountdownActions},
		{Name: "duration", Type: "int", Desc: "countdown duration in minutes; required for set and prolong"},
		{Name: "need-play-audio-at-end", Type: "bool", Desc: "play audio when a set countdown ends"},
		{Name: "reminders-before-end-in-second", Type: "int_array", Desc: "reminder offsets in seconds before countdown end; repeat or use CSV"},
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		if err := validateMeetingEventsMeetingID(runtime.Str("meeting-id")); err != nil {
			return err
		}
		_, err := buildMeetingCountdownBody(runtime)
		return err
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		body, err := buildMeetingCountdownBody(runtime)
		if err != nil {
			return common.NewDryRunAPI().Set("error", err.Error())
		}
		return common.NewDryRunAPI().
			POST(buildMeetingCountdownPath(runtime.Str("meeting-id"))).
			Body(body)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		body, err := buildMeetingCountdownBody(runtime)
		if err != nil {
			return err
		}
		action, _ := body["action"].(string)
		data, err := runtime.CallAPITyped(http.MethodPost, buildMeetingCountdownPath(runtime.Str("meeting-id")), nil, body)
		if err != nil {
			return err
		}
		if data == nil {
			data = map[string]interface{}{}
		}
		runtime.OutFormat(data, nil, func(w io.Writer) {
			fmt.Fprintln(w, "Meeting countdown operated.")
			fmt.Fprintf(w, "  Action:  %s\n", action)
		})
		return nil
	},
}

func buildMeetingCountdownPath(meetingID string) string {
	return fmt.Sprintf("/open-apis/vc/v1/meetings/%s/countdown", validate.EncodePathSegment(strings.TrimSpace(meetingID)))
}

func buildMeetingCountdownBody(runtime *common.RuntimeContext) (map[string]interface{}, error) {
	action := strings.ToLower(strings.TrimSpace(runtime.Str("action")))
	if err := validateMeetingCountdownAction(action); err != nil {
		return nil, err
	}

	duration := runtime.Int("duration")
	reminders := runtime.IntArray("reminders-before-end-in-second")
	if err := validateMeetingCountdownDuration(action, duration); err != nil {
		return nil, err
	}
	if err := validateMeetingCountdownReminders(action, duration, reminders); err != nil {
		return nil, err
	}
	if action != meetingCountdownActionSet && runtime.Bool("need-play-audio-at-end") {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--need-play-audio-at-end is only supported when --action set").WithParam("--need-play-audio-at-end")
	}

	body := map[string]interface{}{
		"action": action,
	}
	if action == meetingCountdownActionSet || action == meetingCountdownActionProlong {
		body["duration"] = duration
	}
	if action == meetingCountdownActionSet {
		if runtime.Bool("need-play-audio-at-end") {
			body["need_play_audio_at_end"] = true
		}
		if len(reminders) > 0 {
			values := make([]int, 0, len(reminders))
			values = append(values, reminders...)
			body["reminders_before_end_in_second"] = values
		}
	}
	return body, nil
}

func validateMeetingCountdownAction(action string) error {
	switch action {
	case meetingCountdownActionSet, meetingCountdownActionProlong, meetingCountdownActionEndInAdvance, meetingCountdownActionClose:
		return nil
	case "":
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--action is required").WithParam("--action")
	default:
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--action must be one of set, prolong, end_in_advance, close").WithParam("--action")
	}
}

func validateMeetingCountdownDuration(action string, duration int) error {
	switch action {
	case meetingCountdownActionSet, meetingCountdownActionProlong:
		if duration <= 0 {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--duration must be a positive number of minutes when --action is set or prolong").WithParam("--duration")
		}
	default:
		if duration != 0 {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--duration is only supported when --action is set or prolong").WithParam("--duration")
		}
	}
	return nil
}

func validateMeetingCountdownReminders(action string, duration int, reminders []int) error {
	if len(reminders) == 0 {
		return nil
	}
	if action != meetingCountdownActionSet {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--reminders-before-end-in-second is only supported when --action set").WithParam("--reminders-before-end-in-second")
	}
	durationSeconds := duration * 60
	for _, reminder := range reminders {
		if reminder <= 0 {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--reminders-before-end-in-second values must be positive seconds").WithParam("--reminders-before-end-in-second")
		}
		if reminder >= durationSeconds {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--reminders-before-end-in-second values must be less than --duration converted to seconds").WithParam("--reminders-before-end-in-second")
		}
	}
	return nil
}

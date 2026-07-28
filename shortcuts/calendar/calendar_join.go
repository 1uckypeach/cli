// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package calendar

import (
	"context"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

const joinCalendarEventPath = "/open-apis/calendar/v4/events/join"

func resolveJoinCredential(runtime *common.RuntimeContext) string {
	if token := strings.TrimSpace(runtime.Str("join-token")); token != "" {
		return token
	}
	return strings.TrimSpace(runtime.Str("share-link"))
}

var CalendarJoin = common.Shortcut{
	Service:     "calendar",
	Command:     "+join",
	Description: "Join a calendar event via the encrypted join token from an RSVP/share card, or via a shared meeting/event link",
	Risk:        "write",
	Scopes:      []string{"calendar:calendar.event:writeonly"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   false,
	Flags: []common.Flag{
		{Name: "join-token", Desc: "encrypted join token issued with the RSVP/share card"},
		{Name: "share-link", Desc: "shared meeting/event link (…/calendar/share?token=xxx) or the bare share token"},
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		return common.NewDryRunAPI().
			POST(joinCalendarEventPath).
			Body(map[string]interface{}{"join_token": resolveJoinCredential(runtime)})
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		if err := rejectCalendarAutoBotFallback(runtime); err != nil {
			return err
		}
		token := strings.TrimSpace(runtime.Str("join-token"))
		shareLink := strings.TrimSpace(runtime.Str("share-link"))
		if token == "" && shareLink == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"one of --join-token or --share-link is required").WithParam("--join-token")
		}
		if token != "" && shareLink != "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"--join-token and --share-link are mutually exclusive, pass only one").WithParam("--share-link")
		}
		if token != "" {
			if err := common.RejectDangerousCharsTyped("--join-token", token); err != nil {
				return err
			}
		}
		if shareLink != "" {
			if err := common.RejectDangerousCharsTyped("--share-link", shareLink); err != nil {
				return err
			}
		}
		return nil
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		credential := resolveJoinCredential(runtime)

		data, err := runtime.CallAPITyped("POST",
			joinCalendarEventPath,
			nil,
			map[string]interface{}{
				"join_token": credential,
			})
		if err != nil {
			return err
		}

		eventID, _ := data["event_id"].(string)
		runtime.Out(map[string]interface{}{
			"event_id": eventID,
		}, nil)
		return nil
	},
}

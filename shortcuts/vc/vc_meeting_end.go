// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package vc

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

const vcMeetingEndPathFormat = "/open-apis/vc/v1/meetings/%s/end"

// VCMeetingEnd ends an ongoing meeting.
var VCMeetingEnd = common.Shortcut{
	Service:                   "vc",
	Command:                   "+meeting-end",
	Description:               "End an ongoing meeting",
	Risk:                      "high-risk-write",
	ConfirmationBeforeNetwork: true,
	ConditionalUserScopes:     []string{"vc:meeting"},
	AuthTypes:                 []string{"user"},
	HasFormat:                 true,
	Flags: []common.Flag{
		{Name: "meeting-id", Required: true, Desc: "positive integer meeting ID to end"},
	},
	Validate: func(_ context.Context, runtime *common.RuntimeContext) error {
		return validateMeetingManagementID(runtime.Str("meeting-id"))
	},
	DryRun: func(_ context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		return common.NewDryRunAPI().
			PATCH(buildMeetingEndPath(runtime.Str("meeting-id")))
	},
	Execute: func(_ context.Context, runtime *common.RuntimeContext) error {
		meetingID := strings.TrimSpace(runtime.Str("meeting-id"))
		if err := runtime.EnsureScopes([]string{"vc:meeting"}); err != nil {
			return err
		}
		envelope, _, err := callMeetingManagementAPIEnvelope(runtime, http.MethodPatch, buildMeetingEndPath(meetingID), nil)
		if err != nil {
			return err
		}
		runtime.OutFormat(envelope, nil, nil)
		return nil
	},
}

func validateMeetingManagementID(meetingID string) error {
	meetingID = strings.TrimSpace(meetingID)
	value, err := strconv.ParseInt(meetingID, 10, 64)
	if err != nil || value <= 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--meeting-id must be a positive base-10 int64").WithParam("--meeting-id")
	}
	return nil
}

func buildMeetingEndPath(meetingID string) string {
	return fmt.Sprintf(vcMeetingEndPathFormat, validate.EncodePathSegment(strings.TrimSpace(meetingID)))
}

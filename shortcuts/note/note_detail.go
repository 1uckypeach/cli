// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT
//
// note +detail — get note metadata and document tokens by a known note_id.

package note

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

type noteDetailArgs struct {
	NoteID string `flag:"note-id" schema:"required;minLength=1" doc:"note ID"`
}

type noteDetailData struct {
	Note noteDetailOutput `json:"note" schema:"required" doc:"note detail"`
}

type noteDetailOutput struct {
	CreateTime       string   `json:"create_time" schema:"required" doc:"note creation time in local time when returned"`
	CreatorID        string   `json:"creator_id" schema:"required" doc:"creator open ID"`
	DisplayType      string   `json:"note_display_type" schema:"required;enum=unknown|normal|unified" doc:"note display type"`
	NoteDocToken     string   `json:"note_doc_token" schema:"required" doc:"main note document token"`
	NoteID           string   `json:"note_id" schema:"required" doc:"note ID"`
	SharedDocTokens  []string `json:"shared_doc_tokens,omitempty" schema:"optional;nonnullable" doc:"shared document tokens"`
	VerbatimDocToken string   `json:"verbatim_doc_token" schema:"required" doc:"verbatim transcript document token"`
}

// NoteDetail queries note metadata, display type and document tokens by note_id.
var NoteDetail = common.Define(common.Definition[noteDetailArgs, noteDetailData]{
	Metadata: common.CommandMetadata{
		Service: "note", Command: "+detail", Description: "Get note detail (display type, document tokens) by note_id", Risk: common.RiskRead,
		Authorization: common.AuthorizationDefinition{Identities: map[common.Identity]common.IdentityAuthorization{
			common.IdentityUser: {RequiredScopes: []string{"vc:note:read"}},
		}},
	},
	Hooks: common.Hooks[noteDetailArgs, noteDetailData]{
		Normalize: func(_ context.Context, _ common.CommandContext, args *noteDetailArgs) error {
			args.NoteID = strings.TrimSpace(args.NoteID)
			return nil
		},
		Validate: func(_ context.Context, _ common.CommandContext, args *noteDetailArgs) error {
			if args.NoteID == "" {
				return errs.NewValidationError(errs.SubtypeInvalidArgument, "--note-id is required").WithParam("--note-id")
			}
			if err := validate.ResourceName(args.NoteID, "--note-id"); err != nil {
				return errs.NewValidationError(errs.SubtypeInvalidArgument, "%s", err).WithParam("--note-id").WithCause(err)
			}
			return nil
		},
		DryRun: func(_ context.Context, _ common.CommandContext, args *noteDetailArgs) *common.DryRunAPI {
			return common.NewDryRunAPI().GET(fmt.Sprintf("/open-apis/vc/v1/notes/%s", validate.EncodePathSegment(args.NoteID)))
		},
		Execute: func(ctx context.Context, command common.CommandContext, args *noteDetailArgs) (common.Result[noteDetailData], error) {
			data, err := common.DoTypedAPIJSON(ctx, command, http.MethodGet, fmt.Sprintf("/open-apis/vc/v1/notes/%s", validate.EncodePathSegment(args.NoteID)), nil, nil)
			if err != nil {
				return common.Result[noteDetailData]{}, mapNoteError(err)
			}
			detail, err := parseDetailData(args.NoteID, data)
			if err != nil {
				return common.Result[noteDetailData]{}, mapNoteError(err)
			}
			return common.Success(noteDetailData{Note: noteDetailOutput{
				CreateTime: detail.CreateTime, CreatorID: detail.CreatorID, DisplayType: detail.DisplayType,
				NoteDocToken: detail.NoteDocToken, NoteID: detail.NoteID, SharedDocTokens: detail.SharedDocTokens,
				VerbatimDocToken: detail.VerbatimDocToken,
			}}), nil
		},
	},
})

// mapNoteError surfaces the no-permission case explicitly and passes through
// any other typed API error unchanged.
func mapNoteError(err error) error {
	if problem, ok := errs.ProblemOf(err); ok && problem.Code == NoNoteReadPermissionCode {
		message := strings.TrimSpace(problem.Message)
		if message == "" {
			message = "no read permission for this note"
		} else if !strings.Contains(message, "no read permission for this note") {
			message = fmt.Sprintf("no read permission for this note: %s", message)
		}
		var permErr *errs.PermissionError
		if errors.As(err, &permErr) {
			mapped := *permErr
			mapped.Problem.Message = message
			if mapped.Problem.Hint == "" {
				mapped.Problem.Hint = "Ask the note owner to grant read permission, then retry"
			}
			mapped.Cause = err
			return &mapped
		}
		mappedProblem := *problem
		mappedProblem.Category = errs.CategoryAuthorization
		mappedProblem.Subtype = errs.SubtypePermissionDenied
		mappedProblem.Message = message
		if mappedProblem.Hint == "" {
			mappedProblem.Hint = "Ask the note owner to grant read permission, then retry"
		}
		return &errs.PermissionError{Problem: mappedProblem, Cause: err}
	}
	return err
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package base

import (
	"context"

	"github.com/larksuite/cli/shortcuts/common"
)

type baseFormDeleteArgs struct {
	BaseToken string `flag:"base-token" schema:"required" doc:"base token"`
	TableID   string `flag:"table-id" schema:"required" doc:"table ID"`
	FormID    string `flag:"form-id" schema:"required" doc:"form ID"`
}

// Field order preserves the legacy map encoding order used by encoding/json.
type baseFormDeleteData struct {
	Deleted bool   `json:"deleted" schema:"required" doc:"whether the form was deleted"`
	FormID  string `json:"form_id" schema:"required" doc:"deleted form ID"`
}

var BaseFormDelete = common.Define(common.Definition[baseFormDeleteArgs, baseFormDeleteData]{
	Metadata: common.CommandMetadata{
		Service: "base", Command: "+form-delete", Description: "Delete a form in a Base table", Risk: common.RiskHighRiskWrite,
		Tips: []string{
			"Use +form-list or +form-get first when the form target is ambiguous.",
			baseHighRiskYesTip,
		},
		Authorization: common.AuthorizationDefinition{
			IdentityOrder: []common.Identity{common.IdentityUser, common.IdentityBot},
			Identities: map[common.Identity]common.IdentityAuthorization{
				common.IdentityUser: {RequiredScopes: []string{"base:form:delete"}},
				common.IdentityBot:  {RequiredScopes: []string{"base:form:delete"}},
			},
		},
	},
	// Legacy used RuntimeContext.Out, so --format remains accepted but success
	// output stays in the JSON envelope for every format value.
	Output: common.OutputDefinition{Mode: common.OutputFixedJSON},
	Hooks: common.Hooks[baseFormDeleteArgs, baseFormDeleteData]{
		DryRun: func(_ context.Context, _ common.CommandContext, args *baseFormDeleteArgs) *common.DryRunAPI {
			return common.NewDryRunAPI().
				DELETE("/open-apis/base/v3/bases/:base_token/tables/:table_id/forms/:form_id").
				Set("base_token", args.BaseToken).
				Set("table_id", args.TableID).
				Set("form_id", args.FormID)
		},
		Execute: func(ctx context.Context, command common.CommandContext, args *baseFormDeleteArgs) (common.Result[baseFormDeleteData], error) {
			_, err := baseV3CallTyped(ctx, command, "DELETE",
				baseV3Path("bases", args.BaseToken, "tables", args.TableID, "forms", args.FormID), nil, nil, "base +form-delete")
			if err != nil {
				return common.Result[baseFormDeleteData]{}, err
			}
			return common.Success(baseFormDeleteData{Deleted: true, FormID: args.FormID}), nil
		},
	},
})

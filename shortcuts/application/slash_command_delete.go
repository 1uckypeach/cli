// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package application

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/larksuite/cli/shortcuts/common"
)

type slashCommandDeleteArgs struct {
	CommandID string `flag:"command-id" schema:"optional" doc:"target command_id"`
	Command   string `flag:"command" schema:"optional" doc:"target command name without leading slash; resolved via live list"`
}

type slashCommandDeleteData struct {
	Action    string `json:"action" schema:"required;enum=deleted" doc:"performed action"`
	Command   string `json:"command,omitempty" schema:"optional" doc:"deleted command name when resolved by name"`
	CommandID string `json:"command_id" schema:"required" doc:"deleted command ID"`
}

// SlashCommandDelete removes a slash command (irreversible; command_id is not
// reused - recreating the same name yields a NEW id).
var SlashCommandDelete = common.Define(common.Definition[slashCommandDeleteArgs, slashCommandDeleteData]{
	Metadata: common.CommandMetadata{
		Service: "application", Command: "+slash-command-delete",
		Description: "Delete a slash command from the current bound app (high-risk: irreversible; recreating the same name yields a new command_id)",
		Risk:        common.RiskHighRiskWrite,
		Authorization: common.AuthorizationDefinition{IdentityOrder: []common.Identity{common.IdentityBot, common.IdentityUser}, Identities: map[common.Identity]common.IdentityAuthorization{
			common.IdentityBot: {
				RequiredScopes:    []string{"application:app_slash_command:write"},
				ConditionalScopes: []common.ConditionalScope{{Scopes: []string{"application:app_slash_command:read"}, When: "--command is used and the command must be resolved by name", Params: []string{"command"}}},
			},
			common.IdentityUser: {
				RequiredScopes:    []string{"application:app_slash_command:write"},
				ConditionalScopes: []common.ConditionalScope{{Scopes: []string{"application:app_slash_command:read"}, When: "--command is used and the command must be resolved by name", Params: []string{"command"}}},
			},
		}},
	},
	Input: common.InputDefinition{Relations: []common.Relation{{
		Kind: common.RelationExactlyOne, Params: []string{"command-id", "command"}, Presence: common.PresenceNonZero, Stage: common.StageAfterPrepare,
	}}},
	Hooks: common.Hooks[slashCommandDeleteArgs, slashCommandDeleteData]{
		Normalize: func(_ context.Context, _ common.CommandContext, args *slashCommandDeleteArgs) error {
			args.CommandID = strings.TrimSpace(args.CommandID)
			args.Command = strings.TrimSpace(args.Command)
			return nil
		},
		Validate: func(_ context.Context, _ common.CommandContext, args *slashCommandDeleteArgs) error {
			if args.Command != "" {
				return validateCommandName(args.Command, "--command")
			}
			return nil
		},
		DryRun: func(_ context.Context, _ common.CommandContext, args *slashCommandDeleteArgs) *common.DryRunAPI {
			dry := common.NewDryRunAPI().Desc("HIGH-RISK: delete a slash command (irreversible; same-name recreate gets a NEW command_id)")
			target := args.CommandID
			if target == "" {
				dry.GET(slashCommandBasePath).Desc(fmt.Sprintf("resolve command_id by name %q via GET list first", args.Command))
				target = "<resolved_command_id>"
			} else {
				target = encodeCommandIDPathSegment(target)
			}
			return dry.DELETE(slashCommandBasePath + "/" + target)
		},
		Execute: func(ctx context.Context, command common.CommandContext, args *slashCommandDeleteArgs) (common.Result[slashCommandDeleteData], error) {
			id := args.CommandID
			if id == "" {
				resolved, err := resolveCommandIDTyped(ctx, command, args.Command)
				if err != nil {
					return common.Result[slashCommandDeleteData]{}, err
				}
				id = resolved
			}
			if _, err := common.CallTypedAPI(ctx, command, "DELETE", slashCommandBasePath+"/"+encodeCommandIDPathSegment(id), nil, nil); err != nil {
				return common.Result[slashCommandDeleteData]{}, err
			}
			fmt.Fprintln(command.Stderr(), clientCacheHint)
			fmt.Fprintln(command.Stderr(), "note: recreating the same command name will yield a NEW command_id.")
			return common.Success(slashCommandDeleteData{Action: "deleted", Command: args.Command, CommandID: id}), nil
		},
		Renderers: map[string]common.Renderer[slashCommandDeleteData]{"pretty": func(w io.Writer, data slashCommandDeleteData) error {
			_, err := fmt.Fprintf(w, "deleted command_id %s\n", data.CommandID)
			return err
		}},
	},
})

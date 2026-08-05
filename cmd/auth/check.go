// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package auth

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	larkauth "github.com/larksuite/cli/internal/auth"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/recovery"
)

// CheckOptions holds all inputs for auth check.
type CheckOptions struct {
	Factory *cmdutil.Factory
	Scope   string
	JSON    bool
}

// unusableTokenError maps a token status to the `error` value auth check
// reports for it, or "" when the token can still serve calls. It keeps the
// predicate aligned with the rest of the tree: a status that makes `--as auto`
// fall back to bot must not be answered with "granted" here.
func unusableTokenError(status string) string {
	switch status {
	case larkauth.TokenStatusCorrupted:
		return "corrupted_token"
	case larkauth.TokenStatusExpired:
		return "expired_token"
	default:
		return ""
	}
}

// NewCmdAuthCheck creates the auth check subcommand.
func NewCmdAuthCheck(f *cmdutil.Factory, runF func(*CheckOptions) error) *cobra.Command {
	return newCmdAuthCheck(f, runF, nil)
}

func newCmdAuthCheck(
	f *cmdutil.Factory,
	runF func(*CheckOptions) error,
	projector *recovery.Projector,
) *cobra.Command {
	opts := &CheckOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check if current token has specified scopes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return authCheckRunWithRecovery(opts, projector)
		},
	}

	cmd.Flags().StringVar(&opts.Scope, "scope", "", "scopes to check (space-separated)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "structured JSON output")
	cmd.MarkFlagRequired("scope")
	cmdutil.SetRisk(cmd, "read")

	return cmd
}

func authCheckRun(opts *CheckOptions) error {
	return authCheckRunWithRecovery(opts, nil)
}

func authCheckRunWithRecovery(opts *CheckOptions, projector *recovery.Projector) error {
	f := opts.Factory

	required := strings.Fields(opts.Scope)
	if len(required) == 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--scope cannot be empty").WithParam("--scope")
	}

	config, err := f.Config()
	if err != nil {
		return err
	}
	if config.UserOpenId == "" {
		output.PrintJson(f.IOStreams.Out, map[string]interface{}{"ok": false, "error": "not_logged_in", "missing": required})
		return output.ErrBare(1)
	}

	stored := larkauth.GetStoredToken(config.AppID, config.UserOpenId)
	if stored == nil {
		output.PrintJson(f.IOStreams.Out, map[string]interface{}{"ok": false, "error": "no_token", "missing": required})
		return output.ErrBare(1)
	}
	// The scope list of an unusable record still looks complete, so reporting
	// scopes here would answer "granted" for a credential that cannot make a
	// single call. Fail with the cause instead.
	//
	// needs_refresh is deliberately not rejected: that token still serves calls
	// after the automatic refresh on the next use, so its scopes are the ones
	// the caller will actually have.
	if unusable := unusableTokenError(larkauth.TokenStatus(stored)); unusable != "" {
		output.PrintJson(f.IOStreams.Out, map[string]interface{}{"ok": false, "error": unusable, "missing": required})
		return output.ErrBare(1)
	}

	missing := larkauth.MissingScopes(stored.Scope, required)
	missingSet := make(map[string]bool, len(missing))
	for _, s := range missing {
		missingSet[s] = true
	}
	var granted []string
	for _, s := range required {
		if !missingSet[s] {
			granted = append(granted, s)
		}
	}

	ok := len(missing) == 0
	result := map[string]interface{}{"ok": ok, "granted": granted, "missing": missing}
	if len(missing) > 0 && projector.CanReference(recovery.TargetAuthLogin) {
		result["suggestion"] = projector.RenderHint(recovery.UserAuthorization(missing...))
	}
	output.PrintJson(f.IOStreams.Out, result)
	if !ok {
		return output.ErrBare(1)
	}
	return nil
}

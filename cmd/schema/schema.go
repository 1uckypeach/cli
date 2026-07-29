// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package schema

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/apicatalog"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/meta"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/registry"
	"github.com/larksuite/cli/internal/schema"
	"github.com/spf13/cobra"
)

// SchemaOptions holds all inputs for the schema command.
type SchemaOptions struct {
	Factory *cmdutil.Factory
	Ctx     context.Context

	// Args are the positional path segments, in either the dotted single-arg
	// form ("im.messages.reply") or the space-separated form ("im messages
	// reply"); apicatalog.ParsePath normalizes both.
	Args []string

	// JqExpr filters the JSON output when non-empty.
	JqExpr string
}

// NewCmdSchema creates the schema command. If runF is non-nil it is called instead of schemaRun (test hook).
func NewCmdSchema(f *cmdutil.Factory, runF func(*SchemaOptions) error) *cobra.Command {
	opts := &SchemaOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "schema [path | service resource method]",
		Short: "View API method parameters, types, and scopes",
		Args:  cobra.MaximumNArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = append([]string(nil), args...)
			opts.Ctx = cmd.Context()
			// schema has no --output, so the mutual-exclusion arm never fires
			// here; going through the shared validator keeps the error text for a
			// bad expression identical to every other command's.
			format, _ := cmd.Flags().GetString("format")
			if err := output.ValidateJqFlags(opts.JqExpr, "", format); err != nil {
				return err
			}
			if runF != nil {
				return runF(opts)
			}
			return schemaRun(opts)
		},
	}
	cmdutil.DisableAuthCheck(cmd)

	// Tolerated for agent compatibility; ignored — schema only emits the JSON
	// envelope, and its output is identity-independent (strict-mode filtering
	// comes from ResolveStrictMode, never from --as).
	cmd.Flags().String("format", "json", "")
	cmd.Flags().Bool("json", true, "")
	cmd.Flags().String("as", "", "")
	_ = cmd.Flags().MarkHidden("format")
	_ = cmd.Flags().MarkHidden("json")
	_ = cmd.Flags().MarkHidden("as")

	cmd.Flags().StringVarP(&opts.JqExpr, "jq", "q", "", "jq expression to filter JSON output")

	cmd.ValidArgsFunction = completeSchemaPath(f)
	cmdutil.SetRisk(cmd, cmdutil.RiskRead)

	return cmd
}

// completeSchemaPath is a thin adapter over the schema catalog's Complete.
// It uses the same source as schema execution so completion candidates match
// what `schema` can resolve.
func completeSchemaPath(f *cmdutil.Factory) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		mode := f.ResolveStrictMode(cmd.Context())
		completions, noSpace := registry.SchemaCatalog().Complete(args, toComplete, registry.FilterForStrictMode(mode))
		directive := cobra.ShellCompDirectiveNoFileComp
		if noSpace {
			directive |= cobra.ShellCompDirectiveNoSpace
		}
		return completions, directive
	}
}

func schemaRun(opts *SchemaOptions) error {
	out := opts.Factory.IOStreams.Out
	mode := opts.Factory.ResolveStrictMode(opts.Ctx)
	return runSchema(out, apicatalog.ParsePath(opts.Args), mode, opts.JqExpr)
}

// runSchema resolves the path through the schema catalog and renders the shape
// matching the target's depth: the whole catalog renders as a service index, a
// service or resource as a method index, and a single method as the full
// envelope. The catalog owns navigation (Resolve + MethodRefs) and schema owns
// rendering; this adapter only chooses the shape and maps resolve failures to
// hints. There is deliberately no bulk-envelope path — rendering every method's
// full contract at once exceeds any practical single-response budget, and
// offline consumers that need the whole contract read the catalog directly.
func runSchema(out io.Writer, parts []string, mode core.StrictMode, jqExpr string) error {
	catalog := registry.SchemaCatalog()
	if len(catalog.Services()) == 0 {
		// No embedded metadata and the runtime fallback is empty too: offline
		// with a cold cache, remote meta off, or an unwritable cache dir.
		return errs.NewValidationError(errs.SubtypeFailedPrecondition, "No API metadata available").
			WithHint("this binary has no embedded API metadata; run any command with network access to the open platform once so metadata can be fetched and cached")
	}
	target, err := catalog.Resolve(parts)
	if err != nil {
		return resolveError(err)
	}
	filter := registry.FilterForStrictMode(mode)

	switch target.Kind {
	case apicatalog.TargetAll:
		return emit(out, jqExpr, schema.BuildServiceIndex(
			visibleServices(catalog, mode, filter),
			func(name string) string { return registry.GetServiceDescription(name, "en") },
		))
	case apicatalog.TargetService, apicatalog.TargetResource:
		return emit(out, jqExpr, schema.BuildMethodIndex(parts[0], catalog.MethodRefs(target, filter)))
	}

	refs := catalog.MethodRefs(target, filter)
	if len(refs) == 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"Method %s not available in current identity mode", target.Method.SchemaPath()).
			WithHint("strict mode hides methods the active account identity cannot call; it is shown for an identity (user or bot) that has the required access token")
	}
	return emit(out, jqExpr, schema.EnvelopeOf(refs[0]))
}

// visibleServices lists the services for the service index. Service-level
// filtering follows strict mode: when it is off (the common case) the listing is
// unfiltered and needs no per-service metadata walk, matching the unpruned root
// help; when it is active the listing drops services with no reachable method,
// matching how the root help is pruned. Filtering unconditionally would make
// the bare `schema` walk every service; filtering never would make this listing
// wider than the root help, re-exposing services the command tree hides.
func visibleServices(catalog apicatalog.Catalog, mode core.StrictMode, filter apicatalog.MethodFilter) []meta.Service {
	all := catalog.Services()
	if !mode.IsActive() {
		return all
	}
	out := make([]meta.Service, 0, len(all))
	for _, svc := range all {
		if len(apicatalog.ServiceMethods(svc, filter)) > 0 {
			out = append(out, svc)
		}
	}
	return out
}

// emit renders v as JSON, applying the jq expression when present.
func emit(out io.Writer, jqExpr string, v any) error {
	if jqExpr == "" {
		output.PrintJson(out, v)
		return nil
	}
	return output.JqFilter(out, v, jqExpr)
}

// resolveError maps a catalog *ResolveError to a typed *errs.ValidationError
// (CategoryValidation drives the exit code; Hint promotes to the envelope),
// preserving the historical message + hint text.
func resolveError(err error) error {
	var re *apicatalog.ResolveError
	if !errors.As(err, &re) {
		return err
	}
	switch re.Kind {
	case apicatalog.ErrService:
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "Unknown service: %s", re.Subject).
			WithHint("Available: %s", strings.Join(re.Candidates, ", "))
	case apicatalog.ErrResource:
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "Unknown resource: %s", re.Subject).
			WithHint("Available: %s", strings.Join(re.Candidates, ", "))
	case apicatalog.ErrMethod:
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "Unknown method: %s", re.Subject).
			WithHint("Available: %s", strings.Join(re.Candidates, ", "))
	case apicatalog.ErrPath:
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "Unknown path: %s", re.Subject).
			WithHint("Method %q exists but the trailing segments %q do not resolve", re.Method, re.Trailing)
	}
	return err
}

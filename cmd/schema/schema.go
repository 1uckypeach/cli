// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package schema

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/apicatalog"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/meta"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/registry"
	"github.com/larksuite/cli/internal/schema"
	"github.com/larksuite/cli/shortcuts"
	"github.com/spf13/cobra"
)

// CommandVisibility reports whether one canonical generated-command path is
// referenceable in the current build. Paths use the same segments as
// apicatalog.MethodRef.CommandPath (for example
// ["mail", "user_mailbox.messages", "list"]). A nil visibility keeps the
// complete schema catalog.
//
// The callback is deliberately command-facing rather than policy-facing:
// cmd/schema only consumes the final build-local presentation surface and does
// not know why a command is or is not referenceable.
type CommandVisibility func(path []string) bool

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

// NewCmdSchema creates the schema command. If runF is non-nil it is called instead of the default runner (test hook).
func NewCmdSchema(f *cmdutil.Factory, runF func(*SchemaOptions) error) *cobra.Command {
	return NewCmdSchemaWithVisibility(f, nil, runF)
}

// NewCmdSchemaWithVisibility creates the schema command projected through one
// build-local command surface. Existing callers should use NewCmdSchema; the
// root builder uses this form so schema execution and completion share the
// exact presentation plan captured by that Cobra tree.
func NewCmdSchemaWithVisibility(
	f *cmdutil.Factory,
	visibility CommandVisibility,
	runF func(*SchemaOptions) error,
) *cobra.Command {
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
			return schemaRunWithVisibility(opts, visibility)
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

	cmd.ValidArgsFunction = completeSchemaPath(f, visibility)
	cmdutil.SetRisk(cmd, cmdutil.RiskRead)

	return cmd
}

// completeSchemaPath is a thin adapter over the schema catalog's Complete.
// It uses the same source as schema execution so completion candidates match
// what `schema` can resolve.
func completeSchemaPath(
	f *cmdutil.Factory,
	visibility CommandVisibility,
) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		mode := f.ResolveStrictMode(cmd.Context())
		catalog := projectSchemaCatalog(f.APICatalog, visibility)
		completions, noSpace := catalog.Complete(args, toComplete, registry.FilterForStrictMode(mode))
		directive := cobra.ShellCompDirectiveNoFileComp
		if noSpace {
			directive |= cobra.ShellCompDirectiveNoSpace
		}
		return completions, directive
	}
}

func schemaRunWithVisibility(opts *SchemaOptions, visibility CommandVisibility) error {
	out := opts.Factory.IOStreams.Out
	mode := opts.Factory.ResolveStrictMode(opts.Ctx)
	return runSchemaCatalog(out, apicatalog.ParsePath(opts.Args), mode, opts.Factory.APICatalog, visibility, opts.JqExpr)
}

// runSchemaCatalog resolves the path through the build-selected schema catalog
// and renders the shape matching the target's depth: the whole catalog renders
// as a service index, a service or resource as a method index, and a single
// method as the full envelope. The catalog owns navigation (Resolve +
// MethodRefs), while this adapter applies presentation visibility, chooses the
// shape, and maps resolve failures to hints. There is deliberately no
// bulk-envelope path — rendering every method's full contract at once exceeds
// any practical single-response budget, and offline consumers that need the
// whole contract read the catalog directly.
func runSchemaCatalog(
	out io.Writer,
	parts []string,
	mode core.StrictMode,
	catalog apicatalog.Catalog,
	visibility CommandVisibility,
	jqExpr string,
) error {
	// Test the source catalog before presentation projection. A distribution
	// that intentionally conceals every generated method still has metadata;
	// bare `schema` should render an empty list rather than claim metadata is
	// unavailable.
	if len(catalog.Services()) == 0 {
		return errs.NewValidationError(errs.SubtypeFailedPrecondition, "No API metadata available").
			WithHint("the current command build did not select any API metadata")
	}
	catalog = projectSchemaCatalog(catalog, visibility)
	target, err := catalog.Resolve(parts)
	if err != nil {
		return resolveError(err, parts)
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
	return emit(out, jqExpr, schema.EnvelopeOfCatalog(catalog, refs[0]))
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

// pathSegRe is the allowed character set for a command path segment echoed back
// into a hint. Anything outside it is replaced wholesale, so a crafted argument
// cannot inject text into the error envelope a consumer parses.
var pathSegRe = regexp.MustCompile(`^[A-Za-z0-9._+-]+$`)

// safeSeg returns seg when it is a plain command-path segment, or a neutral
// placeholder otherwise.
func safeSeg(seg string) string {
	if pathSegRe.MatchString(seg) {
		return seg
	}
	return "<name>"
}

// isShortcutOnlyDomain reports whether name is a domain that exists as a command
// but contributes no API methods. The shortcut registry is the same index the
// domain help and `auth login` consume for the shortcut-domain listing, so a
// hint built from it names a command the caller will actually find.
//
// A domain the current build prunes down to nothing would still be listed here,
// which is the one case where the returned hint can outlive its command; the
// listing is deliberately static because the alternative — reading the live
// Cobra tree — is not available at this depth.
func isShortcutOnlyDomain(name string) bool {
	return slices.Contains(shortcuts.ShortcutServiceNames(), name)
}

// domainOrPlaceholder keeps a hint's example command runnable-looking even when
// no domain segment was supplied.
func domainOrPlaceholder(domain string) string {
	if domain == "" {
		return "<domain>"
	}
	return domain
}

// projectSchemaCatalog produces the metadata view corresponding to one final
// command surface. It lives in cmd/schema so apicatalog remains a policy-free
// navigation module. Resolve, broad listings, and Complete all consume the
// same projected Catalog, which also prevents resolve-error candidate hints
// from naming concealed resources or methods.
//
// Unchanged branches retain their original maps. A parent is removed when
// projection removed its last reachable method, so a fully concealed service
// cannot survive as an empty schema namespace. Originally-empty, unaffected
// metadata remains unchanged for backward compatibility.
func projectSchemaCatalog(catalog apicatalog.Catalog, visibility CommandVisibility) apicatalog.Catalog {
	if visibility == nil {
		return catalog
	}

	services := make([]meta.Service, 0, len(catalog.Services()))
	changed := false
	for _, service := range catalog.Services() {
		servicePath := []string{service.Name}
		if !visibility(servicePath) {
			changed = true
			continue
		}

		resources, resourceChanged, hasVisibleMethod := projectSchemaResources(
			service.Resources,
			servicePath,
			visibility,
		)
		if resourceChanged && !hasVisibleMethod {
			changed = true
			continue
		}
		if resourceChanged {
			service.Resources = resources
			changed = true
		}
		services = append(services, service)
	}
	if !changed {
		return catalog
	}
	return apicatalog.New(catalog.Source(), services)
}

func projectSchemaResources(
	resources map[string]meta.Resource,
	parentPath []string,
	visibility CommandVisibility,
) (projected map[string]meta.Resource, changed, hasVisibleMethod bool) {
	projected = make(map[string]meta.Resource, len(resources))
	for name, resource := range resources {
		resourcePath := appendPath(parentPath, name)
		if !visibility(resourcePath) {
			changed = true
			continue
		}

		methods := make(map[string]meta.Method, len(resource.Methods))
		resourceChanged := false
		resourceHasVisibleMethod := false
		for methodName, method := range resource.Methods {
			if !visibility(appendPath(resourcePath, methodName)) {
				resourceChanged = true
				continue
			}
			methods[methodName] = method
			resourceHasVisibleMethod = true
		}

		subResources, subChanged, subHasVisibleMethod := projectSchemaResources(
			resource.Resources,
			resourcePath,
			visibility,
		)
		resourceChanged = resourceChanged || subChanged
		resourceHasVisibleMethod = resourceHasVisibleMethod || subHasVisibleMethod

		if resourceChanged && !resourceHasVisibleMethod {
			// Projection removed the final method below this resource. Keeping
			// the empty group would still reveal a concealed schema namespace.
			changed = true
			continue
		}
		if resourceChanged {
			resource.Methods = methods
			resource.Resources = subResources
			changed = true
		}
		projected[name] = resource
		hasVisibleMethod = hasVisibleMethod || resourceHasVisibleMethod
	}

	if !changed {
		return resources, false, hasVisibleMethod
	}
	return projected, true, hasVisibleMethod
}

func appendPath(parent []string, segment string) []string {
	path := make([]string, len(parent)+1)
	copy(path, parent)
	path[len(parent)] = segment
	return path
}

// resolveError maps a catalog *ResolveError to a typed *errs.ValidationError
// (CategoryValidation drives the exit code; Hint promotes to the envelope). The
// hints route the caller back to a usable surface instead of dead-ending:
// shortcuts are documented only in help, and an unresolvable typed path gets
// both the candidate list and the service's method index.
func resolveError(err error, parts []string) error {
	var re *apicatalog.ResolveError
	if !errors.As(err, &re) {
		return err
	}

	domain := ""
	if len(parts) > 0 {
		domain = safeSeg(parts[0])
	}

	// A trailing +name is a shortcut: it has no schema, only help. The message
	// echoes the same caller-supplied segment as the hint, so it gets the same
	// whitelist — cleaning one and not the other would leave the pair
	// inconsistent about what was rejected.
	if last := len(parts) - 1; last >= 0 && strings.HasPrefix(parts[last], "+") {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "Unknown resource: %s", safeSeg(re.Subject)).
			WithHint("shortcuts are documented in --help, not schema; run `lark-cli %s %s --help` for parameters and examples, or `lark-cli %s --help` to list all commands",
				domain, safeSeg(parts[last]), domain)
	}

	indexHint := ""
	if domain != "" {
		indexHint = fmt.Sprintf("; run `lark-cli %s --help` to see both +shortcuts and API resources, or `lark-cli schema %s` for the API method index", domain, domain)
	}

	switch re.Kind {
	case apicatalog.ErrService:
		// A domain that only provides +shortcuts is absent from the API catalog
		// but very much exists as a command, so calling it unknown contradicts
		// what `lark-cli --help` just showed and sends the caller looking for a
		// naming mismatch that isn't there. A name that is no domain at all gets
		// the opposite treatment: claiming it "has no API methods" asserts it
		// exists, and pointing at `lark-cli <name> --help` hands back a command
		// that fails the same way — one dead end traded for another.
		if isShortcutOnlyDomain(re.Subject) {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "No API methods for: %s", safeSeg(re.Subject)).
				WithHint("Available: %s; a domain that only provides +shortcuts has no API methods and is not listed here — run `lark-cli %s --help` to see its commands, or `lark-cli --help` to list all domains",
					strings.Join(re.Candidates, ", "), domainOrPlaceholder(domain))
		}
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "Unknown service: %s", safeSeg(re.Subject)).
			WithHint("Available: %s; run `lark-cli --help` to list all domains, including the ones that only provide +shortcuts",
				strings.Join(re.Candidates, ", "))
	case apicatalog.ErrResource:
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "Unknown resource: %s", re.Subject).
			WithHint("Available: %s%s", strings.Join(re.Candidates, ", "), indexHint)
	case apicatalog.ErrMethod:
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "Unknown method: %s", re.Subject).
			WithHint("Available: %s%s", strings.Join(re.Candidates, ", "), indexHint)
	case apicatalog.ErrPath:
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "Unknown path: %s", re.Subject).
			WithHint("Method %q exists but the trailing segments %q do not resolve", re.Method, re.Trailing)
	}
	return err
}

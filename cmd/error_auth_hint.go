// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/apicatalog"
	internalauth "github.com/larksuite/cli/internal/auth"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/registry"
	"github.com/larksuite/cli/shortcuts"
	shortcutcommon "github.com/larksuite/cli/shortcuts/common"
)

type declaredScopeHint struct {
	Scopes     []string
	Groups     [][]string
	GroupHints []string
}

// applyNeedAuthorizationHint augments a typed *errs.AuthenticationError with a
// "current command requires scope(s): X, Y" hint when the underlying error is
// a need_user_authorization signal AND the current command declares scopes
// locally (via shortcut registration or service-method metadata). Existing
// Hint text is preserved; scopes are appended on a new line.
func applyNeedAuthorizationHint(f *cmdutil.Factory, err error) {
	if err == nil || f == nil {
		return
	}
	if !internalauth.IsNeedUserAuthorizationError(err) {
		return
	}
	var authErr *errs.AuthenticationError
	if !errors.As(err, &authErr) {
		return
	}
	hint := resolveDeclaredScopeHintForCurrentCommand(f)
	if len(hint.Scopes) == 0 && len(hint.Groups) == 0 {
		return
	}
	scopeHint := formatDeclaredScopeHint(hint)
	if authErr.Hint == "" {
		authErr.Hint = scopeHint
		return
	}
	authErr.Hint += "\n" + scopeHint
}

// resolveDeclaredScopesForCurrentCommand returns the scopes declared by the
// current command for the resolved identity, checking shortcuts first and then
// service methods from local registry metadata.
func resolveDeclaredScopesForCurrentCommand(f *cmdutil.Factory) []string {
	return resolveDeclaredScopeHintForCurrentCommand(f).Scopes
}

func resolveDeclaredScopeHintForCurrentCommand(f *cmdutil.Factory) declaredScopeHint {
	if f == nil || f.CurrentCommand == nil {
		return declaredScopeHint{}
	}

	identity := string(f.ResolvedIdentity)
	if identity == "" {
		identity = string(core.AsUser)
	}
	if identity != string(core.AsUser) && identity != string(core.AsBot) {
		return declaredScopeHint{}
	}

	if hint := resolveDeclaredShortcutScopeHint(f.CurrentCommand, identity); len(hint.Scopes) > 0 || len(hint.Groups) > 0 {
		return hint
	}
	return declaredScopeHint{Scopes: resolveDeclaredServiceMethodScopes(f.CurrentCommand, identity)}
}

// resolveDeclaredShortcutScopes returns the scopes declared by a mounted
// shortcut command for the given identity.
func resolveDeclaredShortcutScopes(cmd *cobra.Command, identity string) []string {
	return resolveDeclaredShortcutScopeHint(cmd, identity).Scopes
}

func resolveDeclaredShortcutScopeHint(cmd *cobra.Command, identity string) declaredScopeHint {
	if cmd == nil || cmd.Parent() == nil || !strings.HasPrefix(cmd.Name(), "+") {
		return declaredScopeHint{}
	}

	service := cmd.Parent().Name()
	for _, sc := range shortcuts.AllShortcuts() {
		if sc.Service != service || sc.Command != cmd.Name() || !shortcutSupportsIdentity(sc, identity) {
			continue
		}
		scopes := sc.DeclaredScopesForIdentity(identity)
		groups := sc.ScopeGroupsForIdentity(identity)
		if len(scopes) == 0 && len(groups) == 0 {
			return declaredScopeHint{}
		}
		return declaredScopeHint{
			Scopes:     append([]string(nil), scopes...),
			Groups:     cloneScopeGroups(groups),
			GroupHints: append([]string(nil), sc.ScopeGroupHints...),
		}
	}
	return declaredScopeHint{}
}

func formatDeclaredScopeHint(hint declaredScopeHint) string {
	if len(hint.Groups) == 0 {
		return fmt.Sprintf("current command requires scope(s): %s", strings.Join(hint.Scopes, ", "))
	}
	return fmt.Sprintf("current command accepts one of these scope sets: %s", formatScopeGroups(hint.Groups, hint.GroupHints))
}

func cloneScopeGroups(groups [][]string) [][]string {
	out := make([][]string, 0, len(groups))
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		out = append(out, append([]string(nil), group...))
	}
	return out
}

func formatScopeGroups(groups [][]string, hints []string) string {
	parts := make([]string, 0, len(groups))
	for i, group := range groups {
		if len(group) == 0 {
			continue
		}
		part := strings.Join(group, " + ")
		if i < len(hints) && strings.TrimSpace(hints[i]) != "" {
			part += " (" + strings.TrimSpace(hints[i]) + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " OR ")
}

// resolveDeclaredServiceMethodScopes returns the scopes declared by a
// service/resource/method command. It reconstructs the catalog path from the
// command ancestry and resolves it through the same navigation Module the
// command tree is built from (apicatalog), so it stays correct for nested
// resources instead of hard-coding a root->service->resource->method depth.
// Non-method commands (services, resources, shortcuts) resolve to a non-method
// target and yield no scopes.
func resolveDeclaredServiceMethodScopes(cmd *cobra.Command, identity string) []string {
	if cmd == nil || strings.HasPrefix(cmd.Name(), "+") {
		return nil
	}
	path := commandCatalogPath(cmd)
	if len(path) == 0 {
		return nil
	}
	target, err := registry.RuntimeCatalog().Resolve(path)
	if err != nil || target.Kind != apicatalog.TargetMethod {
		return nil
	}
	return registry.DeclaredScopesForMethod(target.Method.Method, identity)
}

// commandCatalogPath reconstructs the catalog path [service, resource..., method]
// from a command's ancestry, excluding the root command. It is the inverse of
// the service command tree's construction, so any depth (flat or nested)
// round-trips through apicatalog.Resolve.
func commandCatalogPath(cmd *cobra.Command) []string {
	var path []string
	for c := cmd; c != nil && c.Parent() != nil; c = c.Parent() {
		path = append([]string{c.Name()}, path...)
	}
	return path
}

// shortcutSupportsIdentity reports whether a shortcut supports the requested
// identity, applying the default user-only behavior when AuthTypes is empty.
func shortcutSupportsIdentity(sc shortcutcommon.Shortcut, identity string) bool {
	authTypes := sc.AuthTypes
	if len(authTypes) == 0 {
		authTypes = []string{string(core.AsUser)}
	}
	for _, authType := range authTypes {
		if authType == identity {
			return true
		}
	}
	return false
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"sort"
	"strings"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	larkauth "github.com/larksuite/cli/internal/auth"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
)

// This file implements the local scope preflight: after
// resolveProvider succeeds and before the real API call, the stored user
// token's scope list is checked against the provider's RequiredScopes
// declaration. The check is all-or-nothing — any real API verb requires the
// provider's entire scope set. It is entirely local — the scope list is read
// from the credential cache (keychain), never from the network — so a missing
// scope surfaces as an actionable validation error (exit 2) instead of a
// round-trip API 99991679. `--dry-run` never reaches it (dry-run returns before
// resolveProvider), preserving its always-available contract.

// storedUserScopes is the token-scope read seam: it returns the granted scope
// list of the stored user token from the LOCAL credential cache (keychain via
// GetStoredToken — same read path as `auth check`), issuing no network
// request. nil/empty means "no usable local scope list" and the caller skips
// preflight. Tests swap it so no unit test touches the real keychain.
var storedUserScopes = func(f *cmdutil.Factory) []string {
	if f == nil || f.Config == nil {
		return nil
	}
	config, err := f.Config()
	if err != nil || config == nil || config.UserOpenId == "" {
		return nil
	}
	stored := larkauth.GetStoredToken(config.AppID, config.UserOpenId)
	if stored == nil {
		return nil
	}
	return strings.Fields(stored.Scope)
}

// preflightInput is the pure input of preflightScopes, so the check itself is
// unit-testable without a Factory, keychain, or provider client.
type preflightInput struct {
	Identity    core.Identity
	TokenScopes []string
	Provider    iagent.Provider
}

// preflightScopes runs the local scope check. It returns nil when the check
// does not apply — bot identity (a tenant token has no scope-list concept; the
// API error + errclass hint own that path) or an unreadable/empty local scope
// list (the downstream not_configured / need-authorization logic owns that).
// The check is all-or-nothing: when any scope in the provider's RequiredScopes
// set is not granted it returns the missing_scope permission error
// (exit 3, mirroring the event-consume scope preflight) carrying every missing
// scope, with a re-auth hint whose --scope
// merges the stored grants with the provider's FULL RequiredScopes set — auth
// login --scope REPLACES the grant, so the hint must be copy-paste-safe
// without dropping existing permissions.
func preflightScopes(in preflightInput) error {
	if in.Identity != core.AsUser || len(in.TokenScopes) == 0 {
		return nil
	}

	granted := make(map[string]bool, len(in.TokenScopes))
	for _, s := range in.TokenScopes {
		granted[s] = true
	}

	var missing []string
	for _, scope := range in.Provider.RequiredScopes {
		if !granted[scope] {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)

	// Merged re-auth scope set: existing grants ∪ the provider's FULL
	// RequiredScopes, sorted for stability.
	mergedSet := make(map[string]bool, len(in.TokenScopes)+len(in.Provider.RequiredScopes))
	for _, s := range in.TokenScopes {
		mergedSet[s] = true
	}
	for _, s := range in.Provider.RequiredScopes {
		mergedSet[s] = true
	}
	merged := make([]string, 0, len(mergedSet))
	for s := range mergedSet {
		merged = append(merged, s)
	}
	sort.Strings(merged)

	return errs.NewPermissionError(errs.SubtypeMissingScope,
		"当前 user 身份缺少本命令所需 scope: %s", strings.Join(missing, ", ")).
		WithIdentity(string(core.AsUser)).
		WithMissingScopes(missing...).
		WithHint("一次性补齐该 agent 全部所需 scope（已合并现有授权，照抄不丢权限）: lark-cli auth login --scope \"%s\"",
			strings.Join(merged, " "))
}

// preflightScopesForRef is the command-layer wiring: it resolves the provider
// registration for ref's scheme, reads the stored user scopes through the
// seam, and runs the all-or-nothing preflight. Any gap in its own inputs (nil
// Factory, unparsable ref, unregistered scheme) yields nil — the preflight is
// an accelerator, never a new failure mode; the paths that validate ref/scheme
// for real have already run inside resolveProvider.
func preflightScopesForRef(f *cmdutil.Factory, id core.Identity, ref string) error {
	if f == nil || id != core.AsUser {
		return nil
	}
	r, err := iagent.ParseRef(ref)
	if err != nil {
		return nil //nolint:nilerr // preflight is best-effort: resolveProvider already surfaced any real ref error
	}
	prov, ok := iagent.Info(r.Scheme)
	if !ok {
		return nil
	}
	return preflightScopes(preflightInput{
		Identity:    id,
		TokenScopes: storedUserScopes(f),
		Provider:    prov,
	})
}

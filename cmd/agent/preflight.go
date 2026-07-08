// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/appmeta"
	larkauth "github.com/larksuite/cli/internal/auth"
	"github.com/larksuite/cli/internal/client"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
)

// This file implements the scope preflight: after the provider is resolved and
// before the real API call, the session's available scopes are checked against
// the provider's RequiredScopes. The check is all-or-nothing — any real API verb
// requires the provider's entire scope set. For USER identity the scope list is
// read locally from the credential cache (no network); for BOT identity it is
// the app's published TenantScopes, fetched best-effort (a fetch failure
// downgrades the check to a no-op, like event's console precheck). A missing
// scope surfaces as a missing_scope permission error (exit 3) with an
// identity-appropriate remediation hint instead of a round-trip API 99991679.
// `--dry-run` never reaches it (dry-run returns before the provider is resolved).

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
// does not apply — bot identity (handled elsewhere) or an unreadable/empty local
// scope list (the downstream not_configured / need-authorization logic owns
// that). The check is all-or-nothing: when any scope in the provider's
// RequiredScopes set is not granted it returns the missing_scope permission
// error (exit 3, mirroring the event-consume scope preflight) carrying every
// missing scope, with a re-auth hint listing ONLY the missing scopes.
//
// The hint lists just the missing scopes (not a merge with existing grants):
// the open platform authorizes INCREMENTALLY — re-login with only the missing
// scopes keeps every previously-granted scope — so re-requesting the existing
// grants would be redundant. This mirrors cmd/event's scopeRemediationHint.
func preflightScopes(in preflightInput) error {
	// No usable scope list → skip (user not logged in, or bot has no published
	// version / the fetch failed); the downstream not_configured / API error owns
	// that path.
	if len(in.TokenScopes) == 0 {
		return nil
	}
	// Only user / bot carry a scope-list concept.
	if in.Identity != core.AsUser && !in.Identity.IsBot() {
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

	return errs.NewPermissionError(errs.SubtypeMissingScope,
		"当前 %s 身份缺少本命令所需 scope: %s", in.Identity, strings.Join(missing, ", ")).
		WithIdentity(string(in.Identity)).
		WithMissingScopes(missing...).
		WithHint("%s", scopeRemediationHint(in.Identity, missing))
}

// scopeRemediationHint returns an identity-appropriate fix for the missing
// scopes, mirroring cmd/event's scopeRemediationHint split:
//   - user: re-login requesting ONLY the missing scopes — the open platform
//     authorizes incrementally, so previously-granted scopes are preserved (no
//     merge needed).
//   - bot: the tenant token's scopes come from the app's published version, so
//     the fix is to add the scopes to the app in the developer console and
//     re-publish — not a per-token re-auth. (event additionally offers a
//     one-click scan-to-enable deep link; that generator lives in cmd/event and
//     is not duplicated here.)
func scopeRemediationHint(id core.Identity, missing []string) string {
	if id.IsBot() {
		return fmt.Sprintf(
			"the bot (tenant) token's scopes come from the app's published version — add these scopes to the app in the developer console and re-publish: %s",
			strings.Join(missing, " "))
	}
	// Canonical repo-wide auth login --scope remediation phrasing (see
	// cmd/event, shortcuts/*). Only the missing scopes are listed — the open
	// platform authorizes incrementally, so existing grants are preserved.
	return fmt.Sprintf(
		"run `lark-cli auth login --scope \"%s\"` in the background. It blocks and outputs a verification URL — retrieve the URL and open it in a browser to complete login.",
		strings.Join(missing, " "))
}

// preflightScopesForRef is the command-layer wiring: it resolves the provider
// registration for ref's scheme, reads the stored user scopes through the
// seam, and runs the all-or-nothing preflight. Any gap in its own inputs (nil
// Factory, unparsable ref, unregistered scheme) yields nil — the preflight is
// an accelerator, never a new failure mode; the paths that validate ref/scheme
// for real have already run inside resolveProvider.
func preflightScopesForRef(f *cmdutil.Factory, id core.Identity, ref string) error {
	if f == nil {
		return nil
	}
	r, err := iagent.ParseRef(ref)
	if err != nil {
		return nil //nolint:nilerr // preflight is best-effort: resolveSpec already surfaced any real ref error
	}
	prov, ok := iagent.Info(r.Scheme)
	if !ok || len(prov.RequiredScopes) == 0 {
		return nil // no scopes to check (e.g. the example mock declares none)
	}

	var tokenScopes []string
	switch {
	case id == core.AsUser:
		tokenScopes = storedUserScopes(f) // local keychain read, no network
	case id.IsBot():
		tokenScopes = botTenantScopes(f) // best-effort app-version fetch
	default:
		return nil
	}
	return preflightScopes(preflightInput{Identity: id, TokenScopes: tokenScopes, Provider: prov})
}

// botTenantScopes is the bot-scope read seam: it fetches the app's
// currently-published version and returns its TenantScopes (the scopes a tenant
// token actually carries). Any failure — no client, no published version,
// network / appmeta error — yields nil so the caller skips the check (weak
// dependency, mirroring event's console precheck downgrade). Tests swap it so no
// unit test touches the network.
var botTenantScopes = func(f *cmdutil.Factory) []string {
	if f == nil || f.Config == nil {
		return nil
	}
	config, err := f.Config()
	if err != nil || config == nil || config.AppID == "" {
		return nil
	}
	apiClient, err := f.NewAPIClient()
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	appVer, err := appmeta.FetchCurrentPublished(ctx, &appmetaBotClient{client: apiClient}, config.AppID)
	if err != nil || appVer == nil {
		return nil
	}
	return appVer.TenantScopes
}

// appmetaBotClient adapts *client.APIClient to appmeta's APIClient shape under a
// pinned bot identity (/app_versions is app-level and rejects UAT). It returns
// the raw JSON body for appmeta to project; any non-typed transport error is
// classified so callers only see typed errs.* values (though botTenantScopes
// treats every error as a no-op anyway).
type appmetaBotClient struct{ client *client.APIClient }

func (c *appmetaBotClient) CallAPI(ctx context.Context, method, path string, body interface{}) (json.RawMessage, error) {
	resp, err := c.client.DoAPI(ctx, client.RawApiRequest{Method: method, URL: path, Data: body, As: core.AsBot})
	if err != nil {
		if _, ok := errs.ProblemOf(err); ok {
			return nil, err
		}
		return nil, errs.NewNetworkError(errs.SubtypeNetworkTransport, "api %s %s: %s", method, path, err).WithCause(err)
	}
	return json.RawMessage(resp.RawBody), nil
}

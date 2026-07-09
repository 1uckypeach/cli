// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package common

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/credential"
)

type scopeCheckTokenResolver struct {
	result *credential.TokenResult
	err    error
}

func (r *scopeCheckTokenResolver) ResolveToken(ctx context.Context, req credential.TokenSpec) (*credential.TokenResult, error) {
	return r.result, r.err
}

// TestEnhancePermissionError_TypedPermissionErrorRouted pins typed routing:
// an *errs.PermissionError gets enhanced regardless of its Message text,
// decoupling this helper from canonical-message rewrites that would
// previously break the legacy keyword scan.
func TestEnhancePermissionError_TypedPermissionErrorRouted(t *testing.T) {
	scopes := []string{"drive:drive:read"}
	err := &errs.PermissionError{
		Problem: errs.Problem{
			Category: errs.CategoryAuthorization,
			Subtype:  errs.SubtypeMissingScope,
			Message:  "access denied: app cli_x has not applied for the required scope(s)",
		},
	}
	got := enhancePermissionError(err, scopes)
	var permErr *errs.PermissionError
	if !errors.As(got, &permErr) {
		t.Fatalf("expected *PermissionError, got %T", got)
	}
	if !strings.Contains(permErr.Hint, "drive:drive:read") {
		t.Errorf("hint %q missing scope info", permErr.Hint)
	}
}

// TestEnhancePermissionError_NonPermissionErrorsPassThrough pins that any
// error that is not an *errs.PermissionError is returned unchanged. Typed
// routing means the upstream message text never flips an unrelated error into
// the permission-enhancement path.
func TestEnhancePermissionError_NonPermissionErrorsPassThrough(t *testing.T) {
	scopes := []string{"contact:contact:read"}
	cases := []struct {
		name string
		err  error
	}{
		{"api error with permission keyword", errs.NewAPIError(errs.SubtypeUnknown, "Permission denied for resource")},
		{"api error with scope keyword", errs.NewAPIError(errs.SubtypeUnknown, "Insufficient scope for operation")},
		{"network error", errs.NewNetworkError(errs.SubtypeNetworkTransport, "request unauthorized by server")},
		{"plain error", fmt.Errorf("plain error")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := enhancePermissionError(tc.err, scopes)
			if got != tc.err {
				t.Errorf("expected original error returned, got %T: %v", got, got)
			}
		})
	}
}

// TestEnhancePermissionError_PermissionErrorGetsScopeHint pins that an
// *errs.PermissionError is enhanced with a hint that names the required
// scopes and the `auth login --scope ...` recovery action.
func TestEnhancePermissionError_PermissionErrorGetsScopeHint(t *testing.T) {
	scopes := []string{"calendar:calendar:read", "drive:drive:read"}
	err := &errs.PermissionError{
		Problem: errs.Problem{
			Category: errs.CategoryAuthorization,
			Subtype:  errs.SubtypeMissingScope,
			Message:  "no permission",
		},
	}
	got := enhancePermissionError(err, scopes)

	var permErr *errs.PermissionError
	if !errors.As(got, &permErr) {
		t.Fatalf("expected *errs.PermissionError, got %T: %v", got, got)
	}
	if permErr.Hint == "" {
		t.Fatal("expected non-empty hint")
	}
	if !strings.Contains(permErr.Hint, "scope") {
		t.Errorf("hint %q does not mention scope", permErr.Hint)
	}
	for _, s := range scopes {
		if !strings.Contains(permErr.Hint, s) {
			t.Errorf("hint %q does not contain scope %q", permErr.Hint, s)
		}
	}
}

func TestCheckShortcutScopes_PropagatesContextCancellation(t *testing.T) {
	f := &cmdutil.Factory{
		Credential: credential.NewCredentialProvider(nil, nil, &scopeCheckTokenResolver{err: context.Canceled}, nil),
	}

	err := checkShortcutScopes(f, context.Background(), core.AsUser, &core.CliConfig{AppID: "app-1"}, []string{"im:message:read"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("checkShortcutScopes() error = %v, want context.Canceled", err)
	}
}

// TestCheckShortcutScopes_ReturnsTypedPermissionError pins that the local
// precheck — when it finds the issued token is missing required scopes —
// emits a typed *errs.PermissionError with Subtype MissingScope, the resolved
// Identity, and the deterministic MissingScopes set. AI/script consumers
// downstream rely on these structured fields instead of parsing the hint
// string. The Hint still carries the actionable `auth login --scope ...`
// command for human consumers.
func TestCheckShortcutScopes_ReturnsTypedPermissionError(t *testing.T) {
	f := &cmdutil.Factory{
		Credential: credential.NewCredentialProvider(nil, nil, &scopeCheckTokenResolver{
			result: &credential.TokenResult{Token: "t", Scopes: "im:message:read calendar:calendar:read"},
		}, nil),
	}

	required := []string{"im:message:read", "drive:drive:read", "docx:document:read"}
	err := checkShortcutScopes(f, context.Background(), core.AsUser, &core.CliConfig{AppID: "app-1"}, required)
	if err == nil {
		t.Fatal("expected error when token is missing required scopes, got nil")
	}

	var permErr *errs.PermissionError
	if !errors.As(err, &permErr) {
		t.Fatalf("expected *errs.PermissionError, got %T: %v", err, err)
	}
	if permErr.Category != errs.CategoryAuthorization {
		t.Errorf("Category = %q, want %q", permErr.Category, errs.CategoryAuthorization)
	}
	if permErr.Subtype != errs.SubtypeMissingScope {
		t.Errorf("Subtype = %q, want %q", permErr.Subtype, errs.SubtypeMissingScope)
	}
	if permErr.Identity != string(core.AsUser) {
		t.Errorf("Identity = %q, want %q", permErr.Identity, string(core.AsUser))
	}
	wantMissing := map[string]bool{"drive:drive:read": true, "docx:document:read": true}
	for _, m := range permErr.MissingScopes {
		if !wantMissing[m] {
			t.Errorf("unexpected MissingScopes entry %q (granted scopes should not appear)", m)
		}
		delete(wantMissing, m)
	}
	if len(wantMissing) != 0 {
		t.Errorf("MissingScopes %v did not include expected entries %v", permErr.MissingScopes, wantMissing)
	}
	if permErr.Hint == "" {
		t.Error("Hint must carry the `auth login --scope ...` recovery action")
	}
	if !strings.Contains(permErr.Hint, "auth login") {
		t.Errorf("Hint = %q, want it to mention `auth login`", permErr.Hint)
	}
}

func TestCheckShortcutScopeRequirements_AllowsAnyCandidateGroup(t *testing.T) {
	tests := []struct {
		name          string
		grantedScopes string
	}{
		{"meetingevent_read", "vc:meeting.meetingevent:read"},
		{"bot_join_write", "vc:meeting.bot.join:write"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &cmdutil.Factory{
				Credential: credential.NewCredentialProvider(nil, nil, &scopeCheckTokenResolver{
					result: &credential.TokenResult{Token: "t", Scopes: tt.grantedScopes},
				}, nil),
			}
			shortcut := &Shortcut{
				ScopeGroups: [][]string{
					{"vc:meeting.meetingevent:read"},
					{"vc:meeting.bot.join:write"},
				},
			}

			err := checkShortcutScopeRequirements(f, context.Background(), core.AsBot, &core.CliConfig{AppID: "app-1"}, shortcut)
			if err != nil {
				t.Fatalf("checkShortcutScopeRequirements() error = %v, want nil", err)
			}
		})
	}
}

func TestCheckShortcutScopeRequirements_ErrorsWhenNoCandidateGroupMatches(t *testing.T) {
	f := &cmdutil.Factory{
		Credential: credential.NewCredentialProvider(nil, nil, &scopeCheckTokenResolver{
			result: &credential.TokenResult{Token: "t", Scopes: "im:message:read"},
		}, nil),
	}
	shortcut := &Shortcut{
		ScopeGroups: [][]string{
			{"vc:meeting.meetingevent:read"},
			{"vc:meeting.bot.join:write"},
		},
		ScopeGroupHints: []string{"UAT recommended", "TAT recommended"},
	}

	err := checkShortcutScopeRequirements(f, context.Background(), core.AsBot, &core.CliConfig{AppID: "app-1"}, shortcut)
	if err == nil {
		t.Fatal("expected error when no candidate scope group matches")
	}
	var permErr *errs.PermissionError
	if !errors.As(err, &permErr) {
		t.Fatalf("expected *errs.PermissionError, got %T: %v", err, err)
	}
	for _, want := range []string{
		"vc:meeting.meetingevent:read",
		"vc:meeting.bot.join:write",
		"UAT recommended",
		"TAT recommended",
		"OR",
	} {
		if !strings.Contains(permErr.Hint, want) {
			t.Fatalf("hint %q missing %q", permErr.Hint, want)
		}
	}
	wantMissing := map[string]bool{
		"vc:meeting.meetingevent:read": true,
		"vc:meeting.bot.join:write":    true,
	}
	for _, scope := range permErr.MissingScopes {
		delete(wantMissing, scope)
	}
	if len(wantMissing) != 0 {
		t.Fatalf("MissingScopes = %v, want to include %v", permErr.MissingScopes, wantMissing)
	}
}

func TestCheckShortcutScopeRequirements_StillRequiresBaseScopes(t *testing.T) {
	f := &cmdutil.Factory{
		Credential: credential.NewCredentialProvider(nil, nil, &scopeCheckTokenResolver{
			result: &credential.TokenResult{Token: "t", Scopes: "vc:meeting.bot.join:write"},
		}, nil),
	}
	shortcut := &Shortcut{
		Scopes:      []string{"im:message:read"},
		ScopeGroups: [][]string{{"vc:meeting.bot.join:write"}},
	}

	err := checkShortcutScopeRequirements(f, context.Background(), core.AsBot, &core.CliConfig{AppID: "app-1"}, shortcut)
	if err == nil {
		t.Fatal("expected missing base scope error")
	}
	var permErr *errs.PermissionError
	if !errors.As(err, &permErr) {
		t.Fatalf("expected *errs.PermissionError, got %T: %v", err, err)
	}
	if got := strings.Join(permErr.MissingScopes, " "); got != "im:message:read" {
		t.Fatalf("MissingScopes = %q, want im:message:read", got)
	}
}

func TestCheckShortcutScopes_IgnoresNonContextTokenErrors(t *testing.T) {
	f := &cmdutil.Factory{
		Credential: credential.NewCredentialProvider(nil, nil, &scopeCheckTokenResolver{err: errors.New("token cache unavailable")}, nil),
	}

	err := checkShortcutScopes(f, context.Background(), core.AsUser, &core.CliConfig{AppID: "app-1"}, []string{"im:message:read"})
	if err != nil {
		t.Fatalf("checkShortcutScopes() error = %v, want nil", err)
	}
}

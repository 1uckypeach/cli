// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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

type orderedScopeTokenResolver struct {
	calls  int
	events *[]string
	result *credential.TokenResult
}

func (r *orderedScopeTokenResolver) ResolveToken(_ context.Context, req credential.TokenSpec) (*credential.TokenResult, error) {
	r.calls++
	if r.events != nil {
		*r.events = append(*r.events, "resolve:"+string(req.Type))
	}
	return r.result, nil
}

func TestRunShortcut_HighRiskScopePreflightOrdering(t *testing.T) {
	tests := []struct {
		identity  core.Identity
		tokenType credential.TokenType
	}{
		{identity: core.AsUser, tokenType: credential.TokenTypeUAT},
		{identity: core.AsBot, tokenType: credential.TokenTypeTAT},
	}

	for _, tt := range tests {
		t.Run(string(tt.identity), func(t *testing.T) {
			t.Run("confirmation before scope preflight", func(t *testing.T) {
				resolver := &orderedScopeTokenResolver{
					result: &credential.TokenResult{Token: "token", Scopes: "test:write"},
				}
				executeCalls := 0
				shortcut := scopeOrderingShortcut(&executeCalls, nil)
				factory, cmd := scopeOrderingRuntime(t, shortcut, resolver, tt.identity)

				err := runShortcut(cmd, factory, shortcut, false)
				problem, ok := errs.ProblemOf(err)
				if !ok || problem.Subtype != errs.SubtypeConfirmationRequired {
					t.Fatalf("error = %T %v, want confirmation_required", err, err)
				}
				if resolver.calls != 0 {
					t.Fatalf("token resolver calls = %d, want 0", resolver.calls)
				}
				if executeCalls != 0 {
					t.Fatalf("Execute calls = %d, want 0", executeCalls)
				}
			})

			t.Run("dry run before scope preflight", func(t *testing.T) {
				resolver := &orderedScopeTokenResolver{
					result: &credential.TokenResult{Token: "token", Scopes: "test:write"},
				}
				executeCalls := 0
				shortcut := scopeOrderingShortcut(&executeCalls, nil)
				factory, cmd := scopeOrderingRuntime(t, shortcut, resolver, tt.identity)
				if err := cmd.Flags().Set("dry-run", "true"); err != nil {
					t.Fatalf("set dry-run: %v", err)
				}

				if err := runShortcut(cmd, factory, shortcut, false); err != nil {
					t.Fatalf("runShortcut() error = %v", err)
				}
				if resolver.calls != 0 {
					t.Fatalf("token resolver calls = %d, want 0", resolver.calls)
				}
				if executeCalls != 0 {
					t.Fatalf("Execute calls = %d, want 0", executeCalls)
				}
				stdout := factory.IOStreams.Out.(*bytes.Buffer).String()
				if !strings.Contains(stdout, "/open-apis/test/v1/items") {
					t.Fatalf("dry-run output has no request preview: %s", stdout)
				}
			})

			t.Run("confirmed execution checks scopes first", func(t *testing.T) {
				events := []string{}
				resolver := &orderedScopeTokenResolver{
					events: &events,
					result: &credential.TokenResult{Token: "token", Scopes: "test:write"},
				}
				executeCalls := 0
				shortcut := scopeOrderingShortcut(&executeCalls, &events)
				factory, cmd := scopeOrderingRuntime(t, shortcut, resolver, tt.identity)
				if err := cmd.Flags().Set("yes", "true"); err != nil {
					t.Fatalf("set yes: %v", err)
				}

				if err := runShortcut(cmd, factory, shortcut, false); err != nil {
					t.Fatalf("runShortcut() error = %v", err)
				}
				if resolver.calls != 1 {
					t.Fatalf("token resolver calls = %d, want 1", resolver.calls)
				}
				wantEvents := []string{"resolve:" + string(tt.tokenType), "execute"}
				if !reflect.DeepEqual(events, wantEvents) {
					t.Fatalf("events = %#v, want %#v", events, wantEvents)
				}
			})
		})
	}
}

func TestRunShortcut_LowRiskExecutionStillChecksScopes(t *testing.T) {
	events := []string{}
	resolver := &orderedScopeTokenResolver{
		events: &events,
		result: &credential.TokenResult{Token: "token", Scopes: "test:write"},
	}
	executeCalls := 0
	shortcut := scopeOrderingShortcut(&executeCalls, &events)
	shortcut.Risk = "read"
	factory, cmd := scopeOrderingRuntime(t, shortcut, resolver, core.AsUser)

	if err := runShortcut(cmd, factory, shortcut, false); err != nil {
		t.Fatalf("runShortcut() error = %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("token resolver calls = %d, want 1", resolver.calls)
	}
	if want := []string{"resolve:uat", "execute"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestRunShortcut_LocalValidationPrecedesScopeAndConfirmation(t *testing.T) {
	resolver := &orderedScopeTokenResolver{
		result: &credential.TokenResult{Token: "token", Scopes: "test:write"},
	}
	executeCalls := 0
	shortcut := scopeOrderingShortcut(&executeCalls, nil)
	shortcut.Flags = append(shortcut.Flags, Flag{Name: "value"})
	shortcut.Validate = func(_ context.Context, runtime *RuntimeContext) error {
		if runtime.Str("value") == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--value is required").WithParam("--value")
		}
		return nil
	}
	factory, cmd := scopeOrderingRuntime(t, shortcut, resolver, core.AsUser)

	err := runShortcut(cmd, factory, shortcut, false)
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != errs.CategoryValidation || problem.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("error = %T %v, want validation invalid_argument", err, err)
	}
	if resolver.calls != 0 {
		t.Fatalf("token resolver calls = %d, want 0", resolver.calls)
	}
	if executeCalls != 0 {
		t.Fatalf("Execute calls = %d, want 0", executeCalls)
	}
}

func scopeOrderingShortcut(executeCalls *int, events *[]string) *Shortcut {
	return &Shortcut{
		Service:     "test",
		Command:     "+scope-order",
		Risk:        "high-risk-write",
		Scopes:      []string{"test:write"},
		AuthTypes:   []string{"user", "bot"},
		Description: "test scope ordering",
		DryRun: func(_ context.Context, _ *RuntimeContext) *DryRunAPI {
			return NewDryRunAPI().GET("/open-apis/test/v1/items")
		},
		Execute: func(_ context.Context, _ *RuntimeContext) error {
			(*executeCalls)++
			if events != nil {
				*events = append(*events, "execute")
			}
			return nil
		},
	}
}

func scopeOrderingRuntime(
	t *testing.T,
	shortcut *Shortcut,
	resolver *orderedScopeTokenResolver,
	identity core.Identity,
) (*cmdutil.Factory, *cobra.Command) {
	t.Helper()

	factory := newTestFactory()
	factory.Credential = credential.NewCredentialProvider(nil, nil, resolver, nil)
	cmd := newTestShortcutCmd(shortcut, factory)
	if err := cmd.Flags().Set("as", string(identity)); err != nil {
		t.Fatalf("set as: %v", err)
	}
	return factory, cmd
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

func TestCheckShortcutScopes_IgnoresNonContextTokenErrors(t *testing.T) {
	f := &cmdutil.Factory{
		Credential: credential.NewCredentialProvider(nil, nil, &scopeCheckTokenResolver{err: errors.New("token cache unavailable")}, nil),
	}

	err := checkShortcutScopes(f, context.Background(), core.AsUser, &core.CliConfig{AppID: "app-1"}, []string{"im:message:read"})
	if err != nil {
		t.Fatalf("checkShortcutScopes() error = %v, want nil", err)
	}
}

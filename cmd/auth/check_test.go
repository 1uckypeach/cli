// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	larkauth "github.com/larksuite/cli/internal/auth"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/recovery"
	"github.com/larksuite/cli/internal/surface"
	"github.com/zalando/go-keyring"
)

// `lark-cli auth check` is a predicate command: its README contract is
// `exit 0 = ok, 1 = missing`. The JSON answer goes to stdout; stderr stays
// empty so callers can write `if lark-cli auth check ...; then ... fi`
// without their logs getting polluted by an error envelope on the negative
// branch. These tests pin that contract end-to-end through the dispatcher.

func TestAuthCheckRun_NotLoggedIn_ExitOneWithStdoutOnly(t *testing.T) {
	f, stdout, stderr, _ := cmdutil.TestFactory(t, &core.CliConfig{
		AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu,
		// UserOpenId left empty: triggers the not_logged_in branch.
	})

	err := authCheckRun(&CheckOptions{Factory: f, Scope: "calendar:calendar:read"})

	if got := output.ExitCodeOf(err); got != 1 {
		t.Errorf("exit code = %d, want 1 (predicate 'missing' signal)", got)
	}
	var bare *output.BareError
	if !errors.As(err, &bare) {
		t.Fatalf("expected *output.BareError (ErrBare), got %T: %v", err, err)
	}

	if stderr.Len() != 0 {
		t.Errorf("stderr must stay empty for predicate negative answer, got:\n%s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout must be valid JSON: %v\nstdout=%s", err, stdout.String())
	}
	if payload["ok"] != false {
		t.Errorf("stdout.ok = %v, want false", payload["ok"])
	}
	if payload["error"] != "not_logged_in" {
		t.Errorf("stdout.error = %v, want 'not_logged_in'", payload["error"])
	}
}

func TestAuthCheckRun_NoStoredToken_ExitOneWithStdoutOnly(t *testing.T) {
	f, stdout, stderr, _ := cmdutil.TestFactory(t, &core.CliConfig{
		AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu,
		UserOpenId: "ou_user", UserName: "tester",
	})

	err := authCheckRun(&CheckOptions{Factory: f, Scope: "calendar:calendar:read"})

	if got := output.ExitCodeOf(err); got != 1 {
		t.Errorf("exit code = %d, want 1", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr must stay empty, got:\n%s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout must be valid JSON: %v", err)
	}
	if payload["ok"] != false {
		t.Errorf("stdout.ok = %v, want false", payload["ok"])
	}
	if payload["error"] != "no_token" {
		t.Errorf("stdout.error = %v, want 'no_token'", payload["error"])
	}
}

func TestAuthCheckRun_ScopedTokenPresent_ExitZero(t *testing.T) {
	// Predicate command happy path: stored token covers every required
	// scope. Exit must be 0 (nil error, not ErrBare), stdout carries the
	// `{"ok":true,...}` JSON answer, and stderr stays empty so shell
	// callers can rely on `if lark-cli auth check ...; then` without log
	// pollution. Pairs with the two exit-1 negatives above so both
	// branches of the predicate contract are pinned.
	keyring.MockInit()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LARKSUITE_CLI_DATA_DIR", t.TempDir())

	cfg := &core.CliConfig{
		AppID:      "test-app",
		AppSecret:  "test-secret",
		Brand:      core.BrandFeishu,
		UserOpenId: "ou_user",
		UserName:   "tester",
	}
	now := time.Now()
	if err := larkauth.SetStoredToken(&larkauth.StoredUAToken{
		AppId:            cfg.AppID,
		UserOpenId:       cfg.UserOpenId,
		AccessToken:      "user-access-token",
		RefreshToken:     "refresh-token",
		ExpiresAt:        now.Add(time.Hour).UnixMilli(),
		RefreshExpiresAt: now.Add(24 * time.Hour).UnixMilli(),
		GrantedAt:        now.Add(-time.Hour).UnixMilli(),
		Scope:            "im:message docx:document",
	}); err != nil {
		t.Fatalf("SetStoredToken() error = %v", err)
	}

	f, stdout, stderr, _ := cmdutil.TestFactory(t, cfg)

	err := authCheckRun(&CheckOptions{Factory: f, Scope: "im:message"})

	if err != nil {
		t.Fatalf("expected nil error for happy path (exit 0), got %v", err)
	}
	if got := output.ExitCodeOf(err); got != 0 {
		t.Errorf("exit code = %d, want 0", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr must stay empty for predicate exit-0 answer, got:\n%s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout must be valid JSON: %v\nstdout=%s", err, stdout.String())
	}
	if payload["ok"] != true {
		t.Errorf("stdout.ok = %v, want true", payload["ok"])
	}
	granted, ok := payload["granted"].([]any)
	if !ok || len(granted) != 1 || granted[0] != "im:message" {
		t.Errorf("stdout.granted = %v, want [im:message]", payload["granted"])
	}
	if payload["missing"] != nil {
		t.Errorf("stdout.missing = %v, want nil/absent on happy path", payload["missing"])
	}
	if _, has := payload["suggestion"]; has {
		t.Errorf("stdout.suggestion must be absent on happy path; got %v", payload["suggestion"])
	}
}

func TestAuthCheckRun_EmptyScopeIsValidationError(t *testing.T) {
	// Scope validation is a real input error, not a predicate negative
	// answer — it must surface as a typed ValidationError with the normal
	// stderr envelope, distinct from the silent ErrBare predicate path.
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{
		AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu,
	})

	err := authCheckRun(&CheckOptions{Factory: f, Scope: "   "})
	if err == nil {
		t.Fatal("expected validation error for empty --scope")
	}
	if got := output.ExitCodeOf(err); got != output.ExitValidation {
		t.Errorf("exit code = %d, want ExitValidation (%d)", got, output.ExitValidation)
	}
}

func TestAuthCheckRun_ConcealedLoginOmitsSuggestion(t *testing.T) {
	keyring.MockInit()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LARKSUITE_CLI_DATA_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())

	cfg := &core.CliConfig{
		AppID:      "test-app",
		AppSecret:  "test-secret",
		Brand:      core.BrandFeishu,
		UserOpenId: "ou_user",
		UserName:   "tester",
	}
	now := time.Now()
	if err := larkauth.SetStoredToken(&larkauth.StoredUAToken{
		AppId:            cfg.AppID,
		UserOpenId:       cfg.UserOpenId,
		AccessToken:      "user-access-token",
		RefreshToken:     "refresh-token",
		ExpiresAt:        now.Add(time.Hour).UnixMilli(),
		RefreshExpiresAt: now.Add(24 * time.Hour).UnixMilli(),
		GrantedAt:        now.Add(-time.Hour).UnixMilli(),
		Scope:            "im:message",
	}); err != nil {
		t.Fatalf("SetStoredToken() error = %v", err)
	}

	visibleFactory, visibleStdout, _, _ := cmdutil.TestFactory(t, cfg)
	if err := authCheckRun(&CheckOptions{
		Factory: visibleFactory,
		Scope:   "calendar:calendar:read",
	}); output.ExitCodeOf(err) != 1 {
		t.Fatalf("default check exit = %d, want predicate miss exit 1", output.ExitCodeOf(err))
	}
	var visiblePayload map[string]any
	if err := json.Unmarshal(visibleStdout.Bytes(), &visiblePayload); err != nil {
		t.Fatalf("default stdout must be valid JSON: %v", err)
	}
	const wantSuggestion = "run `lark-cli auth login --scope \"calendar:calendar:read\" --no-wait --json` to get device_code and verification_url; present verification_url to the user exactly and end this turn; after the user confirms authorization, run `lark-cli auth login --device-code <device_code>` in a later turn to finish login"
	if suggestion, _ := visiblePayload["suggestion"].(string); suggestion != wantSuggestion {
		t.Fatalf("default suggestion = %q, want executable split-flow recovery %q", suggestion, wantSuggestion)
	}

	f, stdout, stderr, _ := cmdutil.TestFactory(t, cfg)
	plan := surface.NewPlan(map[surface.CommandID]surface.CommandState{
		surface.CommandAuthLogin: surface.CommandConcealed,
	})
	err := authCheckRunWithRecovery(
		&CheckOptions{Factory: f, Scope: "calendar:calendar:read"},
		recovery.NewProjector(func() *surface.Plan { return plan }),
	)
	if got := output.ExitCodeOf(err); got != 1 {
		t.Fatalf("exit code = %d, want predicate miss exit 1", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must stay empty, got:\n%s", stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout must be valid JSON: %v\nstdout=%s", err, stdout.String())
	}
	if _, ok := payload["suggestion"]; ok {
		t.Fatalf("concealed auth/login left a dead suggestion: %#v", payload["suggestion"])
	}
	if missing, ok := payload["missing"].([]any); !ok || len(missing) != 1 {
		t.Fatalf("projection removed missing-scope facts: %#v", payload["missing"])
	}
}

// TestAuthCheckRun_UnusableTokenFailsInsteadOfReportingScopes pins that the
// predicate does not answer "granted" from a record that cannot make a call.
// Both statuses below leave the scope list intact and both make `--as auto`
// fall back to bot, so answering from either one contradicts the rest of the
// tree.
func TestAuthCheckRun_UnusableTokenFailsInsteadOfReportingScopes(t *testing.T) {
	tests := []struct {
		name        string
		accessToken string
		expiresIn   time.Duration
		refreshIn   time.Duration
		wantError   string
	}{
		{name: "corrupted", accessToken: "", expiresIn: time.Hour, refreshIn: 24 * time.Hour, wantError: "corrupted_token"},
		{name: "expired", accessToken: "user-access-token", expiresIn: -2 * time.Hour, refreshIn: -time.Hour, wantError: "expired_token"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			keyring.MockInit()
			t.Setenv("HOME", t.TempDir())
			t.Setenv("LARKSUITE_CLI_DATA_DIR", t.TempDir())

			cfg := &core.CliConfig{
				AppID:      "test-app",
				AppSecret:  "test-secret",
				Brand:      core.BrandFeishu,
				UserOpenId: "ou_user",
				UserName:   "tester",
			}
			now := time.Now()
			if err := larkauth.SetStoredToken(&larkauth.StoredUAToken{
				AppId:            cfg.AppID,
				UserOpenId:       cfg.UserOpenId,
				AccessToken:      tc.accessToken,
				RefreshToken:     "refresh-token",
				ExpiresAt:        now.Add(tc.expiresIn).UnixMilli(),
				RefreshExpiresAt: now.Add(tc.refreshIn).UnixMilli(),
				Scope:            "im:message docx:document",
			}); err != nil {
				t.Fatalf("SetStoredToken() error = %v", err)
			}

			f, stdout, _, _ := cmdutil.TestFactory(t, cfg)

			err := authCheckRun(&CheckOptions{Factory: f, Scope: "im:message"})
			if err == nil {
				t.Fatalf("expected a non-zero exit for a %s token", tc.name)
			}
			if got := output.ExitCodeOf(err); got != 1 {
				t.Errorf("exit code = %d, want 1", got)
			}

			var got map[string]interface{}
			if jsonErr := json.Unmarshal(stdout.Bytes(), &got); jsonErr != nil {
				t.Fatalf("json.Unmarshal(stdout) error = %v, stdout:\n%s", jsonErr, stdout.String())
			}
			if ok, _ := got["ok"].(bool); ok {
				t.Fatalf("ok = true, want false; stdout:\n%s", stdout.String())
			}
			if got["error"] != tc.wantError {
				t.Fatalf("error = %v, want %s; stdout:\n%s", got["error"], tc.wantError, stdout.String())
			}
			if _, present := got["granted"]; present {
				t.Fatalf("granted must be absent for a %s token, stdout:\n%s", tc.name, stdout.String())
			}
		})
	}
}

// TestAuthCheckRun_NeedsRefreshTokenStillReportsScopes pins the other side of
// the boundary: a token due for refresh still serves calls after the automatic
// refresh on next use, so rejecting it would make the predicate uselessly strict.
func TestAuthCheckRun_NeedsRefreshTokenStillReportsScopes(t *testing.T) {
	keyring.MockInit()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LARKSUITE_CLI_DATA_DIR", t.TempDir())

	cfg := &core.CliConfig{
		AppID:      "test-app",
		AppSecret:  "test-secret",
		Brand:      core.BrandFeishu,
		UserOpenId: "ou_user",
		UserName:   "tester",
	}
	now := time.Now()
	if err := larkauth.SetStoredToken(&larkauth.StoredUAToken{
		AppId:            cfg.AppID,
		UserOpenId:       cfg.UserOpenId,
		AccessToken:      "user-access-token",
		RefreshToken:     "refresh-token",
		ExpiresAt:        now.Add(time.Minute).UnixMilli(),
		RefreshExpiresAt: now.Add(24 * time.Hour).UnixMilli(),
		Scope:            "im:message docx:document",
	}); err != nil {
		t.Fatalf("SetStoredToken() error = %v", err)
	}
	if got := larkauth.TokenStatus(larkauth.GetStoredToken(cfg.AppID, cfg.UserOpenId)); got != larkauth.TokenStatusNeedsRefresh {
		t.Fatalf("fixture status = %q, want %q", got, larkauth.TokenStatusNeedsRefresh)
	}

	f, stdout, _, _ := cmdutil.TestFactory(t, cfg)

	if err := authCheckRun(&CheckOptions{Factory: f, Scope: "im:message"}); err != nil {
		t.Fatalf("authCheckRun() error = %v, want nil", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(stdout) error = %v", err)
	}
	if ok, _ := got["ok"].(bool); !ok {
		t.Fatalf("ok = false, want true for a refreshable token; stdout:\n%s", stdout.String())
	}
}

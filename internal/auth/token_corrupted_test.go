// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/keychain"
)

// TestTokenStatus_CorruptedWhenAccessTokenEmpty pins that freshness is judged on
// the access token first. Timestamps alone cannot tell a usable record from an
// empty one, and every consumer keys off this status.
func TestTokenStatus_CorruptedWhenAccessTokenEmpty(t *testing.T) {
	future := time.Now().Add(48 * time.Hour).UnixMilli()

	tests := []struct {
		name  string
		token *StoredUAToken
		want  string
	}{
		{
			name:  "nil record",
			token: nil,
			want:  TokenStatusCorrupted,
		},
		{
			name:  "empty access token with healthy expiry",
			token: &StoredUAToken{AccessToken: "", ExpiresAt: future, RefreshExpiresAt: future},
			want:  TokenStatusCorrupted,
		},
		{
			name:  "whitespace-only access token",
			token: &StoredUAToken{AccessToken: "   ", ExpiresAt: future, RefreshExpiresAt: future},
			want:  TokenStatusCorrupted,
		},
		{
			name:  "empty access token with expired refresh window",
			token: &StoredUAToken{AccessToken: "", ExpiresAt: 1, RefreshExpiresAt: 1},
			want:  TokenStatusCorrupted,
		},
		{
			name:  "populated access token stays valid",
			token: &StoredUAToken{AccessToken: "u-token", ExpiresAt: future, RefreshExpiresAt: future},
			want:  TokenStatusValid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := TokenStatus(tc.token); got != tc.want {
				t.Fatalf("TokenStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTokenStatus_MisspelledAccessTokenFieldIsCorrupted reproduces the reported
// failure end to end: a writer that spells the field "userAccessToken" produces
// JSON that unmarshals without error and whose expiry looks healthy. Before this
// contract the record reported "valid" and handed an empty bearer token onward.
func TestTokenStatus_MisspelledAccessTokenFieldIsCorrupted(t *testing.T) {
	keyring.MockInit()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LARKSUITE_CLI_DATA_DIR", t.TempDir())

	const appID = "cli_typo"
	const openID = "ou_typo"
	future := time.Now().Add(48 * time.Hour).UnixMilli()

	raw := fmt.Sprintf(
		`{"appId":%q,"userOpenId":%q,"userAccessToken":"u-real-value","refreshToken":"r","expiresAt":%d,"refreshExpiresAt":%d,"scope":"im:message"}`,
		appID, openID, future, future)
	if err := keychain.Set(keychain.LarkCliService, appID+":"+openID, raw); err != nil {
		t.Fatalf("keychain.Set() error = %v", err)
	}

	stored := GetStoredToken(appID, openID)
	if stored == nil {
		t.Fatal("GetStoredToken() = nil, want the parsed record so the caller can report why it is unusable")
	}
	if stored.AccessToken != "" {
		t.Fatalf("AccessToken = %q, want empty (the misspelled field must not populate it)", stored.AccessToken)
	}
	if got := TokenStatus(stored); got != TokenStatusCorrupted {
		t.Fatalf("TokenStatus() = %q, want %q", got, TokenStatusCorrupted)
	}
}

// TestGetValidAccessToken_CorruptedRecordFailsAndSurvives pins two things: the
// empty token is never returned to a caller, and the corrupted file is left on
// disk. Deleting it would erase the evidence of the writer bug that produced it.
func TestGetValidAccessToken_CorruptedRecordFailsAndSurvives(t *testing.T) {
	keyring.MockInit()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LARKSUITE_CLI_DATA_DIR", t.TempDir())

	const appID = "cli_corrupt"
	const openID = "ou_corrupt"
	future := time.Now().Add(48 * time.Hour).UnixMilli()

	if err := SetStoredToken(&StoredUAToken{
		AppId:            appID,
		UserOpenId:       openID,
		AccessToken:      "",
		RefreshToken:     "refresh-token",
		ExpiresAt:        future,
		RefreshExpiresAt: future,
	}); err != nil {
		t.Fatalf("SetStoredToken() error = %v", err)
	}

	token, err := GetValidAccessToken(nil, UATCallOptions{AppId: appID, UserOpenId: openID})
	if err == nil {
		t.Fatal("GetValidAccessToken() error = nil, want a corrupted-token failure")
	}
	if token != "" {
		t.Fatalf("GetValidAccessToken() token = %q, want empty", token)
	}

	problem, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("ProblemOf() ok = false, want a typed error; got %v", err)
	}
	if problem.Category != errs.CategoryAuthentication {
		t.Fatalf("category = %q, want %q", problem.Category, errs.CategoryAuthentication)
	}
	if problem.Subtype != errs.SubtypeTokenInvalid {
		t.Fatalf("subtype = %q, want %q", problem.Subtype, errs.SubtypeTokenInvalid)
	}
	// Cause preservation, asserted directly rather than through the helper: the
	// sentinel must stay reachable via errors.As for every existing consumer.
	var needAuth *NeedAuthorizationError
	if !errors.As(err, &needAuth) {
		t.Fatalf("errors.As(*NeedAuthorizationError) = false, want the sentinel preserved in the cause chain; got %v", err)
	}
	if needAuth.UserOpenId != openID {
		t.Fatalf("cause UserOpenId = %q, want %q", needAuth.UserOpenId, openID)
	}
	if !IsNeedUserAuthorizationError(err) {
		t.Fatal("IsNeedUserAuthorizationError() = false, want true so existing re-authorization handling still fires")
	}

	if stored := GetStoredToken(appID, openID); stored == nil {
		t.Fatal("GetStoredToken() = nil after the failure, want the corrupted record preserved for diagnosis")
	}
}

// TestNewCorruptedUserTokenError_NamesTheFieldAndCause keeps the message
// actionable: an agent reading it must be able to tell this apart from an
// expired token and know the field to look at.
func TestNewCorruptedUserTokenError_NamesTheFieldAndCause(t *testing.T) {
	err := NewCorruptedUserTokenError("ou_msg")

	problem, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("ProblemOf() ok = false, want typed error; got %v", err)
	}
	if problem.Category != errs.CategoryAuthentication {
		t.Fatalf("category = %q, want %q", problem.Category, errs.CategoryAuthentication)
	}
	if problem.Subtype != errs.SubtypeTokenInvalid {
		t.Fatalf("subtype = %q, want %q", problem.Subtype, errs.SubtypeTokenInvalid)
	}
	var needAuth *NeedAuthorizationError
	if !errors.As(err, &needAuth) {
		t.Fatalf("errors.As(*NeedAuthorizationError) = false, want the sentinel preserved; got %v", err)
	}
	for _, want := range []string{"accessToken is empty", "userAccessToken", "ou_msg"} {
		if !strings.Contains(problem.Message, want) {
			t.Fatalf("message %q missing %q", problem.Message, want)
		}
	}

	// The envelope must stay machine-readable for consumers that parse stderr.
	if _, marshalErr := json.Marshal(problem); marshalErr != nil {
		t.Fatalf("json.Marshal(problem) error = %v", marshalErr)
	}
}

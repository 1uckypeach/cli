// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"fmt"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/recovery"
)

const (
	needUserAuthorizationMarker = "need_user_authorization"
)

// TokenRetryCodes contains error codes that allow retry after token refresh.
var TokenRetryCodes = map[int]bool{
	output.LarkErrTokenInvalid: true,
	output.LarkErrTokenExpired: true,
}

// NeedAuthorizationError is the sentinel preserved in the Cause chain of the
// typed missing-UAT error so existing errors.As(&NeedAuthorizationError{})
// consumers keep matching after the construction site moved to the typed
// taxonomy. It is never surfaced on the wire on its own.
type NeedAuthorizationError struct {
	UserOpenId string
}

// Error returns the error message for NeedAuthorizationError.
func (e *NeedAuthorizationError) Error() string {
	return fmt.Sprintf("%s (user: %s)", needUserAuthorizationMarker, e.UserOpenId)
}

// NewNeedUserAuthorizationError builds the typed *errs.AuthenticationError
// returned when no valid UAT exists for userOpenID. The Message keeps the
// need_user_authorization marker, the Hint converges on the same auth-login
// recovery vocabulary as the token-missing surface in internal/client, and the
// legacy *NeedAuthorizationError sentinel is preserved in the Cause chain for
// errors.As / errors.Is traversal.
func NewNeedUserAuthorizationError(userOpenID string) error {
	e := errs.NewAuthenticationError(errs.SubtypeTokenMissing,
		"%s (user: %s)", needUserAuthorizationMarker, userOpenID).
		WithUserOpenID(userOpenID).
		WithCause(&NeedAuthorizationError{UserOpenId: userOpenID})
	return recovery.Attach(e, recovery.UserAuthorization())
}

// NewCorruptedUserTokenError builds the typed error returned when a stored UAT
// parses but carries no usable access token (TokenStatusCorrupted). The subtype
// is token_invalid rather than token_missing because the record exists and its
// content is wrong — an agent that retries the same call unchanged would fail
// identically, so the message names the field and the likely cause.
//
// The legacy *NeedAuthorizationError sentinel stays in the Cause chain: every
// existing IsNeedUserAuthorizationError consumer (root error rendering, scope
// hint folding) must keep treating this as a re-authorization signal.
func NewCorruptedUserTokenError(userOpenID string) error {
	e := errs.NewAuthenticationError(errs.SubtypeTokenInvalid,
		"%s: stored user token is corrupted — accessToken is empty (user: %s); "+
			"if the token file is written by another process, check for a misspelled field name such as \"userAccessToken\"",
		needUserAuthorizationMarker, userOpenID).
		WithUserOpenID(userOpenID).
		WithCause(&NeedAuthorizationError{UserOpenId: userOpenID})
	return recovery.Attach(e, recovery.UserAuthorization())
}

// IsNeedUserAuthorizationError reports whether err represents a missing-UAT
// failure. It matches the legacy *NeedAuthorizationError sentinel, which is
// preserved in the Cause chain of the typed missing-UAT error, so errors.As
// traverses into the typed *errs.AuthenticationError as well.
func IsNeedUserAuthorizationError(err error) bool {
	if err == nil {
		return false
	}

	var needAuthErr *NeedAuthorizationError
	return errors.As(err, &needAuthErr)
}

// SecurityPolicyError is preserved as a Go type alias so existing
// errors.As(&SecurityPolicyError{}) consumers (cmd/root.go etc.) keep working.
// The concrete struct lives in errs/types.go.
type SecurityPolicyError = errs.SecurityPolicyError

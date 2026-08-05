// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/larksuite/cli/internal/keychain"
)

// StoredUAToken represents a stored user access token.
type StoredUAToken struct {
	UserOpenId       string `json:"userOpenId"`
	AppId            string `json:"appId"`
	AccessToken      string `json:"accessToken"`
	RefreshToken     string `json:"refreshToken"`
	ExpiresAt        int64  `json:"expiresAt"`        // Unix ms
	RefreshExpiresAt int64  `json:"refreshExpiresAt"` // Unix ms
	Scope            string `json:"scope"`
	GrantedAt        int64  `json:"grantedAt"` // Unix ms
}

const refreshAheadMs = 5 * 60 * 1000 // 5 minutes

// Token freshness values reported by TokenStatus. Callers must compare against
// these constants rather than bare string literals: a status added later (as
// TokenStatusCorrupted was) is otherwise silently treated as "not expired" by
// every `!= "expired"` check in the tree.
const (
	TokenStatusValid        = "valid"
	TokenStatusNeedsRefresh = "needs_refresh"
	TokenStatusExpired      = "expired"
	// TokenStatusCorrupted means the stored JSON parsed but carries no usable
	// access token. The common cause is a writer that misspelled the field
	// name (e.g. "userAccessToken" instead of "accessToken"): encoding/json
	// silently drops unknown fields, leaving AccessToken empty while the
	// expiry timestamps still look healthy. Reporting such a record as valid
	// makes `auth status` contradict every business command, so it gets its
	// own status instead.
	TokenStatusCorrupted = "corrupted"
)

// accountKey generates a unique key for an account based on its AppID and UserOpenID.
func accountKey(appId, userOpenId string) string {
	return fmt.Sprintf("%s:%s", appId, userOpenId)
}

// MaskToken masks a token for safe logging.
func MaskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

// GetStoredToken reads the stored UAT for a given (appId, userOpenId) pair.
func GetStoredToken(appId, userOpenId string) *StoredUAToken {
	jsonStr, err := keychain.Get(keychain.LarkCliService, accountKey(appId, userOpenId))
	if err != nil || jsonStr == "" {
		return nil
	}
	var token StoredUAToken
	if err := json.Unmarshal([]byte(jsonStr), &token); err != nil {
		return nil
	}
	return &token
}

// SetStoredToken persists a UAT.
func SetStoredToken(token *StoredUAToken) error {
	key := accountKey(token.AppId, token.UserOpenId)
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return keychain.Set(keychain.LarkCliService, key, string(data))
}

// RemoveStoredToken removes a stored UAT.
func RemoveStoredToken(appId, userOpenId string) error {
	return keychain.Remove(keychain.LarkCliService, accountKey(appId, userOpenId))
}

// TokenStatus determines the freshness of a stored token.
//
// The access token is checked before the timestamps: a record whose
// accessToken is empty cannot be used no matter how far in the future its
// expiry sits, and calling it valid would hand an empty bearer token to the
// next API call.
func TokenStatus(token *StoredUAToken) string {
	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return TokenStatusCorrupted
	}
	now := time.Now().UnixMilli()
	if now < token.ExpiresAt-refreshAheadMs {
		return TokenStatusValid
	}
	if now < token.RefreshExpiresAt {
		return TokenStatusNeedsRefresh
	}
	return TokenStatusExpired
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

// appInfo represents the app object returned by GET /open-apis/spark/v1/apps/{appID}.
type appInfo struct {
	AppID       string `json:"app_id"`
	AppType     string `json:"app_type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IconURL     string `json:"icon_url"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	IsPublished bool   `json:"is_published"`
}

// queryAppType fetches the app's type string from the server via
// GET /open-apis/spark/v1/apps/{appID}. The server returns uppercase
// values ("HTML", "FULL_STACK", "MODERN_HTML"); this function normalizes
// to lowercase. Returns "" when the API is unavailable or returns an
// error — callers fall back to legacy behavior.
func queryAppType(ctx context.Context, rctx *common.RuntimeContext, appID string) string {
	path := fmt.Sprintf("%s/apps/%s", apiBasePath, validate.EncodePathSegment(appID))
	data, err := rctx.CallAPITyped("GET", path, nil, nil)
	if err != nil {
		return ""
	}
	appRaw, _ := data["app"].(map[string]interface{})
	if appRaw == nil {
		return ""
	}
	b, err := json.Marshal(appRaw)
	if err != nil {
		return ""
	}
	var info appInfo
	if err := json.Unmarshal(b, &info); err != nil {
		return ""
	}
	return strings.ToLower(info.AppType)
}

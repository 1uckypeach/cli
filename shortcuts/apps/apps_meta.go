// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"fmt"
	"strings"

	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

// queryAppType fetches the app's type string from the server via
// GET /open-apis/spark/v1/apps/{appID}. The server returns uppercase
// values ("HTML", "FULL_STACK", "MODERN_HTML") in data.app.app_type;
// this function normalizes to lowercase ("html", "full_stack", "modern_html").
// Returns "" when the API is unavailable or returns an error — callers
// fall back to legacy behavior.
func queryAppType(ctx context.Context, rctx *common.RuntimeContext, appID string) string {
	path := fmt.Sprintf("%s/apps/%s", apiBasePath, validate.EncodePathSegment(appID))
	data, err := rctx.CallAPITyped("GET", path, nil, nil)
	if err != nil {
		return ""
	}
	app, _ := data["app"].(map[string]interface{})
	if app == nil {
		return ""
	}
	appType, _ := app["app_type"].(string)
	return strings.ToLower(appType)
}

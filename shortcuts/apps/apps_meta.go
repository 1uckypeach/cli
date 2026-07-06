// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"fmt"

	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

// queryAppType fetches the app's type string ("html", "modern_html",
// "full_stack", etc.) from the server. Returns "" when the API is unavailable
// or returns an error — callers fall back to legacy behavior.
func queryAppType(ctx context.Context, rctx *common.RuntimeContext, appID string) string {
	path := fmt.Sprintf("%s/apps/%s/type", apiBasePath, validate.EncodePathSegment(appID))
	data, err := rctx.CallAPITyped("GET", path, nil, nil)
	if err != nil {
		return ""
	}
	appType, _ := data["app_type"].(string)
	return appType
}

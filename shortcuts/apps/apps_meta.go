// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

// appMeta holds the numeric app_type and arch_type returned by the server.
type appMeta struct {
	AppType  int
	ArchType int
}

// queryAppMeta fetches the app's metadata (numeric app_type and arch_type)
// from the server. Returns nil when the API is unavailable or returns an
// error — callers fall back to legacy behavior.
func queryAppMeta(ctx context.Context, rctx *common.RuntimeContext, appID string) *appMeta {
	path := fmt.Sprintf("%s/apps/%s", apiBasePath, validate.EncodePathSegment(appID))
	data, err := rctx.CallAPITyped("GET", path, nil, nil)
	if err != nil {
		return nil
	}
	app, _ := data["app"].(map[string]interface{})
	if app == nil {
		return nil
	}
	appType, ok1 := toInt(app["app_type"])
	archType, ok2 := toInt(app["arch_type"])
	if !ok1 || !ok2 {
		return nil
	}
	return &appMeta{AppType: appType, ArchType: archType}
}

// toInt converts a JSON number (float64 or json.Number) to int.
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := strconv.Atoi(n.String())
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

// isStaticHtml reports whether the app is a legacy static-HTML app
// (app_type=7, arch_type=3) that has no template and no env vars.
func isStaticHtml(meta *appMeta) bool {
	return meta != nil && meta.AppType == 7 && meta.ArchType == 3
}

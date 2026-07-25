// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"

	"github.com/larksuite/cli/internal/authlog"
	"github.com/larksuite/cli/internal/core"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

type authLogger interface {
	LogResponse(path string, status int, logID string)
}

func newAuthLogger() *authlog.Logger {
	return authlog.New(authlog.Options{RuntimeDir: core.GetRuntimeDir})
}

// logHTTPResponse logs the HTTP response details for an authentication request.
// It extracts the request path, status code, and x-tt-logid from the given HTTP response.
func logHTTPResponse(logger authLogger, resp *http.Response) {
	if logger == nil || resp == nil {
		return
	}

	path := "missing"
	if resp.Request != nil && resp.Request.URL != nil {
		path = resp.Request.URL.Path
	}

	logger.LogResponse(path, resp.StatusCode, resp.Header.Get("x-tt-logid"))
}

// logSDKResponse logs the SDK response details for an authentication request.
// It extracts the status code and x-tt-logid from the given API response object.
func logSDKResponse(logger authLogger, path string, apiResp *larkcore.ApiResp) {
	if logger == nil {
		return
	}
	if path == "" {
		path = "missing"
	}

	if apiResp == nil {
		logger.LogResponse(path, 0, "")
		return
	}

	logger.LogResponse(path, apiResp.StatusCode, apiResp.Header.Get("x-tt-logid"))
}

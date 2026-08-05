// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package client

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/errclass"
)

func parseRetryAfter(header http.Header, now time.Time) (int, string) {
	return errclass.ParseRetryAfter(header, now)
}

func parseRateLimitResult(rawBody []byte) interface{} {
	return errclass.ParseRateLimitJSON(rawBody)
}

func isBusinessRateLimit(result interface{}) bool {
	return errclass.IsBusinessRateLimit(result)
}

func rateLimitError(status int, header http.Header, result interface{}, rawBody []byte, classified error) error {
	businessRateLimit := isBusinessRateLimit(result)
	if status != http.StatusTooManyRequests && !businessRateLimit {
		return nil
	}
	if status == http.StatusTooManyRequests {
		if result == nil && len(rawBody) > 0 {
			result = parseRateLimitResult(rawBody)
		}
		return errclass.ClassifyHTTPRateLimit(status, header, result, classified, time.Now())
	}

	var apiErr *errs.APIError
	var existing *errs.APIError
	if errors.As(classified, &existing) {
		apiErr = existing
	}
	if apiErr == nil {
		message := "request rate limit exceeded"
		if resultMap, ok := result.(map[string]interface{}); ok {
			if msg, _ := resultMap["msg"].(string); strings.TrimSpace(msg) != "" {
				message = msg
			}
		}
		apiErr = errs.NewAPIError(errs.SubtypeRateLimit, "%s", message).WithCode(99991400)
	}
	apiErr.LogID = errclass.RateLimitLogID(result, header)

	seconds, source := errclass.ParseRetryAfter(header, time.Now())
	guidance := fmt.Sprintf("wait %d seconds before reevaluating; retryable does not mean a write request is safe to replay—verify the operation result or idempotency before retrying", seconds)
	apiErr.Hint = mergeRateLimitHint(apiErr.Hint, guidance)
	return apiErr.WithRetryable().WithRetryAfter(seconds, source)
}

func mergeRateLimitHint(existing, guidance string) string {
	if existing == "" || existing == guidance {
		return guidance
	}
	if strings.Contains(existing, guidance) {
		return existing
	}
	return existing + "; " + guidance
}

// ClassifyRateLimitResponse returns an api/rate_limit error for a bare HTTP
// 429 or business code 99991400. An existing classification for another
// business code is returned unchanged. Callers that already parsed and
// classified the body pass both values so this helper can add header-derived
// recovery metadata without decoding the response a second time.
func ClassifyRateLimitResponse(resp *larkcore.ApiResp, result interface{}, classified error) error {
	if resp == nil {
		return nil
	}
	if parsed := errclass.ParseRateLimitJSON(resp.RawBody); parsed != nil {
		result = parsed
	} else if len(resp.RawBody) > 0 {
		// Do not let a caller projection revive code or log_id fields from a
		// malformed raw response.
		result = nil
		classified = nil
	}
	return rateLimitError(resp.StatusCode, resp.Header, result, resp.RawBody, classified)
}

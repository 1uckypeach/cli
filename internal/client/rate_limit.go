// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package client

import (
	"errors"
	"net/http"
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

func isClassifiedBusinessRateLimit(classified error) bool {
	problem, ok := errs.ProblemOf(classified)
	return ok && problem.Code == 99991400
}

func rateLimitError(status int, header http.Header, result interface{}, rawBody []byte, classified error) error {
	businessRateLimit := isBusinessRateLimit(result)
	classifiedRateLimit := isClassifiedBusinessRateLimit(classified)
	if status != http.StatusTooManyRequests && !businessRateLimit && !classifiedRateLimit {
		return nil
	}

	// Only candidate rate-limit responses pay for a strict raw-body reparse.
	// This prevents malformed JSON prefixes from forging a business code or log
	// ID without decoding every successful API response a second time.
	if len(rawBody) > 0 {
		parsed, parseErr := errclass.DecodeSingleJSON(rawBody)
		if parseErr != nil {
			result = nil
			classified = nil
			if status != http.StatusTooManyRequests {
				return errs.NewInternalError(errs.SubtypeInvalidResponse,
					"failed to validate candidate rate-limit response: %v", parseErr).WithCause(parseErr)
			}
		} else {
			result = parsed
			businessRateLimit = isBusinessRateLimit(result)
			if status != http.StatusTooManyRequests && !businessRateLimit {
				return errs.NewInternalError(errs.SubtypeInvalidResponse,
					"candidate rate-limit classification does not match the response body")
			}
		}
	}

	if status == http.StatusTooManyRequests {
		return errclass.ClassifyHTTPRateLimit(status, header, result, classified, time.Now())
	}

	var apiErr *errs.APIError
	var existing *errs.APIError
	if errors.As(classified, &existing) {
		apiErr = existing
	}
	if apiErr == nil {
		apiErr = errs.NewAPIError(errs.SubtypeRateLimit, errclass.RateLimitMessage).WithCode(99991400)
	}
	apiErr.Message = errclass.RateLimitMessage
	apiErr.LogID = errclass.RateLimitLogID(result, header)

	seconds, source := errclass.ParseRetryAfter(header, time.Now())
	apiErr.Hint = errclass.MergeRateLimitHint(apiErr.Hint, errclass.RateLimitGuidance(seconds))
	return apiErr.WithRetryable().WithRetryAfter(seconds, source)
}

// ClassifyRateLimitResponse returns an api/rate_limit error for a bare HTTP
// 429 or business code 99991400. Non-rate-limit responses return nil so callers
// can retain their existing classification. Ordinary success responses are not
// decoded again; only candidate limits receive strict raw-body validation before
// header-derived recovery metadata is added.
func ClassifyRateLimitResponse(resp *larkcore.ApiResp, result interface{}, classified error) error {
	if resp == nil {
		return nil
	}
	return rateLimitError(resp.StatusCode, resp.Header, result, resp.RawBody, classified)
}

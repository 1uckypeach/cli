// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package client

import (
	"errors"
	"net/http"
	"strings"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/errclass"
	"github.com/larksuite/cli/internal/util"
)

// APIResponseChecker classifies a successfully decoded Lark API envelope.
// It should return nil for a successful business response.
type APIResponseChecker func(result interface{}) error

// ClassifyAPIResponse is the common response boundary for buffered Lark API
// calls. It decodes the JSON body once for normal responses, applies the
// caller's business-error classifier, gives explicit rate limits precedence,
// and finally rejects otherwise-unclassified HTTP errors.
//
// The decoded result is returned alongside a business error because some
// callers need response data from failed operations.
func ClassifyAPIResponse(resp *larkcore.ApiResp, check APIResponseChecker) (interface{}, error) {
	if resp == nil {
		return nil, errs.NewInternalError(errs.SubtypeInvalidResponse, "API returned a nil response")
	}

	result, parseErr := ParseJSONResponse(resp)
	if parseErr != nil {
		if resp.StatusCode >= http.StatusBadRequest {
			return nil, classifyAPIResponseError(resp, nil, nil)
		}
		return nil, WrapJSONResponseParseError(parseErr, resp.RawBody)
	}

	var classified error
	if check != nil {
		classified = check(result)
	}
	if err := classifyAPIResponseError(resp, result, classified); err != nil {
		return result, err
	}
	return result, nil
}

// classifyAPIResponseError resolves the precedence between HTTP rate limits,
// business-envelope errors, and bare HTTP status failures. The raw body is
// strictly revalidated only when a rate-limit signal is present, preventing a
// malformed JSON prefix from forging trusted recovery metadata without adding
// a second decode to ordinary successful responses.
func classifyAPIResponseError(resp *larkcore.ApiResp, result interface{}, classified error) error {
	if resp == nil {
		return errs.NewInternalError(errs.SubtypeInvalidResponse, "API returned a nil response")
	}

	businessRateLimit := errclass.IsBusinessRateLimit(result)
	classifiedRateLimit := isClassifiedBusinessRateLimit(classified)
	if resp.StatusCode == http.StatusTooManyRequests || businessRateLimit || classifiedRateLimit {
		if len(resp.RawBody) > 0 {
			parsed, parseErr := errclass.DecodeSingleJSON(resp.RawBody)
			if parseErr != nil {
				result = nil
				classified = nil
				if resp.StatusCode != http.StatusTooManyRequests {
					return errs.NewInternalError(errs.SubtypeInvalidResponse,
						"failed to validate candidate rate-limit response: %v", parseErr).WithCause(parseErr)
				}
			} else {
				result = parsed
				businessRateLimit = errclass.IsBusinessRateLimit(result)
				if resp.StatusCode != http.StatusTooManyRequests && !businessRateLimit {
					return errs.NewInternalError(errs.SubtypeInvalidResponse,
						"candidate rate-limit classification does not match the response body")
				}
			}
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			return errclass.ClassifyHTTPRateLimit(resp.StatusCode, resp.Header, result, classified, time.Now())
		}

		var apiErr *errs.APIError
		var existing *errs.APIError
		if errors.As(classified, &existing) {
			apiErr = existing
		}
		if apiErr == nil {
			apiErr = errs.NewAPIError(errs.SubtypeRateLimit, errclass.RateLimitMessage).WithCode(99991400)
		}
		apiErr.Subtype = errs.SubtypeRateLimit
		apiErr.Code = 99991400
		apiErr.Message = errclass.RateLimitMessage
		apiErr.LogID = errclass.RateLimitLogID(result, resp.Header)

		seconds, source := errclass.ParseRetryAfter(resp.Header, time.Now())
		apiErr.Hint = errclass.MergeRateLimitHint(apiErr.Hint, errclass.RateLimitGuidance(seconds))
		return apiErr.WithRetryable().WithRetryAfter(seconds, source)
	}

	if classified != nil {
		return classified
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return httpStatusError(resp.StatusCode, resp.RawBody, resp.Header)
	}
	return nil
}

func isClassifiedBusinessRateLimit(classified error) bool {
	problem, ok := errs.ProblemOf(classified)
	return ok && problem.Code == 99991400
}

// httpStatusError classifies an HTTP error whose body carries no usable
// business error. Header request IDs are attached when they pass the same
// validation used for rate-limit metadata.
func httpStatusError(status int, rawBody []byte, header http.Header) error {
	body := util.TruncateStrWithEllipsis(strings.TrimSpace(string(rawBody)), 500)
	logID := errclass.RateLimitLogID(nil, header)
	if status >= http.StatusInternalServerError {
		err := errs.NewNetworkError(errs.SubtypeNetworkServer,
			"HTTP %d: %s", status, body).
			WithCode(status).
			WithRetryable()
		if logID != "" {
			err = err.WithLogID(logID)
		}
		return err
	}
	subtype := errs.SubtypeUnknown
	if status == http.StatusNotFound {
		subtype = errs.SubtypeNotFound
	}
	err := errs.NewAPIError(subtype, "HTTP %d: %s", status, body).WithCode(status)
	if logID != "" {
		err = err.WithLogID(logID)
	}
	return err
}

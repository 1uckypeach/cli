// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package errclass

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/larksuite/cli/errs"
)

const (
	defaultRetryAfterSeconds = 1
	maxRetryAfterSeconds     = 24 * 60 * 60
	retryAfterSourceHeader   = "retry-after"
	retryAfterSourceDefault  = "default"
	// RateLimitMessage is the stable user-facing message for short-term limits.
	RateLimitMessage = "request rate limit exceeded"
)

// ClassifyHTTPRateLimit classifies only HTTP 429 responses. Business-code
// rate limits returned with another HTTP status remain the caller's concern.
func ClassifyHTTPRateLimit(status int, header http.Header, result any, classified error, now time.Time) error {
	if status != http.StatusTooManyRequests {
		return nil
	}

	businessRateLimit := IsBusinessRateLimit(result)
	if !businessRateLimit && classified != nil {
		if problem, ok := errs.ProblemOf(classified); ok {
			problem.LogID = RateLimitLogID(result, header)
		}
		return classified
	}

	code := http.StatusTooManyRequests
	if businessRateLimit {
		code = 99991400
	}

	var apiErr *errs.APIError
	if businessRateLimit {
		var existing *errs.APIError
		if errors.As(classified, &existing) {
			apiErr = existing
		}
	}
	if apiErr == nil {
		apiErr = errs.NewAPIError(errs.SubtypeRateLimit, RateLimitMessage).WithCode(code)
	}
	if businessRateLimit {
		apiErr.Message = RateLimitMessage
	}

	// Derive the identifier exclusively from validated structured/header
	// sources. This also clears an unsafe value set by an earlier classifier.
	apiErr.LogID = RateLimitLogID(result, header)
	seconds, source := ParseRetryAfter(header, now)
	apiErr.Hint = MergeRateLimitHint(apiErr.Hint, RateLimitGuidance(seconds))
	return apiErr.WithRetryable().WithRetryAfter(seconds, source)
}

// ParseRetryAfter accepts exactly one bounded Retry-After value. Unsupported
// reset headers and ambiguous/malformed values use the conservative default.
func ParseRetryAfter(header http.Header, now time.Time) (int, string) {
	values := header.Values("Retry-After")
	if len(values) != 1 {
		return defaultRetryAfterSeconds, retryAfterSourceDefault
	}
	rawValue := values[0]
	if len(rawValue) > 128 {
		return defaultRetryAfterSeconds, retryAfterSourceDefault
	}
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return defaultRetryAfterSeconds, retryAfterSourceDefault
	}

	allDigits := true
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		seconds, err := strconv.ParseInt(value, 10, 64)
		if err == nil && seconds <= maxRetryAfterSeconds {
			return int(seconds), retryAfterSourceHeader
		}
		return defaultRetryAfterSeconds, retryAfterSourceDefault
	}

	retryAt, err := http.ParseTime(value)
	if err != nil {
		return defaultRetryAfterSeconds, retryAfterSourceDefault
	}
	delay := retryAt.Sub(now)
	if delay <= 0 || delay > maxRetryAfterSeconds*time.Second {
		return defaultRetryAfterSeconds, retryAfterSourceDefault
	}
	seconds := int((delay + time.Second - 1) / time.Second)
	if seconds > maxRetryAfterSeconds {
		return defaultRetryAfterSeconds, retryAfterSourceDefault
	}
	return seconds, retryAfterSourceHeader
}

var errTrailingJSONContent = errors.New("trailing content after JSON value")

// DecodeSingleJSON decodes one JSON value and accepts only JSON whitespace
// after it. The remaining bytes are inspected directly so a trailing value is
// never decoded or allocated.
func DecodeSingleJSON(rawBody []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(rawBody))
	decoder.UseNumber()
	var result any
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	for _, c := range rawBody[decoder.InputOffset():] {
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			return nil, errTrailingJSONContent
		}
	}
	return result, nil
}

// ParseRateLimitJSON decodes exactly one complete JSON value. A malformed
// value, trailing non-whitespace, or a second JSON value is rejected.
func ParseRateLimitJSON(rawBody []byte) any {
	result, err := DecodeSingleJSON(rawBody)
	if err != nil {
		return nil
	}
	return result
}

// RateLimitLogID returns the first valid identifier from the documented
// structured-body and response-header sources.
func RateLimitLogID(result any, header http.Header) string {
	if resultMap, ok := result.(map[string]any); ok {
		if logID := validRateLimitLogID(resultMap["log_id"]); logID != "" {
			return logID
		}
		if errBlock, ok := resultMap["error"].(map[string]any); ok {
			if logID := validRateLimitLogID(errBlock["log_id"]); logID != "" {
				return logID
			}
		}
	}
	for _, name := range []string{"X-Tt-Logid", "X-Request-Id"} {
		values := header.Values(name)
		if len(values) == 1 {
			if logID := validRateLimitLogID(values[0]); logID != "" {
				return logID
			}
		}
	}
	return ""
}

func validRateLimitLogID(value any) string {
	logID, ok := value.(string)
	if !ok {
		return ""
	}
	logID = strings.TrimSpace(logID)
	if len(logID) < 1 || len(logID) > 128 {
		return ""
	}
	for i := 0; i < len(logID); i++ {
		c := logID[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			continue
		}
		return ""
	}
	return logID
}

// IsBusinessRateLimit reports whether result carries the exact integer Lark
// short-term rate-limit code.
func IsBusinessRateLimit(result any) bool {
	resultMap, ok := result.(map[string]any)
	if !ok {
		return false
	}
	return exactBusinessRateLimitCode(resultMap["code"])
}

func exactBusinessRateLimitCode(value any) bool {
	const target int64 = 99991400
	switch code := value.(type) {
	case int:
		return int64(code) == target
	case int8:
		return int64(code) == target
	case int16:
		return int64(code) == target
	case int32:
		return int64(code) == target
	case int64:
		return code == target
	case uint:
		return uint64(code) == uint64(target)
	case uint8:
		return uint64(code) == uint64(target)
	case uint16:
		return uint64(code) == uint64(target)
	case uint32:
		return uint64(code) == uint64(target)
	case uint64:
		return code == uint64(target)
	case float32:
		value := float64(code)
		return !math.IsInf(value, 0) && !math.IsNaN(value) && math.Trunc(value) == value && value == float64(target)
	case float64:
		return !math.IsInf(code, 0) && !math.IsNaN(code) && math.Trunc(code) == code && code == float64(target)
	case json.Number:
		rational, ok := new(big.Rat).SetString(string(code))
		return ok && rational.IsInt() && rational.Num().IsInt64() && rational.Num().Int64() == target
	default:
		return false
	}
}

// RateLimitGuidance returns the canonical retry scheduling and replay-safety hint.
func RateLimitGuidance(seconds int) string {
	return fmt.Sprintf("wait %d seconds before reevaluating; retryable does not mean a write request is safe to replay—verify the operation result or idempotency before retrying", seconds)
}

// MergeRateLimitHint appends the canonical guidance without duplicating it.
func MergeRateLimitHint(existing, guidance string) string {
	if existing == "" || existing == guidance {
		return guidance
	}
	if strings.Contains(existing, guidance) {
		return existing
	}
	return existing + "; " + guidance
}

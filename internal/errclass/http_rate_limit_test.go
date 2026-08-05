// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package errclass

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/larksuite/cli/errs"
)

func TestClassifyHTTPRateLimit_StatusIsolation(t *testing.T) {
	if got := ClassifyHTTPRateLimit(http.StatusBadRequest, nil, map[string]any{"code": 99991400}, nil, time.Now()); got != nil {
		t.Fatalf("non-429 classification = %v, want nil", got)
	}
}

func TestClassifyHTTPRateLimit_RetryAfterSafety(t *testing.T) {
	now := time.Date(2026, 8, 5, 0, 0, 0, 250_000_000, time.UTC)
	tests := []struct {
		name   string
		values []string
		want   int
		source string
	}{
		{name: "zero", values: []string{"0"}, want: 0, source: "retry-after"},
		{name: "one day", values: []string{"86400"}, want: 86400, source: "retry-after"},
		{name: "date rounds up", values: []string{now.Add(2250 * time.Millisecond).Format(http.TimeFormat)}, want: 2, source: "retry-after"},
		{name: "multiple", values: []string{"2", "3"}, want: 1, source: "default"},
		{name: "negative", values: []string{"-1"}, want: 1, source: "default"},
		{name: "overflow", values: []string{"999999999999999999999999"}, want: 1, source: "default"},
		{name: "too large", values: []string{"86401"}, want: 1, source: "default"},
		{name: "expired date", values: []string{now.Add(-time.Second).Format(http.TimeFormat)}, want: 1, source: "default"},
		{name: "too long", values: []string{strings.Repeat("1", 129)}, want: 1, source: "default"},
		{name: "raw too long before trim", values: []string{strings.Repeat(" ", 128) + "0"}, want: 1, source: "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := make(http.Header)
			if tt.values != nil {
				header["Retry-After"] = tt.values
			}
			err := ClassifyHTTPRateLimit(http.StatusTooManyRequests, header, nil, nil, now)
			var apiErr *errs.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %T (%v), want APIError", err, err)
			}
			problem, ok := errs.ProblemOf(err)
			if !ok || problem.Category != errs.CategoryAPI || problem.Subtype != errs.SubtypeRateLimit {
				t.Fatalf("problem = %#v, want api/rate_limit", problem)
			}
			if apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != tt.want || apiErr.RetryAfterSource != tt.source {
				t.Fatalf("retry metadata = (%v, %q), want (%d, %q)", apiErr.RetryAfterSeconds, apiErr.RetryAfterSource, tt.want, tt.source)
			}
		})
	}
}

func TestClassifyHTTPRateLimit_BusinessCodeMustBeExactInteger(t *testing.T) {
	tests := []struct {
		name string
		code any
		want int
	}{
		{name: "exact json number", code: json.Number("99991400"), want: 99991400},
		{name: "exact decimal json number", code: json.Number("99991400.0"), want: 99991400},
		{name: "exact exponent json number", code: json.Number("9.99914e7"), want: 99991400},
		{name: "fractional json number", code: json.Number("99991400.5"), want: 429},
		{name: "overflow json number", code: json.Number("999914000000000000000000000"), want: 429},
		{name: "fractional float", code: float64(99991400.5), want: 429},
		{name: "infinite float", code: math.Inf(1), want: 429},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var apiErr *errs.APIError
			if !errors.As(ClassifyHTTPRateLimit(429, nil, map[string]any{"code": tt.code}, nil, time.Now()), &apiErr) {
				t.Fatal("expected APIError")
			}
			if apiErr.Code != tt.want {
				t.Fatalf("Code = %d, want %d", apiErr.Code, tt.want)
			}
		})
	}
}

func TestParseRateLimitJSONRequiresSingleCompleteValue(t *testing.T) {
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		{name: "single value", body: `{"code":99991400}`, ok: true},
		{name: "trailing whitespace", body: "{\"code\":99991400}\n\t", ok: true},
		{name: "trailing junk", body: `{"code":99991400,"log_id":"forged"}trailing-junk`},
		{name: "concatenated values", body: `{"code":99991400}{"log_id":"forged"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseRateLimitJSON([]byte(tt.body)); (got != nil) != tt.ok {
				t.Fatalf("ParseRateLimitJSON() = %#v, want ok=%v", got, tt.ok)
			}
		})
	}
}

func TestParseRateLimitJSONRejectsLargeTrailingObject(t *testing.T) {
	body := []byte(`{"code":99991400}{"blob":"` + strings.Repeat("界", 1<<20) + `"}`)
	if got := ParseRateLimitJSON(body); got != nil {
		t.Fatalf("ParseRateLimitJSON() = %#v, want nil for large trailing value", got)
	}
}

func TestClassifyHTTPRateLimit_LogIDAllowlistAndPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		result any
		header http.Header
		want   string
	}{
		{name: "top level", result: map[string]any{"log_id": " top.1_2-3 ", "error": map[string]any{"log_id": "nested"}}, header: http.Header{"X-Tt-Logid": []string{"header"}}, want: "top.1_2-3"},
		{name: "nested", result: map[string]any{"log_id": "bad/id", "error": map[string]any{"log_id": "nested"}}, header: http.Header{"X-Tt-Logid": []string{"header"}}, want: "nested"},
		{name: "tt header", result: map[string]any{"log_id": "bad\nvalue"}, header: http.Header{"X-Tt-Logid": []string{"tt-log"}, "X-Request-Id": []string{"request-log"}}, want: "tt-log"},
		{name: "request header", header: http.Header{"X-Tt-Logid": []string{"bad/value"}, "X-Request-Id": []string{"request-log"}}, want: "request-log"},
		{name: "invalid all", result: map[string]any{"log_id": strings.Repeat("a", 129)}, header: http.Header{"X-Tt-Logid": []string{"bad value"}}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var apiErr *errs.APIError
			if !errors.As(ClassifyHTTPRateLimit(429, tt.header, tt.result, nil, time.Now()), &apiErr) {
				t.Fatal("expected APIError")
			}
			if apiErr.LogID != tt.want {
				t.Fatalf("LogID = %q, want %q", apiErr.LogID, tt.want)
			}
		})
	}
}

func TestClassifyHTTPRateLimit_FixedSafeMessageAndBusinessCode(t *testing.T) {
	err := ClassifyHTTPRateLimit(429, nil, map[string]any{
		"code": 99991400,
		"msg":  "secret\x00oauth description",
	}, nil, time.Now())
	var apiErr *errs.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T (%v), want APIError", err, err)
	}
	if apiErr.Code != 99991400 || apiErr.Message != "request rate limit exceeded" || !apiErr.Retryable {
		t.Fatalf("API error = %#v", apiErr)
	}
	if apiErr.Cause != nil || strings.Contains(apiErr.Hint, "secret") {
		t.Fatalf("unsafe payload leaked: %#v", apiErr)
	}
}

func TestClassifyHTTPRateLimit_PreservedClassificationStillSanitizesLogID(t *testing.T) {
	original := errs.NewAPIError(errs.SubtypeRateLimit, "daily quota").WithCode(1063006).WithLogID("unsafe/log")
	got := ClassifyHTTPRateLimit(429, http.Header{"X-Request-Id": []string{"request-id"}}, map[string]any{"code": 1063006}, original, time.Now())
	if got != original || original.Retryable || original.RetryAfterSeconds != nil {
		t.Fatalf("classification changed: %#v", original)
	}
	if original.LogID != "request-id" {
		t.Fatalf("LogID = %q, want sanitized header fallback", original.LogID)
	}
}

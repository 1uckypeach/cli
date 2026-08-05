// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package client

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
)

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		header http.Header
		want   int
		source string
	}{
		{name: "delta seconds", header: http.Header{"Retry-After": []string{"45"}}, want: 45, source: "retry-after"},
		{name: "HTTP date", header: http.Header{"Retry-After": []string{now.Add(45 * time.Second).Format(http.TimeFormat)}}, want: 45, source: "retry-after"},
		{name: "negative", header: http.Header{"Retry-After": []string{"-1"}}, want: 1, source: "default"},
		{name: "non-standard signed delta", header: http.Header{"Retry-After": []string{"+1"}}, want: 1, source: "default"},
		{name: "past HTTP date", header: http.Header{"Retry-After": []string{now.Add(-time.Second).Format(http.TimeFormat)}}, want: 1, source: "default"},
		{name: "integer overflow", header: http.Header{"Retry-After": []string{"999999999999999999999999999999"}}, want: 1, source: "default"},
		{name: "over one day", header: http.Header{"Retry-After": []string{"86401"}}, want: 1, source: "default"},
		{name: "missing", header: http.Header{}, want: 1, source: "default"},
		{name: "ignore x-ogw", header: http.Header{"X-Ogw-Ratelimit-Reset": []string{"60"}}, want: 1, source: "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, source := parseRetryAfter(tt.header, now)
			if got != tt.want || source != tt.source {
				t.Fatalf("parseRetryAfter() = (%d, %q), want (%d, %q)", got, source, tt.want, tt.source)
			}
		})
	}
}

func TestClassifyRateLimitResponse_HTTP429PreservesExplicitLongTermQuota(t *testing.T) {
	original := errs.NewAPIError(errs.SubtypeRateLimit, "daily quota exceeded").
		WithCode(1063006).
		WithHint("this operation is limited to 5 times per day")
	resp := &larkcore.ApiResp{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": []string{"9"}}}
	result := map[string]interface{}{"code": float64(1063006), "msg": "daily quota exceeded"}

	got := ClassifyRateLimitResponse(resp, result, original)
	if !errors.Is(got, original) || got != original {
		t.Fatalf("ClassifyRateLimitResponse() = %T %v, want original error unchanged", got, got)
	}
	if original.Retryable || original.RetryAfterSeconds != nil {
		t.Fatalf("long-term quota was incorrectly decorated as short-term retryable: %#v", original)
	}
}

func TestClassifyRateLimitResponse_MergesExistingHint(t *testing.T) {
	original := errs.NewAPIError(errs.SubtypeRateLimit, "slow").
		WithCode(99991400).
		WithHint("server says slow down")
	resp := &larkcore.ApiResp{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": []string{"9"}}}
	result := map[string]interface{}{"code": float64(99991400), "msg": "slow"}

	got := ClassifyRateLimitResponse(resp, result, original)
	if got != original {
		t.Fatalf("ClassifyRateLimitResponse() did not preserve the classified APIError pointer")
	}
	if !strings.HasPrefix(original.Hint, "server says slow down;") || !strings.Contains(original.Hint, "safe to replay") {
		t.Fatalf("merged hint = %q, want server hint followed by replay guidance", original.Hint)
	}
}

func TestClassifyRateLimitResponse_BackfillsExistingAPIErrorLogID(t *testing.T) {
	tests := []struct {
		name       string
		existingID string
		result     interface{}
		want       string
	}{
		{name: "header fills empty", want: "header-log"},
		{name: "body nested before header", result: map[string]interface{}{"code": float64(99991400), "error": map[string]interface{}{"log_id": "nested-log"}}, want: "nested-log"},
		{name: "structured body has defined precedence", existingID: "existing-log", result: map[string]interface{}{"code": float64(99991400), "log_id": "body-log"}, want: "body-log"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := errs.NewAPIError(errs.SubtypeRateLimit, "slow").WithCode(99991400)
			original.LogID = tt.existingID
			result := tt.result
			if result == nil {
				result = map[string]interface{}{"code": float64(99991400)}
			}
			resp := &larkcore.ApiResp{StatusCode: http.StatusTooManyRequests, Header: http.Header{"X-Tt-Logid": []string{"header-log"}}}
			if got := ClassifyRateLimitResponse(resp, result, original); got != original {
				t.Fatalf("ClassifyRateLimitResponse() did not reuse existing APIError")
			}
			if original.LogID != tt.want {
				t.Fatalf("LogID = %q, want %q", original.LogID, tt.want)
			}
		})
	}
}

func TestClassifyRateLimitResponse_BareHTTP429LogIDPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		result interface{}
		raw    []byte
		want   string
	}{
		{name: "top level before nested and header", result: map[string]interface{}{"log_id": "top", "error": map[string]interface{}{"log_id": "nested"}}, want: "top"},
		{name: "nested before header", result: map[string]interface{}{"error": map[string]interface{}{"log_id": "nested"}}, want: "nested"},
		{name: "raw body before header when result unavailable", raw: []byte(`{"error":{"log_id":"raw-nested"}}`), want: "raw-nested"},
		{name: "header fallback", result: map[string]interface{}{"msg": "slow"}, want: "header"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &larkcore.ApiResp{StatusCode: http.StatusTooManyRequests, Header: http.Header{"X-Tt-Logid": []string{"header"}}, RawBody: tt.raw}
			var apiErr *errs.APIError
			err := ClassifyRateLimitResponse(resp, tt.result, nil)
			if !errors.As(err, &apiErr) {
				t.Fatal("expected APIError")
			}
			problem, ok := errs.ProblemOf(err)
			if !ok || problem.Category != errs.CategoryAPI || problem.Subtype != errs.SubtypeRateLimit {
				t.Fatalf("problem = %#v, want api/rate_limit", problem)
			}
			if apiErr.LogID != tt.want {
				t.Fatalf("LogID = %q, want %q", apiErr.LogID, tt.want)
			}
		})
	}
}

func TestClassifyRateLimitResponse_BusinessCodeUsesCanonicalMessageAcrossHTTPStatuses(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusTooManyRequests} {
		body := []byte(`{"code":99991400,"msg":"status-specific upstream text"}`)
		result := map[string]interface{}{"code": float64(99991400), "msg": "status-specific upstream text"}
		classified := errs.NewAPIError(errs.SubtypeRateLimit, "status-specific upstream text").WithCode(99991400)
		resp := &larkcore.ApiResp{StatusCode: status, Header: http.Header{}, RawBody: body}

		var apiErr *errs.APIError
		if !errors.As(ClassifyRateLimitResponse(resp, result, classified), &apiErr) {
			t.Fatalf("status %d: expected APIError", status)
		}
		if apiErr.Message != "request rate limit exceeded" {
			t.Fatalf("status %d: message = %q, want canonical rate-limit message", status, apiErr.Message)
		}
	}
}

func TestClassifyRateLimitResponse_MalformedBusinessCandidateFailsClosed(t *testing.T) {
	projected := map[string]interface{}{"code": float64(99991400)}
	classified := errs.NewAPIError(errs.SubtypeRateLimit, "projected").WithCode(99991400)
	resp := &larkcore.ApiResp{StatusCode: http.StatusBadRequest, RawBody: []byte(`{"code":99991400}trailing`)}

	err := ClassifyRateLimitResponse(resp, projected, classified)
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != errs.CategoryInternal || problem.Subtype != errs.SubtypeInvalidResponse {
		t.Fatalf("problem = %#v, want internal/invalid_response", problem)
	}
}

func TestParseRateLimitResultRejectsTrailingContent(t *testing.T) {
	for _, body := range []string{
		`{"code":99991400,"log_id":"forged"}trailing-junk`,
		`{"code":99991400,"log_id":"forged"}{"code":99991400}`,
	} {
		if got := parseRateLimitResult([]byte(body)); got != nil {
			t.Fatalf("parseRateLimitResult(%q) = %#v, want nil", body, got)
		}
	}
}

func TestClassifyRateLimitResponse_MalformedRawBodyRejectsCallerProjection(t *testing.T) {
	projected := map[string]interface{}{"code": float64(99991400), "log_id": "projected-log"}
	classified := errs.NewAPIError(errs.SubtypeRateLimit, "projected").WithCode(99991400).WithLogID("projected-log")
	resp := &larkcore.ApiResp{
		StatusCode: http.StatusTooManyRequests,
		RawBody:    []byte(`{"code":99991400,"log_id":"forged"}trailing`),
		Header:     http.Header{"X-Request-Id": []string{"header-log"}},
	}
	var apiErr *errs.APIError
	if !errors.As(ClassifyRateLimitResponse(resp, projected, classified), &apiErr) {
		t.Fatal("expected APIError")
	}
	if apiErr.Code != 429 || apiErr.LogID != "header-log" || apiErr == classified {
		t.Fatalf("malformed raw body revived projection: %#v", apiErr)
	}
}

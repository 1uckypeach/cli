// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
)

func TestClassifyAPIResponse_DecodesOnceAndReturnsBusinessErrorWithResult(t *testing.T) {
	resp := &larkcore.ApiResp{
		StatusCode: http.StatusOK,
		RawBody:    []byte(`{"code":123,"msg":"failed","data":{"token":"keep"}}`),
	}
	wantErr := errs.NewAPIError(errs.SubtypeUnknown, "failed").WithCode(123)
	checks := 0

	result, err := ClassifyAPIResponse(resp, func(result interface{}) error {
		checks++
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("ClassifyAPIResponse() error = %T (%v), want original business error", err, err)
	}
	if checks != 1 {
		t.Fatalf("business checker calls = %d, want 1", checks)
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok || resultMap["data"] == nil {
		t.Fatalf("ClassifyAPIResponse() result = %#v, want decoded failure envelope", result)
	}
}

func TestClassifyAPIResponse_BareHTTP429PrecedesJSONParseFailure(t *testing.T) {
	resp := &larkcore.ApiResp{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"7"}},
		RawBody:    []byte("rate limit exceeded"),
	}

	_, err := ClassifyAPIResponse(resp, nil)
	problem, ok := errs.ProblemOf(err)
	var apiErr *errs.APIError
	if !ok || !errors.As(err, &apiErr) || problem.Category != errs.CategoryAPI ||
		problem.Subtype != errs.SubtypeRateLimit || !problem.Retryable ||
		apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != 7 {
		t.Fatalf("problem = %#v, want retryable api/rate_limit with retry_after_seconds=7", problem)
	}
}

func TestClassifyAPIResponseError_HTTP429PreservesExplicitLongTermQuota(t *testing.T) {
	original := errs.NewAPIError(errs.SubtypeRateLimit, "daily quota exceeded").
		WithCode(1063006).
		WithHint("this operation is limited to 5 times per day")
	resp := &larkcore.ApiResp{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": []string{"9"}}}
	result := map[string]interface{}{"code": float64(1063006), "msg": "daily quota exceeded"}

	got := classifyAPIResponseError(resp, result, original)
	if !errors.Is(got, original) || got != original {
		t.Fatalf("classifyAPIResponseError() = %T %v, want original error unchanged", got, got)
	}
	if original.Retryable || original.RetryAfterSeconds != nil {
		t.Fatalf("long-term quota was incorrectly decorated as short-term retryable: %#v", original)
	}
}

func TestClassifyAPIResponseError_MergesExistingHint(t *testing.T) {
	original := errs.NewAPIError(errs.SubtypeRateLimit, "slow").
		WithCode(99991400).
		WithHint("server says slow down")
	resp := &larkcore.ApiResp{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": []string{"9"}}}
	result := map[string]interface{}{"code": float64(99991400), "msg": "slow"}

	got := classifyAPIResponseError(resp, result, original)
	if got != original {
		t.Fatalf("classifyAPIResponseError() did not preserve the classified APIError pointer")
	}
	if !strings.HasPrefix(original.Hint, "server says slow down;") || !strings.Contains(original.Hint, "safe to replay") {
		t.Fatalf("merged hint = %q, want server hint followed by replay guidance", original.Hint)
	}
}

func TestClassifyAPIResponseError_BackfillsExistingAPIErrorLogID(t *testing.T) {
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
			if got := classifyAPIResponseError(resp, result, original); got != original {
				t.Fatalf("classifyAPIResponseError() did not reuse existing APIError")
			}
			if original.LogID != tt.want {
				t.Fatalf("LogID = %q, want %q", original.LogID, tt.want)
			}
		})
	}
}

func TestClassifyAPIResponseError_BareHTTP429LogIDPrecedence(t *testing.T) {
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
			err := classifyAPIResponseError(resp, tt.result, nil)
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

func TestClassifyAPIResponseError_BusinessCodeUsesCanonicalMessageAcrossHTTPStatuses(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusTooManyRequests} {
		body := []byte(`{"code":99991400,"msg":"status-specific upstream text"}`)
		result := map[string]interface{}{"code": float64(99991400), "msg": "status-specific upstream text"}
		classified := errs.NewAPIError(errs.SubtypeRateLimit, "status-specific upstream text").WithCode(99991400)
		resp := &larkcore.ApiResp{StatusCode: status, Header: http.Header{}, RawBody: body}

		var apiErr *errs.APIError
		if !errors.As(classifyAPIResponseError(resp, result, classified), &apiErr) {
			t.Fatalf("status %d: expected APIError", status)
		}
		if apiErr.Message != "request rate limit exceeded" {
			t.Fatalf("status %d: message = %q, want canonical rate-limit message", status, apiErr.Message)
		}
	}
}

func TestClassifyAPIResponseError_MalformedBusinessCandidateFailsClosed(t *testing.T) {
	projected := map[string]interface{}{"code": float64(99991400)}
	classified := errs.NewAPIError(errs.SubtypeRateLimit, "projected").WithCode(99991400)
	resp := &larkcore.ApiResp{StatusCode: http.StatusBadRequest, RawBody: []byte(`{"code":99991400,]`)}

	err := classifyAPIResponseError(resp, projected, classified)
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != errs.CategoryInternal || problem.Subtype != errs.SubtypeInvalidResponse {
		t.Fatalf("problem = %#v, want internal/invalid_response", problem)
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("error chain does not preserve JSON syntax error: %v", err)
	}
}

func TestClassifyAPIResponseError_TrailingBusinessCandidateFailsClosed(t *testing.T) {
	projected := map[string]interface{}{"code": float64(99991400)}
	classified := errs.NewAPIError(errs.SubtypeRateLimit, "projected").WithCode(99991400)
	resp := &larkcore.ApiResp{StatusCode: http.StatusBadRequest, RawBody: []byte(`{"code":99991400}trailing`)}

	err := classifyAPIResponseError(resp, projected, classified)
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != errs.CategoryInternal || problem.Subtype != errs.SubtypeInvalidResponse {
		t.Fatalf("problem = %#v, want internal/invalid_response", problem)
	}
	if errors.Unwrap(err) == nil {
		t.Fatalf("error chain does not preserve trailing-content cause: %v", err)
	}
}

func TestClassifyAPIResponseError_MalformedRawBodyRejectsCallerProjection(t *testing.T) {
	projected := map[string]interface{}{"code": float64(99991400), "log_id": "projected-log"}
	classified := errs.NewAPIError(errs.SubtypeRateLimit, "projected").WithCode(99991400).WithLogID("projected-log")
	resp := &larkcore.ApiResp{
		StatusCode: http.StatusTooManyRequests,
		RawBody:    []byte(`{"code":99991400,"log_id":"forged"}trailing`),
		Header:     http.Header{"X-Request-Id": []string{"header-log"}},
	}
	var apiErr *errs.APIError
	if !errors.As(classifyAPIResponseError(resp, projected, classified), &apiErr) {
		t.Fatal("expected APIError")
	}
	if apiErr.Code != 429 || apiErr.LogID != "header-log" || apiErr == classified {
		t.Fatalf("malformed raw body revived projection: %#v", apiErr)
	}
}

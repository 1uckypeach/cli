// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package errs_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/larksuite/cli/errs"
)

func TestPaginationError_PreservesTypedErrorAndAddsProgress(t *testing.T) {
	sentinel := errors.New("permission cause")
	inner := errs.NewPermissionError(errs.SubtypeMissingScope, "missing scope").
		WithCode(99991679).
		WithHint("authorize the scope").
		WithLogID("log-1").
		WithMissingScopes("drive:drive:readonly").
		WithIdentity("user").
		WithConsoleURL("https://open.feishu.cn/app/cli_test").
		WithCause(sentinel)
	wrapped := errs.NewPaginationError(inner, 3, "resume-page-4")

	if !errors.Is(wrapped, sentinel) {
		t.Fatal("errors.Is did not preserve the inner cause chain")
	}
	var permissionErr *errs.PermissionError
	if !errors.As(wrapped, &permissionErr) || permissionErr != inner {
		t.Fatalf("errors.As = %#v, want original PermissionError", permissionErr)
	}
	problem, ok := errs.ProblemOf(wrapped)
	if !ok || *problem != inner.Problem {
		t.Fatalf("ProblemOf = %#v, %v; want preserved Problem", problem, ok)
	}
	if !errs.IsPagination(wrapped) {
		t.Fatal("IsPagination = false, want true")
	}

	encoded, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	want := map[string]any{
		"type":                "authorization",
		"subtype":             "missing_scope",
		"code":                float64(99991679),
		"message":             "missing scope",
		"hint":                "authorize the scope",
		"log_id":              "log-1",
		"identity":            "user",
		"console_url":         "https://open.feishu.cn/app/cli_test",
		"completed_pages":     float64(3),
		"next_page_token":     "resume-page-4",
		"missing_scopes_size": 1,
	}
	for field, wantValue := range want {
		if field == "missing_scopes_size" {
			scopes, ok := got["missing_scopes"].([]any)
			if !ok || len(scopes) != 1 || scopes[0] != "drive:drive:readonly" {
				t.Errorf("missing_scopes = %#v", got["missing_scopes"])
			}
			continue
		}
		if got[field] != wantValue {
			t.Errorf("field %q = %#v, want %#v", field, got[field], wantValue)
		}
	}
}

func TestPaginationError_PreservesRateLimitExtensions(t *testing.T) {
	inner := errs.NewAPIError(errs.SubtypeRateLimit, "slow down").
		WithCode(99991400).
		WithRetryable().
		WithRetryAfter(20, "retry-after")
	wrapped := errs.NewPaginationError(inner, 1, "next")

	encoded, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got["retryable"] != true || got["retry_after_seconds"] != float64(20) || got["retry_after_source"] != "retry-after" {
		t.Fatalf("rate-limit extensions lost: %#v", got)
	}
}

func TestPaginationError_ZeroProgressAndOmittedToken(t *testing.T) {
	inner := errs.NewNetworkError(errs.SubtypeNetworkTransport, "offline")
	wrapped := errs.NewPaginationError(inner, 0, "")

	encoded, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got["completed_pages"] != float64(0) {
		t.Fatalf("completed_pages = %#v, want 0", got["completed_pages"])
	}
	if _, ok := got["next_page_token"]; ok {
		t.Fatalf("next_page_token unexpectedly present: %#v", got)
	}
}

type collidingPaginationError struct {
	problem errs.Problem
}

func (e *collidingPaginationError) Error() string                { return e.problem.Message }
func (e *collidingPaginationError) ProblemDetail() *errs.Problem { return &e.problem }
func (e *collidingPaginationError) MarshalJSON() ([]byte, error) {
	return []byte(`{"type":"api","subtype":"unknown","message":"collision","completed_pages":99}`), nil
}

func TestPaginationError_FieldCollisionFailsClosed(t *testing.T) {
	inner := &collidingPaginationError{problem: errs.Problem{
		Category: errs.CategoryAPI,
		Subtype:  errs.SubtypeUnknown,
		Message:  "collision",
	}}
	wrapped := errs.NewPaginationError(inner, 1, "next")
	problem, ok := errs.ProblemOf(wrapped)
	if !ok || problem.Category != errs.CategoryInternal || problem.Subtype != errs.SubtypeInvalidResponse {
		t.Fatalf("ProblemOf = %#v, %v; want internal/invalid_response collision error", problem, ok)
	}
	if !errors.Is(wrapped, inner) {
		t.Fatal("collision fallback did not preserve the original typed cause")
	}
	var recovered *collidingPaginationError
	if !errors.As(wrapped, &recovered) || recovered != inner {
		t.Fatalf("errors.As = %#v, want original colliding error", recovered)
	}
	encoded, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatalf("collision fallback must remain JSON-serializable: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got["type"] != "internal" || got["subtype"] != "invalid_response" {
		t.Fatalf("collision fallback JSON = %#v, want internal/invalid_response", got)
	}
	if _, exists := got["completed_pages"]; exists {
		t.Fatalf("collision fallback must not overwrite reserved fields: %#v", got)
	}
}

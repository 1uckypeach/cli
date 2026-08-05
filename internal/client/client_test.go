// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
	internalauth "github.com/larksuite/cli/internal/auth"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/credential"
	"github.com/larksuite/cli/internal/output"
)

// roundTripFunc is an adapter to use a function as http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// jsonResponse creates an HTTP response with JSON body.
func jsonResponse(body interface{}) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(b)),
	}
}

// staticTokenResolver always returns a fixed token without any HTTP calls.
type staticTokenResolver struct{}

func (s *staticTokenResolver) ResolveToken(_ context.Context, _ credential.TokenSpec) (*credential.TokenResult, error) {
	return &credential.TokenResult{Token: "test-token"}, nil
}

// newTestAPIClient creates an APIClient with a mock HTTP transport.
func newTestAPIClient(t *testing.T, rt http.RoundTripper) (*APIClient, *bytes.Buffer) {
	t.Helper()
	errBuf := &bytes.Buffer{}
	httpClient := &http.Client{Transport: rt}
	sdk := lark.NewClient("test-app", "test-secret",
		lark.WithEnableTokenCache(false),
		lark.WithLogLevel(larkcore.LogLevelError),
		lark.WithHttpClient(httpClient),
	)
	testCred := credential.NewCredentialProvider(nil, nil, &staticTokenResolver{}, nil)
	cfg := &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu}
	return &APIClient{
		SDK:        sdk,
		ErrOut:     errBuf,
		Credential: testCred,
		Config:     cfg,
	}, errBuf
}

func TestIsJSONContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"text/json", true},
		{"application/octet-stream", false},
		{"image/png", false},
		{"text/html", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsJSONContentType(tt.ct); got != tt.want {
			t.Errorf("IsJSONContentType(%q) = %v, want %v", tt.ct, got, tt.want)
		}
	}
}

func TestMimeToExt(t *testing.T) {
	tests := []struct {
		ct   string
		want string
	}{
		{"image/png", ".png"},
		{"image/jpeg", ".jpg"},
		{"application/pdf", ".pdf"},
		{"text/plain", ".txt"},
		{"application/octet-stream", ".bin"},
		{"", ".bin"},
	}
	for _, tt := range tests {
		if got := mimeToExt(tt.ct); got != tt.want {
			t.Errorf("mimeToExt(%q) = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestStreamPages_NonBatchAPI_NoArrayField(t *testing.T) {
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(map[string]interface{}{
			"code": 0, "msg": "ok",
			"data": map[string]interface{}{
				"user_id": "u123",
				"name":    "Test User",
			},
		}), nil
	})

	ac, errBuf := newTestAPIClient(t, rt)

	result, hasItems, err := ac.StreamPages(context.Background(), RawApiRequest{
		Method: "GET",
		URL:    "/open-apis/contact/v3/users/u123",
		As:     "bot",
	}, func(items []interface{}) error {
		t.Error("onItems should not be called for non-batch API")
		return nil
	}, PaginationOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasItems {
		t.Error("expected hasItems=false for non-batch API")
	}
	if strings.Contains(errBuf.String(), "[pagination] streamed") {
		t.Error("expected no pagination summary log for non-batch API")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be a map")
	}
	data, _ := resultMap["data"].(map[string]interface{})
	if data["user_id"] != "u123" {
		t.Errorf("expected user_id=u123, got %v", data["user_id"])
	}
}

func TestStreamPages_BatchAPI_WithArrayField(t *testing.T) {
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(map[string]interface{}{
			"code": 0, "msg": "ok",
			"data": map[string]interface{}{
				"items":    []interface{}{map[string]interface{}{"id": "1"}, map[string]interface{}{"id": "2"}},
				"has_more": false,
			},
		}), nil
	})

	ac, errBuf := newTestAPIClient(t, rt)

	var streamedItems []interface{}
	result, hasItems, err := ac.StreamPages(context.Background(), RawApiRequest{
		Method: "GET",
		URL:    "/open-apis/contact/v3/users",
		As:     "bot",
	}, func(items []interface{}) error {
		streamedItems = append(streamedItems, items...)
		return nil
	}, PaginationOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasItems {
		t.Error("expected hasItems=true for batch API")
	}
	if len(streamedItems) != 2 {
		t.Errorf("expected 2 streamed items, got %d", len(streamedItems))
	}
	if !strings.Contains(errBuf.String(), "[pagination] streamed") {
		t.Error("expected pagination summary log for batch API")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestStreamPages_OnItemsErrorStopsPagination(t *testing.T) {
	apiCalls := 0
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		apiCalls++
		if apiCalls == 1 {
			return jsonResponse(map[string]interface{}{
				"code": 0, "msg": "ok",
				"data": map[string]interface{}{
					"items":      []interface{}{map[string]interface{}{"id": "1"}},
					"has_more":   true,
					"page_token": "next",
				},
			}), nil
		}
		return jsonResponse(map[string]interface{}{
			"code": 0, "msg": "ok",
			"data": map[string]interface{}{
				"items":    []interface{}{map[string]interface{}{"id": "2"}},
				"has_more": false,
			},
		}), nil
	})

	ac, _ := newTestAPIClient(t, rt)
	sentinel := errors.New("stop streaming")
	var streamedItems []interface{}
	result, hasItems, err := ac.StreamPages(context.Background(), RawApiRequest{
		Method: "GET",
		URL:    "/open-apis/contact/v3/users",
		As:     "bot",
	}, func(items []interface{}) error {
		streamedItems = append(streamedItems, items...)
		return sentinel
	}, PaginationOptions{PageDelay: 0})

	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
	var paginationErr *errs.PaginationError
	if !errors.As(err, &paginationErr) {
		t.Fatalf("err = %T (%v), want PaginationError", err, err)
	}
	if paginationErr.CompletedPages != 0 || paginationErr.NextPageToken != "" {
		t.Fatalf("progress = %d/%q, want 0/empty for failed first-page callback", paginationErr.CompletedPages, paginationErr.NextPageToken)
	}
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != errs.CategoryInternal || problem.Subtype != errs.SubtypeUnknown {
		t.Fatalf("ProblemOf = %#v, %v; want internal/unknown", problem, ok)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil when callback stops pagination", result)
	}
	if hasItems {
		t.Fatal("hasItems = true, want false when callback stops before returning")
	}
	if apiCalls != 1 {
		t.Fatalf("apiCalls = %d, want early stop after first page", apiCalls)
	}
	if len(streamedItems) != 1 {
		t.Fatalf("streamedItems = %d, want first page only", len(streamedItems))
	}
}

func TestPaginateLoop_TypedCallbackErrorPreservesTypeAndCurrentCursor(t *testing.T) {
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(map[string]interface{}{
			"code": 0, "msg": "ok",
			"data": map[string]interface{}{
				"items":      []interface{}{map[string]interface{}{"id": "current"}},
				"has_more":   true,
				"page_token": "future-page",
			},
		}), nil
	})
	ac, _ := newTestAPIClient(t, rt)
	inner := errs.NewValidationError(errs.SubtypeFailedPrecondition, "typed callback stopped")

	results, err := ac.paginateLoop(context.Background(), RawApiRequest{
		Method: http.MethodGet,
		URL:    "/open-apis/test",
		Params: map[string]interface{}{"page_token": "current-page"},
		As:     core.AsBot,
	}, PaginationOptions{PageDelay: -1}, func(interface{}) error {
		return inner
	})
	if len(results) != 0 {
		t.Fatalf("results = %d pages, want callback-failed page excluded", len(results))
	}
	var paginationErr *errs.PaginationError
	if !errors.As(err, &paginationErr) {
		t.Fatalf("err = %T (%v), want PaginationError", err, err)
	}
	if paginationErr.CompletedPages != 0 || paginationErr.NextPageToken != "current-page" {
		t.Fatalf("progress = %d/%q, want 0/current-page", paginationErr.CompletedPages, paginationErr.NextPageToken)
	}
	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) || validationErr != inner {
		t.Fatalf("errors.As = %#v, want original typed callback error", validationErr)
	}
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != errs.CategoryValidation || problem.Subtype != errs.SubtypeFailedPrecondition {
		t.Fatalf("ProblemOf = %#v, %v; want validation/failed_precondition", problem, ok)
	}
}

func TestPaginateAll_PageLimitStopsPagination(t *testing.T) {
	apiCalls := 0
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		apiCalls++
		return jsonResponse(map[string]interface{}{
			"code": 0, "msg": "ok",
			"data": map[string]interface{}{
				"items":      []interface{}{map[string]interface{}{"id": apiCalls}},
				"has_more":   true,
				"page_token": "next",
			},
		}), nil
	})

	ac, errBuf := newTestAPIClient(t, rt)

	result, err := ac.PaginateAll(context.Background(), RawApiRequest{
		Method: "GET",
		URL:    "/open-apis/test",
		As:     "bot",
	}, PaginationOptions{PageLimit: 2, PageDelay: 0})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiCalls != 2 {
		t.Errorf("expected 2 API calls with PageLimit=2, got %d", apiCalls)
	}
	if !strings.Contains(errBuf.String(), "reached page limit (2), stopping. Use --page-all --page-limit 0 to fetch all pages.") {
		t.Errorf("expected page limit log, got: %s", errBuf.String())
	}

	// Truncation must surface in the merged output: has_more stays true so
	// callers can detect loss. page_token is intentionally dropped from the
	// aggregate view — to fetch more, re-run with a larger --page-limit.
	resultMap, _ := result.(map[string]interface{})
	data, _ := resultMap["data"].(map[string]interface{})
	if hasMore, _ := data["has_more"].(bool); !hasMore {
		t.Errorf("expected has_more=true when page limit truncates, got false")
	}
	if _, exists := data["page_token"]; exists {
		t.Errorf("expected page_token to be dropped from merged output, got %v", data["page_token"])
	}
}

func TestPaginateAll_NaturalEndClearsPageToken(t *testing.T) {
	apiCalls := 0
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		apiCalls++
		hasMore := apiCalls < 2
		body := map[string]interface{}{
			"code": 0, "msg": "ok",
			"data": map[string]interface{}{
				"items":    []interface{}{map[string]interface{}{"id": apiCalls}},
				"has_more": hasMore,
			},
		}
		if hasMore {
			body["data"].(map[string]interface{})["page_token"] = "next"
		}
		return jsonResponse(body), nil
	})

	ac, _ := newTestAPIClient(t, rt)

	result, err := ac.PaginateAll(context.Background(), RawApiRequest{
		Method: "GET",
		URL:    "/open-apis/test",
		As:     "bot",
	}, PaginationOptions{PageLimit: 10, PageDelay: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap, _ := result.(map[string]interface{})
	data, _ := resultMap["data"].(map[string]interface{})
	if hasMore, _ := data["has_more"].(bool); hasMore {
		t.Errorf("expected has_more=false at natural end, got true")
	}
	if _, exists := data["page_token"]; exists {
		t.Errorf("expected page_token absent at natural end, got %v", data["page_token"])
	}
}

func TestPaginateAll_SecondPageFailuresReturnProgress(t *testing.T) {
	tests := []struct {
		name       string
		secondPage func() (*http.Response, error)
		category   errs.Category
		subtype    errs.Subtype
	}{
		{
			name: "transport",
			secondPage: func() (*http.Response, error) {
				return nil, &net.DNSError{Err: "no such host", Name: "example.invalid"}
			},
			category: errs.CategoryNetwork,
			subtype:  errs.SubtypeNetworkDNS,
		},
		{
			name: "malformed JSON",
			secondPage: func() (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"code":`)),
				}, nil
			},
			category: errs.CategoryInternal,
			subtype:  errs.SubtypeInvalidResponse,
		},
		{
			name: "business error",
			secondPage: func() (*http.Response, error) {
				return jsonResponse(map[string]interface{}{"code": 230027, "msg": "user not authorized"}), nil
			},
			category: errs.CategoryAuthorization,
			subtype:  errs.SubtypeUserUnauthorized,
		},
		{
			name: "HTTP error",
			secondPage: func() (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Header:     http.Header{"Content-Type": []string{"text/plain"}},
					Body:       io.NopCloser(strings.NewReader("gateway unavailable")),
				}, nil
			},
			category: errs.CategoryNetwork,
			subtype:  errs.SubtypeNetworkServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return jsonResponse(map[string]interface{}{
						"code": 0, "msg": "ok",
						"data": map[string]interface{}{
							"items":      []interface{}{map[string]interface{}{"id": "first"}},
							"has_more":   true,
							"page_token": "resume-secret",
						},
					}), nil
				}
				return tt.secondPage()
			})
			ac, errBuf := newTestAPIClient(t, rt)

			result, err := ac.PaginateAll(context.Background(), RawApiRequest{
				Method: http.MethodGet,
				URL:    "/open-apis/test",
				As:     core.AsUser,
			}, PaginationOptions{PageDelay: -1})
			if err == nil {
				t.Fatal("PaginateAll() error = nil")
			}
			if result != nil {
				t.Fatalf("PaginateAll() result = %#v, want nil on partial failure", result)
			}
			var paginationErr *errs.PaginationError
			if !errors.As(err, &paginationErr) {
				t.Fatalf("error = %T (%v), want *errs.PaginationError", err, err)
			}
			if paginationErr.CompletedPages != 1 || paginationErr.NextPageToken != "resume-secret" {
				t.Fatalf("progress = %d/%q, want 1/resume-secret", paginationErr.CompletedPages, paginationErr.NextPageToken)
			}
			problem, ok := errs.ProblemOf(err)
			if !ok || problem.Category != tt.category || problem.Subtype != tt.subtype {
				t.Fatalf("ProblemOf = %#v, %v; want %s/%s", problem, ok, tt.category, tt.subtype)
			}
			if strings.Contains(errBuf.String(), "resume-secret") || strings.Contains(errBuf.String(), "error, stopping pagination") {
				t.Fatalf("stderr leaked cursor or temporary error log: %q", errBuf.String())
			}
		})
	}
}

func TestPaginateAll_FirstPageFailureReportsStartingCursor(t *testing.T) {
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(map[string]interface{}{"code": 230027, "msg": "user not authorized"}), nil
	})
	ac, errBuf := newTestAPIClient(t, rt)

	result, err := ac.PaginateAll(context.Background(), RawApiRequest{
		Method: http.MethodGet,
		URL:    "/open-apis/test",
		Params: map[string]interface{}{"page_token": "starting-secret"},
		As:     core.AsUser,
	}, PaginationOptions{PageDelay: -1})
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
	var paginationErr *errs.PaginationError
	if !errors.As(err, &paginationErr) {
		t.Fatalf("error = %T (%v), want *errs.PaginationError", err, err)
	}
	if paginationErr.CompletedPages != 0 || paginationErr.NextPageToken != "starting-secret" {
		t.Fatalf("progress = %d/%q, want 0/starting-secret", paginationErr.CompletedPages, paginationErr.NextPageToken)
	}
	if strings.Contains(errBuf.String(), "starting-secret") {
		t.Fatalf("stderr leaked starting cursor: %q", errBuf.String())
	}
}

func TestPaginateAll_BusinessErrorClassificationIdentityPrecedence(t *testing.T) {
	tests := []struct {
		name            string
		requestIdentity core.Identity
		optionIdentity  core.Identity
		wantIdentity    string
	}{
		{name: "options override request", requestIdentity: core.AsBot, optionIdentity: core.AsUser, wantIdentity: "user"},
		{name: "request fallback", requestIdentity: core.AsBot, wantIdentity: "bot"},
		{name: "default fallback", wantIdentity: "user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(map[string]interface{}{
					"code": 99991679,
					"msg":  "missing scope",
				}), nil
			})
			ac, _ := newTestAPIClient(t, rt)

			result, err := ac.PaginateAll(context.Background(), RawApiRequest{
				Method: http.MethodGet,
				URL:    "/open-apis/test",
				As:     tt.requestIdentity,
			}, PaginationOptions{Identity: tt.optionIdentity, PageDelay: -1})
			if result != nil {
				t.Fatalf("result = %#v, want nil", result)
			}
			var paginationErr *errs.PaginationError
			if !errors.As(err, &paginationErr) {
				t.Fatalf("error = %T (%v), want PaginationError", err, err)
			}
			if paginationErr.CompletedPages != 0 || paginationErr.NextPageToken != "" {
				t.Fatalf("progress = %d/%q, want 0/empty", paginationErr.CompletedPages, paginationErr.NextPageToken)
			}
			var permissionErr *errs.PermissionError
			if !errors.As(err, &permissionErr) {
				t.Fatalf("error = %T (%v), want PermissionError in wrapper", err, err)
			}
			if permissionErr.Identity != tt.wantIdentity {
				t.Errorf("PermissionError.Identity = %q, want %q", permissionErr.Identity, tt.wantIdentity)
			}
			if tt.wantIdentity == "bot" {
				if !strings.Contains(permissionErr.Hint, "developer console") {
					t.Errorf("PermissionError.Hint = %q, want bot scope-application guidance", permissionErr.Hint)
				}
			} else if !strings.Contains(permissionErr.Hint, "lark-cli auth login") {
				t.Errorf("PermissionError.Hint = %q, want user authorization recovery guidance", permissionErr.Hint)
			}
			problem, ok := errs.ProblemOf(err)
			if !ok || problem.Category != errs.CategoryAuthorization || problem.Subtype != errs.SubtypeMissingScope {
				t.Fatalf("ProblemOf = %#v, %v; want authorization/missing_scope", problem, ok)
			}
		})
	}
}

func TestPaginateLoop_ThirdPageFailureExcludesFailedPage(t *testing.T) {
	calls := 0
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 3 {
			return jsonResponse(map[string]interface{}{"code": 230027, "msg": "user not authorized"}), nil
		}
		return jsonResponse(map[string]interface{}{
			"code": 0, "msg": "ok",
			"data": map[string]interface{}{
				"items":      []interface{}{map[string]interface{}{"page": calls}},
				"has_more":   true,
				"page_token": map[int]string{1: "page-2", 2: "page-3"}[calls],
			},
		}), nil
	})
	ac, _ := newTestAPIClient(t, rt)

	results, err := ac.paginateLoop(context.Background(), RawApiRequest{
		Method: http.MethodGet,
		URL:    "/open-apis/test",
		As:     core.AsUser,
	}, PaginationOptions{PageDelay: -1}, nil)
	if len(results) != 2 {
		t.Fatalf("accumulated pages = %d, want only two successful pages", len(results))
	}
	var paginationErr *errs.PaginationError
	if !errors.As(err, &paginationErr) {
		t.Fatalf("error = %T (%v), want *errs.PaginationError", err, err)
	}
	if paginationErr.CompletedPages != 2 || paginationErr.NextPageToken != "page-3" {
		t.Fatalf("progress = %d/%q, want 2/page-3", paginationErr.CompletedPages, paginationErr.NextPageToken)
	}
}

func TestStreamPages_PreservesWrittenPagesBeforeFailure(t *testing.T) {
	calls := 0
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return jsonResponse(map[string]interface{}{
				"code": 0, "msg": "ok",
				"data": map[string]interface{}{
					"items":      []interface{}{map[string]interface{}{"id": "written"}},
					"has_more":   true,
					"page_token": "second-page",
				},
			}), nil
		}
		return jsonResponse(map[string]interface{}{"code": 230027, "msg": "user not authorized"}), nil
	})
	ac, _ := newTestAPIClient(t, rt)
	var written []interface{}

	result, hasItems, err := ac.StreamPages(context.Background(), RawApiRequest{
		Method: http.MethodGet,
		URL:    "/open-apis/test",
		As:     core.AsUser,
	}, func(items []interface{}) error {
		written = append(written, items...)
		return nil
	}, PaginationOptions{PageDelay: -1})
	if err == nil {
		t.Fatal("StreamPages() error = nil")
	}
	if result != nil || hasItems {
		t.Fatalf("result/hasItems = %#v/%v, want nil/false on final error", result, hasItems)
	}
	if len(written) != 1 {
		t.Fatalf("written items = %d, want successful first page preserved", len(written))
	}
	var paginationErr *errs.PaginationError
	if !errors.As(err, &paginationErr) || paginationErr.CompletedPages != 1 {
		t.Fatalf("error progress = %#v, want one completed page", paginationErr)
	}
}

func TestPaginateAll_PageDelayHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		cancel()
		return jsonResponse(map[string]interface{}{
			"code": 0, "msg": "ok",
			"data": map[string]interface{}{
				"items":      []interface{}{},
				"has_more":   true,
				"page_token": "not-requested",
			},
		}), nil
	})
	ac, _ := newTestAPIClient(t, rt)

	started := time.Now()
	_, err := ac.PaginateAll(ctx, RawApiRequest{
		Method: http.MethodGet,
		URL:    "/open-apis/test",
		As:     core.AsBot,
	}, PaginationOptions{PageDelay: 5_000})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	var paginationErr *errs.PaginationError
	if !errors.As(err, &paginationErr) {
		t.Fatalf("error = %T (%v), want PaginationError", err, err)
	}
	if paginationErr.CompletedPages != 1 || paginationErr.NextPageToken != "not-requested" {
		t.Fatalf("progress = %d/%q, want 1/not-requested", paginationErr.CompletedPages, paginationErr.NextPageToken)
	}
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != errs.CategoryInternal || problem.Subtype != errs.SubtypeUnknown {
		t.Fatalf("ProblemOf = %#v, %v; want internal/unknown", problem, ok)
	}
	encoded, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("json.Marshal: %v", marshalErr)
	}
	var wire map[string]any
	if unmarshalErr := json.Unmarshal(encoded, &wire); unmarshalErr != nil {
		t.Fatalf("json.Unmarshal: %v", unmarshalErr)
	}
	if wire["completed_pages"] != float64(1) || wire["next_page_token"] != "not-requested" {
		t.Fatalf("wire progress = %#v", wire)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cancellation took %s, want prompt return", elapsed)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want no request after cancellation", calls)
	}
}

func TestBuildApiReq_QueryParams(t *testing.T) {
	ac := &APIClient{}

	tests := []struct {
		name   string
		params map[string]interface{}
		want   larkcore.QueryParams
	}{
		{
			name:   "scalar values",
			params: map[string]interface{}{"page_size": 20, "user_id_type": "open_id"},
			want: larkcore.QueryParams{
				"page_size":    []string{"20"},
				"user_id_type": []string{"open_id"},
			},
		},
		{
			name:   "[]interface{} array",
			params: map[string]interface{}{"department_ids": []interface{}{"d1", "d2", "d3"}},
			want: larkcore.QueryParams{
				"department_ids": []string{"d1", "d2", "d3"},
			},
		},
		{
			name:   "[]string array",
			params: map[string]interface{}{"statuses": []string{"active", "inactive"}},
			want: larkcore.QueryParams{
				"statuses": []string{"active", "inactive"},
			},
		},
		{
			name: "mixed scalar and array",
			params: map[string]interface{}{
				"user_id_type": "open_id",
				"ids":          []interface{}{"id1", "id2"},
			},
			want: larkcore.QueryParams{
				"user_id_type": []string{"open_id"},
				"ids":          []string{"id1", "id2"},
			},
		},
		{
			name:   "empty array",
			params: map[string]interface{}{"tags": []interface{}{}},
			want:   larkcore.QueryParams{},
		},
		{
			name:   "nil params",
			params: nil,
			want:   larkcore.QueryParams{},
		},
		{
			name:   "bool value",
			params: map[string]interface{}{"with_bot": true},
			want:   larkcore.QueryParams{"with_bot": []string{"true"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiReq, _ := ac.buildApiReq(RawApiRequest{
				Method: "GET",
				URL:    "/open-apis/test",
				Params: tt.params,
			})
			got := apiReq.QueryParams
			// Check all expected keys exist with correct values
			for k, wantVals := range tt.want {
				gotVals, ok := got[k]
				if !ok {
					t.Errorf("missing key %q", k)
					continue
				}
				if len(gotVals) != len(wantVals) {
					t.Errorf("key %q: got %d values %v, want %d values %v", k, len(gotVals), gotVals, len(wantVals), wantVals)
					continue
				}
				for i := range wantVals {
					if gotVals[i] != wantVals[i] {
						t.Errorf("key %q[%d]: got %q, want %q", k, i, gotVals[i], wantVals[i])
					}
				}
			}
			// Check no unexpected keys
			for k := range got {
				if _, ok := tt.want[k]; !ok {
					t.Errorf("unexpected key %q with values %v", k, got[k])
				}
			}
		})
	}
}

func TestPaginateAll_NoStreamSummaryLog(t *testing.T) {
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(map[string]interface{}{
			"code": 0, "msg": "ok",
			"data": map[string]interface{}{
				"items":    []interface{}{map[string]interface{}{"id": "1"}},
				"has_more": false,
			},
		}), nil
	})

	ac, errBuf := newTestAPIClient(t, rt)

	result, err := ac.PaginateAll(context.Background(), RawApiRequest{
		Method: "GET",
		URL:    "/open-apis/contact/v3/users",
		As:     "bot",
	}, PaginationOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(errBuf.String(), "[pagination] streamed") {
		t.Error("expected no streaming summary log from PaginateAll")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestDoStream_IgnoresBaseHTTPClientTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(25 * time.Millisecond)
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	ac := &APIClient{
		HTTP:       &http.Client{Timeout: 5 * time.Millisecond},
		Credential: credential.NewCredentialProvider(nil, nil, &staticTokenResolver{}, nil),
		Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
	}

	resp, err := ac.DoStream(context.Background(), &larkcore.ApiReq{
		HttpMethod: http.MethodGet,
		ApiPath:    srv.URL,
	}, core.AsBot)
	if err != nil {
		t.Fatalf("DoStream() error = %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("response body = %q, want %q", string(body), "ok")
	}
}

// TestDoStream_TransportFailureSplitsSubtype pins that a streaming-request
// transport failure routes through classifyNetworkSubtype rather than emitting
// a hardcoded SubtypeNetworkTransport for every cause. Concretely: a DNS
// failure must surface as SubtypeNetworkDNS so downstream agents can react
// (retry / give up / show recovery hint) without parsing the message text.
// Pre-fix, DoStream collapsed every httpClient.Do failure to NetworkTransport,
// erasing the timeout / TLS / DNS distinctions the SDK path already preserved.
func TestDoStream_TransportFailureSplitsSubtype(t *testing.T) {
	rt := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, &net.DNSError{Err: "no such host", Name: "nowhere.invalid"}
	})
	ac := &APIClient{
		HTTP:       &http.Client{Transport: rt},
		Credential: credential.NewCredentialProvider(nil, nil, &staticTokenResolver{}, nil),
		Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
	}

	_, err := ac.DoStream(context.Background(), &larkcore.ApiReq{
		HttpMethod: http.MethodGet,
		ApiPath:    "/open-apis/drive/v1/files/file_token/download",
	}, core.AsBot)
	if err == nil {
		t.Fatal("expected DNS error from DoStream transport, got nil")
	}
	var netErr *errs.NetworkError
	if !errors.As(err, &netErr) {
		t.Fatalf("expected *errs.NetworkError, got %T (%v)", err, err)
	}
	if netErr.Subtype != errs.SubtypeNetworkDNS {
		t.Errorf("Subtype = %q, want %q (DNS failures must not be classified as generic transport)", netErr.Subtype, errs.SubtypeNetworkDNS)
	}
}

func TestDoStream_PreservesTypedTransportError(t *testing.T) {
	policyErr := errs.NewSecurityPolicyError(errs.SubtypeAccessDenied, "blocked redirect")
	ac := &APIClient{
		HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, policyErr
		})},
		Credential: credential.NewCredentialProvider(nil, nil, &staticTokenResolver{}, nil),
		Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
	}

	_, err := ac.DoStream(context.Background(), &larkcore.ApiReq{
		HttpMethod: http.MethodGet,
		ApiPath:    "/open-apis/drive/v1/files/file_token/download",
	}, core.AsBot)
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != errs.CategoryPolicy || problem.Subtype != errs.SubtypeAccessDenied {
		t.Fatalf("DoStream() problem = %#v, %v; want policy/access_denied", problem, ok)
	}
	if !errors.Is(err, policyErr) {
		t.Fatal("DoStream() did not preserve the typed transport error")
	}
}

func TestDoStream_RateLimitSendsOnce(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			ac := &APIClient{
				HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					return &http.Response{
						StatusCode: http.StatusTooManyRequests,
						Header: http.Header{
							"Content-Type": []string{"application/json"},
							"Retry-After":  []string{"7"},
						},
						Body: io.NopCloser(strings.NewReader(`{"code":99991400,"msg":"slow"}`)),
					}, nil
				})},
				Credential: credential.NewCredentialProvider(nil, nil, &staticTokenResolver{}, nil),
				Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
			}
			_, err := ac.DoStream(context.Background(), &larkcore.ApiReq{HttpMethod: method, ApiPath: "https://example.invalid/open-apis/test", Body: strings.NewReader("payload")}, core.AsBot)
			var apiErr *errs.APIError
			if !errors.As(err, &apiErr) || apiErr.Subtype != errs.SubtypeRateLimit {
				t.Fatalf("DoStream() error = %T (%v), want api/rate_limit", err, err)
			}
			if calls != 1 {
				t.Fatalf("request calls = %d, want exactly 1", calls)
			}
		})
	}
}

func TestDoAPI_RateLimitSendsOnceForEveryMethod(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			ac, _ := newTestAPIClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
						"Retry-After":  []string{"2"},
					},
					Body: io.NopCloser(strings.NewReader(`{"code":99991400,"msg":"slow"}`)),
				}, nil
			}))

			resp, err := ac.DoAPI(context.Background(), RawApiRequest{
				Method: method,
				URL:    "/open-apis/test/v1/resources",
				Data:   map[string]interface{}{"value": "body must not be replayed"},
				As:     core.AsBot,
			})
			if err != nil {
				t.Fatalf("DoAPI() error = %v", err)
			}
			err = HandleResponse(resp, ResponseOptions{Out: io.Discard, ErrOut: io.Discard})
			var apiErr *errs.APIError
			if !errors.As(err, &apiErr) || apiErr.Subtype != errs.SubtypeRateLimit {
				t.Fatalf("HandleResponse() error = %T (%v), want api/rate_limit", err, err)
			}
			if calls != 1 {
				t.Fatalf("request calls = %d, want exactly 1", calls)
			}
		})
	}
}

func TestDoStream_RateLimitErrorBodyIsBounded(t *testing.T) {
	body := strings.Repeat("x", 8192)
	ac := &APIClient{
		HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})},
		Credential: credential.NewCredentialProvider(nil, nil, &staticTokenResolver{}, nil),
		Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
	}
	_, err := ac.DoStream(context.Background(), &larkcore.ApiReq{HttpMethod: http.MethodGet, ApiPath: "https://example.invalid/open-apis/test"}, core.AsBot)
	var apiErr *errs.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("DoStream() error = %T (%v), want *errs.APIError", err, err)
	}
	if len(apiErr.Message) > 4600 {
		t.Fatalf("rate-limit message unexpectedly contains unbounded body: len=%d", len(apiErr.Message))
	}
}

type streamBoundaryErrorReader struct {
	prefix *strings.Reader
	err    error
}

func (r *streamBoundaryErrorReader) Read(p []byte) (int, error) {
	if r.prefix.Len() > 0 {
		return r.prefix.Read(p)
	}
	return 0, r.err
}

func TestDoStream_HTTP429RejectsUntrustedFourKiBPrefixMetadata(t *testing.T) {
	const maxBody = 4096
	prefix := `{"code":99991400,"log_id":"body-forged"}`
	trustedPrefix := prefix + strings.Repeat(" ", maxBody-len(prefix))
	sentinel := errors.New("read beyond four KiB")
	tests := []struct {
		name string
		body func() io.ReadCloser
	}{
		{name: "trailing junk beyond limit", body: func() io.ReadCloser { return io.NopCloser(strings.NewReader(trustedPrefix + "junk")) }},
		{name: "second JSON beyond limit", body: func() io.ReadCloser { return io.NopCloser(strings.NewReader(trustedPrefix + `{"code":99991400}`)) }},
		{name: "boundary read error", body: func() io.ReadCloser {
			return io.NopCloser(&streamBoundaryErrorReader{prefix: strings.NewReader(trustedPrefix), err: sentinel})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			ac := &APIClient{
				HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					calls++
					return &http.Response{
						StatusCode: http.StatusTooManyRequests,
						Header: http.Header{
							"X-Request-Id": []string{"header-log"},
							"Retry-After":  []string{"12"},
						},
						Body: tt.body(),
					}, nil
				})},
				Credential: credential.NewCredentialProvider(nil, nil, &staticTokenResolver{}, nil),
				Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
			}
			_, err := ac.DoStream(context.Background(), &larkcore.ApiReq{HttpMethod: http.MethodGet, ApiPath: "https://example.invalid/open-apis/test"}, core.AsBot)
			var apiErr *errs.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("DoStream() error = %T (%v), want APIError", err, err)
			}
			if apiErr.Code != 429 || apiErr.LogID != "header-log" || apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != 12 {
				t.Fatalf("untrusted 4 KiB prefix supplied metadata: %#v", apiErr)
			}
			if apiErr.Cause != nil || errors.Is(err, sentinel) {
				t.Fatalf("body read error leaked into classification: %#v", apiErr)
			}
			if calls != 1 {
				t.Fatalf("request calls = %d, want 1", calls)
			}
		})
	}
}

func TestDoStream_BusinessRateLimitOnHTTP400(t *testing.T) {
	ac := &APIClient{
		HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"code":99991400,"msg":"slow"}`)),
			}, nil
		})},
		Credential: credential.NewCredentialProvider(nil, nil, &staticTokenResolver{}, nil),
		Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
	}
	_, err := ac.DoStream(context.Background(), &larkcore.ApiReq{HttpMethod: http.MethodGet, ApiPath: "https://example.invalid/open-apis/test"}, core.AsBot)
	var apiErr *errs.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 99991400 || apiErr.Subtype != errs.SubtypeRateLimit {
		t.Fatalf("DoStream() error = %#v (%v), want api/rate_limit code 99991400", apiErr, err)
	}
}

func TestDoStream_HTTP429PreservesLongTermQuotaClassification(t *testing.T) {
	ac := &APIClient{
		HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Retry-After": []string{"4"}},
				Body:       io.NopCloser(strings.NewReader(`{"code":1063006,"msg":"daily quota"}`)),
			}, nil
		})},
		Credential: credential.NewCredentialProvider(nil, nil, &staticTokenResolver{}, nil),
		Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
	}
	_, err := ac.DoStream(context.Background(), &larkcore.ApiReq{HttpMethod: http.MethodPost, ApiPath: "https://example.invalid/open-apis/test"}, core.AsUser)
	var apiErr *errs.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("DoStream() error = %T (%v), want *errs.APIError", err, err)
	}
	if apiErr.Code != 1063006 || apiErr.Retryable || apiErr.RetryAfterSeconds != nil {
		t.Fatalf("long-term quota classification was changed: %#v", apiErr)
	}
}

// failingTokenResolver always returns TokenUnavailableError, exercising the
// auth/credential failure path through resolveAccessToken.
type failingTokenResolver struct{}

func (f *failingTokenResolver) ResolveToken(_ context.Context, spec credential.TokenSpec) (*credential.TokenResult, error) {
	return nil, &credential.TokenUnavailableError{Source: "test", Type: spec.Type}
}

// TestResolveAccessToken_NoToken_ReturnsTypedAuthenticationError pins that
// the missing-token path of resolveAccessToken returns the typed
// *errs.AuthenticationError{Subtype: TokenMissing}.
func TestResolveAccessToken_NoToken_ReturnsTypedAuthenticationError(t *testing.T) {
	ac := &APIClient{
		HTTP:       &http.Client{},
		Credential: credential.NewCredentialProvider(nil, nil, &failingTokenResolver{}, nil),
		Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
	}

	_, err := ac.resolveAccessToken(context.Background(), core.AsUser)
	if err == nil {
		t.Fatal("expected error when no token available, got nil")
	}

	var authErr *errs.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected *errs.AuthenticationError, got %T (%v)", err, err)
	}
	if authErr.Category != errs.CategoryAuthentication {
		t.Errorf("Category = %v, want %v", authErr.Category, errs.CategoryAuthentication)
	}
	if authErr.Subtype != errs.SubtypeTokenMissing {
		t.Errorf("Subtype = %v, want %v", authErr.Subtype, errs.SubtypeTokenMissing)
	}
}

// needAuthTokenResolver mirrors the production credential chain: the
// missing-UAT case is constructed typed at the source (internal/auth) and
// carries the legacy *NeedAuthorizationError sentinel in its Cause chain. It
// must surface as a typed AuthenticationError and flow through resolveAccessToken
// and WrapDoAPIError unchanged (never mis-classified as NetworkError).
type needAuthTokenResolver struct {
	userOpenID string
}

func (f *needAuthTokenResolver) ResolveToken(_ context.Context, _ credential.TokenSpec) (*credential.TokenResult, error) {
	return nil, internalauth.NewNeedUserAuthorizationError(f.userOpenID)
}

// TestResolveAccessToken_NeedAuthorization_SurfacesAsTypedAuthentication
// pins that the typed missing-UAT error from the credential chain reaches the
// caller as a typed AuthenticationError with the marker and sentinel intact.
func TestResolveAccessToken_NeedAuthorization_SurfacesAsTypedAuthentication(t *testing.T) {
	ac := &APIClient{
		HTTP:       &http.Client{},
		Credential: credential.NewCredentialProvider(nil, nil, &needAuthTokenResolver{userOpenID: "ou_test_user"}, nil),
		Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
	}

	_, err := ac.resolveAccessToken(context.Background(), core.AsUser)
	if err == nil {
		t.Fatal("expected error when credential chain signals need_user_authorization, got nil")
	}

	var authErr *errs.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected *errs.AuthenticationError, got %T (%v)", err, err)
	}
	if authErr.Subtype != errs.SubtypeTokenMissing {
		t.Errorf("Subtype = %v, want %v", authErr.Subtype, errs.SubtypeTokenMissing)
	}
	if !strings.Contains(authErr.Message, "need_user_authorization") {
		t.Errorf("Message must contain the marker 'need_user_authorization' (invariant), got %q", authErr.Message)
	}
	// Underlying NeedAuthorizationError preserved in Cause chain so
	// existing errors.As(&NeedAuthorizationError{}) consumers still match.
	var needErr *internalauth.NeedAuthorizationError
	if !errors.As(err, &needErr) {
		t.Errorf("NeedAuthorizationError not preserved in Cause chain")
	}
}

// TestDoSDKRequest_AuthFailureSurfacesTypedAuthenticationError pins the
// end-to-end invariant codex caught the day this PR landed: when
// resolveAccessToken fails because no token is cached, DoSDKRequest must
// surface that as a typed *errs.AuthenticationError — not silently downgrade
// it to a network error via the SDK-failure wrap.
//
// Regression scenario: shortcut path
// (shortcuts/common/runner.go DoAPI → DoSDKRequest) calling against a user
// identity with no cached token. Pre-fix this surfaced as exit 4/type=network
// and routed agents into "check your connection" instead of "log in".
func TestDoSDKRequest_AuthFailureSurfacesTypedAuthenticationError(t *testing.T) {
	ac := &APIClient{
		HTTP:       &http.Client{},
		Credential: credential.NewCredentialProvider(nil, nil, &failingTokenResolver{}, nil),
		Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
	}

	_, err := ac.DoSDKRequest(context.Background(), &larkcore.ApiReq{
		HttpMethod: http.MethodGet,
		ApiPath:    "/open-apis/contact/v3/users/me",
	}, core.AsUser)

	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
	var authErr *errs.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected *errs.AuthenticationError, got %T (%v) — WrapDoAPIError must pass typed *errs.* through unchanged", err, err)
	}
	if authErr.Subtype != errs.SubtypeTokenMissing {
		t.Errorf("Subtype = %v, want %v", authErr.Subtype, errs.SubtypeTokenMissing)
	}
}

// TestDoSDKRequest_TransportFailureWrapsAsNetwork pins that genuinely untyped
// SDK transport errors get the typed network classification via WrapDoAPIError.
// io.ErrUnexpectedEOF from a RoundTripper surfaces through net/http as a
// *url.Error, which the wrap classifier reaches as the transport-error
// fallback (no specific subtype matches — falls back to transport).
func TestDoSDKRequest_TransportFailureWrapsAsNetwork(t *testing.T) {
	rt := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	})
	ac, _ := newTestAPIClient(t, rt)

	_, err := ac.DoSDKRequest(context.Background(), &larkcore.ApiReq{
		HttpMethod: http.MethodGet,
		ApiPath:    "/open-apis/contact/v3/users/me",
	}, core.AsBot)

	if err == nil {
		t.Fatal("expected error from broken transport, got nil")
	}
	var netErr *errs.NetworkError
	if !errors.As(err, &netErr) {
		t.Fatalf("expected *errs.NetworkError, got %T (%v)", err, err)
	}
	if netErr.Category != errs.CategoryNetwork {
		t.Errorf("Category = %v, want %v", netErr.Category, errs.CategoryNetwork)
	}
	if netErr.Subtype != errs.SubtypeNetworkTransport {
		t.Errorf("Subtype = %v, want %v", netErr.Subtype, errs.SubtypeNetworkTransport)
	}
	// io.ErrUnexpectedEOF round-tripping through net/http does not satisfy
	// any of the specific cause checks; subtype falls back to transport.
	if output.ExitCodeOf(err) != output.ExitNetwork {
		t.Errorf("ExitCodeOf = %d, want %d (network)", output.ExitCodeOf(err), output.ExitNetwork)
	}
}

// TestCallAPI_ParseJSONFailureWrapsAsAPI pins the typed-envelope contract for
// malformed JSON response bodies: WrapJSONResponseParseError emits
// *errs.InternalError{Subtype: invalid_response} with the rawAPIJSONHint
// preserved on Problem.Hint. Pagination / cmd/api / cmd/service callers see
// the typed JSON stderr envelope (exit 5/internal) — wire `type` is
// "internal".
func TestCallAPI_ParseJSONFailureWrapsAsAPI(t *testing.T) {
	rt := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{ malformed`)),
		}, nil
	})
	ac, _ := newTestAPIClient(t, rt)

	_, err := ac.CallAPI(context.Background(), RawApiRequest{
		Method: "GET",
		URL:    "/open-apis/contact/v3/users/me",
		As:     "bot",
	})

	if err == nil {
		t.Fatal("expected JSON parse error, got nil")
	}
	var intErr *errs.InternalError
	if !errors.As(err, &intErr) {
		t.Fatalf("expected *errs.InternalError, got %T (%v)", err, err)
	}
	if intErr.Category != errs.CategoryInternal {
		t.Errorf("Category = %v, want %v", intErr.Category, errs.CategoryInternal)
	}
	if intErr.Subtype != errs.SubtypeInvalidResponse {
		t.Errorf("Subtype = %v, want %v", intErr.Subtype, errs.SubtypeInvalidResponse)
	}
	if intErr.Hint != rawAPIJSONHint {
		t.Errorf("Hint = %q, want rawAPIJSONHint preserved", intErr.Hint)
	}
	if output.ExitCodeOf(err) != output.ExitInternal {
		t.Errorf("ExitCodeOf = %d, want %d (internal)", output.ExitCodeOf(err), output.ExitInternal)
	}
}

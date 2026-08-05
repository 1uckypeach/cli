// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package event

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/client"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/credential"
)

// staticTokenResolver always returns a fixed token without any HTTP calls.
type staticTokenResolver struct{}

func (s *staticTokenResolver) ResolveToken(_ context.Context, _ credential.TokenSpec) (*credential.TokenResult, error) {
	return &credential.TokenResult{Token: "test-token"}, nil
}

// stubRoundTripper intercepts every outgoing request with a canned response.
type stubRoundTripper struct {
	respond func(*http.Request) (*http.Response, error)
}

func (s stubRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return s.respond(r) }

func newTestConsumeRuntime(rt http.RoundTripper) *consumeRuntime {
	sdk := lark.NewClient("test-app", "test-secret",
		lark.WithEnableTokenCache(false),
		lark.WithLogLevel(larkcore.LogLevelError),
		lark.WithHttpClient(&http.Client{Transport: rt}),
	)
	return &consumeRuntime{
		client: &client.APIClient{
			SDK:        sdk,
			ErrOut:     io.Discard,
			Credential: credential.NewCredentialProvider(nil, nil, &staticTokenResolver{}, nil),
			Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
		},
		accessIdentity: core.AsBot,
	}
}

func stubResponse(status int, contentType, body string) func(*http.Request) (*http.Response, error) {
	return func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{contentType}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	}
}

func requireCallAPIProblem(t *testing.T, err error, category errs.Category, subtype errs.Subtype) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("expected typed errs error, got %T: %v", err, err)
	}
	if p.Category != category || p.Subtype != subtype {
		t.Fatalf("problem = %s/%s, want %s/%s", p.Category, p.Subtype, category, subtype)
	}
}

func TestConsumeRuntimeCallAPI_NonJSONHTTPError(t *testing.T) {
	r := newTestConsumeRuntime(stubRoundTripper{respond: stubResponse(http.StatusNotFound, "text/plain", "gone")})
	_, err := r.CallAPI(context.Background(), "GET", "/open-apis/event/v1/connection", nil)
	requireCallAPIProblem(t, err, errs.CategoryInternal, errs.SubtypeInvalidResponse)
	if !strings.Contains(err.Error(), "returned 404") {
		t.Errorf("error should echo the HTTP status, got: %v", err)
	}
}

func TestConsumeRuntimeCallAPI_NonJSONHTTPErrorTruncatesLongBody(t *testing.T) {
	long := strings.Repeat("x", 300)
	r := newTestConsumeRuntime(stubRoundTripper{respond: stubResponse(http.StatusBadGateway, "text/html", long)})
	_, err := r.CallAPI(context.Background(), "GET", "/open-apis/event/v1/connection", nil)
	requireCallAPIProblem(t, err, errs.CategoryNetwork, errs.SubtypeNetworkServer)
	p, _ := errs.ProblemOf(err)
	if !p.Retryable {
		t.Fatal("5xx non-JSON response should be marked retryable")
	}
	if !strings.Contains(err.Error(), "…(truncated)") {
		t.Errorf("long body should be truncated in the message, got: %v", err)
	}
}

func TestConsumeRuntimeCallAPI_UnparsableJSONBody(t *testing.T) {
	r := newTestConsumeRuntime(stubRoundTripper{respond: stubResponse(http.StatusOK, "application/json", "{not json")})
	const path = "/open-apis/event/v1/connection"
	_, err := r.CallAPI(context.Background(), "GET", path, nil)
	requireCallAPIProblem(t, err, errs.CategoryInternal, errs.SubtypeInvalidResponse)
	if !strings.Contains(err.Error(), "api GET "+path) {
		t.Fatalf("parse error lost method/path context: %v", err)
	}
}

func TestConsumeRuntimeCallAPI_JSON5xxMatchesEventRetryabilityContract(t *testing.T) {
	r := newTestConsumeRuntime(stubRoundTripper{respond: stubResponse(http.StatusBadGateway, "application/json", `{"code":0}`)})
	_, err := r.CallAPI(context.Background(), "GET", "/open-apis/event/v1/connection", nil)
	requireCallAPIProblem(t, err, errs.CategoryNetwork, errs.SubtypeNetworkServer)
	problem, _ := errs.ProblemOf(err)
	if !problem.Retryable {
		t.Fatal("event JSON 5xx must match the existing retryable non-JSON 5xx contract")
	}
}

func TestConsumeRuntimeCallAPI_TransportFailure(t *testing.T) {
	r := newTestConsumeRuntime(stubRoundTripper{respond: func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}})
	_, err := r.CallAPI(context.Background(), "GET", "/open-apis/event/v1/connection", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("expected typed errs error, got %T: %v", err, err)
	}
	if p.Category != errs.CategoryNetwork {
		t.Fatalf("category = %s, want %s", p.Category, errs.CategoryNetwork)
	}
}

func TestConsumeRuntimeCallAPI_EnvelopeErrorIsTyped(t *testing.T) {
	r := newTestConsumeRuntime(stubRoundTripper{respond: stubResponse(http.StatusOK, "application/json",
		`{"code":99991663,"msg":"app not found"}`)})
	_, err := r.CallAPI(context.Background(), "GET", "/open-apis/event/v1/connection", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errs.ProblemOf(err); !ok {
		t.Fatalf("envelope error should be typed via BuildAPIError, got %T: %v", err, err)
	}
}

func TestConsumeRuntimeCallAPI_RateLimitRecoveryMetadata(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		ct       string
		body     string
		wantCode int
	}{
		{name: "HTTP 429 JSON code zero", status: http.StatusTooManyRequests, ct: "application/json", body: `{"code":0,"msg":"slow"}`, wantCode: 429},
		{name: "HTTP 429 JSON without code", status: http.StatusTooManyRequests, ct: "application/json", body: `{"msg":"slow"}`, wantCode: 429},
		{name: "HTTP 429 non JSON", status: http.StatusTooManyRequests, ct: "text/plain", body: "slow", wantCode: 429},
		{name: "HTTP 429 empty body", status: http.StatusTooManyRequests, body: "", wantCode: 429},
		{name: "business rate limit HTTP 200", status: http.StatusOK, ct: "application/json", body: `{"code":99991400,"msg":"slow"}`, wantCode: 99991400},
		{name: "business rate limit HTTP 400", status: http.StatusBadRequest, ct: "application/json", body: `{"code":99991400,"msg":"slow"}`, wantCode: 99991400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			r := newTestConsumeRuntime(stubRoundTripper{respond: func(req *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{
					StatusCode: tt.status,
					Header: http.Header{
						"Content-Type": []string{tt.ct},
						"Retry-After":  []string{"13"},
					},
					Body:    io.NopCloser(strings.NewReader(tt.body)),
					Request: req,
				}, nil
			}})

			_, err := r.CallAPI(context.Background(), "POST", "/open-apis/event/v1/connection", map[string]any{"write": true})
			var apiErr *errs.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("CallAPI() error = %T (%v), want *errs.APIError", err, err)
			}
			if apiErr.Subtype != errs.SubtypeRateLimit || apiErr.Code != tt.wantCode || !apiErr.Retryable {
				t.Fatalf("rate limit problem = %#v, want code %d", apiErr.Problem, tt.wantCode)
			}
			if apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != 13 || apiErr.RetryAfterSource != "retry-after" {
				t.Fatalf("retry metadata = (%v, %q), want (13, retry-after)", apiErr.RetryAfterSeconds, apiErr.RetryAfterSource)
			}
			if !strings.Contains(apiErr.Hint, "safe to replay") {
				t.Fatalf("hint does not warn against unsafe write replay: %q", apiErr.Hint)
			}
			if calls != 1 {
				t.Fatalf("request count = %d, want 1", calls)
			}
		})
	}
}

func TestConsumeRuntimeCallAPI_Success(t *testing.T) {
	r := newTestConsumeRuntime(stubRoundTripper{respond: stubResponse(http.StatusOK, "application/json",
		`{"code":0,"data":{"ok":true}}`)})
	raw, err := r.CallAPI(context.Background(), "GET", "/open-apis/event/v1/connection", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(raw), `"code":0`) {
		t.Errorf("raw body should pass through, got: %s", raw)
	}
}

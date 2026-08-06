// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package credential

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/core"
)

// stubRoundTripper lets us assert request shape and return canned responses.
type stubRoundTripper struct {
	gotReq   *http.Request
	gotBody  string
	respCode int
	respBody string
	respRead io.ReadCloser
	header   http.Header
	err      error
	calls    int
}

func (s *stubRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	s.calls++
	s.gotReq = req
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		s.gotBody = string(b)
	}
	if s.err != nil {
		return nil, s.err
	}
	body := s.respRead
	if body == nil {
		body = io.NopCloser(strings.NewReader(s.respBody))
	}
	header := make(http.Header)
	if s.header != nil {
		header = s.header.Clone()
	}
	return &http.Response{
		StatusCode: s.respCode,
		Body:       body,
		Header:     header,
	}, nil
}

type boundaryErrorReader struct {
	prefix        *strings.Reader
	err           error
	boundaryReads int
}

func (r *boundaryErrorReader) Read(p []byte) (int, error) {
	if r.prefix.Len() > 0 {
		return r.prefix.Read(p)
	}
	r.boundaryReads++
	return 0, r.err
}

func TestFetchTAT_Success(t *testing.T) {
	rt := &stubRoundTripper{
		respCode: 200,
		respBody: `{"code":0,"access_token":"t-abc","token_type":"Bearer","expires_in":7200}`,
	}
	hc := &http.Client{Transport: rt}

	token, err := FetchTAT(context.Background(), hc, core.BrandFeishu, "cli_app", "secret_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "t-abc" {
		t.Errorf("token = %q, want t-abc", token)
	}
	if rt.gotReq.URL.String() != "https://accounts.feishu.cn/oauth/v3/token" {
		t.Errorf("url = %s", rt.gotReq.URL.String())
	}
	if ct := rt.gotReq.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", ct)
	}
	// client_secret_post: grant_type + client_id + client_secret in the form body.
	for _, want := range []string{"grant_type=client_credentials", "client_id=cli_app", "client_secret=secret_x"} {
		if !strings.Contains(rt.gotBody, want) {
			t.Errorf("request body missing %q: %s", want, rt.gotBody)
		}
	}
}

// invalid_client (wrong app_id/app_secret on the client_credentials grant) is a
// deterministic client-side rejection that FetchTAT routes to
// classifyTATResponseCode as CategoryConfig / SubtypeInvalidClient — the same
// typed error doResolveTAT (and thus every token-resolving command) returns.
// The v3 endpoint reports it as HTTP 400 with the OAuth2 error body (wrong
// secret → code 20002, unknown app → code 20048).
func TestFetchTAT_InvalidClient_ConfigInvalidClient(t *testing.T) {
	rt := &stubRoundTripper{respCode: 400, respBody: `{"error":"invalid_client","error_description":"The client secret is invalid.","code":20002}`}
	hc := &http.Client{Transport: rt}

	token, err := FetchTAT(context.Background(), hc, core.BrandFeishu, "cli_app", "secret_x")
	if err == nil {
		t.Fatal("expected error for invalid_client")
	}
	if token != "" {
		t.Errorf("token = %q, want empty", token)
	}
	var cfgErr *errs.ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("error not *errs.ConfigError: %T %v", err, err)
	}
	if cfgErr.Category != errs.CategoryConfig {
		t.Errorf("Category = %q, want %q", cfgErr.Category, errs.CategoryConfig)
	}
	if cfgErr.Subtype != errs.SubtypeInvalidClient {
		t.Errorf("Subtype = %q, want %q", cfgErr.Subtype, errs.SubtypeInvalidClient)
	}
}

// Any other deterministic client-side OAuth error (e.g. invalid_scope) still
// yields a typed error (errs.IsTyped) via BuildAPIError — so a probe caller
// surfaces it rather than silently swallowing it — but is NOT classified as a
// credential (invalid_client) problem.
func TestFetchTAT_OtherClientError_Typed(t *testing.T) {
	rt := &stubRoundTripper{respCode: 400, respBody: `{"code":20068,"error":"invalid_scope","error_description":"unauthorized scope"}`}
	hc := &http.Client{Transport: rt}

	_, err := FetchTAT(context.Background(), hc, core.BrandFeishu, "cli_app", "secret_x")
	if err == nil {
		t.Fatal("expected error for invalid_scope")
	}
	if !errs.IsTyped(err) {
		t.Fatalf("expected a typed errs.* error, got %T %v", err, err)
	}
	var cfgErr *errs.ConfigError
	if errors.As(err, &cfgErr) {
		t.Errorf("invalid_scope must not be classified as ConfigError/InvalidClient, got %T", err)
	}
}

// A deterministic OAuth error that arrives WITHOUT a numeric code (code defaults to
// 0) must still surface as a non-nil typed error — never the ("", nil) success pair.
// Guards the code-0 backstop in classifyTATResponseCode: BuildAPIError returns nil
// for code 0, which would otherwise swallow this rejection into an empty-token success.
func TestFetchTAT_OtherClientError_CodeZero_Typed(t *testing.T) {
	rt := &stubRoundTripper{respCode: 400, respBody: `{"error":"invalid_scope","error_description":"the requested scope is not granted"}`}
	hc := &http.Client{Transport: rt}

	tok, err := FetchTAT(context.Background(), hc, core.BrandFeishu, "cli_app", "secret_x")
	if err == nil {
		t.Fatal("expected non-nil error for code-0 invalid_scope (must not return empty token + nil error)")
	}
	if tok != "" {
		t.Errorf("token = %q, want empty", tok)
	}
	if !errs.IsTyped(err) {
		t.Fatalf("expected a typed errs.* error, got %T %v", err, err)
	}
}

// A gateway-style {code, msg} error (no OAuth error / error_description fields)
// must still surface its msg on the typed error, not degrade to a generic
// "API error: [code]". Guards the legacy-msg fallback in FetchTAT.
func TestFetchTAT_LarkStyleMsg_FallsBackOnTypedError(t *testing.T) {
	rt := &stubRoundTripper{respCode: 400, respBody: `{"code":99999,"msg":"app ticket invalid"}`}
	hc := &http.Client{Transport: rt}

	_, err := FetchTAT(context.Background(), hc, core.BrandFeishu, "cli_app", "secret_x")
	if err == nil {
		t.Fatal("expected error for {code, msg} response")
	}
	if !errs.IsTyped(err) {
		t.Fatalf("expected a typed errs.* error, got %T %v", err, err)
	}
	if !strings.Contains(err.Error(), "app ticket invalid") {
		t.Errorf("typed error must carry the Lark msg, got: %v", err)
	}
}

// Transient server-side failures (5xx / server_error) are NOT deterministic
// credential rejections — they must stay UNTYPED so a probe caller treats them
// as upstream noise and stays silent (and retryers can back off).
func TestFetchTAT_ServerError_Untyped(t *testing.T) {
	rt := &stubRoundTripper{respCode: 500, respBody: `{"code":20050,"error":"server_error","error_description":"please retry"}`}
	hc := &http.Client{Transport: rt}

	_, err := FetchTAT(context.Background(), hc, core.BrandFeishu, "cli_app", "secret_x")
	if err == nil {
		t.Fatal("expected error for server_error")
	}
	if errs.IsTyped(err) {
		t.Errorf("server_error must be UNTYPED (transient), got typed %T %v", err, err)
	}
}

func TestFetchTAT_HTTP429TypedSafeRateLimit(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		header   http.Header
		wantCode int
		wantLog  string
		wantWait int
		wantFrom string
	}{
		{name: "business code", body: `{"code":99991400,"error_description":"secret\u0000oauth text","log_id":"body-log"}`, header: http.Header{"Retry-After": []string{"7"}, "X-Tt-Logid": []string{"header-log"}}, wantCode: 99991400, wantLog: "body-log", wantWait: 7, wantFrom: "retry-after"},
		{name: "platform reset header", body: `{"error":"too_many_requests"}`, header: http.Header{"X-Ogw-Ratelimit-Reset": []string{"8"}, "Retry-After": []string{"4"}}, wantCode: 429, wantWait: 8, wantFrom: "x-ogw-ratelimit-reset"},
		{name: "unrelated business code", body: `{"code":20002,"msg":"secret"}`, wantCode: 429, wantWait: 1},
		{name: "plain text", body: "secret plaintext", wantCode: 429, wantWait: 1},
		{name: "html", body: "<html>secret</html>", wantCode: 429, wantWait: 1},
		{name: "empty", wantCode: 429, wantWait: 1},
		{name: "malformed invalid utf8", body: "{\xffsecret", wantCode: 429, wantWait: 1},
		{name: "trailing junk cannot forge metadata", body: `{"code":99991400,"log_id":"forged"}trailing-junk`, wantCode: 429, wantWait: 1},
		{name: "concatenated values cannot forge metadata", body: `{"code":99991400,"log_id":"forged"}{"code":99991400}`, wantCode: 429, wantWait: 1},
		{name: "invalid log falls through", body: `{"log_id":"bad/value","error":{"log_id":"nested-ok"}}`, header: http.Header{"X-Tt-Logid": []string{"header-log"}}, wantCode: 429, wantLog: "nested-ok", wantWait: 1},
		{name: "multiple retry after defaults", header: http.Header{"Retry-After": []string{"3", "4"}}, wantCode: 429, wantWait: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &stubRoundTripper{respCode: 429, respBody: tt.body, header: tt.header}
			_, err := FetchTAT(context.Background(), &http.Client{Transport: rt}, core.BrandFeishu, "cli_app", "secret_x")
			var apiErr *errs.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %T (%v), want APIError", err, err)
			}
			if apiErr.Code != tt.wantCode || apiErr.Subtype != errs.SubtypeRateLimit || !apiErr.Retryable {
				t.Fatalf("problem = %#v", apiErr.Problem)
			}
			if apiErr.Message != "request rate limit exceeded" || strings.Contains(apiErr.Hint, "secret") || apiErr.Cause != nil {
				t.Fatalf("unsafe response detail leaked: %#v", apiErr)
			}
			if apiErr.LogID != tt.wantLog || apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != tt.wantWait {
				t.Fatalf("metadata = log %q retry %v, want %q/%d", apiErr.LogID, apiErr.RetryAfterSeconds, tt.wantLog, tt.wantWait)
			}
			if tt.wantFrom != "" && apiErr.RetryAfterSource != tt.wantFrom {
				t.Fatalf("retry source = %q, want %q", apiErr.RetryAfterSource, tt.wantFrom)
			}
			if rt.calls != 1 {
				t.Fatalf("request count = %d, want 1", rt.calls)
			}
		})
	}
}

func TestFetchTAT_HTTP429BoundedBodyDoesNotLeak(t *testing.T) {
	prefix := `{"code":99991400,"log_id":"body-forged"}`
	padding := strings.Repeat(" ", maxTATResponseBodyBytes-len(prefix))
	for _, suffix := range []string{
		"trailing-junk",
		`{"code":99991400,"log_id":"second-forged"}`,
	} {
		t.Run(suffix, func(t *testing.T) {
			rt := &stubRoundTripper{
				respCode: 429,
				respBody: prefix + padding + suffix,
				header:   http.Header{"X-Request-Id": []string{"header-log"}, "Retry-After": []string{"11"}},
			}
			_, err := FetchTAT(context.Background(), &http.Client{Transport: rt}, core.BrandFeishu, "cli_app", "secret_x")
			var apiErr *errs.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %T (%v), want APIError", err, err)
			}
			if apiErr.Code != 429 || apiErr.LogID != "header-log" {
				t.Fatalf("overflow body supplied metadata: %#v", apiErr.Problem)
			}
			if apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != 11 || apiErr.RetryAfterSource != "retry-after" {
				t.Fatalf("header retry metadata = (%v, %q), want (11, retry-after)", apiErr.RetryAfterSeconds, apiErr.RetryAfterSource)
			}
			if apiErr.Message != "request rate limit exceeded" || strings.Contains(apiErr.Error(), "forged") || apiErr.Cause != nil {
				t.Fatalf("bounded response leaked into error: %#v", apiErr)
			}
			if rt.calls != 1 {
				t.Fatalf("request count = %d, want 1", rt.calls)
			}
		})
	}
}

func TestFetchTAT_Non429OverflowKeepsOneMiBPrefixSemantics(t *testing.T) {
	prefix := `{"code":0,"access_token":"t-ok"}`
	body := prefix + strings.Repeat(" ", maxTATResponseBodyBytes-len(prefix)) + "trailing-junk"
	rt := &stubRoundTripper{respCode: http.StatusOK, respBody: body}
	token, err := FetchTAT(context.Background(), &http.Client{Transport: rt}, core.BrandFeishu, "cli_app", "secret_x")
	if err != nil || token != "t-ok" {
		t.Fatalf("FetchTAT() = (%q, %v), want historical 1 MiB prefix success", token, err)
	}
	if rt.calls != 1 {
		t.Fatalf("request count = %d, want 1", rt.calls)
	}
}

func TestFetchTAT_ReadBoundaryCompatibilityByHTTPStatus(t *testing.T) {
	errorsAfterBoundary := []struct {
		name string
		err  error
	}{
		{name: "sentinel", err: errors.New("read beyond legacy boundary")},
		{name: "unexpected EOF", err: io.ErrUnexpectedEOF},
	}
	for _, tt := range errorsAfterBoundary {
		t.Run(tt.name, func(t *testing.T) {
			successPrefix := `{"code":0,"access_token":"t-ok"}`
			successBody := successPrefix + strings.Repeat(" ", maxTATResponseBodyBytes-len(successPrefix))
			successReader := &boundaryErrorReader{prefix: strings.NewReader(successBody), err: tt.err}
			successRT := &stubRoundTripper{respCode: http.StatusOK, respRead: io.NopCloser(successReader)}
			token, err := FetchTAT(context.Background(), &http.Client{Transport: successRT}, core.BrandFeishu, "cli_app", "secret_x")
			if err != nil || token != "t-ok" {
				t.Fatalf("HTTP 200 FetchTAT() = (%q, %v), want legacy prefix success", token, err)
			}
			if successReader.boundaryReads != 0 {
				t.Fatalf("HTTP 200 read beyond 1 MiB %d times, want 0", successReader.boundaryReads)
			}

			ratePrefix := `{"code":99991400,"log_id":"body-forged"}`
			rateBody := ratePrefix + strings.Repeat(" ", maxTATResponseBodyBytes-len(ratePrefix))
			rateReader := &boundaryErrorReader{prefix: strings.NewReader(rateBody), err: tt.err}
			rateRT := &stubRoundTripper{
				respCode: http.StatusTooManyRequests,
				respRead: io.NopCloser(rateReader),
				header:   http.Header{"X-Tt-Logid": []string{"header-log"}, "Retry-After": []string{"6"}},
			}
			_, err = FetchTAT(context.Background(), &http.Client{Transport: rateRT}, core.BrandFeishu, "cli_app", "secret_x")
			var apiErr *errs.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("HTTP 429 error = %T (%v), want safe APIError", err, err)
			}
			if apiErr.Code != 429 || apiErr.LogID != "header-log" || apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != 6 || apiErr.Cause != nil {
				t.Fatalf("HTTP 429 boundary classification = %#v", apiErr)
			}
			if rateReader.boundaryReads != 1 {
				t.Fatalf("HTTP 429 boundary probes = %d, want 1", rateReader.boundaryReads)
			}
		})
	}
}

func TestFetchTAT_EarlyBodyReadErrorByHTTPStatus(t *testing.T) {
	errorsBeforeBoundary := []struct {
		name string
		err  error
	}{
		{name: "sentinel", err: errors.New("early body read failure")},
		{name: "unexpected EOF", err: io.ErrUnexpectedEOF},
	}
	for _, tt := range errorsBeforeBoundary {
		t.Run(tt.name, func(t *testing.T) {
			partialBody := `{"code":99991400,"log_id":"body-forged"}`

			nonRateReader := &boundaryErrorReader{prefix: strings.NewReader(partialBody), err: tt.err}
			nonRateRT := &stubRoundTripper{respCode: http.StatusOK, respRead: io.NopCloser(nonRateReader)}
			_, err := FetchTAT(context.Background(), &http.Client{Transport: nonRateRT}, core.BrandFeishu, "cli_app", "secret_x")
			if !errors.Is(err, tt.err) {
				t.Fatalf("HTTP 200 error = %v, want wrapped read error %v", err, tt.err)
			}

			rateReader := &boundaryErrorReader{prefix: strings.NewReader(partialBody), err: tt.err}
			rateRT := &stubRoundTripper{
				respCode: http.StatusTooManyRequests,
				respRead: io.NopCloser(rateReader),
				header:   http.Header{"X-Request-Id": []string{"header-log"}, "Retry-After": []string{"8"}},
			}
			_, err = FetchTAT(context.Background(), &http.Client{Transport: rateRT}, core.BrandFeishu, "cli_app", "secret_x")
			var apiErr *errs.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("HTTP 429 error = %T (%v), want APIError", err, err)
			}
			if apiErr.Code != 429 || apiErr.LogID != "header-log" || apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != 8 {
				t.Fatalf("HTTP 429 early-read classification = %#v", apiErr)
			}
			if apiErr.Cause != nil || errors.Is(err, tt.err) || strings.Contains(apiErr.Message, tt.err.Error()) || strings.Contains(apiErr.Hint, tt.err.Error()) {
				t.Fatalf("HTTP 429 leaked read error: %#v", apiErr)
			}
		})
	}
}

func TestFetchTAT_OAuthSlowDownRemainsUntyped(t *testing.T) {
	rt := &stubRoundTripper{respCode: 200, respBody: `{"error":"slow_down","error_description":"polling too fast"}`}
	_, err := FetchTAT(context.Background(), &http.Client{Transport: rt}, core.BrandFeishu, "cli_app", "secret_x")
	if err == nil || errs.IsTyped(err) {
		t.Fatalf("slow_down error = %T (%v), want non-nil untyped", err, err)
	}
}

// Non-2xx HTTP with a non-JSON body is ambiguous (not a structured OAuth
// rejection) — it must stay UNTYPED so a probe caller treats it as upstream
// noise and stays silent.
func TestFetchTAT_HTTPNon200_Untyped(t *testing.T) {
	for _, code := range []int{401, 403, 500, 503} {
		rt := &stubRoundTripper{respCode: code, respBody: `whatever`}
		hc := &http.Client{Transport: rt}
		_, err := FetchTAT(context.Background(), hc, core.BrandFeishu, "cli_app", "secret_x")
		if err == nil {
			t.Fatalf("HTTP %d: expected error", code)
		}
		if errs.IsTyped(err) {
			t.Errorf("HTTP %d: must be UNTYPED (ambiguous), got typed %T %v", code, err, err)
		}
	}
}

func TestFetchTAT_TransportError_Untyped(t *testing.T) {
	sentinel := errors.New("network down")
	rt := &stubRoundTripper{err: sentinel}
	hc := &http.Client{Transport: rt}

	_, err := FetchTAT(context.Background(), hc, core.BrandFeishu, "cli_app", "secret_x")
	if err == nil {
		t.Fatal("expected error")
	}
	if errs.IsTyped(err) {
		t.Errorf("transport error must be UNTYPED, got typed %T", err)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error chain missing sentinel: %v", err)
	}
}

func TestFetchTAT_ParseError_Untyped(t *testing.T) {
	rt := &stubRoundTripper{respCode: 200, respBody: `not json`}
	hc := &http.Client{Transport: rt}

	_, err := FetchTAT(context.Background(), hc, core.BrandFeishu, "cli_app", "secret_x")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if errs.IsTyped(err) {
		t.Errorf("parse error must be UNTYPED, got typed %T", err)
	}
}

func TestFetchTAT_BrandRouting(t *testing.T) {
	tests := []struct {
		brand   core.LarkBrand
		wantURL string
	}{
		{core.BrandFeishu, "https://accounts.feishu.cn/oauth/v3/token"},
		{core.BrandLark, "https://accounts.larksuite.com/oauth/v3/token"},
	}
	for _, tc := range tests {
		t.Run(string(tc.brand), func(t *testing.T) {
			rt := &stubRoundTripper{respCode: 200, respBody: `{"code":0,"access_token":"t","token_type":"Bearer"}`}
			hc := &http.Client{Transport: rt}
			if _, err := FetchTAT(context.Background(), hc, tc.brand, "a", "b"); err != nil {
				t.Fatal(err)
			}
			if got := rt.gotReq.URL.String(); got != tc.wantURL {
				t.Errorf("url = %s, want %s", got, tc.wantURL)
			}
		})
	}
}

func TestFetchTAT_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	rt := &urlRewriteRT{base: srv.URL}
	hc := &http.Client{Transport: rt}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-canceled

	_, err := FetchTAT(ctx, hc, core.BrandFeishu, "a", "b")
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
	if errs.IsTyped(err) {
		t.Errorf("canceled context must be UNTYPED, got typed %T", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error chain missing context.Canceled: %v", err)
	}
}

// urlRewriteRT forwards requests to a fixed base URL (test server).
type urlRewriteRT struct{ base string }

func (r *urlRewriteRT) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := r.base + req.URL.Path
	req2, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	req2.Header = req.Header
	return http.DefaultTransport.RoundTrip(req2)
}

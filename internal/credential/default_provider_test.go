// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package credential

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larksuite/cli/errs"
)

func TestDefaultTokenProvider_Dispatches(t *testing.T) {
	// Just verify the type implements DefaultTokenResolver
	var _ DefaultTokenResolver = &DefaultTokenProvider{}
}

func TestDefaultAccountProvider_Implements(t *testing.T) {
	var _ DefaultAccountResolver = &DefaultAccountProvider{}
}

func requireTATProblem(t *testing.T, err error, category errs.Category, subtype errs.Subtype, retryable bool) {
	t.Helper()
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != category || problem.Subtype != subtype || problem.Retryable != retryable {
		t.Fatalf("problem = %#v, want category=%q subtype=%q retryable=%v", problem, category, subtype, retryable)
	}
}

func TestDefaultTokenProvider_RetryableTATErrorIsNotCached(t *testing.T) {
	retryableErr := errs.NewAPIError(errs.SubtypeRateLimit, "rate limited").WithRetryable()
	var calls atomic.Int32
	p := NewDefaultTokenProvider(nil, nil, nil)
	p.tatResolver = func(context.Context) (*TokenResult, error) {
		if calls.Add(1) == 1 {
			return nil, retryableErr
		}
		return &TokenResult{Token: "recovered-token"}, nil
	}

	if result, err := p.resolveTAT(context.Background()); result != nil || !errors.Is(err, retryableErr) {
		t.Fatalf("first resolution = (%v, %v), want retryable error", result, err)
	} else {
		requireTATProblem(t, err, errs.CategoryAPI, errs.SubtypeRateLimit, true)
	}
	result, err := p.resolveTAT(context.Background())
	if err != nil {
		t.Fatalf("second resolution returned error: %v", err)
	}
	if result == nil || result.Token != "recovered-token" {
		t.Fatalf("second resolution = %#v, want recovered token", result)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("resolver calls = %d, want 2", got)
	}
}

func TestDefaultTokenProvider_ConcurrentRetryableTATFlightIsSharedThenRetried(t *testing.T) {
	const callers = 16
	retryableErr := errs.NewAPIError(errs.SubtypeRateLimit, "rate limited").WithRetryable()
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	p := NewDefaultTokenProvider(nil, nil, nil)
	p.tatResolver = func(context.Context) (*TokenResult, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			return nil, retryableErr
		}
		return &TokenResult{Token: "recovered-token"}, nil
	}

	type outcome struct {
		result *TokenResult
		err    error
	}
	outcomes := make(chan outcome, callers)
	go func() {
		result, err := p.resolveTAT(context.Background())
		outcomes <- outcome{result: result, err: err}
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first TAT resolution to start")
	}

	for i := 1; i < callers; i++ {
		go func() {
			result, err := p.resolveTAT(context.Background())
			outcomes <- outcome{result: result, err: err}
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		p.tatMu.Lock()
		followers := p.tatFlight.followers
		p.tatMu.Unlock()
		if followers == callers-1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("in-flight followers = %d, want %d", followers, callers-1)
		}
		runtime.Gosched()
	}

	close(release)
	for i := 0; i < callers; i++ {
		select {
		case got := <-outcomes:
			if got.result != nil || !errors.Is(got.err, retryableErr) {
				t.Fatalf("shared retryable resolution = (%v, %v), want retryable error", got.result, got.err)
			}
			requireTATProblem(t, got.err, errs.CategoryAPI, errs.SubtypeRateLimit, true)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for shared retryable resolution")
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("resolver calls after shared flight = %d, want 1", got)
	}

	result, err := p.resolveTAT(context.Background())
	if err != nil || result == nil || result.Token != "recovered-token" {
		t.Fatalf("later resolution = (%v, %v), want recovered success", result, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("resolver calls after recovery = %d, want 2", got)
	}
}

func TestDefaultTokenProvider_ConcurrentTATResolverPanicUnblocksFollowersAndCleansFlight(t *testing.T) {
	const callers = 16
	panicVal := &struct{ message string }{message: "resolver panic"}
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	p := NewDefaultTokenProvider(nil, nil, nil)
	p.tatResolver = func(context.Context) (*TokenResult, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			panic(panicVal)
		}
		return &TokenResult{Token: "recovered-after-panic"}, nil
	}

	recovered := make(chan any, callers)
	resolveAndRecover := func() {
		defer func() {
			recovered <- recover()
		}()
		_, _ = p.resolveTAT(context.Background())
	}
	go resolveAndRecover()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the panicking TAT resolution to start")
	}

	for i := 1; i < callers; i++ {
		go resolveAndRecover()
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		p.tatMu.Lock()
		followers := p.tatFlight.followers
		p.tatMu.Unlock()
		if followers == callers-1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("in-flight followers = %d, want %d", followers, callers-1)
		}
		runtime.Gosched()
	}

	close(release)
	for i := 0; i < callers; i++ {
		select {
		case got := <-recovered:
			if got != panicVal {
				t.Fatalf("recovered panic = %#v, want %#v", got, panicVal)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for caller to recover resolver panic")
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("resolver calls after panicking flight = %d, want 1", got)
	}

	result, err := p.resolveTAT(context.Background())
	if err != nil || result == nil || result.Token != "recovered-after-panic" {
		t.Fatalf("later resolution = (%v, %v), want recovered success", result, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("resolver calls after panic recovery = %d, want 2", got)
	}
}

func TestDefaultTokenProvider_SuccessfulTATIsCached(t *testing.T) {
	var calls atomic.Int32
	want := &TokenResult{Token: "cached-token"}
	p := NewDefaultTokenProvider(nil, nil, nil)
	p.tatResolver = func(context.Context) (*TokenResult, error) {
		calls.Add(1)
		return want, nil
	}

	for i := 0; i < 2; i++ {
		result, err := p.resolveTAT(context.Background())
		if err != nil || result != want {
			t.Fatalf("resolution %d = (%v, %v), want cached success", i+1, result, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("resolver calls = %d, want 1", got)
	}
}

func TestDefaultTokenProvider_NonRetryableTATErrorIsCached(t *testing.T) {
	wantErr := errs.NewConfigError(errs.SubtypeInvalidClient, "invalid credentials")
	var calls atomic.Int32
	p := NewDefaultTokenProvider(nil, nil, nil)
	p.tatResolver = func(context.Context) (*TokenResult, error) {
		calls.Add(1)
		return nil, wantErr
	}

	for i := 0; i < 2; i++ {
		result, err := p.resolveTAT(context.Background())
		if result != nil || !errors.Is(err, wantErr) {
			t.Fatalf("resolution %d = (%v, %v), want cached error", i+1, result, err)
		}
		requireTATProblem(t, err, errs.CategoryConfig, errs.SubtypeInvalidClient, false)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("resolver calls = %d, want 1", got)
	}
}

func TestDefaultTokenProvider_ContextCancellationIsNotCached(t *testing.T) {
	var calls atomic.Int32
	p := NewDefaultTokenProvider(nil, nil, nil)
	p.tatResolver = func(ctx context.Context) (*TokenResult, error) {
		if calls.Add(1) == 1 {
			return nil, ctx.Err()
		}
		return &TokenResult{Token: "recovered-token"}, nil
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := p.resolveTAT(canceledCtx); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled resolution = (%v, %v), want context.Canceled", result, err)
	}
	result, err := p.resolveTAT(context.Background())
	if err != nil || result == nil || result.Token != "recovered-token" {
		t.Fatalf("resolution after cancellation = (%v, %v), want recovered token", result, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("resolver calls = %d, want 2", got)
	}
}

func TestDefaultTokenProvider_ContextDeadlineIsNotCached(t *testing.T) {
	var calls atomic.Int32
	p := NewDefaultTokenProvider(nil, nil, nil)
	p.tatResolver = func(context.Context) (*TokenResult, error) {
		if calls.Add(1) == 1 {
			return nil, context.DeadlineExceeded
		}
		return &TokenResult{Token: "recovered-token"}, nil
	}

	if result, err := p.resolveTAT(context.Background()); result != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline resolution = (%v, %v), want context.DeadlineExceeded", result, err)
	}
	result, err := p.resolveTAT(context.Background())
	if err != nil || result == nil || result.Token != "recovered-token" {
		t.Fatalf("resolution after deadline = (%v, %v), want recovered token", result, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("resolver calls = %d, want 2", got)
	}
}

func TestDefaultTokenProvider_FollowerObservesOwnContextCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	p := NewDefaultTokenProvider(nil, nil, nil)
	p.tatResolver = func(context.Context) (*TokenResult, error) {
		close(started)
		<-release
		return &TokenResult{Token: "leader-token"}, nil
	}

	type outcome struct {
		result *TokenResult
		err    error
	}
	leaderDone := make(chan outcome, 1)
	go func() {
		result, err := p.resolveTAT(context.Background())
		leaderDone <- outcome{result: result, err: err}
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for leader resolution")
	}

	followerCtx, cancel := context.WithCancel(context.Background())
	followerDone := make(chan error, 1)
	go func() {
		_, err := p.resolveTAT(followerCtx)
		followerDone <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		p.tatMu.Lock()
		followers := p.tatFlight.followers
		p.tatMu.Unlock()
		if followers == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("in-flight followers = %d, want 1", followers)
		}
		runtime.Gosched()
	}

	cancel()
	select {
	case err := <-followerDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("follower error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("follower did not observe its context cancellation")
	}
	close(release)
	select {
	case got := <-leaderDone:
		if got.err != nil || got.result == nil || got.result.Token != "leader-token" {
			t.Fatalf("leader resolution = (%v, %v), want leader-token", got.result, got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for leader resolution")
	}
}

func TestDefaultTokenProvider_ConcurrentTATSuccessCoalesces(t *testing.T) {
	const callers = 16
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	p := NewDefaultTokenProvider(nil, nil, nil)
	p.tatResolver = func(context.Context) (*TokenResult, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return &TokenResult{Token: "shared-token"}, nil
	}

	type outcome struct {
		result *TokenResult
		err    error
	}
	outcomes := make(chan outcome, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	begin := make(chan struct{})
	for i := 0; i < callers; i++ {
		go func() {
			ready.Done()
			<-begin
			result, err := p.resolveTAT(context.Background())
			outcomes <- outcome{result: result, err: err}
		}()
	}
	ready.Wait()
	close(begin)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for TAT resolution to start")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		p.tatMu.Lock()
		followers := p.tatFlight.followers
		p.tatMu.Unlock()
		if followers == callers-1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("in-flight followers = %d, want %d", followers, callers-1)
		}
		runtime.Gosched()
	}
	close(release)

	for i := 0; i < callers; i++ {
		select {
		case got := <-outcomes:
			if got.err != nil || got.result == nil || got.result.Token != "shared-token" {
				t.Fatalf("concurrent resolution = (%v, %v), want shared success", got.result, got.err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for concurrent TAT resolution")
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("resolver calls = %d, want 1", got)
	}
}

// TestClassifyTATResponseCode_InvalidClient_MapsToInvalidClient pins that the
// unified Token Endpoint's OAuth2 invalid_client error surfaces as
// CategoryConfig/InvalidClient — the configured app_id/app_secret cannot mint a
// tenant access token, the same actionable failure the legacy 10003/10014 codes
// produced. The numeric code is intentionally not asserted: the v3 endpoint may
// return invalid_client with no Lark code (code defaults to 0).
func TestClassifyTATResponseCode_InvalidClient_MapsToInvalidClient(t *testing.T) {
	err := classifyTATResponseCode(0, "invalid_client", "client authentication failed", "feishu", "cli_app_x")
	if err == nil {
		t.Fatal("expected non-nil error for invalid_client")
	}
	var cfgErr *errs.ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected *errs.ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Category != errs.CategoryConfig {
		t.Errorf("Category = %q, want %q", cfgErr.Category, errs.CategoryConfig)
	}
	if cfgErr.Subtype != errs.SubtypeInvalidClient {
		t.Errorf("Subtype = %q, want %q", cfgErr.Subtype, errs.SubtypeInvalidClient)
	}
	if cfgErr.Hint == "" {
		t.Error("Hint must be non-empty so the user gets a recovery action")
	}
}

// TestClassifyTATResponseCode_UnauthorizedClient_MapsToInvalidClient pins that
// unauthorized_client is treated as the same credential failure as
// invalid_client.
func TestClassifyTATResponseCode_UnauthorizedClient_MapsToInvalidClient(t *testing.T) {
	err := classifyTATResponseCode(0, "unauthorized_client", "client not authorized", "feishu", "cli_app_x")
	var cfgErr *errs.ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected *errs.ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Subtype != errs.SubtypeInvalidClient {
		t.Errorf("Subtype = %q, want %q", cfgErr.Subtype, errs.SubtypeInvalidClient)
	}
}

// TestClassifyTATResponseCode_OtherErrorFallsThrough pins that OAuth errors
// outside the credential set fall through to the generic BuildAPIError fallback
// — still typed, but not a ConfigError. The mapping is narrow and intentional.
func TestClassifyTATResponseCode_OtherErrorFallsThrough(t *testing.T) {
	err := classifyTATResponseCode(20068, "invalid_scope", "unauthorized scope", "feishu", "cli_app_x")
	if err == nil {
		t.Fatal("expected non-nil error for invalid_scope")
	}
	var cfgErr *errs.ConfigError
	if errors.As(err, &cfgErr) {
		t.Fatalf("invalid_scope must not be classified as ConfigError, got %T", err)
	}
}

// TestClassifyTATResponseCode_CodeZeroOtherError_StillTyped pins the code-0
// backstop: a non-credential OAuth error (e.g. invalid_scope) that arrives with no
// numeric code (code 0) must still produce a non-nil typed error. BuildAPIError
// returns nil for code 0 (Feishu's success convention); without the backstop,
// FetchTAT would surface this deterministic rejection as ("", nil) — an empty token
// with no error.
func TestClassifyTATResponseCode_CodeZeroOtherError_StillTyped(t *testing.T) {
	err := classifyTATResponseCode(0, "invalid_scope", "the requested scope is not granted", "feishu", "cli_app_x")
	if err == nil {
		t.Fatal("expected non-nil error for code-0 invalid_scope (must not be swallowed as success)")
	}
	if !errs.IsTyped(err) {
		t.Fatalf("expected a typed errs.* error, got %T %v", err, err)
	}
	var cfgErr *errs.ConfigError
	if errors.As(err, &cfgErr) {
		t.Fatalf("code-0 invalid_scope must not be a ConfigError, got %T", err)
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

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
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/client"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/credential"
)

// staticTokenResolver always returns a fixed token without any HTTP call.
type staticTokenResolver struct{}

func (s *staticTokenResolver) ResolveToken(_ context.Context, _ credential.TokenSpec) (*credential.TokenResult, error) {
	return &credential.TokenResult{Token: "test-token"}, nil
}

// stubRoundTripper intercepts every outgoing request with a canned response.
type stubRoundTripper struct {
	respond func(*http.Request) (*http.Response, error)
}

func (s stubRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return s.respond(r) }

// newTestCmdRuntime builds a cmdRuntime whose client routes every request through
// rt (mirrors cmd/event/runtime_test.go's consumeRuntime harness). Identity is
// pinned to as; agentID is fixed.
func newTestCmdRuntime(rt http.RoundTripper, as core.Identity, agentID string) *cmdRuntime {
	sdk := lark.NewClient("test-app", "test-secret",
		lark.WithEnableTokenCache(false),
		lark.WithLogLevel(larkcore.LogLevelError),
		lark.WithHttpClient(&http.Client{Transport: rt}),
	)
	return &cmdRuntime{
		client: &client.APIClient{
			SDK:        sdk,
			ErrOut:     io.Discard,
			Credential: credential.NewCredentialProvider(nil, nil, &staticTokenResolver{}, nil),
			Config:     &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu},
		},
		as:      as,
		agentID: agentID,
	}
}

func jsonResponse(status int, body string) func(*http.Request) (*http.Response, error) {
	return func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	}
}

// TestCmdRuntime_IdentityAndAgentID pins invariant #4: the resolved identity is
// surfaced only via IsBot() (never the raw client), and AgentID echoes the
// addressed agent.
func TestCmdRuntime_IdentityAndAgentID(t *testing.T) {
	bot := newTestCmdRuntime(stubRoundTripper{}, core.AsBot, "agt_1")
	if !bot.IsBot() {
		t.Error("bot runtime should report IsBot()=true")
	}
	if bot.AgentID() != "agt_1" {
		t.Errorf("AgentID should be agt_1, got %q", bot.AgentID())
	}
	usr := newTestCmdRuntime(stubRoundTripper{}, core.AsUser, "agt_2")
	if usr.IsBot() {
		t.Error("user runtime should report IsBot()=false")
	}
}

// TestCmdRuntime_CallAPI_UnwrapsData pins do(): a 200 OAPI envelope with code=0
// returns the decoded "data" object (not the whole envelope).
func TestCmdRuntime_CallAPI_UnwrapsData(t *testing.T) {
	rt := stubRoundTripper{respond: jsonResponse(200, `{"code":0,"msg":"ok","data":{"task_id":"t1","state":"completed"}}`)}
	r := newTestCmdRuntime(rt, core.AsBot, "agt_1")

	data, err := r.CallAPI(context.Background(), "GET", "/open-apis/example/v1/tasks/t1", nil, nil)
	if err != nil {
		t.Fatalf("CallAPI should succeed: %v", err)
	}
	if data["task_id"] != "t1" || data["state"] != "completed" {
		t.Errorf("CallAPI should return the unwrapped data object, got %+v", data)
	}
}

// TestCmdRuntime_CallAPI_APIError pins that a non-zero code becomes a typed error
// (CheckResponse), not a silent success.
func TestCmdRuntime_CallAPI_APIError(t *testing.T) {
	rt := stubRoundTripper{respond: jsonResponse(200, `{"code":1254043,"msg":"task not found"}`)}
	r := newTestCmdRuntime(rt, core.AsBot, "agt_1")

	if _, err := r.CallAPI(context.Background(), "GET", "/open-apis/example/v1/tasks/nope", nil, nil); err == nil {
		t.Fatal("a non-zero API code should surface as an error")
	} else if _, ok := errs.ProblemOf(err); !ok {
		t.Fatalf("API error should be a typed errs error, got %T: %v", err, err)
	}
}

// TestCmdRuntime_CallAPI_TransportError pins the transport-error branch: a
// RoundTrip failure is classified as a network transport error.
func TestCmdRuntime_CallAPI_TransportError(t *testing.T) {
	rt := stubRoundTripper{respond: func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial refused")
	}}
	r := newTestCmdRuntime(rt, core.AsBot, "agt_1")

	_, err := r.CallAPI(context.Background(), "POST", "/open-apis/example/v1/messages", nil, map[string]any{"text": "hi"})
	if err == nil {
		t.Fatal("a transport error should propagate")
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Category != errs.CategoryNetwork {
		t.Fatalf("transport error should be a network error, got %+v", p)
	}
}

// TestCmdRuntime_CallMultipart_RejectsUnsafePath pins invariant #5: CallMultipart
// SafeInputPath-validates every --file BEFORE opening it, so an absolute /
// traversal path is rejected as invalid_argument (param --file) and NO request
// is issued (the transport panics if reached).
func TestCmdRuntime_CallMultipart_RejectsUnsafePath(t *testing.T) {
	rt := stubRoundTripper{respond: func(*http.Request) (*http.Response, error) {
		t.Fatal("no request should be issued when the --file path is unsafe")
		return nil, nil
	}}
	r := newTestCmdRuntime(rt, core.AsBot, "agt_1")

	for _, bad := range []string{"/etc/hosts", "../../etc/passwd"} {
		_, err := r.CallMultipart(context.Background(), "POST", "/open-apis/example/v1/attachments",
			map[string]string{"type": "file"},
			[]iagent.FilePart{{Field: "file", Path: bad}})
		if err == nil {
			t.Fatalf("an unsafe --file path %q should be rejected", bad)
		}
		if !errs.IsValidation(err) {
			t.Fatalf("unsafe path %q should be a validation error, got %T: %v", bad, err, err)
		}
		var ve *errs.ValidationError
		if !errors.As(err, &ve) || ve.Param != "--file" {
			t.Errorf("unsafe path %q should carry param --file, got %+v", bad, ve)
		}
	}
}

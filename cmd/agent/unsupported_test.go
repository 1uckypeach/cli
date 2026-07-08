// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
)

// unsupProvider is a stub provider driving the slice-2 command-layer wirings
// without any HTTP: ListContexts/DeleteContext return the bare
// agent.ErrUnsupported sentinel (convertUnsupported must promote it to the
// typed unsupported_capability), and GetTask returns a task whose IsTerminal
// deliberately mismatches its State (normalizeTask must re-derive it). Any
// other Provider method call panics via the nil embedded interface — a
// tripwire against the test reaching an unexpected path.
type unsupProvider struct{ iagent.Provider }

func (p *unsupProvider) Card(ctx context.Context) (*iagent.AgentCard, error) {
	return iagent.NewCard("fakeunsup", "a1"), nil
}

func (p *unsupProvider) ListContexts(ctx context.Context) ([]iagent.ContextSummary, error) {
	return nil, iagent.ErrUnsupported
}

func (p *unsupProvider) DeleteContext(ctx context.Context, ctxID string) error {
	return iagent.ErrUnsupported
}

func (p *unsupProvider) GetTask(ctx context.Context, taskID string) (*iagent.AgentTask, error) {
	// Deliberate mismatch: State is terminal but IsTerminal=false (simulating a
	// provider that forgot to set it or set it wrong).
	return &iagent.AgentTask{TaskID: taskID, State: iagent.StateCompleted, IsTerminal: false}, nil
}

// registerFakeUnsup registers the fakeunsup scheme exactly once (Register
// panics on duplicates). Like fakedisc/fakepause it leaks into the
// package-level registry for the remaining tests of this package run.
var registerFakeUnsupOnce sync.Once

func registerFakeUnsup() {
	registerFakeUnsupOnce.Do(func() {
		iagent.Register("fakeunsup", iagent.ProviderInfo{
			Factory:        func(deps iagent.Deps, agentID string) (iagent.Provider, error) { return &unsupProvider{}, nil },
			Label:          "test fake (ErrUnsupported)",
			AgentRefFormat: "fakeunsup:<agent_id>",
			AgentIDSource:  "test only",
			Kind:           iagent.KindInstance,
			Identities:     []iagent.IdentitySpec{{Type: iagent.IdentityUser}, {Type: iagent.IdentityBot}},
		})
	})
}

// assertUnsupportedCapability pins the full convertUnsupported contract on err:
// validation typed, subtype unsupported_capability, exit 2, hint pointing at
// `agent card <ref>`, and — because the Factory's httpmock registry has zero
// stubs — no HTTP was issued (any network attempt would have surfaced as an
// "httpmock: no stub" error instead of the typed one).
func assertUnsupportedCapability(t *testing.T, err error, ref string) {
	t.Helper()
	if err == nil {
		t.Fatal("a provider returning ErrUnsupported should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("want validation error, got %T (%v)", err, err)
	}
	if code := output.ExitCodeOf(err); code != output.ExitValidation {
		t.Fatalf("exit code should be %d, got %d", output.ExitValidation, code)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.SubtypeUnsupportedCapability {
		t.Fatalf("subtype should be unsupported_capability, got %+v", p)
	}
	if !strings.Contains(p.Hint, "agent card "+ref) {
		t.Errorf("hint should point to agent card %s, got %q", ref, p.Hint)
	}
	if strings.Contains(err.Error(), "httpmock") {
		t.Errorf("should not issue any HTTP request, but the error contains httpmock traces: %v", err)
	}
}

// TestContextListConvertsErrUnsupported pins the ErrUnsupported wiring on
// `context list`: a provider sentinel maps to typed unsupported_capability
// (exit 2) with the agent-card hint, without any HTTP.
func TestContextListConvertsErrUnsupported(t *testing.T) {
	registerFakeUnsup()
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	opts := &contextOptions{
		Factory: f, Cmd: contextCmdCtx(t, "list"), Ref: "fakeunsup:a1", As: "bot", Format: "json",
	}
	assertUnsupportedCapability(t, agentContextListRun(opts), "fakeunsup:a1")
}

// TestContextDeleteConvertsErrUnsupported pins the same wiring on the confirmed
// `context delete` path (--yes passes, provider then returns the sentinel).
func TestContextDeleteConvertsErrUnsupported(t *testing.T) {
	registerFakeUnsup()
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	opts := &contextOptions{
		Factory: f, Cmd: contextCmdCtx(t, "delete"), Ref: "fakeunsup:a1", CtxID: "c1", Yes: true, As: "bot", Format: "json",
	}
	assertUnsupportedCapability(t, agentContextDeleteRun(opts), "fakeunsup:a1")
}

// TestConvertUnsupported_Passthrough pins the transparency contract: nil and
// non-sentinel errors (including already-typed ones) pass through unchanged, so
// wrapping every provider call site is side-effect free.
func TestConvertUnsupported_Passthrough(t *testing.T) {
	if err := convertUnsupported("example:agt_x", "send", nil); err != nil {
		t.Errorf("nil should pass through unchanged, got %v", err)
	}
	plain := errors.New("boom")
	if err := convertUnsupported("example:agt_x", "send", plain); err != plain {
		t.Errorf("a non-sentinel error should pass through unchanged, got %v", err)
	}
	typed := errs.NewValidationError(errs.SubtypeInvalidArgument, "bad input")
	if err := convertUnsupported("example:agt_x", "send", typed); !errors.Is(err, typed) {
		t.Errorf("an already-typed error should pass through unchanged, got %v", err)
	}
	// A wrapped sentinel must also match (errors.Is semantics).
	wrapped := errs.NewInternalError(errs.SubtypeUnknown, "wrapped").WithCause(iagent.ErrUnsupported)
	converted := convertUnsupported("example:agt_x", "send", wrapped)
	p, ok := errs.ProblemOf(converted)
	if !ok || p.Subtype != errs.SubtypeUnsupportedCapability {
		t.Errorf("a wrapped sentinel should also convert to unsupported_capability, got %+v", p)
	}
}

// TestTaskGetDerivesIsTerminalFromState pins the normalizeTask wiring: a
// provider returning a State/IsTerminal-mismatched task (completed +
// is_terminal=false) must emit is_terminal=true — the command layer derives
// the flag from State, the single source of truth.
func TestTaskGetDerivesIsTerminalFromState(t *testing.T) {
	registerFakeUnsup()
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	opts := &taskOptions{
		Factory: f, Cmd: taskCmdCtx(t, "get"), Ref: "fakeunsup:a1", TaskID: "t1", As: "bot", Format: "json",
	}
	out := f.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentTaskGetRun(opts); err != nil {
		t.Fatalf("task get should not error: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("output should be valid envelope JSON: %v (%s)", err, string(out.Bytes()))
	}
	data, _ := env.Data.(map[string]interface{})
	if data["state"] != "completed" {
		t.Fatalf("data.state should be completed, got %v", data["state"])
	}
	if data["is_terminal"] != true {
		t.Errorf("is_terminal should be derived from State as true (correcting a provider that set false), got %v", data["is_terminal"])
	}
}

// TestNormalizeTaskSummaries_DerivesFromState pins the summary-side derivation
// (task list / context get share this helper for their nested Tasks).
func TestNormalizeTaskSummaries_DerivesFromState(t *testing.T) {
	ts := normalizeTaskSummaries([]iagent.TaskSummary{
		{TaskID: "t1", State: iagent.StateCompleted, IsTerminal: false}, // missing
		{TaskID: "t2", State: iagent.StateWorking, IsTerminal: true},    // wrong
	})
	if !ts[0].IsTerminal {
		t.Error("completed summary should derive is_terminal=true")
	}
	if ts[1].IsTerminal {
		t.Error("working summary should derive is_terminal=false")
	}
	if normalizeTask(nil) != nil {
		t.Error("normalizeTask(nil) should be nil-safe")
	}
}

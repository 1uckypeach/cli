// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
)

// newUnsupProvider builds a stub *Provider driving the command-layer
// capability-gate wirings without any HTTP: ListContexts / DeleteContext are
// left UNWIRED (nil), so the command layer's nil-gate must return the typed
// unsupported_capability before any network access. GetTask is wired to return a
// task whose IsTerminal deliberately mismatches its State (normalizeTask must
// re-derive it). Send is wired (core, required by Register) but never called
// here. There is no capability-refusal code in the provider — "unsupported" is
// expressed purely by the absent fields.
func newUnsupProvider() *iagent.Provider {
	return &iagent.Provider{
		Send: func(ctx context.Context, in iagent.SendInput) (*iagent.AgentTask, error) {
			panic("unsup provider: Send should not be called")
		},
		GetTask: func(ctx context.Context, taskID string) (*iagent.AgentTask, error) {
			// Deliberate mismatch: State is terminal but IsTerminal=false (simulating
			// a provider that forgot to set it or set it wrong).
			return &iagent.AgentTask{TaskID: taskID, State: iagent.StateCompleted, IsTerminal: false}, nil
		},
		// ListContexts / DeleteContext intentionally unwired ⇒ unsupported.
	}
}

// registerFakeUnsup registers the fakeunsup scheme exactly once (Register
// panics on duplicates). Like the other fakes it leaks into the package-level
// registry for the remaining tests of this package run.
var registerFakeUnsupOnce sync.Once

func registerFakeUnsup() {
	registerFakeUnsupOnce.Do(func() {
		iagent.Register("fakeunsup", iagent.ProviderInfo{
			Factory:        func(deps iagent.Deps, agentID string) (*iagent.Provider, error) { return newUnsupProvider(), nil },
			Label:          "test fake (unwired optional capabilities)",
			AgentRefFormat: "fakeunsup:<agent_id>",
			AgentIDSource:  "test only",
			Kind:           iagent.KindInstance,
			Identities:     []iagent.IdentitySpec{{Type: iagent.IdentityUser}, {Type: iagent.IdentityBot}},
		})
	})
}

// assertUnsupportedCapability pins the full capability-gate contract on err:
// validation typed, subtype unsupported_capability, exit 2, hint pointing at
// `agent card <ref>`, and — because the Factory's httpmock registry has zero
// stubs — no HTTP was issued (any network attempt would have surfaced as an
// "httpmock: no stub" error instead of the typed one).
func assertUnsupportedCapability(t *testing.T, err error, ref string) {
	t.Helper()
	if err == nil {
		t.Fatal("an unsupported capability should error")
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

// TestContextListUnsupportedGated pins the capability gate on `context list`: a
// provider that does not wire ListContexts returns typed unsupported_capability
// (exit 2) with the agent-card hint, without any HTTP.
func TestContextListUnsupportedGated(t *testing.T) {
	registerFakeUnsup()
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	opts := &contextOptions{
		Factory: f, Cmd: contextCmdCtx(t, "list"), Ref: "fakeunsup:a1", As: "bot", Format: "json",
	}
	assertUnsupportedCapability(t, agentContextListRun(opts), "fakeunsup:a1")
}

// TestContextDeleteUnsupportedGated pins the same gate on the confirmed
// `context delete` path (--yes passes, provider does not wire DeleteContext).
func TestContextDeleteUnsupportedGated(t *testing.T) {
	registerFakeUnsup()
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	opts := &contextOptions{
		Factory: f, Cmd: contextCmdCtx(t, "delete"), Ref: "fakeunsup:a1", CtxID: "c1", Yes: true, As: "bot", Format: "json",
	}
	assertUnsupportedCapability(t, agentContextDeleteRun(opts), "fakeunsup:a1")
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

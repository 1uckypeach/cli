// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	extcs "github.com/larksuite/cli/extension/contentsafety"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
)

func TestValidateParamsAgainstCard(t *testing.T) {
	// Card mixes a required and an optional param so both loop branches run:
	// the optional param must be skipped (the `!p.Required continue` path) while
	// the required one is still enforced.
	card := &iagent.AgentCard{Parameters: []iagent.CardParam{
		{Name: "app_id", Required: true},
		{Name: "locale", Required: false},
	}}
	// missing required
	if _, err := parseAndValidateParams([]string{}, card, "example:agt_x"); err == nil {
		t.Error("missing required app_id should error")
	}
	// provide required, omit optional: the optional param is skipped and must not error
	m, err := parseAndValidateParams([]string{"app_id=app_sales"}, card, "example:agt_x")
	if err != nil || m["app_id"] != "app_sales" {
		t.Fatalf("should parse app_id and allow omitting optional locale: %v %v", m, err)
	}
	if _, ok := m["locale"]; ok {
		t.Errorf("an optional param that was not provided should not appear in the result: %v", m)
	}
	// invalid format
	if _, err := parseAndValidateParams([]string{"noequals"}, card, "example:agt_x"); err == nil {
		t.Error("--param without = should error")
	}
}

// TestParseParams_ValueWithEquals ensures values may themselves contain '='
// (only the first '=' splits key from value).
func TestParseParams_ValueWithEquals(t *testing.T) {
	card := &iagent.AgentCard{Parameters: []iagent.CardParam{{Name: "filter"}}}
	m, err := parseAndValidateParams([]string{"filter=a=b"}, card, "example:agt_x")
	if err != nil {
		t.Fatalf("a value containing = should not error: %v", err)
	}
	if m["filter"] != "a=b" {
		t.Fatalf("value should preserve =, got %q", m["filter"])
	}
}

// TestParseParams_EmptyKey rejects an empty key (leading '=').
func TestParseParams_EmptyKey(t *testing.T) {
	if _, err := parseAndValidateParams([]string{"=v"}, &iagent.AgentCard{}, "example:agt_x"); err == nil {
		t.Error("empty key should error")
	}
}

// TestParseParams_UnknownKeyRejected pins that a --param key not declared in the
// card's Parameters is a validation error (subtype invalid_argument, param
// "param:<key>") whose hint points at `agent card`; a declared optional key
// still passes.
func TestParseParams_UnknownKeyRejected(t *testing.T) {
	card := &iagent.AgentCard{Parameters: []iagent.CardParam{{Name: "foo"}}}
	m, err := parseAndValidateParams([]string{"foo=1"}, card, "example:agt_x")
	if err != nil || m["foo"] != "1" {
		t.Fatalf("a declared optional param should pass: %v %v", m, err)
	}

	_, err = parseAndValidateParams([]string{"bar=1"}, card, "example:agt_x")
	if err == nil {
		t.Fatal("an undeclared --param should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T", err)
	}
	var verr *errs.ValidationError
	if !errors.As(err, &verr) || verr.Param != "param:bar" {
		t.Fatalf("param should be param:bar, got %+v", verr)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("subtype should be invalid_argument, got %+v", p)
	}
	if !strings.Contains(p.Hint, "agent card example:agt_x") {
		t.Fatalf("hint should point to agent card, got %q", p.Hint)
	}
}

// TestParseParams_NilCard tolerates a nil card (no required/unknown-param check).
func TestParseParams_NilCard(t *testing.T) {
	m, err := parseAndValidateParams([]string{"k=v"}, nil, "example:agt_x")
	if err != nil || m["k"] != "v" {
		t.Fatalf("nil card should parse normally: %v %v", m, err)
	}
}

// TestParseParams_MissingRequiredIsValidation confirms the missing-required
// error is a validation typed error with subtype invalid_argument, its param
// carries the param: prefix, and its hint points at agent card (Task 2 review
// leftover).
func TestParseParams_MissingRequiredIsValidation(t *testing.T) {
	card := &iagent.AgentCard{Parameters: []iagent.CardParam{{Name: "app_id", Required: true}}}
	_, err := parseAndValidateParams([]string{}, card, "example:agt_x")
	if err == nil {
		t.Fatal("missing required should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T", err)
	}
	p, _ := errs.ProblemOf(err)
	if p == nil || p.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("subtype should be invalid_argument, got %+v", p)
	}
	var verr *errs.ValidationError
	if !errors.As(err, &verr) || verr.Param != "param:app_id" {
		t.Fatalf("param should be param:app_id, got %+v", verr)
	}
	if !strings.Contains(p.Hint, "agent card example:agt_x") {
		t.Fatalf("hint should point to agent card, got %q", p.Hint)
	}
}

// TestParseParams_UnsafeRefDegradesHint pins the ref-interpolation whitelist on
// the hint side: a ref that fails the <charset>:<charset> whitelist must not be
// echoed into the hint command; the hint degrades to plain guidance instead.
func TestParseParams_UnsafeRefDegradesHint(t *testing.T) {
	dirtyRef := "example:agt x; rm -rf /"
	card := &iagent.AgentCard{Parameters: []iagent.CardParam{{Name: "app_id", Required: true}}}

	_, err := parseAndValidateParams([]string{}, card, dirtyRef)
	if err == nil {
		t.Fatal("missing required should error")
	}
	p, _ := errs.ProblemOf(err)
	if p == nil || p.Hint == "" {
		t.Fatalf("hint should degrade to plain-text guidance rather than be emptied, got %+v", p)
	}
	if strings.Contains(p.Hint, dirtyRef) {
		t.Fatalf("an unsafe ref must not be interpolated into the hint, got %q", p.Hint)
	}

	// the unknown-param path is handled the same way.
	_, err = parseAndValidateParams([]string{"app_id=1", "bogus=1"}, card, dirtyRef)
	if err == nil {
		t.Fatal("an undeclared param should error")
	}
	p, _ = errs.ProblemOf(err)
	if p == nil || p.Hint == "" || strings.Contains(p.Hint, dirtyRef) {
		t.Fatalf("unknown-param hint should degrade and not contain the unsafe ref, got %+v", p)
	}
}

// TestCapabilityError_UnsafeRefDegradesHint pins the same whitelist on the
// capability-gate hint: an unsafe ref degrades the hint to plain guidance.
func TestCapabilityError_UnsafeRefDegradesHint(t *testing.T) {
	err := capabilityError("example:agt x", "task cancel", iagent.CapTaskCancel)
	p, ok := errs.ProblemOf(err)
	if !ok || p.Hint == "" {
		t.Fatalf("hint should degrade to plain-text guidance rather than be emptied, got %+v", p)
	}
	if strings.Contains(p.Hint, "example:agt x") {
		t.Fatalf("an unsafe ref must not be interpolated into the hint, got %q", p.Hint)
	}
}

// TestCapabilityError pins the unsupported_capability contract.
func TestCapabilityError(t *testing.T) {
	err := capabilityError("example:agt_xxx", "task cancel", iagent.CapTaskCancel)
	if err == nil {
		t.Fatal("should return an error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T", err)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.Subtype("unsupported_capability") {
		t.Fatalf("subtype should be unsupported_capability, got %+v", p)
	}
	if output.ExitCodeOf(err) != output.ExitValidation {
		t.Fatalf("exit should be %d, got %d", output.ExitValidation, output.ExitCodeOf(err))
	}
}

// TestSemanticExitError maps terminal task states to the wait/watch exit code.
func TestSemanticExitError(t *testing.T) {
	cases := []struct {
		state    iagent.TaskState
		wantExit int
	}{
		{iagent.StateCompleted, output.ExitOK},
		{iagent.StateFailed, 1},
		{iagent.StateRejected, 1},
		{iagent.StateCanceled, 1},
		{iagent.StateInputRequired, output.ExitOK}, // non-terminal, not treated as failure
		{iagent.StateWorking, output.ExitOK},
	}
	for _, c := range cases {
		task := &iagent.AgentTask{State: c.state, IsTerminal: c.state.IsTerminal()}
		err := semanticExitError(task)
		if got := output.ExitCodeOf(err); got != c.wantExit {
			t.Errorf("state=%s exit expected %d got %d (err=%v)", c.state, c.wantExit, got, err)
		}
	}
	// nil task should not panic and is treated as success
	if err := semanticExitError(nil); err != nil {
		t.Errorf("nil task should return nil, got %v", err)
	}
}

// fakePollProvider drives pollToStop through a scripted state sequence. getTask
// is the closure pollToStop takes (spec.GetTask bound to a runtime in
// production); calls/err stay observable on the struct after the poll.
type fakePollProvider struct {
	states []iagent.TaskState
	calls  int
	err    error
}

func (f *fakePollProvider) getTask(ctx context.Context, taskID string) (*iagent.AgentTask, error) {
	if f.err != nil {
		return nil, f.err
	}
	i := f.calls
	if i >= len(f.states) {
		i = len(f.states) - 1
	}
	f.calls++
	s := f.states[i]
	return &iagent.AgentTask{TaskID: taskID, State: s, IsTerminal: s.IsTerminal()}, nil
}

// TestPollToStop_ReachesTerminal stops once a terminal state is observed.
func TestPollToStop_ReachesTerminal(t *testing.T) {
	restore := swapSleep()
	defer restore()

	p := &fakePollProvider{states: []iagent.TaskState{iagent.StateWorking, iagent.StateWorking, iagent.StateCompleted}}
	task, err := pollToStop(context.Background(), p.getTask, "chat_1")
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if task == nil || task.State != iagent.StateCompleted {
		t.Fatalf("should stop at completed, got %+v", task)
	}
	if p.calls < 3 {
		t.Fatalf("should poll at least 3 times, got %d", p.calls)
	}
}

// TestPollToStop_StopsOnInputRequired treats input_required as a stop point.
func TestPollToStop_StopsOnInputRequired(t *testing.T) {
	restore := swapSleep()
	defer restore()

	p := &fakePollProvider{states: []iagent.TaskState{iagent.StateWorking, iagent.StateInputRequired}}
	task, err := pollToStop(context.Background(), p.getTask, "chat_1")
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if task.State != iagent.StateInputRequired {
		t.Fatalf("should stop at input_required, got %s", task.State)
	}
}

// TestPollToStop_ContextTimeoutNotFailure confirms that timeout returns the
// current task with a nil error (exit 0), not a failure.
func TestPollToStop_ContextTimeoutNotFailure(t *testing.T) {
	restore := swapSleep()
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // expire immediately
	p := &fakePollProvider{states: []iagent.TaskState{iagent.StateWorking}}
	task, err := pollToStop(ctx, p.getTask, "chat_1")
	if err != nil {
		t.Fatalf("timeout should not be treated as failure: %v", err)
	}
	if task == nil || task.State != iagent.StateWorking {
		t.Fatalf("timeout should return the current task, got %+v", task)
	}
}

// TestPollToStop_GetTaskError surfaces a provider error.
func TestPollToStop_GetTaskError(t *testing.T) {
	restore := swapSleep()
	defer restore()

	p := &fakePollProvider{states: []iagent.TaskState{iagent.StateWorking}, err: errors.New("boom")}
	if _, err := pollToStop(context.Background(), p.getTask, "chat_1"); err == nil {
		t.Fatal("a GetTask error should propagate")
	}
}

// swapSleep replaces the package sleep with a no-op for fast tests.
func swapSleep() func() {
	orig := sleep
	sleep = func(context.Context, time.Duration) bool { return true }
	return func() { sleep = orig }
}

// swapSleepCapture replaces the package sleep with a no-op that records every
// backoff duration it was asked to wait, so tests can assert the exponential /
// clamp schedule. It always returns true (full duration elapsed).
func swapSleepCapture(delays *[]time.Duration) func() {
	orig := sleep
	sleep = func(_ context.Context, d time.Duration) bool {
		*delays = append(*delays, d)
		return true
	}
	return func() { sleep = orig }
}

// swapSleepFalseAt replaces the package sleep with a no-op that returns false
// (as if ctx were canceled during backoff) on the falseCall-th invocation
// (1-indexed) and true otherwise. Lets tests exercise the sleep-returns-false
// branch in isolation without racing a real ctx timeout.
func swapSleepFalseAt(falseCall int) func() {
	orig := sleep
	n := 0
	sleep = func(context.Context, time.Duration) bool {
		n++
		return n != falseCall
	}
	return func() { sleep = orig }
}

// TestPollToStop_ClampsDelayToMax drives >=4 backoff rounds so the exponential
// delay overshoots the 5s cap and the clamp branch (line 179) executes. The
// captured schedule must never exceed maxDelay and must actually reach it.
func TestPollToStop_ClampsDelayToMax(t *testing.T) {
	var delays []time.Duration
	restore := swapSleepCapture(&delays)
	defer restore()

	// 5 Working states then Completed: forces backoff 1s,2s,4s,5s(clamped),5s...
	p := &fakePollProvider{states: []iagent.TaskState{
		iagent.StateWorking, iagent.StateWorking, iagent.StateWorking,
		iagent.StateWorking, iagent.StateWorking, iagent.StateCompleted,
	}}
	task, err := pollToStop(context.Background(), p.getTask, "chat_1")
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if task == nil || task.State != iagent.StateCompleted {
		t.Fatalf("should stop at completed, got %+v", task)
	}
	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	if len(delays) != len(want) {
		t.Fatalf("backoff count should be %d, got %d (%v)", len(want), len(delays), delays)
	}
	for i, d := range delays {
		if d > 5*time.Second {
			t.Errorf("backoff #%d=%v exceeds the 5s cap", i, d)
		}
		if d != want[i] {
			t.Errorf("backoff #%d expected %v got %v", i, want[i], d)
		}
	}
}

// TestPollToStop_SleepCanceledDuringBackoff isolates the sleep-returns-false
// branch (lines 173-177): ctx.Err() is still nil when the loop reaches the
// sleep, but sleep reports the wait was cut short, so pollToStop returns the
// most recent task with a nil error (not a failure).
func TestPollToStop_SleepCanceledDuringBackoff(t *testing.T) {
	restore := swapSleepFalseAt(1) // first backoff sleep is interrupted
	defer restore()

	p := &fakePollProvider{states: []iagent.TaskState{iagent.StateWorking, iagent.StateCompleted}}
	task, err := pollToStop(context.Background(), p.getTask, "chat_1")
	if err != nil {
		t.Fatalf("an interrupted sleep should not be treated as failure: %v", err)
	}
	if task == nil || task.State != iagent.StateWorking {
		t.Fatalf("should return the working task observed before interruption, got %+v", task)
	}
	if p.calls != 1 {
		t.Fatalf("should not poll again after sleep interruption, expected 1 GetTask call got %d", p.calls)
	}
}

// TestJqExpr covers both jqExpr branches: a command with a registered --jq flag
// returns its value; a command without the flag returns "".
func TestJqExpr(t *testing.T) {
	withFlag := &cobra.Command{Use: "get"}
	withFlag.Flags().String("jq", "", "")
	if err := withFlag.Flags().Set("jq", ".state"); err != nil {
		t.Fatal(err)
	}
	if got := jqExpr(withFlag); got != ".state" {
		t.Errorf("with a --jq flag it should return its value, got %q", got)
	}

	noFlag := &cobra.Command{Use: "list"}
	if got := jqExpr(noFlag); got != "" {
		t.Errorf("without a --jq flag it should return empty, got %q", got)
	}
}

// newEmitCmd builds a `lark-cli agent <name>` command whose CommandPath() is
// non-empty (required for content-safety scanning to engage) and optionally
// registers a --jq flag with the given value.
func newEmitCmd(name, jq string) *cobra.Command {
	root := &cobra.Command{Use: "lark-cli"}
	agentGroup := &cobra.Command{Use: "agent"}
	leaf := &cobra.Command{Use: name}
	root.AddCommand(agentGroup)
	agentGroup.AddCommand(leaf)
	if jq != "" {
		leaf.Flags().String("jq", "", "")
		_ = leaf.Flags().Set("jq", jq)
	}
	leaf.SetContext(context.Background())
	return leaf
}

// emitFactory returns a Factory writing to fresh out/err buffers.
func emitFactory() (*cmdutil.Factory, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	f := &cmdutil.Factory{
		IOStreams:        &cmdutil.IOStreams{Out: out, ErrOut: errOut},
		ResolvedIdentity: core.AsBot,
	}
	return f, out, errOut
}

// csProvider is a content-safety provider stub returning a fixed alert.
type csProvider struct{ alert *extcs.Alert }

func (p *csProvider) Name() string { return "test" }
func (p *csProvider) Scan(context.Context, extcs.ScanRequest) (*extcs.Alert, error) {
	return p.alert, nil
}

// TestEmitTask_PlainSuccess emits a task with no jq, no alert: the full envelope
// lands on stdout with ok=true and the identity.
func TestEmitTask_PlainSuccess(t *testing.T) {
	f, out, _ := emitFactory()
	cmd := newEmitCmd("task", "")
	task := &iagent.AgentTask{TaskID: "chat_1", State: iagent.StateCompleted, IsTerminal: true}

	next := []output.NextAction{{Label: "poll", Command: "lark-cli agent task get example:x chat_1"}}
	if err := emitTask(f, cmd, task, next, "json"); err != nil {
		t.Fatalf("emit should not error: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("envelope should be valid JSON: %v (%s)", err, out.String())
	}
	if !env.OK || env.Identity != string(core.AsBot) {
		t.Errorf("ok/identity mismatch: %+v", env)
	}
	if !strings.Contains(out.String(), `"next"`) || !strings.Contains(out.String(), "poll") {
		t.Errorf("meta.next should appear in the output: %s", out.String())
	}
}

// TestEmitTask_NoNextOmitsMeta pins the omitempty branch (common.go line 113):
// when next is nil or an empty (non-nil) slice, emitTask must leave env.Meta nil
// so "meta" is absent from the serialized envelope. Covers both len(next)==0
// inputs the branch can receive.
func TestEmitTask_NoNextOmitsMeta(t *testing.T) {
	for _, tc := range []struct {
		name string
		next []output.NextAction
	}{
		{"nil next", nil},
		{"empty non-nil next", []output.NextAction{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, out, _ := emitFactory()
			cmd := newEmitCmd("task", "")
			task := &iagent.AgentTask{TaskID: "chat_1", State: iagent.StateCompleted, IsTerminal: true}

			if err := emitTask(f, cmd, task, tc.next, "json"); err != nil {
				t.Fatalf("emit should not error: %v", err)
			}
			var env output.Envelope
			if err := json.Unmarshal(out.Bytes(), &env); err != nil {
				t.Fatalf("envelope should be valid JSON: %v (%s)", err, out.String())
			}
			if env.Meta != nil {
				t.Errorf("Meta should be nil when len(next)==0, got %+v", env.Meta)
			}
			if strings.Contains(out.String(), `"meta"`) {
				t.Errorf("meta should be omitted by omitempty when next is empty: %s", out.String())
			}
		})
	}
}

// TestEmitTask_JqFilter routes stdout through a valid jq expression.
func TestEmitTask_JqFilter(t *testing.T) {
	f, out, _ := emitFactory()
	cmd := newEmitCmd("task", ".data.state")
	task := &iagent.AgentTask{TaskID: "chat_1", State: iagent.StateWorking}

	if err := emitTask(f, cmd, task, nil, "json"); err != nil {
		t.Fatalf("jq filtering should not error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "working" {
		t.Errorf("jq .data.state should output working, got %q", got)
	}
}

// TestEmitTask_JqFilterError surfaces a malformed jq expression as an error.
func TestEmitTask_JqFilterError(t *testing.T) {
	f, _, _ := emitFactory()
	cmd := newEmitCmd("task", "{") // unbalanced → gojq.Parse fails
	task := &iagent.AgentTask{TaskID: "chat_1", State: iagent.StateWorking}

	if err := emitTask(f, cmd, task, nil, "json"); err == nil {
		t.Fatal("a malformed jq expression should error")
	}
}

// TestEmitTask_ContentSafetyAlertWarn attaches a warn-mode alert to the envelope
// without blocking output.
func TestEmitTask_ContentSafetyAlertWarn(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONTENT_SAFETY_MODE", "warn")
	extcs.Register(&csProvider{alert: &extcs.Alert{Provider: "test", MatchedRules: []string{"r1"}}})
	defer extcs.Register(nil)

	f, out, _ := emitFactory()
	cmd := newEmitCmd("task", "")
	task := &iagent.AgentTask{TaskID: "chat_1", State: iagent.StateCompleted, IsTerminal: true}

	if err := emitTask(f, cmd, task, nil, "json"); err != nil {
		t.Fatalf("warn mode should not error: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, out.String())
	}
	if env.ContentSafetyAlert == nil {
		t.Error("warn mode should attach the alert to the envelope")
	}
}

// TestEmitTask_ContentSafetyAlertWarnWithJq exercises the WriteAlertWarning +
// JqFilter branch: an alert plus a --jq expression writes a stderr warning and
// still filters stdout.
func TestEmitTask_ContentSafetyAlertWarnWithJq(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONTENT_SAFETY_MODE", "warn")
	extcs.Register(&csProvider{alert: &extcs.Alert{Provider: "test", MatchedRules: []string{"r1"}}})
	defer extcs.Register(nil)

	f, out, errOut := emitFactory()
	cmd := newEmitCmd("task", ".data.state")
	task := &iagent.AgentTask{TaskID: "chat_1", State: iagent.StateWorking}

	if err := emitTask(f, cmd, task, nil, "json"); err != nil {
		t.Fatalf("warn+jq should not error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "working" {
		t.Errorf("jq output should be working, got %q", got)
	}
	if !strings.Contains(errOut.String(), "content safety alert") {
		t.Errorf("stderr should contain a content-safety warning, got %q", errOut.String())
	}
}

// TestEmitTask_ContentSafetyBlocked returns the block error and writes nothing
// to stdout.
func TestEmitTask_ContentSafetyBlocked(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONTENT_SAFETY_MODE", "block")
	extcs.Register(&csProvider{alert: &extcs.Alert{Provider: "test", MatchedRules: []string{"r1"}}})
	defer extcs.Register(nil)

	f, out, _ := emitFactory()
	cmd := newEmitCmd("task", "")
	task := &iagent.AgentTask{TaskID: "chat_1", State: iagent.StateCompleted, IsTerminal: true}

	err := emitTask(f, cmd, task, nil, "json")
	if err == nil {
		t.Fatal("block mode should return BlockErr")
	}
	if !errs.IsContentSafety(err) {
		t.Errorf("should be a content-safety error, got %T", err)
	}
	if out.Len() > 0 {
		t.Errorf("block mode should not write to stdout, got %q", out.String())
	}
}

// resolveCmd builds an `agent card` command carrying an `--as` flag. When
// asChanged is true the flag is marked as explicitly set, so ResolveAs honors
// the passed identity verbatim (needed to exercise the identity-check branch).
func resolveCmd(t *testing.T, asChanged bool, asVal string) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "lark-cli"}
	group := &cobra.Command{Use: "agent"}
	leaf := &cobra.Command{Use: "card"}
	root.AddCommand(group)
	group.AddCommand(leaf)
	leaf.Flags().String("as", "", "identity")
	if asChanged {
		if err := leaf.Flags().Set("as", asVal); err != nil {
			t.Fatal(err)
		}
	}
	leaf.SetContext(context.Background())
	return leaf
}

// TestResolveSpec_Success resolves a valid example ref under an explicit bot
// identity and returns a non-nil spec offline (no client).
func TestResolveSpec_Success(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	cmd := resolveCmd(t, true, "bot")

	prov, spec, agentID, id, err := resolveSpec(f, cmd, "example:echo", "bot")
	if err != nil {
		t.Fatalf("a valid ref + bot should succeed: %v", err)
	}
	if spec == nil || spec.Send == nil {
		t.Fatal("should return a non-nil spec with core hooks")
	}
	if prov.Scheme != "example" || agentID != "echo" {
		t.Errorf("provider/agent id: scheme=%q agentID=%q", prov.Scheme, agentID)
	}
	if id != core.AsBot {
		t.Errorf("identity should be bot, got %s", id)
	}
}

// TestResolveSpec_MalformedRef wraps a ParseRef failure into an
// invalid_argument validation error (exit 2).
func TestResolveSpec_MalformedRef(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	cmd := resolveCmd(t, true, "bot")

	_, _, _, _, err := resolveSpec(f, cmd, "no-colon", "bot")
	if err == nil {
		t.Fatal("malformed ref should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T", err)
	}
	p, _ := errs.ProblemOf(err)
	if p == nil || p.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("subtype should be invalid_argument, got %+v", p)
	}
	// A malformed ref teaches the <scheme>:<agent_id> shape.
	if !strings.Contains(p.Hint, "<scheme>:<agent_id>") {
		t.Errorf("malformed-ref hint should teach the ref shape, got %q", p.Hint)
	}
}

// TestResolveSpec_UnknownScheme rejects an unregistered provider scheme.
func TestResolveSpec_UnknownScheme(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	cmd := resolveCmd(t, true, "bot")

	_, _, _, _, err := resolveSpec(f, cmd, "nope:agt_x", "bot")
	if err == nil {
		t.Fatal("an unknown scheme should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T", err)
	}
	p, _ := errs.ProblemOf(err)
	if p == nil || !strings.Contains(p.Hint, "agent list") {
		t.Errorf("unknown-scheme hint should point to `agent list`, got %+v", p)
	}
}

// TestResolveSpec_UnknownCatalogID rejects an unknown catalog entry id — the
// framework validates it offline (a change from the old construct-only path).
func TestResolveSpec_UnknownCatalogID(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	cmd := resolveCmd(t, true, "bot")

	_, spec, _, _, err := resolveSpec(f, cmd, "example:nope", "bot")
	if err == nil || spec != nil {
		t.Fatal("an unknown catalog id should error with a nil spec")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T", err)
	}
}

// TestResolveSpec_IdentityRejected fails the user|bot whitelist when an
// unsupported --as is explicitly requested; no spec is returned.
func TestResolveSpec_IdentityRejected(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	cmd := resolveCmd(t, true, "admin")

	_, spec, _, _, err := resolveSpec(f, cmd, "example:echo", "admin")
	if err == nil {
		t.Fatal("an unsupported identity should error")
	}
	if spec != nil {
		t.Error("should not return a spec when identity validation fails")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T", err)
	}
}

// TestRuntimeFor_APIClientError surfaces a NewAPIClient failure (Config error).
func TestRuntimeFor_APIClientError(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	f.Config = func() (*core.CliConfig, error) { return nil, errors.New("config boom") }

	if _, err := runtimeFor(f, core.AsBot, "echo"); err == nil {
		t.Fatal("a Config error should propagate")
	}
}

// unconfiguredFactory returns a Factory whose Config() errors (simulating a
// fresh install that hasn't run `config init`), so NewAPIClient fails. Used to
// pin that the API-free paths never reach the config gate.
func unconfiguredFactory(t *testing.T) *cmdutil.Factory {
	t.Helper()
	f, _, _, _ := cmdutil.TestFactory(t, nil)
	f.Config = func() (*core.CliConfig, error) { return nil, errors.New("not configured") }
	return f
}

// TestResolveSpec_WorksWhenUnconfigured guards the acceptance regression: offline
// resolution must NOT touch NewAPIClient, so it succeeds even when Config errors,
// while runtimeFor (the client path) still fails at the config gate.
func TestResolveSpec_WorksWhenUnconfigured(t *testing.T) {
	f := unconfiguredFactory(t)
	cmd := resolveCmd(t, true, "bot")

	_, spec, _, id, err := resolveSpec(f, cmd, "example:echo", "bot")
	if err != nil {
		t.Fatalf("offline resolution should succeed when unconfigured: %v", err)
	}
	if spec == nil || id != core.AsBot {
		t.Fatalf("should return spec + bot identity, got spec=%v id=%s", spec, id)
	}
	if _, err := runtimeFor(f, id, "echo"); err == nil {
		t.Fatal("the client path (runtimeFor) should error when unconfigured (config gate)")
	}
}

// TestResolveSpec_ValidatesRefBeforeConfig pins that a malformed ref / unknown
// scheme is a validation error (exit 2) even when unconfigured — it must not be
// masked by not_configured.
func TestResolveSpec_ValidatesRefBeforeConfig(t *testing.T) {
	f := unconfiguredFactory(t)
	cmd := resolveCmd(t, true, "bot")

	for _, ref := range []string{"no-colon", "nope:agt_x"} {
		_, _, _, _, err := resolveSpec(f, cmd, ref, "bot")
		if err == nil {
			t.Fatalf("ref %q should also report a validation error when unconfigured", ref)
		}
		if !errs.IsValidation(err) {
			t.Fatalf("ref %q should be a validation error, got %T", ref, err)
		}
	}
}

// TestAgentCardRun_WorksUnconfigured guards the acceptance regression: `agent
// card` is statically synthesized and must succeed unconfigured, never hitting
// the config gate.
func TestAgentCardRun_WorksUnconfigured(t *testing.T) {
	f := unconfiguredFactory(t)
	cmd := resolveCmd(t, true, "bot")

	if err := agentCardRun(&cardOptions{Factory: f, Cmd: cmd, Ref: "example:echo", As: "bot", Format: "json"}); err != nil {
		t.Fatalf("agent card should succeed when unconfigured (API-free): %v", err)
	}
}

// TestAgentSendRun_DryRunWorksUnconfigured guards the acceptance regression:
// `agent send --dry-run` is a client-side preview and must succeed
// unconfigured — the example echo card declares no parameters, so no --param is
// needed. A malformed --param must still surface as validation, unconfigured.
func TestAgentSendRun_DryRunWorksUnconfigured(t *testing.T) {
	f := unconfiguredFactory(t)
	cmd := resolveCmd(t, true, "bot")

	err := agentSendRun(&sendOptions{
		Factory: f, Cmd: cmd, Ref: "example:echo", Text: "hi", DryRun: true, As: "bot",
	})
	if err != nil {
		t.Fatalf("send --dry-run should succeed when unconfigured: %v", err)
	}

	// A malformed --param (no '=') is still a validation error, unconfigured.
	err = agentSendRun(&sendOptions{
		Factory: f, Cmd: cmd, Ref: "example:echo", Text: "hi",
		Params: []string{"noequals"}, DryRun: true, As: "bot",
	})
	if err == nil || !errs.IsValidation(err) {
		t.Fatalf("a malformed --param should report a validation error when unconfigured, got %v", err)
	}
}

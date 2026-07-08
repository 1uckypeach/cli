// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
)

// sendCmdCtx builds a `lark-cli agent send` leaf command whose CommandPath() is
// non-empty (required for content-safety scanning) and whose --as flag is
// explicitly set to bot so ResolveAs honors it verbatim.
func sendCmdCtx(t *testing.T) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "lark-cli"}
	group := &cobra.Command{Use: "agent"}
	leaf := &cobra.Command{Use: "send"}
	root.AddCommand(group)
	group.AddCommand(leaf)
	leaf.Flags().String("as", "", "identity")
	if err := leaf.Flags().Set("as", "bot"); err != nil {
		t.Fatal(err)
	}
	leaf.SetContext(context.Background())
	return leaf
}

// sendTestOpts wires a sendOptions against a real (test) Factory, addressing
// the scripted fakeflow agent agt_x under an explicit bot identity. The
// Factory's httpmock registry holds zero stubs, so any HTTP attempt fails the
// test — everything under test here is command-layer behavior over the
// scripted provider.
func sendTestOpts(t *testing.T) *sendOptions {
	t.Helper()
	registerScripted()
	cfg := &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu}
	f, _, _, _ := cmdutil.TestFactory(t, cfg)
	return &sendOptions{
		Factory: f,
		Cmd:     sendCmdCtx(t),
		Ref:     "fakeflow:agt_x",
		As:      "bot",
	}
}

// TestSendRequiresText pins that an empty --text is a validation error
// (subtype invalid_argument) raised before any provider is built.
func TestSendRequiresText(t *testing.T) {
	err := agentSendRun(&sendOptions{Ref: "example:agt_x", Text: ""})
	if err == nil {
		t.Fatal("missing --text should raise a validation error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("want validation error, got %T", err)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("subtype should be invalid_argument, got %+v", p)
	}
	// hint contract: a missing --text must carry a copy-pasteable remediation
	// hint, and the param uses the -- prefix.
	if !strings.Contains(p.Hint, "--text") {
		t.Errorf("hint should guide adding --text, got %q", p.Hint)
	}
	var verr *errs.ValidationError
	if !errors.As(err, &verr) || verr.Param != "--text" {
		t.Errorf("param should be --text, got %+v", verr)
	}
}

// TestSendTaskIDRequiresContextID pins that --task-id without --context-id is a
// validation error, raised before any provider is built.
func TestSendTaskIDRequiresContextID(t *testing.T) {
	err := agentSendRun(&sendOptions{Ref: "example:agt_x", Text: "x", TaskID: "t1"})
	if err == nil {
		t.Fatal("--task-id without --context-id should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("want validation error, got %T", err)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("subtype should be invalid_argument, got %+v", p)
	}
	// hint contract: state the next step clearly (--task-id must be provided
	// together with --context-id).
	if !strings.Contains(p.Hint, "--context-id") {
		t.Errorf("hint should note it must be used with --context-id, got %q", p.Hint)
	}
	var verr *errs.ValidationError
	if !errors.As(err, &verr) || verr.Param != "--task-id" {
		t.Errorf("param should be --task-id, got %+v", verr)
	}
}

// workingTask is the canonical non-terminal task the scripted Send returns for
// the happy-path tests.
func workingTask() *iagent.AgentTask {
	return &iagent.AgentTask{TaskID: "chat_1", ContextID: "sess_1", State: iagent.StateWorking}
}

// TestSendPrettyFormat pins that `send --format pretty` renders the
// resulting task as key: value lines (previously the flag was registered but
// silently ignored).
func TestSendPrettyFormat(t *testing.T) {
	opts := sendTestOpts(t)
	opts.Text = "分析销售"
	opts.Format = "pretty"
	setScripted(t, scriptedHooks{send: func(iagent.SendInput) (*iagent.AgentTask, error) {
		return workingTask(), nil
	}})
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentSendRun(opts); err != nil {
		t.Fatalf("send --format pretty should not error: %v", err)
	}
	text := string(out.Bytes())
	for _, want := range []string{"state: working", "task_id: chat_1", "context_id: sess_1"} {
		if !strings.Contains(text, want) {
			t.Errorf("pretty output should contain %q, got:\n%s", want, text)
		}
	}
	var env output.Envelope
	if json.Unmarshal(out.Bytes(), &env) == nil && env.OK {
		t.Errorf("pretty should not be a JSON envelope: %s", text)
	}
}

// TestSendDryRunPrettyFormat pins that --dry-run also consumes --format pretty
// (key: value preview) instead of silently emitting JSON.
func TestSendDryRunPrettyFormat(t *testing.T) {
	opts := sendTestOpts(t)
	opts.Text = "分析销售"
	opts.DryRun = true
	opts.Format = "pretty"
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentSendRun(opts); err != nil {
		t.Fatalf("dry-run pretty should not error: %v", err)
	}
	text := string(out.Bytes())
	for _, want := range []string{"dry_run: true", "ref: fakeflow:agt_x", "text: 分析销售"} {
		if !strings.Contains(text, want) {
			t.Errorf("pretty output should contain %q, got:\n%s", want, text)
		}
	}
	var env output.Envelope
	if json.Unmarshal(out.Bytes(), &env) == nil && env.OK {
		t.Errorf("pretty should not be a JSON envelope: %s", text)
	}
}

// TestSendDryRunPrettyNeutralizesInjection pins F2: the dry-run pretty preview
// runs context_id/task_id through kvValue (like every other pretty face), so a
// value carrying a newline cannot forge an adjacent "key: value" field row.
func TestSendDryRunPrettyNeutralizesInjection(t *testing.T) {
	opts := sendTestOpts(t)
	opts.Text = "hi"
	opts.DryRun = true
	opts.Format = "pretty"
	opts.ContextID = "ctx1\nstate: completed"
	opts.TaskID = "task1\ndeleted: true"
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentSendRun(opts); err != nil {
		t.Fatalf("dry-run pretty should not error: %v", err)
	}
	text := string(out.Bytes())
	// The raw newline must not survive into a forged adjacent row.
	if strings.Contains(text, "context_id: ctx1\nstate: completed") {
		t.Errorf("context_id newline not neutralized, forged a field row:\n%s", text)
	}
	if strings.Contains(text, "task_id: task1\ndeleted: true") {
		t.Errorf("task_id newline not neutralized, forged a field row:\n%s", text)
	}
	// kvValue collapses the newline to a space, keeping the value on one line.
	if !strings.Contains(text, "context_id: ctx1 state: completed") {
		t.Errorf("context_id should collapse to one line, got:\n%s", text)
	}
	if !strings.Contains(text, "task_id: task1 deleted: true") {
		t.Errorf("task_id should collapse to one line, got:\n%s", text)
	}
}

// TestSendNoParamsRequired pins card v2: the scripted card declares no
// parameters, so a send without any --param passes card validation — asserted
// via --dry-run so no provider Send fires. A malformed --param is still a
// validation error.
func TestSendNoParamsRequired(t *testing.T) {
	opts := sendTestOpts(t)
	opts.Text = "分析销售"
	opts.Params = nil
	opts.DryRun = true
	if err := agentSendRun(opts); err != nil {
		t.Fatalf("card has no required params, send without --param should pass validation: %v", err)
	}

	opts2 := sendTestOpts(t)
	opts2.Text = "分析销售"
	opts2.Params = []string{"noequals"} // a --param without '=' should still raise validation
	opts2.DryRun = true
	err := agentSendRun(opts2)
	if err == nil {
		t.Fatal("malformed --param should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("want validation error, got %T", err)
	}
}

// TestSendUnknownParamRejected pins, against an empty-parameters card, that
// any --param key is unknown → invalid_argument with a hint pointing at
// `agent card`, raised before any provider Send (asserted via --dry-run with
// no send hook installed).
func TestSendUnknownParamRejected(t *testing.T) {
	opts := sendTestOpts(t)
	opts.Text = "分析销售"
	opts.Params = []string{"app_id=app_1"}
	opts.DryRun = true
	err := agentSendRun(opts)
	if err == nil {
		t.Fatal("card did not declare app_id, --param app_id should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("want validation error, got %T", err)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("subtype should be invalid_argument, got %+v", p)
	}
	if !strings.Contains(p.Hint, "agent card") {
		t.Fatalf("hint should point to agent card, got %q", p.Hint)
	}
}

// TestSendDryRun pins that --dry-run prints a would_send preview and never
// calls the provider (no send hook installed → a Send would panic).
func TestSendDryRun(t *testing.T) {
	opts := sendTestOpts(t)
	opts.Text = "分析销售"
	opts.DryRun = true
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentSendRun(opts); err != nil {
		t.Fatalf("dry-run should not error: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("dry-run output should be valid envelope JSON: %v (%s)", err, string(out.Bytes()))
	}
	if !env.OK {
		t.Errorf("ok should be true: %+v", env)
	}
	data, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data should be an object, got %T", env.Data)
	}
	if data["dry_run"] != true {
		t.Errorf("data.dry_run should be true, got %v", data["dry_run"])
	}
	would, ok := data["would_send"].(map[string]interface{})
	if !ok {
		t.Fatalf("data.would_send should be an object, got %T", data["would_send"])
	}
	if would["text"] != "分析销售" {
		t.Errorf("would_send.text should echo the text, got %v", would["text"])
	}
}

// TestSendStartsTask pins the happy path: a single Send fires and returns the
// submitted / working task in a success envelope immediately (no polling), with
// a meta.next hint pointing at task get --watch.
func TestSendStartsTask(t *testing.T) {
	opts := sendTestOpts(t)
	opts.Text = "分析销售"
	var gotText string
	setScripted(t, scriptedHooks{send: func(in iagent.SendInput) (*iagent.AgentTask, error) {
		gotText = in.Text
		return workingTask(), nil
	}})
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentSendRun(opts); err != nil {
		t.Fatalf("send should not error: %v", err)
	}
	if gotText != "分析销售" {
		t.Errorf("provider should receive the original text, got %q", gotText)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("output should be valid envelope JSON: %v (%s)", err, string(out.Bytes()))
	}
	data, _ := env.Data.(map[string]interface{})
	if data["task_id"] != "chat_1" {
		t.Errorf("task_id should be chat_1, got %v", data["task_id"])
	}
	if data["state"] != string(iagent.StateWorking) {
		t.Errorf("state should be working, got %v", data["state"])
	}
	// meta.next should suggest polling / continuing.
	if !strings.Contains(string(out.Bytes()), `"next"`) {
		t.Errorf("non-terminal should provide meta.next follow-up: %s", string(out.Bytes()))
	}
}

// TestSendSendError surfaces a provider Send failure unchanged.
func TestSendSendError(t *testing.T) {
	opts := sendTestOpts(t)
	opts.Text = "x"
	setScripted(t, scriptedHooks{send: func(iagent.SendInput) (*iagent.AgentTask, error) {
		return nil, errs.NewAPIError(errs.SubtypeUnknown, "app ticket invalid").WithCode(99991663)
	}})
	if err := agentSendRun(opts); err == nil {
		t.Fatal("Send error should propagate")
	}
}

// TestSendInvalidRef surfaces a malformed ref as a validation error after the
// text/task-id guards pass.
func TestSendInvalidRef(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	err := agentSendRun(&sendOptions{Ref: "no-colon", Text: "x", Cmd: sendCmdCtx(t), As: "bot", Factory: f})
	if err == nil {
		t.Fatal("malformed ref should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("want validation error, got %T", err)
	}
}

// TestNewCmdAgentSend_WriteRiskAndArgs pins ExactArgs(1), write risk, and the
// presence of the send-specific flags.
func TestNewCmdAgentSend_WriteRiskAndArgs(t *testing.T) {
	cmd := NewCmdAgentSend(nil, nil)
	if level, ok := cmdutil.GetRisk(cmd); !ok || level != cmdutil.RiskWrite {
		t.Errorf("agent send should be marked write risk, got level=%q ok=%v", level, ok)
	}
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("agent send missing ref should raise an args error (ExactArgs 1)")
	}
	if err := cmd.Args(cmd, []string{"example:x"}); err != nil {
		t.Errorf("agent send with a single ref should be valid: %v", err)
	}
	for _, name := range []string{"text", "file", "param", "context-id", "task-id", "dry-run", "as", "format", "jq"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("agent send should have --%s flag", name)
		}
	}
	if cmd.Flags().Lookup("wait") != nil {
		t.Error("agent send --wait should be removed (polling goes through task get --watch)")
	}
	// The --file help must point out that files are sent off to the remote
	// provider (file-egress requirement).
	fileFlag := cmd.Flags().Lookup("file")
	if fileFlag != nil && !strings.Contains(fileFlag.Usage, "外发") && !strings.Contains(fileFlag.Usage, "上传") {
		t.Errorf("--file help should note files are sent out to the remote provider, got %q", fileFlag.Usage)
	}
}

// TestNewCmdAgentSend_RunFOverride confirms the injected runF hook is used
// instead of the production path (construction-time seam).
func TestNewCmdAgentSend_RunFOverride(t *testing.T) {
	called := false
	var captured *sendOptions
	cmd := NewCmdAgentSend(nil, func(opts *sendOptions) error {
		called = true
		captured = opts
		return nil
	})
	cmd.SetArgs([]string{"example:agt_x", "--text", "hi"})
	cmd.SetContext(context.Background())
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute should not error: %v", err)
	}
	if !called {
		t.Fatal("runF should be called")
	}
	if captured.Ref != "example:agt_x" || captured.Text != "hi" {
		t.Errorf("opts not populated correctly: %+v", captured)
	}
}

// TestSend_FileRequiresYes pins the --file exfil confirmation gate: a real send
// carrying --file to a provider that supports file upload (the scripted card has
// file_input=true) requires --yes, so without it the command returns
// confirmation_required (exit 10) BEFORE reaching the provider — the unset send
// hook is a tripwire that would panic if the gate let the upload through.
func TestSend_FileRequiresYes(t *testing.T) {
	opts := sendTestOpts(t)
	opts.Text = "hi"
	opts.Files = []string{"local.txt"} // no --yes

	err := agentSendRun(opts)
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.SubtypeConfirmationRequired {
		t.Fatalf("send --file without --yes should be confirmation_required, got %+v (err=%v)", p, err)
	}
	if output.ExitCodeOf(err) != output.ExitConfirmationRequired {
		t.Fatalf("exit should be %d, got %d", output.ExitConfirmationRequired, output.ExitCodeOf(err))
	}
}

// TestSend_FileWithYesProceeds pins that --yes satisfies the --file gate: the
// send reaches the provider, which receives the file path.
func TestSend_FileWithYesProceeds(t *testing.T) {
	opts := sendTestOpts(t)
	sent := false
	setScripted(t, scriptedHooks{send: func(in iagent.SendInput) (*iagent.AgentTask, error) {
		sent = true
		if len(in.Files) != 1 || in.Files[0] != "local.txt" {
			t.Errorf("provider should receive the --file path, got %v", in.Files)
		}
		return &iagent.AgentTask{TaskID: "t1", State: iagent.StateCompleted, IsTerminal: true}, nil
	}})
	opts.Text = "hi"
	opts.Files = []string{"local.txt"}
	opts.Yes = true

	if err := agentSendRun(opts); err != nil {
		t.Fatalf("send --file --yes should proceed: %v", err)
	}
	if !sent {
		t.Error("provider Send should be reached after --yes")
	}
}

// TestSend_FileDryRunNotGated pins that --dry-run with --file is exempt from the
// gate (dry-run never uploads), so it needs no --yes and never reaches the
// provider (unset send hook stays a tripwire).
func TestSend_FileDryRunNotGated(t *testing.T) {
	opts := sendTestOpts(t)
	opts.Text = "hi"
	opts.Files = []string{"local.txt"}
	opts.DryRun = true // no --yes

	if err := agentSendRun(opts); err != nil {
		t.Fatalf("dry-run --file should not be gated: %v", err)
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/internal/output"
)

// contextCmdCtx builds a `lark-cli agent context <leaf>` command whose --as flag
// is set to bot so ResolveAs honors it verbatim, and carries a context.
func contextCmdCtx(t *testing.T, leaf string) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "lark-cli"}
	group := &cobra.Command{Use: "agent"}
	grp := &cobra.Command{Use: "context"}
	l := &cobra.Command{Use: leaf}
	root.AddCommand(group)
	group.AddCommand(grp)
	grp.AddCommand(l)
	l.Flags().String("as", "", "identity")
	if err := l.Flags().Set("as", "bot"); err != nil {
		t.Fatal(err)
	}
	l.SetContext(context.Background())
	return l
}

// contextTestOpts wires a contextOptions against a real (test) Factory,
// addressing the scripted fakeflow agent agt_x under a bot identity. The
// Factory's httpmock registry holds zero stubs, so any HTTP attempt fails the
// test; provider behavior is scripted via setScripted.
func contextTestOpts(t *testing.T, leaf string) (*contextOptions, *httpmock.Registry) {
	t.Helper()
	registerScripted()
	cfg := &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu}
	f, _, _, reg := cmdutil.TestFactory(t, cfg)
	return &contextOptions{
		Factory: f,
		Cmd:     contextCmdCtx(t, leaf),
		Ref:     "fakeflow:agt_x",
		As:      "bot",
	}, reg
}

// TestContextDeleteRequiresYes pins that `context delete` without --yes is a
// confirmation_required error (exit 10), raised before any provider is built.
func TestContextDeleteRequiresYes(t *testing.T) {
	err := agentContextDeleteRun(&contextOptions{Ref: "example:agt_x", CtxID: "c1", Yes: false})
	if err == nil {
		t.Fatal("context delete without --yes should report confirmation_required")
	}
	if !errs.IsConfirmationRequired(err) {
		t.Fatalf("should be a confirmation_required error, got %T", err)
	}
	if code := output.ExitCodeOf(err); code != output.ExitConfirmationRequired {
		t.Fatalf("exit code should be 10, got %d", code)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.SubtypeConfirmationRequired {
		t.Fatalf("subtype should be confirmation_required, got %+v", p)
	}
}

// TestContextDeleteWithYes pins the confirmed path: --yes reaches the provider,
// deletes the session, and emits a success envelope.
func TestContextDeleteWithYes(t *testing.T) {
	opts, _ := contextTestOpts(t, "delete")
	opts.CtxID = "sess_1"
	opts.Yes = true
	var deleted string
	setScripted(t, scriptedHooks{deleteContext: func(ctxID string) error {
		deleted = ctxID
		return nil
	}})
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentContextDeleteRun(opts); err != nil {
		t.Fatalf("context delete --yes should not error: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("output should be valid envelope JSON: %v (%s)", err, string(out.Bytes()))
	}
	data, _ := env.Data.(map[string]interface{})
	if data["context_id"] != "sess_1" || data["deleted"] != true {
		t.Errorf("data should echo {context_id, deleted:true}, got %v", env.Data)
	}
	if deleted != "sess_1" {
		t.Errorf("provider should receive the context id to delete, got %q", deleted)
	}
}

// TestContextDeleteProviderError surfaces a provider DeleteContext failure
// (non-zero business code) after --yes passes.
func TestContextDeleteProviderError(t *testing.T) {
	opts, _ := contextTestOpts(t, "delete")
	opts.CtxID = "sess_1"
	opts.Yes = true
	setScripted(t, scriptedHooks{deleteContext: func(string) error {
		return errs.NewAPIError(errs.SubtypeUnknown, "app ticket invalid").WithCode(99991663)
	}})
	if err := agentContextDeleteRun(opts); err == nil {
		t.Fatal("a DeleteContext error should propagate")
	}
}

// TestContextDeleteInvalidRef surfaces a malformed ref as a validation error
// after the --yes confirmation guard passes.
func TestContextDeleteInvalidRef(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	err := agentContextDeleteRun(&contextOptions{Ref: "no-colon", CtxID: "c1", Yes: true, Cmd: contextCmdCtx(t, "delete"), As: "bot", Factory: f})
	if err == nil {
		t.Fatal("malformed ref should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T", err)
	}
}

// TestContextListEmitsContexts pins that `context list` returns
// {contexts:[...]} with a meta.count.
func TestContextListEmitsContexts(t *testing.T) {
	opts, _ := contextTestOpts(t, "list")
	setScripted(t, scriptedHooks{listContexts: func() ([]iagent.ContextSummary, error) {
		return []iagent.ContextSummary{
			{ContextID: "sess_1", Title: "销售分析", CreatedAt: "2026-07-05T10:01:11+08:00"},
			{ContextID: "sess_2"},
		}, nil
	}})
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentContextListRun(opts); err != nil {
		t.Fatalf("context list should not error: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("output should be valid envelope JSON: %v (%s)", err, string(out.Bytes()))
	}
	data, _ := env.Data.(map[string]interface{})
	contexts, ok := data["contexts"].([]interface{})
	if !ok || len(contexts) != 2 {
		t.Fatalf("data.contexts should have 2 entries, got %v", data["contexts"])
	}
	if env.Meta == nil || env.Meta.Count != 2 {
		t.Errorf("meta.count should be 2, got %+v", env.Meta)
	}
}

// TestContextListError surfaces a provider ListContexts failure.
func TestContextListError(t *testing.T) {
	opts, _ := contextTestOpts(t, "list")
	setScripted(t, scriptedHooks{listContexts: func() ([]iagent.ContextSummary, error) {
		return nil, errs.NewAPIError(errs.SubtypeUnknown, "app ticket invalid").WithCode(99991663)
	}})
	if err := agentContextListRun(opts); err == nil {
		t.Fatal("a ListContexts error should propagate")
	}
}

// TestContextListInvalidRef surfaces a malformed ref as a validation error.
func TestContextListInvalidRef(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	err := agentContextListRun(&contextOptions{Ref: "no-colon", Cmd: contextCmdCtx(t, "list"), As: "bot", Factory: f})
	if err == nil {
		t.Fatal("malformed ref should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T", err)
	}
}

// TestContextGetEmitsDetail pins that `context get` returns a single context
// detail.
func TestContextGetEmitsDetail(t *testing.T) {
	opts, _ := contextTestOpts(t, "get")
	opts.CtxID = "sess_1"
	setScripted(t, scriptedHooks{getContext: func(ctxID string) (*iagent.ContextDetail, error) {
		return &iagent.ContextDetail{ContextID: ctxID, Title: "销售分析", CreatedAt: "2026-07-05T10:01:11+08:00"}, nil
	}})
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentContextGetRun(opts); err != nil {
		t.Fatalf("context get should not error: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("output should be valid envelope JSON: %v (%s)", err, string(out.Bytes()))
	}
	data, _ := env.Data.(map[string]interface{})
	if data["context_id"] != "sess_1" {
		t.Errorf("data.context_id should be sess_1, got %v", data["context_id"])
	}
	if data["title"] != "销售分析" {
		t.Errorf("data.title should be echoed, got %v", data["title"])
	}
}

// TestContextGetError surfaces a provider GetContext failure.
func TestContextGetError(t *testing.T) {
	opts, _ := contextTestOpts(t, "get")
	opts.CtxID = "sess_1"
	setScripted(t, scriptedHooks{getContext: func(string) (*iagent.ContextDetail, error) {
		return nil, errs.NewAPIError(errs.SubtypeUnknown, "app ticket invalid").WithCode(99991663)
	}})
	if err := agentContextGetRun(opts); err == nil {
		t.Fatal("a GetContext error should propagate")
	}
}

// TestContextGetInvalidRef surfaces a malformed ref as a validation error.
func TestContextGetInvalidRef(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	err := agentContextGetRun(&contextOptions{Ref: "no-colon", CtxID: "c1", Cmd: contextCmdCtx(t, "get"), As: "bot", Factory: f})
	if err == nil {
		t.Fatal("malformed ref should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T", err)
	}
}

// TestContextListWithJq exercises the --jq output branch for list.
func TestContextListWithJq(t *testing.T) {
	opts, _ := contextTestOpts(t, "list")
	opts.Cmd.Flags().String("jq", ".data.contexts | length", "")
	setScripted(t, scriptedHooks{listContexts: func() ([]iagent.ContextSummary, error) {
		return []iagent.ContextSummary{{ContextID: "sess_1"}}, nil
	}})
	if err := agentContextListRun(opts); err != nil {
		t.Fatalf("context list --jq should not error: %v", err)
	}
}

// TestContextListPretty exercises the --format pretty human-view branch for
// list: header TSV rows (not a JSON envelope), with the agent-controlled Title
// stripped of ANSI escapes.
func TestContextListPretty(t *testing.T) {
	opts, _ := contextTestOpts(t, "list")
	opts.Format = "pretty"
	setScripted(t, scriptedHooks{listContexts: func() ([]iagent.ContextSummary, error) {
		return []iagent.ContextSummary{
			{ContextID: "sess_1", Title: "\x1b[2J销售分析", CreatedAt: "2026-07-05T10:01:11+08:00"},
		}, nil
	}})
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })
	if err := agentContextListRun(opts); err != nil {
		t.Fatalf("context list --format pretty should not error: %v", err)
	}
	s := string(out.Bytes())
	if !strings.HasPrefix(s, "CONTEXT_ID\tCREATED_AT\tTITLE\n") {
		t.Errorf("pretty output should start with a header row, got %q", s)
	}
	if !strings.Contains(s, "sess_1") || !strings.Contains(s, "销售分析") {
		t.Errorf("pretty output should contain context_id and title, got %q", s)
	}
	if strings.Contains(s, "\x1b") {
		t.Errorf("ANSI sequences in Title must be stripped: %q", s)
	}
	if strings.Contains(s, `"ok"`) {
		t.Errorf("pretty output should be a human view, not a JSON envelope, got %q", s)
	}
}

// TestContextGetWithJq pins the added --jq flag on context get: the envelope is
// filtered through the jq expression.
func TestContextGetWithJq(t *testing.T) {
	opts, _ := contextTestOpts(t, "get")
	opts.CtxID = "sess_1"
	opts.Cmd.Flags().String("jq", "", "")
	if err := opts.Cmd.Flags().Set("jq", ".data.context_id"); err != nil {
		t.Fatal(err)
	}
	setScripted(t, scriptedHooks{getContext: func(ctxID string) (*iagent.ContextDetail, error) {
		return &iagent.ContextDetail{ContextID: ctxID}, nil
	}})
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })
	if err := agentContextGetRun(opts); err != nil {
		t.Fatalf("context get --jq should not error: %v", err)
	}
	got := strings.TrimSpace(string(out.Bytes()))
	if !strings.Contains(got, "sess_1") || strings.Contains(got, `"ok"`) {
		t.Errorf("--jq .data.context_id should output only the filtered result, got %q", got)
	}
}

// TestContextGetPretty pins the added --format pretty branch on context get:
// key: value lines with the tasks count, title ANSI-stripped.
func TestContextGetPretty(t *testing.T) {
	opts, _ := contextTestOpts(t, "get")
	opts.CtxID = "sess_1"
	opts.Format = "pretty"
	setScripted(t, scriptedHooks{getContext: func(ctxID string) (*iagent.ContextDetail, error) {
		return &iagent.ContextDetail{
			ContextID: ctxID, Title: "\x1b[31m销售分析\x1b[0m",
			Tasks: []iagent.TaskSummary{{TaskID: "chat_1", State: iagent.StateCompleted, IsTerminal: true}},
		}, nil
	}})
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })
	if err := agentContextGetRun(opts); err != nil {
		t.Fatalf("context get --format pretty should not error: %v", err)
	}
	s := string(out.Bytes())
	for _, want := range []string{"context_id: sess_1", "title: 销售分析", "tasks: 1"} {
		if !strings.Contains(s, want) {
			t.Errorf("pretty output should contain %q, got %q", want, s)
		}
	}
	if strings.Contains(s, "\x1b") {
		t.Errorf("ANSI sequences in title must be stripped: %q", s)
	}
}

// findSub returns the direct subcommand of cmd whose Name() == name, or nil.
func findSub(cmd *cobra.Command, name string) *cobra.Command {
	for _, c := range cmd.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

// TestNewCmdAgentContext_GroupHasSubcommands pins the group is a pure group (no
// RunE) with list/get/delete leaves.
func TestNewCmdAgentContext_GroupHasSubcommands(t *testing.T) {
	cmd := NewCmdAgentContext(nil)
	if cmd.RunE != nil || cmd.Run != nil {
		t.Error("agent context group should not have RunE")
	}
	want := []string{"list", "get", "delete"}
	for _, name := range want {
		if findSub(cmd, name) == nil {
			t.Errorf("missing subcommand context %s", name)
		}
	}
}

// TestNewCmdAgentContextList_ReadRisk pins list = read risk, ExactArgs(1), and
// the default flip: --format defaults to json.
func TestNewCmdAgentContextList_ReadRisk(t *testing.T) {
	cmd := NewCmdAgentContextList(nil)
	if level, ok := cmdutil.GetRisk(cmd); !ok || level != cmdutil.RiskRead {
		t.Errorf("context list should be marked read risk, got level=%q ok=%v", level, ok)
	}
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("context list missing ref should report an argument error (ExactArgs 1)")
	}
	if err := cmd.Args(cmd, []string{"example:x"}); err != nil {
		t.Errorf("context list with a single ref should be valid: %v", err)
	}
	fl := cmd.Flags().Lookup("format")
	if fl == nil || fl.DefValue != "json" {
		t.Errorf("context list --format default should flip to json, got %+v", fl)
	}
}

// TestNewCmdAgentContextGet_ReadRisk pins get = read risk, ExactArgs(2), and
// the added --format / --jq flags.
func TestNewCmdAgentContextGet_ReadRisk(t *testing.T) {
	cmd := NewCmdAgentContextGet(nil)
	if level, ok := cmdutil.GetRisk(cmd); !ok || level != cmdutil.RiskRead {
		t.Errorf("context get should be marked read risk, got level=%q ok=%v", level, ok)
	}
	if err := cmd.Args(cmd, []string{"example:x"}); err == nil {
		t.Error("context get missing ctx-id should report an argument error (ExactArgs 2)")
	}
	if err := cmd.Args(cmd, []string{"example:x", "c1"}); err != nil {
		t.Errorf("context get ref+ctx-id should be valid: %v", err)
	}
	for _, name := range []string{"format", "jq"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("context get should have a --%s flag", name)
		}
	}
}

// TestNewCmdAgentContextDelete_HighRiskWrite pins delete = high-risk-write risk,
// ExactArgs(2), a --yes flag, and the added --format / --jq flags.
func TestNewCmdAgentContextDelete_HighRiskWrite(t *testing.T) {
	cmd := NewCmdAgentContextDelete(nil)
	if level, ok := cmdutil.GetRisk(cmd); !ok || level != cmdutil.RiskHighRiskWrite {
		t.Errorf("context delete should be marked high-risk-write risk, got level=%q ok=%v", level, ok)
	}
	if err := cmd.Args(cmd, []string{"example:x"}); err == nil {
		t.Error("context delete missing ctx-id should report an argument error (ExactArgs 2)")
	}
	if cmd.Flags().Lookup("yes") == nil {
		t.Error("context delete should have a --yes flag")
	}
	for _, name := range []string{"format", "jq"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("context delete should have a --%s flag", name)
		}
	}
}

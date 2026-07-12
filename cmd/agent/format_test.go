// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/output"
)

// TestPrintTaskPrettyRendersDecision pins that printTaskPretty surfaces a
// structured input_required decision (prompt + decision_id + options), with all
// agent-controlled fields ANSI-stripped.
func TestPrintTaskPrettyRendersDecision(t *testing.T) {
	out := &bytes.Buffer{}
	printTaskPretty(out, &iagent.AgentTask{
		TaskID: "task_1", State: iagent.StateInputRequired,
		InputRequired: &iagent.InputRequired{
			DecisionID: "dec_7f3a",
			Prompt:     "按大区还是品类\x1b[2J拆?",
			Options: []iagent.Option{
				{OptionID: "by_region", Label: "按大区"},
				{OptionID: "by_category", Label: "按品类"},
			},
		},
	})
	text := out.String()
	for _, want := range []string{"input_required:", "decision_id: dec_7f3a", "option by_region: 按大区", "option by_category: 按品类"} {
		if !strings.Contains(text, want) {
			t.Errorf("pretty task should render decision part %q, got:\n%s", want, text)
		}
	}
	if strings.Contains(text, "\x1b") {
		t.Errorf("ANSI in the decision prompt must be stripped, got %q", text)
	}
}

// TestValidateFormat_Valid pins that json/pretty (and the zero value, which
// only occurs when options structs are built directly in tests) pass.
func TestValidateFormat_Valid(t *testing.T) {
	for _, f := range []string{"", "json", "pretty"} {
		if err := validateFormat(f); err != nil {
			t.Errorf("format %q should be valid: %v", f, err)
		}
	}
}

// TestValidateFormat_Invalid pins that a --format outside json|pretty is a
// validation/invalid_argument error (exit 2) whose hint lists the legal values
// and whose param names the flag with the -- prefix.
func TestValidateFormat_Invalid(t *testing.T) {
	err := validateFormat("yaml")
	if err == nil {
		t.Fatal("--format yaml should error (currently silently treated as json)")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T", err)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("subtype should be invalid_argument, got %+v", p)
	}
	if output.ExitCodeOf(err) != output.ExitValidation {
		t.Fatalf("exit should be 2, got %d", output.ExitCodeOf(err))
	}
	if !strings.Contains(p.Hint, "json | pretty") {
		t.Errorf("hint should list the legal values json | pretty, got %q", p.Hint)
	}
	var verr *errs.ValidationError
	if !errors.As(err, &verr) || verr.Param != "--format" {
		t.Errorf("param should be --format, got %+v", verr)
	}
}

// agentRootTree builds `lark-cli agent ...` as production wires it (root Use
// lark-cli), with a nil Factory: format validation must fire at the RunE
// entry, before any Factory access.
func agentRootTree() *cobra.Command {
	root := &cobra.Command{Use: "lark-cli", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(NewCmdAgent(nil))
	return root
}

// TestFormatYamlRejectedAcrossLeaves pins that EVERY leaf of the agent tree
// consumes validateFormat: `--format yaml` is exit 2 with the json|pretty
// hint, uniformly, before any provider/Factory is touched.
func TestFormatYamlRejectedAcrossLeaves(t *testing.T) {
	leaves := [][]string{
		{"agent", "list", "--format", "yaml"},
		{"agent", "card", "example:x", "--format", "yaml"},
		{"agent", "send", "example:x", "--text", "hi", "--format", "yaml"},
		{"agent", "task", "get", "example:x", "t1", "--format", "yaml"},
		{"agent", "task", "list", "example:x", "--format", "yaml"},
		{"agent", "task", "cancel", "example:x", "t1", "--format", "yaml"},
		{"agent", "context", "list", "example:x", "--format", "yaml"},
		{"agent", "context", "get", "example:x", "c1", "--format", "yaml"},
		{"agent", "context", "delete", "example:x", "c1", "--yes", "--format", "yaml"},
	}
	for _, argv := range leaves {
		t.Run(strings.Join(argv[:len(argv)-2], " "), func(t *testing.T) {
			root := agentRootTree()
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			root.SetArgs(argv)
			err := root.Execute()
			if err == nil {
				t.Fatalf("%v should report a --format validation error", argv)
			}
			if !errs.IsValidation(err) {
				t.Fatalf("should be a validation error, got %T: %v", err, err)
			}
			if output.ExitCodeOf(err) != output.ExitValidation {
				t.Fatalf("exit should be 2, got %d", output.ExitCodeOf(err))
			}
			p, ok := errs.ProblemOf(err)
			if !ok || !strings.Contains(p.Hint, "json | pretty") {
				t.Errorf("hint should contain json | pretty, got %+v", p)
			}
		})
	}
}

// TestFormatHelpTextUniform pins the mandated uniform help text
// "output format: json (default) | pretty" across every leaf that has --format.
func TestFormatHelpTextUniform(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"list":           NewCmdAgentList(nil),
		"card":           NewCmdAgentCard(nil),
		"send":           NewCmdAgentSend(nil, nil),
		"task get":       NewCmdAgentTaskGet(nil),
		"task list":      NewCmdAgentTaskList(nil),
		"task cancel":    NewCmdAgentTaskCancel(nil),
		"context list":   NewCmdAgentContextList(nil),
		"context get":    NewCmdAgentContextGet(nil),
		"context delete": NewCmdAgentContextDelete(nil),
	}
	for name, cmd := range cmds {
		fl := cmd.Flags().Lookup("format")
		if fl == nil {
			t.Errorf("%s should have a --format flag", name)
			continue
		}
		if fl.DefValue != "json" {
			t.Errorf("%s --format default should be json, got %q", name, fl.DefValue)
		}
		if fl.Usage != "output format: json (default) | pretty" {
			t.Errorf("%s --format help should be uniform, got %q", name, fl.Usage)
		}
	}
}

// TestStripANSI pins that CSI sequences, OSC sequences and bare ESC bytes are
// all removed before agent text reaches a terminal.
func TestStripANSI(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"before\x1b[31mred\x1b[0mafter", "beforeredafter"},
		{"a\x1bb", "ab"}, // bare ESC
		{"t\x1b]0;evil\x07x", "tx"},
		{"clean 文本", "clean 文本"},
	} {
		if got := stripANSI(tt.in); got != tt.want {
			t.Errorf("stripANSI(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestPrintTaskPretty pins the task-class pretty spec: line-per-field
// key: value with state / task_id / context_id / first text message truncated
// to 120 runes / artifacts count — and the agent-controlled text stripped of
// ANSI escapes.
func TestPrintTaskPretty(t *testing.T) {
	long := strings.Repeat("字", 130)
	task := &iagent.AgentTask{
		TaskID:    "chat_1",
		ContextID: "sess_1",
		State:     iagent.StateCompleted,
		Messages: []iagent.Message{{
			Role:  "agent",
			Parts: []iagent.Part{{Type: "text", Text: "\x1b[31m" + long + "\x1b[0m"}},
		}},
		Artifacts: []iagent.Artifact{{ID: "a1"}, {ID: "a2"}},
	}
	out := &bytes.Buffer{}
	printTaskPretty(out, task)
	text := out.String()

	for _, want := range []string{"state: completed", "task_id: chat_1", "context_id: sess_1", "artifacts: 2"} {
		if !strings.Contains(text, want) {
			t.Errorf("pretty output should contain %q, got:\n%s", want, text)
		}
	}
	if strings.Contains(text, "\x1b") {
		t.Errorf("ANSI sequences in agent body text must be stripped: %q", text)
	}
	if strings.Contains(text, long) {
		t.Errorf("body should be truncated to 120 chars, the full 130-char body should not appear")
	}
	if !strings.Contains(text, strings.Repeat("字", 120)) {
		t.Errorf("body should keep the first 120 chars, got:\n%s", text)
	}
	var env output.Envelope
	if json.Unmarshal(out.Bytes(), &env) == nil && env.OK {
		t.Errorf("pretty should not be a JSON envelope: %s", text)
	}
}

// TestPrintTaskPretty_NewlineForgeryNeutralized pins the key:value forgery
// fix: agent text containing newlines must not be able to fake an adjacent
// field row ("done\nstate: completed") — \n/\t in single-line values collapse
// to spaces, so exactly one state: line exists.
func TestPrintTaskPretty_NewlineForgeryNeutralized(t *testing.T) {
	task := &iagent.AgentTask{
		TaskID: "chat_1",
		State:  iagent.StateFailed,
		Messages: []iagent.Message{{
			Role:  "agent",
			Parts: []iagent.Part{{Type: "text", Text: "done\nstate: completed\tok"}},
		}},
	}
	out := &bytes.Buffer{}
	printTaskPretty(out, task)

	var stateLines int
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "state: ") {
			stateLines++
		}
	}
	if stateLines != 1 {
		t.Fatalf("body newlines must not forge an adjacent field row; there should be exactly 1 state: line, got %d:\n%s", stateLines, out.String())
	}
	if !strings.Contains(out.String(), "state: failed") {
		t.Errorf("the real state line should remain, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "reply: done state: completed ok") {
		t.Errorf("\\n/\\t in the body should be replaced by spaces, got:\n%s", out.String())
	}
}

// TestPrintContextDetailPretty_NewlineForgeryNeutralized pins the same fix on
// the context title row.
func TestPrintContextDetailPretty_NewlineForgeryNeutralized(t *testing.T) {
	out := &bytes.Buffer{}
	printContextDetailPretty(out, &iagent.ContextDetail{
		ContextID: "sess_1",
		Title:     "标题\ncontext_id: forged",
	})
	var idLines int
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "context_id: ") {
			idLines++
		}
	}
	if idLines != 1 {
		t.Fatalf("title newlines must not forge a context_id row; there should be exactly 1 line, got %d:\n%s", idLines, out.String())
	}
}

// TestPrintTaskPretty_NilTask pins the nil degradation (no panic).
func TestPrintTaskPretty_NilTask(t *testing.T) {
	out := &bytes.Buffer{}
	printTaskPretty(out, nil)
	if out.Len() == 0 {
		t.Error("nil task should print a placeholder line")
	}
}

// TestPrintTaskSummariesTSV pins the list-class pretty spec: a header row
// naming the json fields (now including UPDATED_AT + SUMMARY), then one
// tab-separated row per task. Summary is agent-controlled, so it is
// ANSI-stripped AND newline/tab-flattened via kvValue.
func TestPrintTaskSummariesTSV(t *testing.T) {
	out := &bytes.Buffer{}
	printTaskSummariesTSV(out, []iagent.TaskSummary{
		{TaskID: "chat_1", ContextID: "sess_1", State: iagent.StateCompleted, IsTerminal: true,
			UpdatedAt: "2026-07-05T12:00:00Z", Summary: "分析\n完成\x1b[0m"},
	})
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("should have a header + 1 data row, got %q", out.String())
	}
	if lines[0] != "TASK_ID\tCONTEXT_ID\tSTATE\tIS_TERMINAL\tUPDATED_AT\tSUMMARY" {
		t.Errorf("header columns should match the json field names, got %q", lines[0])
	}
	// Summary: ANSI escape stripped, newline flattened to a space.
	if lines[1] != "chat_1\tsess_1\tcompleted\ttrue\t2026-07-05T12:00:00Z\t分析 完成" {
		t.Errorf("data row mismatch, got %q", lines[1])
	}
}

// TestPrintContextsTSV pins the context-list pretty spec: header row (now
// carrying the UPDATED_AT / TASK_COUNT / AWAITING_INPUT rollup columns) plus
// rows, with the agent-controlled Title stripped of ANSI escapes.
func TestPrintContextsTSV(t *testing.T) {
	out := &bytes.Buffer{}
	printContextsTSV(out, []iagent.ContextSummary{
		{ContextID: "sess_1", CreatedAt: "2026-07-05T10:00:00+08:00", UpdatedAt: "2026-07-05T12:00:00+08:00",
			Title: "\x1b[2J销售分析", TaskCount: 3, AwaitingInput: true},
	})
	text := out.String()
	if !strings.HasPrefix(text, "CONTEXT_ID\tCREATED_AT\tUPDATED_AT\tTITLE\tTASK_COUNT\tAWAITING_INPUT\n") {
		t.Errorf("should have a header row with the rollup columns, got %q", text)
	}
	if !strings.Contains(text, "销售分析") {
		t.Errorf("should contain the title text, got %q", text)
	}
	if strings.Contains(text, "\x1b") {
		t.Errorf("ANSI sequences in Title must be stripped: %q", text)
	}
	// The rollup columns (task_count + awaiting_input) trail the row.
	if !strings.Contains(text, "\t3\ttrue") {
		t.Errorf("should carry the task_count + awaiting_input rollup, got %q", text)
	}
}

// TestPrintContextDetailPretty pins the context-get pretty rendering as a
// conversation overview: metadata + the task_count / awaiting_input rollup and
// a one-line active_task digest — NOT a full tasks[] list (that is `agent task
// list --context-id`). Title and the active-task Summary are agent-controlled,
// so both are ANSI-stripped + newline-flattened.
func TestPrintContextDetailPretty(t *testing.T) {
	out := &bytes.Buffer{}
	printContextDetailPretty(out, &iagent.ContextDetail{
		ContextID:     "sess_1",
		CreatedAt:     "2026-07-05T10:00:00+08:00",
		UpdatedAt:     "2026-07-05T12:00:00+08:00",
		Title:         "\x1b[31m分析\x1b[0m",
		TaskCount:     2,
		AwaitingInput: true,
		ActiveTask: &iagent.TaskSummary{
			TaskID: "chat_2", State: iagent.StateInputRequired,
			UpdatedAt: "2026-07-05T12:00:00+08:00", Summary: "请提供\n季度\x1b[0m",
		},
	})
	text := out.String()
	for _, want := range []string{
		"context_id: sess_1", "updated_at: 2026-07-05T12:00:00+08:00", "title: 分析",
		"task_count: 2", "awaiting_input: true", "active_task: input_required",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("pretty output should contain %q, got:\n%s", want, text)
		}
	}
	// active-task Summary: newline flattened to a space.
	if !strings.Contains(text, "请提供 季度") {
		t.Errorf("active_task summary should be ANSI-stripped + newline-flattened, got:\n%s", text)
	}
	if strings.Contains(text, "\x1b") {
		t.Errorf("ANSI sequences must be stripped: %q", text)
	}
	// The full task enumeration must NOT appear here anymore.
	if strings.Contains(text, "tasks:") {
		t.Errorf("context get should no longer render a tasks[] list, got:\n%s", text)
	}
}

// TestExactArgsUsageHint pins that an arg-count error carries a usage hint
// built from the real command path + Use shape, so the caller learns what is
// missing instead of cobra's bare "accepts 2 arg(s)".
func TestExactArgsUsageHint(t *testing.T) {
	root := agentRootTree()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"agent", "task", "get", "example:x"}) // missing task-id
	err := root.Execute()
	if err == nil {
		t.Fatal("task get with a single argument should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("an arg-count error should be a validation type, got %T: %v", err, err)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || !strings.Contains(p.Hint, "用法: lark-cli agent task get <agent_ref> <task-id>") {
		t.Fatalf("hint should contain the usage string, got %+v", p)
	}
	if output.ExitCodeOf(err) != output.ExitValidation {
		t.Fatalf("exit should be 2, got %d", output.ExitCodeOf(err))
	}
}

// TestMaximumArgsUsageHint pins the same treatment for the MaximumNArgs leaf
// (`agent list [scheme]`).
func TestMaximumArgsUsageHint(t *testing.T) {
	root := agentRootTree()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"agent", "list", "example", "extra"})
	err := root.Execute()
	if err == nil {
		t.Fatal("list with more than 1 positional argument should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("an arg-count error should be a validation type, got %T: %v", err, err)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || !strings.Contains(p.Hint, "用法: lark-cli agent list [scheme]") {
		t.Fatalf("hint should contain the usage string, got %+v", p)
	}
}

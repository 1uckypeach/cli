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

// Register the example provider for this test binary (provider packages are pure
// data now — the top-level agent package's init does this in production, but that
// package cannot be imported here without an import cycle).
func init() { agent.Register(Provider()) }

// fakeRuntime is the offline test runtime: it supplies the addressed agent_id
// and no-ops the network methods (the mock hooks only ever read AgentID()).
type fakeRuntime struct{ agentID string }

func (r fakeRuntime) AgentID() string { return r.agentID }
func (r fakeRuntime) IsBot() bool     { return false }
func (r fakeRuntime) CallAPI(context.Context, string, string, map[string]string, any) (map[string]any, error) {
	return nil, nil
}
func (r fakeRuntime) CallMultipart(context.Context, string, string, map[string]string, []agent.FilePart) (map[string]any, error) {
	return nil, nil
}

// swapStore replaces the package-level store with an isolated instance pointing at
// t.TempDir, so tests do not pollute each other or the local demo snapshot.
func swapStore(t *testing.T) {
	t.Helper()
	old := store
	store = newMemoryStore(filepath.Join(t.TempDir(), "state.json"))
	t.Cleanup(func() { store = old })
}

// TestConformance runs the shared conformance suite for both catalog entries.
func TestConformance(t *testing.T) {
	agenttest.RunConformance(t, "example", "echo")
}

func TestConformanceReporter(t *testing.T) {
	agenttest.RunConformance(t, "example", "reporter")
}

// TestCapabilityMatrixDiverges pins the deliberate difference between the two
// agents, derived purely from which hooks each spec wires.
func TestCapabilityMatrixDiverges(t *testing.T) {
	ec := agent.DeriveCapabilities(&echoSpec)
	rc := agent.DeriveCapabilities(&reporterSpec)
	if ec.ArtifactDownload || ec.FileInput || ec.TaskCancel {
		t.Errorf("echo should be the minimal set (no artifact/file/cancel), got %+v", ec)
	}
	if !ec.MultiTurn || !ec.TaskGet || !ec.TaskList {
		t.Errorf("echo should support multi_turn/task_get/task_list, got %+v", ec)
	}
	if !(rc.ArtifactDownload && rc.FileInput && rc.TaskCancel && rc.InputRequired && rc.MultiTurn && rc.TaskGet && rc.TaskList) {
		t.Errorf("reporter should have everything enabled, got %+v", rc)
	}
}

// TestEchoUnwiredCapabilities verifies the new model: echo simply leaves
// CancelTask / DownloadArtifact unwired and FileInput false — no refusal code.
func TestEchoUnwiredCapabilities(t *testing.T) {
	if echoSpec.CancelTask != nil {
		t.Error("echo should not wire CancelTask (task_cancel=false)")
	}
	if echoSpec.DownloadArtifact != nil {
		t.Error("echo should not wire DownloadArtifact (artifact_download=false)")
	}
	if echoSpec.FileInput {
		t.Error("echo should not accept file input (file_input=false)")
	}
}

// TestEchoMultiTurn verifies multi-turn context memory across the read verbs.
func TestEchoMultiTurn(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{"echo"}
	ctx := context.Background()

	t1, err := echoSend(ctx, rt, agent.SendInput{Text: "hello"})
	if err != nil {
		t.Fatalf("first-turn send: %v", err)
	}
	if t1.State != agent.StateCompleted || t1.ContextID == "" || t1.TaskID == "" {
		t.Fatalf("first turn should be completed with context_id/task_id: %+v", t1)
	}
	if got := agentReply(t, t1); got != "hello" {
		t.Fatalf("first-turn echo should be the original text, got %q", got)
	}

	t2, err := echoSend(ctx, rt, agent.SendInput{Text: "再来", ContextID: t1.ContextID})
	if err != nil {
		t.Fatalf("follow-up send: %v", err)
	}
	if t2.ContextID != t1.ContextID {
		t.Fatalf("follow-up should stay in the same context: %q vs %q", t2.ContextID, t1.ContextID)
	}
	if got := agentReply(t, t2); got != "再来（第 2 轮）" {
		t.Fatalf("second-turn echo should carry a turn marker, got %q", got)
	}

	got, err := getTask(ctx, rt, t2.TaskID)
	if err != nil {
		t.Fatalf("getTask: %v", err)
	}
	if agentReply(t, got) != "再来（第 2 轮）" {
		t.Fatalf("getTask should replay the stored messages, got %+v", got.Messages)
	}
	tasks, err := listTasks(ctx, rt, t1.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("the same context should have 2 tasks, got %d", len(tasks))
	}
	ctxs, err := listContexts(ctx, rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctxs) != 1 || ctxs[0].ContextID != t1.ContextID {
		t.Fatalf("should have exactly 1 context with a matching id, got %+v", ctxs)
	}
	detail, err := getContext(ctx, rt, t1.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Tasks) != 2 {
		t.Fatalf("context detail should contain 2 tasks, got %+v", detail)
	}
}

// TestStateSurvivesReload pins the cross-process semantics via the shared snapshot.
func TestStateSurvivesReload(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{"echo"}
	task, err := echoSend(context.Background(), rt, agent.SendInput{Text: "persist"})
	if err != nil {
		t.Fatal(err)
	}
	store = newMemoryStore(store.path) // a new process view; only the snapshot file is shared
	got, err := getTask(context.Background(), rt, task.TaskID)
	if err != nil {
		t.Fatalf("getTask after reload: %v", err)
	}
	if got.ContextID != task.ContextID {
		t.Fatalf("task should replay fully after reload: %+v", got)
	}
}

// TestReporterArtifactFlow verifies the full artifact chain.
func TestReporterArtifactFlow(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{"reporter"}
	ctx := context.Background()

	task, err := reporterSend(ctx, rt, agent.SendInput{Text: "本季度报表"})
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

	data, err := downloadArtifact(ctx, rt, task.TaskID, art.ID)
	if err != nil {
		t.Fatalf("downloadArtifact: %v", err)
	}
	if data.Name != "quarterly_report.csv" || data.Mime != "text/csv" {
		t.Errorf("suggested_name/mime wrong: %+v", data)
	}
	if !strings.HasPrefix(string(data.Bytes), "quarter,revenue") {
		t.Errorf("should return inline CSV bytes, got %q", string(data.Bytes))
	}

	if _, err := downloadArtifact(ctx, rt, task.TaskID, "art_nope"); err == nil {
		t.Fatal("unknown artifact id should return an error")
	} else if _, ok := errs.ProblemOf(err); !ok {
		t.Fatalf("unknown artifact id should be a typed error, got %T: %v", err, err)
	}
}

// TestReporterCancelTerminal verifies reporter's cancel returns failed_precondition
// for a terminal task (the mock task is completed the moment it is sent).
func TestReporterCancelTerminal(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{"reporter"}
	ctx := context.Background()
	task, err := reporterSend(ctx, rt, agent.SendInput{Text: "报表"})
	if err != nil {
		t.Fatal(err)
	}
	err = cancelTask(ctx, rt, task.TaskID)
	if err == nil {
		t.Fatal("canceling a terminal task should return an error")
	}
	prob, ok := errs.ProblemOf(err)
	if !ok || prob.Subtype != errs.SubtypeFailedPrecondition {
		t.Fatalf("terminal cancel should be failed_precondition, got %v", err)
	}
}

// TestUnknownCatalogID verifies an unknown catalog id is a typed error from the
// framework's LookupSpec (with a hint pointing to agent list example).
func TestUnknownCatalogID(t *testing.T) {
	_, _, _, err := agent.LookupSpec("example:nonexistent")
	if err == nil {
		t.Fatal("an unknown catalog id should return an error")
	}
	prob, ok := errs.ProblemOf(err)
	if !ok || prob.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("unknown catalog id should be an invalid_argument typed error, got %v", err)
	}
}

// TestSendGuards pins send's two typed rejections: --task-id follow-up and an
// unknown context id.
func TestSendGuards(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{"echo"}
	ctx := context.Background()

	_, err := echoSend(ctx, rt, agent.SendInput{Text: "hi", ContextID: "ctx_x", TaskID: "task_x"})
	if prob, ok := errs.ProblemOf(err); !ok || prob.Subtype != errs.SubtypeFailedPrecondition {
		t.Fatalf("--task-id follow-up should be failed_precondition, got %v", err)
	}

	_, err = echoSend(ctx, rt, agent.SendInput{Text: "hi", ContextID: "ctx_missing"})
	if prob, ok := errs.ProblemOf(err); !ok || prob.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("unknown context id should be invalid_argument, got %v", err)
	}
}

// TestDeleteContext verifies deleting a context also cleans up its tasks.
func TestDeleteContext(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{"echo"}
	ctx := context.Background()
	task, err := echoSend(ctx, rt, agent.SendInput{Text: "bye"})
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteContext(ctx, rt, task.ContextID); err != nil {
		t.Fatal(err)
	}
	if _, err := getTask(ctx, rt, task.TaskID); err == nil {
		t.Fatal("after deleting the context its tasks should be unqueryable")
	}
	ctxs, err := listContexts(ctx, rt)
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

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
	// Every summary carries the enriched fields: a status timestamp and the
	// one-line digest (the last agent message). listTasks returns creation order,
	// so tasks[0] is the first turn and tasks[1] the second.
	for _, ts := range tasks {
		if ts.UpdatedAt == "" {
			t.Errorf("task summary should carry updated_at: %+v", ts)
		}
	}
	if tasks[0].Summary != "hello" {
		t.Errorf("first task summary should be the last agent message %q, got %q", "hello", tasks[0].Summary)
	}
	if tasks[1].Summary != "再来（第 2 轮）" {
		t.Errorf("second task summary should carry the round marker, got %q", tasks[1].Summary)
	}
	ctxs, err := listContexts(ctx, rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctxs) != 1 || ctxs[0].ContextID != t1.ContextID {
		t.Fatalf("should have exactly 1 context with a matching id, got %+v", ctxs)
	}
	if ctxs[0].TaskCount != 2 || ctxs[0].AwaitingInput {
		t.Errorf("context summary should roll up task_count=2, awaiting_input=false, got %+v", ctxs[0])
	}
	if ctxs[0].UpdatedAt == "" {
		t.Error("context summary should carry updated_at")
	}

	// context get NO LONGER returns a full tasks[]: it is metadata + rollup + the
	// single most-recent active_task (t2, the latest by updated_at).
	detail, err := getContext(ctx, rt, t1.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.TaskCount != 2 {
		t.Fatalf("context detail should report task_count=2, got %+v", detail)
	}
	if detail.AwaitingInput {
		t.Errorf("both tasks are completed, awaiting_input should be false: %+v", detail)
	}
	if detail.ActiveTask == nil || detail.ActiveTask.TaskID != t2.TaskID {
		t.Fatalf("active_task should be the most recent task (t2 %s), got %+v", t2.TaskID, detail.ActiveTask)
	}
	if detail.ActiveTask.Summary != "再来（第 2 轮）" {
		t.Errorf("active_task.summary should be the last agent message, got %q", detail.ActiveTask.Summary)
	}
	if detail.ActiveTask.UpdatedAt == "" {
		t.Error("active_task.updated_at should be populated")
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

// TestContextRollupPicksLatestUpdated pins the enriched-summary rollup rule: the
// active_task is the task with the LATEST updated_at (not the last created), the
// rollup counts tasks and flags awaiting_input, and an input_required active
// task's summary is its pending prompt. It seeds the store directly with
// out-of-creation-order timestamps so "latest updated_at wins" is tested
// independently of insertion order.
func TestContextRollupPicksLatestUpdated(t *testing.T) {
	swapStore(t)
	store.loaded = true // seed in-memory directly; skip the (missing) snapshot load
	store.Contexts["ctx_1"] = &contextRecord{
		AgentID: "echo", ContextID: "ctx_1", CreatedAt: "2026-07-01T00:00:00Z",
		Seq: 1, TaskIDs: []string{"t_a", "t_b", "t_c"},
	}
	store.Tasks["t_a"] = &taskRecord{AgentID: "echo", Seq: 2, Task: agent.AgentTask{
		TaskID: "t_a", ContextID: "ctx_1", State: agent.StateCompleted, IsTerminal: true,
		UpdatedAt: "2026-07-03T00:00:00Z", Messages: agentMessage("A 完成"),
	}}
	// t_b has the LATEST updated_at yet is created before t_c, and is input_required.
	store.Tasks["t_b"] = &taskRecord{AgentID: "echo", Seq: 3, Task: agent.AgentTask{
		TaskID: "t_b", ContextID: "ctx_1", State: agent.StateInputRequired,
		UpdatedAt: "2026-07-05T00:00:00Z", InputRequired: &agent.InputRequired{Prompt: "按大区还是品类拆?"},
	}}
	store.Tasks["t_c"] = &taskRecord{AgentID: "echo", Seq: 4, Task: agent.AgentTask{
		TaskID: "t_c", ContextID: "ctx_1", State: agent.StateCompleted, IsTerminal: true,
		UpdatedAt: "2026-07-04T00:00:00Z", Messages: agentMessage("C 完成"),
	}}

	rt := fakeRuntime{"echo"}
	detail, err := getContext(context.Background(), rt, "ctx_1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.TaskCount != 3 {
		t.Errorf("task_count should be 3, got %d", detail.TaskCount)
	}
	if !detail.AwaitingInput {
		t.Error("awaiting_input should be true (t_b is input_required)")
	}
	if detail.ActiveTask == nil || detail.ActiveTask.TaskID != "t_b" {
		t.Fatalf("active_task should be t_b (latest updated_at), not the last-created task, got %+v", detail.ActiveTask)
	}
	if detail.ActiveTask.Summary != "按大区还是品类拆?" {
		t.Errorf("an input_required active task's summary should be its pending prompt, got %q", detail.ActiveTask.Summary)
	}
	if detail.UpdatedAt != "2026-07-05T00:00:00Z" {
		t.Errorf("context updated_at should roll up to the latest task, got %q", detail.UpdatedAt)
	}

	// context list carries the same rollup.
	ctxs, err := listContexts(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctxs) != 1 {
		t.Fatalf("expected 1 context, got %d", len(ctxs))
	}
	if ctxs[0].UpdatedAt != "2026-07-05T00:00:00Z" || ctxs[0].TaskCount != 3 || !ctxs[0].AwaitingInput {
		t.Errorf("context summary rollup wrong: %+v", ctxs[0])
	}
}

// TestTaskSummaryText pins the digest rule: rune-safe truncation to ~100 runes,
// and that an input_required task prefers its pending prompt over the last agent
// message.
func TestTaskSummaryText(t *testing.T) {
	long := strings.Repeat("字", 250)
	got := taskSummaryText(agent.AgentTask{Messages: agentMessage(long)})
	if n := len([]rune(got)); n != summaryMaxRunes {
		t.Errorf("summary should be rune-truncated to %d runes, got %d", summaryMaxRunes, n)
	}
	prompt := taskSummaryText(agent.AgentTask{
		State:         agent.StateInputRequired,
		InputRequired: &agent.InputRequired{Prompt: "补充预算区间?"},
		Messages:      agentMessage("忽略我"),
	})
	if prompt != "补充预算区间?" {
		t.Errorf("input_required summary should be the pending prompt, got %q", prompt)
	}
}

// agentMessage builds a single agent-role text message for seeding task fixtures.
func agentMessage(text string) []agent.Message {
	return []agent.Message{{Role: "agent", Parts: []agent.Part{{Type: "text", Text: text}}}}
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

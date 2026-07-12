// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package example

import (
	"context"
	"encoding/json"
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
type fakeRuntime struct {
	agentID string
	params  map[string]string
}

func (r fakeRuntime) AgentID() string           { return r.agentID }
func (r fakeRuntime) IsBot() bool               { return false }
func (r fakeRuntime) Params() map[string]string { return r.params }
func (r fakeRuntime) CallAPI(context.Context, string, string, map[string]string, any) (json.RawMessage, error) {
	return nil, nil
}
func (r fakeRuntime) CallMultipart(context.Context, string, string, map[string]string, []agent.FilePart) (json.RawMessage, error) {
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

// TestConformance runs the shared conformance suite for every catalog entry.
func TestConformance(t *testing.T) {
	agenttest.RunConformance(t, "example", "echo")
}

func TestConformancePlanner(t *testing.T) {
	agenttest.RunConformance(t, "example", "planner")
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
	if !ec.ContextList || !ec.ContextGet || !ec.ContextDelete || !ec.TaskGet || !ec.TaskList {
		t.Errorf("echo should support context_list/get/delete + task_get/task_list, got %+v", ec)
	}
	if !(rc.ArtifactDownload && rc.FileInput && rc.TaskCancel && rc.InputRequired && rc.ContextList && rc.ContextGet && rc.ContextDelete && rc.TaskGet && rc.TaskList) {
		t.Errorf("reporter should have everything enabled, got %+v", rc)
	}
}

// TestEchoUnwiredCapabilities verifies the new model: echo simply leaves
// CancelTask / DownloadArtifact unwired and FileInput false — no refusal code.
func TestEchoUnwiredCapabilities(t *testing.T) {
	if echoSpec.CancelTask.Handler != nil {
		t.Error("echo should not wire CancelTask (task_cancel=false)")
	}
	if echoSpec.DownloadArtifact.Handler != nil {
		t.Error("echo should not wire DownloadArtifact (artifact_download=false)")
	}
	if echoSpec.FileInput {
		t.Error("echo should not accept file input (file_input=false)")
	}
}

// TestEchoMultiTurn verifies multi-turn context memory across the read verbs.
func TestEchoMultiTurn(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{agentID: "echo"}
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

// TestCrossAgentIsolation pins the load-bearing per-agent isolation guard: echo
// and reporter share one package-global store, so a task/context created under
// one agent MUST be invisible to the other agent's runtime (get/delete return a
// not-found error; list returns nothing). Without this guard
// `agent task get example:reporter <echo-task-id>` would leak echo's data.
func TestCrossAgentIsolation(t *testing.T) {
	swapStore(t)
	ctx := context.Background()
	echo := fakeRuntime{agentID: "echo"}
	reporter := fakeRuntime{agentID: "reporter"}

	t1, err := echoSend(ctx, echo, agent.SendInput{Text: "secret"})
	if err != nil {
		t.Fatalf("echo send: %v", err)
	}

	// reporter must not read/delete echo's task or context.
	if _, err := getTask(ctx, reporter, t1.TaskID); err == nil {
		t.Error("reporter must not read echo's task (cross-agent leak)")
	}
	if _, err := getContext(ctx, reporter, t1.ContextID); err == nil {
		t.Error("reporter must not read echo's context (cross-agent leak)")
	}
	if err := deleteContext(ctx, reporter, t1.ContextID); err == nil {
		t.Error("reporter must not delete echo's context (cross-agent leak)")
	}
	if tasks, _ := listTasks(ctx, reporter, ""); len(tasks) != 0 {
		t.Errorf("reporter should see no echo tasks, got %d", len(tasks))
	}
	if ctxs, _ := listContexts(ctx, reporter); len(ctxs) != 0 {
		t.Errorf("reporter should see no echo contexts, got %d", len(ctxs))
	}

	// echo still sees its own data, and its context survived reporter's delete.
	if _, err := getTask(ctx, echo, t1.TaskID); err != nil {
		t.Errorf("echo must still read its own task: %v", err)
	}
	if _, err := getContext(ctx, echo, t1.ContextID); err != nil {
		t.Errorf("echo's context must survive a cross-agent delete attempt: %v", err)
	}
}

// TestStateSurvivesReload pins the cross-process semantics via the shared snapshot.
func TestStateSurvivesReload(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{agentID: "echo"}
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

// TestPlannerDecisionFlow drives the input_required HITL loop end to end on the
// reference provider: the first send opens a non-terminal decision, answering it
// by option completes the task and records the winning option, and a second
// answer to the same decision is a conflict (the arbitration path).
func TestPlannerDecisionFlow(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{agentID: "planner"}
	ctx := context.Background()

	t1, err := plannerSend(ctx, rt, agent.SendInput{Text: "出个季度报表"})
	if err != nil {
		t.Fatalf("planner open send: %v", err)
	}
	if t1.State != agent.StateInputRequired || t1.InputRequired == nil {
		t.Fatalf("first send should open an input_required decision, got %+v", t1)
	}
	ir := t1.InputRequired
	if ir.DecisionID == "" || ir.InputType != agent.InputTypeSingleSelect || len(ir.Options) != 2 {
		t.Fatalf("decision should carry id + single_select + 2 options, got %+v", ir)
	}
	if ir.Submitted {
		t.Error("a freshly opened decision should not be submitted")
	}

	done, err := plannerSend(ctx, rt, agent.SendInput{
		ContextID: t1.ContextID, TaskID: t1.TaskID,
		DecisionID: ir.DecisionID, OptionIDs: []string{"by_region"},
	})
	if err != nil {
		t.Fatalf("answering the decision: %v", err)
	}
	if done.State != agent.StateCompleted {
		t.Fatalf("answered task should be completed, got %s", done.State)
	}
	if done.InputRequired == nil || !done.InputRequired.Submitted || done.InputRequired.SubmittedOptionID != "by_region" {
		t.Errorf("decision should be marked submitted with the winning option, got %+v", done.InputRequired)
	}

	_, err = plannerSend(ctx, rt, agent.SendInput{
		ContextID: t1.ContextID, TaskID: t1.TaskID,
		DecisionID: ir.DecisionID, OptionIDs: []string{"by_category"},
	})
	if err == nil {
		t.Fatal("answering an already-submitted decision should conflict")
	}
	if p, ok := errs.ProblemOf(err); !ok || p.Subtype != errs.SubtypeConflict {
		t.Fatalf("re-answer should be a conflict, got %+v (%v)", p, err)
	}
}

// TestPlannerRejectsUnknownOption pins that an option_id not in the decision is
// invalid_argument and leaves the decision unanswered.
func TestPlannerRejectsUnknownOption(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{agentID: "planner"}
	ctx := context.Background()
	t1, err := plannerSend(ctx, rt, agent.SendInput{Text: "出报表"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = plannerSend(ctx, rt, agent.SendInput{
		ContextID: t1.ContextID, TaskID: t1.TaskID,
		DecisionID: t1.InputRequired.DecisionID, OptionIDs: []string{"nonexistent"},
	})
	if p, ok := errs.ProblemOf(err); !ok || p.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("unknown option should be invalid_argument, got %+v (%v)", p, err)
	}
	got, err := getTask(ctx, rt, t1.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != agent.StateInputRequired || got.InputRequired.Submitted {
		t.Errorf("a rejected option must not change the decision, got state=%s submitted=%v", got.State, got.InputRequired.Submitted)
	}
}

// TestReporterParamsBinding locks the declaration↔consumption contract: the
// reporterSendParams struct tags must reference params declared on send with
// compatible kinds (a renamed/retyped declaration fails here in CI, not as a
// silent zero value at runtime).
func TestReporterParamsBinding(t *testing.T) {
	agenttest.CheckParamsBinding[reporterSendParams](t, &reporterSpec, agent.VerbSend)
}

// TestReporterConsumesParams drives reporterSend with framework-style resolved
// params (defaults backfilled) and pins that BindParams feeds the reply: the
// default shape keeps the historical reply, a non-default format changes it.
func TestReporterConsumesParams(t *testing.T) {
	swapStore(t)
	ctx := context.Background()

	// defaults → historical reply, byte-identical
	rt := fakeRuntime{agentID: "reporter", params: map[string]string{"report_format": "csv", "quarters": "4"}}
	task, err := reporterSend(ctx, rt, agent.SendInput{Text: "报表"})
	if err != nil {
		t.Fatal(err)
	}
	if got := agentReply(t, task); !strings.HasPrefix(got, "报表已生成：quarterly_report.csv") {
		t.Fatalf("default params should keep the historical reply, got %q", got)
	}

	// non-default format → the reply reflects the params
	rt2 := fakeRuntime{agentID: "reporter", params: map[string]string{"report_format": "xlsx", "quarters": "6"}}
	task2, err := reporterSend(ctx, rt2, agent.SendInput{Text: "报表"})
	if err != nil {
		t.Fatal(err)
	}
	if got := agentReply(t, task2); !strings.Contains(got, "xlsx") || !strings.Contains(got, "6 个季度") {
		t.Fatalf("params should feed the reply, got %q", got)
	}
}

// TestReporterRenderObject drives the object param end to end on the reference
// provider: framework-style resolved leaves reach the hook, the nested struct
// binds, and the reply reflects them.
func TestReporterRenderObject(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{agentID: "reporter", params: map[string]string{
		"report_format": "csv", "quarters": "4",
		"render.theme": "dark", "render.watermark": "true",
	}}
	task, err := reporterSend(context.Background(), rt, agent.SendInput{Text: "报表"})
	if err != nil {
		t.Fatal(err)
	}
	if got := agentReply(t, task); !strings.Contains(got, "dark 主题，含水印") {
		t.Fatalf("render object should feed the reply, got %q", got)
	}
}

// TestReporterArtifactFlow verifies the full artifact chain.
func TestReporterArtifactFlow(t *testing.T) {
	swapStore(t)
	rt := fakeRuntime{agentID: "reporter"}
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
	rt := fakeRuntime{agentID: "reporter"}
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
	rt := fakeRuntime{agentID: "echo"}
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
	rt := fakeRuntime{agentID: "echo"}
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

	rt := fakeRuntime{agentID: "echo"}
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

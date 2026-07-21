// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package example

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/agents"
	"github.com/larksuite/cli/internal/vfs"
)

// ============================================================================
// In-memory state machine (teaching focus: concurrency safety of package-level
// state + the CLI process boundary)
//
// A real provider's context/task state lives on the server, so the adapter is
// naturally stateless; example is a pure mock and must manage state itself. Two
// disciplines the integrator needs to know:
//
//  1. Concurrency safety: package-level mutable state must be locked. A single
//     coarse-grained Mutex covers all reads and writes here — the mock does not
//     chase throughput; correctness comes first.
//  2. CLI process boundary: every lark-cli command is a fresh process, so a pure
//     in-memory map does not survive a single command — after `send`, a
//     `task get` would find nothing. So a lazy JSON snapshot layer sits beneath
//     the in-memory map (under os.TempDir, last-writer-wins) to make the offline
//     demo chain work across commands. A real provider neither needs nor should
//     have this layer — it is a mock-only demo device.
//
// Note that the snapshot is loaded lazily (only on the first real read/write of
// state): provider registration is a pure declarative Register(Provider) call
// (see agent/register.go) with no construction and no side effects, so nothing
// touches store at registration time — the snapshot is read on the first hook
// invocation, not at init.
// ============================================================================

// taskRecord is a task's storage form: a full AgentTask snapshot + owning agent
// + creation sequence number (list output sorts by creation order to guarantee
// stable enumeration).
type taskRecord struct {
	AgentID string           `json:"agent_id"`
	Seq     int              `json:"seq"`
	Task    agents.AgentTask `json:"task"`
	// Accepted is the acceptance record of the task's question group (§10.1 key
	// encoding), written atomically with the state transition: it is what a
	// late/second submission gets echoed back as resolved_answers — the
	// machine-readable "who won" signal.
	Accepted map[string][]string `json:"accepted,omitempty"`
}

// contextRecord is a multi-turn context's storage form. TaskIDs is appended in
// creation order — len(TaskIDs)+1 is the next round number, which echo uses to
// demonstrate "context memory".
type contextRecord struct {
	AgentID   string   `json:"agent_id"`
	ContextID string   `json:"context_id"`
	CreatedAt string   `json:"created_at"`
	Title     string   `json:"title,omitempty"`
	Seq       int      `json:"seq"`
	TaskIDs   []string `json:"task_ids"`
}

// memoryStore is the package-level state machine itself: mu covers all fields;
// path is the JSON snapshot location; loaded ensures the snapshot is read only
// once, on first access.
type memoryStore struct {
	mu     sync.Mutex
	path   string
	loaded bool

	Contexts map[string]*contextRecord `json:"contexts"`
	Tasks    map[string]*taskRecord    `json:"tasks"`
	NextSeq  int                       `json:"next_seq"`
}

// store is the package-level singleton. Tests use swapStoreForTest to replace it
// with an instance pointing at t.TempDir, avoiding cross-contamination between
// tests and between tests and the local demo state.
var store = newMemoryStore(filepath.Join(os.TempDir(), "lark-cli-example-agents.json"))

func newMemoryStore(path string) *memoryStore {
	return &memoryStore{
		path:     path,
		Contexts: map[string]*contextRecord{},
		Tasks:    map[string]*taskRecord{},
	}
}

// loadLocked lazily reads in the snapshot (the caller must already hold the
// lock). A missing / corrupt snapshot is uniformly treated as empty state — the
// mock's demo data is not worth erroring over, so it just starts fresh.
func (s *memoryStore) loadLocked() {
	if s.loaded {
		return
	}
	s.loaded = true
	data, err := vfs.ReadFile(s.path)
	if err != nil {
		return
	}
	var snap memoryStore
	if json.Unmarshal(data, &snap) != nil {
		return
	}
	if snap.Contexts != nil {
		s.Contexts = snap.Contexts
	}
	if snap.Tasks != nil {
		s.Tasks = snap.Tasks
	}
	s.NextSeq = snap.NextSeq
}

// saveLocked writes the current state back to the snapshot (the caller must
// already hold the lock). A write failure returns a typed internal error
// (storage subtype) — the mock does not swallow errors either: silently losing
// state would make the next command report "task not found", which is harder to
// diagnose than a clear error.
func (s *memoryStore) saveLocked() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return errs.NewInternalError(errs.SubtypeStorage, "序列化 example 状态失败: %v", err).WithCause(err)
	}
	if err := vfs.WriteFile(s.path, data, 0o600); err != nil {
		return errs.NewInternalError(errs.SubtypeStorage, "写 example 状态快照失败: %v", err).WithCause(err)
	}
	return nil
}

// newID generates a random id that is safe for [A-Za-z0-9_-]. The character set
// deliberately aligns with the command layer's meta.next interpolation
// allowlist (cmd/agent/send.go safeNextID): the id is spliced into a command
// string "the AI copies and runs", and an id with shell metacharacters would
// cause the whole hint to be suppressed.
func newID(prefix string) string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand being unavailable is an environment-level failure; the mock
		// degrades to a timestamp that still satisfies the character set.
		return prefix + "_" + time.Now().UTC().Format("20060102150405")
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

// newGroupSuffix mints the per-group question-id suffix (4 hex chars,
// key-safe): random at GROUP-CREATION time — the randomness is what makes a
// successor group's minted ids necessarily differ (§6.2 cross-group
// uniqueness), which in turn is what makes a stale retry hit unknown_question
// instead of silently answering the next group.
func newGroupSuffix() string {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("0405")
	}
	return hex.EncodeToString(b[:])
}

// createContext creates a new context and returns its id (the first-turn send goes here).
func (s *memoryStore) createContext(agentID, title string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	id := newID("ctx")
	s.NextSeq++
	s.Contexts[id] = &contextRecord{
		AgentID:   agentID,
		ContextID: id,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Title:     title,
		Seq:       s.NextSeq,
	}
	return id, s.saveLocked()
}

// createTask appends a task under ctxID: validate context ownership → compute
// the round (which task number in this conversation) → call build under the lock
// to construct the task → insert and write the snapshot. build runs inside the
// lock to guarantee "compute the round" and "store the task" are atomic, so two
// concurrent sends never get the same round.
// An unknown / cross-agents context id returns a typed validation error (teaching
// point: every error a provider returns must be typed — a bare error would land
// as internal/exit 5, whereas this is clearly "the caller passed a wrong
// argument", semantically invalid_argument/exit 2, and the AI relies on this
// classification to decide between "fix the argument and retry" and "report an
// environment failure").
func (s *memoryStore) createTask(agentID, ctxID string, build func(round int) agents.AgentTask) (agents.AgentTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	ctx, ok := s.Contexts[ctxID]
	if !ok || ctx.AgentID != agentID {
		return agents.AgentTask{}, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"未知的 context id '%s'（example:%s 名下不存在）", ctxID, agentID).
			WithHint("运行 lark-cli agents context list example:%s 查看现有会话", agentID)
	}
	task := build(len(ctx.TaskIDs) + 1)
	// Stamp lifecycle timestamps at creation. Example tasks are born terminal, so
	// created_at == updated_at; a real provider bumps updated_at on every status
	// change (see setTaskState). RFC3339 UTC strings are fixed-width, so their
	// lexicographic order equals chronological order (relied on by the rollup).
	now := time.Now().UTC().Format(time.RFC3339)
	task.CreatedAt = now
	task.UpdatedAt = now
	s.NextSeq++
	s.Tasks[task.TaskID] = &taskRecord{AgentID: agentID, Seq: s.NextSeq, Task: task}
	ctx.TaskIDs = append(ctx.TaskIDs, task.TaskID)
	return task, s.saveLocked()
}

// getTask fetches a task snapshot by id (returns a copy by value, so the command
// layer's in-place edits like normalizeTask do not write through to store). A
// cross-agents task is treated as "not found", without leaking another agent's state.
func (s *memoryStore) getTask(agentID, taskID string) (agents.AgentTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	rec, ok := s.Tasks[taskID]
	if !ok || rec.AgentID != agentID {
		return agents.AgentTask{}, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"未知的 task id '%s'（example:%s 名下不存在）", taskID, agentID).
			WithHint("运行 lark-cli agents task list example:%s 查看现有任务", agentID)
	}
	task := rec.Task
	// AgentTask is returned by value, but InputRequired is a pointer — clone it
	// so the command layer's in-place normalization can never write through into
	// the store (one-process runs must behave like per-process runs).
	task.InputRequired = cloneGroup(rec.Task.InputRequired)
	return task, nil
}

// cloneGroup deep-copies a question group (nil-safe).
func cloneGroup(ir *agents.InputRequired) *agents.InputRequired {
	if ir == nil {
		return nil
	}
	out := *ir
	out.Questions = make([]agents.Question, len(ir.Questions))
	for i, q := range ir.Questions {
		out.Questions[i] = q
		out.Questions[i].Options = append([]agents.Option(nil), q.Options...)
	}
	return &out
}

// setTaskState updates a task's state (used by reporter's cancel).
func (s *memoryStore) setTaskState(taskID string, state agents.TaskState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	rec, ok := s.Tasks[taskID]
	if !ok {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "未知的 task id '%s'", taskID)
	}
	rec.Task.State = state
	rec.Task.IsTerminal = state.IsTerminal()
	rec.Task.UpdatedAt = time.Now().UTC().Format(time.RFC3339) // status changed ⇒ record when
	return s.saveLocked()
}

// answerGroup applies a group answer (§10.1 key encoding) to a task's pending
// input_required question group. It is the mock's stand-in for a STRICT-posture
// server (a form backend): every question required, bare values validated
// against the stored options, single-select cardinality enforced, the skip
// option exclusive — with every violation collected into ONE ValidationError
// (params[] entries with the Reason enum + the question declaration as Spec) so
// the caller fixes everything in a single resend. A tolerant LLM-backed
// provider may instead consume partial/free answers — validation POLICY is the
// provider's own; only the error FORMAT here is contractual.
//
// Acceptance is atomic under the store lock (validate → record Accepted →
// leave input_required in one critical section, the reply message inside it) —
// two racing submissions get exactly one winner; the loser (and any late
// retry) gets failed_precondition carrying resolved_answers, the
// machine-readable "already decided, here is what won" signal.
func (s *memoryStore) answerGroup(agentID, ctxID, taskID string, answers map[string][]string, remark string) (agents.AgentTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	rec, ok := s.Tasks[taskID]
	if !ok || rec.AgentID != agentID {
		return agents.AgentTask{}, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"未知的 task id '%s'（example:%s 名下不存在）", taskID, agentID).
			WithHint("运行 lark-cli agents task list example:%s 查看现有任务", agentID)
	}
	// context_id+task_id is the group's unique address (§2.1) — the CLI forces
	// both flags for that binding, so honoring only half of it here would teach
	// integrators to silently ignore the other half.
	if ctxID != "" && ctxID != rec.Task.ContextID {
		return agents.AgentTask{}, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"context_id '%s' 与任务 '%s' 所属会话不符", ctxID, taskID).
			WithHint("用 lark-cli agents task get example:%s %s 确认该任务的 context_id", agentID, taskID)
	}
	ir := rec.Task.InputRequired
	if rec.Task.State != agents.StateInputRequired || ir == nil {
		e := errs.NewValidationError(errs.SubtypeFailedPrecondition,
			"任务 '%s' 已不在等待输入", taskID).
			WithHint("用 lark-cli agents task get example:%s %s 查看当前状态与结果", agentID, taskID)
		if rec.Accepted != nil {
			// The group was already resolved (another endpoint, or a retry whose
			// first attempt landed): echo what won, machine-readable.
			e = e.WithResolvedAnswers(rec.Accepted)
		}
		return agents.AgentTask{}, e
	}

	byID := make(map[string]agents.Question, len(ir.Questions))
	currentIDs := make([]string, 0, len(ir.Questions))
	for _, q := range ir.Questions {
		byID[q.QuestionID] = q
		currentIDs = append(currentIDs, q.QuestionID)
	}

	// Deterministic violation order: sorted answer keys, then missing questions
	// in group order.
	keys := make([]string, 0, len(answers))
	for k := range answers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var viols []errs.InvalidParam
	answered := make(map[string]bool, len(answers))
	for _, key := range keys {
		values := answers[key]
		qid, isText := agents.SplitAnswerKey(key)
		q, known := byID[qid]
		if !known {
			// A stale retry (the group changed under the caller) lands exactly
			// here — Suggestions carries the CURRENT group's keys so the caller
			// can tell "typo" from "new group" without a discovery round-trip.
			viols = append(viols, errs.InvalidParam{Name: key, Reason: "unknown_question",
				Suggestions: currentIDs})
			continue
		}
		answered[qid] = true
		if isText {
			// Free text is always consumable here (the strict-but-LLM-ish demo
			// posture); a pure form backend MAY reject it with reason
			// invalid_option-style clarity instead — never silently drop it.
			if len(q.Options) == 0 {
				if _, both := answers[qid]; both {
					viols = append(viols, errs.InvalidParam{Name: key, Reason: "conflict", Spec: q})
				}
			}
			continue
		}
		if len(q.Options) == 0 {
			// Text question answered via the bare-value alias: legal, but only one
			// text per question.
			if len(values) > 1 {
				viols = append(viols, errs.InvalidParam{Name: key, Reason: "count_violation", Spec: q})
			}
			continue
		}
		picked := 0
		for _, v := range values {
			if _, ok := optionLabel(q.Options, v); !ok {
				viols = append(viols, errs.InvalidParam{Name: key, Reason: "invalid_option", Spec: q})
			} else {
				picked++
			}
		}
		if !q.MultiSelect && len(values) > 1 {
			viols = append(viols, errs.InvalidParam{Name: key, Reason: "count_violation", Spec: q})
		}
		if picked > 1 && hasValue(values, "skip") {
			// planner's own policy: its skip option means "let the agent decide"
			// and is exclusive with real picks.
			viols = append(viols, errs.InvalidParam{Name: key, Reason: "conflict", Spec: q})
		}
	}
	// Strict posture: every question of the group is required.
	for _, q := range ir.Questions {
		if !answered[q.QuestionID] {
			viols = append(viols, errs.InvalidParam{Name: q.QuestionID, Reason: "missing", Spec: q})
		}
	}
	if len(viols) > 0 {
		return agents.AgentTask{}, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"%d 个答案有问题", len(viols)).
			WithParams(viols...).
			WithHint("按 params 里的题目声明修正后整组重发（含未报错的题）")
	}

	// Atomic acceptance: record + reply + state transition in one critical
	// section, snapshot write last.
	rec.Accepted = answers
	rec.Task.State = agents.StateCompleted
	rec.Task.IsTerminal = true
	if remark != "" {
		// The §4.1 message-level remark (--text alongside --answer) is part of
		// the user's message — record it, never silently drop it (§6.4).
		rec.Task.Messages = append(rec.Task.Messages, agents.Message{
			Role: "user", Parts: []agents.Part{{Type: "text", Text: remark}},
		})
	}
	rec.Task.Messages = append(rec.Task.Messages, agents.Message{
		Role:  "agent",
		Parts: []agents.Part{{Type: "text", Text: acceptanceReply(ir, answers)}},
	})
	rec.Task.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return rec.Task, s.saveLocked()
}

// acceptanceReply composes the post-acceptance agent message, resolving option
// ids back to labels from the stored group — the §6.1 store-and-resolve
// pattern: the wire carried keys, the business reads values.
func acceptanceReply(ir *agents.InputRequired, answers map[string][]string) string {
	var parts []string
	for _, q := range ir.Questions {
		var vals []string
		for _, v := range answers[q.QuestionID] {
			if label, ok := optionLabel(q.Options, v); ok {
				vals = append(vals, label)
			} else {
				vals = append(vals, v)
			}
		}
		vals = append(vals, answers[q.QuestionID+agents.AnswerTextSuffix]...)
		if len(vals) > 0 {
			parts = append(parts, q.Question+"「"+strings.Join(vals, "、")+"」")
		}
	}
	return "已按答复出报表：" + strings.Join(parts, "；")
}

// hasValue reports whether vals contains v.
func hasValue(vals []string, v string) bool {
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}

// optionLabel returns the label of optionID within opts (ok=false if not found).
func optionLabel(opts []agents.Option, optionID string) (string, bool) {
	for _, o := range opts {
		if o.OptionID == optionID {
			return o.Label, true
		}
	}
	return "", false
}

// pageWindow computes the [lo,hi) slice bounds and the resulting PageInfo for an
// offset-cursor paginated list of `total` items. The token is an opaque offset —
// strconv.Itoa of the first item's index; an unparseable / negative token is
// leniently treated as offset 0 (the store is a mock, so it does not reject a bad
// cursor). Size<=0 returns all remaining items (the CLI always passes ≥1). The
// NextToken is the offset just past this page (lo+len), set only when more items
// remain.
func pageWindow(total int, page agents.PageParams) (lo, hi int, info agents.PageInfo) {
	if page.Token != "" {
		if n, err := strconv.Atoi(page.Token); err == nil && n > 0 {
			lo = n
		}
	}
	if lo > total {
		lo = total
	}
	hi = total
	if page.Size > 0 && lo+page.Size < total {
		hi = lo + page.Size
	}
	if hi < total {
		info = agents.PageInfo{NextToken: strconv.Itoa(hi), HasMore: true}
	}
	return lo, hi, info
}

// listTasks lists an agent's task summaries, optionally filtered by contextID
// (empty string means no filter), MOST-RECENT-FIRST (Seq descending — Seq grows
// with creation, so descending is newest first; example tasks are terminal at
// creation so Seq desc equals UpdatedAt desc), then paginated by page. IsTerminal
// is carried along here for convenience, but the command layer re-derives it from
// State via normalizeTask* (single source), so the integrator need not worry
// about this field.
func (s *memoryStore) listTasks(agentID, contextID string, page agents.PageParams) ([]agents.TaskSummary, agents.PageInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	recs := make([]*taskRecord, 0, len(s.Tasks))
	for _, rec := range s.Tasks {
		if rec.AgentID != agentID {
			continue
		}
		if contextID != "" && rec.Task.ContextID != contextID {
			continue
		}
		recs = append(recs, rec)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Seq > recs[j].Seq })
	lo, hi, info := pageWindow(len(recs), page)
	out := make([]agents.TaskSummary, 0, hi-lo)
	for _, rec := range recs[lo:hi] {
		out = append(out, taskSummaryOf(rec.Task))
	}
	return out, info
}

// listContexts lists an agent's context summaries, MOST-RECENT-FIRST (Seq
// descending — newest first), then paginated by page.
func (s *memoryStore) listContexts(agentID string, page agents.PageParams) ([]agents.ContextSummary, agents.PageInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	recs := make([]*contextRecord, 0, len(s.Contexts))
	for _, ctx := range s.Contexts {
		if ctx.AgentID == agentID {
			recs = append(recs, ctx)
		}
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Seq > recs[j].Seq })
	lo, hi, info := pageWindow(len(recs), page)
	out := make([]agents.ContextSummary, 0, hi-lo)
	for _, ctx := range recs[lo:hi] {
		updatedAt, _, awaiting, _ := s.contextRollupLocked(ctx)
		out = append(out, agents.ContextSummary{
			ContextID:     ctx.ContextID,
			CreatedAt:     ctx.CreatedAt,
			UpdatedAt:     updatedAt,
			Title:         ctx.Title,
			AwaitingInput: awaiting,
		})
	}
	return out, info
}

// getContext returns a context's detail: metadata plus a rollup (updated_at,
// task_count, awaiting_input) and the single most-actionable ActiveTask (the task
// with the latest updated_at; nil for an empty context). It deliberately does NOT
// enumerate every task — the full list is `listTasks(agentID, ctxID)` behind
// `agents task list --context-id`.
func (s *memoryStore) getContext(agentID, ctxID string) (*agents.ContextDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	ctx, ok := s.Contexts[ctxID]
	if !ok || ctx.AgentID != agentID {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"未知的 context id '%s'（example:%s 名下不存在）", ctxID, agentID).
			WithHint("运行 lark-cli agents context list example:%s 查看现有会话", agentID)
	}
	updatedAt, taskCount, awaiting, active := s.contextRollupLocked(ctx)
	detail := &agents.ContextDetail{
		ContextID: ctx.ContextID,
		CreatedAt: ctx.CreatedAt,
		UpdatedAt: updatedAt,
		Title:     ctx.Title,
		// The mock can always count its tasks; a real provider whose backend
		// does not return a total leaves TaskCount nil (unknown ≠ 0).
		TaskCount:     &taskCount,
		AwaitingInput: awaiting,
	}
	if active != nil {
		summary := taskSummaryOf(active.Task)
		detail.ActiveTask = &summary
	}
	return detail, nil
}

// deleteContext deletes a context and its tasks (a destructive operation, already gated by --yes in the command layer).
func (s *memoryStore) deleteContext(agentID, ctxID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	ctx, ok := s.Contexts[ctxID]
	if !ok || ctx.AgentID != agentID {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"未知的 context id '%s'（example:%s 名下不存在）", ctxID, agentID).
			WithHint("运行 lark-cli agents context list example:%s 查看现有会话", agentID)
	}
	for _, tid := range ctx.TaskIDs {
		delete(s.Tasks, tid)
	}
	delete(s.Contexts, ctxID)
	return s.saveLocked()
}

// ── Derived rollups (the enriched-summary provider side) ──

// summaryMaxRunes is the rune budget for a task Summary — a one-line content
// digest, not full content. Truncation is rune-safe so a multibyte character is
// never cut in half.
const summaryMaxRunes = 100

// contextRollupLocked derives a context's summary fields from its tasks (the
// caller must already hold the lock). updatedAt is the newest task updated_at,
// falling back to the context's created_at when it has no tasks; awaitingInput is
// set when any task sits in input_required/auth_required; active is the task with
// the latest updated_at (ties broken by creation order so it is deterministic),
// nil when the context is empty.
func (s *memoryStore) contextRollupLocked(ctx *contextRecord) (updatedAt string, taskCount int, awaitingInput bool, active *taskRecord) {
	updatedAt = ctx.CreatedAt
	for _, tid := range ctx.TaskIDs {
		rec, ok := s.Tasks[tid]
		if !ok {
			continue
		}
		taskCount++
		if rec.Task.UpdatedAt > updatedAt { // fixed-width RFC3339 UTC ⇒ lexicographic == chronological
			updatedAt = rec.Task.UpdatedAt
		}
		if isAwaiting(rec.Task.State) {
			awaitingInput = true
		}
		if active == nil || rec.Task.UpdatedAt > active.Task.UpdatedAt ||
			(rec.Task.UpdatedAt == active.Task.UpdatedAt && rec.Seq > active.Seq) {
			active = rec
		}
	}
	return updatedAt, taskCount, awaitingInput, active
}

// isAwaiting reports whether a state is paused waiting on the caller (the
// awaiting_input rollup bit).
func isAwaiting(state agents.TaskState) bool {
	return state == agents.StateInputRequired || state == agents.StateAuthRequired
}

// taskSummaryOf projects a stored task into its list/active summary, carrying the
// timestamp and the one-line content digest alongside the identity fields.
func taskSummaryOf(task agents.AgentTask) agents.TaskSummary {
	return agents.TaskSummary{
		TaskID:     task.TaskID,
		ContextID:  task.ContextID,
		State:      task.State,
		IsTerminal: task.IsTerminal,
		UpdatedAt:  task.UpdatedAt,
		Summary:    taskSummaryText(task),
	}
}

// taskSummaryText is the one-line content digest: the pending group's triage
// digest (§3.3: label else first question, question count suffixed) for a task
// awaiting input, otherwise the last agent message's text. It returns RAW text
// (only rune-truncated) — ANSI-stripping + flattening for pretty/TSV is the
// command layer's job, and it is empty when nothing is available.
func taskSummaryText(task agents.AgentTask) string {
	if task.State == agents.StateInputRequired && task.InputRequired != nil {
		if s := task.InputRequired.SummaryText(); s != "" {
			return truncateRunes(s, summaryMaxRunes)
		}
	}
	for i := len(task.Messages) - 1; i >= 0; i-- {
		if task.Messages[i].Role != "agent" {
			continue
		}
		for _, p := range task.Messages[i].Parts {
			if p.Type == "text" && p.Text != "" {
				return truncateRunes(p.Text, summaryMaxRunes)
			}
		}
	}
	return ""
}

// truncateRunes cuts s to at most max runes (rune-safe, no character split). It
// does not append an ellipsis: the Summary is meant to be raw text.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

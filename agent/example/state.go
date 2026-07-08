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
	"sync"
	"time"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/agent"
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
//  1. Concurrency safety: provider instances may be constructed / called
//     concurrently (e.g. list's probe alongside the real call), so package-level
//     mutable state must be locked. A single coarse-grained Mutex covers all
//     reads and writes here — the mock does not chase throughput; correctness comes first.
//  2. CLI process boundary: every lark-cli command is a fresh process, so a pure
//     in-memory map does not survive a single command — after `send`, a
//     `task get` would find nothing. So a lazy JSON snapshot layer sits beneath
//     the in-memory map (under os.TempDir, last-writer-wins) to make the offline
//     demo chain work across commands. A real provider neither needs nor should
//     have this layer — it is a mock-only demo device.
//
// Note that the snapshot is loaded lazily (only on the first real read/write of
// state): Register's zero-value Deps probe constructs a provider once at
// registration time, and construction must have no side effects (the registry.go
// contract), so Factory / Card / ListAgents must not touch store.
// ============================================================================

// taskRecord is a task's storage form: a full AgentTask snapshot + owning agent
// + creation sequence number (list output sorts by creation order to guarantee
// stable enumeration).
type taskRecord struct {
	AgentID string          `json:"agent_id"`
	Seq     int             `json:"seq"`
	Task    agent.AgentTask `json:"task"`
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
var store = newMemoryStore(filepath.Join(os.TempDir(), "lark-cli-example-agent.json"))

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
// An unknown / cross-agent context id returns a typed validation error (teaching
// point: every error a provider returns must be typed — a bare error would land
// as internal/exit 5, whereas this is clearly "the caller passed a wrong
// argument", semantically invalid_argument/exit 2, and the AI relies on this
// classification to decide between "fix the argument and retry" and "report an
// environment failure").
func (s *memoryStore) createTask(agentID, ctxID string, build func(round int) agent.AgentTask) (agent.AgentTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	ctx, ok := s.Contexts[ctxID]
	if !ok || ctx.AgentID != agentID {
		return agent.AgentTask{}, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"未知的 context id '%s'（example:%s 名下不存在）", ctxID, agentID).
			WithHint("运行 lark-cli agent context list example:%s 查看现有会话", agentID)
	}
	task := build(len(ctx.TaskIDs) + 1)
	s.NextSeq++
	s.Tasks[task.TaskID] = &taskRecord{AgentID: agentID, Seq: s.NextSeq, Task: task}
	ctx.TaskIDs = append(ctx.TaskIDs, task.TaskID)
	return task, s.saveLocked()
}

// getTask fetches a task snapshot by id (returns a copy by value, so the command
// layer's in-place edits like normalizeTask do not write through to store). A
// cross-agent task is treated as "not found", without leaking another agent's state.
func (s *memoryStore) getTask(agentID, taskID string) (agent.AgentTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	rec, ok := s.Tasks[taskID]
	if !ok || rec.AgentID != agentID {
		return agent.AgentTask{}, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"未知的 task id '%s'（example:%s 名下不存在）", taskID, agentID).
			WithHint("运行 lark-cli agent task list example:%s 查看现有任务", agentID)
	}
	return rec.Task, nil
}

// setTaskState updates a task's state (used by reporter's cancel).
func (s *memoryStore) setTaskState(taskID string, state agent.TaskState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	rec, ok := s.Tasks[taskID]
	if !ok {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "未知的 task id '%s'", taskID)
	}
	rec.Task.State = state
	rec.Task.IsTerminal = state.IsTerminal()
	return s.saveLocked()
}

// listTasks lists an agent's task summaries, optionally filtered by contextID
// (empty string means no filter), output in creation order. IsTerminal is
// carried along here for convenience, but the command layer re-derives it from
// State via normalizeTask* (single source), so the integrator need not worry
// about this field.
func (s *memoryStore) listTasks(agentID, contextID string) []agent.TaskSummary {
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
	sort.Slice(recs, func(i, j int) bool { return recs[i].Seq < recs[j].Seq })
	out := make([]agent.TaskSummary, 0, len(recs))
	for _, rec := range recs {
		out = append(out, agent.TaskSummary{
			TaskID:     rec.Task.TaskID,
			ContextID:  rec.Task.ContextID,
			State:      rec.Task.State,
			IsTerminal: rec.Task.IsTerminal,
		})
	}
	return out
}

// listContexts lists an agent's context summaries, output in creation order.
func (s *memoryStore) listContexts(agentID string) []agent.ContextSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	recs := make([]*contextRecord, 0, len(s.Contexts))
	for _, ctx := range s.Contexts {
		if ctx.AgentID == agentID {
			recs = append(recs, ctx)
		}
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Seq < recs[j].Seq })
	out := make([]agent.ContextSummary, 0, len(recs))
	for _, ctx := range recs {
		out = append(out, agent.ContextSummary{
			ContextID: ctx.ContextID,
			CreatedAt: ctx.CreatedAt,
			Title:     ctx.Title,
		})
	}
	return out
}

// getContext returns a context's detail (including its task summaries, in creation order).
func (s *memoryStore) getContext(agentID, ctxID string) (*agent.ContextDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	ctx, ok := s.Contexts[ctxID]
	if !ok || ctx.AgentID != agentID {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"未知的 context id '%s'（example:%s 名下不存在）", ctxID, agentID).
			WithHint("运行 lark-cli agent context list example:%s 查看现有会话", agentID)
	}
	detail := &agent.ContextDetail{
		ContextID: ctx.ContextID,
		CreatedAt: ctx.CreatedAt,
		Title:     ctx.Title,
	}
	for _, tid := range ctx.TaskIDs {
		if rec, ok := s.Tasks[tid]; ok {
			detail.Tasks = append(detail.Tasks, agent.TaskSummary{
				TaskID:     rec.Task.TaskID,
				ContextID:  rec.Task.ContextID,
				State:      rec.Task.State,
				IsTerminal: rec.Task.IsTerminal,
			})
		}
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
			WithHint("运行 lark-cli agent context list example:%s 查看现有会话", agentID)
	}
	for _, tid := range ctx.TaskIDs {
		delete(s.Tasks, tid)
	}
	delete(s.Contexts, ctxID)
	return s.saveLocked()
}

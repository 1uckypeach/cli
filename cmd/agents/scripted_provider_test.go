// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agents

import (
	"context"
	"sync"
	"testing"

	iagents "github.com/larksuite/cli/internal/agents"
)

// scriptedHooks scripts a fake provider's behavior per test. Each hook maps to
// one AgentSpec verb; an unset hook that gets called panics — a tripwire against
// a test reaching an unexpected provider path. The command-layer contracts under
// test (envelope shape, watch exit codes, meta.next, pretty rendering, error
// propagation) are provider-neutral, so the scripted hooks ignore the Runtime.
type scriptedHooks struct {
	send             func(in iagents.SendInput) (*iagents.AgentTask, error)
	getTask          func(taskID string) (*iagents.AgentTask, error)
	listTasks        func(contextID string, page iagents.PageParams) ([]iagents.TaskSummary, iagents.PageInfo, error)
	listContexts     func(page iagents.PageParams) ([]iagents.ContextSummary, iagents.PageInfo, error)
	getContext       func(ctxID string) (*iagents.ContextDetail, error)
	deleteContext    func(ctxID string) error
	downloadArtifact func(taskID, artifactID string) (*iagents.ArtifactData, error)
}

// scripted is the package-level hook set shared by every scripted instance (the
// registered provider is fixed per package run, the hooks can be re-pointed).
var scripted scriptedHooks

// setScripted installs the hooks for one test and restores the empty (panic
// tripwire) set on cleanup.
func setScripted(t *testing.T, h scriptedHooks) {
	t.Helper()
	scripted = h
	t.Cleanup(func() { scripted = scriptedHooks{} })
}

// scriptedSpec is the instance template whose capability surface is fixed by
// which hooks are wired: CancelTask is deliberately left UNWIRED so
// task_cancel=false (the cancel gate is exercised via example:echo); everything
// else the command tests drive is wired, and FileInput=true so the --file
// gate/confirm path is reachable. Each wired hook delegates to the per-test hook
// and panics if it was not set.
func scriptedSpec() *iagents.AgentSpec {
	return &iagents.AgentSpec{
		FileInput: true,
		Send: iagents.SendOp{Handler: func(_ context.Context, _ iagents.Runtime, in iagents.SendInput) (*iagents.AgentTask, error) {
			if scripted.send == nil {
				panic("scripted provider: Send hook not set")
			}
			return scripted.send(in)
		}},
		GetTask: iagents.TaskGetOp{Handler: func(_ context.Context, _ iagents.Runtime, taskID string) (*iagents.AgentTask, error) {
			if scripted.getTask == nil {
				panic("scripted provider: GetTask hook not set")
			}
			return scripted.getTask(taskID)
		}},
		ListTasks: iagents.TaskListOp{Handler: func(_ context.Context, _ iagents.Runtime, contextID string, page iagents.PageParams) ([]iagents.TaskSummary, iagents.PageInfo, error) {
			if scripted.listTasks == nil {
				panic("scripted provider: ListTasks hook not set")
			}
			return scripted.listTasks(contextID, page)
		}},
		ListContexts: iagents.ContextListOp{Handler: func(_ context.Context, _ iagents.Runtime, page iagents.PageParams) ([]iagents.ContextSummary, iagents.PageInfo, error) {
			if scripted.listContexts == nil {
				panic("scripted provider: ListContexts hook not set")
			}
			return scripted.listContexts(page)
		}},
		GetContext: iagents.ContextGetOp{Handler: func(_ context.Context, _ iagents.Runtime, ctxID string) (*iagents.ContextDetail, error) {
			if scripted.getContext == nil {
				panic("scripted provider: GetContext hook not set")
			}
			return scripted.getContext(ctxID)
		}},
		DeleteContext: iagents.ContextDeleteOp{Handler: func(_ context.Context, _ iagents.Runtime, ctxID string) error {
			if scripted.deleteContext == nil {
				panic("scripted provider: DeleteContext hook not set")
			}
			return scripted.deleteContext(ctxID)
		}},
		DownloadArtifact: iagents.ArtifactDownloadOp{Handler: func(_ context.Context, _ iagents.Runtime, taskID, artifactID string) (*iagents.ArtifactData, error) {
			if scripted.downloadArtifact == nil {
				panic("scripted provider: DownloadArtifact hook not set")
			}
			return scripted.downloadArtifact(taskID, artifactID)
		}},
	}
}

// fakescopedAllScopes is the full RequiredScopes set of the fakescoped test
// provider, sorted — the all-or-nothing preflight requires every one for any
// real API verb.
var fakescopedAllScopes = []string{
	"fakescoped:agent_artifact:read",
	"fakescoped:agent_attachment:write",
	"fakescoped:agent_chat:read",
	"fakescoped:agent_chat:write",
}

// fakeflowAgentIDSource is the AgentIDSource text of the fakeflow provider —
// the non-enumerable `agents list <scheme>` error surfaces it as the hint.
const fakeflowAgentIDSource = "在 fakeflow 测试控制台获取 agent_id（形如 agt_xxx）"

// registerScripted registers the two scripted schemes exactly once (Register
// panics on duplicates). Both are instance-type (agent_id is arbitrary), and not
// enumerable (no ListAgents hook). They leak into the package-level registry for
// the rest of this package run — so no test may assert an exact provider set.
//
//   - fakeflow: no RequiredScopes (preflight always passes) — the workhorse.
//   - fakescoped: a 4-scope RequiredScopes set, for the scope-preflight tests.
var registerScriptedOnce sync.Once

func registerScripted() {
	registerScriptedOnce.Do(func() {
		iagents.Register(iagents.Provider{
			Scheme:        "fakeflow",
			Label:         "test fake (scripted flow)",
			AgentIDSource: fakeflowAgentIDSource,
			Identities:    []iagents.IdentitySpec{{Type: iagents.IdentityUser}, {Type: iagents.IdentityBot}},
			Instance:      scriptedSpec(),
		})
		iagents.Register(iagents.Provider{
			Scheme:         "fakescoped",
			Label:          "test fake (scoped)",
			AgentIDSource:  "test only",
			RequiredScopes: fakescopedAllScopes,
			Identities:     []iagents.IdentitySpec{{Type: iagents.IdentityUser}, {Type: iagents.IdentityBot}},
			Instance:       scriptedSpec(),
		})
	})
}

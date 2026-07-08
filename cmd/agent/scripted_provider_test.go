// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"sync"
	"testing"

	iagent "github.com/larksuite/cli/internal/agent"
)

// scriptedHooks scripts a fake provider's behavior per test. Each hook maps to
// one Provider method; an unset hook that gets called panics — a tripwire
// against a test reaching an unexpected provider path. This replaces the old
// pattern of driving the (removed) real-OAPI adapter through httpmock stubs:
// the command-layer contracts under test (envelope shape, watch exit codes,
// meta.next, pretty rendering, error propagation) are provider-neutral.
type scriptedHooks struct {
	send             func(in iagent.SendInput) (*iagent.AgentTask, error)
	getTask          func(taskID string) (*iagent.AgentTask, error)
	listTasks        func(contextID string) ([]iagent.TaskSummary, error)
	cancelTask       func(taskID string) error
	listContexts     func() ([]iagent.ContextSummary, error)
	getContext       func(ctxID string) (*iagent.ContextDetail, error)
	deleteContext    func(ctxID string) error
	downloadArtifact func(taskID, artifactID string) (*iagent.ArtifactData, error)
}

// scripted is the package-level hook set shared by every scriptedProvider
// instance (the registry factory cannot be re-pointed per test, the hooks can).
var scripted scriptedHooks

// setScripted installs the hooks for one test and restores the empty (panic
// tripwire) set on cleanup.
func setScripted(t *testing.T, h scriptedHooks) {
	t.Helper()
	scripted = h
	t.Cleanup(func() { scripted = scriptedHooks{} })
}

// scriptedProvider delegates every Provider method to the scripted hooks. Its
// Card is synthesized statically (offline, zero-Deps safe) with a fixed honest
// capability matrix: task_cancel=false so the command-layer cancel gate is
// exercisable, everything else true except input_required.
type scriptedProvider struct{ scheme string }

func (p *scriptedProvider) Card(ctx context.Context) (*iagent.AgentCard, error) {
	card := iagent.NewCard(p.scheme, "agt_x")
	card.Capabilities = iagent.Capabilities{
		TaskGet:          true,
		TaskList:         true,
		TaskCancel:       false,
		InputRequired:    false,
		FileInput:        true,
		ArtifactDownload: true,
		MultiTurn:        true,
	}
	return card, nil
}

func (p *scriptedProvider) Send(ctx context.Context, in iagent.SendInput) (*iagent.AgentTask, error) {
	if scripted.send == nil {
		panic("scripted provider: Send hook not set")
	}
	return scripted.send(in)
}

func (p *scriptedProvider) GetTask(ctx context.Context, taskID string) (*iagent.AgentTask, error) {
	if scripted.getTask == nil {
		panic("scripted provider: GetTask hook not set")
	}
	return scripted.getTask(taskID)
}

func (p *scriptedProvider) ListTasks(ctx context.Context, contextID string) ([]iagent.TaskSummary, error) {
	if scripted.listTasks == nil {
		panic("scripted provider: ListTasks hook not set")
	}
	return scripted.listTasks(contextID)
}

func (p *scriptedProvider) CancelTask(ctx context.Context, taskID string) error {
	if scripted.cancelTask == nil {
		panic("scripted provider: CancelTask hook not set")
	}
	return scripted.cancelTask(taskID)
}

func (p *scriptedProvider) ListContexts(ctx context.Context) ([]iagent.ContextSummary, error) {
	if scripted.listContexts == nil {
		panic("scripted provider: ListContexts hook not set")
	}
	return scripted.listContexts()
}

func (p *scriptedProvider) GetContext(ctx context.Context, ctxID string) (*iagent.ContextDetail, error) {
	if scripted.getContext == nil {
		panic("scripted provider: GetContext hook not set")
	}
	return scripted.getContext(ctxID)
}

func (p *scriptedProvider) DeleteContext(ctx context.Context, ctxID string) error {
	if scripted.deleteContext == nil {
		panic("scripted provider: DeleteContext hook not set")
	}
	return scripted.deleteContext(ctxID)
}

func (p *scriptedProvider) DownloadArtifact(ctx context.Context, taskID, artifactID string) (*iagent.ArtifactData, error) {
	if scripted.downloadArtifact == nil {
		panic("scripted provider: DownloadArtifact hook not set")
	}
	return scripted.downloadArtifact(taskID, artifactID)
}

// fakescopedAllScopes is the full RequiredScopes set of the fakescoped test
// provider, sorted — the all-or-nothing preflight requires every one of these
// for any real API verb.
var fakescopedAllScopes = []string{
	"fakescoped:agent_artifact:read",
	"fakescoped:agent_attachment:write",
	"fakescoped:agent_chat:read",
	"fakescoped:agent_chat:write",
}

// fakeflowAgentIDSource is the AgentIDSource text of the fakeflow provider —
// the non-Discoverer `agent list <scheme>` error surfaces it as the hint.
const fakeflowAgentIDSource = "在 fakeflow 测试控制台获取 agent_id（形如 agt_xxx）"

// registerScripted registers the two scripted schemes exactly once (Register
// panics on duplicates). Like the other fakes they leak into the package-level
// registry for the remaining tests of this package run — so no test in this
// package may assert an exact provider set or provider count.
//
//   - fakeflow: instance kind, no RequiredScopes (preflight always passes) —
//     the workhorse for send/task/context command-layer tests.
//   - fakescoped: same behavior but declares a 4-scope RequiredScopes set, for
//     the scope-preflight framework tests.
var registerScriptedOnce sync.Once

func registerScripted() {
	registerScriptedOnce.Do(func() {
		iagent.Register("fakeflow", iagent.ProviderInfo{
			Factory: func(deps iagent.Deps, agentID string) (iagent.Provider, error) {
				return &scriptedProvider{scheme: "fakeflow"}, nil
			},
			Label:          "test fake (scripted flow)",
			AgentRefFormat: "fakeflow:<agent_id>",
			AgentIDSource:  fakeflowAgentIDSource,
			Kind:           iagent.KindInstance,
			Identities:     []iagent.IdentitySpec{{Type: iagent.IdentityUser}, {Type: iagent.IdentityBot}},
		})
		iagent.Register("fakescoped", iagent.ProviderInfo{
			Factory: func(deps iagent.Deps, agentID string) (iagent.Provider, error) {
				return &scriptedProvider{scheme: "fakescoped"}, nil
			},
			Label:          "test fake (scoped)",
			AgentRefFormat: "fakescoped:<agent_id>",
			AgentIDSource:  "test only",
			Kind:           iagent.KindInstance,
			RequiredScopes: fakescopedAllScopes,
			Identities:     []iagent.IdentitySpec{{Type: iagent.IdentityUser}, {Type: iagent.IdentityBot}},
		})
	})
}

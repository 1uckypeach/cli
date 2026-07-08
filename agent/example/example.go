// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package example is the in-repo agent provider onboarding template and offline
// demo backend: a hypothetical example business domain whose data / calls are
// entirely in-memory mocks, with zero network. It has three roles:
//
//  1. A copy-start point for new integrators — copy the whole package and rename
//     it; every key decision point carries a teaching comment from the
//     "integrator's perspective" (how to fill registration fields, why typed
//     errors are required, how to make capability-matrix trade-offs);
//  2. The command tree's offline demo backend — the full agent
//     list/card/send/task/context chain runs for real without any platform
//     configuration;
//  3. A stable mock scheme for cmd-layer tests.
//
// Minimal checklist for onboarding a new provider (each item is demonstrated in
// this package):
//   - register metadata via agent.Register in init() (see the per-field comments below);
//   - implement the 9 methods of agent.Provider (internal/agent/provider.go);
//   - a catalog type (KindCatalog) must implement agent.Discoverer (asserted at registration time);
//   - add a blank import in cmd/agent/agent.go to trigger init registration;
//   - run agenttest.RunConformance in tests to lock down implicit contracts.
package example

import (
	"context"
	"fmt"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/agent"
)

// scheme is this provider's ref prefix (example:<agent_id>). It is globally
// unique; duplicate registration panics during init (aligned with the
// sql.Register convention, fail-fast to expose onboarding errors).
const scheme = "example"

// catalog is the full agent set known at registration time. The catalog
// boilerplate (enumeration / per-agent rich Card / typed error for unknown ids)
// is entirely handled by the framework's StaticCatalog; the integrator only
// declares the data.
//
// Teaching focus — the capability matrix must be "declared honestly":
//   - The two agents' matrices differ on purpose: echo is the minimal set (no
//     artifact, no file input, not cancelable), reporter has everything on. The
//     AI decides its next command from the card, so whatever is declared must be
//     delivered.
//   - The asymmetry is directional: declaring something you "can't do" as true
//     leads the AI into a dead end (e.g. declaring input_required true while the
//     task never stops in that state, so the AI waits forever for a state that
//     never appears); conversely, declaring an "unused" capability as true when
//     the state never appears (like reporter's input_required) merely means the
//     AI never uses it and does not go wrong. Only the former warrants being
//     conservatively false.
//   - task_cancel being true for one and false for the other is not arbitrary:
//     echo=false demonstrates the command layer's card gating
//     (cmd/agent/task.go intercepts and reports unsupported_capability before
//     sending any request), while reporter=true demonstrates actually reaching
//     the provider's CancelTask.
var catalog = agent.NewStaticCatalog(scheme, []agent.CatalogEntry{
	{
		ID:          "echo",
		Name:        "复读机",
		Description: "把你发的话原样复读一遍（同一会话续发时带轮次，证明上下文记忆）。最小能力集示范。",
		Capabilities: agent.Capabilities{
			TaskGet:  true,
			TaskList: true,
			// echo is not cancelable: once the command layer reads false it
			// rejects task cancel outright, so the provider is never even called
			// (gate-before-network).
			TaskCancel: false,
			// The mock task completes instantly and never stops in
			// input_required; echo honestly declares false per the minimal set.
			InputRequired: false,
			// echo takes no files and produces no artifact — Send returns
			// agent.ErrUnsupported for --file, consistent with the false here
			// (declaration and behavior single-sourced).
			FileInput:        false,
			ArtifactDownload: false,
			MultiTurn:        true,
		},
	},
	{
		ID:          "reporter",
		Name:        "报表生成器",
		Description: "对任意请求产出一份内联 CSV 报表 artifact，示范 artifact 下载与任务取消链路。",
		Capabilities: agent.Capabilities{
			TaskGet:          true,
			TaskList:         true,
			TaskCancel:       true,
			InputRequired:    true,
			FileInput:        true,
			ArtifactDownload: true,
			MultiTurn:        true,
		},
	},
})

func init() {
	// Registration contract (internal/agent/registry.go): everything except
	// RequiredScopes is required; missing / invalid values panic. At
	// registration time it also constructs a Provider once via a zero-value Deps
	// probe — so Factory must accept zero-value Deps and have no side effects
	// during construction (no network, no disk).
	agent.Register(scheme, agent.ProviderInfo{
		Factory: newExampleProvider,
		// Label: the user-facing provider name (the LABEL column in agent list).
		Label: "Example 演示 agent（内存 mock，零网络）",
		// AgentRefFormat: the written format of agent_ref, must start with "<scheme>:" (validated at registration).
		AgentRefFormat: "example:<agent_id>",
		// AgentIDSource: tells the user / AI where to get the agent_id — key
		// information for AI-guided onboarding, referenced by the unknown-id hint
		// and the not-discoverable list hint.
		AgentIDSource: "运行 lark-cli agent list example 查看内置演示 agent 及其 agent_ref（无需任何平台配置）",
		// Kind: catalog type. Registration asserts the instance implements
		// Discoverer, so `agent list example` can enumerate (an instance-type
		// provider that does not support enumeration returns unsupported_capability).
		Kind: agent.KindCatalog,
		// RequiredScopes: the full set of scopes this provider's real API calls
		// need. example has zero network and calls no OAPI, so it is empty —
		// scope preflight (cmd/agent/preflight.go) always passes for the empty
		// set. A real provider must list every scope used by any verb (preflight
		// is all-or-nothing).
		RequiredScopes: nil,
		// Identities: supported calling identities and their preconditions. The
		// mock treats user/bot alike; if a real provider has a precondition for
		// some identity (e.g. a bot needs channel whitelisting), put it in
		// Precondition and the card passes it through to the AI verbatim.
		Identities: []agent.IdentitySpec{
			{Type: agent.IdentityUser},
			{Type: agent.IdentityBot},
		},
	})
}

// exampleProvider addresses one agent in the catalog. agentID may be empty — the
// Discoverer path (agent list example) constructs an instance without an id only
// to enumerate the catalog.
type exampleProvider struct {
	deps    agent.Deps
	agentID string
}

// newExampleProvider is the registered Factory. Teaching point: it does pure
// assignment only.
//   - It does not validate agentID: an unknown id is rejected by catalog.Lookup
//     inside "the verbs that actually use the id" (the empty-id enumeration
//     instance must construct successfully);
//   - It does not touch deps: the mock has no use for Client/As; a real provider
//     uses deps.Client to send requests and deps.As to pick the identity in its
//     methods, but construction must still have no side effects (the zero-value
//     Deps probe contract).
func newExampleProvider(deps agent.Deps, agentID string) (agent.Provider, error) {
	return &exampleProvider{deps: deps, agentID: agentID}, nil
}

// Card returns the agent's capability card. Synthesis is fully offline: NewCard
// fills in registration-time fields (Provider/Label/Identity/AgentIDSource/empty
// Parameters), and the catalog entry adds Name/Description/Capabilities. An
// unknown id returns a typed validation error from StaticCatalog.Lookup
// (invalid_argument/exit 2, hint pointing to agent list example) — the
// integrator need not build its own unknown-agent error.
func (p *exampleProvider) Card(ctx context.Context) (*agent.AgentCard, error) {
	return catalog.Card(p.agentID)
}

// ListAgents enumerates the catalog (Discoverer): `agent list example` goes here.
func (p *exampleProvider) ListAgents(ctx context.Context) ([]agent.AgentSummary, error) {
	return catalog.ListAgents(ctx)
}

// Send sends one message: the first turn generates a context_id to start a new
// conversation, and --context-id continues within the same conversation. The
// mock task has no async execution body, so Send immediately returns in the
// completed terminal state — the command layer's meta.next therefore directly
// gives the terminal-state suggestion "view task detail and artifacts" rather
// than a polling command.
//
// Teaching point (IsTerminal): IsTerminal is filled in here for convenience, but
// leaving it out would be fine — the command layer's normalizeTask always
// re-derives this field from State (single source), so a provider filling it in
// wrong does not affect the watch exit code. The integrator need not worry about it.
func (p *exampleProvider) Send(ctx context.Context, in agent.SendInput) (*agent.AgentTask, error) {
	entry, err := catalog.Lookup(p.agentID)
	if err != nil {
		return nil, err
	}
	// Capability declaration and behavior are single-sourced: if the card says it
	// takes no files, the behavior must reject them. The command layer does not
	// card-gate --file (only cancel is gated), so the provider returns the
	// agent.ErrUnsupported sentinel, and the command layer's convertUnsupported
	// turns it into a typed unsupported_capability (exit 2) — the full
	// sentinel→typed chain.
	if len(in.Files) > 0 && !entry.Capabilities.FileInput {
		return nil, fmt.Errorf("example:%s 不接收文件输入: %w", p.agentID, agent.ErrUnsupported)
	}
	// The mock task is instantly terminal, so there is no "feed input to a running
	// task" scenario. Continuing via --task-id returns failed_precondition: the
	// request itself is valid but the target resource's state does not satisfy it
	// — reading this subtype, the AI knows to "try a different way" (start a new
	// task) rather than retry as-is.
	if in.TaskID != "" {
		return nil, errs.NewValidationError(errs.SubtypeFailedPrecondition,
			"example 的任务发出即完成（终态），无法向已有任务续发").
			WithParam("--task-id").
			WithHint("去掉 --task-id，用 --context-id 在同一会话起新一轮任务")
	}

	ctxID := in.ContextID
	if ctxID == "" {
		// First turn: generate a context_id (the anchor for the multi-turn
		// context; later sends use it to continue the conversation).
		ctxID, err = store.createContext(p.agentID, truncateTitle(in.Text))
		if err != nil {
			return nil, err
		}
	}

	// createTask validates context ownership while holding the lock (an unknown /
	// cross-agent context id is rejected inside with a typed validation error),
	// computes the round, and inserts atomically; the build callback only
	// assembles the task body according to the round.
	task, err := store.createTask(p.agentID, ctxID, func(round int) agent.AgentTask {
		var reply string
		switch entry.ID {
		case "echo":
			// Echo the input; from round 2 on, add a round marker to prove
			// across commands that context memory really works.
			reply = in.Text
			if round > 1 {
				reply = fmt.Sprintf("%s（第 %d 轮）", in.Text, round)
			}
		default: // reporter
			reply = "报表已生成：quarterly_report.csv（见 artifacts，用 task get --artifact <id> -o <path> 下载）"
			if n := len(in.Files); n > 0 {
				reply = fmt.Sprintf("已收到 %d 个附件；%s", n, reply)
			}
		}
		t := agent.AgentTask{
			TaskID:     newID("task"),
			ContextID:  ctxID,
			State:      agent.StateCompleted,
			IsTerminal: true,
			Messages: []agent.Message{
				{Role: "user", Parts: []agent.Part{{Type: "text", Text: in.Text}}},
				{Role: "agent", Parts: []agent.Part{{Type: "text", Text: reply}}},
			},
		}
		if entry.ID == "reporter" {
			// The artifact exposes only fields the provider can truly deliver
			// (the contract.go rule: do not create empty shell fields that cannot
			// be filled): the GetTask stage gives ID + Kind (a coarse-grained type
			// hint), while the file name / mime are exposed at the
			// DownloadArtifact stage as suggested_name.
			t.Artifacts = []agent.Artifact{{ID: newID("art"), Kind: "text"}}
		}
		return t
	})
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetTask queries a single task's state and artifacts (reads the in-memory state machine).
func (p *exampleProvider) GetTask(ctx context.Context, taskID string) (*agent.AgentTask, error) {
	if _, err := catalog.Lookup(p.agentID); err != nil {
		return nil, err
	}
	task, err := store.getTask(p.agentID, taskID)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// ListTasks lists tasks, optionally filtered by contextID (empty string means no filter).
func (p *exampleProvider) ListTasks(ctx context.Context, contextID string) ([]agent.TaskSummary, error) {
	if _, err := catalog.Lookup(p.agentID); err != nil {
		return nil, err
	}
	return store.listTasks(p.agentID, contextID), nil
}

// CancelTask cancels a task. Both paths are demonstrated here:
//   - echo (task_cancel=false): returns the agent.ErrUnsupported sentinel. Via
//     the CLI this is unreachable (cmd/agent/task.go's card gating comes first),
//     but tests calling the SPI directly / future callers that bypass the gate
//     still get the correct semantics — belt-and-braces.
//   - reporter (task_cancel=true): actually dispatched. But the mock task is
//     completed the moment it is sent, so canceling a terminal task returns a
//     failed_precondition typed error (state not satisfied, exit 2) rather than
//     pretending success — honest error semantics matter as much as honest
//     capability declaration.
func (p *exampleProvider) CancelTask(ctx context.Context, taskID string) error {
	entry, err := catalog.Lookup(p.agentID)
	if err != nil {
		return err
	}
	if !entry.Capabilities.TaskCancel {
		return agent.ErrUnsupported
	}
	task, err := store.getTask(p.agentID, taskID)
	if err != nil {
		return err
	}
	if task.State.IsTerminal() {
		return errs.NewValidationError(errs.SubtypeFailedPrecondition,
			"任务 '%s' 已处于终态 %s，无法取消", taskID, task.State).
			WithHint("终态任务不可取消；用 lark-cli agent task get example:%s %s 查看结果", p.agentID, taskID)
	}
	return store.setTaskState(taskID, agent.StateCanceled)
}

// ListContexts lists multi-turn contexts.
func (p *exampleProvider) ListContexts(ctx context.Context) ([]agent.ContextSummary, error) {
	if _, err := catalog.Lookup(p.agentID); err != nil {
		return nil, err
	}
	return store.listContexts(p.agentID), nil
}

// GetContext returns a single context's detail (including its task list).
func (p *exampleProvider) GetContext(ctx context.Context, ctxID string) (*agent.ContextDetail, error) {
	if _, err := catalog.Lookup(p.agentID); err != nil {
		return nil, err
	}
	return store.getContext(p.agentID, ctxID)
}

// DeleteContext deletes a context (a destructive operation; the --yes gate is in the command layer).
func (p *exampleProvider) DeleteContext(ctx context.Context, ctxID string) error {
	if _, err := catalog.Lookup(p.agentID); err != nil {
		return err
	}
	return store.deleteContext(p.agentID, ctxID)
}

// reportCSV is the fixed content of the reporter artifact (inline text, demonstrating a Bytes-type artifact).
const reportCSV = "quarter,revenue,cost,margin\n" +
	"2026Q1,1250,830,0.336\n" +
	"2026Q2,1410,905,0.358\n"

// DownloadArtifact fetches artifact data. example uses the inline Bytes type
// (the command layer writes it to disk directly); the URL type (a real
// provider's signed URL) fills the URL field, and SSRF validation plus the
// download are handled uniformly by the command layer.
//
// Teaching point (suggested_name): ArtifactData.Name is the "server-suggested
// file name", echoed back only as a suggested_name for the caller to reference
// when choosing -o — it is untrusted input and must never participate in
// constructing the local save path (the contract.go rule; the save path is
// always determined by -o/SafeOutputPath).
func (p *exampleProvider) DownloadArtifact(ctx context.Context, taskID, artifactID string) (*agent.ArtifactData, error) {
	entry, err := catalog.Lookup(p.agentID)
	if err != nil {
		return nil, err
	}
	if !entry.Capabilities.ArtifactDownload {
		// Keep behavior consistent with the card's artifact_download=false (the sentinel→typed chain).
		return nil, fmt.Errorf("example:%s 不产出可下载产物: %w", p.agentID, agent.ErrUnsupported)
	}
	task, err := store.getTask(p.agentID, taskID)
	if err != nil {
		return nil, err
	}
	for _, a := range task.Artifacts {
		if a.ID == artifactID {
			return &agent.ArtifactData{
				Name:  "quarterly_report.csv",
				Mime:  "text/csv",
				Bytes: []byte(reportCSV),
			}, nil
		}
	}
	return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
		"任务 '%s' 名下没有产物 '%s'", taskID, artifactID).
		WithHint("运行 lark-cli agent task get example:%s %s 查看该任务的 artifacts", p.agentID, taskID)
}

// truncateTitle takes the first few characters of the message as the
// conversation title (truncated by rune to avoid cutting a character in half).
func truncateTitle(s string) string {
	const max = 20
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

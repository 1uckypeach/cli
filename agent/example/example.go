// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package example is the in-repo agent provider onboarding template and offline
// demo backend: a hypothetical example business domain whose data / calls are
// entirely in-memory mocks, with zero network. It has three roles:
//
//  1. A copy-start point for new integrators — copy the whole package and rename
//     it; every key decision point carries a teaching comment from the
//     "integrator's perspective" (how to fill registration fields, which
//     capabilities to wire, how to make capability trade-offs);
//  2. The command tree's offline demo backend — the full agent
//     list/card/send/task/context chain runs for real without any platform
//     configuration;
//  3. A stable mock scheme for cmd-layer tests.
//
// Minimal checklist for onboarding a new provider (each item is demonstrated in
// this package):
//   - register metadata via agent.Register in init() (see the per-field comments below);
//   - construct a *agent.Provider in the Factory, wiring one func field per
//     capability you support — the core Send/GetTask are mandatory, every other
//     field is optional and "not wired = not supported" (the framework returns a
//     unified unsupported_capability error and derives the card matrix from what
//     is wired, so there is no bool matrix to keep in sync and no capability-
//     refusal code to write);
//   - a catalog type (KindCatalog) must wire ListAgents (asserted at registration);
//   - add a blank import under agent/register.go to trigger init registration;
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
// boilerplate (enumeration / per-agent Card metadata / typed error for unknown
// ids) is handled by the framework's StaticCatalog; the integrator only declares
// the descriptive data. Capabilities are NOT declared here — see newProvider,
// where each agent's supported capabilities are expressed by which Provider func
// fields the Factory wires.
var catalog = agent.NewStaticCatalog(scheme, []agent.CatalogEntry{
	{
		ID:          "echo",
		Name:        "复读机",
		Description: "把你发的话原样复读一遍（同一会话续发时带轮次，证明上下文记忆）。最小能力集示范。",
	},
	{
		ID:          "reporter",
		Name:        "报表生成器",
		Description: "对任意请求产出一份内联 CSV 报表 artifact，示范 artifact 下载与任务取消链路。",
	},
})

func init() {
	// Registration contract (internal/agent/registry.go): everything except
	// RequiredScopes is required; missing / invalid values panic. At registration
	// time it also constructs a Provider once via a zero-value Deps probe — so the
	// Factory must accept zero-value Deps and an empty agentID, have no side
	// effects during construction (no network, no disk), and wire the mandatory
	// core fields (Send/GetTask) plus, for a catalog type, ListAgents.
	agent.Register(scheme, agent.ProviderInfo{
		Factory: newProvider,
		// Label: the user-facing provider name (the LABEL column in agent list).
		Label: "Example 演示 agent（内存 mock，零网络）",
		// AgentRefFormat: the written format of agent_ref, must start with "<scheme>:" (validated at registration).
		AgentRefFormat: "example:<agent_id>",
		// AgentIDSource: tells the user / AI where to get the agent_id — key
		// information for AI-guided onboarding, referenced by the unknown-id hint
		// and the not-discoverable list hint.
		AgentIDSource: "运行 lark-cli agent list example 查看内置演示 agent 及其 agent_ref（无需任何平台配置）",
		// Kind: catalog type. Registration asserts the provider wires ListAgents,
		// so `agent list example` can enumerate.
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

// state addresses one agent in the catalog. agentID may be empty — the
// enumeration path (agent list example) and the registration probe construct a
// state without an id.
type state struct {
	deps    agent.Deps
	agentID string
}

// newProvider is the registered Factory. It assembles a *agent.Provider by
// wiring the func fields for the capabilities this agent supports.
//
// Teaching focus — capability is expressed as wiring, per agent:
//   - Core Send/GetTask are wired unconditionally (mandatory).
//   - The always-on optionals (ListTasks, the context trio, ListAgents, Describe)
//     are wired for every agent.
//   - reporter additionally wires CancelTask + DownloadArtifact and sets
//     FileInput — echo does not, so echo's card honestly shows task_cancel /
//     artifact_download / file_input = false. There is no bool matrix: the card
//     is derived from exactly these fields (internal/agent/card.go DeriveCapabilities).
//   - A capability you do not wire needs zero refusal code: the command layer
//     gates on the nil field and returns unified unsupported_capability before
//     any provider method runs.
//
// Teaching point — the Factory does pure assignment only: it does not validate
// agentID (an unknown id is rejected by catalog.Lookup inside the verbs that use
// it, and by Describe on the card path; the empty-id probe/enumeration instance
// must construct successfully) and does not touch deps (the mock has no use for
// Client/As, but construction must have no side effects either way — the
// zero-value Deps probe contract).
func newProvider(deps agent.Deps, agentID string) (*agent.Provider, error) {
	s := &state{deps: deps, agentID: agentID}
	p := &agent.Provider{
		Send:          s.send,
		GetTask:       s.getTask,
		ListTasks:     s.listTasks,
		ListContexts:  s.listContexts,
		GetContext:    s.getContext,
		DeleteContext: s.deleteContext,
		ListAgents:    s.listAgents,
		Describe:      s.describe,
	}
	// Per-agent capability: reporter can be canceled and produces a downloadable
	// artifact, accepts file input, and may pause a task in input_required; echo
	// (minimal set) does none of these, so those fields stay nil/false and the
	// framework reports them unsupported.
	if agentID == "reporter" {
		p.CancelTask = s.cancelTask
		p.DownloadArtifact = s.downloadArtifact
		p.FileInput = true
		p.InputRequired = true
	}
	return p, nil
}

// describe supplies the per-agent Card metadata and validates the agent_id
// (StaticCatalog.Describe returns a typed unknown-id error). Capabilities are
// derived by the framework from the wired fields, so Describe never touches them.
func (s *state) describe(ctx context.Context) (*agent.CardInfo, error) {
	return catalog.Describe(s.agentID)
}

// listAgents enumerates the catalog: `agent list example` goes here.
func (s *state) listAgents(ctx context.Context) ([]agent.AgentSummary, error) {
	return catalog.ListAgents(ctx)
}

// send sends one message: the first turn generates a context_id to start a new
// conversation, and --context-id continues within the same conversation. The
// mock task has no async execution body, so send immediately returns in the
// completed terminal state — the command layer's meta.next therefore directly
// gives the terminal-state suggestion "view task detail and artifacts" rather
// than a polling command.
//
// Teaching point (IsTerminal): IsTerminal is filled in here for convenience, but
// leaving it out would be fine — the command layer's normalizeTask always
// re-derives this field from State (single source), so a provider filling it in
// wrong does not affect the watch exit code.
func (s *state) send(ctx context.Context, in agent.SendInput) (*agent.AgentTask, error) {
	entry, err := catalog.Lookup(s.agentID)
	if err != nil {
		return nil, err
	}
	// The mock task is instantly terminal, so there is no "feed input to a running
	// task" scenario. Continuing via --task-id returns failed_precondition: the
	// request itself is valid but the target resource's state does not satisfy it
	// — reading this subtype, the AI knows to "try a different way" (start a new
	// task) rather than retry as-is. (This is a genuine runtime precondition, not
	// a capability gate — hence a typed error here, not an unwired field.)
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
		ctxID, err = store.createContext(s.agentID, truncateTitle(in.Text))
		if err != nil {
			return nil, err
		}
	}

	// createTask validates context ownership while holding the lock (an unknown /
	// cross-agent context id is rejected inside with a typed validation error),
	// computes the round, and inserts atomically; the build callback only
	// assembles the task body according to the round.
	task, err := store.createTask(s.agentID, ctxID, func(round int) agent.AgentTask {
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

// getTask queries a single task's state and artifacts (reads the in-memory state machine).
func (s *state) getTask(ctx context.Context, taskID string) (*agent.AgentTask, error) {
	if _, err := catalog.Lookup(s.agentID); err != nil {
		return nil, err
	}
	task, err := store.getTask(s.agentID, taskID)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// listTasks lists tasks, optionally filtered by contextID (empty string means no filter).
func (s *state) listTasks(ctx context.Context, contextID string) ([]agent.TaskSummary, error) {
	if _, err := catalog.Lookup(s.agentID); err != nil {
		return nil, err
	}
	return store.listTasks(s.agentID, contextID), nil
}

// cancelTask cancels a task. It is wired only for reporter (task_cancel=true), so
// echo never reaches it — the command layer gates echo's cancel on the nil field
// and returns unsupported_capability before any provider code runs. The mock
// task is completed the moment it is sent, so canceling a terminal task returns a
// failed_precondition typed error (state not satisfied, exit 2) rather than
// pretending success — honest error semantics matter as much as honest capability
// wiring.
func (s *state) cancelTask(ctx context.Context, taskID string) error {
	if _, err := catalog.Lookup(s.agentID); err != nil {
		return err
	}
	task, err := store.getTask(s.agentID, taskID)
	if err != nil {
		return err
	}
	if task.State.IsTerminal() {
		return errs.NewValidationError(errs.SubtypeFailedPrecondition,
			"任务 '%s' 已处于终态 %s，无法取消", taskID, task.State).
			WithHint("终态任务不可取消；用 lark-cli agent task get example:%s %s 查看结果", s.agentID, taskID)
	}
	return store.setTaskState(taskID, agent.StateCanceled)
}

// listContexts lists multi-turn contexts.
func (s *state) listContexts(ctx context.Context) ([]agent.ContextSummary, error) {
	if _, err := catalog.Lookup(s.agentID); err != nil {
		return nil, err
	}
	return store.listContexts(s.agentID), nil
}

// getContext returns a single context's detail (including its task list).
func (s *state) getContext(ctx context.Context, ctxID string) (*agent.ContextDetail, error) {
	if _, err := catalog.Lookup(s.agentID); err != nil {
		return nil, err
	}
	return store.getContext(s.agentID, ctxID)
}

// deleteContext deletes a context (a destructive operation; the --yes gate is in the command layer).
func (s *state) deleteContext(ctx context.Context, ctxID string) error {
	if _, err := catalog.Lookup(s.agentID); err != nil {
		return err
	}
	return store.deleteContext(s.agentID, ctxID)
}

// reportCSV is the fixed content of the reporter artifact (inline text, demonstrating a Bytes-type artifact).
const reportCSV = "quarter,revenue,cost,margin\n" +
	"2026Q1,1250,830,0.336\n" +
	"2026Q2,1410,905,0.358\n"

// downloadArtifact fetches artifact data. It is wired only for reporter
// (artifact_download=true); echo never reaches it (gated on the nil field).
// example uses the inline Bytes type (the command layer writes it to disk
// directly); the URL type (a real provider's signed URL) fills the URL field, and
// SSRF validation plus the download are handled uniformly by the command layer.
//
// Teaching point (suggested_name): ArtifactData.Name is the "server-suggested
// file name", echoed back only as a suggested_name for the caller to reference
// when choosing -o — it is untrusted input and must never participate in
// constructing the local save path (the contract.go rule; the save path is
// always determined by -o/SafeOutputPath).
func (s *state) downloadArtifact(ctx context.Context, taskID, artifactID string) (*agent.ArtifactData, error) {
	if _, err := catalog.Lookup(s.agentID); err != nil {
		return nil, err
	}
	task, err := store.getTask(s.agentID, taskID)
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
		WithHint("运行 lark-cli agent task get example:%s %s 查看该任务的 artifacts", s.agentID, taskID)
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

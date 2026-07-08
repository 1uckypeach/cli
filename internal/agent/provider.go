// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import "context"

// SendInput is the input to send (Params has already passed Card validation).
type SendInput struct {
	Text      string
	Files     []string
	Params    map[string]string
	ContextID string
	TaskID    string
}

// CardInfo is the per-agent descriptive metadata a provider supplies for its
// Card (everything the framework cannot fill from registration data or derive
// from capabilities): the display Name/Description, declared input Parameters,
// and Skills. It is returned by Provider.Describe.
type CardInfo struct {
	Name        string
	Description string
	Parameters  []CardParam
	Skills      []CardSkill
}

// Provider is a remote agent adapter: it translates the unified commands into a
// specific vendor's OAPI. It is a struct of function fields rather than a fat
// interface, mirroring the events KeyDefinition / shortcuts Shortcut convention:
// a provider fills only the capabilities it supports, and a nil optional field
// means "unsupported" — the command layer gates on it and returns a unified
// unsupported_capability error before any network access, so a provider never
// writes capability-refusal code itself. The Card capability matrix is derived
// by the framework from which fields are non-nil (see BuildCard), so declaration
// and behavior are single-sourced and cannot drift.
//
// Because a Provider is constructed per (deps, agentID) by its Factory, a
// catalog provider whose agents differ in capability wires different fields per
// agentID (see agent/example) — capability is expressed as code, not a
// hand-maintained bool matrix.
type Provider struct {
	// ── Core (Register validates both non-nil for every provider) ──

	// Send sends one message, starting a new task or continuing an existing one.
	Send func(ctx context.Context, in SendInput) (*AgentTask, error)
	// GetTask queries a single task's state and artifacts.
	GetTask func(ctx context.Context, taskID string) (*AgentTask, error)

	// ── Optional capabilities (nil = unsupported; framework gates) ──

	// ListTasks lists tasks, optionally filtered by contextID (empty = no filter).
	// nil ⇒ card task_list=false.
	ListTasks func(ctx context.Context, contextID string) ([]TaskSummary, error)
	// CancelTask cancels (interrupts) a task. nil ⇒ card task_cancel=false.
	CancelTask func(ctx context.Context, taskID string) error
	// ListContexts lists multi-turn contexts. nil ⇒ card multi_turn=false (the
	// multi_turn capability is derived from this, the enumeration entry point).
	ListContexts func(ctx context.Context) ([]ContextSummary, error)
	// GetContext returns a single context's detail. nil ⇒ context get unsupported.
	GetContext func(ctx context.Context, ctxID string) (*ContextDetail, error)
	// DeleteContext deletes a context (destructive). nil ⇒ context delete unsupported.
	DeleteContext func(ctx context.Context, ctxID string) error
	// DownloadArtifact fetches artifact data: the URL type returns URL, the inline
	// type returns Bytes. nil ⇒ card artifact_download=false.
	DownloadArtifact func(ctx context.Context, taskID, artifactID string) (*ArtifactData, error)
	// ListAgents enumerates the provider's own agents (catalog discovery). nil ⇒
	// `agent list <scheme>` reports the provider is not enumerable. A KindCatalog
	// provider must wire it (asserted at Register time).
	ListAgents func(ctx context.Context) ([]AgentSummary, error)

	// ── Optional descriptive metadata ──

	// Describe supplies the per-agent Card metadata (Name/Description/Parameters/
	// Skills) and is the place to validate an unknown agent_id (return a typed
	// error). nil ⇒ the card carries only registration fields + derived
	// capabilities. Called at card-display time (may hit the network for an
	// instance provider that fetches its card remotely).
	Describe func(ctx context.Context) (*CardInfo, error)

	// ── Behavioral flags (not derivable from method presence) ──

	// FileInput reports whether Send accepts SendInput.Files (drives card
	// file_input and the --file off-machine-upload confirmation gate).
	FileInput bool
	// InputRequired reports whether the agent may pause a task in the
	// input_required state awaiting more input (drives card input_required).
	InputRequired bool
}

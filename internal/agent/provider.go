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
// and Skills. It is returned by AgentSpec.Describe.
type CardInfo struct {
	Name        string
	Description string
	Parameters  []CardParam
	Skills      []CardSkill
}

// Provider is one business domain (one scheme): registration metadata plus its
// agent set. It is a declarative value — registered from agent/register.go, not
// constructed via a factory. Exactly one of Catalog / Instance is set (Register
// enforces), which encodes the kind, so there is no separate Kind field to keep
// in sync.
type Provider struct {
	Scheme         string         // ref prefix, e.g. "example"
	Label          string         // `agent list` LABEL column
	AgentIDSource  string         // where to get an agent_id (AI onboarding cue)
	RequiredScopes []string       // flat set; preflight is all-or-nothing
	Identities     []IdentitySpec // non-empty; Type ∈ {user,bot}

	// Exactly one of these is set:
	Catalog  []AgentSpec // finite, offline-enumerable set (kind = catalog)
	Instance *AgentSpec  // single template for any runtime agent_id (kind = instance)

	// ListAgents is the optional ONLINE enumeration hook — only meaningful for an
	// instance provider whose platform has a "list my agents" endpoint. Wired ⇒
	// `agent list <scheme>` enumerates via this call. A catalog provider leaves it
	// nil (enumeration is derived offline from Catalog); an instance platform with
	// only get-by-id and no list endpoint also leaves it nil (not enumerable).
	// This is independent of AgentSpec.Describe: ListAgents = "which agents exist"
	// (a list endpoint), Describe = "what one agent looks like" (get-by-id).
	ListAgents func(ctx context.Context, rt Runtime) ([]AgentSummary, error)
}

// AgentSpec is the declarative unit for one agent: card metadata plus the verb
// hooks it implements. Capability is derived from which hooks are non-nil
// ("implement it = support it", see DeriveCapabilities), so the card and the
// behavior are single-sourced and cannot drift.
//
//   - Catalog: each predefined agent is its own AgentSpec with its own wired
//     hooks — two agents honestly differ in capability with zero bool matrix and
//     zero per-id branching.
//   - Instance: ONE template applied to every runtime agent_id; hooks read
//     rt.AgentID() to know which agent they serve.
type AgentSpec struct {
	ID string // catalog: required + unique; instance: MUST be empty

	// Per-agent card metadata (static, read offline).
	Name        string
	Description string
	Parameters  []CardParam
	Skills      []CardSkill

	// Behavioral flags with no backing hook (the only capability bits not derived
	// from a hook).
	FileInput     bool
	InputRequired bool

	// Core (Register asserts both non-nil for every spec).
	Send    func(ctx context.Context, rt Runtime, in SendInput) (*AgentTask, error)
	GetTask func(ctx context.Context, rt Runtime, taskID string) (*AgentTask, error)

	// Optional capability hooks (nil = unsupported; the framework gates on the nil
	// field and returns a unified unsupported_capability before any network call,
	// and derives the card matrix from which of these are wired).
	ListTasks        func(ctx context.Context, rt Runtime, contextID string) ([]TaskSummary, error)
	CancelTask       func(ctx context.Context, rt Runtime, taskID string) error
	ListContexts     func(ctx context.Context, rt Runtime) ([]ContextSummary, error)
	GetContext       func(ctx context.Context, rt Runtime, ctxID string) (*ContextDetail, error)
	DeleteContext    func(ctx context.Context, rt Runtime, ctxID string) error
	DownloadArtifact func(ctx context.Context, rt Runtime, taskID, artifactID string) (*ArtifactData, error)

	// Describe optionally supplies per-agent Card metadata (Name/Description/
	// Parameters/Skills) and is the place to validate an unknown agent_id (return
	// a typed error). It is invoked ONLY when a runtime is available (configured),
	// so offline the card is always caps + registration metadata + the static
	// fields above. A catalog spec typically leaves it nil and uses the static
	// Name/Description; an instance provider wires it to fetch its card remotely.
	Describe func(ctx context.Context, rt Runtime) (*CardInfo, error)
}

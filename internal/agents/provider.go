// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agents

import "context"

// SendInput is the input to send. Business parameters are NOT here — they ride
// Runtime.Params() like every other operation, so send and the other seven
// verbs share one parameter model.
type SendInput struct {
	Text      string
	Files     []string
	ContextID string
	TaskID    string

	// Structured answer to an input_required decision (see InputRequired). The
	// caller echoes DecisionID and either picks OptionIDs (single/multi_select) or
	// leaves them empty and answers via Text (input_type=text). An empty
	// DecisionID means this send is not answering a decision. A provider
	// serializes these into the reply message's A2A DataPart; the server
	// arbitrates (and MAY reject an already-answered decision as a conflict).
	DecisionID string
	OptionIDs  []string
}

// CardInfo is the per-agent descriptive metadata a provider supplies for its
// Card (the display Name/Description and Skills). It is returned by
// AgentSpec.Describe. It deliberately does NOT carry parameters: offline
// validation only trusts the static per-operation declarations, and dynamic
// per-agent parameter contracts belong to the future overlay phase.
type CardInfo struct {
	Name        string
	Description string
	Skills      []CardSkill
}

// Provider is one business domain (one scheme): registration metadata plus its
// agent set. It is a declarative value — registered from agent/register.go, not
// constructed via a factory. Exactly one of Catalog / Instance is set (Register
// enforces), which encodes the kind, so there is no separate Kind field to keep
// in sync.
type Provider struct {
	Scheme         string         // ref prefix, e.g. "example"
	Label          string         // `agents list` LABEL column
	AgentIDSource  string         // where to get an agent_id (AI onboarding cue)
	RequiredScopes []string       // flat set; preflight is all-or-nothing
	Identities     []IdentitySpec // non-empty; Type ∈ {user,bot}

	// Exactly one of these is set:
	Catalog  []AgentSpec // finite, offline-enumerable set (kind = catalog)
	Instance *AgentSpec  // single template for any runtime agent_id (kind = instance)

	// ListAgents is the optional ONLINE enumeration hook — only meaningful for an
	// instance provider whose platform has a "list my agents" endpoint. Wired ⇒
	// `agents list <scheme>` enumerates via this call. A catalog provider leaves it
	// nil (enumeration is derived offline from Catalog); an instance platform with
	// only get-by-id and no list endpoint also leaves it nil (not enumerable).
	// This is independent of AgentSpec.Describe: ListAgents = "which agents exist"
	// (a list endpoint), Describe = "what one agent looks like" (get-by-id). It is
	// paginated: the framework passes the requested cursor/size as PageParams and
	// surfaces the returned PageInfo as meta.has_more / meta.page_token plus a
	// next-page command.
	ListAgents func(ctx context.Context, rt Runtime, page PageParams) ([]AgentSummary, PageInfo, error)

	// ListParams declares the business parameters of `agents list <scheme>` itself
	// (list is a provider-level discovery operation, so its parameters live here,
	// not on any single agent's spec). Discovered via `agents list` (no scheme)
	// output's providers[].list_parameters. Register panics when ListParams is
	// declared without a ListAgents hook.
	ListParams []CardParam
}

// AgentSpec is the declarative unit for one agent: card metadata plus the
// operations it implements. Each operation is an Op unit binding the business
// parameters it accepts to the handler that serves it — parameters physically
// cannot be declared on an unimplemented operation. Capability is derived from
// which handlers are wired ("implement it = support it", see
// DeriveCapabilities), so the card and the behavior are single-sourced and
// cannot drift.
//
//   - Catalog: each predefined agent is its own AgentSpec with its own wired
//     operations — two agents honestly differ in capability with zero bool
//     matrix and zero per-id branching.
//   - Instance: ONE template applied to every runtime agent_id; handlers read
//     rt.AgentID() to know which agent they serve.
type AgentSpec struct {
	ID string // catalog: required + unique; instance: MUST be empty

	// Per-agent card metadata (static, read offline).
	Name        string
	Description string
	Skills      []CardSkill

	// Behavioral flags with no backing operation (the only capability bits not
	// derived from a handler).
	FileInput     bool
	InputRequired bool

	// Core operations (Register asserts both handlers non-nil for every spec).
	Send    SendOp
	GetTask TaskGetOp

	// Optional operations (zero-value Op = unsupported; the command layer gates
	// on the unwired handler and returns a unified unsupported_capability before
	// any network call, and derives the card matrix from which are wired).
	ListTasks        TaskListOp
	CancelTask       TaskCancelOp
	ListContexts     ContextListOp
	GetContext       ContextGetOp
	DeleteContext    ContextDeleteOp
	DownloadArtifact ArtifactDownloadOp

	// Describe optionally supplies per-agent Card metadata (Name/Description/
	// Skills) and is the place to validate an unknown agent_id (return a typed
	// error). It is invoked ONLY when a runtime is available (configured), so
	// offline the card is always caps + registration metadata + the static
	// fields above. It is card enrichment, not an operation, so it stays a plain
	// func. A catalog spec typically leaves it nil and uses the static
	// Name/Description; an instance provider wires it to fetch its card remotely.
	Describe func(ctx context.Context, rt Runtime) (*CardInfo, error)
}

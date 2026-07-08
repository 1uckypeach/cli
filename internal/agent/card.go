// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

// capability key constants (the JSON key names in capabilities, also the
// capability identifiers used by Supports / capabilityError). Only capabilities
// that "can change the AI's next command line and are currently deliverable" are
// exposed.
const (
	CapTaskGet          = "task_get"
	CapTaskList         = "task_list"
	CapTaskCancel       = "task_cancel"
	CapInputRequired    = "input_required"
	CapFileInput        = "file_input"
	CapArtifactDownload = "artifact_download"
	CapMultiTurn        = "multi_turn"
)

// Capabilities is the closed set of capabilities: making it a struct means an
// omitted field is an explicit false and a typo is a compile error. Fields are
// ordered by json tag alphabetically to keep the key order identical to the old
// map serialization.
type Capabilities struct {
	ArtifactDownload bool `json:"artifact_download"`
	FileInput        bool `json:"file_input"`
	InputRequired    bool `json:"input_required"`
	MultiTurn        bool `json:"multi_turn"`
	TaskCancel       bool `json:"task_cancel"`
	TaskGet          bool `json:"task_get"`
	TaskList         bool `json:"task_list"`
}

// AgentCard is a remote agent's capability card (schema v2): provider metadata,
// the supported capability matrix, identity precondition declarations, and
// parameter / skill declarations (scopes are not in the card; they are internal
// registration data for preflight only).
type AgentCard struct {
	Provider      string         `json:"provider"`
	ProviderLabel string         `json:"provider_label"`
	AgentID       string         `json:"agent_id"`
	Name          string         `json:"name,omitempty"` // dynamic card only
	Description   string         `json:"description,omitempty"`
	Capabilities  Capabilities   `json:"capabilities"`
	Identity      []IdentitySpec `json:"identity"`
	Parameters    []CardParam    `json:"parameters"` // always emitted (empty is [])
	AgentIDSource string         `json:"agent_id_source"`
	Skills        []CardSkill    `json:"skills,omitempty"`
}

// NewCard fills in all fields known at registration time from the registration
// info (Provider/ProviderLabel/Identity/AgentIDSource/empty Parameters); the
// integrator only supplies the per-agent part (Capabilities, plus Name/
// Description for catalog types). An unregistered scheme is a programming error
// (a provider should only pass its own scheme), so it panics fail-fast.
func NewCard(scheme, agentID string) *AgentCard {
	info, ok := Info(scheme)
	if !ok {
		panic("agent: NewCard for unregistered scheme: " + scheme)
	}
	return &AgentCard{
		Provider:      scheme,
		ProviderLabel: info.Label,
		AgentID:       agentID,
		Identity:      info.Identities,
		Parameters:    []CardParam{},
		AgentIDSource: info.AgentIDSource,
	}
}

// CardParam is one input parameter declared by a Card (used for --param validation).
type CardParam struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Desc     string `json:"desc,omitempty"`
}

// CardSkill is one skill / scenario declared by a Card (with example usages).
type CardSkill struct {
	ID       string   `json:"id"`
	Name     string   `json:"name,omitempty"`
	Examples []string `json:"examples,omitempty"`
}

// Supports reports whether a capability is declared as supported (an unknown key
// or a nil card is treated as unsupported).
func (c *AgentCard) Supports(cap string) bool {
	if c == nil {
		return false
	}
	switch cap {
	case CapArtifactDownload:
		return c.Capabilities.ArtifactDownload
	case CapFileInput:
		return c.Capabilities.FileInput
	case CapInputRequired:
		return c.Capabilities.InputRequired
	case CapMultiTurn:
		return c.Capabilities.MultiTurn
	case CapTaskCancel:
		return c.Capabilities.TaskCancel
	case CapTaskGet:
		return c.Capabilities.TaskGet
	case CapTaskList:
		return c.Capabilities.TaskList
	default:
		return false
	}
}

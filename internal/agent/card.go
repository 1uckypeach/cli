// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import "context"

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

// DeriveCapabilities computes the capability matrix from which AgentSpec hooks
// are wired — the single source of truth. The method-backed capabilities are
// derived from the corresponding func field being non-nil (implement it =
// support it); file_input / input_required are behavioral flags with no backing
// hook and are read straight from the spec. Send/GetTask are mandatory (Register
// enforces), so task_get is always true.
func DeriveCapabilities(s *AgentSpec) Capabilities {
	return Capabilities{
		TaskGet:          s.GetTask != nil,
		TaskList:         s.ListTasks != nil,
		TaskCancel:       s.CancelTask != nil,
		ArtifactDownload: s.DownloadArtifact != nil,
		MultiTurn:        s.ListContexts != nil,
		FileInput:        s.FileInput,
		InputRequired:    s.InputRequired,
	}
}

// BuildCard synthesizes an agent's full Card: registration metadata from the
// Provider, the capability matrix from DeriveCapabilities (wired hooks), and the
// static per-agent metadata from the spec. When rt != nil AND the spec wires
// Describe, it best-effort enriches Name/Description/Parameters/Skills from the
// remote — a Describe error is swallowed so the card degrades to the offline
// (caps + static) version rather than hard-failing (the caps matrix is the
// primary value). Pass rt=nil for the guaranteed-offline path (card before
// config init, dry-run). A provider never assembles its own card or declares its
// own capability bools.
func BuildCard(ctx context.Context, p Provider, s *AgentSpec, agentID string, rt Runtime) *AgentCard {
	card := &AgentCard{
		Provider:      p.Scheme,
		ProviderLabel: p.Label,
		AgentID:       agentID,
		Name:          s.Name,
		Description:   s.Description,
		Capabilities:  DeriveCapabilities(s),
		Identity:      p.Identities,
		Parameters:    nonNilParams(s.Parameters),
		AgentIDSource: p.AgentIDSource,
		Skills:        s.Skills,
	}
	if rt != nil && s.Describe != nil {
		if info, err := s.Describe(ctx, rt); err == nil && info != nil {
			if info.Name != "" {
				card.Name = info.Name
			}
			if info.Description != "" {
				card.Description = info.Description
			}
			if info.Parameters != nil {
				card.Parameters = info.Parameters
			}
			if info.Skills != nil {
				card.Skills = info.Skills
			}
		}
	}
	return card
}

// nonNilParams keeps Parameters always emitted (empty is [], never null).
func nonNilParams(p []CardParam) []CardParam {
	if p == nil {
		return []CardParam{}
	}
	return p
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
func (c *AgentCard) Supports(capKey string) bool {
	if c == nil {
		return false
	}
	switch capKey {
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

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

// IdentityType is the closed set of values for IdentitySpec.Type (validated at
// Register time to guard against typos).
type IdentityType string

const (
	IdentityUser IdentityType = "user"
	IdentityBot  IdentityType = "bot"
)

// IdentitySpec declares a supported identity and its precondition, if any.
type IdentitySpec struct {
	Type         IdentityType `json:"type"` // IdentityUser | IdentityBot
	Precondition string       `json:"precondition,omitempty"`
}

// AgentSummary is one discoverable agent in `agent list <scheme>` output.
type AgentSummary struct {
	AgentRef    string `json:"agent_ref"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import "testing"

func TestCardSupports(t *testing.T) {
	c := &AgentCard{Capabilities: Capabilities{TaskCancel: false, MultiTurn: true}}
	if c.Supports(CapTaskCancel) {
		t.Error("task_cancel should not be supported")
	}
	if !c.Supports(CapMultiTurn) {
		t.Error("multi_turn should be supported")
	}
	if c.Supports("nonexistent") {
		t.Error("unknown capability should be treated as unsupported")
	}
	// nil guard branch: a nil receiver is treated as unsupported; a zero-value Capabilities is all false.
	var nilCard *AgentCard
	if nilCard.Supports(CapMultiTurn) {
		t.Error("nil card should be treated as unsupported")
	}
	if (&AgentCard{}).Supports(CapMultiTurn) {
		t.Error("zero-value Capabilities should be treated as unsupported")
	}
	// Each capability constant must map to its own struct field (the switch has no gaps or mismatches).
	all := &AgentCard{Capabilities: Capabilities{
		ArtifactDownload: true, FileInput: true, InputRequired: true,
		MultiTurn: true, TaskCancel: true, TaskGet: true, TaskList: true,
	}}
	for _, k := range []string{
		CapArtifactDownload, CapFileInput, CapInputRequired,
		CapMultiTurn, CapTaskCancel, CapTaskGet, CapTaskList,
	} {
		if !all.Supports(k) {
			t.Errorf("Supports(%q) should be true when all Capabilities are true", k)
		}
	}
}

// TestNewCardFillsRegistrationFields pins that NewCard pre-fills every
// registration-known field and panics on an unregistered scheme.
func TestNewCardFillsRegistrationFields(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	info := testInfo("nc", func(Deps, string) (Provider, error) { return nil, nil })
	info.Identities = []IdentitySpec{{Type: IdentityBot, Precondition: "需要白名单"}}
	Register("nc", info)

	card := NewCard("nc", "agt_1")
	if card.Provider != "nc" || card.AgentID != "agt_1" {
		t.Fatalf("provider/agent_id: %+v", card)
	}
	if card.ProviderLabel != info.Label || card.AgentIDSource != info.AgentIDSource {
		t.Fatalf("registration metadata should be pre-filled: %+v", card)
	}
	if len(card.Identity) != 1 || card.Identity[0].Type != IdentityBot {
		t.Fatalf("identity should come from registration info: %+v", card.Identity)
	}
	if card.Parameters == nil || len(card.Parameters) != 0 {
		t.Fatalf("parameters should be empty but non-nil (always emit []): %#v", card.Parameters)
	}

	mustPanic(t, "unregistered scheme", func() { NewCard("ghost", "agt_1") })
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/larksuite/cli/errs"
)

// testCatalogEntries is declared out of order to verify ListAgents sorting stability.
func testCatalogEntries() []CatalogEntry {
	return []CatalogEntry{
		{ID: "zeta", Name: "Zeta 助手", Description: "z desc",
			Capabilities: Capabilities{TaskGet: true}},
		{ID: "alpha", Name: "Alpha 助手", Description: "a desc",
			Capabilities: Capabilities{TaskGet: true, MultiTurn: true}},
	}
}

// TestStaticCatalogListAgentsSorted asserts the enumeration is sorted by AgentRef
// and that two consecutive results are DeepEqual (stable sort, the same contract
// asserted by agenttest discovery).
func TestStaticCatalogListAgentsSorted(t *testing.T) {
	c := NewStaticCatalog("cattest", testCatalogEntries())
	got, err := c.ListAgents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []AgentSummary{
		{AgentRef: "cattest:alpha", Name: "Alpha 助手", Description: "a desc"},
		{AgentRef: "cattest:zeta", Name: "Zeta 助手", Description: "z desc"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListAgents should be sorted by AgentRef, want %+v got %+v", want, got)
	}
	got2, err := c.ListAgents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, got2) {
		t.Fatalf("two consecutive ListAgents calls should be DeepEqual, got\n%+v\nvs\n%+v", got, got2)
	}
}

// TestStaticCatalogCard asserts Card = NewCard pre-filling registration-time fields
// plus the entry filling in Name/Description/Capabilities.
func TestStaticCatalogCard(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	Register("cattest", testInfo("cattest",
		func(Deps, string) (Provider, error) { return &stubProvider{}, nil }))
	info, _ := Info("cattest")

	c := NewStaticCatalog("cattest", testCatalogEntries())
	card, err := c.Card("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if card.Provider != "cattest" || card.AgentID != "alpha" {
		t.Fatalf("provider/agent_id: %+v", card)
	}
	if card.ProviderLabel != info.Label || card.AgentIDSource != info.AgentIDSource {
		t.Fatalf("registration-time fields should be pre-filled by NewCard: %+v", card)
	}
	if !reflect.DeepEqual(card.Identity, info.Identities) {
		t.Fatalf("Identity should match the registered value: %+v", card.Identity)
	}
	if card.Parameters == nil {
		t.Fatal("Parameters must not be nil (always emitted, empty is [])")
	}
	if card.Name != "Alpha 助手" || card.Description != "a desc" {
		t.Fatalf("entry should fill in Name/Description: %+v", card)
	}
	wantCaps := Capabilities{TaskGet: true, MultiTurn: true}
	if card.Capabilities != wantCaps {
		t.Fatalf("entry should fill in Capabilities, want %+v got %+v", wantCaps, card.Capabilities)
	}
}

// TestStaticCatalogUnknownID asserts Lookup / Card return a typed
// validation/invalid_argument error for an unknown id (exit 2 rather than
// internal/exit 5), with the hint pointing to `agent list <scheme>`.
func TestStaticCatalogUnknownID(t *testing.T) {
	c := NewStaticCatalog("cattest", testCatalogEntries())
	_, err := c.Lookup("nonexistent")
	if err == nil {
		t.Fatal("unknown id should return an error")
	}
	var ve *errs.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("unknown id should be an *errs.ValidationError, got %T: %v", err, err)
	}
	if ve.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("subtype should be invalid_argument, got %q", ve.Subtype)
	}
	if want := "未知的 cattest agent 'nonexistent'"; ve.Message != want {
		t.Fatalf("message should be %q, got %q", want, ve.Message)
	}
	if want := "运行 lark-cli agent list cattest 查看可用 agent"; ve.Hint != want {
		t.Fatalf("hint should be %q, got %q", want, ve.Hint)
	}
	// Card goes through the same Lookup path.
	if _, err := c.Card("nonexistent"); !errors.As(err, &ve) {
		t.Fatalf("Card with an unknown id should return the same typed error, got %v", err)
	}
}

// TestStaticCatalogDuplicateIDPanic asserts a duplicate entry ID triggers a
// fail-fast panic (aligned with the Register convention).
func TestStaticCatalogDuplicateIDPanic(t *testing.T) {
	entries := []CatalogEntry{{ID: "a"}, {ID: "a"}}
	mustPanic(t, "duplicate entry ID", func() { NewStaticCatalog("cattest", entries) })
}

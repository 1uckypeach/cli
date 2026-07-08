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
		{ID: "zeta", Name: "Zeta 助手", Description: "z desc"},
		{ID: "alpha", Name: "Alpha 助手", Description: "a desc"},
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

// TestStaticCatalogDescribe asserts Describe returns the entry's per-agent
// Name/Description (the framework fills registration fields and derives
// capabilities from the wired Provider fields, so those are not Describe's job).
func TestStaticCatalogDescribe(t *testing.T) {
	c := NewStaticCatalog("cattest", testCatalogEntries())
	info, err := c.Describe("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Alpha 助手" || info.Description != "a desc" {
		t.Fatalf("Describe should return the entry Name/Description, got %+v", info)
	}
	// Describe carries no capabilities/parameters — those are the framework's job.
	if len(info.Parameters) != 0 || len(info.Skills) != 0 {
		t.Fatalf("Describe should not populate Parameters/Skills, got %+v", info)
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
	// Describe goes through the same Lookup path.
	if _, err := c.Describe("nonexistent"); !errors.As(err, &ve) {
		t.Fatalf("Describe with an unknown id should return the same typed error, got %v", err)
	}
}

// TestStaticCatalogDuplicateIDPanic asserts a duplicate entry ID triggers a
// fail-fast panic (aligned with the Register convention).
func TestStaticCatalogDuplicateIDPanic(t *testing.T) {
	entries := []CatalogEntry{{ID: "a"}, {ID: "a"}}
	mustPanic(t, "duplicate entry ID", func() { NewStaticCatalog("cattest", entries) })
}

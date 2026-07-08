// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package agenttest provides provider conformance tests: a new integrator calls
// RunConformance in its own test to lock down registration metadata, the
// zero-value Deps contract, Card single-sourcing, and other implicit contracts.
// All assertions run offline (zero-value Deps, no API calls).
package agenttest

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/agent"
)

// RunConformance runs the full set of conformance assertions against a
// registered scheme. sampleAgentID must be a valid agent id for which the
// provider can produce a Card (catalog-type: an id from the catalog;
// instance-type: any non-empty id).
func RunConformance(t *testing.T, scheme, sampleAgentID string) {
	t.Helper()
	info, ok := agent.Info(scheme)
	if !ok {
		t.Fatalf("conformance: scheme %q not registered (the provider package must be imported to trigger init registration)", scheme)
	}

	t.Run("metadata", func(t *testing.T) {
		if info.Label == "" {
			t.Error("conformance: ProviderInfo.Label must not be empty")
		}
		if info.AgentIDSource == "" {
			t.Error("conformance: ProviderInfo.AgentIDSource must not be empty")
		}
		if info.Kind != agent.KindCatalog && info.Kind != agent.KindInstance {
			t.Errorf("conformance: Kind should be %q|%q, got %q", agent.KindCatalog, agent.KindInstance, info.Kind)
		}
		if !strings.HasPrefix(info.AgentRefFormat, scheme+":") {
			t.Errorf("conformance: AgentRefFormat should start with %q, got %q", scheme+":", info.AgentRefFormat)
		}
		if len(info.Identities) == 0 {
			t.Error("conformance: Identities must not be empty")
		}
		for i, id := range info.Identities {
			if id.Type != agent.IdentityUser && id.Type != agent.IdentityBot {
				t.Errorf("conformance: Identities[%d].Type should be user|bot, got %q", i, id.Type)
			}
		}
		seen := make(map[string]bool, len(info.RequiredScopes))
		for _, s := range info.RequiredScopes {
			if seen[s] {
				t.Errorf("conformance: RequiredScopes contains duplicate %q", s)
			}
			seen[s] = true
		}
	})

	t.Run("factory", func(t *testing.T) {
		p, err := info.Factory(agent.Deps{}, sampleAgentID)
		if err != nil {
			t.Fatalf("conformance: Factory must accept zero-value Deps (expected nil error), got %v", err)
		}
		if p == nil {
			t.Fatal("conformance: Factory must not return a nil provider")
		}
		// Core fields are mandatory (the command layer dispatches them without a
		// nil-check); Register enforces this at registration, re-assert here.
		if p.Send == nil {
			t.Error("conformance: Provider.Send (core) must be wired")
		}
		if p.GetTask == nil {
			t.Error("conformance: Provider.GetTask (core) must be wired")
		}
	})

	t.Run("card", func(t *testing.T) {
		newCard := func() *agent.AgentCard {
			t.Helper()
			p, err := info.Factory(agent.Deps{}, sampleAgentID)
			if err != nil {
				t.Fatalf("conformance: Factory(zero-value Deps) returned error: %v", err)
			}
			card, err := agent.BuildCard(context.Background(), scheme, sampleAgentID, p)
			if err != nil {
				t.Fatalf("conformance: BuildCard should be available offline (expected nil error), got %v", err)
			}
			if card == nil {
				t.Fatal("conformance: Card must not return nil")
			}
			return card
		}
		card := newCard()
		if card.Provider != scheme {
			t.Errorf("conformance: Card.Provider should be %q, got %q", scheme, card.Provider)
		}
		if card.AgentID != sampleAgentID {
			t.Errorf("conformance: Card.AgentID should echo the constructor input %q, got %q", sampleAgentID, card.AgentID)
		}
		if card.ProviderLabel != info.Label {
			t.Errorf("conformance: Card.ProviderLabel should equal the registered Label %q, got %q", info.Label, card.ProviderLabel)
		}
		if !reflect.DeepEqual(card.Identity, info.Identities) {
			t.Errorf("conformance: Card.Identity should match the registered Identities (single source), expected %+v got %+v", info.Identities, card.Identity)
		}
		if card.AgentIDSource != info.AgentIDSource {
			t.Errorf("conformance: Card.AgentIDSource should equal the registered value %q, got %q", info.AgentIDSource, card.AgentIDSource)
		}
		if card.Parameters == nil {
			t.Error("conformance: Card.Parameters must not be nil (always emitted, empty is [])")
		}
		// Single-sourcing: two independent instances each produce a Card, and the
		// results must DeepEqual (no hidden instance state).
		if card2 := newCard(); !reflect.DeepEqual(card, card2) {
			t.Errorf("conformance: Cards from two instances should DeepEqual (single source), got\n%+v\nvs\n%+v", card, card2)
		}
	})

	if info.Kind == agent.KindCatalog {
		t.Run("discovery", func(t *testing.T) {
			p, err := info.Factory(agent.Deps{}, sampleAgentID)
			if err != nil {
				t.Fatalf("conformance: Factory(zero-value Deps) returned error: %v", err)
			}
			if p.ListAgents == nil {
				t.Fatal("conformance: catalog-type provider must wire ListAgents")
			}
			list, err := p.ListAgents(context.Background())
			if err != nil {
				t.Fatalf("conformance: catalog-type ListAgents should be available offline (expected nil error), got %v", err)
			}
			wantRef := scheme + ":" + sampleAgentID
			found := false
			for i, a := range list {
				r, err := agent.ParseRef(a.AgentRef)
				if err != nil {
					t.Errorf("conformance: ListAgents[%d].AgentRef %q should be parseable by agent.ParseRef: %v", i, a.AgentRef, err)
					continue
				}
				if r.Scheme != scheme {
					t.Errorf("conformance: ListAgents[%d].AgentRef %q scheme should be %q, got %q", i, a.AgentRef, scheme, r.Scheme)
				}
				if a.Name == "" {
					t.Errorf("conformance: ListAgents[%d] (%s) Name must not be empty", i, a.AgentRef)
				}
				if a.AgentRef == wantRef {
					found = true
				}
			}
			if !found {
				t.Errorf("conformance: sampleAgentID should appear in the enumeration (expected to contain %q), got %+v", wantRef, list)
			}
			list2, err := p.ListAgents(context.Background())
			if err != nil {
				t.Fatalf("conformance: second ListAgents returned error: %v", err)
			}
			if !reflect.DeepEqual(list, list2) {
				t.Errorf("conformance: two consecutive ListAgents results should DeepEqual (stable enumeration), got\n%+v\nvs\n%+v", list, list2)
			}
		})
	}
}

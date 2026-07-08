// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package agenttest provides provider conformance tests: a new integrator calls
// RunConformance in its own test to lock down registration metadata, offline
// resolution, the mandatory core hooks, and single-sourced card derivation. All
// assertions run offline (no runtime, no API calls).
package agenttest

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/agent"
)

// RunConformance runs the full set of conformance assertions against a
// registered scheme. sampleAgentID must be a valid agent id (catalog: an id from
// the Catalog; instance: any non-empty id).
func RunConformance(t *testing.T, scheme, sampleAgentID string) {
	t.Helper()
	prov, ok := agent.Info(scheme)
	if !ok {
		t.Fatalf("conformance: scheme %q not registered (the top-level agent package must be imported to trigger init registration)", scheme)
	}

	t.Run("metadata", func(t *testing.T) {
		if prov.Scheme != scheme {
			t.Errorf("conformance: Provider.Scheme should be %q, got %q", scheme, prov.Scheme)
		}
		if prov.Label == "" {
			t.Error("conformance: Provider.Label must not be empty")
		}
		if prov.AgentIDSource == "" {
			t.Error("conformance: Provider.AgentIDSource must not be empty")
		}
		if len(prov.Identities) == 0 {
			t.Error("conformance: Identities must not be empty")
		}
		for i, id := range prov.Identities {
			if id.Type != agent.IdentityUser && id.Type != agent.IdentityBot {
				t.Errorf("conformance: Identities[%d].Type should be user|bot, got %q", i, id.Type)
			}
		}
		// Exactly one of Catalog / Instance is set (Register enforces; re-assert).
		if (len(prov.Catalog) > 0) == (prov.Instance != nil) {
			t.Error("conformance: exactly one of Catalog / Instance must be set")
		}
		seen := make(map[string]bool, len(prov.RequiredScopes))
		for _, s := range prov.RequiredScopes {
			if seen[s] {
				t.Errorf("conformance: RequiredScopes contains duplicate %q", s)
			}
			seen[s] = true
		}
	})

	t.Run("lookup", func(t *testing.T) {
		gotProv, spec, agentID, err := agent.LookupSpec(scheme + ":" + sampleAgentID)
		if err != nil {
			t.Fatalf("conformance: LookupSpec(%s:%s) offline should succeed, got %v", scheme, sampleAgentID, err)
		}
		if gotProv.Scheme != scheme {
			t.Errorf("conformance: LookupSpec provider scheme should be %q, got %q", scheme, gotProv.Scheme)
		}
		if agentID != sampleAgentID {
			t.Errorf("conformance: LookupSpec should echo the agent id %q, got %q", sampleAgentID, agentID)
		}
		// Core hooks are mandatory (the command layer dispatches them without a
		// nil-check); Register enforces this at registration, re-assert here.
		if spec.Send == nil {
			t.Error("conformance: spec.Send (core) must be wired")
		}
		if spec.GetTask == nil {
			t.Error("conformance: spec.GetTask (core) must be wired")
		}
	})

	t.Run("card", func(t *testing.T) {
		buildCard := func() *agent.AgentCard {
			t.Helper()
			_, spec, agentID, err := agent.LookupSpec(scheme + ":" + sampleAgentID)
			if err != nil {
				t.Fatalf("conformance: LookupSpec returned error: %v", err)
			}
			// rt=nil: the guaranteed-offline card (caps + registration + static
			// metadata). Describe enrichment is never exercised here.
			return agent.BuildCard(context.Background(), prov, spec, agentID, nil)
		}
		card := buildCard()
		if card.Provider != scheme {
			t.Errorf("conformance: Card.Provider should be %q, got %q", scheme, card.Provider)
		}
		if card.AgentID != sampleAgentID {
			t.Errorf("conformance: Card.AgentID should echo the input %q, got %q", sampleAgentID, card.AgentID)
		}
		if card.ProviderLabel != prov.Label {
			t.Errorf("conformance: Card.ProviderLabel should equal the registered Label %q, got %q", prov.Label, card.ProviderLabel)
		}
		if !reflect.DeepEqual(card.Identity, prov.Identities) {
			t.Errorf("conformance: Card.Identity should match the registered Identities, expected %+v got %+v", prov.Identities, card.Identity)
		}
		if card.AgentIDSource != prov.AgentIDSource {
			t.Errorf("conformance: Card.AgentIDSource should equal the registered value %q, got %q", prov.AgentIDSource, card.AgentIDSource)
		}
		if card.Parameters == nil {
			t.Error("conformance: Card.Parameters must not be nil (always emitted, empty is [])")
		}
		if !card.Capabilities.TaskGet {
			t.Error("conformance: task_get must be true (GetTask is a mandatory core hook)")
		}
		// Single-sourcing: two independent offline builds must DeepEqual.
		if card2 := buildCard(); !reflect.DeepEqual(card, card2) {
			t.Errorf("conformance: two offline BuildCard results should DeepEqual (single source), got\n%+v\nvs\n%+v", card, card2)
		}
	})

	if prov.Kind() == agent.KindCatalog {
		t.Run("enumeration", func(t *testing.T) {
			list := prov.ListCatalog()
			wantRef := scheme + ":" + sampleAgentID
			found := false
			for i, a := range list {
				r, err := agent.ParseRef(a.AgentRef)
				if err != nil {
					t.Errorf("conformance: ListCatalog[%d].AgentRef %q should be parseable: %v", i, a.AgentRef, err)
					continue
				}
				if r.Scheme != scheme {
					t.Errorf("conformance: ListCatalog[%d].AgentRef %q scheme should be %q, got %q", i, a.AgentRef, scheme, r.Scheme)
				}
				if a.Name == "" {
					t.Errorf("conformance: ListCatalog[%d] (%s) Name must not be empty", i, a.AgentRef)
				}
				if a.AgentRef == wantRef {
					found = true
				}
			}
			if !found {
				t.Errorf("conformance: sampleAgentID should appear in the enumeration (expected %q), got %+v", wantRef, list)
			}
			// stable, sorted by AgentRef.
			list2 := prov.ListCatalog()
			if !reflect.DeepEqual(list, list2) {
				t.Errorf("conformance: two ListCatalog results should DeepEqual (stable), got\n%+v\nvs\n%+v", list, list2)
			}
			for i := 1; i < len(list); i++ {
				if strings.Compare(list[i-1].AgentRef, list[i].AgentRef) > 0 {
					t.Errorf("conformance: ListCatalog should be sorted by AgentRef, got %q before %q", list[i-1].AgentRef, list[i].AgentRef)
				}
			}
		})
	}
}

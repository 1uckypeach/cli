// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"sort"

	"github.com/larksuite/cli/errs"
)

// CatalogEntry is the provider-neutral description of one predefined agent in a
// catalog-type provider. It holds only fields the framework can consume
// (enumeration / Card synthesis); a provider's private business fields (such as
// the execution backend it points to) are maintained alongside in the
// integrator's own package and do not enter framework types.
type CatalogEntry struct {
	ID           string
	Name         string
	Description  string
	Capabilities Capabilities
}

// StaticCatalog carries the common boilerplate of a catalog-type provider:
// catalog enumeration (Discoverer semantics), per-agent rich Card (NewCard fills
// in registration-time fields, the entry adds Name/Description/Capabilities), and
// a typed validation error for unknown ids. Business differences (such as the
// execution backend) are composed by the provider itself on the outer layer.
// It is read-only after construction and safe for concurrent use.
type StaticCatalog struct {
	scheme  string
	entries map[string]CatalogEntry
}

// NewStaticCatalog constructs a static catalog. A duplicate entry ID is an
// integrator coding error and panics fail-fast (aligned with the Register convention).
func NewStaticCatalog(scheme string, entries []CatalogEntry) *StaticCatalog {
	m := make(map[string]CatalogEntry, len(entries))
	for _, e := range entries {
		if _, dup := m[e.ID]; dup {
			panic("agent: StaticCatalog duplicate entry ID for scheme " + scheme + ": " + e.ID)
		}
		m[e.ID] = e
	}
	return &StaticCatalog{scheme: scheme, entries: m}
}

// ListAgents enumerates the catalog (Discoverer semantics), sorted by AgentRef
// to guarantee stable output.
func (c *StaticCatalog) ListAgents(ctx context.Context) ([]AgentSummary, error) {
	out := make([]AgentSummary, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, AgentSummary{
			AgentRef:    c.scheme + ":" + e.ID,
			Name:        e.Name,
			Description: e.Description,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AgentRef < out[j].AgentRef })
	return out, nil
}

// Card synthesizes the agent's rich card: NewCard fills in registration-time
// fields (Provider/Label/Identity/AgentIDSource/empty Parameters), and the entry
// adds the per-agent Name/Description/Capabilities. An unknown id returns the
// typed error from Lookup.
func (c *StaticCatalog) Card(agentID string) (*AgentCard, error) {
	e, err := c.Lookup(agentID)
	if err != nil {
		return nil, err
	}
	card := NewCard(c.scheme, agentID)
	card.Name = e.Name
	card.Description = e.Description
	card.Capabilities = e.Capabilities
	return card, nil
}

// Lookup fetches a catalog entry by id. An unknown id returns a typed
// validation/invalid_argument error (exit 2) whose hint points to
// `agent list <scheme>`, so a provider need not define its own unknown-agent error.
func (c *StaticCatalog) Lookup(agentID string) (CatalogEntry, error) {
	e, ok := c.entries[agentID]
	if !ok {
		return CatalogEntry{}, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"未知的 %s agent '%s'", c.scheme, agentID).
			WithHint("运行 lark-cli agent list %s 查看可用 agent", c.scheme)
	}
	return e, nil
}

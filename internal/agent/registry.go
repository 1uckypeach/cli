// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/larksuite/cli/errs"
)

// ProviderKind is the closed set of provider forms, derived from whether a
// Provider set Catalog or Instance (exposed via Provider.Kind()).
type ProviderKind string

const (
	// KindCatalog: the full agent set is known offline (Provider.Catalog).
	KindCatalog ProviderKind = "catalog"
	// KindInstance: agents are created on the platform at runtime, addressed by an
	// unbounded agent_id (Provider.Instance).
	KindInstance ProviderKind = "instance"
)

var providerRegistry = map[string]Provider{}

// Register records a provider (called from the agent/register.go aggregator,
// mirroring events/shortcuts). It is pure struct validation — no construction,
// no probe. Missing / invalid metadata is an integrator coding error and panics
// fail-fast (aligned with the sql.Register convention, including duplicate
// registration).
func Register(p Provider) {
	switch {
	case p.Scheme == "":
		panic("agent: provider registration with empty Scheme")
	case p.Label == "":
		panic("agent: provider missing Label: " + p.Scheme)
	case p.AgentIDSource == "":
		panic("agent: provider missing AgentIDSource: " + p.Scheme)
	case len(p.Identities) == 0:
		panic("agent: provider missing Identities: " + p.Scheme)
	}
	if _, dup := providerRegistry[p.Scheme]; dup {
		panic("agent: Register called twice for scheme: " + p.Scheme)
	}
	for _, id := range p.Identities {
		if id.Type != IdentityUser && id.Type != IdentityBot {
			panic("agent: provider invalid Identity Type (want user|bot): " + p.Scheme + ", got: " + string(id.Type))
		}
	}
	hasCatalog, hasInstance := len(p.Catalog) > 0, p.Instance != nil
	if hasCatalog == hasInstance {
		panic("agent: provider must set exactly one of Catalog / Instance: " + p.Scheme)
	}
	if hasCatalog {
		seen := make(map[string]bool, len(p.Catalog))
		for i := range p.Catalog {
			checkSpec(p.Scheme, &p.Catalog[i], true)
			if seen[p.Catalog[i].ID] {
				panic("agent: catalog duplicate entry ID for scheme " + p.Scheme + ": " + p.Catalog[i].ID)
			}
			seen[p.Catalog[i].ID] = true
		}
	} else {
		checkSpec(p.Scheme, p.Instance, false)
	}
	providerRegistry[p.Scheme] = p
}

// checkSpec asserts the mandatory core hooks and the ID rule for one spec. The
// command layer dispatches Send/GetTask without a nil-check, so they must exist.
func checkSpec(scheme string, s *AgentSpec, catalog bool) {
	if s.Send == nil {
		panic("agent: spec missing core Send: " + scheme + ":" + s.ID)
	}
	if s.GetTask == nil {
		panic("agent: spec missing core GetTask: " + scheme + ":" + s.ID)
	}
	if catalog && s.ID == "" {
		panic("agent: catalog spec missing ID: " + scheme)
	}
	if !catalog && s.ID != "" {
		panic("agent: instance template must have empty ID: " + scheme + ", got: " + s.ID)
	}
}

// Info returns the registered provider for a scheme (ok=false if not registered).
func Info(scheme string) (Provider, bool) {
	p, ok := providerRegistry[scheme]
	return p, ok
}

// LookupSpec resolves the AgentSpec addressed by ref, fully offline: it parses
// the ref, finds the provider, and returns the matching spec (the instance
// template, or the catalog entry whose ID matches) plus the parsed agent_id (so
// callers need not re-parse for rt.AgentID() / the card). An unknown scheme or
// unknown catalog id returns a typed error (the command layer promotes
// ParseRef/scheme errors via wrapRefResolveError; the unknown-id error is
// already typed).
func LookupSpec(ref string) (Provider, *AgentSpec, string, error) {
	r, err := ParseRef(ref)
	if err != nil {
		return Provider{}, nil, "", err
	}
	p, ok := providerRegistry[r.Scheme]
	if !ok {
		return Provider{}, nil, "", fmt.Errorf("未知的 agent provider '%s'，当前支持: %s", r.Scheme, KnownSchemes())
	}
	if p.Instance != nil {
		return p, p.Instance, r.AgentID, nil
	}
	for i := range p.Catalog {
		if p.Catalog[i].ID == r.AgentID {
			return p, &p.Catalog[i], r.AgentID, nil
		}
	}
	return p, nil, "", errs.NewValidationError(errs.SubtypeInvalidArgument,
		"未知的 %s agent '%s'", r.Scheme, r.AgentID).
		WithHint("运行 lark-cli agent list %s 查看可用 agent", r.Scheme)
}

// Kind reports the provider form derived from Catalog vs Instance.
func (p Provider) Kind() ProviderKind {
	if p.Instance != nil {
		return KindInstance
	}
	return KindCatalog
}

// AgentRefFormat is the written form of an agent_ref for this provider, always
// "<scheme>:<agent_id>" (derived, not stored).
func (p Provider) AgentRefFormat() string {
	return p.Scheme + ":<agent_id>"
}

// ListCatalog is the offline enumeration for a catalog provider (sorted by
// AgentRef, stable). An instance provider has no static set and returns nil — the
// command layer then falls back to the optional ListAgents online hook.
func (p Provider) ListCatalog() []AgentSummary {
	if p.Instance != nil {
		return nil
	}
	out := make([]AgentSummary, 0, len(p.Catalog))
	for _, s := range p.Catalog {
		out = append(out, AgentSummary{
			AgentRef:    p.Scheme + ":" + s.ID,
			Name:        s.Name,
			Description: s.Description,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AgentRef < out[j].AgentRef })
	return out
}

// KnownSchemes returns a comma-separated list of registered schemes (stably
// sorted), or "(none)" when empty (reused by cmd/agent's unknown-scheme message).
func KnownSchemes() string {
	s := RegisteredSchemes()
	if len(s) == 0 {
		return "(none)"
	}
	return strings.Join(s, ", ")
}

// RegisteredSchemes lets `agent list` enumerate registered providers (sorted).
func RegisteredSchemes() []string {
	s := make([]string, 0, len(providerRegistry))
	for k := range providerRegistry {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

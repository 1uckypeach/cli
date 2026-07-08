// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/larksuite/cli/internal/client"
	"github.com/larksuite/cli/internal/core"
)

// Deps are the dependencies a provider factory needs (injected by the command
// layer to avoid internal/agent depending on cmd).
type Deps struct {
	Client *client.APIClient
	As     core.Identity
}

// Factory constructs a Provider from an agentID plus dependencies.
type Factory func(deps Deps, agentID string) (*Provider, error)

// ProviderKind is the closed set of provider forms (validated at Register time
// to guard against cast typos).
type ProviderKind string

const (
	// KindCatalog is the catalog type: the full agent set is known at
	// registration time, and it must wire Provider.ListAgents.
	KindCatalog ProviderKind = "catalog"
	// KindInstance is the instance type: agents are created by users on the
	// platform and cannot be enumerated by the CLI in advance.
	KindInstance ProviderKind = "instance"
)

// ProviderInfo is a provider's registration contract: metadata beyond Factory
// consumed by platform capabilities such as `agent list`, card synthesis, and
// scope preflight. Everything except RequiredScopes is required (Register
// validates fail-fast).
type ProviderInfo struct {
	// Factory constructs the Provider for this scheme. Factory must accept
	// zero-value Deps and have no side effects during construction — this
	// contract is enforced at registration time by Register's zero-value Deps
	// probe (a violation panics), and agent list also constructs a probe
	// instance with empty Deps to read the ListAgents capability
	// (cmd/agent/list.go probeDiscoverer). Because the probe passes zero Deps and
	// empty agentID, capability wiring must not depend on either.
	Factory Factory
	// Label is the user-facing provider name.
	Label string
	// AgentRefFormat is the written format of agent_ref, e.g. "example:<agent_id>";
	// it must be prefixed with "<scheme>:" (validated by Register).
	AgentRefFormat string
	// AgentIDSource tells the user where to obtain the agent_id (key information
	// for AI-guided onboarding).
	AgentIDSource string
	// Kind is the provider form: KindCatalog (catalog type) or KindInstance
	// (instance type). Catalog types must wire Provider.ListAgents (asserted at
	// Register time).
	Kind ProviderKind
	// RequiredScopes is the full (flat) set of scopes needed by any real API
	// call this provider makes; preflight is all-or-nothing.
	RequiredScopes []string
	// Identities declares the supported calling identities and their
	// preconditions; non-empty and Type ∈ {user, bot} (validated by Register).
	Identities []IdentitySpec
}

var providerRegistry = map[string]ProviderInfo{}

// Register is called by each adapter package in its init() to register itself
// (exported so adapter packages like example can call it across packages).
// Missing / invalid metadata is an integrator coding error and panics fail-fast
// (including duplicate registration, aligned with the sql.Register convention).
// At registration time it also constructs a Provider once via a zero-value Deps
// probe: Factory must accept zero-value Deps (returning an error panics), and a
// KindCatalog instance must implement Discoverer.
func Register(scheme string, info ProviderInfo) {
	if scheme == "" {
		panic("agent: provider registration with empty scheme")
	}
	if _, dup := providerRegistry[scheme]; dup {
		panic("agent: Register called twice for scheme: " + scheme)
	}
	switch {
	case info.Factory == nil:
		panic("agent: provider registration missing Factory: " + scheme)
	case info.Label == "":
		panic("agent: provider registration missing Label: " + scheme)
	case info.AgentRefFormat == "":
		panic("agent: provider registration missing AgentRefFormat: " + scheme)
	case !strings.HasPrefix(info.AgentRefFormat, scheme+":"):
		panic("agent: provider registration AgentRefFormat must start with \"" + scheme + ":\": " + scheme + ", got: " + info.AgentRefFormat)
	case info.AgentIDSource == "":
		panic("agent: provider registration missing AgentIDSource: " + scheme)
	case info.Kind != KindCatalog && info.Kind != KindInstance:
		panic("agent: provider registration invalid Kind (want catalog|instance): " + scheme + ", got: " + string(info.Kind))
	case len(info.Identities) == 0:
		panic("agent: provider registration missing Identities: " + scheme)
	}
	for _, id := range info.Identities {
		if id.Type != IdentityUser && id.Type != IdentityBot {
			panic("agent: provider registration invalid Identity Type (want user|bot): " + scheme + ", got: " + string(id.Type))
		}
	}
	// Zero-value Deps construction probe: turns the Factory contract (see the
	// ProviderInfo.Factory comment) from a pure convention into a
	// registration-time enforcement, preventing capabilities from silently
	// disappearing on the agent list probing path.
	p, err := info.Factory(Deps{}, "")
	if err != nil {
		panic("agent: provider factory must accept zero-value Deps: " + scheme + ", got error: " + err.Error())
	}
	if p == nil {
		panic("agent: provider factory returned nil Provider: " + scheme)
	}
	// Core capabilities are mandatory for every provider — a provider you cannot
	// send to or read a task back from is not usable. The command layer relies on
	// these never being nil (no nil-check before dispatch), so enforce it here.
	switch {
	case p.Send == nil:
		panic("agent: provider missing core Send: " + scheme)
	case p.GetTask == nil:
		panic("agent: provider missing core GetTask: " + scheme)
	}
	// A catalog provider's full agent set is known offline, so it must be
	// enumerable (wire ListAgents); an instance provider need not be.
	if info.Kind == KindCatalog && p.ListAgents == nil {
		panic("agent: catalog provider must wire ListAgents: " + scheme)
	}
	providerRegistry[scheme] = info
}

// Info returns the registration value for a scheme (the struct is returned by
// value, but its slice fields share the underlying array with the registry, so
// the caller must treat them as read-only); returns ok=false if not registered.
func Info(scheme string) (ProviderInfo, bool) {
	info, ok := providerRegistry[scheme]
	return info, ok
}

// providerFor fetches the factory for a scheme and constructs a Provider. An
// unknown scheme returns an error listing the available options.
func providerFor(scheme, agentID string, deps Deps) (*Provider, error) {
	info, ok := providerRegistry[scheme]
	if !ok {
		return nil, fmt.Errorf("未知的 agent provider '%s'，当前支持: %s", scheme, KnownSchemes())
	}
	return info.Factory(deps, agentID)
}

// KnownSchemes returns a comma-separated list of registered schemes (stably
// sorted), or "(none)" when empty (exported: cmd/agent's unknown-scheme message
// reuses the same implementation to avoid double-sourcing).
func KnownSchemes() string {
	s := RegisteredSchemes()
	if len(s) == 0 {
		return "(none)"
	}
	return strings.Join(s, ", ")
}

// Resolve parses a ref and constructs the corresponding Provider (command-layer entry point).
func Resolve(ref string, deps Deps) (*Provider, error) {
	r, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	return providerFor(r.Scheme, r.AgentID, deps)
}

// RegisteredSchemes lets `agent list` enumerate registered providers (exported for cmd/agent).
func RegisteredSchemes() []string {
	s := make([]string, 0, len(providerRegistry))
	for k := range providerRegistry {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

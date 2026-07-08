// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// swapRegistry replaces the global providerRegistry with the given map (restored
// automatically via t.Cleanup), for test isolation. It swaps the global variable
// without a lock, so callers must not use t.Parallel.
func swapRegistry(t *testing.T, m map[string]ProviderInfo) {
	t.Helper()
	saved := providerRegistry
	providerRegistry = m
	t.Cleanup(func() { providerRegistry = saved })
}

// testInfo builds a minimal ProviderInfo that passes Register validation
// (AgentRefFormat is generated from the scheme so it satisfies the prefix check),
// reused by cases that only care about the Factory.
func testInfo(scheme string, f Factory) ProviderInfo {
	return ProviderInfo{
		Factory:        f,
		Label:          "test provider",
		AgentRefFormat: scheme + ":<agent_id>",
		AgentIDSource:  "test source",
		Kind:           KindInstance,
		Identities:     []IdentitySpec{{Type: IdentityUser}},
	}
}

// mustPanic asserts that fn panics and the message contains wantMsg.
func mustPanic(t *testing.T, wantMsg string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("should panic (want message containing %q)", wantMsg)
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, wantMsg) {
			t.Fatalf("panic message should contain %q, got %q", wantMsg, msg)
		}
	}()
	fn()
}

// TestRegisterPanicBranches table-drives the Register fail-fast panic branches
// on metadata fields: missing Factory / Label / AgentRefFormat / AgentIDSource /
// Identities, an invalid Kind, an invalid Identity Type, and an AgentRefFormat
// that does not start with "<scheme>:" (panic messages must carry the actual
// offending value).
func TestRegisterPanicBranches(t *testing.T) {
	nop := func(Deps, string) (Provider, error) { return nil, nil }
	cases := []struct {
		name    string
		mutate  func(info *ProviderInfo)
		wantMsg string
	}{
		{"missing Factory", func(info *ProviderInfo) { info.Factory = nil }, "missing Factory"},
		{"missing Label", func(info *ProviderInfo) { info.Label = "" }, "missing Label"},
		{"missing AgentRefFormat", func(info *ProviderInfo) { info.AgentRefFormat = "" }, "missing AgentRefFormat"},
		{"missing AgentIDSource", func(info *ProviderInfo) { info.AgentIDSource = "" }, "missing AgentIDSource"},
		{"invalid Kind", func(info *ProviderInfo) { info.Kind = "weird" }, "got: weird"},
		{"missing Identities", func(info *ProviderInfo) { info.Identities = nil }, "missing Identities"},
		{"invalid Identity Type", func(info *ProviderInfo) {
			info.Identities = []IdentitySpec{{Type: "robot"}}
		}, "got: robot"},
		{"AgentRefFormat wrong prefix", func(info *ProviderInfo) {
			info.AgentRefFormat = "other:<agent_id>"
		}, "must start with \"bad:\""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			swapRegistry(t, map[string]ProviderInfo{})
			info := testInfo("bad", nop)
			tc.mutate(&info)
			mustPanic(t, tc.wantMsg, func() { Register("bad", info) })
		})
	}
}

// TestRegisterEmptyScheme pins the empty-scheme fail-fast branch.
func TestRegisterEmptyScheme(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	nop := func(Deps, string) (Provider, error) { return nil, nil }
	mustPanic(t, "empty scheme", func() { Register("", testInfo("", nop)) })
}

// TestRegisterDuplicateScheme pins the sql.Register-style dup panic.
func TestRegisterDuplicateScheme(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	nop := func(Deps, string) (Provider, error) { return nil, nil }
	Register("dup", testInfo("dup", nop))
	mustPanic(t, "called twice for scheme: dup", func() { Register("dup", testInfo("dup", nop)) })
}

// TestRegisterFactoryZeroDepsProbe pins the registration-time zero-Deps probe:
// a factory erroring under zero-value Deps is a contract violation and panics.
func TestRegisterFactoryZeroDepsProbe(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	bad := func(Deps, string) (Provider, error) { return nil, errors.New("need client") }
	mustPanic(t, "must accept zero-value Deps", func() { Register("zd", testInfo("zd", bad)) })
}

// TestRegisterCatalogRequiresDiscoverer pins the catalog-archetype MUST:
// a KindCatalog provider whose probe instance lacks Discoverer panics.
func TestRegisterCatalogRequiresDiscoverer(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	info := testInfo("cat", func(Deps, string) (Provider, error) { return &stubProvider{}, nil })
	info.Kind = KindCatalog
	mustPanic(t, "must implement Discoverer", func() { Register("cat", info) })
}

func TestInfoReturnsRegisteredMetadata(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	Register("t1", ProviderInfo{
		Factory:        func(Deps, string) (Provider, error) { return nil, nil },
		Label:          "测试 provider",
		AgentRefFormat: "t1:<agent_id>",
		AgentIDSource:  "在 T1 控制台获取",
		Kind:           KindInstance,
		RequiredScopes: []string{"t1:chat:write"},
		Identities:     []IdentitySpec{{Type: IdentityUser}},
	})
	info, ok := Info("t1")
	if !ok || info.Label != "测试 provider" || info.Kind != KindInstance {
		t.Fatalf("Info(t1) = %+v, %v", info, ok)
	}
	if _, ok := Info("nonexistent"); ok {
		t.Fatal("Info(nonexistent) should return ok=false")
	}
}

func TestRegistryUnknownScheme(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	// unknown scheme: the factory is never called, so deps value is irrelevant; use zero-value Deps{}.
	_, err := providerFor("nosuch", "agt_x", Deps{})
	if err == nil {
		t.Fatal("unknown scheme should return an error")
	}
}

func TestRegistryKnownScheme(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	// The factory passes the zero-value Deps probe (empty agentID) and only errors on a real construction, staying compatible with the registration-time probe.
	Register("stub", testInfo("stub", func(f Deps, agentID string) (Provider, error) {
		if agentID == "" {
			return nil, nil
		}
		return nil, errors.New("stub called")
	}))
	_, err := providerFor("stub", "agt_x", Deps{})
	if err == nil || err.Error() != "stub called" {
		t.Fatalf("should reach the stub factory, got %v", err)
	}
}

func TestKnownSchemesEmpty(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	if got := KnownSchemes(); got != "(none)" {
		t.Fatalf("an empty registry should return \"(none)\", got %q", got)
	}
}

func TestRegisteredSchemesSorted(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	nop := func(Deps, string) (Provider, error) { return nil, nil }
	// Register out of order to verify enumeration + sort stability.
	Register("gamma", testInfo("gamma", nop))
	Register("alpha", testInfo("alpha", nop))
	Register("beta", testInfo("beta", nop))
	got := RegisteredSchemes()
	want := []string{"alpha", "beta", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RegisteredSchemes should enumerate and sort, want %v got %v", want, got)
	}
	// knownSchemes reuses RegisteredSchemes; verify the comma joining.
	if s := KnownSchemes(); s != "alpha, beta, gamma" {
		t.Fatalf("knownSchemes should be comma-joined, got %q", s)
	}
}

func TestResolveInvalidRef(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	// Missing the <scheme>:<agent_id> separator, so ParseRef errors and Resolve propagates it as-is.
	_, err := Resolve("no-colon", Deps{})
	if !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("an invalid ref should propagate ErrInvalidRef, got %v", err)
	}
}

func TestResolveUnknownScheme(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	// The ref is valid but the scheme is unregistered, so the error comes from providerFor.
	_, err := Resolve("nosuch:agt_x", Deps{})
	if err == nil {
		t.Fatal("an unregistered scheme should return an error")
	}
	if errors.Is(err, ErrInvalidRef) {
		t.Fatalf("an unregistered scheme should not be ErrInvalidRef, got %v", err)
	}
}

func TestResolveSuccess(t *testing.T) {
	swapRegistry(t, map[string]ProviderInfo{})
	sentinel := &stubProvider{}
	var gotDeps Deps
	var gotAgentID string
	Register("demo", testInfo("demo", func(deps Deps, agentID string) (Provider, error) {
		gotDeps = deps
		gotAgentID = agentID
		return sentinel, nil
	}))
	deps := Deps{}
	p, err := Resolve("demo:agt_42", deps)
	if err != nil {
		t.Fatalf("a valid ref + registered scheme should succeed, got %v", err)
	}
	if p != sentinel {
		t.Fatalf("should return the Provider built by the factory, got %v", p)
	}
	if gotAgentID != "agt_42" {
		t.Fatalf("factory should receive the parsed agentID, got %q", gotAgentID)
	}
	if gotDeps != deps {
		t.Fatalf("factory should receive the passed-in Deps, got %+v", gotDeps)
	}
}

// stubProvider is an empty Provider implementation used only for Resolve success-path assertions.
type stubProvider struct{ Provider }

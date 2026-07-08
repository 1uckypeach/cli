// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
)

// listFactory returns a Factory writing to a fresh stdout buffer plus a
// listOptions bound to it, ready to drive agentListRun without any API.
func listFactory() (*listOptions, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{Out: out, ErrOut: errOut}}
	return &listOptions{Factory: f, Format: "json"}, out
}

// decodeProviders unmarshals the envelope on out and returns data.providers.
func decodeProviders(t *testing.T, out *bytes.Buffer) []interface{} {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("output should be valid envelope JSON: %v (%s)", err, out.String())
	}
	data, _ := env.Data.(map[string]interface{})
	providers, _ := data["providers"].([]interface{})
	return providers
}

// findProvider returns the provider entry whose scheme matches, or nil.
func findProvider(providers []interface{}, scheme string) map[string]interface{} {
	for _, pv := range providers {
		p, _ := pv.(map[string]interface{})
		if p["scheme"] == scheme {
			return p
		}
	}
	return nil
}

// TestAgentListRun_ProviderFieldsV2 pins the provider entry contract: the
// example entry carries all fields sourced from iagent.Info (the single source
// of truth), the legacy free-text description field is gone, and discoverable
// is no longer exposed.
func TestAgentListRun_ProviderFieldsV2(t *testing.T) {
	opts, out := listFactory()
	if err := agentListRun(opts); err != nil {
		t.Fatalf("list should not error: %v", err)
	}
	info, ok := iagent.Info("example")
	if !ok {
		t.Fatal("the example provider should already be registered (blank import in agent.go)")
	}
	p := findProvider(decodeProviders(t, out), "example")
	if p == nil {
		t.Fatalf("list should include the example provider: %s", out.String())
	}
	if p["label"] != info.Label {
		t.Errorf("label should come from ProviderInfo.Label %q, got %v", info.Label, p["label"])
	}
	if p["agent_ref_format"] != info.AgentRefFormat {
		t.Errorf("agent_ref_format should come from ProviderInfo.AgentRefFormat %q, got %v", info.AgentRefFormat, p["agent_ref_format"])
	}
	if p["kind"] != string(info.Kind) {
		t.Errorf("kind should come from ProviderInfo.Kind %q, got %v", info.Kind, p["kind"])
	}
	if p["agent_id_source"] != info.AgentIDSource {
		t.Errorf("agent_id_source should come from ProviderInfo.AgentIDSource, got %v", p["agent_id_source"])
	}
	if _, present := p["description"]; present {
		t.Errorf("the old description field should be removed (double-source with label), got %v", p)
	}
	if _, present := p["discoverable"]; present {
		t.Errorf("the discoverable field should be removed from the provider list, got %v", p["discoverable"])
	}
}

// TestAgentListRun_EnvelopeShape verifies the JSON envelope carries
// data.providers[] with the full field contract.
func TestAgentListRun_EnvelopeShape(t *testing.T) {
	opts, out := listFactory()
	if err := agentListRun(opts); err != nil {
		t.Fatalf("list should not error: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("output should be valid envelope JSON: %v (%s)", err, out.String())
	}
	if !env.OK {
		t.Errorf("ok should be true: %+v", env)
	}
	providers := decodeProviders(t, out)
	if len(providers) == 0 {
		t.Fatalf("data.providers should be a non-empty array: %s", out.String())
	}
	first, ok := providers[0].(map[string]interface{})
	if !ok {
		t.Fatalf("provider entry should be an object, got %T", providers[0])
	}
	for _, key := range []string{"scheme", "label", "agent_ref_format", "kind", "agent_id_source"} {
		if _, present := first[key]; !present {
			t.Errorf("provider entry missing field %q: %v", key, first)
		}
	}
	if _, present := first["discoverable"]; present {
		t.Errorf("provider entry should not contain a discoverable field: %v", first)
	}
}

// TestAgentListDefaultFormatIsJSON pins the default flip: `agent list`
// without --format emits the JSON envelope (pretty is opt-in).
func TestAgentListDefaultFormatIsJSON(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{Out: out, ErrOut: errOut}}
	cmd := NewCmdAgentList(f)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent list should not error: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("default output should be a JSON envelope: %v (%s)", err, out.String())
	}
	if !env.OK {
		t.Errorf("ok should be true: %+v", env)
	}
}

// TestAgentListRun_PrettyFormat pins the opt-in --format pretty branch: a header
// row plus tab-separated provider lines, not a JSON envelope.
func TestAgentListRun_PrettyFormat(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{Out: out, ErrOut: errOut}}
	opts := &listOptions{Factory: f, Format: "pretty"}

	if err := agentListRun(opts); err != nil {
		t.Fatalf("list pretty should not error: %v", err)
	}
	text := out.String()
	// A pretty rendering is human text, not a JSON envelope.
	var env output.Envelope
	if json.Unmarshal(out.Bytes(), &env) == nil && env.OK {
		t.Fatalf("pretty format should not output a JSON envelope: %s", text)
	}
	if !strings.HasPrefix(text, "SCHEME") {
		t.Errorf("pretty output should start with a header row: %s", text)
	}
	if !strings.Contains(text, "example") {
		t.Errorf("pretty output should contain the example provider: %s", text)
	}
	if !strings.Contains(text, "example:<agent_id>") {
		t.Errorf("pretty output should contain the example ref format: %s", text)
	}
	// agent_id_source is surfaced as a footer (not a column) so the newcomer's
	// "where do I get an agent_id" cue does not disappear in the pretty view.
	if !strings.Contains(text, "agent_id 获取") {
		t.Errorf("pretty output should contain the agent_id_source footer hint: %s", text)
	}
}

// TestAgentListScheme_UnsupportedCapability pins that `agent list fakeflow`
// on a provider without Discoverer is unsupported_capability (exit 2) with the
// AgentIDSource text as hint, and — because the probe runs before any client
// construction — works on an unconfigured Factory.
func TestAgentListScheme_UnsupportedCapability(t *testing.T) {
	registerScripted()
	opts, _ := listFactory()
	opts.Scheme = "fakeflow"
	err := agentListRun(opts)
	if err == nil {
		t.Fatal("fakeflow does not implement Discoverer, so list fakeflow should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T (%v)", err, err)
	}
	if code := output.ExitCodeOf(err); code != output.ExitValidation {
		t.Fatalf("exit code should be 2, got %d", code)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.Subtype("unsupported_capability") {
		t.Fatalf("subtype should be unsupported_capability, got %+v", p)
	}
	if !strings.Contains(err.Error(), "provider 'fakeflow' 暂不支持列举 agent") {
		t.Errorf("message should state that listing is not supported, got %q", err.Error())
	}
	if !strings.Contains(p.Hint, fakeflowAgentIDSource) {
		t.Errorf("hint should be the AgentIDSource text, got %q", p.Hint)
	}
}

// TestAgentListScheme_UnknownScheme pins that an unregistered scheme is
// invalid_argument and the message lists the registered schemes.
func TestAgentListScheme_UnknownScheme(t *testing.T) {
	opts, _ := listFactory()
	opts.Scheme = "nosuch"
	err := agentListRun(opts)
	if err == nil {
		t.Fatal("an unknown scheme should error")
	}
	if !errs.IsValidation(err) {
		t.Fatalf("should be a validation error, got %T (%v)", err, err)
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("subtype should be invalid_argument, got %+v", p)
	}
	if !strings.Contains(err.Error(), "nosuch") || !strings.Contains(err.Error(), "example") {
		t.Errorf("message should contain the unknown scheme and the registered scheme list, got %q", err.Error())
	}
	// Hand-written validation errors carry a recovery hint pointing at
	// `agent list` for provider discovery.
	if !strings.Contains(p.Hint, "agent list") {
		t.Errorf("unknown-scheme hint should point to `agent list`, got %q", p.Hint)
	}
}

// fakeDiscProvider is a test-only provider implementing Discoverer, to pin the
// `agent list <scheme>` positive path without a real catalog provider.
type fakeDiscProvider struct{ iagent.Provider }

func (p *fakeDiscProvider) ListAgents(ctx context.Context) ([]iagent.AgentSummary, error) {
	return []iagent.AgentSummary{
		{AgentRef: "fakedisc:a1", Name: "Agent One", Description: "第一个"},
		{AgentRef: "fakedisc:a2", Name: "Agent Two"},
	}, nil
}

// registerFakeDisc registers the fakedisc scheme. Like fakepause in
// send_test.go this leaks into the package-level registry for the remaining
// tests of this package run — so no test in this package may assert an exact
// provider set or provider count.
func registerFakeDisc() {
	iagent.Register("fakedisc", iagent.ProviderInfo{
		Factory:        func(deps iagent.Deps, agentID string) (iagent.Provider, error) { return &fakeDiscProvider{}, nil },
		Label:          "test fake (discoverer)",
		AgentRefFormat: "fakedisc:<agent_id>",
		AgentIDSource:  "test only",
		Kind:           iagent.KindCatalog,
		Identities:     []iagent.IdentitySpec{{Type: iagent.IdentityUser}},
	})
}

// TestAgentListScheme_DiscovererListsAgents pins the positive path: a
// provider implementing Discoverer yields {agents:[AgentSummary...]} plus
// meta.count.
func TestAgentListScheme_DiscovererListsAgents(t *testing.T) {
	registerFakeDisc()
	cfg := &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu}
	f, _, _, _ := cmdutil.TestFactory(t, cfg)
	cmd := &cobra.Command{Use: "list"}
	cmd.SetContext(context.Background())
	opts := &listOptions{Factory: f, Cmd: cmd, Format: "json", Scheme: "fakedisc"}
	out := f.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentListRun(opts); err != nil {
		t.Fatalf("list fakedisc should not error: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("output should be valid envelope JSON: %v (%s)", err, string(out.Bytes()))
	}
	data, _ := env.Data.(map[string]interface{})
	agents, ok := data["agents"].([]interface{})
	if !ok || len(agents) != 2 {
		t.Fatalf("data.agents should have 2 entries, got %v", data["agents"])
	}
	first, _ := agents[0].(map[string]interface{})
	if first["agent_ref"] != "fakedisc:a1" || first["name"] != "Agent One" {
		t.Errorf("agents[0] should be an AgentSummary {agent_ref, name}, got %v", first)
	}
	if env.Meta == nil || env.Meta.Count != 2 {
		t.Errorf("meta.count should be 2, got %+v", env.Meta)
	}
}

// TestAgentListScheme_PropagatesIdentity pins the Task 10 review item: the
// provider rebuilt for the real ListAgents call must carry the resolved
// identity in its Deps (aligned with resolveProvider), not a zero As.
func TestAgentListScheme_PropagatesIdentity(t *testing.T) {
	var captured iagent.Deps
	iagent.Register("fakedeps", iagent.ProviderInfo{
		Factory: func(deps iagent.Deps, agentID string) (iagent.Provider, error) {
			captured = deps
			return &fakeDiscProvider{}, nil
		},
		Label:          "test fake (deps capture)",
		AgentRefFormat: "fakedeps:<agent_id>",
		AgentIDSource:  "test only",
		Kind:           iagent.KindCatalog,
		Identities:     []iagent.IdentitySpec{{Type: iagent.IdentityUser}},
	})

	cfg := &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu}
	f, _, _, _ := cmdutil.TestFactory(t, cfg)
	cmd := &cobra.Command{Use: "list"}
	cmd.SetContext(context.Background())
	opts := &listOptions{Factory: f, Cmd: cmd, Format: "json", Scheme: "fakedeps"}

	if err := agentListRun(opts); err != nil {
		t.Fatalf("list fakedeps should not error: %v", err)
	}
	if captured.As == "" {
		t.Error("the rebuilt provider's Deps.As should carry the resolved identity, got empty")
	}
	if captured.As != f.ResolvedIdentity {
		t.Errorf("Deps.As should match the Factory's resolved identity, got %q vs %q", captured.As, f.ResolvedIdentity)
	}
}

// dirtyNameProvider is a Discoverer whose agent names carry ANSI escapes, to
// pin the pretty-path sanitization of agent-controlled fields.
type dirtyNameProvider struct{ iagent.Provider }

func (p *dirtyNameProvider) ListAgents(ctx context.Context) ([]iagent.AgentSummary, error) {
	return []iagent.AgentSummary{
		{AgentRef: "fakedirty:a1", Name: "\x1b[31mEvil\x1b[0m One", Description: "d\x1b[2Jesc"},
	}, nil
}

// TestAgentListScheme_PrettyStripsANSI pins the Task 10 review item: `agent list
// <scheme> --format pretty` must strip ANSI escapes from the agent-controlled
// Name/Description before they reach the terminal.
func TestAgentListScheme_PrettyStripsANSI(t *testing.T) {
	iagent.Register("fakedirty", iagent.ProviderInfo{
		Factory:        func(deps iagent.Deps, agentID string) (iagent.Provider, error) { return &dirtyNameProvider{}, nil },
		Label:          "test fake (dirty names)",
		AgentRefFormat: "fakedirty:<agent_id>",
		AgentIDSource:  "test only",
		Kind:           iagent.KindCatalog,
		Identities:     []iagent.IdentitySpec{{Type: iagent.IdentityUser}},
	})

	cfg := &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu}
	f, _, _, _ := cmdutil.TestFactory(t, cfg)
	cmd := &cobra.Command{Use: "list"}
	cmd.SetContext(context.Background())
	opts := &listOptions{Factory: f, Cmd: cmd, Format: "pretty", Scheme: "fakedirty"}
	out := f.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentListRun(opts); err != nil {
		t.Fatalf("list fakedirty pretty should not error: %v", err)
	}
	text := string(out.Bytes())
	if strings.Contains(text, "\x1b") {
		t.Errorf("ANSI sequences in agent Name/Description must be stripped: %q", text)
	}
	if !strings.Contains(text, "Evil One") || !strings.Contains(text, "desc") {
		t.Errorf("readable text should remain after stripping, got %q", text)
	}
}

// TestAgentListJqFlagRegisteredAndConsumed pins the quality-review fix: the
// --jq flag must be registered on `agent list` and filter the envelope.
func TestAgentListJqFlagRegisteredAndConsumed(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{Out: out, ErrOut: errOut}}
	cmd := NewCmdAgentList(f)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"--jq", ".ok"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent list --jq should not error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "true" {
		t.Errorf("--jq .ok should output only true, got %q", got)
	}
}

// TestNewCmdAgentList_ReadRisk pins the read risk annotation, the json default
// of --format, the --jq flag presence, and that list takes at most one
// positional arg (the scheme).
func TestNewCmdAgentList_ReadRisk(t *testing.T) {
	cmd := NewCmdAgentList(nil)
	if level, ok := cmdutil.GetRisk(cmd); !ok || level != cmdutil.RiskRead {
		t.Errorf("agent list should be marked read risk, got level=%q ok=%v", level, ok)
	}
	fl := cmd.Flags().Lookup("format")
	if fl == nil {
		t.Fatal("agent list should have a --format flag")
	}
	if fl.DefValue != "json" {
		t.Errorf("--format default should flip to json, got %q", fl.DefValue)
	}
	if cmd.Flags().Lookup("jq") == nil {
		t.Error("agent list should have a --jq flag")
	}
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("agent list with no args should be valid: %v", err)
	}
	if err := cmd.Args(cmd, []string{"example"}); err != nil {
		t.Errorf("agent list <scheme> should be valid: %v", err)
	}
	if err := cmd.Args(cmd, []string{"example", "extra"}); err == nil {
		t.Error("agent list with more than 1 positional argument should error (MaximumNArgs 1)")
	}
}

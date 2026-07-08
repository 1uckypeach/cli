// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
)

// cardTestOpts builds a cardOptions driving agentCardRun against a real
// (test) Factory. The example card is synthesized statically, so no API call
// is made and stdout carries the capability card envelope.
func cardTestOpts(t *testing.T, ref string) (*cardOptions, *core.CliConfig) {
	t.Helper()
	cfg := &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu}
	f, _, _, _ := cmdutil.TestFactory(t, cfg)
	cmd := resolveCmd(t, true, "bot") // reuses the common_test.go helper (--as=bot)
	return &cardOptions{Factory: f, Cmd: cmd, Ref: ref, As: "bot", Format: "json"}, cfg
}

// TestAgentCardRun_ExampleStaticCard verifies that `agent card example:echo`
// returns the statically synthesized capability card (no API), with
// task_cancel gated off and multi_turn on, and the agent_id echoed from the
// ref.
func TestAgentCardRun_ExampleStaticCard(t *testing.T) {
	opts, _ := cardTestOpts(t, "example:echo")
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentCardRun(opts); err != nil {
		t.Fatalf("card should be statically synthesized and not error: %v", err)
	}

	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("output should be valid envelope JSON: %v", err)
	}
	if !env.OK {
		t.Errorf("ok should be true: %+v", env)
	}
	data, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data should be a card object, got %T", env.Data)
	}
	if data["agent_id"] != "echo" {
		t.Errorf("agent_id should echo the ref, got %v", data["agent_id"])
	}
	if data["provider"] != "example" {
		t.Errorf("provider should be example, got %v", data["provider"])
	}
	// source was removed from the card (schema tightening).
	if _, present := data["source"]; present {
		t.Errorf("card should no longer carry a source field, got %v", data["source"])
	}
	caps, ok := data["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("capabilities should be an object, got %T", data["capabilities"])
	}
	if caps["task_cancel"] != false {
		t.Errorf("echo task_cancel should be false, got %v", caps["task_cancel"])
	}
	if caps["multi_turn"] != true {
		t.Errorf("echo multi_turn should be true, got %v", caps["multi_turn"])
	}
	// parameters / identity must serialize as non-null (guard against omitempty
	// regression): parameters is always an array (empty [] for example),
	// identity is a non-empty array.
	if params, ok := data["parameters"].([]interface{}); !ok {
		t.Errorf("parameters should be a non-null array, got %T (%v)", data["parameters"], data["parameters"])
	} else if len(params) != 0 {
		t.Errorf("example parameters should be an empty array, got %v", params)
	}
	if ids, ok := data["identity"].([]interface{}); !ok || len(ids) == 0 {
		t.Errorf("identity should be a non-null non-empty array, got %T (%v)", data["identity"], data["identity"])
	}
	// card no longer exposes scope: the required_scopes field was removed from
	// AgentCard (scope is an internal registration item used only for preflight).
	if _, present := data["required_scopes"]; present {
		t.Errorf("card should no longer carry a required_scopes field, got %v", data["required_scopes"])
	}
}

// TestAgentCardRun_PrettyFormat verifies that with --format pretty (opt-in
// since the json default flip), the card renders as a human-readable listing.
// The output must surface the identity and capability names in plain text so
// the stream is not valid envelope JSON.
func TestAgentCardRun_PrettyFormat(t *testing.T) {
	opts, _ := cardTestOpts(t, "example:echo")
	opts.Format = "pretty"
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentCardRun(opts); err != nil {
		t.Fatalf("card pretty should not error: %v", err)
	}

	text := string(out.Bytes())
	// A pretty rendering is human text, not a JSON envelope.
	var env output.Envelope
	if json.Unmarshal(out.Bytes(), &env) == nil && env.OK {
		t.Fatalf("pretty format should not output a JSON envelope: %s", text)
	}
	if !strings.Contains(text, "echo") {
		t.Errorf("pretty output should contain agent_id: %s", text)
	}
	// multi_turn is a declared capability of the echo card; it must appear.
	if !strings.Contains(text, "multi_turn") {
		t.Errorf("pretty output should list capabilities: %s", text)
	}
}

// TestAgentCardRun_JSONFormat pins that --format json still emits the envelope.
func TestAgentCardRun_JSONFormat(t *testing.T) {
	opts, _ := cardTestOpts(t, "example:echo")
	opts.Format = "json"
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })

	if err := agentCardRun(opts); err != nil {
		t.Fatalf("card json should not error: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("json format should be a valid envelope: %v (%s)", err, string(out.Bytes()))
	}
	if !env.OK {
		t.Errorf("ok should be true: %+v", env)
	}
}

// TestAgentCardJqFlagRegisteredAndConsumed pins the quality-review fix: the
// --jq flag must actually be REGISTERED on `agent card` (the run path already
// called jqExpr/JqFilter, but without the flag `--jq` was an unknown-flag
// exit 2 — and the skill doc teaches AI to copy `card ... --jq`). Executed via
// the real command so registration + consumption are proven together.
func TestAgentCardJqFlagRegisteredAndConsumed(t *testing.T) {
	cfg := &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu}
	f, _, _, _ := cmdutil.TestFactory(t, cfg)
	cmd := NewCmdAgentCard(f)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"example:echo", "--as", "bot", "--jq", ".data.agent_id"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("card --jq should not error: %v", err)
	}
	out := f.IOStreams.Out.(interface{ Bytes() []byte })
	got := strings.TrimSpace(string(out.Bytes()))
	if !strings.Contains(got, "echo") || strings.Contains(got, `"ok"`) {
		t.Errorf("--jq .data.agent_id should output only the filtered result, got %q", got)
	}
}

// TestPrintCardPretty_NilCard pins that a nil card degrades to a placeholder
// line instead of panicking (card.go nil branch).
func TestPrintCardPretty_NilCard(t *testing.T) {
	out := &bytes.Buffer{}
	printCardPretty(out, nil)
	if !strings.Contains(out.String(), "(no card)") {
		t.Errorf("nil card should print a placeholder line, got: %q", out.String())
	}
}

// TestPrintCardPretty_AllOptionalFields exercises every optional-field branch of
// the pretty renderer that a minimal static card omits: the dynamic-card Name
// (taking precedence over ProviderLabel), Description, declared Parameters, and
// the Skills block (both the named skill and the id-fallback when Name is empty).
func TestPrintCardPretty_AllOptionalFields(t *testing.T) {
	card := &iagent.AgentCard{
		Provider:      "demo",
		ProviderLabel: "demo 自定义智能体",
		Name:          "Demo Agent", // only dynamic cards have Name; it should override ProviderLabel
		AgentID:       "agt_demo",
		Description:   "a helpful demo agent",
		Identity: []iagent.IdentitySpec{
			{Type: "user"},
			{Type: "bot", Precondition: "需加入渠道白名单"},
		},
		Capabilities: iagent.Capabilities{
			MultiTurn:  true,
			TaskCancel: false,
		},
		Parameters: []iagent.CardParam{
			{Name: "locale", Type: "string", Required: true, Desc: "reply locale"},
		},
		Skills: []iagent.CardSkill{
			{ID: "sk_1", Name: "Sales Analysis"},
			{ID: "sk_2"}, // no Name → falls back to ID
		},
	}
	out := &bytes.Buffer{}
	printCardPretty(out, card)
	text := out.String()

	for _, want := range []string{
		"Demo Agent (agt_demo)", // dynamic Name takes precedence over ProviderLabel
		"a helpful demo agent",  // Description branch
		"identity: user, bot",   // IdentitySpec types are joined
		"需加入渠道白名单",              // identity precondition must be visible in pretty (Task 11 wrap-up)
		"locale",                // Parameters branch
		"skills:",               // Skills block header
		"Sales Analysis",        // skill with a Name
		"sk_2",                  // skill without a Name → id fallback
	} {
		if !strings.Contains(text, want) {
			t.Errorf("pretty output should contain %q, got:\n%s", want, text)
		}
	}
}

// TestPrintCardPretty_StripsANSIFromRemoteFields pins that a remote card's
// agent-controlled Name/Description cannot smuggle ANSI escapes to the
// terminal (this sanitization is applied to every pretty surface).
func TestPrintCardPretty_StripsANSIFromRemoteFields(t *testing.T) {
	card := &iagent.AgentCard{
		Provider:    "demo",
		AgentID:     "agt_demo",
		Name:        "\x1b[31mEvil\x1b[0m Agent",
		Description: "desc\x1b[2Jwipe",
	}
	out := &bytes.Buffer{}
	printCardPretty(out, card)
	text := out.String()
	if strings.Contains(text, "\x1b") {
		t.Errorf("ANSI sequences in remote card fields must be stripped: %q", text)
	}
	if !strings.Contains(text, "Evil Agent") || !strings.Contains(text, "descwipe") {
		t.Errorf("readable text should remain after stripping, got: %q", text)
	}
}

// TestPrintCardPretty_StaticFallsBackToProviderLabel pins that a static card
// (no dynamic Name) renders its ProviderLabel as the header.
func TestPrintCardPretty_StaticFallsBackToProviderLabel(t *testing.T) {
	card := &iagent.AgentCard{
		Provider:      "demo",
		ProviderLabel: "demo 自定义智能体",
		AgentID:       "agt_demo",
	}
	out := &bytes.Buffer{}
	printCardPretty(out, card)
	if !strings.Contains(out.String(), "demo 自定义智能体 (agt_demo)") {
		t.Errorf("should fall back to ProviderLabel when Name is empty, got:\n%s", out.String())
	}
}

// TestAgentCardRun_InvalidRef surfaces a malformed ref as a validation error
// before any provider is built.
func TestAgentCardRun_InvalidRef(t *testing.T) {
	opts, _ := cardTestOpts(t, "no-colon")
	if err := agentCardRun(opts); err == nil {
		t.Fatal("malformed ref should error")
	}
}

// TestNewCmdAgentCard_ReadRiskAndArgs pins ExactArgs(1), read risk, and the
// presence of --format and --as flags.
func TestNewCmdAgentCard_ReadRiskAndArgs(t *testing.T) {
	cmd := NewCmdAgentCard(nil)
	if level, ok := cmdutil.GetRisk(cmd); !ok || level != cmdutil.RiskRead {
		t.Errorf("agent card should be marked read risk, got level=%q ok=%v", level, ok)
	}
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("agent card missing ref should report an argument error (ExactArgs 1)")
	}
	if err := cmd.Args(cmd, []string{"example:x"}); err != nil {
		t.Errorf("agent card with a single ref should be valid: %v", err)
	}
	fl := cmd.Flags().Lookup("format")
	if fl == nil {
		t.Fatal("agent card should have a --format flag")
	}
	// Default output format is unified: card default flips from pretty to json.
	if fl.DefValue != "json" {
		t.Errorf("card --format default should flip to json, got %q", fl.DefValue)
	}
	if cmd.Flags().Lookup("as") == nil {
		t.Error("agent card should have an --as flag")
	}
}

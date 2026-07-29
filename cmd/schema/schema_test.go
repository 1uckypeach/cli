// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package schema

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/registry"
)

func TestSchemaCmd_FlagParsing(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, nil)

	var gotOpts *SchemaOptions
	cmd := NewCmdSchema(f, func(opts *SchemaOptions) error {
		gotOpts = opts
		return nil
	})
	cmd.SetArgs([]string{"calendar.events.list"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotOpts.Args) != 1 || gotOpts.Args[0] != "calendar.events.list" {
		t.Errorf("expected args [calendar.events.list], got %v", gotOpts.Args)
	}
}

func TestSchemaCmd_OutputFlagsAcceptedForCompat(t *testing.T) {
	// Agents are habituated to --format/--json/--as from api/service commands.
	// schema must accept them without erroring and always emit the JSON envelope —
	// its output is structured JSON and identity-independent, so the values have
	// no effect.
	argSets := [][]string{
		{"--format", "json"},
		{"--format", "pretty"},
		{"--format", "table"}, // no table rendering for a nested schema -> JSON
		{"--format", "csv"},
		{"--json"},
		{"--json", "--format", "ndjson"},
		{"--as", "user"},
		{"--as", "bot"},
		{"--as", "user", "--json"},
	}
	for _, extra := range argSets {
		f, stdout, _, _ := cmdutil.TestFactory(t, nil)
		cmd := NewCmdSchema(f, nil)
		cmd.SetArgs(append([]string{"im.images.create"}, extra...))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("args %v should be accepted, got error: %v", extra, err)
		}
		var env map[string]interface{}
		if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
			t.Fatalf("args %v: output is not a JSON envelope: %v\n%s", extra, err, stdout.String())
		}
		if env["name"] != "im images create" {
			t.Errorf("args %v: expected the im images create envelope, got name=%v", extra, env["name"])
		}
	}
}

func TestSchemaCmd_NoArgs_RendersServiceIndex(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, nil)

	cmd := NewCmdSchema(f, nil)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := strings.TrimSpace(stdout.String())
	// The bare form renders a service index object, not the former array of
	// every method's full envelope — that shape exceeded any practical
	// single-response budget.
	if !strings.HasPrefix(out, "{") {
		head := out
		if len(head) > 80 {
			head = head[:80]
		}
		t.Errorf("expected JSON object root, first 80 chars:\n%s", head)
	}
	var idx struct {
		Kind     string `json:"kind"`
		Services []struct {
			Name string `json:"name"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(out), &idx); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if idx.Kind != "service_index" {
		t.Errorf("kind = %q, want service_index", idx.Kind)
	}
	// Every service the catalog knows must be listed — this index is the entry
	// point for discovering them, so a missing one is unreachable.
	if want := len(registry.SchemaCatalog().Services()); len(idx.Services) != want {
		t.Errorf("services count = %d, want %d", len(idx.Services), want)
	}
}

func TestSchemaCmd_JSONIsEnvelope(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, nil)

	cmd := NewCmdSchema(f, nil)
	cmd.SetArgs([]string{"im.images.create"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var env map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, stdout.String())
	}
	if env["name"] != "im images create" {
		t.Errorf("name = %v, want \"im images create\"", env["name"])
	}
	for _, key := range []string{"description", "inputSchema", "outputSchema", "_meta"} {
		if _, ok := env[key]; !ok {
			t.Errorf("missing top-level key: %s", key)
		}
	}
	meta, _ := env["_meta"].(map[string]interface{})
	if meta["envelope_version"] != "1.0" {
		t.Errorf("envelope_version = %v, want \"1.0\"", meta["envelope_version"])
	}
}

func TestSchemaCmd_SpaceSeparatedPath_EqualsDotted(t *testing.T) {
	f1, out1, _, _ := cmdutil.TestFactory(t, nil)
	cmd1 := NewCmdSchema(f1, nil)
	cmd1.SetArgs([]string{"im", "images", "create"})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("space form failed: %v", err)
	}

	f2, out2, _, _ := cmdutil.TestFactory(t, nil)
	cmd2 := NewCmdSchema(f2, nil)
	cmd2.SetArgs([]string{"im.images.create"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("dotted form failed: %v", err)
	}

	if out1.String() != out2.String() {
		t.Errorf("space and dotted forms produced different output")
	}
}

func TestSchemaCmd_ServiceRendersMethodIndex(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, nil)

	cmd := NewCmdSchema(f, nil)
	cmd.SetArgs([]string{"im"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var idx struct {
		Kind    string `json:"kind"`
		Service string `json:"service"`
		Methods []struct {
			Path string `json:"path"`
		} `json:"methods"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &idx); err != nil {
		t.Fatalf("unmarshal failed: %v\n%s", err, stdout.String())
	}
	if idx.Kind != "method_index" || idx.Service != "im" {
		t.Errorf("kind/service = %q/%q, want method_index/im", idx.Kind, idx.Service)
	}
	if len(idx.Methods) == 0 {
		t.Fatal("expected non-empty method index for service im")
	}
	// Scoping to one service must not leak another service's methods.
	for _, m := range idx.Methods {
		if !strings.HasPrefix(m.Path, "im.") {
			t.Errorf("method path %q does not start with \"im.\"", m.Path)
		}
	}
}

func TestSchemaCmd_HighRiskYesInjection(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, nil)

	cmd := NewCmdSchema(f, nil)
	cmd.SetArgs([]string{"im.messages.delete"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var env map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	is, _ := env["inputSchema"].(map[string]interface{})
	props, _ := is["properties"].(map[string]interface{})
	if _, ok := props["yes"]; !ok {
		t.Errorf("inputSchema.properties.yes missing for high-risk-write command")
	}
}

func TestSchemaCmd_NoYesForReadRisk(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, nil)

	cmd := NewCmdSchema(f, nil)
	cmd.SetArgs([]string{"im.reactions.list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var env map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	is, _ := env["inputSchema"].(map[string]interface{})
	props, _ := is["properties"].(map[string]interface{})
	if _, ok := props["yes"]; ok {
		t.Errorf("yes property should not appear for risk=read command")
	}
}

func TestSchemaCmd_UnknownService(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{
		AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu,
	})

	cmd := NewCmdSchema(f, nil)
	cmd.SetArgs([]string{"nonexistent_service"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown service")
	}
	if !strings.Contains(err.Error(), "Unknown service") {
		t.Errorf("expected 'Unknown service' error, got: %v", err)
	}
	var ve *errs.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *errs.ValidationError, got %T: %v", err, err)
	}
	if ve.Subtype != errs.SubtypeInvalidArgument {
		t.Errorf("Subtype = %q, want %q", ve.Subtype, errs.SubtypeInvalidArgument)
	}
	if !strings.Contains(ve.Hint, "Available:") {
		t.Errorf("expected hint listing available services, got: %q", ve.Hint)
	}
}

// TestSchemaCmd_UnknownMethod_TypedValidation pins the typed envelope for the
// JSON-mode unknown-method path: *errs.ValidationError with
// subtype invalid_argument and a hint listing the available methods.
func TestSchemaCmd_UnknownMethod_TypedValidation(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, &core.CliConfig{
		AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu,
	})

	cmd := NewCmdSchema(f, nil)
	cmd.SetArgs([]string{"calendar.events.nonexistent_method"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
	var ve *errs.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *errs.ValidationError, got %T: %v", err, err)
	}
	if ve.Subtype != errs.SubtypeInvalidArgument {
		t.Errorf("Subtype = %q, want %q", ve.Subtype, errs.SubtypeInvalidArgument)
	}
	if !strings.Contains(err.Error(), "Unknown method") {
		t.Errorf("expected 'Unknown method' error, got: %v", err)
	}
	if !strings.Contains(ve.Hint, "Available:") {
		t.Errorf("expected hint listing available methods, got: %q", ve.Hint)
	}
}

// Completion candidate generation (dotted + space forms, strict-mode filtering,
// dotted-resource handling) now lives in internal/apicatalog and is covered by
// apicatalog's TestComplete. cmd/schema only adapts catalog.Complete to cobra.

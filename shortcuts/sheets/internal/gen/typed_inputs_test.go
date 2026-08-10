// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadTypedInputTestDefs(t *testing.T) map[string]commandDef {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(sheetsDir(), "data", "flag-defs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var defs map[string]commandDef
	if err := json.Unmarshal(raw, &defs); err != nil {
		t.Fatal(err)
	}
	return defs
}

func TestTypedInputsGeneratedFileIsCurrent(t *testing.T) {
	want, err := renderTypedInputs(loadTypedInputTestDefs(t), typedInputConfigs)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(sheetsDir(), "typed_inputs_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("typed_inputs_gen.go differs from data/flag-defs.json and the opt-in config; run go generate ./shortcuts/sheets/...")
	}
}

func TestTypedInputsGenerateOnlyReviewedCommands(t *testing.T) {
	out, err := renderTypedInputs(loadTypedInputTestDefs(t), typedInputConfigs)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "type CSVPutGeneratedInput struct") {
		t.Fatal("generated output is missing the opted-in +csv-put fragment")
	}
	if strings.Contains(text, "type CellsSetGeneratedInput struct") {
		t.Fatal("generated output contains non-opted-in +cells-set fragment")
	}
	if strings.Contains(text, `flag:\"dry-run\"`) {
		t.Fatal("generated output contains a framework-owned system flag")
	}
	for _, fact := range []string{
		`schema:\"optional;default=\\\"A1\\\"\"`,
		`cli:\"sources=flag|file|stdin\"`,
		`common.Provided[bool]`,
	} {
		if !strings.Contains(text, fact) {
			t.Fatalf("generated output is missing %q", fact)
		}
	}
}

func TestTypedInputsRejectUnreviewedRequiredDefault(t *testing.T) {
	_, err := renderTypedInputs(loadTypedInputTestDefs(t), map[string]typedInputConfig{"+csv-put": {}})
	if err == nil || !strings.Contains(err.Error(), "+csv-put --start-cell is required and defaulted") {
		t.Fatalf("error = %v", err)
	}
}

func TestTypedInputsRejectStaleCompatibilityOverride(t *testing.T) {
	defs := loadTypedInputTestDefs(t)
	definition := defs["+csv-put"]
	definition.Flags = append([]flagDef(nil), definition.Flags...)
	for i := range definition.Flags {
		if definition.Flags[i].Name == "start-cell" {
			definition.Flags[i].Required = "optional"
		}
	}
	defs["+csv-put"] = definition
	_, err := renderTypedInputs(defs, typedInputConfigs)
	if err == nil || !strings.Contains(err.Error(), `required override expected "required", got "optional"`) {
		t.Fatalf("error = %v", err)
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package sheets

import (
	"reflect"
	"testing"

	"github.com/larksuite/cli/shortcuts/common"
)

// TestFlagDefs_EmbedParses asserts the embedded flag-defs.json blob is valid
// JSON with at least one command entry.
func TestFlagDefs_EmbedParses(t *testing.T) {
	t.Parallel()
	defs, err := loadFlagDefs()
	if err != nil {
		t.Fatalf("loadFlagDefs error: %v", err)
	}
	if len(defs) == 0 {
		t.Fatal("flag-defs.json has no command entries")
	}
}

// TestFlagsFor_SkipsSystemFlags verifies system-kind flags (--dry-run, --yes)
// are never materialized into a shortcut's Flags slice — the framework injects
// those based on Risk / DryRun.
func TestFlagsFor_SkipsSystemFlags(t *testing.T) {
	t.Parallel()
	for _, cmd := range []string{"+sheet-delete", "+batch-update", "+csv-get"} {
		for _, f := range flagsFor(cmd) {
			if f.Name == "dry-run" || f.Name == "yes" {
				t.Errorf("%s: system flag --%s leaked into Flags", cmd, f.Name)
			}
		}
	}
}

// TestFlagsFor_MapsAllFields spot-checks that name/type/default/enum/input/
// required/hidden are carried over from the JSON correctly.
func TestFlagsFor_MapsAllFields(t *testing.T) {
	t.Parallel()
	byName := func(cmd, name string) *common.Flag {
		flags := flagsFor(cmd)
		for i := range flags {
			if flags[i].Name == name {
				return &flags[i]
			}
		}
		return nil
	}

	// enum + default
	rt := byName("+dim-insert", "inherit-style")
	if rt == nil || len(rt.Enum) != 2 || rt.Default != "" {
		t.Errorf("+dim-insert --inherit-style not mapped: %+v", rt)
	}
	// required
	title := byName("+sheet-create", "title")
	if title == nil || !title.Required {
		t.Errorf("+sheet-create --title should be required: %+v", title)
	}
	// xor is NOT cobra-required (enforced by Validate hooks)
	url := byName("+sheet-create", "url")
	if url == nil || url.Required {
		t.Errorf("+sheet-create --url should not be cobra-required: %+v", url)
	}
	// visible + int default
	cap := byName("+cells-get", "max-chars")
	if cap == nil || cap.Hidden || cap.Default != "500000" {
		t.Errorf("+cells-get --max-chars not mapped: %+v", cap)
	}
	// input sources
	cells := byName("+cells-set", "cells")
	if cells == nil || len(cells.Input) != 2 {
		t.Errorf("+cells-set --cells should support file+stdin: %+v", cells)
	}
	// float64 type
	fs := byName("+cells-set-style", "font-size")
	if fs == nil || fs.Type != "float64" {
		t.Errorf("+cells-set-style --font-size should be float64: %+v", fs)
	}
}

// TestFlagsFor_EveryRegisteredCommandHasDefs ensures every shortcut returned by
// Shortcuts() has a flag-defs.json entry and that its flags match the JSON's
// non-system flags exactly (name + type + required + default + hidden). This is
// the contract that lets shortcuts drop hand-written flag literals.
func TestFlagsFor_EveryRegisteredCommandHasDefs(t *testing.T) {
	t.Parallel()
	defs, err := loadFlagDefs()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range Shortcuts() {
		spec, ok := defs[s.Command]
		if !ok {
			t.Errorf("%s has no flag-defs.json entry", s.Command)
			continue
		}
		want := map[string]flagDef{}
		for _, df := range spec.Flags {
			if df.Kind != "system" {
				want[df.Name] = df
			}
		}
		got := map[string]bool{}
		for _, f := range s.Flags {
			got[f.Name] = true
			df, ok := want[f.Name]
			if !ok {
				t.Errorf("%s --%s present in Go but not in JSON (non-system)", s.Command, f.Name)
				continue
			}
			ft := f.Type
			if ft == "" {
				ft = "string"
			}
			jt := df.Type
			if jt == "" {
				jt = "string"
			}
			if ft != jt {
				t.Errorf("%s --%s type: go=%s json=%s", s.Command, f.Name, ft, jt)
			}
			// +csv-put accepts the hidden --range compatibility anchor, so its
			// Typed Definition models start-cell/range as an exactly-one relation
			// instead of marking start-cell individually required. The mounted
			// contract remains pinned by csv_put_legacy_contract_test.
			isCSVPutAnchor := s.Command == "+csv-put" && f.Name == "start-cell"
			if !isCSVPutAnchor && f.Required != (df.Required == "required") {
				t.Errorf("%s --%s required: go=%v json=%s", s.Command, f.Name, f.Required, df.Required)
			}
			if f.Default != df.Default {
				t.Errorf("%s --%s default: go=%q json=%q", s.Command, f.Name, f.Default, df.Default)
			}
			if f.Hidden != df.Hidden {
				t.Errorf("%s --%s hidden: go=%v json=%v", s.Command, f.Name, f.Hidden, df.Hidden)
			}
			if s.Command == "+csv-put" && f.Desc != df.Desc {
				t.Errorf("%s --%s description differs from canonical flag-defs.json", s.Command, f.Name)
			}
			if !reflect.DeepEqual(f.Enum, df.Enum) {
				t.Errorf("%s --%s enum: go=%v json=%v", s.Command, f.Name, f.Enum, df.Enum)
			}
			if !reflect.DeepEqual(f.Input, df.Input) {
				t.Errorf("%s --%s input sources: go=%v json=%v", s.Command, f.Name, f.Input, df.Input)
			}
		}
		for name := range want {
			if !got[name] {
				t.Errorf("%s --%s in JSON but missing from Go Flags", s.Command, name)
			}
		}
	}
}

// TestCSVPutUsesGeneratedTypedInputFragment pins the production consumption
// side of the opt-in generator, not only the existence of generated source.
func TestCSVPutUsesGeneratedTypedInputFragment(t *testing.T) {
	t.Parallel()
	argsType := reflect.TypeFor[csvPutArgs]()
	field, ok := argsType.FieldByName("CSVPutGeneratedInput")
	if !ok || !field.Anonymous || field.Type != reflect.TypeFor[CSVPutGeneratedInput]() || field.Tag.Get("arg") != "inline" {
		t.Fatalf("csvPutArgs generated inline field = %#v, found = %v", field, ok)
	}
	for _, name := range []string{"URL", "SpreadsheetToken", "SheetID", "SheetName", "StartCell", "CSV", "AllowOverwrite", "Range"} {
		promoted, ok := argsType.FieldByName(name)
		if !ok || len(promoted.Index) != 2 || promoted.Index[0] != field.Index[0] {
			t.Errorf("generated field %s is not promoted through CSVPutGeneratedInput: %#v", name, promoted)
		}
	}
}

// TestFlagAcceptsStdin verifies the stdin-capability probe that decides whether
// an "invalid JSON" error should also steer the caller toward stdin: a composite
// flag (cells) accepts stdin, a plain locator (spreadsheet-token) does not, and
// an unknown command/flag returns false without panicking (it runs on an error
// path, unlike flagsFor).
func TestFlagAcceptsStdin(t *testing.T) {
	t.Parallel()
	if !flagAcceptsStdin("+cells-set", "cells") {
		t.Error("+cells-set --cells should accept stdin")
	}
	if flagAcceptsStdin("+cells-set", "spreadsheet-token") {
		t.Error("--spreadsheet-token should not accept stdin")
	}
	if flagAcceptsStdin("+nope", "cells") {
		t.Error("unknown command should be false (and must not panic)")
	}
	if flagAcceptsStdin("+cells-set", "nope") {
		t.Error("unknown flag should be false")
	}
}

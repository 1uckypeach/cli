// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package core_test

import (
	"os"
	"regexp"
	"sort"
	"testing"

	"github.com/larksuite/cli/extension/credential"
	"github.com/larksuite/cli/internal/core"
)

// The brand rule is implemented twice, once per Brand type, because
// extension/credential ships as a standalone SDK and may not import internal.
// Cross-reference comments ask the next author to change both; this file makes
// the request enforceable. It lives here rather than beside either parser so
// neither side owns the contract.
//
// The constants are read out of the source instead of listed again, so adding a
// brand to one package is enough to put it under test.
var brandConstant = regexp.MustCompile(`(?m)^\s+Brand[A-Za-z0-9]*\s+(?:Brand|LarkBrand)\s+=\s+"([^"]+)"`)

func brandValues(t *testing.T, path string) []string {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	matches := brandConstant.FindAllStringSubmatch(string(source), -1)
	if len(matches) == 0 {
		t.Fatalf("no brand constants found in %s; the pattern needs updating", path)
	}

	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, match[1])
	}
	sort.Strings(values)
	return values
}

// TestBrandConstantsMatchAcrossPackages fails when one package learns a brand
// the other has not.
func TestBrandConstantsMatchAcrossPackages(t *testing.T) {
	sdk := brandValues(t, "../../extension/credential/types.go")
	internal := brandValues(t, "types.go")

	if len(sdk) != len(internal) {
		t.Fatalf("brand constants differ: extension/credential has %v, internal/core has %v", sdk, internal)
	}
	for i := range sdk {
		if sdk[i] != internal[i] {
			t.Fatalf("brand constants differ: extension/credential has %v, internal/core has %v", sdk, internal)
		}
	}
}

// TestParseBrandAgreesAcrossPackages fails when the two parsers resolve the same
// input differently — the drift the cross-reference comments only ask for.
func TestParseBrandAgreesAcrossPackages(t *testing.T) {
	inputs := brandValues(t, "../../extension/credential/types.go")
	inputs = append(inputs,
		"",           // unset
		"LARK",       // case
		"  lark  ",   // padding
		"Feishu",     // case on the default
		"neo",        // a brand neither package knows yet
		"lark-suite", // near miss
	)

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			fromCore := string(core.ParseBrand(input))
			fromSDK := string(credential.ParseBrand(input))
			if fromCore != fromSDK {
				t.Fatalf("ParseBrand(%q): internal/core = %q, extension/credential = %q", input, fromCore, fromSDK)
			}
		})
	}
}

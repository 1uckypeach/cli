// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package affordance

import (
	"os"
	"testing"

	"github.com/larksuite/cli/internal/meta"
)

func TestSlidesUpdateAffordancePreservesSafetyAndShellGuidance(t *testing.T) {
	prev := mdSource
	t.Cleanup(func() { SetSource(prev) })
	SetSource(os.DirFS("../../affordance"))

	raw, ok := For("slides", "+update-slide")
	if !ok {
		t.Fatal("For(slides, +update-slide) ok=false")
	}
	a, ok := (meta.Method{Affordance: raw}).ParsedAffordance()
	if !ok {
		t.Fatal("slides +update-slide affordance did not parse")
	}

	for _, want := range []string{
		"Read the page first",
		"Anything left out",
		"Editing one element",
		`--content - < "$file"`,
	} {
		if !containsItem(a.Tips, want) {
			t.Errorf("tips must contain %q: %v", want, a.Tips)
		}
	}
}

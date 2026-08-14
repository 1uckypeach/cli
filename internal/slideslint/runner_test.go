// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slideslint

import (
	"context"
	"testing"
)

const overlapSlide = `<slide id="p"><data>` +
	`<shape type="text" topLeftX="80" topLeftY="80" width="300" height="40"><content textType="title"><p>very long title that overflows</p></content></shape>` +
	`<shape type="text" topLeftX="90" topLeftY="85" width="300" height="40"><content textType="body"><p>B</p></content></shape>` +
	`</data></slide>`

const cleanSlide = `<slide id="p"><data>` +
	`<shape type="text" topLeftX="80" topLeftY="60" width="800" height="80"><content textType="title"><p>Title</p></content></shape>` +
	`</data></slide>`

func TestLintSlide_BlocksOverlap(t *testing.T) {
	r, err := LintSlide(context.Background(), overlapSlide)
	if err != nil {
		t.Fatalf("lint error: %v", err)
	}
	if !r.Blocked() {
		t.Fatalf("expected blocked, got status=%q errors=%d", r.Summary.Status, r.Summary.ErrorCount)
	}
	if be := BlockedError(r); be == nil {
		t.Fatal("expected BlockedError, got nil")
	}
	found := false
	for _, f := range r.errorFindings() {
		if f.Code == "bbox_overlap" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected bbox_overlap finding, got %+v", r.errorFindings())
	}
}

func TestLintSlide_PassesClean(t *testing.T) {
	r, err := LintSlide(context.Background(), cleanSlide)
	if err != nil {
		t.Fatalf("lint error: %v", err)
	}
	if r.Blocked() {
		t.Fatalf("expected clean pass, got blocked with %+v", r.errorFindings())
	}
	if be := BlockedError(r); be != nil {
		t.Fatalf("expected no BlockedError, got %v", be)
	}
}

func TestLintSlides_Batch(t *testing.T) {
	rs, err := LintSlides(context.Background(), []string{overlapSlide, cleanSlide, overlapSlide})
	if err != nil {
		t.Fatalf("batch lint error: %v", err)
	}
	if len(rs) != 3 {
		t.Fatalf("expected 3 results, got %d", len(rs))
	}
	if !rs[0].Blocked() || rs[1].Blocked() || !rs[2].Blocked() {
		t.Fatalf("unexpected batch verdicts: %v %v %v", rs[0].Blocked(), rs[1].Blocked(), rs[2].Blocked())
	}
}

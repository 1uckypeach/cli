// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package wiki

import (
	"errors"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
)

func TestAnnotateWikiPermissionDeniedMarks131006Terminal(t *testing.T) {
	t.Parallel()

	cause := errors.New("opaque upstream cause")
	err := errs.NewPermissionError(errs.SubtypePermissionDenied, "opaque upstream message").
		WithCode(131006).
		WithCause(cause)

	got := annotateWikiPermissionDenied(err)
	p, ok := errs.ProblemOf(got)
	if !ok {
		t.Fatalf("ProblemOf() ok=false")
	}
	if p.Category != errs.CategoryAuthorization || p.Subtype != errs.SubtypePermissionDenied || p.Code != 131006 {
		t.Fatalf("problem = %#v, want authorization/permission_denied/131006", p)
	}
	if p.Retryable {
		t.Fatalf("problem retryable = true, want false: %#v", p)
	}
	if !errors.Is(got, cause) {
		t.Fatalf("errors.Is(got, cause) = false, want preserved cause")
	}
	if p.Hint != wikiPermissionDeniedHint() {
		t.Fatalf("hint = %q, want %q", p.Hint, wikiPermissionDeniedHint())
	}
}

func TestAnnotateWikiWritePermissionDeniedUsesContainerEditGuidance(t *testing.T) {
	t.Parallel()

	cause := errors.New("opaque upstream cause")
	err := errs.NewPermissionError(errs.SubtypePermissionDenied, "no destination parent node permission").
		WithCode(131006).
		WithCause(cause)

	got := annotateWikiWritePermissionDenied(err)
	p, ok := errs.ProblemOf(got)
	if !ok {
		t.Fatalf("ProblemOf() ok=false")
	}
	if p.Code != 131006 || p.Retryable {
		t.Fatalf("problem = %#v, want non-retryable 131006", p)
	}
	if !errors.Is(got, cause) {
		t.Fatalf("errors.Is(got, cause) = false, want preserved cause")
	}
	if p.Hint != wikiWritePermissionDeniedHint() {
		t.Fatalf("hint = %q, want %q", p.Hint, wikiWritePermissionDeniedHint())
	}
	if strings.Contains(p.Hint, "grant read access") {
		t.Fatalf("write hint = %q, must not prescribe read access", p.Hint)
	}
}

func TestAnnotateWikiPermissionDeniedLeavesOtherCodesUnchanged(t *testing.T) {
	t.Parallel()

	err := errs.NewAPIError(errs.SubtypeNotFound, "node not found").WithCode(131005)
	got := annotateWikiPermissionDenied(err)
	if got != err {
		t.Fatalf("annotateWikiPermissionDenied() = %v, want original error", got)
	}
	p, ok := errs.ProblemOf(got)
	if !ok {
		t.Fatalf("ProblemOf() ok=false")
	}
	if p.Code != 131005 || p.Hint != "" {
		t.Fatalf("problem = %#v, want unchanged 131005 without permission hint", p)
	}
}

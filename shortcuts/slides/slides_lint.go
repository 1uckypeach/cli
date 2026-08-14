// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slides

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/slideslint"
	"github.com/larksuite/cli/shortcuts/common"
)

// lintGateOff reports whether the pre-submit lint should be skipped, either
// per-call via --no-lint or globally via LARKSUITE_CLI_SLIDES_LINT=off (an
// operational kill-switch, mirroring LARKSUITE_CLI_REMOTE_META).
func lintGateOff(runtime *common.RuntimeContext) bool {
	return runtime.Bool("no-lint") || os.Getenv("LARKSUITE_CLI_SLIDES_LINT") == "off"
}

// topLevelSlideElements are the element roots that can stand alone as a direct
// child of <data>. Only these can be lint-checked in isolation by wrapping them
// in a synthetic <slide>; others (<td>, <tr>, <col>...) are only valid nested
// inside a parent and would produce false schema errors on their own.
var topLevelSlideElements = map[string]bool{
	"shape": true, "img": true, "line": true, "polyline": true,
	"icon": true, "table": true, "chart": true,
}

// fragmentRootTag returns the local name of a fragment's root element, or "".
func fragmentRootTag(frag string) string {
	s := strings.TrimSpace(frag)
	if !strings.HasPrefix(s, "<") {
		return ""
	}
	s = s[1:]
	end := strings.IndexFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '>' || r == '/'
	})
	if end < 0 {
		return s
	}
	return s[:end]
}

// lintSlideGate runs the pre-submit lint on one full-page <slide> XML and blocks
// the edit if the linter reports errors. Skipped when --no-lint is set.
//
// Fail-open on infrastructure errors: if the lint engine itself cannot run
// (e.g. an unwritable compile-cache dir), the edit proceeds with a stderr note
// rather than being blocked by a gate defect. The gate only stops genuine lint
// findings, never its own failures.
func lintSlideGate(ctx context.Context, runtime *common.RuntimeContext, slideXML string) error {
	if lintGateOff(runtime) {
		return nil
	}
	res, err := slideslint.LintSlide(ctx, slideXML)
	if err != nil {
		fmt.Fprintf(runtime.IO().ErrOut, "slide lint skipped (engine error): %s\n", err)
		return nil
	}
	if blockErr := slideslint.BlockedError(res); blockErr != nil {
		return blockErr
	}
	for _, w := range slideslint.WarningLines(res) {
		fmt.Fprintf(runtime.IO().ErrOut, "slide lint warning: %s\n", w)
	}
	return nil
}

// lintSlideGateBatch lints many full-page <slide> XMLs in one interpreter run and
// blocks if any page has errors, naming the failing page(s). Skipped when
// --no-lint is set. Fail-open on engine errors, like lintSlideGate.
func lintSlideGateBatch(ctx context.Context, runtime *common.RuntimeContext, slideXMLs []string) error {
	if lintGateOff(runtime) || len(slideXMLs) == 0 {
		return nil
	}
	results, err := slideslint.LintSlides(ctx, slideXMLs)
	if err != nil {
		fmt.Fprintf(runtime.IO().ErrOut, "slide lint skipped (engine error): %s\n", err)
		return nil
	}
	var blockedPages []int
	var firstBlockErr error
	for i, res := range results {
		if blockErr := slideslint.BlockedError(res); blockErr != nil {
			blockedPages = append(blockedPages, i+1)
			if firstBlockErr == nil {
				firstBlockErr = blockErr
			}
			continue
		}
		for _, w := range slideslint.WarningLines(res) {
			fmt.Fprintf(runtime.IO().ErrOut, "slide lint warning (page %d): %s\n", i+1, w)
		}
	}
	if len(blockedPages) > 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"slide lint blocked %d of %d pages (pages %v); first failure:\n%s\nPass --no-lint to bypass.",
			len(blockedPages), len(slideXMLs), blockedPages, firstBlockErr)
	}
	return nil
}

// lintFragmentGate lints the new/changed elements of a +replace-slide call.
//
// Block edits submit fragments, not a whole page, so a full-page geometry check
// would need re-reading the page and re-applying the parts. Instead each fragment
// whose root can stand alone under <data> is wrapped in a synthetic <slide> and
// linted for element-local errors (schema conformance, out-of-canvas). Cross-
// element overlap against the rest of the page is NOT checked here — for a
// full-page rebuild that gets the complete geometry lint, use +update-slide.
//
// Fragments whose root only exists nested (<td>/<tr>/<col>/<colgroup>) are skipped
// to avoid false schema errors. Warnings from the single-element synthetic slide
// are meaningless and dropped; only errors block.
func lintFragmentGate(ctx context.Context, runtime *common.RuntimeContext, parts []replacePart) error {
	if lintGateOff(runtime) {
		return nil
	}
	var wrapped []string
	var partIdx []int
	for i, p := range parts {
		var frag string
		switch {
		case p.Replacement != nil:
			frag = *p.Replacement
		case p.Insertion != nil:
			frag = *p.Insertion
		default:
			continue
		}
		if !topLevelSlideElements[fragmentRootTag(frag)] {
			continue // nested-only element (td/tr/...) — cannot lint in isolation
		}
		wrapped = append(wrapped, "<slide id=\"lintprobe\"><data>"+frag+"</data></slide>")
		partIdx = append(partIdx, i)
	}
	if len(wrapped) == 0 {
		return nil
	}
	results, err := slideslint.LintSlides(ctx, wrapped)
	if err != nil {
		fmt.Fprintf(runtime.IO().ErrOut, "slide lint skipped (engine error): %s\n", err)
		return nil
	}
	for j, res := range results {
		if blockErr := slideslint.BlockedError(res); blockErr != nil {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"replace-slide part %d failed lint:\n%s\nPass --no-lint to bypass.", partIdx[j]+1, blockErr)
		}
	}
	return nil
}

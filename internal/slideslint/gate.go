// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slideslint

import (
	"context"
	"fmt"
	"strings"

	"github.com/larksuite/cli/errs"
)

// LintSlides lints many full-page <slide> XML documents in a single interpreter
// run (fixed ~0.7s setup amortized across all pages, ~30ms per extra page).
func LintSlides(ctx context.Context, slideXMLs []string) ([]Result, error) {
	if len(slideXMLs) == 0 {
		return nil, nil
	}
	return runBatch(ctx, slideXMLs)
}

// LintSlide lints one full-page <slide> XML document.
func LintSlide(ctx context.Context, slideXML string) (Result, error) {
	results, err := runBatch(ctx, []string{slideXML})
	if err != nil {
		return Result{}, err
	}
	if len(results) == 0 {
		return Result{}, errs.NewInternalError(errs.SubtypeSDKError, "slides lint returned no result")
	}
	return results[0], nil
}

// BlockedError turns a blocked Result into a typed validation error whose message
// lists the error-level findings. Returns nil if the result is not blocked.
func BlockedError(r Result) error {
	if !r.Blocked() {
		return nil
	}
	findings := r.errorFindings()
	var b strings.Builder
	b.WriteString("slide XML failed lint (")
	b.WriteString(fmt.Sprintf("%d error", len(findings)))
	if len(findings) != 1 {
		b.WriteString("s")
	}
	b.WriteString("); fix these or pass --no-lint to bypass:")
	for _, f := range findings {
		b.WriteString("\n  - [")
		b.WriteString(f.Code)
		b.WriteString("] ")
		b.WriteString(f.Message)
		if f.Hint != "" {
			b.WriteString("\n    hint: ")
			b.WriteString(f.Hint)
		}
	}
	return errs.NewValidationError(errs.SubtypeInvalidArgument, "%s", b.String())
}

// WarningLines returns human-readable warning lines for surfacing in output when
// the slide is not blocked but has advisory findings.
func WarningLines(r Result) []string {
	var lines []string
	for _, f := range r.warningFindings() {
		lines = append(lines, fmt.Sprintf("[%s] %s", f.Code, f.Message))
	}
	return lines
}

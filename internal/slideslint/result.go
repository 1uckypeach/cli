// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slideslint

// Finding is one lint issue (a subset of xml_lint's per-issue object).
type Finding struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type slideResult struct {
	SlideNumber int       `json:"slide_number"`
	Status      string    `json:"status"`
	Errors      []Finding `json:"errors"`
	Warnings    []Finding `json:"warnings"`
}

// Result mirrors the fields of xml_lint's JSON output that the gate consumes.
// Unmodeled fields (measurements, related_objects, rule, ...) are ignored.
type Result struct {
	Summary struct {
		Status       string `json:"status"`
		ErrorCount   int    `json:"error_count"`
		WarningCount int    `json:"warning_count"`
	} `json:"summary"`
	Document struct {
		Errors   []Finding `json:"errors"`
		Warnings []Finding `json:"warnings"`
	} `json:"document"`
	Slides []slideResult `json:"slides"`
	// DriverError is set by lint_batch.py when it cannot parse the driver payload.
	DriverError string `json:"driver_error"`
}

// Blocked reports whether the linter judged this slide unfit to submit.
func (r Result) Blocked() bool { return r.Summary.Status == "blocked" || r.Summary.ErrorCount > 0 }

// Errors returns every error-level finding across document and slide scopes.
func (r Result) errorFindings() []Finding {
	out := append([]Finding{}, r.Document.Errors...)
	for _, s := range r.Slides {
		out = append(out, s.Errors...)
	}
	return out
}

// Warnings returns every warning-level finding across document and slide scopes.
func (r Result) warningFindings() []Finding {
	out := append([]Finding{}, r.Document.Warnings...)
	for _, s := range r.Slides {
		out = append(out, s.Warnings...)
	}
	return out
}

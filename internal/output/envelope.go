// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package output

// Envelope is the standard success response wrapper.
type Envelope struct {
	OK                 bool                   `json:"ok"`
	Identity           string                 `json:"identity,omitempty"`
	Data               interface{}            `json:"data,omitempty"`
	Meta               *Meta                  `json:"meta,omitempty"`
	ContentSafetyAlert interface{}            `json:"_content_safety_alert,omitempty"`
	Notice             map[string]interface{} `json:"_notice,omitempty"`
}

// Meta carries optional metadata in envelope responses.
type Meta struct {
	Count    int          `json:"count,omitempty"`
	Rollback string       `json:"rollback,omitempty"`
	Next     []NextAction `json:"next,omitempty"`
}

// NextAction is a typed "suggested next command" that an AI caller can execute
// directly.
type NextAction struct {
	Label   string `json:"label"`
	Command string `json:"command"`
	// Template, when true, marks a Command that contains <...> placeholders and
	// must be fully substituted by the caller before execution; it is not
	// directly executable as-is. Directly executable commands omit the field.
	Template bool `json:"template,omitempty"`
}

// PendingNotice, if set, returns system-level notices to inject as the
// "_notice" field in JSON output envelopes. Set by cmd/root.go.
// Returns nil when there is nothing to report.
var PendingNotice func() map[string]interface{}

// GetNotice returns the current pending notice for struct-based callers.
// Returns nil when there is nothing to report.
func GetNotice() map[string]interface{} {
	if PendingNotice == nil {
		return nil
	}
	return PendingNotice()
}

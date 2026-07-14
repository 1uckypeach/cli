// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package output

import (
	"slices"
	"strings"

	"github.com/larksuite/cli/errs"
)

// FormatCapabilities is the single description of the output formats a
// command supports. Help text, shell completion, shorthand normalization and
// runtime validation all consume the same value so they cannot drift apart.
type FormatCapabilities struct {
	names []string
}

var (
	// StandardFormats applies to API, service and ordinary shortcut commands.
	StandardFormats = NewFormatCapabilities("json", "pretty", "table", "ndjson", "csv")
	// JSONPrettyFormats applies to commands with a dedicated human renderer and
	// no streaming/tabular output, such as auth scopes.
	JSONPrettyFormats = NewFormatCapabilities("json", "pretty")
)

// NewFormatCapabilities constructs an immutable format capability set.
func NewFormatCapabilities(names ...string) FormatCapabilities {
	return FormatCapabilities{names: append([]string(nil), names...)}
}

// Names returns a copy suitable for completion candidates.
func (c FormatCapabilities) Names() []string {
	return append([]string(nil), c.names...)
}

// Usage returns the canonical help text for a --format flag.
func (c FormatCapabilities) Usage() string {
	return "output format: " + strings.Join(c.names, "|")
}

// Supports reports whether name is part of this command's output contract.
func (c FormatCapabilities) Supports(name string) bool {
	return slices.Contains(c.names, strings.ToLower(name))
}

// Resolve applies the --json shorthand and validates the selected format.
// An explicit --format always wins over --json.
func (c FormatCapabilities) Resolve(format string, formatExplicit, jsonShorthand bool) (string, error) {
	if jsonShorthand && !formatExplicit {
		format = "json"
	}
	format = strings.ToLower(format)
	if c.Supports(format) {
		return format, nil
	}
	return "", errs.NewValidationError(errs.SubtypeInvalidArgument,
		"unsupported output format %q; supported formats: %s", format, strings.Join(c.names, ", ")).
		WithParam("--format")
}

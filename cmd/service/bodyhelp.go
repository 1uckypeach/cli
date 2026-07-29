// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// bodyhelp renders the request-body contract into a method command's help so
// that help is self-sufficient: whoever reached `<method> --help` can construct
// the call without a follow-up schema query. Scope is deliberately input-only —
// the response structure stays in `lark-cli schema <path>`, which is what the
// pointer at the end of method help is for.

package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/larksuite/cli/internal/meta"
	"github.com/larksuite/cli/internal/schema"
)

// bodyHelp renders the --data contract for the method's declared body fields,
// or "" when the method declares none (the bare --data escape hatch then keeps
// its current one-line usage). Skeleton first (a copyable JSON shape), then one
// facts line per field. Nesting renders one level deep; anything deeper is left
// to the schema pointer rather than inflating every method's help.
func bodyHelp(fields []meta.Field) string {
	if len(fields) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Request body (--data '<json>'):\n")
	b.WriteString("  " + bodySkeleton(fields) + "\n")
	b.WriteString("  Fields:\n")
	for _, f := range fields {
		writeFieldLine(&b, f, "    ")
		for _, ch := range f.Children() {
			writeFieldLine(&b, ch, "      ")
		}
	}
	return b.String()
}

func writeFieldLine(b *strings.Builder, f meta.Field, indent string) {
	req := "optional"
	if f.Required {
		req = "required"
	}
	// Name and type come from the same upstream document as the description, so
	// they get the same sanitizing — a facts line that cleaned only one of the
	// three would still be forgeable through the others.
	line := fmt.Sprintf("%s%s  (%s, %s)", indent,
		schema.SanitizeIndexDesc(f.Name), req, schema.SanitizeIndexDesc(f.CanonicalType()))
	if d := schema.SanitizeIndexDesc(schema.FirstSentence(f.Description)); d != "" {
		line += "  " + d
	}
	b.WriteString(line + "\n")
}

// bodySkeleton renders a single-line JSON object with type placeholders, e.g.
// {"receive_id": "<string>", "filter": {"user_ids": ["<item>", "..."]}}. It is
// meant to be copied straight into --data, so field names are JSON-quoted
// rather than interpolated: a name carrying a quote or newline must not be able
// to break the shape.
func bodySkeleton(fields []meta.Field) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, quoteJSONKey(f.Name)+": "+skeletonValue(f, 1))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// quoteJSONKey renders name as a JSON string literal. The name is sanitized
// first: json.Marshal escapes quotes, backslashes and C0, but passes bidi
// controls, C1 and zero-width characters through — which would let the skeleton
// line and the facts line below it disagree about what a field is called, and
// let two different fields render identically.
func quoteJSONKey(name string) string {
	name = schema.SanitizeIndexDesc(name)
	if q, err := json.Marshal(name); err == nil {
		return string(q)
	}
	return fmt.Sprintf("%q", name)
}

func skeletonValue(f meta.Field, depth int) string {
	switch f.CanonicalType() {
	case "object":
		if depth <= 0 || len(f.Properties) == 0 {
			return "{}"
		}
		inner := make([]string, 0, len(f.Properties))
		for _, ch := range f.Children() {
			inner = append(inner, quoteJSONKey(ch.Name)+": "+skeletonValue(ch, depth-1))
		}
		return "{" + strings.Join(inner, ", ") + "}"
	case "array":
		return `["<item>"]`
	case "boolean":
		return "false"
	case "integer", "number":
		return "0"
	default:
		return `"<string>"`
	}
}

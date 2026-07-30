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
	// Allowed values and bounds are what make the line enough to construct the
	// call with: a description like "参与人权限" names the field but not its four
	// legal values, and "智能体任务状态" hides the 1-4 range entirely. Only the two
	// inline formatters are shared with the param-flag side — NOT fieldFacts,
	// which also carries a cobra-specific bool clause and cuts descriptions at
	// `；` (that would truncate body descriptions such as mode's "任务完成模式, 1 -
	// 会签任务; 2 - 或签任务" to just the first alternative).
	//
	// The enum clause is sanitized because both the values and their meanings are
	// upstream text, same as the name/type/description above; bounds come from
	// strconv, so they carry nothing to sanitize.
	if opts := f.EnumOptions(); len(opts) > 0 {
		line += "  enum: " + schema.SanitizeIndexDesc(formatEnumInline(opts))
	}
	if bounds := formatBoundsInline(f); bounds != "" {
		line += "  " + bounds
	}
	b.WriteString(line + "\n")
}

// bodySkeleton renders a single-line JSON object with type placeholders, e.g.
// {"receive_id": "<string>", "members": [{"id": "<string>"}]}. It is meant to be
// copied straight into --data, so field names are JSON-quoted rather than
// interpolated: a name carrying a quote or newline must not be able to break the
// shape.
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

// skeletonValue renders one field's placeholder. Two rules govern the whole
// skeleton, and the array branch below is an instance of the first, not a
// special case.
//
// One: a placeholder must never positively assert something the API rejects.
// The skeleton is meant to be copied verbatim and --dry-run does not validate
// the body, so a wrong assertion has nothing local to catch it.
//
// Two: where the type admits a placeholder that advertises itself as one — a
// string can hold "<string>", a number cannot hold anything comparable — prefer
// that over a real value, even a legal one. A skeleton whose every field already
// holds a plausible value reads as ready to send, and 56 of the enum-carrying
// body fields sit on write or high-risk-write methods with drive
// permission.public.patch's share_entity among them, where the first allowed
// value happens to be "anyone". Nothing is lost by not filling them in: the
// facts line below the skeleton now lists the allowed values, so the skeleton is
// no longer the only place to learn them.
//
// The two rules only ever conflict for numbers, where rule two has nothing to
// offer, so there `0` gives way to an allowed value or to a floor that puts 0
// out of range.
func skeletonValue(f meta.Field, depth int) string {
	switch f.CanonicalType() {
	case "object":
		if depth <= 0 || len(f.Properties) == 0 {
			return "{}"
		}
		return "{" + joinProperties(f, depth) + "}"
	case "array":
		// The metadata wraps an array's *element* schema in "properties", so
		// these children describe one element rather than sibling fields. The
		// two reasons to stop recursing are not interchangeable here: with no
		// element schema the array really is scalar, but running out of nesting
		// budget says nothing about the element type. Collapsing both to
		// ["<item>"] would assert "array of strings" for an array of objects —
		// the wrong assertion the rule above forbids. So the budget case
		// degrades to [{}], mirroring how an object degrades to {}.
		if len(f.Properties) == 0 {
			return `["<item>"]`
		}
		if depth <= 0 {
			return "[{}]"
		}
		return "[{" + joinProperties(f, depth) + "}]"
	case "boolean":
		return "false"
	case "integer", "number":
		if opts := f.EnumOptions(); len(opts) > 0 {
			if v, ok := skeletonLiteral(opts[0].Value); ok {
				return v
			}
		}
		// 0 stays the placeholder unless it is below the field's floor. Reaching
		// for the floor unconditionally trades a legal neutral value for a legal
		// absurd one: okr indicators patch declares min -99999999999 on three
		// value fields whose upstream example is plain 0. Of the 27 numeric fields
		// declaring a min, only 10 put 0 out of range; 6 declare a negative floor,
		// and those are the ones this guard protects. No field declares a max
		// below 0, so "below the floor" is the whole of "out of range" here.
		//
		// min/max are type-agnostic upstream — they may bound a value or a
		// string's length (see meta.Field.MinBound) — so a bound is only read as
		// a value here, where a length reading is meaningless. String fields keep
		// their type marker for that reason as well as rule two above.
		if min := f.MinBound(); min != nil && *min > 0 {
			if v, ok := skeletonLiteral(*min); ok {
				return v
			}
		}
		return "0"
	default:
		return `"<string>"`
	}
}

// skeletonLiteral renders a concrete placeholder — an allowed value or a numeric
// floor — as a JSON literal, reporting false when it cannot be rendered both
// safely and faithfully. Callers fall back to the type marker.
//
// The live guard is the marshal error. Both callers are the numeric branch, and
// meta.coerceLiteral reaches "number" through strconv.ParseFloat, which accepts
// "Inf" and "NaN"; rendering those with strconv would print `+Inf` and cost the
// skeleton its one hard promise, that it parses. json.Marshal refuses them.
//
// The sanitize comparison guards the other half of "safely": json.Marshal
// escapes quotes, backslashes and C0 but passes bidi controls, C1 and zero-width
// characters through. No caller passes text today — a numeric enum coerces to
// int64/float64 or is dropped — so this half is currently unreachable, and it
// stays because the alternative is a helper whose contract quietly depends on
// who calls it. Note that sanitizing a *value* is not the fix it is for a field
// name (quoteJSONKey): the sanitized text is no longer the value the API
// accepts, so it would trade a rendering risk for the wrong-assertion risk
// skeletonValue exists to prevent. Hence reject rather than clean.
//
// Marshal FIRST, then compare — the order is load-bearing, not stylistic.
// Comparing the marshalled form puts quotes around the payload, so
// SanitizeIndexDesc's TrimSpace cannot reach a value's own leading or trailing
// space, and a tab arrives already escaped to `\t` so it cannot trip the
// whitespace-run collapse. Sanitizing before marshalling loses both properties
// and starts rejecting legitimate values.
func skeletonLiteral(v any) (string, bool) {
	q, err := json.Marshal(v)
	if err != nil {
		return "", false
	}
	if s := string(q); schema.SanitizeIndexDesc(s) == s {
		return s, true
	}
	return "", false
}

// joinProperties renders f's children as JSON members, one nesting level down.
func joinProperties(f meta.Field, depth int) string {
	children := f.Children()
	inner := make([]string, 0, len(children))
	for _, ch := range children {
		inner = append(inner, quoteJSONKey(ch.Name)+": "+skeletonValue(ch, depth-1))
	}
	return strings.Join(inner, ", ")
}

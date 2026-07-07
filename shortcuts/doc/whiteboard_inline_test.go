// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"testing"
)

func TestIsValidWhiteboardType(t *testing.T) {
	tests := []struct {
		typ  string
		want bool
	}{
		{"raw", true},
		{"plantuml", true},
		{"mermaid", true},
		{"svg", true},
		{"", false},
		{"unknown", false},
		{"RAW", false},
		{"PlantUML", false},
	}
	for _, tt := range tests {
		got := isValidWhiteboardType(tt.typ)
		if got != tt.want {
			t.Errorf("isValidWhiteboardType(%q) = %v, want %v", tt.typ, got, tt.want)
		}
	}
}

func TestParseWhiteboardStartTag(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
		attrs   map[string]string
	}{
		{
			name:    "path attribute",
			raw:     `<whiteboard type="plantuml" path="@./diagram.puml">`,
			wantErr: false,
			attrs:   map[string]string{"type": "plantuml", "path": "@./diagram.puml"},
		},
		{
			name:    "self-closing",
			raw:     `<whiteboard token="abc"/>`,
			wantErr: false,
			attrs:   map[string]string{"token": "abc"},
		},
		{
			name:    "no attributes",
			raw:     `<whiteboard>`,
			wantErr: false,
			attrs:   map[string]string{},
		},
	}
	for _, tt := range tests {
		got, err := parseWhiteboardStartTag(tt.raw)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: parseWhiteboardStartTag(%q) error = %v, wantErr = %v", tt.name, tt.raw, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		for k, want := range tt.attrs {
			gotVal, ok := got.attr(k)
			if !ok {
				t.Errorf("%s: expected attr %q not found", tt.name, k)
			} else if gotVal != want {
				t.Errorf("%s: attr %q = %q, want %q", tt.name, k, gotVal, want)
			}
		}
	}
}

func TestRemoveAttrs(t *testing.T) {
	tag := whiteboardStartTag{
		Attrs: []whiteboardAttr{
			{Name: "type", Value: "plantuml"},
			{Name: "path", Value: "@./diagram.puml"},
		},
	}
	tag.removeAttrs("path")
	if _, ok := tag.attr("path"); ok {
		t.Error("path attribute should have been removed")
	}
	if _, ok := tag.attr("type"); !ok {
		t.Error("type attribute should still exist")
	}
}

func TestRenderWhiteboardStartTag(t *testing.T) {
	tag := whiteboardStartTag{
		Attrs: []whiteboardAttr{
			{Name: "type", Value: "plantuml"},
		},
	}
	result := tag.render(false)
	if result != `<whiteboard type="plantuml">` {
		t.Errorf("render() = %q, want %q", result, `<whiteboard type="plantuml">`)
	}
}

func TestWhiteboardStartTagHasAttr(t *testing.T) {
	tag := whiteboardStartTag{
		Attrs: []whiteboardAttr{
			{Name: "type", Value: "plantuml"},
		},
	}
	if !tag.hasAttr("type") {
		t.Error("should have type attr")
	}
	if tag.hasAttr("path") {
		t.Error("should not have path attr")
	}
}
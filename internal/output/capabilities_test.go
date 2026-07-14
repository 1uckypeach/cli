// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package output

import (
	"errors"
	"reflect"
	"testing"

	"github.com/larksuite/cli/errs"
)

func TestFormatCapabilitiesResolve(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		formatExplicit bool
		jsonShorthand  bool
		want           string
		wantErr        bool
	}{
		{name: "default", format: "json", want: "json"},
		{name: "json shorthand", format: "table", jsonShorthand: true, want: "json"},
		{name: "explicit format wins", format: "table", formatExplicit: true, jsonShorthand: true, want: "table"},
		{name: "case normalized", format: "PRETTY", formatExplicit: true, want: "pretty"},
		{name: "unsupported", format: "xml", formatExplicit: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StandardFormats.Resolve(tt.format, tt.formatExplicit, tt.jsonShorthand)
			if tt.wantErr {
				var validationErr *errs.ValidationError
				if !errors.As(err, &validationErr) || validationErr.Subtype != errs.SubtypeInvalidArgument || validationErr.Param != "--format" {
					t.Fatalf("Resolve() error = %T %v, want invalid_argument --format validation error", err, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("Resolve() = %q, %v; want %q, nil", got, err, tt.want)
			}
		})
	}
}

func TestFormatCapabilitiesDriveHelpAndCompletion(t *testing.T) {
	if got, want := StandardFormats.Usage(), "output format: json|pretty|table|ndjson|csv"; got != want {
		t.Fatalf("Usage() = %q, want %q", got, want)
	}
	if got, want := StandardFormats.Names(), []string{"json", "pretty", "table", "ndjson", "csv"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
}

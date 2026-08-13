// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package errclass

import (
	"testing"

	"github.com/larksuite/cli/errs"
)

func TestLookupCodeMeta_DocCodes(t *testing.T) {
	tests := []struct {
		code     int
		category errs.Category
		subtype  errs.Subtype
	}{
		{3380002, errs.CategoryAPI, errs.SubtypeNotFound},
		{3380004, errs.CategoryAuthorization, errs.SubtypePermissionDenied},
	}
	for _, test := range tests {
		meta, ok := LookupCodeMeta(test.code)
		if !ok {
			t.Fatalf("LookupCodeMeta(%d) ok=false", test.code)
		}
		if meta.Category != test.category || meta.Subtype != test.subtype || meta.Retryable {
			t.Fatalf("LookupCodeMeta(%d) = %+v, want category=%s subtype=%s retryable=false", test.code, meta, test.category, test.subtype)
		}
	}
}

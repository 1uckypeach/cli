// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package errclass

import "github.com/larksuite/cli/errs"

// docCodeMeta holds docs_ai document error mappings. Recovery wording remains
// at the docs shortcut boundary because it depends on document operations.
var docCodeMeta = map[int]CodeMeta{
	3380002: {Category: errs.CategoryAPI, Subtype: errs.SubtypeNotFound},                   // document id invalid, missing, or invisible
	3380004: {Category: errs.CategoryAuthorization, Subtype: errs.SubtypePermissionDenied}, // no permission to operate on this document
}

func init() { mergeCodeMeta(docCodeMeta, "doc") }

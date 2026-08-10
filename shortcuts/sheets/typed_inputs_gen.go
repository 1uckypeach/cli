// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Code generated from data/flag-defs.json; DO NOT EDIT.

package sheets

import (
	"encoding/json"

	"github.com/larksuite/cli/shortcuts/common"
)

// These fragments are generated only for shortcuts explicitly opted into a
// behavior-reviewed Typed migration. Relations, aliases, hidden compatibility
// metadata, hooks, authorization, Data, and Output remain handwritten.

type BatchUpdateGeneratedInput struct {
	URL              common.Provided[string] "flag:\"url\" schema:\"optional\" doc:\"Spreadsheet locator (independent from per-operation sheet locator)\""
	SpreadsheetToken common.Provided[string] "flag:\"spreadsheet-token\" schema:\"optional\" doc:\"Spreadsheet locator (independent from per-operation sheet locator)\""
	Operations       json.RawMessage         "flag:\"operations\" schema:\"required\" cli:\"sources=flag|file|stdin;encoding=json\" doc:\"JSON array: [{\\\"shortcut\\\":\\\"+xxx-yyy\\\",\\\"input\\\":{...}}, ...]. shortcut uses CLI names; input is that shortcut's flag set — it includes the per-operation sheet locator (sheet_id or sheet_name) but not the spreadsheet token/url (pass that once at the top level via --url/--spreadsheet-token; +batch-update has no top-level --sheet-id). input keys are the shortcut's flags flattened into JSON (e.g. \\\"range\\\":\\\"A11:B12\\\"), not another nested layer. For basic flags use lark-cli sheets <shortcut> --help; for composite JSON flags use --print-schema --flag-name <flag>. Do not pass an explicit operation field. Fail-fast by default: the first failure aborts the remaining operations and already-applied sub-operations are NOT rolled back (on \\\"N succeeded, M failed\\\" resend only the failed tail, not the whole batch); pass --continue-on-error to keep going past failures; no nesting; executed serially.\""
	ContinueOnError  common.Provided[bool]   "flag:\"continue-on-error\" schema:\"optional\" doc:\"Continue with remaining operations when a sub-operation fails; default false (abort on first failure)\""
}

type CSVPutGeneratedInput struct {
	URL              common.Provided[string] "flag:\"url\" schema:\"optional\" doc:\"Spreadsheet URL (XOR with `--spreadsheet-token`)\""
	SpreadsheetToken common.Provided[string] "flag:\"spreadsheet-token\" schema:\"optional\" doc:\"Spreadsheet token (XOR with `--url`)\""
	SheetID          common.Provided[string] "flag:\"sheet-id\" schema:\"optional\" doc:\"Sheet reference_id — required: pass this or `--sheet-name` (exactly one of the two)\""
	SheetName        common.Provided[string] "flag:\"sheet-name\" schema:\"optional\" doc:\"Sheet name — required: pass this or `--sheet-id` (exactly one of the two)\""
	StartCell        common.Provided[string] "flag:\"start-cell\" schema:\"optional;default=\\\"A1\\\"\" doc:\"Top-left A1 anchor (e.g. `A1`, `B5`; no sheet prefix — use `--sheet-id` / `--sheet-name` to select the sheet); must be a single cell, range notation not accepted; the bottom-right is inferred from CSV row/column counts\""
	CSV              string                  "flag:\"csv\" schema:\"required\" cli:\"sources=flag|file|stdin\" doc:\"RFC 4180 CSV text; values or formulas (a leading = is evaluated as a formula); no styles / comments / images (use +cells-set for those).\""
	AllowOverwrite   common.Provided[bool]   "flag:\"allow-overwrite\" schema:\"optional;default=true\" doc:\"Allow overwriting (default true); set false to error if any target cell is non-empty\""
	Range            common.Provided[string] "flag:\"range\" schema:\"optional\" doc:\"alias for --start-cell (parity with +csv-get / +cells-set, which locate with --range); a range like A1:H17 collapses to its top-left cell\""
}

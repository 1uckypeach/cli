// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package sheets

import (
	"context"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

// ─── lark_sheet_range_operations ──────────────────────────────────────
//
// Four tools, nine shortcuts:
//
//   - clear_cell_range  → +cells-clear              (high-risk-write)
//   - merge_cells       → +cells-merge / +cells-unmerge
//   - resize_range      → +rows-resize / +cols-resize
//   - transform_range   → +range-move / +range-copy / +range-fill / +range-sort
//
// +rows-resize / +cols-resize are grouped under "工作表" for CLI discoverability
// even though the backing tool lives in this skill.

// CellsClear wraps clear_cell_range.
//
// CLI's --scope vocabulary (content / formats / all) is normalized to the
// tool's clear_type vocabulary (contents / formats / all) — the spec's
// singular/plural mismatch is intentionally absorbed here.
var CellsClear = common.Shortcut{
	Service:     "sheets",
	Command:     "+cells-clear",
	Description: "Clear cell content, formats, or both within a range (irreversible).",
	Risk:        "high-risk-write",
	Scopes:      []string{"sheets:spreadsheet:write_only"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags:       flagsFor("+cells-clear"),
	Validate:    validateViaInput(cellsClearInput),
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		token, _ := resolveSpreadsheetToken(runtime)
		sheetID, sheetName, _ := resolveSheetSelector(runtime)
		input, _ := cellsClearInput(runtime, token, sheetID, sheetName)
		return invokeToolDryRun(token, ToolKindWrite, "clear_cell_range", input)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, err := resolveSpreadsheetTokenExec(runtime)
		if err != nil {
			return err
		}
		sheetID, sheetName, err := resolveSheetSelector(runtime)
		if err != nil {
			return err
		}
		input, err := cellsClearInput(runtime, token, sheetID, sheetName)
		if err != nil {
			return err
		}
		out, err := callTool(ctx, runtime, token, ToolKindWrite, "clear_cell_range", input)
		if err != nil {
			return annotateEmbeddedBlockClearErr(err)
		}
		runtime.Out(out, nil)
		return nil
	},
	Tips: []string{
		"high-risk-write — always preview with --dry-run; clear is not undoable.",
		"Can't delete an embedded pivot/chart by clearing cells — remove the object itself with +pivot-delete / +chart-delete.",
	},
}

func cellsClearInput(runtime flagView, token, sheetID, sheetName string) (map[string]interface{}, error) {
	if err := requireSheetSelector(sheetID, sheetName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(runtime.Str("range")) == "" {
		return nil, sheetsValidationForFlag("range", "--range is required")
	}
	input := map[string]interface{}{
		"excel_id":   token,
		"range":      strings.TrimSpace(runtime.Str("range")),
		"clear_type": normalizeClearType(runtime.Str("scope")),
	}
	sheetSelectorForToolInput(input, sheetID, sheetName)
	return input, nil
}

// normalizeClearType maps the CLI --scope vocabulary (content / formats / all)
// to the clear_cell_range tool's clear_type vocabulary (contents / formats /
// all). The content↔contents singular/plural mismatch is absorbed here so both
// +cells-clear and the +cells-batch-clear fan-out stay in lockstep.
func normalizeClearType(scope string) string {
	switch scope {
	case "formats", "all":
		return scope
	default: // "content" or unset
		return "contents"
	}
}

// annotateEmbeddedBlockClearErr augments the backend's "embedded block" clear
// failure with the concrete fix. clear_cell_range only clears cell values /
// formats — it cannot delete an embedded object (pivot table / chart) that
// overlaps the range, which is what the backend's "can not find embedded block"
// actually means. Trajectories burned dozens of commands trying to recover a
// pivot-occupied A1 with cells-clear; point the agent at the object's own
// delete command instead. Non-matching errors pass through untouched.
func annotateEmbeddedBlockClearErr(err error) error {
	p, ok := errs.ProblemOf(err)
	if !ok {
		return err
	}
	if !strings.Contains(strings.ToLower(p.Message), "embedded block") {
		return err
	}
	const hint = "the range overlaps an embedded object (pivot table / chart); " +
		"cells-clear only clears cell values/formats and cannot delete it — " +
		"delete the object with its own command (+pivot-delete / +chart-delete; find the id via +pivot-list / +chart-list)"
	if p.Hint == "" {
		p.Hint = hint
	} else {
		p.Hint += "; " + hint
	}
	return err
}

// CellsMerge / CellsUnmerge share the merge_cells tool, dispatched by the
// `operation` enum. --merge-type applies to merge only and maps to tool
// field merge_type (`all` / `rows` / `columns`).
var CellsMerge = newMergeShortcut(
	"+cells-merge", "Merge cells in a range.", "merge", true,
)
var CellsUnmerge = newMergeShortcut(
	"+cells-unmerge", "Unmerge cells in a range.", "unmerge", false,
)

func newMergeShortcut(command, desc, op string, withMergeType bool) common.Shortcut {
	flags := flagsFor(command)
	return common.Shortcut{
		Service:     "sheets",
		Command:     command,
		Description: desc,
		Risk:        "write",
		Scopes:      []string{"sheets:spreadsheet:write_only"},
		AuthTypes:   []string{"user", "bot"},
		HasFormat:   true,
		Flags:       flags,
		Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
			token, err := resolveSpreadsheetToken(runtime)
			if err != nil {
				return err
			}
			sheetID := strings.TrimSpace(runtime.Str("sheet-id"))
			sheetName := strings.TrimSpace(runtime.Str("sheet-name"))
			_, err = mergeInput(runtime, token, sheetID, sheetName, op, withMergeType)
			return err
		},
		DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
			token, _ := resolveSpreadsheetToken(runtime)
			sheetID, sheetName, _ := resolveSheetSelector(runtime)
			input, _ := mergeInput(runtime, token, sheetID, sheetName, op, withMergeType)
			return invokeToolDryRun(token, ToolKindWrite, "merge_cells", input)
		},
		Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
			token, err := resolveSpreadsheetTokenExec(runtime)
			if err != nil {
				return err
			}
			sheetID, sheetName, err := resolveSheetSelector(runtime)
			if err != nil {
				return err
			}
			input, err := mergeInput(runtime, token, sheetID, sheetName, op, withMergeType)
			if err != nil {
				return err
			}
			out, err := callTool(ctx, runtime, token, ToolKindWrite, "merge_cells", input)
			if err != nil {
				return err
			}
			runtime.Out(out, nil)
			return nil
		},
	}
}

func mergeInput(runtime flagView, token, sheetID, sheetName, op string, withMergeType bool) (map[string]interface{}, error) {
	if err := requireSheetSelector(sheetID, sheetName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(runtime.Str("range")) == "" {
		return nil, sheetsValidationForFlag("range", "--range is required")
	}
	input := map[string]interface{}{
		"excel_id":  token,
		"range":     strings.TrimSpace(runtime.Str("range")),
		"operation": op,
	}
	sheetSelectorForToolInput(input, sheetID, sheetName)
	if withMergeType {
		if mt := runtime.Str("merge-type"); mt != "" && mt != "all" {
			input["merge_type"] = mt
		} else {
			input["merge_type"] = "all"
		}
	}
	return input, nil
}

// resize_range exposes two CLI shortcuts:
//
//   +rows-resize / +cols-resize — set row heights / column widths. Common
//   case: pass --height <px> / --width <px>; the pixel mode is implied so
//   --type can be omitted (or set to `pixel` — equivalent). Non-pixel modes
//   go through --type standard / --type auto (rows only) and cannot be
//   combined with the pixel flag. At least one of {--height/--width, --type}
//   must be given. --range is an A1 closed range ("2:10" / "5" rows or
//   "A:E" / "C" columns); single-element form is expanded to "N:N" before
//   send because resize_range rejects bare single-element ranges.
//
// Wire shape: resize_height / resize_width carries { type, value? }, e.g.
//   { "type": "pixel", "value": 30 }  or  { "type": "standard" }.

// RowsResize wraps resize_range for row heights. Pass --height <px> for a
// pixel value (--type may be omitted); --type standard/auto for non-pixel
// modes.
var RowsResize = common.Shortcut{
	Service:     "sheets",
	Command:     "+rows-resize",
	Description: "Resize rows: --height <px> for a pixel height (--type optional), or --type standard/auto for non-pixel modes (--range is 1-based A1 like \"2:10\" or \"5\").",
	Risk:        "write",
	Scopes:      []string{"sheets:spreadsheet:write_only"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags:       flagsFor("+rows-resize"),
	Validate:    validateViaResize("row"),
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		token, _ := resolveSpreadsheetToken(runtime)
		sheetID, sheetName, _ := resolveSheetSelector(runtime)
		input, _ := resizeInput(runtime, token, sheetID, sheetName, "row")
		return invokeToolDryRun(token, ToolKindWrite, "resize_range", input)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, err := resolveSpreadsheetTokenExec(runtime)
		if err != nil {
			return err
		}
		sheetID, sheetName, err := resolveSheetSelector(runtime)
		if err != nil {
			return err
		}
		input, err := resizeInput(runtime, token, sheetID, sheetName, "row")
		if err != nil {
			return err
		}
		out, err := callTool(ctx, runtime, token, ToolKindWrite, "resize_range", input)
		if err != nil {
			return err
		}
		runtime.Out(out, nil)
		return nil
	},
}

// ColsResize wraps resize_range for column widths. Pass --width <px> for a
// pixel value (--type may be omitted); --type standard for the default
// width. Column widths do not support auto-fit — --type does not accept
// auto.
var ColsResize = common.Shortcut{
	Service:     "sheets",
	Command:     "+cols-resize",
	Description: "Resize columns: --width <px> for a pixel width (--type optional), or --type standard to reset (--range is column letters like \"A:E\" or \"C\"; no auto for cols).",
	Risk:        "write",
	Scopes:      []string{"sheets:spreadsheet:write_only"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags:       flagsFor("+cols-resize"),
	Validate:    validateViaResize("column"),
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		token, _ := resolveSpreadsheetToken(runtime)
		sheetID, sheetName, _ := resolveSheetSelector(runtime)
		input, _ := resizeInput(runtime, token, sheetID, sheetName, "column")
		return invokeToolDryRun(token, ToolKindWrite, "resize_range", input)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, err := resolveSpreadsheetTokenExec(runtime)
		if err != nil {
			return err
		}
		sheetID, sheetName, err := resolveSheetSelector(runtime)
		if err != nil {
			return err
		}
		input, err := resizeInput(runtime, token, sheetID, sheetName, "column")
		if err != nil {
			return err
		}
		out, err := callTool(ctx, runtime, token, ToolKindWrite, "resize_range", input)
		if err != nil {
			return err
		}
		runtime.Out(out, nil)
		return nil
	},
}

// validateViaResize wires the standalone Validate to resizeInput so both
// paths (standalone + batch sub-op) emit the same error for missing --type,
// malformed --range, or --type auto on columns.
func validateViaResize(dimension string) func(ctx context.Context, runtime *common.RuntimeContext) error {
	return func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, err := resolveSpreadsheetToken(runtime)
		if err != nil {
			return err
		}
		sheetID := strings.TrimSpace(runtime.Str("sheet-id"))
		sheetName := strings.TrimSpace(runtime.Str("sheet-name"))
		_, err = resizeInput(runtime, token, sheetID, sheetName, dimension)
		return err
	}
}

// nonPixelTypes lists the --type values a given dimension accepts (rows also
// accept auto; columns only accept standard). Used to shape the hint printed
// when --type is missing or invalid.
func nonPixelTypes(dimension string) string {
	if dimension == "row" {
		return "standard / auto"
	}
	return "standard"
}

// pixelFlag maps a dimension to its pixel-value flag name (--height for rows,
// --width for cols). The wire block always emits "pixel" as the mode; the
// per-dimension flag name is just the surface knob.
func pixelFlag(dimension string) string {
	if dimension == "row" {
		return "height"
	}
	return "width"
}

// commandForDimension returns the shortcut command name a given dimension
// belongs to; used in error messages so users see "+rows-resize" / "+cols-resize"
// instead of the internal "row" / "column" tag.
func commandForDimension(dimension string) string {
	if dimension == "row" {
		return "+rows-resize"
	}
	return "+cols-resize"
}

// resizeInput builds the resize_range tool input. dimension is "row" /
// "column" (selected by the calling shortcut); --range must match that
// dimension (row → digits like "2:10" / "5"; column → letters like "A:E" /
// "C"). Single-element form is expanded to "N:N" because resize_range
// rejects bare single-element ranges.
//
// Surface: pixel size goes through --height / --width (dimension-specific).
// --type is optional when the pixel flag is present (defaults to "pixel");
// explicit --type pixel is accepted and equivalent. --type standard / auto
// select non-pixel modes and cannot be combined with the pixel flag.
func resizeInput(runtime flagView, token, sheetID, sheetName, dimension string) (map[string]interface{}, error) {
	if err := requireSheetSelector(sheetID, sheetName); err != nil {
		return nil, err
	}
	if !runtime.Changed("range") {
		return nil, sheetsValidationForFlag("range", "--range is required")
	}
	rangeStr := strings.TrimSpace(runtime.Str("range"))
	parsedDim, _, _, err := parseA1Range(rangeStr)
	if err != nil {
		return nil, sheetsValidationForFlag("range", "invalid --range %q: %v", rangeStr, err)
	}
	if parsedDim != dimension {
		want := "row numbers (e.g. \"2:10\")"
		if dimension == "column" {
			want = "column letters (e.g. \"A:E\")"
		}
		return nil, sheetsValidationForFlag("range", "--range %q is a %s range; %s expects %s", rangeStr, parsedDim, commandForDimension(dimension), want)
	}
	if !strings.Contains(rangeStr, ":") {
		rangeStr = rangeStr + ":" + rangeStr
	}

	sizeFlag := pixelFlag(dimension)
	hasSize := runtime.Changed(sizeFlag)
	typ := strings.TrimSpace(runtime.Str("type"))
	hasType := typ != ""

	if !hasSize && !hasType {
		return nil, common.ValidationErrorf("give --%s <px> for a pixel size, or --type %s", sizeFlag, nonPixelTypes(dimension)).WithParams(sheetsInvalidParam(sizeFlag, "required"), sheetsInvalidParam("type", "required"))
	}
	if hasSize && hasType && typ != "pixel" {
		return nil, common.ValidationErrorf("--%s cannot be combined with --type %s", sizeFlag, typ).WithParams(sheetsInvalidParam(sizeFlag, "mutually exclusive"), sheetsInvalidParam("type", "mutually exclusive"))
	}
	if hasType && dimension == "column" && typ == "auto" {
		return nil, sheetsValidationForFlag("type", "--type auto is rows-only (column widths do not support auto-fit); use +rows-resize")
	}
	if hasType && typ == "pixel" && !hasSize {
		return nil, common.ValidationErrorf("--type pixel requires --%s <px>", sizeFlag).WithParams(sheetsInvalidParam("type", "required"), sheetsInvalidParam(sizeFlag, "required"))
	}

	sizeBlock := map[string]interface{}{}
	if hasSize {
		px := runtime.Int(sizeFlag)
		if px <= 0 {
			return nil, sheetsValidationForFlag(sizeFlag, "--%s must be > 0", sizeFlag)
		}
		sizeBlock["type"] = "pixel"
		sizeBlock["value"] = px
	} else {
		sizeBlock["type"] = typ
	}

	input := map[string]interface{}{
		"excel_id": token,
		"range":    rangeStr,
	}
	sheetSelectorForToolInput(input, sheetID, sheetName)
	if dimension == "row" {
		input["resize_height"] = sizeBlock
	} else {
		input["resize_width"] = sizeBlock
	}
	return input, nil
}

// ─── transform_range (4 shortcuts) ────────────────────────────────────
//
// move / copy take --source-range + --target-range (+ optional cross-sheet
// target). fill takes --source-range + --target-range + --series-type. sort
// takes --range + --sort-keys + --has-header.

// RangeMove cuts data from --source-range and pastes at --target-range,
// optionally on another sheet.
var RangeMove = common.Shortcut{
	Service:     "sheets",
	Command:     "+range-move",
	Description: "Cut a range and paste it at a new location (optionally cross-sheet).",
	Risk:        "write",
	Scopes:      []string{"sheets:spreadsheet:write_only"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags:       flagsFor("+range-move"),
	Validate:    validateRangeMoveOrCopy("move", false),
	DryRun:      transformDryRunFn("move", false, false),
	Execute:     transformExecuteFn("move", false, false),
}

// RangeCopy duplicates a range to a new location with optional paste-type
// filter (values / formulas / formats / all).
var RangeCopy = common.Shortcut{
	Service:     "sheets",
	Command:     "+range-copy",
	Description: "Copy a range to a new location (--paste-type controls what is copied).",
	Risk:        "write",
	Scopes:      []string{"sheets:spreadsheet:write_only"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags:       flagsFor("+range-copy"),
	Validate:    validateRangeMoveOrCopy("copy", true),
	DryRun:      transformDryRunFn("copy", true, false),
	Execute:     transformExecuteFn("copy", true, false),
}

// RangeFill performs autofill from a template range into a target range.
// --series-type is a 5-value CLI vocabulary; the tool only distinguishes
// `copyCells` from `fillSeries`. The mapping is documented in
// fillSeriesToToolType.
var RangeFill = common.Shortcut{
	Service:     "sheets",
	Command:     "+range-fill",
	Description: "Autofill a target range from a source template (copy / linear / growth / date series).",
	Risk:        "write",
	Scopes:      []string{"sheets:spreadsheet:write_only"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags:       flagsFor("+range-fill"),
	Validate:    validateViaInput(rangeFillInput),
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		token, _ := resolveSpreadsheetToken(runtime)
		sheetID, sheetName, _ := resolveSheetSelector(runtime)
		input, _ := rangeFillInput(runtime, token, sheetID, sheetName)
		return invokeToolDryRun(token, ToolKindWrite, "transform_range", input)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, err := resolveSpreadsheetTokenExec(runtime)
		if err != nil {
			return err
		}
		sheetID, sheetName, err := resolveSheetSelector(runtime)
		if err != nil {
			return err
		}
		input, err := rangeFillInput(runtime, token, sheetID, sheetName)
		if err != nil {
			return err
		}
		out, err := callTool(ctx, runtime, token, ToolKindWrite, "transform_range", input)
		if err != nil {
			return err
		}
		runtime.Out(out, nil)
		return nil
	},
}

// RangeSort sorts rows within a range by one or more columns.
var RangeSort = common.Shortcut{
	Service:     "sheets",
	Command:     "+range-sort",
	Description: "Sort rows within a range by one or more columns.",
	Risk:        "write",
	Scopes:      []string{"sheets:spreadsheet:write_only"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags:       flagsFor("+range-sort"),
	Validate:    validateViaInput(rangeSortInput),
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		token, _ := resolveSpreadsheetToken(runtime)
		sheetID, sheetName, _ := resolveSheetSelector(runtime)
		input, _ := rangeSortInput(runtime, token, sheetID, sheetName)
		return invokeToolDryRun(token, ToolKindWrite, "transform_range", input)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, err := resolveSpreadsheetTokenExec(runtime)
		if err != nil {
			return err
		}
		sheetID, sheetName, err := resolveSheetSelector(runtime)
		if err != nil {
			return err
		}
		input, err := rangeSortInput(runtime, token, sheetID, sheetName)
		if err != nil {
			return err
		}
		out, err := callTool(ctx, runtime, token, ToolKindWrite, "transform_range", input)
		if err != nil {
			return err
		}
		runtime.Out(out, nil)
		return nil
	},
}

// ─── transform_range helpers ──────────────────────────────────────────

// validateRangeMoveOrCopy wires the standalone Validate to transformMoveCopyInput
// so missing --source-range / --target-range fire the same friendly error on
// the batch sub-op path.
func validateRangeMoveOrCopy(op string, withPasteType bool) func(ctx context.Context, runtime *common.RuntimeContext) error {
	return func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, err := resolveSpreadsheetToken(runtime)
		if err != nil {
			return err
		}
		sheetID := strings.TrimSpace(runtime.Str("sheet-id"))
		sheetName := strings.TrimSpace(runtime.Str("sheet-name"))
		_, err = transformMoveCopyInput(runtime, token, sheetID, sheetName, op, withPasteType)
		return err
	}
}

func transformDryRunFn(op string, withPasteType, _ bool) func(context.Context, *common.RuntimeContext) *common.DryRunAPI {
	return func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		token, _ := resolveSpreadsheetToken(runtime)
		sheetID, sheetName, _ := resolveSheetSelector(runtime)
		input, _ := transformMoveCopyInput(runtime, token, sheetID, sheetName, op, withPasteType)
		return invokeToolDryRun(token, ToolKindWrite, "transform_range", input)
	}
}

func transformExecuteFn(op string, withPasteType, _ bool) func(context.Context, *common.RuntimeContext) error {
	return func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, err := resolveSpreadsheetTokenExec(runtime)
		if err != nil {
			return err
		}
		sheetID, sheetName, err := resolveSheetSelector(runtime)
		if err != nil {
			return err
		}
		input, err := transformMoveCopyInput(runtime, token, sheetID, sheetName, op, withPasteType)
		if err != nil {
			return err
		}
		out, err := callTool(ctx, runtime, token, ToolKindWrite, "transform_range", input)
		if err != nil {
			return err
		}
		runtime.Out(out, nil)
		return nil
	}
}

func transformMoveCopyInput(runtime flagView, token, sheetID, sheetName, op string, withPasteType bool) (map[string]interface{}, error) {
	if err := requireSheetSelector(sheetID, sheetName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(runtime.Str("source-range")) == "" {
		return nil, sheetsValidationForFlag("source-range", "--source-range is required")
	}
	if strings.TrimSpace(runtime.Str("target-range")) == "" {
		return nil, sheetsValidationForFlag("target-range", "--target-range is required")
	}
	input := map[string]interface{}{
		"excel_id":          token,
		"operation":         op,
		"range":             strings.TrimSpace(runtime.Str("source-range")),
		"destination_range": strings.TrimSpace(runtime.Str("target-range")),
	}
	sheetSelectorForToolInput(input, sheetID, sheetName)
	if tgt := strings.TrimSpace(runtime.Str("target-sheet-id")); tgt != "" {
		input["destination_sheet_id"] = tgt
	}
	if withPasteType {
		if pt := runtime.Str("paste-type"); pt != "" && pt != "all" {
			input["paste_type"] = pasteTypeToTool(pt)
		}
	}
	return input, nil
}

// pasteTypeToTool maps the CLI vocabulary (values / formulas / formats / all)
// to the tool's paste_type field (all / value_only / formula_only / format_only).
func pasteTypeToTool(pt string) string {
	switch pt {
	case "values":
		return "value_only"
	case "formulas":
		return "formula_only"
	case "formats":
		return "format_only"
	}
	return "all"
}

func rangeFillInput(runtime flagView, token, sheetID, sheetName string) (map[string]interface{}, error) {
	if err := requireSheetSelector(sheetID, sheetName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(runtime.Str("source-range")) == "" {
		return nil, sheetsValidationForFlag("source-range", "--source-range is required")
	}
	if strings.TrimSpace(runtime.Str("target-range")) == "" {
		return nil, sheetsValidationForFlag("target-range", "--target-range is required")
	}
	input := map[string]interface{}{
		"excel_id":          token,
		"operation":         "fill",
		"range":             strings.TrimSpace(runtime.Str("source-range")),
		"destination_range": strings.TrimSpace(runtime.Str("target-range")),
		"fill_type":         fillSeriesToToolType(runtime.Str("series-type")),
	}
	sheetSelectorForToolInput(input, sheetID, sheetName)
	return input, nil
}

// fillSeriesToToolType maps the CLI series vocabulary to the tool's fill_type.
// The tool only distinguishes copy vs series; the CLI's series flavor (linear /
// growth / date / auto) all collapse to fillSeries — the actual progression is
// inferred by the server from the source cells.
func fillSeriesToToolType(seriesType string) string {
	if seriesType == "copy" {
		return "copyCells"
	}
	return "fillSeries"
}

func rangeSortInput(runtime flagView, token, sheetID, sheetName string) (map[string]interface{}, error) {
	if err := requireSheetSelector(sheetID, sheetName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(runtime.Str("range")) == "" {
		return nil, sheetsValidationForFlag("range", "--range is required")
	}
	// requireJSONArray runs the embedded JSON Schema for --sort-keys
	// via parseJSONFlag → validateParsedJSONFlag, so each item is
	// already pinned to {column: string, ascending: bool} with the
	// failing index reported. No per-item hand-written guard needed.
	keys, err := requireJSONArray(runtime, "sort-keys")
	if err != nil {
		return nil, err
	}
	input := map[string]interface{}{
		"excel_id":        token,
		"operation":       "sort",
		"range":           strings.TrimSpace(runtime.Str("range")),
		"sort_conditions": keys,
	}
	sheetSelectorForToolInput(input, sheetID, sheetName)
	if runtime.Bool("has-header") {
		input["has_header"] = true
	}
	return input, nil
}

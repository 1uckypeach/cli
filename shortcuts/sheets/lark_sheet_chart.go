// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package sheets

import (
	"context"
	"regexp"
	"strings"

	"github.com/larksuite/cli/shortcuts/common"
)

var chartHexColorPattern = regexp.MustCompile(`^#?[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?$`)

var chartSemanticConfigFlags = []string{
	"title",
	"subtitle",
	"legend-position",
	"x-axis-title",
	"y-axis-title",
	"secondary-y-axis-title",
	"x-axis-label-angle",
	"y-axis-label-angle",
	"data-labels",
	"data-label-position",
	"stack",
	"color-palette",
}

// ChartCreateBasic creates a complete server-side chart snapshot from a chart
// type and a rectangular source range. The CLI only forwards semantic input;
// it deliberately does not own or duplicate the full chart snapshot template.
var ChartCreateBasic = common.Shortcut{
	Service:     "sheets",
	Command:     "+chart-create-basic",
	Description: "Create a basic chart from a chart type and data range; the server builds and validates the full snapshot.",
	Risk:        "write",
	Scopes:      []string{"sheets:spreadsheet:write_only"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags:       flagsFor("+chart-create-basic"),
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, err := resolveSpreadsheetToken(runtime)
		if err != nil {
			return err
		}
		sheetID, sheetName, err := resolveSheetSelector(runtime)
		if err != nil {
			return err
		}
		_, err = chartCreateBasicInput(runtime, token, sheetID, sheetName)
		return err
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		token, _ := resolveSpreadsheetToken(runtime)
		sheetID, sheetName, _ := resolveSheetSelector(runtime)
		input, _ := chartCreateBasicInput(runtime, token, sheetID, sheetName)
		return invokeToolDryRun(token, ToolKindWrite, "manage_chart_object", input)
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
		input, err := chartCreateBasicInput(runtime, token, sheetID, sheetName)
		if err != nil {
			return err
		}
		out, err := callTool(ctx, runtime, token, ToolKindWrite, "manage_chart_object", input)
		if err != nil {
			return err
		}
		runtime.Out(out, nil)
		return nil
	},
}

// ChartConfigUpdate updates the common chart settings that repeatedly caused
// full-snapshot retries in eval traces. Advanced per-series and marker styling
// remains on +chart-update --properties.
var ChartConfigUpdate = common.Shortcut{
	Service:     "sheets",
	Command:     "+chart-config-update",
	Description: "Update common chart titles, axes, legend, labels, stacking, smoothing, or chart-level colors without sending a snapshot.",
	Risk:        "write",
	Scopes:      []string{"sheets:spreadsheet:write_only"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags:       flagsFor("+chart-config-update"),
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, err := resolveSpreadsheetToken(runtime)
		if err != nil {
			return err
		}
		sheetID, sheetName, err := resolveSheetSelector(runtime)
		if err != nil {
			return err
		}
		_, err = chartConfigUpdateInput(runtime, token, sheetID, sheetName)
		return err
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		token, _ := resolveSpreadsheetToken(runtime)
		sheetID, sheetName, _ := resolveSheetSelector(runtime)
		input, _ := chartConfigUpdateInput(runtime, token, sheetID, sheetName)
		return invokeToolDryRun(token, ToolKindWrite, "manage_chart_object", input)
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
		input, err := chartConfigUpdateInput(runtime, token, sheetID, sheetName)
		if err != nil {
			return err
		}
		out, err := callTool(ctx, runtime, token, ToolKindWrite, "manage_chart_object", input)
		if err != nil {
			return err
		}
		runtime.Out(out, nil)
		return nil
	},
}

func chartCreateBasicInput(rt flagView, token, sheetID, sheetName string) (map[string]interface{}, error) {
	if err := requireSheetSelector(sheetID, sheetName); err != nil {
		return nil, err
	}
	chartType := strings.TrimSpace(rt.Str("chart-type"))
	if chartType == "" {
		return nil, sheetsValidationForFlag("chart-type", "--chart-type is required")
	}
	dataRange := strings.TrimSpace(rt.Str("data-range"))
	if dataRange == "" {
		return nil, sheetsValidationForFlag("data-range", "--data-range is required")
	}
	rows, cols, err := rangeDimensions(dataRange)
	if err != nil || rows < 2 || cols < 2 {
		return nil, sheetsValidationForFlag("data-range", "--data-range must be a rectangular range with at least 2 rows and 2 columns")
	}
	direction := rt.Str("data-direction")
	if direction == "" {
		direction = "column"
	}
	dimensionCount := cols
	if direction == "row" {
		dimensionCount = rows
	}
	if chartType == "combo" && dimensionCount < 3 {
		return nil, sheetsValidationForFlag("data-range", "combo chart requires at least 3 rows or columns along --data-direction")
	}

	basic := map[string]interface{}{
		"chart_type": chartType,
		"data_range": dataRange,
	}
	if rt.Changed("data-direction") {
		basic["data_direction"] = rt.Str("data-direction")
	}
	if err := validateChartColorFlags(rt); err != nil {
		return nil, err
	}
	addChartSemanticConfig(rt, basic)

	if rt.Changed("anchor-cell") {
		anchor := strings.TrimSpace(rt.Str("anchor-cell"))
		_, row, ok := splitCellRef(anchor)
		if !ok {
			return nil, sheetsValidationForFlag("anchor-cell", "--anchor-cell must be a single A1 cell such as F2")
		}
		colEnd := 0
		for colEnd < len(anchor) && ((anchor[colEnd] >= 'A' && anchor[colEnd] <= 'Z') || (anchor[colEnd] >= 'a' && anchor[colEnd] <= 'z')) {
			colEnd++
		}
		basic["position"] = map[string]interface{}{"row": row, "col": strings.ToUpper(anchor[:colEnd])}
	}
	widthChanged := rt.Changed("width")
	heightChanged := rt.Changed("height")
	if widthChanged != heightChanged {
		return nil, common.ValidationErrorf("--width and --height must be provided together").WithParams(
			sheetsInvalidParam("width", "must be paired with --height"),
			sheetsInvalidParam("height", "must be paired with --width"),
		)
	}
	if widthChanged {
		if rt.Int("width") < 10 || rt.Int("height") < 10 {
			return nil, common.ValidationErrorf("--width and --height must be at least 10")
		}
		basic["size"] = map[string]interface{}{"width": rt.Int("width"), "height": rt.Int("height")}
	}

	input := map[string]interface{}{"excel_id": token, "operation": "create", "basic_chart": basic}
	sheetSelectorForToolInput(input, sheetID, sheetName)
	if err := validateInputAgainstSchema(rt, input); err != nil {
		return nil, err
	}
	return input, nil
}

func chartConfigUpdateInput(rt flagView, token, sheetID, sheetName string) (map[string]interface{}, error) {
	if err := requireSheetSelector(sheetID, sheetName); err != nil {
		return nil, err
	}
	chartID := strings.TrimSpace(rt.Str("chart-id"))
	if chartID == "" {
		return nil, sheetsValidationForFlag("chart-id", "--chart-id is required")
	}
	updates := map[string]interface{}{}
	if err := validateChartColorFlags(rt); err != nil {
		return nil, err
	}
	addChartSemanticConfig(rt, updates)
	if len(updates) == 0 {
		return nil, common.ValidationErrorf("at least one chart configuration flag is required")
	}
	input := map[string]interface{}{
		"excel_id":       token,
		"operation":      "update",
		"chart_id":       chartID,
		"config_updates": updates,
	}
	sheetSelectorForToolInput(input, sheetID, sheetName)
	if err := validateInputAgainstSchema(rt, input); err != nil {
		return nil, err
	}
	return input, nil
}

func addChartSemanticConfig(rt flagView, out map[string]interface{}) {
	for _, flag := range chartSemanticConfigFlags {
		if !rt.Changed(flag) {
			continue
		}
		key := strings.ReplaceAll(flag, "-", "_")
		if flag == "x-axis-label-angle" || flag == "y-axis-label-angle" {
			out[key] = rt.Int(flag)
		} else {
			out[key] = rt.Str(flag)
		}
	}
	if rt.Changed("smooth") {
		out["smooth"] = rt.Bool("smooth")
	}
	if rt.Changed("colors") {
		out["colors"] = normalizedChartColors(rt)
	}
}

func validateChartColorFlags(rt flagView) error {
	if rt.Changed("color-palette") && rt.Changed("colors") {
		return common.ValidationErrorf("--color-palette and --colors are mutually exclusive").WithParams(
			sheetsInvalidParam("color-palette", "cannot be used with --colors"),
			sheetsInvalidParam("colors", "cannot be used with --color-palette"),
		)
	}
	if rt.Changed("colors") {
		colors := normalizedChartColors(rt)
		if len(colors) < 2 {
			return sheetsValidationForFlag("colors", "--colors must contain at least two hex colors")
		}
		for _, color := range colors {
			if !chartHexColorPattern.MatchString(color) {
				return sheetsValidationForFlag("colors", "--colors contains invalid hex color %q", color)
			}
		}
	}
	return nil
}

func normalizedChartColors(rt flagView) []string {
	raw := rt.StrSlice("colors")
	colors := make([]string, len(raw))
	for i := range raw {
		colors[i] = strings.TrimSpace(raw[i])
	}
	return colors
}

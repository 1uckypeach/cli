// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package sheets

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/larksuite/cli/shortcuts/common"
	"github.com/spf13/cobra"
)

var chartHexColorPattern = regexp.MustCompile(`^#?[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?$`)

// Sheet currently accepts at most 50 value series in one chart. Keep the
// client-side guard aligned with the tool-layer contract so an obviously wide
// source fails before a write call and tells the agent how to recover.
const chartSeriesMaxCount = 50

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
	PostMount:   configureChartSemanticCommand,
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
	PostMount:   configureChartSemanticCommand,
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

// ChartDataUpdate rebinds an existing chart to a new source range. The server
// reads the current snapshot, rebuilds its data mapping, and preserves the
// chart's layout and visual configuration.
var ChartDataUpdate = common.Shortcut{
	Service:     "sheets",
	Command:     "+chart-data-update",
	Description: "Update an existing chart's data range or direction while preserving its layout and visual configuration.",
	Risk:        "write",
	Scopes:      []string{"sheets:spreadsheet:write_only"},
	AuthTypes:   []string{"user", "bot"},
	HasFormat:   true,
	Flags:       flagsFor("+chart-data-update"),
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, err := resolveSpreadsheetToken(runtime)
		if err != nil {
			return err
		}
		sheetID, sheetName, err := resolveSheetSelector(runtime)
		if err != nil {
			return err
		}
		_, err = chartDataUpdateInput(runtime, token, sheetID, sheetName)
		return err
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		token, _ := resolveSpreadsheetToken(runtime)
		sheetID, sheetName, _ := resolveSheetSelector(runtime)
		input, _ := chartDataUpdateInput(runtime, token, sheetID, sheetName)
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
		input, err := chartDataUpdateInput(runtime, token, sheetID, sheetName)
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
	direction := rt.Str("data-direction")
	if direction == "" {
		direction = "column"
	}
	normalizedDataRange, dimensionCount, dataPointCount, err := normalizeBasicChartDataRanges(dataRange, direction)
	if err != nil {
		return nil, err
	}
	if dimensionCount < 2 || dataPointCount < 2 {
		if direction == "column" && dataPointCount < 2 && dimensionCount >= 2 {
			return nil, sheetsValidationForFlag(
				"data-range",
				"--data-range has one row and multiple columns; if categories run horizontally, include the category and value rows in --data-range and use --data-direction row",
			)
		}
		return nil, sheetsValidationForFlag("data-range", "--data-range must provide at least 2 data points and 2 dimensions")
	}

	dim1Index := 1
	if rt.Changed("dim1-index") {
		dim1Index = rt.Int("dim1-index")
	}
	if dim1Index < 1 || dim1Index > dimensionCount {
		return nil, sheetsValidationForFlag(
			"dim1-index",
			"--dim1-index must be between 1 and %d for the selected --data-range",
			dimensionCount,
		)
	}
	dim2Indexes := make([]int, 0, dimensionCount-1)
	for index := 1; index <= dimensionCount; index++ {
		if index != dim1Index {
			dim2Indexes = append(dim2Indexes, index)
		}
	}
	if chartType == "pie" {
		dim2Indexes = dim2Indexes[:1]
	}
	if rt.Changed("dim2-indexes") {
		dim2Indexes, err = parseChartDim2Indexes(rt.Str("dim2-indexes"))
		if err != nil {
			return nil, sheetsValidationForFlag("dim2-indexes", "%v", err)
		}
	}
	for _, index := range dim2Indexes {
		if index > dimensionCount {
			return nil, sheetsValidationForFlag(
				"dim2-indexes",
				"--dim2-indexes must contain only indexes between 1 and %d for the selected --data-range",
				dimensionCount,
			)
		}
		if index == dim1Index {
			return nil, sheetsValidationForFlag(
				"dim2-indexes",
				"--dim2-indexes must not contain the dim1 index %d",
				dim1Index,
			)
		}
	}
	if chartType == "pie" && len(dim2Indexes) != 1 {
		return nil, sheetsValidationForFlag("dim2-indexes", "pie charts require exactly one --dim2-indexes value")
	}
	if chartType == "combo" && len(dim2Indexes) < 2 {
		return nil, sheetsValidationForFlag("dim2-indexes", "combo charts require at least two --dim2-indexes values")
	}
	if len(dim2Indexes) > chartSeriesMaxCount {
		flagName := "data-range"
		if rt.Changed("dim2-indexes") {
			flagName = "dim2-indexes"
		}
		return nil, sheetsValidationForFlag(
			flagName,
			"the selected dimensions create %d series, over the current limit of %d; provide at most %d --dim2-indexes values or build a compact summary table",
			len(dim2Indexes),
			chartSeriesMaxCount,
			chartSeriesMaxCount,
		)
	}

	basic := map[string]interface{}{
		"chart_type": chartType,
		"data_range": normalizedDataRange,
	}
	if rt.Changed("header-range") {
		headerRange := strings.TrimSpace(rt.Str("header-range"))
		if err := validateChartRangeListFlag("header-range", headerRange); err != nil {
			return nil, err
		}
		if err := validateChartHeaderRangeDirection(headerRange, direction); err != nil {
			return nil, err
		}
		basic["header_range"] = headerRange
	}
	if rt.Changed("data-direction") {
		basic["data_direction"] = rt.Str("data-direction")
	}
	if rt.Changed("dim1-index") {
		basic["dim1_index"] = dim1Index
	}
	if rt.Changed("dim2-indexes") {
		basic["dim2_indexes"] = dim2Indexes
	}
	if err := validateChartColorFlags(rt); err != nil {
		return nil, err
	}
	if err := validateChartSemanticEnums(rt); err != nil {
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
	if err := validateChartSemanticEnums(rt); err != nil {
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

func chartDataUpdateInput(rt flagView, token, sheetID, sheetName string) (map[string]interface{}, error) {
	if err := requireSheetSelector(sheetID, sheetName); err != nil {
		return nil, err
	}
	chartID := strings.TrimSpace(rt.Str("chart-id"))
	if chartID == "" {
		return nil, sheetsValidationForFlag("chart-id", "--chart-id is required")
	}
	dataRange := strings.TrimSpace(rt.Str("data-range"))
	if dataRange == "" {
		return nil, sheetsValidationForFlag("data-range", "--data-range is required")
	}
	ranges, err := splitChartDataRanges(dataRange)
	if err != nil {
		return nil, sheetsValidationForFlag("data-range", "invalid --data-range %q: %v", dataRange, err)
	}
	explicitSheet := ""
	for _, value := range ranges {
		item, parseErr := parseChartDataRange(value)
		if parseErr != nil {
			return nil, sheetsValidationForFlag("data-range", "invalid --data-range item %q: %v", value, parseErr)
		}
		if item.sheet != "" {
			if explicitSheet != "" && item.sheet != explicitSheet {
				return nil, sheetsValidationForFlag("data-range", "all --data-range items must belong to the same sheet")
			}
			explicitSheet = item.sheet
		}
	}

	updates := map[string]interface{}{"data_range": dataRange}
	if rt.Changed("header-range") {
		headerRange := strings.TrimSpace(rt.Str("header-range"))
		if err := validateChartRangeListFlag("header-range", headerRange); err != nil {
			return nil, err
		}
		updates["header_range"] = headerRange
	}
	if rt.Changed("data-direction") {
		updates["data_direction"] = rt.Str("data-direction")
	}
	dim1Index := 1
	if rt.Changed("dim1-index") {
		dim1Index = rt.Int("dim1-index")
		if dim1Index < 1 {
			return nil, sheetsValidationForFlag("dim1-index", "--dim1-index must be a positive 1-based index")
		}
		updates["dim1_index"] = dim1Index
	}
	if rt.Changed("dim2-indexes") {
		dim2Indexes, parseErr := parseChartDim2Indexes(rt.Str("dim2-indexes"))
		if parseErr != nil {
			return nil, sheetsValidationForFlag("dim2-indexes", "%v", parseErr)
		}
		for _, index := range dim2Indexes {
			if index == dim1Index {
				return nil, sheetsValidationForFlag(
					"dim2-indexes",
					"--dim2-indexes must not contain the dim1 index %d",
					dim1Index,
				)
			}
		}
		if len(dim2Indexes) > chartSeriesMaxCount {
			return nil, sheetsValidationForFlag(
				"dim2-indexes",
				"--dim2-indexes selects %d series, over the current limit of %d; select fewer series",
				len(dim2Indexes),
				chartSeriesMaxCount,
			)
		}
		updates["dim2_indexes"] = dim2Indexes
	}
	input := map[string]interface{}{
		"excel_id":     token,
		"operation":    "update",
		"chart_id":     chartID,
		"data_updates": updates,
	}
	sheetSelectorForToolInput(input, sheetID, sheetName)
	if err := validateInputAgainstSchema(rt, input); err != nil {
		return nil, err
	}
	return input, nil
}

func validateChartRangeListFlag(flagName, value string) error {
	if value == "" {
		return sheetsValidationForFlag(flagName, "--%s must not be empty", flagName)
	}
	ranges, err := splitChartDataRanges(value)
	if err != nil {
		return sheetsValidationForFlag(flagName, "invalid --%s %q: %v", flagName, value, err)
	}
	explicitSheet := ""
	for _, rangeValue := range ranges {
		item, parseErr := parseChartHeaderRange(rangeValue)
		if parseErr != nil {
			return sheetsValidationForFlag(flagName, "invalid --%s item %q: %v", flagName, rangeValue, parseErr)
		}
		if item.sheet != "" {
			if explicitSheet != "" && item.sheet != explicitSheet {
				return sheetsValidationForFlag(flagName, "all --%s items must belong to the same sheet", flagName)
			}
			explicitSheet = item.sheet
		}
	}
	return nil
}

func validateChartHeaderRangeDirection(value, direction string) error {
	ranges, err := splitChartDataRanges(value)
	if err != nil {
		return sheetsValidationForFlag("header-range", "invalid --header-range %q: %v", value, err)
	}
	for _, rangeValue := range ranges {
		item, parseErr := parseChartHeaderRange(rangeValue)
		if parseErr != nil {
			return sheetsValidationForFlag("header-range", "invalid --header-range item %q: %v", rangeValue, parseErr)
		}
		if direction == "row" && item.colCount != 1 {
			return sheetsValidationForFlag(
				"header-range",
				"row-oriented --header-range must be one column of dimension/series names; this horizontal range looks like a category row, so include it in --data-range and omit --header-range",
			)
		}
		if direction == "column" && item.rowCount != 1 {
			return sheetsValidationForFlag(
				"header-range",
				"column-oriented --header-range must be one row of dimension/series names",
			)
		}
	}
	return nil
}

func parseChartHeaderRange(value string) (chartDataRange, error) {
	item := chartDataRange{}
	ref := strings.TrimSpace(value)
	if bang := strings.LastIndex(ref, "!"); bang >= 0 {
		item.sheet = strings.TrimSpace(ref[:bang])
		ref = strings.TrimSpace(ref[bang+1:])
	}
	if strings.Contains(ref, ":") {
		return parseChartDataRange(value)
	}
	col, row, ok := splitCellRef(ref)
	if !ok {
		return item, common.ValidationErrorf("expected an A1 cell or rectangular range such as A1 or A1:C1")
	}
	item.row, item.col = row, col
	item.rowCount, item.colCount = 1, 1
	return item, nil
}

func parseChartDim2Indexes(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") {
		var indexes []int
		if err := json.Unmarshal([]byte(raw), &indexes); err != nil {
			return nil, common.ValidationErrorf("--dim2-indexes must be a comma-separated list or an array of positive 1-based indexes")
		}
		return validateChartDim2Indexes(indexes)
	}
	parts := strings.Split(raw, ",")
	indexes := make([]int, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			return nil, common.ValidationErrorf("--dim2-indexes must be a comma-separated list of positive 1-based indexes")
		}
		index, err := strconv.Atoi(value)
		if err != nil || index < 1 {
			return nil, common.ValidationErrorf("--dim2-indexes must contain only positive 1-based indexes")
		}
		indexes = append(indexes, index)
	}
	return validateChartDim2Indexes(indexes)
}

func validateChartDim2Indexes(indexes []int) ([]int, error) {
	if len(indexes) == 0 {
		return nil, common.ValidationErrorf("--dim2-indexes must contain at least one positive 1-based index")
	}
	seen := make(map[int]struct{}, len(indexes))
	for _, index := range indexes {
		if index < 1 {
			return nil, common.ValidationErrorf("--dim2-indexes must contain only positive 1-based indexes")
		}
		if _, exists := seen[index]; exists {
			return nil, common.ValidationErrorf("--dim2-indexes must not contain duplicate index %d", index)
		}
		seen[index] = struct{}{}
	}
	return indexes, nil
}

type chartDataRange struct {
	sheet              string
	row, col           int
	rowCount, colCount int
}

func normalizeBasicChartDataRanges(dataRange, direction string) (normalized string, dimensionCount, dataPointCount int, err error) {
	ranges, err := splitChartDataRanges(dataRange)
	if err != nil {
		return "", 0, 0, sheetsValidationForFlag("data-range", "invalid --data-range %q: %v", dataRange, err)
	}
	parsed := make([]chartDataRange, 0, len(ranges))
	for _, value := range ranges {
		item, parseErr := parseChartDataRange(value)
		if parseErr != nil {
			return "", 0, 0, sheetsValidationForFlag("data-range", "invalid --data-range item %q: %v", value, parseErr)
		}
		parsed = append(parsed, item)
	}
	first := parsed[0]
	explicitSheet := ""
	spans := make([][2]int, 0, len(parsed))
	aligned := true
	minRow, minCol := first.row, first.col
	maxRow, maxCol := first.row+first.rowCount, first.col+first.colCount
	for _, item := range parsed {
		if item.sheet != "" {
			if explicitSheet != "" && item.sheet != explicitSheet {
				return "", 0, 0, sheetsValidationForFlag("data-range", "all --data-range items must belong to the same sheet")
			}
			explicitSheet = item.sheet
		}
		if direction == "row" {
			if item.col != first.col || item.colCount != first.colCount {
				aligned = false
			}
			dimensionCount += item.rowCount
			spans = append(spans, [2]int{item.row, item.row + item.rowCount})
		} else {
			if item.row != first.row || item.rowCount != first.rowCount {
				aligned = false
			}
			dimensionCount += item.colCount
			spans = append(spans, [2]int{item.col, item.col + item.colCount})
		}
		minRow = min(minRow, item.row)
		minCol = min(minCol, item.col)
		maxRow = max(maxRow, item.row+item.rowCount)
		maxCol = max(maxCol, item.col+item.colCount)
	}
	overlapping := false
	for i, current := range spans {
		for j := 0; j < i; j++ {
			if current[0] < spans[j][1] && spans[j][0] < current[1] {
				overlapping = true
			}
		}
	}
	normalized = strings.Join(ranges, ",")
	if len(ranges) > 1 && (!aligned || overlapping) {
		prefix := ""
		if explicitSheet != "" {
			prefix = explicitSheet + "!"
		}
		normalized = prefix + columnIndexToLetter(minCol) + strconv.Itoa(minRow+1) + ":" + columnIndexToLetter(maxCol-1) + strconv.Itoa(maxRow)
		dimensionCount = maxCol - minCol
		dataPointCount = maxRow - minRow
		if direction == "row" {
			dimensionCount, dataPointCount = dataPointCount, dimensionCount
		}
		return normalized, dimensionCount, dataPointCount, nil
	}
	if direction == "row" {
		dataPointCount = first.colCount
	} else {
		dataPointCount = first.rowCount
	}
	return normalized, dimensionCount, dataPointCount, nil
}

func splitChartDataRanges(value string) ([]string, error) {
	var ranges []string
	start := 0
	inQuote := false
	for i := 0; i <= len(value); i++ {
		if i < len(value) && value[i] == '\'' {
			if inQuote && i+1 < len(value) && value[i+1] == '\'' {
				i++
			} else {
				inQuote = !inQuote
			}
		}
		if i == len(value) || (value[i] == ',' && !inQuote) {
			part := strings.TrimSpace(value[start:i])
			if part == "" {
				return nil, common.ValidationErrorf("range list contains an empty item")
			}
			ranges = append(ranges, part)
			start = i + 1
		}
	}
	if inQuote {
		return nil, common.ValidationErrorf("unterminated quoted sheet name")
	}
	return ranges, nil
}

func parseChartDataRange(value string) (chartDataRange, error) {
	item := chartDataRange{}
	ref := strings.TrimSpace(value)
	if bang := strings.LastIndex(ref, "!"); bang >= 0 {
		item.sheet = strings.TrimSpace(ref[:bang])
		ref = strings.TrimSpace(ref[bang+1:])
	}
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return item, common.ValidationErrorf("expected a rectangular A1 range such as A1:C10")
	}
	startCol, startRow, startOK := splitCellRef(parts[0])
	endCol, endRow, endOK := splitCellRef(parts[1])
	if !startOK || !endOK || endCol < startCol || endRow < startRow {
		return item, common.ValidationErrorf("expected a rectangular A1 range such as A1:C10")
	}
	item.row, item.col = startRow, startCol
	item.rowCount, item.colCount = endRow-startRow+1, endCol-startCol+1
	return item, nil
}

func configureChartSemanticCommand(cmd *cobra.Command) {
	if cmd.Flags().Lookup("stacked") == nil {
		cmd.Flags().Bool("stacked", false, "compatibility alias for --stack normal")
		_ = cmd.Flags().MarkHidden("stacked")
	}
	originalArgs := cmd.Args
	cmd.Args = func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && cmd.Flags().Changed("smooth") && (args[0] == "true" || args[0] == "false") {
			return cmd.Flags().Set("smooth", args[0])
		}
		return originalArgs(cmd, args)
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		message := err.Error()
		if strings.Contains(message, "unknown flag: --stacked") {
			return sheetsValidationForFlag("stacked", "--stacked is not supported; use --stack normal (or --stack percent for 100%% stacking)")
		}
		return err
	})
}

func addChartSemanticConfig(rt flagView, out map[string]interface{}) {
	for _, flag := range chartSemanticConfigFlags {
		if !rt.Changed(flag) {
			continue
		}
		key := strings.ReplaceAll(flag, "-", "_")
		if flag == "x-axis-label-angle" || flag == "y-axis-label-angle" {
			out[key] = rt.Int(flag)
		} else if flag == "data-labels" && rt.Str(flag) == "category_percentage" {
			out[key] = "value_percentage"
		} else {
			out[key] = rt.Str(flag)
		}
	}
	if rt.Changed("stacked") {
		out["stack"] = "normal"
	}
	if rt.Changed("smooth") {
		out["smooth"] = rt.Bool("smooth")
	}
	if rt.Changed("colors") {
		out["colors"] = normalizedChartColors(rt)
	}
}

func validateChartSemanticEnums(rt flagView) error {
	if rt.Changed("stack") && rt.Changed("stacked") {
		return common.ValidationErrorf("--stack and --stacked are mutually exclusive").WithParams(
			sheetsInvalidParam("stack", "cannot be used with --stacked"),
			sheetsInvalidParam("stacked", "cannot be used with --stack"),
		)
	}
	return nil
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
		if len(colors) == 0 {
			return sheetsValidationForFlag("colors", "--colors must contain at least one hex color")
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
	if len(colors) == 1 {
		colors = append(colors, colors[0])
	}
	return colors
}

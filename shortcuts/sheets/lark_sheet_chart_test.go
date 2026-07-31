// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package sheets

import (
	"strings"
	"testing"
)

func chartDryRunSnapshot(t *testing.T, input map[string]interface{}) map[string]interface{} {
	t.Helper()
	properties, ok := input["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("input.properties = %#v, want object", input["properties"])
	}
	snapshot, ok := properties["snapshot"].(map[string]interface{})
	if !ok {
		t.Fatalf("input.properties.snapshot = %#v, want object", properties["snapshot"])
	}
	return snapshot
}

func TestChartCreateBasic_AllTypes(t *testing.T) {
	t.Parallel()

	types := []string{"column", "bar", "line", "area", "pie", "scatter", "combo", "radar"}
	for _, chartType := range types {
		chartType := chartType
		t.Run(chartType, func(t *testing.T) {
			t.Parallel()
			rangeValue := "A1:C4"
			if chartType == "combo" {
				rangeValue = "A1:D4"
			}
			body := parseDryRunBody(t, ChartCreateBasic, []string{
				"--url", testURL,
				"--sheet-id", testSheetID,
				"--chart-type", chartType,
				"--data-range", rangeValue,
			})
			input := decodeToolInput(t, body, "manage_chart_object")
			if input["operation"] != "create" {
				t.Fatalf("operation = %v, want create", input["operation"])
			}
			if _, ok := input["properties"]; ok {
				t.Fatal("semantic create must not send properties")
			}
			basic, _ := input["basic_chart"].(map[string]interface{})
			if basic["chart_type"] != chartType || basic["data_range"] != rangeValue {
				t.Fatalf("basic_chart = %#v", basic)
			}
		})
	}
}

func TestChartCreateBasic_ConfigAndPlacement(t *testing.T) {
	t.Parallel()
	body := parseDryRunBody(t, ChartCreateBasic, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-type", "line",
		"--data-range", "A1:C4",
		"--anchor-cell", "f2",
		"--width", "640",
		"--height", "360",
		"--title", "Trend",
		"--legend-position", "bottom",
		"--smooth=false",
		"--data-direction", "row",
		"--color-palette", "brandColorSeries@v2",
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	basic, _ := input["basic_chart"].(map[string]interface{})
	position, _ := basic["position"].(map[string]interface{})
	size, _ := basic["size"].(map[string]interface{})
	if position["col"] != "F" || position["row"] != float64(1) {
		t.Errorf("position = %#v, want F2 as zero-based row 1", position)
	}
	if size["width"] != float64(640) || size["height"] != float64(360) {
		t.Errorf("size = %#v", size)
	}
	if basic["title"] != "Trend" || basic["legend_position"] != "bottom" || basic["smooth"] != false ||
		basic["data_direction"] != "row" || basic["color_palette"] != "brandColorSeries@v2" {
		t.Errorf("semantic config = %#v", basic)
	}
}

func TestChartCreateBasic_MultipleAlignedRanges(t *testing.T) {
	t.Parallel()
	rangeValue := "'Data, 2026'!A1:A10,'Data, 2026'!K1:L10"
	body := parseDryRunBody(t, ChartCreateBasic, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-type", "line",
		"--data-range", rangeValue,
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	basic := input["basic_chart"].(map[string]interface{})
	if basic["data_range"] != rangeValue {
		t.Fatalf("basic_chart.data_range = %v, want %q", basic["data_range"], rangeValue)
	}
}

func TestChartCreateBasic_DetachedHeaderRange(t *testing.T) {
	t.Parallel()
	dataRange := "'Sheet1'!A2:A10,'Sheet1'!K2:L10"
	headerRange := "'Sheet1'!A1,'Sheet1'!K1:L1"
	body := parseDryRunBody(t, ChartCreateBasic, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-type", "line",
		"--data-range", dataRange,
		"--header-range", headerRange,
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	basic := input["basic_chart"].(map[string]interface{})
	if basic["data_range"] != dataRange || basic["header_range"] != headerRange {
		t.Fatalf("basic_chart = %#v", basic)
	}
}

func TestChartCreateBasic_MergesMisalignedOrOverlappingRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "separated rows", input: "'Sheet1'!A1:M1,'Sheet1'!A3:M3", expected: "'Sheet1'!A1:M3"},
		{name: "overlapping columns", input: "A1:B10,B1:C10", expected: "A1:C10"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := parseDryRunBody(t, ChartCreateBasic, []string{
				"--url", testURL,
				"--sheet-id", testSheetID,
				"--chart-type", "line",
				"--data-range", tt.input,
			})
			input := decodeToolInput(t, body, "manage_chart_object")
			basic := input["basic_chart"].(map[string]interface{})
			if basic["data_range"] != tt.expected {
				t.Fatalf("basic_chart.data_range = %v, want %q", basic["data_range"], tt.expected)
			}
		})
	}
}

func TestChartCreateBasic_PreservesAlignedCrossSheetRanges(t *testing.T) {
	t.Parallel()

	body := parseDryRunBody(t, ChartCreateBasic, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-type", "line",
		"--data-range", "'A'!A1:A10,'B'!A1:B10",
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	basic := input["basic_chart"].(map[string]interface{})
	if got, want := basic["data_range"], "'A'!A1:A10,'B'!A1:B10"; got != want {
		t.Fatalf("basic_chart.data_range = %v, want %q", got, want)
	}
}

func TestChartSemanticShortcuts_InDedicatedBatch(t *testing.T) {
	body := parseDryRunBody(t, BatchChartCreate, []string{
		"--url", testURL,
		"--operations", `[
			{"sheet-id":"sh1","chart-type":"column","data-range":"A1:C10","title":"Sales"},
			{"sheet-id":"sh1","chart-type":"line","data-range":"E1:G10","title":"Trend"}
		]`,
	})
	input := decodeToolInput(t, body, "batch_update")
	ops := input["operations"].([]interface{})
	if len(ops) != 2 {
		t.Fatalf("operations len = %d, want 2", len(ops))
	}
	for i, op := range ops {
		item := op.(map[string]interface{})
		if item["tool_name"] != "manage_chart_object" {
			t.Fatalf("operations[%d].tool_name = %v", i, item["tool_name"])
		}
		chartInput := item["input"].(map[string]interface{})
		if chartInput["operation"] != "create" {
			t.Fatalf("operations[%d].input.operation = %v", i, chartInput["operation"])
		}
		if _, ok := chartInput["basic_chart"].(map[string]interface{}); !ok {
			t.Fatalf("operations[%d].input.basic_chart = %#v", i, chartInput["basic_chart"])
		}
	}
}

func TestBatchChartCreate_LegacyWrappedInputStillAccepted(t *testing.T) {
	t.Parallel()
	body := parseDryRunBody(t, BatchChartCreate, []string{
		"--url", testURL,
		"--operations", `[{
			"shortcut":"+chart-create-basic",
			"input":{"sheet_id":"sh1","chart_type":"line","data_range":"A1:C10"}
		}]`,
	})
	input := decodeToolInput(t, body, "batch_update")
	ops := input["operations"].([]interface{})
	if len(ops) != 1 || ops[0].(map[string]interface{})["tool_name"] != "manage_chart_object" {
		t.Fatalf("legacy wrapped operation was not translated: %#v", ops)
	}
}

func TestChartConfigUpdate_PartialFields(t *testing.T) {
	t.Parallel()
	body := parseDryRunBody(t, ChartConfigUpdate, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-id", "chart-1",
		"--y-axis-title", "Revenue",
		"--stack", "percent",
		"--smooth=false",
		"--colors", "#112233,#445566",
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	if input["operation"] != "update" || input["chart_id"] != "chart-1" {
		t.Fatalf("input = %#v", input)
	}
	snapshot := chartDryRunSnapshot(t, input)
	plotArea := snapshot["plotArea"].(map[string]interface{})
	plot := plotArea["plot"].(map[string]interface{})
	extra := plot["extra"].(map[string]interface{})
	if extra["smooth"] != false || extra["stack"].(map[string]interface{})["percentage"] != true {
		t.Errorf("plot extra = %#v", extra)
	}
	style, _ := snapshot["style"].(map[string]interface{})
	colors, _ := style["colorTheme"].([]interface{})
	if len(colors) != 2 || colors[0] != "#112233" || colors[1] != "#445566" {
		t.Errorf("snapshot.style.colorTheme = %#v", style["colorTheme"])
	}
	axes := plotArea["axes"].([]interface{})
	if axes[0].(map[string]interface{})["title"].(map[string]interface{})["text"] != "Revenue" {
		t.Errorf("axes = %#v", axes)
	}
}

func TestChartConfigUpdate_SpacedSmoothBool(t *testing.T) {
	t.Parallel()
	body := parseDryRunBody(t, ChartConfigUpdate, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-id", "chart-1",
		"--smooth", "false",
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	plot := chartDryRunSnapshot(t, input)["plotArea"].(map[string]interface{})["plot"].(map[string]interface{})
	if plot["extra"].(map[string]interface{})["smooth"] != false {
		t.Fatalf("snapshot smooth = %v, want false", plot)
	}
}

func TestChartSemanticShortcuts_CompatibleAliases(t *testing.T) {
	t.Parallel()
	chartCreateBasic := shortcutFromRegistry(t, "+chart-create-basic")
	chartConfigUpdate := shortcutFromRegistry(t, "+chart-config-update")
	body := parseDryRunBody(t, chartCreateBasic, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--type", "line",
		"--range", "A1:C10",
		"--x-axis", "Month",
		"--y-axis", "Revenue",
	})
	basic := decodeToolInput(t, body, "manage_chart_object")["basic_chart"].(map[string]interface{})
	if basic["chart_type"] != "line" || basic["data_range"] != "A1:C10" ||
		basic["x_axis_title"] != "Month" || basic["y_axis_title"] != "Revenue" {
		t.Fatalf("chart create aliases = %#v", basic)
	}
	body = parseDryRunBody(t, chartConfigUpdate, []string{
		"--url", testURL, "--sheet-id", testSheetID, "--chart-id", "chart-1", "--stacked",
	})
	snapshot := chartDryRunSnapshot(t, decodeToolInput(t, body, "manage_chart_object"))
	extra := snapshot["plotArea"].(map[string]interface{})["plot"].(map[string]interface{})["extra"].(map[string]interface{})
	if extra["stack"].(map[string]interface{})["percentage"] != false {
		t.Fatalf("--stacked normalized stack = %#v, want non-percentage stack", extra["stack"])
	}
	body = parseDryRunBody(t, chartConfigUpdate, []string{
		"--url", testURL, "--sheet-id", testSheetID, "--chart-id", "chart-1", "--data-labels", "category_percentage",
	})
	snapshot = chartDryRunSnapshot(t, decodeToolInput(t, body, "manage_chart_object"))
	labels := snapshot["plotArea"].(map[string]interface{})["plot"].(map[string]interface{})["labels"].(map[string]interface{})
	if labels["value"] != true || labels["percentage"] != true {
		t.Fatalf("data-labels normalized value = %#v, want value+percentage", labels)
	}
	body = parseDryRunBody(t, chartConfigUpdate, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-id", "chart-1",
		"--x-axis", "Month",
		"--y-axis", "Revenue",
		"--data-labels", "percentage,value",
	})
	snapshot = chartDryRunSnapshot(t, decodeToolInput(t, body, "manage_chart_object"))
	plotArea := snapshot["plotArea"].(map[string]interface{})
	axes := plotArea["axes"].([]interface{})
	labels = plotArea["plot"].(map[string]interface{})["labels"].(map[string]interface{})
	if axes[0].(map[string]interface{})["title"].(map[string]interface{})["text"] != "Month" ||
		axes[1].(map[string]interface{})["title"].(map[string]interface{})["text"] != "Revenue" ||
		labels["value"] != true || labels["percentage"] != true {
		t.Fatalf("chart config aliases = %#v", snapshot)
	}
}

func TestChartSemanticShortcuts_CompatibleAliasesInBatch(t *testing.T) {
	t.Parallel()
	body := parseDryRunBody(t, BatchChartUpdate, []string{
		"--url", testURL,
		"--operations", `[{"shortcut":"+chart-config-update","input":{"sheet_id":"sh1","chart_id":"chart-1","stacked":true,"x_axis":"Month","y_axis":"Revenue","data_labels":"value,percentage","smooth":false}}]`,
	})
	input := decodeToolInput(t, body, "batch_update")
	ops := input["operations"].([]interface{})
	chartInput := ops[0].(map[string]interface{})["input"].(map[string]interface{})
	snapshot := chartDryRunSnapshot(t, chartInput)
	plotArea := snapshot["plotArea"].(map[string]interface{})
	plot := plotArea["plot"].(map[string]interface{})
	labels := plot["labels"].(map[string]interface{})
	extra := plot["extra"].(map[string]interface{})
	if labels["value"] != true || labels["percentage"] != true || extra["smooth"] != false ||
		extra["stack"].(map[string]interface{})["percentage"] != false {
		t.Fatalf("batch config patch = %#v", snapshot)
	}
}

func TestChartSemanticShortcuts_SingleCustomColorIsExpanded(t *testing.T) {
	t.Parallel()

	body := parseDryRunBody(t, ChartCreateBasic, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-type", "line",
		"--data-range", "A1:C10",
		"--colors", "#112233",
	})
	basic := decodeToolInput(t, body, "manage_chart_object")["basic_chart"].(map[string]interface{})
	colors := basic["colors"].([]interface{})
	if len(colors) != 2 || colors[0] != "#112233" || colors[1] != "#112233" {
		t.Fatalf("standalone colors = %#v", colors)
	}

	body = parseDryRunBody(t, BatchChartCreate, []string{
		"--url", testURL,
		"--operations", `[{
			"sheet_id":"sh1","type":"line","range":"A1:C10","colors":["#445566"]
		}]`,
	})
	input := decodeToolInput(t, body, "batch_update")
	ops := input["operations"].([]interface{})
	basic = ops[0].(map[string]interface{})["input"].(map[string]interface{})["basic_chart"].(map[string]interface{})
	colors = basic["colors"].([]interface{})
	if len(colors) != 2 || colors[0] != "#445566" || colors[1] != "#445566" {
		t.Fatalf("batch array colors = %#v", colors)
	}
}

func TestChartCreateBasic_RejectsMoreThanFiftySeries(t *testing.T) {
	t.Parallel()
	_, _, err := runShortcutCapturingErr(t, ChartCreateBasic, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-type", "line",
		"--data-range", "A2:DV7",
		"--data-direction", "column",
	})
	requireValidation(t, err, "create 125 series")
	if !strings.Contains(err.Error(), "current limit of 50") ||
		!strings.Contains(err.Error(), "compact summary table") {
		t.Fatalf("series limit error is not actionable: %v", err)
	}
}

func TestChartCreateBasic_SelectsDimensionsAtCreation(t *testing.T) {
	t.Parallel()
	body := parseDryRunBody(t, ChartCreateBasic, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-type", "line",
		"--data-range", "A1:DV7",
		"--dim1-index", "3",
		"--dim2-indexes", "2,6,8",
	})
	basic := decodeToolInput(t, body, "manage_chart_object")["basic_chart"].(map[string]interface{})
	if basic["dim1_index"] != float64(3) {
		t.Fatalf("basic_chart.dim1_index = %#v", basic["dim1_index"])
	}
	indexes := basic["dim2_indexes"].([]interface{})
	if len(indexes) != 3 || indexes[0] != float64(2) || indexes[1] != float64(6) || indexes[2] != float64(8) {
		t.Fatalf("basic_chart.dim2_indexes = %#v", indexes)
	}
}

func TestChartCreateBasic_SelectsDimensionsInBatch(t *testing.T) {
	t.Parallel()
	body := parseDryRunBody(t, BatchChartCreate, []string{
		"--url", testURL,
		"--operations", `[{
			"sheet_id":"sh1","chart_type":"line","data_range":"A1:DV7","dim1_index":3,"dim2_indexes":[2,6,8]
		}]`,
	})
	input := decodeToolInput(t, body, "batch_update")
	ops := input["operations"].([]interface{})
	basic := ops[0].(map[string]interface{})["input"].(map[string]interface{})["basic_chart"].(map[string]interface{})
	if basic["dim1_index"] != float64(3) {
		t.Fatalf("batch basic_chart.dim1_index = %#v", basic["dim1_index"])
	}
	indexes := basic["dim2_indexes"].([]interface{})
	if len(indexes) != 3 || indexes[0] != float64(2) || indexes[1] != float64(6) || indexes[2] != float64(8) {
		t.Fatalf("batch basic_chart.dim2_indexes = %#v", indexes)
	}
}

func TestChartCreateBasic_RejectsHorizontalHeaderForRowDirection(t *testing.T) {
	t.Parallel()
	_, _, err := runShortcutCapturingErr(t, ChartCreateBasic, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-type", "line",
		"--data-range", "'Sheet1'!A3:M3,'Sheet1'!A5:M5",
		"--header-range", "'Sheet1'!A1:M1",
		"--data-direction", "row",
	})
	requireValidation(t, err, "looks like a category row")
	if !strings.Contains(err.Error(), "include it in --data-range") {
		t.Fatalf("header-range error is not actionable: %v", err)
	}
}

func TestChartCreateBasic_SuggestsRowDirectionForHorizontalCategories(t *testing.T) {
	t.Parallel()
	_, _, err := runShortcutCapturingErr(t, ChartCreateBasic, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-type", "line",
		"--data-range", "'Sheet1'!A1:M1",
	})
	requireValidation(t, err, "--data-direction row")
}

func TestChartDataUpdate_MapsToPartialProperties(t *testing.T) {
	t.Parallel()
	body := parseDryRunBody(t, ChartDataUpdate, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-id", "chart-1",
		"--data-range", "'Sheet1'!A1:M6",
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	if input["operation"] != "update" || input["chart_id"] != "chart-1" {
		t.Fatalf("input = %#v", input)
	}
	data := chartDryRunSnapshot(t, input)["data"].(map[string]interface{})
	refs := data["refs"].([]interface{})
	if refs[0].(map[string]interface{})["value"] != "'Sheet1'!A1:M6" {
		t.Errorf("data patch = %#v", data)
	}
	if _, ok := data["direction"]; ok {
		t.Errorf("omitted --data-direction must be resolved from the current snapshot during execution: %#v", data)
	}
}

func TestChartDataUpdate_ExplicitDirectionAndMultipleRanges(t *testing.T) {
	t.Parallel()
	dataRange := "'Sheet1'!A1:A10,'Sheet2'!A1:B10"
	body := parseDryRunBody(t, ChartDataUpdate, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-id", "chart-1",
		"--data-range", dataRange,
		"--data-direction", "column",
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	data := chartDryRunSnapshot(t, input)["data"].(map[string]interface{})
	refs := data["refs"].([]interface{})
	if data["direction"] != "column" || len(refs) != 2 {
		t.Errorf("data patch = %#v", data)
	}
	if refs[0].(map[string]interface{})["value"] != "'Sheet1'!A1:A10" ||
		refs[1].(map[string]interface{})["value"] != "'Sheet2'!A1:B10" {
		t.Errorf("cross-sheet refs = %#v, want %q", refs, dataRange)
	}
}

func TestChartDataUpdate_DetachedHeaderRange(t *testing.T) {
	t.Parallel()
	dataRange := "'Sheet1'!A2:A10,'Sheet1'!K2:L10"
	headerRange := "'Sheet1'!A1,'Sheet1'!K1:L1"
	body := parseDryRunBody(t, ChartDataUpdate, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-id", "chart-1",
		"--data-range", dataRange,
		"--header-range", headerRange,
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	data := chartDryRunSnapshot(t, input)["data"].(map[string]interface{})
	if data["headerMode"] != "detached" {
		t.Fatalf("data patch = %#v", data)
	}
	dim1 := data["dim1"].(map[string]interface{})["serie"].(map[string]interface{})
	if dim1["nameRef"] != "'Sheet1'!A1" {
		t.Fatalf("detached dim1 = %#v", dim1)
	}
}

func TestChartDataUpdate_ExplicitSeriesIndexes(t *testing.T) {
	t.Parallel()
	body := parseDryRunBody(t, ChartDataUpdate, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-id", "chart-1",
		"--data-range", "'Sheet1'!A1:M6",
		"--dim1-index", "1",
		"--dim2-indexes", "4, 8",
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	data := chartDryRunSnapshot(t, input)["data"].(map[string]interface{})
	dim1 := data["dim1"].(map[string]interface{})["serie"].(map[string]interface{})
	if dim1["index"] != float64(1) {
		t.Errorf("data.dim1 = %#v", dim1)
	}
	series := data["dim2"].(map[string]interface{})["series"].([]interface{})
	if len(series) != 2 || series[0].(map[string]interface{})["index"] != float64(4) ||
		series[1].(map[string]interface{})["index"] != float64(8) {
		t.Errorf("data.dim2 = %#v", series)
	}
}

func TestChartSemanticShortcuts_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "unsupported type", args: []string{"--url", testURL, "--sheet-id", testSheetID, "--chart-type", "donut", "--data-range", "A1:C4"}},
		{name: "invalid semantic enum", args: []string{"--url", testURL, "--sheet-id", testSheetID, "--chart-type", "line", "--data-range", "A1:C4", "--legend-position", "diagonal"}},
		{name: "range too small", args: []string{"--url", testURL, "--sheet-id", testSheetID, "--chart-type", "line", "--data-range", "A1:A4"}},
		{name: "combo needs two series", args: []string{"--url", testURL, "--sheet-id", testSheetID, "--chart-type", "combo", "--data-range", "A1:B4"}},
		{name: "invalid direction", args: []string{"--url", testURL, "--sheet-id", testSheetID, "--chart-type", "line", "--data-range", "A1:C4", "--data-direction", "horizontal"}},
		{name: "colors cannot be empty", args: []string{"--url", testURL, "--sheet-id", testSheetID, "--chart-type", "line", "--data-range", "A1:C4", "--colors", ""}},
		{name: "palette and colors are exclusive", args: []string{"--url", testURL, "--sheet-id", testSheetID, "--chart-type", "line", "--data-range", "A1:C4", "--color-palette", "brandColorSeries@v2", "--colors", "#112233,#445566"}},
		{name: "size must be paired", args: []string{"--url", testURL, "--sheet-id", testSheetID, "--chart-type", "line", "--data-range", "A1:C4", "--width", "640"}},
		{name: "misaligned cross-sheet ranges", args: []string{"--url", testURL, "--sheet-id", testSheetID, "--chart-type", "line", "--data-range", "'A'!A1:A4,'B'!B2:C4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := runShortcutCapturingErr(t, ChartCreateBasic, tt.args)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	body := parseDryRunBody(t, ChartCreateBasic, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-type", "line",
		"--data-range", "A2:C4",
		"--header-range", "'A'!A1,'B'!B1:C1",
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	basic := input["basic_chart"].(map[string]interface{})
	if got, want := basic["header_range"], "'A'!A1,'B'!B1:C1"; got != want {
		t.Fatalf("basic_chart.header_range = %v, want %q", got, want)
	}

	_, _, err := runShortcutCapturingErr(t, ChartConfigUpdate, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-id", "chart-1",
	})
	if err == nil {
		t.Fatal("expected config update with no changed field to fail")
	}

	for _, args := range [][]string{
		{"--url", testURL, "--sheet-id", testSheetID, "--data-range", "A1:C4"},
		{"--url", testURL, "--sheet-id", testSheetID, "--chart-id", "chart-1"},
		{"--url", testURL, "--sheet-id", testSheetID, "--chart-id", "chart-1", "--data-range", "A1:C4", "--data-direction", "horizontal"},
		{"--url", testURL, "--sheet-id", testSheetID, "--chart-id", "chart-1", "--data-range", "A1:C4", "--dim1-index", "0"},
		{"--url", testURL, "--sheet-id", testSheetID, "--chart-id", "chart-1", "--data-range", "A1:C4", "--dim2-indexes", "2,2"},
		{"--url", testURL, "--sheet-id", testSheetID, "--chart-id", "chart-1", "--data-range", "A1:C4", "--dim2-indexes", "1,2"},
	} {
		_, _, err = runShortcutCapturingErr(t, ChartDataUpdate, args)
		if err == nil {
			t.Fatalf("expected chart data update validation error for args %#v", args)
		}
	}
}

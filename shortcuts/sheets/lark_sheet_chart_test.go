// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package sheets

import "testing"

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
	if basic["title"] != "Trend" || basic["legend_position"] != "bottom" || basic["smooth"] != false {
		t.Errorf("semantic config = %#v", basic)
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
	})
	input := decodeToolInput(t, body, "manage_chart_object")
	if input["operation"] != "update" || input["chart_id"] != "chart-1" {
		t.Fatalf("input = %#v", input)
	}
	if _, ok := input["properties"]; ok {
		t.Fatal("semantic update must not send properties")
	}
	updates, _ := input["config_updates"].(map[string]interface{})
	if updates["y_axis_title"] != "Revenue" || updates["stack"] != "percent" || updates["smooth"] != false {
		t.Errorf("config_updates = %#v", updates)
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
		{name: "size must be paired", args: []string{"--url", testURL, "--sheet-id", testSheetID, "--chart-type", "line", "--data-range", "A1:C4", "--width", "640"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := runShortcutCapturingErr(t, ChartCreateBasic, tt.args)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	_, _, err := runShortcutCapturingErr(t, ChartConfigUpdate, []string{
		"--url", testURL,
		"--sheet-id", testSheetID,
		"--chart-id", "chart-1",
	})
	if err == nil {
		t.Fatal("expected config update with no changed field to fail")
	}
}

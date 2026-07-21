// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agents

import (
	"encoding/json"
	"testing"
)

// TestAgentTaskJSON pins the question-group wire shape (design doc §3): group
// label/description, questions[] with question_id/question/options/multi_select,
// option description, and the deleted decision-era fields staying deleted.
func TestAgentTaskJSON(t *testing.T) {
	at := AgentTask{TaskID: "chat_1", ContextID: "sess_1", State: StateInputRequired,
		IsTerminal: false,
		InputRequired: &InputRequired{
			Label:       "报表生成确认",
			Description: "生成前需确认以下口径",
			Questions: []Question{
				{QuestionID: "q1_a8", Question: "按什么维度拆分？",
					Options: []Option{{OptionID: "by_region", Label: "按大区", Description: "华东/华北/华南汇总"}, {OptionID: "by_category", Label: "按品类"}}},
				{QuestionID: "q2_a8", Question: "时间范围？"},
				{QuestionID: "q3_a8", Question: "包含哪些区域？", MultiSelect: true,
					Options: []Option{{OptionID: "east", Label: "华东"}, {OptionID: "north", Label: "华北"}}},
			}}}
	b, _ := json.Marshal(at)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if m["state"] != "input_required" {
		t.Errorf("state=%v", m["state"])
	}
	ir, ok := m["input_required"].(map[string]interface{})
	if !ok {
		t.Fatal("input_required should appear as an object in the input_required state")
	}
	if ir["label"] != "报表生成确认" || ir["description"] != "生成前需确认以下口径" {
		t.Errorf("group label/description should serialize, got %v", ir)
	}
	qs, ok := ir["questions"].([]interface{})
	if !ok || len(qs) != 3 {
		t.Fatalf("questions should serialize as a 3-element array, got %v", ir["questions"])
	}
	q1, _ := qs[0].(map[string]interface{})
	if q1["question_id"] != "q1_a8" || q1["question"] != "按什么维度拆分？" {
		t.Errorf("questions[0] should carry question_id/question, got %v", q1)
	}
	if _, present := q1["multi_select"]; present {
		t.Errorf("false multi_select should be omitted via omitempty, got %v", q1["multi_select"])
	}
	opts, _ := q1["options"].([]interface{})
	if len(opts) != 2 {
		t.Fatalf("questions[0].options should be 2 elements, got %v", q1["options"])
	}
	if o0, _ := opts[0].(map[string]interface{}); o0["option_id"] != "by_region" || o0["label"] != "按大区" || o0["description"] != "华东/华北/华南汇总" {
		t.Errorf("options[0] should be {option_id,label,description}, got %v", opts[0])
	}
	q2, _ := qs[1].(map[string]interface{})
	if _, present := q2["options"]; present {
		t.Errorf("a text question must omit options entirely, got %v", q2["options"])
	}
	q3, _ := qs[2].(map[string]interface{})
	if q3["multi_select"] != true {
		t.Errorf("multi_select=true should serialize, got %v", q3["multi_select"])
	}
	// The decision era is over: no group-level machine id, no arbitration fields.
	for _, gone := range []string{"decision_id", "input_type", "prompt", "submitted", "submitted_option_id"} {
		if _, present := ir[gone]; present {
			t.Errorf("deleted field %q must stay off the wire, got %v", gone, ir[gone])
		}
	}
	// unset artifacts should be omitted via omitempty
	if _, ok := m["artifacts"]; ok {
		t.Error("artifacts should be omitted via omitempty")
	}
}

func TestStructuredArtifactAndInputDetailsJSON(t *testing.T) {
	at := AgentTask{
		TaskID: "task_1",
		State:  StateInputRequired,
		Artifacts: []Artifact{{
			ID: "artifact_1", OutputID: "103:artifact:1", Source: "table_agent", GroupID: "grp_2",
			Kind: "table", Name: "销售表", Status: "ready",
			Data: map[string]interface{}{"resource": map[string]string{"block_id": "block_1"}},
		}},
		InputRequired: &InputRequired{
			Label: "请选择",
			Questions: []Question{{
				QuestionID: "q_1",
				Question:   "选择操作",
				Options:    []Option{{OptionID: "opt_1", Label: "新建", Description: "创建新表"}},
			}},
		},
	}
	b, err := json.Marshal(at)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatal(err)
	}
	artifacts := payload["artifacts"].([]interface{})
	artifact := artifacts[0].(map[string]interface{})
	if artifact["status"] != "ready" || artifact["data"] == nil || artifact["output_id"] != "103:artifact:1" || artifact["source"] != "table_agent" || artifact["group_id"] != "grp_2" {
		t.Fatalf("artifact=%v", artifact)
	}
	input := payload["input_required"].(map[string]interface{})
	questions := input["questions"].([]interface{})
	question := questions[0].(map[string]interface{})
	options := question["options"].([]interface{})
	if input["label"] != "请选择" || question["question_id"] != "q_1" || options[0].(map[string]interface{})["description"] != "创建新表" {
		t.Fatalf("input_required=%v", input)
	}
}

// TestAgentTaskTimestampsJSON pins the added lifecycle timestamps: created_at /
// updated_at are emitted when set and omitted via omitempty when empty.
func TestAgentTaskTimestampsJSON(t *testing.T) {
	b, _ := json.Marshal(AgentTask{TaskID: "chat_1", State: StateCompleted,
		CreatedAt: "2026-07-07T00:00:00Z", UpdatedAt: "2026-07-07T00:01:00Z"})
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if m["created_at"] != "2026-07-07T00:00:00Z" || m["updated_at"] != "2026-07-07T00:01:00Z" {
		t.Errorf("created_at/updated_at should be emitted, got %v", m)
	}

	b, _ = json.Marshal(AgentTask{TaskID: "chat_1", State: StateWorking})
	m = map[string]interface{}{}
	_ = json.Unmarshal(b, &m)
	if _, ok := m["created_at"]; ok {
		t.Error("created_at should be omitted via omitempty when empty")
	}
	if _, ok := m["updated_at"]; ok {
		t.Error("updated_at should be omitted via omitempty when empty")
	}
}

// TestTaskSummaryJSON pins the enriched task-summary shape: updated_at + summary
// are emitted when set and omitted via omitempty when empty.
func TestTaskSummaryJSON(t *testing.T) {
	b, _ := json.Marshal(TaskSummary{TaskID: "chat_1", ContextID: "sess_1",
		State: StateCompleted, IsTerminal: true,
		UpdatedAt: "2026-07-07T00:01:00Z", Summary: "报表已生成"})
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if m["updated_at"] != "2026-07-07T00:01:00Z" {
		t.Errorf("updated_at should be emitted, got %v", m["updated_at"])
	}
	if m["summary"] != "报表已生成" {
		t.Errorf("summary should be emitted, got %v", m["summary"])
	}

	b, _ = json.Marshal(TaskSummary{TaskID: "x", State: StateWorking})
	m = map[string]interface{}{}
	_ = json.Unmarshal(b, &m)
	if _, ok := m["summary"]; ok {
		t.Error("summary should be omitted via omitempty when empty")
	}
	if _, ok := m["updated_at"]; ok {
		t.Error("updated_at should be omitted via omitempty when empty")
	}
}

// TestContextSummaryJSON pins the rollup shape: the summary carries NO
// task_count (the count lives on ContextDetail only); awaiting_input is
// omitted when false; updated_at is carried.
func TestContextSummaryJSON(t *testing.T) {
	b, _ := json.Marshal(ContextSummary{ContextID: "sess_1"})
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if _, ok := m["task_count"]; ok {
		t.Error("ContextSummary must not carry task_count (list-level counts were removed)")
	}
	if _, ok := m["awaiting_input"]; ok {
		t.Error("awaiting_input should be omitted via omitempty when false")
	}

	b, _ = json.Marshal(ContextSummary{ContextID: "sess_1",
		UpdatedAt: "2026-07-07T00:01:00Z", AwaitingInput: true})
	m = map[string]interface{}{}
	_ = json.Unmarshal(b, &m)
	if m["awaiting_input"] != true {
		t.Errorf("awaiting_input should be true, got %v", m["awaiting_input"])
	}
	if m["updated_at"] != "2026-07-07T00:01:00Z" {
		t.Errorf("updated_at should be emitted, got %v", m["updated_at"])
	}
}

// TestContextDetailJSON pins that context detail NO LONGER embeds a full tasks[]:
// it carries task_count + awaiting_input + a single nested active_task (omitted
// when nil). task_count is tri-state: nil = unknown (omitted), &0 = genuinely
// empty, &n = n tasks.
func TestContextDetailJSON(t *testing.T) {
	b, _ := json.Marshal(ContextDetail{ContextID: "sess_1", TaskCount: Int(2), AwaitingInput: true,
		ActiveTask: &TaskSummary{TaskID: "chat_1", State: StateInputRequired, Summary: "按大区还是品类拆?"}})
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if _, ok := m["tasks"]; ok {
		t.Error("ContextDetail must NOT embed a full tasks[] anymore")
	}
	if tc, _ := m["task_count"].(float64); tc != 2 {
		t.Errorf("task_count should be 2, got %v", m["task_count"])
	}
	if m["awaiting_input"] != true {
		t.Errorf("awaiting_input should be true, got %v", m["awaiting_input"])
	}
	at, ok := m["active_task"].(map[string]interface{})
	if !ok {
		t.Fatalf("active_task should be a nested object, got %v", m["active_task"])
	}
	if at["summary"] != "按大区还是品类拆?" {
		t.Errorf("active_task.summary should be carried, got %v", at["summary"])
	}

	// A genuinely empty context keeps an explicit 0 (not conflated with unknown);
	// active_task is omitted; awaiting_input stays omitted when false.
	b, _ = json.Marshal(ContextDetail{ContextID: "empty", TaskCount: Int(0)})
	m = map[string]interface{}{}
	_ = json.Unmarshal(b, &m)
	if tc, ok := m["task_count"].(float64); !ok || tc != 0 {
		t.Errorf("an explicit &0 task_count must stay on the wire as 0, got %v", m["task_count"])
	}
	if _, ok := m["active_task"]; ok {
		t.Error("active_task should be omitted via omitempty when nil")
	}
	if _, ok := m["awaiting_input"]; ok {
		t.Error("awaiting_input should be omitted via omitempty when false")
	}

	// nil task_count = the provider cannot supply the count: the field is
	// omitted entirely, so unknown is never mistaken for an empty context.
	b, _ = json.Marshal(ContextDetail{ContextID: "unknown"})
	m = map[string]interface{}{}
	_ = json.Unmarshal(b, &m)
	if _, ok := m["task_count"]; ok {
		t.Error("a nil TaskCount should omit task_count from the wire (unknown ≠ 0)")
	}
}

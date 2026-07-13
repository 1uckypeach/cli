// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agents

import (
	"encoding/json"
	"testing"
)

func TestAgentTaskJSON(t *testing.T) {
	at := AgentTask{TaskID: "chat_1", ContextID: "sess_1", State: StateInputRequired,
		IsTerminal: false,
		InputRequired: &InputRequired{
			DecisionID: "dec_7f3a",
			Prompt:     "按大区还是品类拆?",
			InputType:  InputTypeSingleSelect,
			Options:    []Option{{OptionID: "by_region", Label: "按大区"}, {OptionID: "by_category", Label: "按品类"}},
		}}
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
	if ir["decision_id"] != "dec_7f3a" || ir["input_type"] != "single_select" {
		t.Errorf("decision_id/input_type should serialize, got %v", ir)
	}
	opts, ok := ir["options"].([]interface{})
	if !ok || len(opts) != 2 {
		t.Fatalf("options should serialize as a 2-element array of {option_id,label}, got %v", ir["options"])
	}
	if o0, _ := opts[0].(map[string]interface{}); o0["option_id"] != "by_region" || o0["label"] != "按大区" {
		t.Errorf("options[0] should be {option_id,label}, got %v", opts[0])
	}
	// submitted is false → omitted via omitempty.
	if _, present := ir["submitted"]; present {
		t.Errorf("unset submitted should be omitted via omitempty, got %v", ir["submitted"])
	}
	// unset artifacts should be omitted via omitempty
	if _, ok := m["artifacts"]; ok {
		t.Error("artifacts should be omitted via omitempty")
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

// TestContextSummaryJSON pins the rollup shape: task_count ALWAYS appears (no
// omitempty, so a zero count stays explicit); awaiting_input is omitted when
// false; updated_at is carried.
func TestContextSummaryJSON(t *testing.T) {
	b, _ := json.Marshal(ContextSummary{ContextID: "sess_1", TaskCount: 0})
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if _, ok := m["task_count"]; !ok {
		t.Error("task_count must always be present (no omitempty), even when 0")
	}
	if _, ok := m["awaiting_input"]; ok {
		t.Error("awaiting_input should be omitted via omitempty when false")
	}

	b, _ = json.Marshal(ContextSummary{ContextID: "sess_1",
		UpdatedAt: "2026-07-07T00:01:00Z", TaskCount: 2, AwaitingInput: true})
	m = map[string]interface{}{}
	_ = json.Unmarshal(b, &m)
	if tc, _ := m["task_count"].(float64); tc != 2 {
		t.Errorf("task_count should be 2, got %v", m["task_count"])
	}
	if m["awaiting_input"] != true {
		t.Errorf("awaiting_input should be true, got %v", m["awaiting_input"])
	}
	if m["updated_at"] != "2026-07-07T00:01:00Z" {
		t.Errorf("updated_at should be emitted, got %v", m["updated_at"])
	}
}

// TestContextDetailJSON pins that context detail NO LONGER embeds a full tasks[]:
// it carries task_count + awaiting_input + a single nested active_task (omitted
// when nil).
func TestContextDetailJSON(t *testing.T) {
	b, _ := json.Marshal(ContextDetail{ContextID: "sess_1", TaskCount: 2, AwaitingInput: true,
		ActiveTask: &TaskSummary{TaskID: "chat_1", State: StateInputRequired, Summary: "按大区还是品类拆?"}})
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if _, ok := m["tasks"]; ok {
		t.Error("ContextDetail must NOT embed a full tasks[] anymore")
	}
	if _, ok := m["task_count"]; !ok {
		t.Error("task_count must always be present")
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

	// active_task is omitted for an empty context; awaiting_input stays omitted when false.
	b, _ = json.Marshal(ContextDetail{ContextID: "empty", TaskCount: 0})
	m = map[string]interface{}{}
	_ = json.Unmarshal(b, &m)
	if _, ok := m["active_task"]; ok {
		t.Error("active_task should be omitted via omitempty when nil")
	}
	if _, ok := m["awaiting_input"]; ok {
		t.Error("awaiting_input should be omitted via omitempty when false")
	}
}

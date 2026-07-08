// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"encoding/json"
	"testing"
)

func TestAgentTaskJSON(t *testing.T) {
	at := AgentTask{TaskID: "chat_1", ContextID: "sess_1", State: StateInputRequired,
		IsTerminal:    false,
		InputRequired: &InputRequired{Prompt: "按大区还是品类拆?", Options: []string{"region", "category"}}}
	b, _ := json.Marshal(at)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if m["state"] != "input_required" {
		t.Errorf("state=%v", m["state"])
	}
	if _, ok := m["input_required"]; !ok {
		t.Error("input_required should appear in the input_required state")
	}
	// unset artifacts should be omitted via omitempty
	if _, ok := m["artifacts"]; ok {
		t.Error("artifacts should be omitted via omitempty")
	}
}

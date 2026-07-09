// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package affordance

import (
	"encoding/json"
	"os"
	"testing"
)

// The 21 im raw-API methods that affordance/im.md must cover: 17 first-batch
// methods plus 4 "prefer the shortcut" entries. Keys follow the parsed heading
// form (spaces become dots), same as TestFor's fixture keys.
var imAffordanceMethods = []string{
	"chat.members.create", "chat.members.delete", "chat.members.get", "chat.members.bots",
	"messages.forward", "messages.delete", "messages.merge_forward", "messages.read_users",
	"reactions.create", "reactions.delete", "reactions.list", "reactions.batch_query",
	"pins.create", "pins.delete", "pins.list",
	"images.create",
	"threads.forward",
	"chats.get", "chats.update", "chats.create", "chats.link",
}

type parsedAffordance struct {
	UseWhen       []string `json:"use_when"`
	AvoidWhen     []string `json:"avoid_when"`
	Prerequisites []string `json:"prerequisites"`
	Examples      []struct {
		Command string `json:"command"`
	} `json:"examples"`
}

// TestForIMRealFile parses the real affordance/im.md through the production
// parser and asserts coverage plus depth on the showcase method.
func TestForIMRealFile(t *testing.T) {
	prev := mdSource
	t.Cleanup(func() { SetSource(prev) })
	SetSource(os.DirFS("../../affordance"))

	for _, m := range imAffordanceMethods {
		raw, ok := For("im", m)
		if !ok {
			t.Errorf("For(\"im\", %q) ok=false, want an overlay section in affordance/im.md", m)
			continue
		}
		var a parsedAffordance
		if err := json.Unmarshal(raw, &a); err != nil {
			t.Errorf("%s: overlay is not valid affordance JSON: %v", m, err)
			continue
		}
		if len(a.UseWhen) == 0 {
			t.Errorf("%s: missing lead paragraph (use_when)", m)
		}
		if len(a.AvoidWhen) == 0 {
			t.Errorf("%s: missing Avoid when section", m)
		}
	}

	// Showcase depth: messages forward (spec §3.3, mirrors the tech-plan diff).
	raw, ok := For("im", "messages.forward")
	if !ok {
		t.Fatal("messages.forward overlay missing")
	}
	var fwd parsedAffordance
	if err := json.Unmarshal(raw, &fwd); err != nil {
		t.Fatalf("messages.forward overlay invalid: %v", err)
	}
	if len(fwd.AvoidWhen) < 3 {
		t.Errorf("messages.forward: want >=3 avoid_when entries, got %d", len(fwd.AvoidWhen))
	}
	if len(fwd.Prerequisites) < 2 {
		t.Errorf("messages.forward: want >=2 prerequisites, got %d", len(fwd.Prerequisites))
	}
	if len(fwd.Examples) < 1 || fwd.Examples[0].Command == "" {
		t.Errorf("messages.forward: want >=1 fenced example command")
	}
}

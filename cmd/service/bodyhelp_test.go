// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/meta"
	"github.com/spf13/cobra"
)

// methodCmdWithBodyForTest registers a method that declares body fields and
// returns its command, so the lazy help rebuild can be exercised end to end.
func methodCmdWithBodyForTest(t *testing.T) *cobra.Command {
	t.Helper()
	parent := &cobra.Command{Use: "root"}
	svc := meta.ServiceFromMap(map[string]interface{}{
		"name":        "im",
		"description": "Messaging API",
		"servicePath": "/open-apis/im/v1",
		"resources": map[string]interface{}{
			"messages": map[string]interface{}{
				"methods": map[string]interface{}{
					"create": map[string]interface{}{
						"description": "发送消息",
						"httpMethod":  "POST",
						"requestBody": map[string]interface{}{
							"receive_id": map[string]interface{}{
								"type": "string", "required": true, "description": "消息接收者的 ID",
							},
							"content": map[string]interface{}{
								"type": "string", "required": true, "description": "消息内容",
							},
						},
					},
				},
			},
		},
	})
	registerService(parent, svc, &cmdutil.Factory{})

	cmd, _, err := parent.Find([]string{"im", "messages", "create"})
	if err != nil {
		t.Fatalf("method command not registered: %v", err)
	}
	return cmd
}

func TestBodyHelp_RendersSkeletonAndFacts(t *testing.T) {
	fields := []meta.Field{
		{Name: "receive_id", Type: "string", Required: true, Description: "消息接收者的 ID。Identity: tail"},
		{Name: "content", Type: "string", Required: true, Description: "消息内容"},
		{Name: "uuid", Type: "string", Description: "幂等 uuid"},
	}
	got := bodyHelp(fields)
	for _, want := range []string{
		"Request body (--data",
		`"receive_id"`,
		"required",
		"消息接收者的 ID", // first sentence only — the Identity tail must be cut
	} {
		if !strings.Contains(got, want) {
			t.Errorf("bodyHelp must contain %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Identity:") {
		t.Error("descriptions must be first-sentence only")
	}
	if !strings.Contains(got, "optional") {
		t.Error("a non-required field must be marked optional")
	}
}

func TestBodyHelp_EmptyForNoBody(t *testing.T) {
	if got := bodyHelp(nil); got != "" {
		t.Errorf("no body fields must render nothing, got %q", got)
	}
}

func TestBodyHelp_NestedFieldsOneLevel(t *testing.T) {
	fields := []meta.Field{{
		Name: "filter", Type: "object", Description: "过滤条件",
		Properties: map[string]meta.Field{
			"user_ids": {Name: "user_ids", Type: "array", Description: "用户 ID 列表"},
		},
	}}
	got := bodyHelp(fields)
	if !strings.Contains(got, "user_ids") {
		t.Errorf("nested fields must be listed, got:\n%s", got)
	}
}

// The skeleton is meant to be copied straight into --data, so it has to parse.
func TestBodyHelp_SkeletonIsValidJSON(t *testing.T) {
	fields := []meta.Field{
		{Name: "receive_id", Type: "string", Required: true},
		{Name: "count", Type: "int", Required: false},
		{Name: "enabled", Type: "boolean"},
		{Name: "items", Type: "array"},
		{Name: "filter", Type: "object", Properties: map[string]meta.Field{
			"user_ids": {Name: "user_ids", Type: "array"},
		}},
	}
	skeleton := bodySkeleton(fields)
	var into map[string]any
	if err := json.Unmarshal([]byte(skeleton), &into); err != nil {
		t.Fatalf("skeleton is not valid JSON: %v\n%s", err, skeleton)
	}
	for _, key := range []string{"receive_id", "count", "enabled", "items", "filter"} {
		if _, ok := into[key]; !ok {
			t.Errorf("skeleton missing field %q: %s", key, skeleton)
		}
	}
}

// Field names reach the skeleton from upstream metadata, so a name carrying a
// quote or control character must not be able to break the JSON shape.
func TestBodyHelp_SkeletonEscapesFieldNames(t *testing.T) {
	skeleton := bodySkeleton([]meta.Field{{Name: `we"ird` + "\n", Type: "string"}})
	var into map[string]any
	if err := json.Unmarshal([]byte(skeleton), &into); err != nil {
		t.Fatalf("skeleton with a quoted field name must stay valid JSON: %v\n%s", err, skeleton)
	}
}

// The facts line renders name, type and description from the same upstream
// document; cleaning only the description would leave the row forgeable through
// the other two.
func TestBodyHelp_SanitizesNameAndType(t *testing.T) {
	got := bodyHelp([]meta.Field{{
		Name: "na\x1b[31mme", Type: "str\x1b[0ming", Description: "desc",
	}})
	if strings.Contains(got, "\x1b") {
		t.Errorf("facts line must not carry escape sequences, got %q", got)
	}
	if !strings.Contains(got, "name") {
		t.Errorf("sanitized name must survive, got %q", got)
	}
}

// The skeleton and the facts line below it name the same fields, so they have to
// sanitize identically. json.Marshal alone escapes C0 and quotes but passes bidi
// controls and zero-width characters, which would make the two lines disagree.
func TestBodyHelp_SkeletonSanitizesInvisibleChars(t *testing.T) {
	for name, raw := range map[string]string{
		"bidi override":    "user_id‮",
		"zero width":       "user​id",
		"C1 control":       "userid",
		"arabic letter mk": "user؜id",
	} {
		got := bodyHelp([]meta.Field{{Name: raw, Type: "string"}})
		for _, bad := range []string{"‮", "​", "", "؜"} {
			if strings.Contains(got, bad) {
				t.Errorf("%s: output must not carry %q, got %q", name, bad, got)
			}
		}
	}
	// Two fields differing only by an invisible character must not render as two
	// identical skeleton keys — sanitizing collapses them, so the shape stays
	// honest about there being two distinct fields only if both are cleaned the
	// same way.
	same := bodySkeleton([]meta.Field{{Name: "user_id", Type: "string"}})
	withBidi := bodySkeleton([]meta.Field{{Name: "user_id‮", Type: "string"}})
	if same != withBidi {
		t.Errorf("skeleton must sanitize the key: %q vs %q", same, withBidi)
	}
}

// bodyHelp has to survive PrepareMethodHelp's lazy Long rebuild — that path
// recomposes from annotations, so a body section only written at build time
// would vanish the moment help is actually rendered.
func TestPrepareMethodHelp_KeepsBodyContract(t *testing.T) {
	cmd := methodCmdWithBodyForTest(t)
	if !PrepareMethodHelp(cmd, nil) {
		t.Fatal("PrepareMethodHelp must apply to a method command")
	}
	if !strings.Contains(cmd.Long, "Request body (--data") {
		t.Errorf("rebuilt Long must keep the request body contract, got:\n%s", cmd.Long)
	}
	if !strings.Contains(cmd.Long, "Full parameter schema") {
		t.Error("rebuilt Long must keep the schema pointer")
	}
}

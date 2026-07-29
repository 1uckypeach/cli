// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package schema

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/larksuite/cli/internal/core"
)

func TestRunSchema_BareRendersServiceIndex(t *testing.T) {
	var buf bytes.Buffer
	if err := runSchema(&buf, nil, core.StrictModeOff, ""); err != nil {
		t.Fatalf("runSchema: %v", err)
	}
	var got struct {
		Kind     string `json:"kind"`
		Services []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"services"`
		Hint string `json:"hint"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got.Kind != "service_index" {
		t.Errorf("kind = %q, want service_index", got.Kind)
	}
	if len(got.Services) == 0 {
		t.Fatal("services must not be empty")
	}
	if got.Hint == "" {
		t.Error("hint must be present")
	}
	// A method count would force parsing every service's metadata, which is the
	// cost this index exists to avoid.
	if bytes.Contains(buf.Bytes(), []byte(`"methods"`)) {
		t.Error("service_index must not carry a method count")
	}
}

func TestRunSchema_ServiceRendersMethodIndex(t *testing.T) {
	var buf bytes.Buffer
	if err := runSchema(&buf, []string{"im"}, core.StrictModeOff, ""); err != nil {
		t.Fatalf("runSchema: %v", err)
	}
	var got struct {
		Kind    string `json:"kind"`
		Service string `json:"service"`
		Methods []struct {
			Path         string   `json:"path"`
			Description  string   `json:"description"`
			Risk         string   `json:"risk"`
			AccessTokens []string `json:"access_tokens"`
		} `json:"methods"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got.Kind != "method_index" || got.Service != "im" {
		t.Errorf("kind/service = %q/%q, want method_index/im", got.Kind, got.Service)
	}
	if bytes.Contains(buf.Bytes(), []byte(`"path": "im"`)) {
		t.Error(`top-level field must be "service", not "path"`)
	}
	if len(got.Methods) == 0 {
		t.Fatal("im must expose methods")
	}
	for i, m := range got.Methods {
		if m.Path == "" || m.Risk == "" {
			t.Errorf("methods[%d]: path and risk must be set, got %+v", i, m)
		}
		if i > 0 && got.Methods[i-1].Path >= m.Path {
			t.Errorf("methods must be sorted by path: %q >= %q", got.Methods[i-1].Path, m.Path)
		}
	}
	// The index exists to fit an agent's single-response budget.
	if buf.Len() > 16*1024 {
		t.Errorf("method_index for im is %d bytes, want <= 16KB", buf.Len())
	}
}

func TestRunSchema_ResourceRendersMethodIndex(t *testing.T) {
	var buf bytes.Buffer
	if err := runSchema(&buf, []string{"im", "chat.members"}, core.StrictModeOff, ""); err != nil {
		t.Fatalf("runSchema: %v", err)
	}
	var got struct {
		Kind    string `json:"kind"`
		Service string `json:"service"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got.Kind != "method_index" || got.Service != "im" {
		t.Errorf("kind/service = %q/%q, want method_index/im", got.Kind, got.Service)
	}
}

func TestRunSchema_MethodOutputUnchanged(t *testing.T) {
	var buf bytes.Buffer
	if err := runSchema(&buf, []string{"im", "chat.members", "get"}, core.StrictModeOff, ""); err != nil {
		t.Fatalf("runSchema: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	// Still the envelope, not an index.
	for _, key := range []string{"name", "description", "inputSchema", "outputSchema", "_meta"} {
		if _, ok := got[key]; !ok {
			t.Errorf("envelope must keep field %q", key)
		}
	}
	if _, ok := got["kind"]; ok {
		t.Error("method output must not gain a kind field")
	}
}

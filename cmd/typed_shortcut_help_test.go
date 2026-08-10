// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/cmdutil"
)

var wikiNodeGetLegacyTips = []string{
	"--node-token accepts a raw wiki node_token, obj_token, or a Lark URL like https://feishu.cn/wiki/<token> or https://feishu.cn/docx/<token>.",
	"For raw obj_tokens, pass --obj-type so the API knows how to resolve them; URL inputs infer it from the path.",
	"Pair with +move / +node-copy / +delete-space to confirm space_id, obj_type, and parent before mutating.",
	"--token is the deprecated original name and still works for backward compatibility; new scripts should use --node-token.",
}

func TestTypedShortcutHelpStructureAndBudget(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	root := Build(context.Background(), cmdutil.InvocationContext{}, WithoutPlugins())
	cases := []struct {
		path    string
		snippet string
	}{
		{"note +detail", "--note-id"},
		{"wiki +node-get", "--node-token"},
		{"sheets +csv-put", "--csv"},
		{"task +tasklist-task-add", "--tasklist-id"},
		{"slides +screenshot", "--presentation"},
		{"application +slash-command-delete", "--command-id"},
	}
	sizes := make([]int, 0, len(cases))
	for _, tt := range cases {
		t.Run(tt.path, func(t *testing.T) {
			command := findCommand(root, tt.path)
			if command == nil {
				t.Fatalf("command %q not found", tt.path)
			}
			command.InitDefaultHelpFlag()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			if err := command.Help(); err != nil {
				t.Fatal(err)
			}
			got := output.String()
			for _, required := range []string{tt.snippet, "Execution:", "Output:"} {
				if !strings.Contains(got, required) {
					t.Fatalf("help for %s is missing %q:\n%s", tt.path, required, got)
				}
			}
			if len(got) > 8*1024 {
				t.Fatalf("help size = %d bytes, limit = 8192", len(got))
			}
			sizes = append(sizes, len(got))
		})
	}
	sort.Ints(sizes)
	if len(sizes) > 0 {
		p95 := sizes[(95*len(sizes)-1)/100]
		if p95 > 4*1024 {
			t.Fatalf("typed help P95 = %d bytes, limit = 4096", p95)
		}
	}
}

func TestWikiNodeGetTypedHelpPreservesLegacyTips(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	root := Build(context.Background(), cmdutil.InvocationContext{}, WithoutPlugins())
	command := findCommand(root, "wiki +node-get")
	if command == nil {
		t.Fatal("wiki +node-get not found")
	}
	command.InitDefaultHelpFlag()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	if err := command.Help(); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, "Tips:") {
		t.Fatalf("help is missing Tips section:\n%s", got)
	}
	for _, tip := range wikiNodeGetLegacyTips {
		if !strings.Contains(got, tip) {
			t.Fatalf("help is missing legacy tip %q:\n%s", tip, got)
		}
	}
}

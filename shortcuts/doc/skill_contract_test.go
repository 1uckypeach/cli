// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"os"
	"strings"
	"testing"
)

func TestLarkDocSkillRecoveryContracts(t *testing.T) {
	tests := []struct {
		path  string
		wants []string
	}{
		{
			path: "../../skills/lark-doc/references/lark-doc-fetch.md",
			wants: []string{
				"`full`（默认）\\| `outline`",
				"省略等价于 `--scope full`",
				"`--scope` 决定读取范围，`--detail` 决定返回详细度",
			},
		},
		{
			path:  "../../skills/lark-doc/references/lark-doc-create-workflow.md",
			wants: []string{"`network/timeout`", "不得直接重跑 `+create`"},
		},
		{
			path:  "../../skills/lark-doc/references/lark-doc-update.md",
			wants: []string{"`network/timeout`", "不得直接重放原命令"},
		},
		{
			path:  "../../skills/lark-doc/SKILL.md",
			wants: []string{"`3380002`", "`3380004`", "不要根据域名"},
		},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			data, err := os.ReadFile(test.path)
			if err != nil {
				t.Fatalf("ReadFile(%s): %v", test.path, err)
			}
			text := string(data)
			for _, want := range test.wants {
				if !strings.Contains(text, want) {
					t.Errorf("%s missing %q", test.path, want)
				}
			}
		})
	}
}

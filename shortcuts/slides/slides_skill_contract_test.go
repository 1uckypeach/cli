// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slides

import (
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/vfs"
)

func TestSlidesSkillStopsRetryAfterMissingScope(t *testing.T) {
	readNormalized := func(path string) string {
		t.Helper()
		content, err := vfs.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return strings.Join(strings.Fields(string(content)), " ")
	}

	skill := readNormalized("../../skills/lark-slides/SKILL.md")
	for _, want := range []string{
		"error.subtype == missing_scope",
		"code == 99991679",
		"立刻停止本轮所有后续 slides shortcut 和直接 OpenAPI",
		"截图失败后的 `+xml-get` / GET 演示对象",
		"missing_scopes",
		"禁止 `auth login`",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("lark-slides skill missing %q", want)
		}
	}

	errorsDoc := readNormalized("../../skills/lark-slides/references/workflow/error-handling.md")
	for _, want := range []string{
		"`99991679` / `missing_scope`",
		"slides:presentation:screenshot",
		"slides:presentation:read",
		"slides:presentation:create",
		"不要降级 `+xml-get` / GET",
		"不要用 `auth login --scope` 试错",
	} {
		if !strings.Contains(errorsDoc, want) {
			t.Fatalf("slides error-handling missing %q", want)
		}
	}

	screenshot := readNormalized("../../skills/lark-slides/references/cli/lark-slides-screenshot.md")
	if !strings.Contains(screenshot, "只有非授权类错误才降级") {
		t.Fatal("screenshot doc must not fall back to XML after missing_scope")
	}
	if !strings.Contains(screenshot, "停止全部剩余批次") {
		t.Fatal("screenshot doc must stop remaining batches after missing_scope")
	}
	if !strings.Contains(screenshot, "missing_scope") || !strings.Contains(screenshot, "99991679") {
		t.Fatal("screenshot doc must name missing_scope / 99991679 as terminal")
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slides

import (
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/vfs"
)

func TestSlidesScreenshotDoesNotFallBackAfterMissingScope(t *testing.T) {
	content, err := vfs.ReadFile("../../skills/lark-slides/references/cli/lark-slides-screenshot.md")
	if err != nil {
		t.Fatalf("read screenshot skill: %v", err)
	}
	doc := strings.Join(strings.Fields(string(content)), " ")
	if strings.Contains(doc, "截图失败则降级到 XML") {
		t.Fatal("screenshot doc must not fall back to XML after every failure")
	}
	if !strings.Contains(doc, "missing_scope") || !strings.Contains(doc, "不要降级到 `+xml-get`") {
		t.Fatal("screenshot doc must keep missing_scope from falling back to +xml-get")
	}
}

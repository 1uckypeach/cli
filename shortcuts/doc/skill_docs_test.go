// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"os"
	"strings"
	"testing"
)

func readDocSkillFile(t *testing.T, relPath string) string {
	t.Helper()
	raw, err := os.ReadFile("../../skills/lark-doc/" + relPath)
	if err != nil {
		t.Fatalf("read lark-doc skill file %s: %v", relPath, err)
	}
	return string(raw)
}

func TestDocSkillKeepsAgentDraftsOutOfShellTextPipelines(t *testing.T) {
	skill := readDocSkillFile(t, "SKILL.md")
	for _, contract := range []string{
		"文件创建/编辑工具和 `lark-cli` 进程必须以同一目录为根",
		"必须保存为 UTF-8 文件，并通过相对 `@./path` 传给 CLI",
		"Windows 下禁止用 `Get-Content`、`Out-String`",
		"不作为 Agent 的文件路径错误恢复方案",
	} {
		if !strings.Contains(skill, contract) {
			t.Errorf("skills/lark-doc/SKILL.md must contain %q", contract)
		}
	}

	createWorkflow := readDocSkillFile(t, "references/lark-doc-create-workflow.md")
	for _, contract := range []string{
		"`workdir` / `cwd` 在该目录启动",
		"--presentation-decision \"@./<decision_input_path>\"",
		"不得使用 PowerShell 文本管道或 `--content -` 代替",
		"核对标题和至少一段包含非 ASCII 字符的代表性正文",
	} {
		if !strings.Contains(createWorkflow, contract) {
			t.Errorf("lark-doc-create-workflow.md must contain %q", contract)
		}
	}

	createReference := readDocSkillFile(t, "references/lark-doc-create.md")
	if strings.Contains(createReference, "简单内容优先使用 `--content -`") {
		t.Error("lark-doc-create.md must not steer agents to stdin for simple content")
	}

	markdownReference := readDocSkillFile(t, "references/lark-doc-md.md")
	if !strings.Contains(markdownReference, "`-`（读 stdin）仅供能直接提供原始 UTF-8 字节的宿主调用方使用") {
		t.Error("lark-doc-md.md must reserve stdin for hosts that provide raw UTF-8 bytes")
	}
}

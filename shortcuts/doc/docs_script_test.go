// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/shortcuts/doc/internal/docxparse"
)

func TestDocsScriptParsesAndProfilesXML(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, docsTestConfigWithAppID("docs-script-test"))
	source := `<title>标题</title><p>一个苹果是 an apple。</p>`

	err := mountAndRunDocs(t, DocsScript, []string{
		"+script",
		"--command", docsScriptParse,
		"--content", source,
		"--as", "bot",
	}, f, stdout)
	if err != nil {
		t.Fatalf("execute docs +script: %v", err)
	}

	var envelope struct {
		OK   bool                       `json:"ok"`
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout)
	}
	if !envelope.OK {
		t.Fatalf("ok = false: %s", stdout)
	}
	if len(envelope.Data) != 1 || envelope.Data["profile"] == nil {
		t.Fatalf("data = %+v, want only profile", envelope.Data)
	}
	var profile docsScriptPublicProfile
	if err := json.Unmarshal(envelope.Data["profile"], &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	var profileFields map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Data["profile"], &profileFields); err != nil {
		t.Fatalf("decode profile fields: %v", err)
	}
	if len(profileFields) != 4 || profileFields["breakdown"] != nil {
		t.Fatalf("profile fields = %+v, want breakdown hidden", profileFields)
	}
	if profile.WordCount != 10 || profile.CharCount != 15 || profile.BlockCount != 2 {
		t.Fatalf("profile = %+v", profile)
	}
	if got := blockCount(profile.Blocks, "title"); got != 1 {
		t.Fatalf("title count = %d, want 1", got)
	}
	if got := blockCount(profile.Blocks, "p"); got != 1 {
		t.Fatalf("p count = %d, want 1", got)
	}
}

func TestDocsScriptParseAutoDetectsMarkdown(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, docsTestConfigWithAppID("docs-script-auto-markdown"))

	err := mountAndRunDocs(t, DocsScript, []string{
		"+script",
		"--command", docsScriptParse,
		"--content", "# 标题\n\n- item",
		"--as", "bot",
	}, f, stdout)
	if err != nil {
		t.Fatalf("execute docs +script: %v", err)
	}

	var envelope struct {
		Data docsScriptParseResult `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout)
	}
	if envelope.Data.Profile.BlockCount != 3 {
		t.Fatalf("profile = %+v, want 3 blocks", envelope.Data.Profile)
	}
}

func TestDocsScriptConvertsMarkdownFromStdin(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, docsTestConfigWithAppID("docs-script-markdown"))
	f.IOStreams.In = bytes.NewBufferString("# 标题\n\n- item")

	err := mountAndRunDocs(t, DocsScript, []string{
		"+script",
		"--command", docsScriptMarkdownToXML,
		"--content", "-",
		"--as", "bot",
	}, f, stdout)
	if err != nil {
		t.Fatalf("execute docs +script: %v", err)
	}
	if !strings.Contains(stdout.String(), `<h1>标题</h1><ul><li>item</li></ul>`) {
		t.Fatalf("stdout missing converted XML: %s", stdout)
	}
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout)
	}
	if len(envelope.Data) != 1 || envelope.Data["xml"] == nil {
		t.Fatalf("data = %+v, want only xml", envelope.Data)
	}
}

func TestDocsScriptDryRunHasNoAPICall(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, docsTestConfigWithAppID("docs-script-dry-run"))

	err := mountAndRunDocs(t, DocsScript, []string{
		"+script",
		"--command", docsScriptParse,
		"--content", `<p>text</p>`,
		"--dry-run",
		"--as", "bot",
	}, f, stdout)
	if err != nil {
		t.Fatalf("execute docs +script dry-run: %v", err)
	}
	var got struct {
		API     []any  `json:"api"`
		Command string `json:"command"`
		Network bool   `json:"network"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode dry-run stdout: %v\n%s", err, stdout)
	}
	if len(got.API) != 0 || got.Command != docsScriptParse || got.Network {
		t.Fatalf("dry-run output = %+v", got)
	}
}

func TestDocsScriptReturnsTypedParseError(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, docsTestConfigWithAppID("docs-script-error"))

	err := mountAndRunDocs(t, DocsScript, []string{
		"+script",
		"--command", docsScriptParse,
		"--content", `<!DOCTYPE document><p>text</p>`,
		"--as", "bot",
	}, f, nil)
	if err == nil {
		t.Fatal("expected parse error")
	}
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != errs.CategoryValidation || problem.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("problem = %+v, ok=%v", problem, ok)
	}
	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Param != "--content" {
		t.Fatalf("error = %#v, want --content metadata", err)
	}
}

func TestDocsScriptRejectsMalformedXML(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, docsTestConfigWithAppID("docs-script-malformed"))

	err := mountAndRunDocs(t, DocsScript, []string{
		"+script",
		"--command", docsScriptParse,
		"--content", `<p>text`,
		"--as", "bot",
	}, f, nil)
	if err == nil {
		t.Fatal("expected malformed XML error")
	}
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != errs.CategoryValidation || problem.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("problem = %+v, ok=%v", problem, ok)
	}
}

func TestDocsScriptHelpExamplesAreCrossShellSafe(t *testing.T) {
	cmd := &cobra.Command{Short: "local document parser"}
	installDocsScriptHelp(cmd)
	if strings.Contains(cmd.Example, "cat ") {
		t.Fatalf("help examples require a platform-specific command: %q", cmd.Example)
	}
	if strings.Contains(cmd.Example, "--content @") {
		t.Fatalf("help examples contain an unquoted @file argument: %q", cmd.Example)
	}
	for _, want := range []string{`--content "@draft.xml"`, `--content "@draft.md"`} {
		if !strings.Contains(cmd.Example, want) {
			t.Errorf("help examples missing %q: %q", want, cmd.Example)
		}
	}
}

func blockCount(blocks []docxparse.BlockShare, typ string) int {
	for _, block := range blocks {
		if block.Type == typ {
			return block.Count
		}
	}
	return 0
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
	"github.com/larksuite/cli/shortcuts/doc/internal/docxparse"
)

const (
	docsScriptParse         = "parse"
	docsScriptMarkdownToXML = "markdown-to-xml"
)

var DocsScript = common.Shortcut{
	Service:     "docs",
	Command:     "+script",
	Description: "Parse and profile XML or Markdown, or convert Markdown to LarkOpenCLI XML",
	Risk:        "read",
	AuthTypes:   []string{"user", "bot"},
	Scopes:      []string{},
	Flags: []common.Flag{
		{
			Name:     "command",
			Desc:     "local document operation",
			Required: true,
			Enum:     []string{docsScriptParse, docsScriptMarkdownToXML},
		},
		{
			Name:     "content",
			Desc:     "document content; use @relative-file or - for stdin",
			Required: true,
			Input:    []string{common.File, common.Stdin},
		},
	},
	Tips: []string{
		"parse auto-detects XML or Markdown and returns only the text and block profile",
		"markdown-to-xml converts Markdown to LarkOpenCLI XML",
	},
	PostMount: installDocsScriptHelp,
	Validate:  validateDocsScript,
	DryRun:    dryRunDocsScript,
	Execute:   executeDocsScript,
}

type docsScriptParseResult struct {
	Profile docsScriptPublicProfile `json:"profile"`
}

// docsScriptPublicProfile is the stable shortcut response. The parser keeps
// the more detailed breakdown internally so it can be exposed later without
// changing the counting implementation.
type docsScriptPublicProfile struct {
	WordCount  int                    `json:"word_count"`
	CharCount  int                    `json:"char_count"`
	BlockCount int                    `json:"block_count"`
	Blocks     []docxparse.BlockShare `json:"blocks"`
}

type docsScriptMarkdownResult struct {
	XML string `json:"xml"`
}

func installDocsScriptHelp(cmd *cobra.Command) {
	installDocsShortcutHelp("+script")(cmd)
	cmd.Example = `  lark-cli docs +script --command parse --content "@draft.xml"
  lark-cli docs +script --command parse --content "@draft.md"
  lark-cli docs +script --command markdown-to-xml --content "@draft.md"`
}

func validateDocsScript(_ context.Context, runtime *common.RuntimeContext) error {
	if strings.TrimSpace(runtime.Str("content")) == "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--content cannot be empty").
			WithParam("--content")
	}
	return nil
}

func dryRunDocsScript(_ context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
	return common.NewDryRunAPI().
		Desc("Local LarkOpenCLI document parsing or conversion; no API call is made").
		Set("command", runtime.Str("command")).
		Set("input_bytes", len(runtime.Str("content"))).
		Set("network", false)
}

func executeDocsScript(_ context.Context, runtime *common.RuntimeContext) error {
	command := runtime.Str("command")
	content := runtime.Str("content")
	switch command {
	case docsScriptParse:
		profile, err := docxparse.ParseAuto(content)
		if err != nil {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"could not parse --content as LarkOpenCLI XML or Markdown: %s", err).
				WithParam("--content").
				WithCause(err)
		}
		runtime.OutFormatRaw(docsScriptParseResult{Profile: docsScriptPublicProfile{
			WordCount:  profile.WordCount,
			CharCount:  profile.CharCount,
			BlockCount: profile.BlockCount,
			Blocks:     profile.Blocks,
		}}, nil, nil)
		return nil
	case docsScriptMarkdownToXML:
		xml, err := docxparse.MarkdownToXML(content)
		if err != nil {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"could not convert --content from Markdown to LarkOpenCLI XML: %s", err).
				WithParam("--content").
				WithCause(err)
		}
		runtime.OutFormatRaw(docsScriptMarkdownResult{XML: xml}, nil, nil)
		return nil
	default:
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"unsupported --command %q", command).
			WithParam("--command")
	}
}

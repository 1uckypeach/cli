// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/output"
)

// cardOptions holds all inputs for `agent card <ref>`.
type cardOptions struct {
	Factory *cmdutil.Factory
	Cmd     *cobra.Command
	Ref     string
	As      string
	Format  string
}

// NewCmdAgentCard builds `agent card <ref>`: fetch and display an agent's
// capability card. Adapters synthesize the card statically from their known
// capability matrix — no API call is made, and the command works offline /
// under mock. Risk=read.
func NewCmdAgentCard(f *cmdutil.Factory) *cobra.Command {
	opts := &cardOptions{Factory: f}
	cmd := &cobra.Command{
		Use:   "card <agent_ref>",
		Short: "Show a remote agent's capability card (capabilities / parameters / identity)",
		Long:  "Fetch and show an agent's capability card. Use its capabilities to decide which verbs are available and its parameters to decide the --param a send needs. Some providers synthesize the card statically without calling the remote API.",
		Args:  exactArgsWithUsage(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(opts.Format); err != nil {
				return err
			}
			opts.Cmd = cmd
			opts.Ref = args[0]
			return agentCardRun(opts)
		},
	}
	cmd.Flags().StringVar(&opts.Format, "format", "json", formatFlagHelp)
	cmd.Flags().String("jq", "", "用 jq 表达式过滤 JSON 输出")
	if f != nil {
		cmdutil.AddAPIIdentityFlag(cmd.Context(), cmd, f, &opts.As)
	} else {
		// f is nil only in construction-time unit tests; register a bare --as so
		// the flag surface is still assertable without a Factory.
		cmd.Flags().StringVar(&opts.As, "as", "", "identity type: user | bot")
	}
	cmdutil.SetRisk(cmd, cmdutil.RiskRead)
	return cmd
}

// agentCardRun resolves the provider addressed by ref and emits its capability
// card. The card is first-party static data (not agent-generated content), so
// it bypasses content-safety scanning. The JSON success envelope is the
// default; --format pretty opts into the human-readable listing. A --jq
// expression forces JSON (jq operates on the envelope) and, when present,
// filters stdout.
func agentCardRun(opts *cardOptions) error {
	f := opts.Factory
	// Card synthesis is API-free, so resolve without requiring a
	// configured client: `agent card` must work offline / before config init.
	p, id, err := resolveProviderNoClient(f, opts.Cmd, opts.Ref, opts.As)
	if err != nil {
		return err
	}
	card, err := p.Card(opts.Cmd.Context())
	if err != nil {
		return err
	}

	jq := jqExpr(opts.Cmd)
	// pretty is a human view only; a --jq expression implies structured JSON,
	// so it takes precedence over the pretty format.
	if opts.Format == "pretty" && jq == "" {
		printCardPretty(f.IOStreams.Out, card)
		return nil
	}

	env := output.Envelope{
		OK:       true,
		Identity: string(id),
		Data:     card,
		Notice:   output.GetNotice(),
	}
	if jq != "" {
		return output.JqFilter(f.IOStreams.Out, env, jq)
	}
	output.PrintJson(f.IOStreams.Out, env)
	return nil
}

// printCardPretty writes a compact human-readable view of an agent card:
// identity header (with per-identity preconditions), the sorted capability
// matrix, declared parameters and skills — the key constraints an AI reads
// from json must also be visible to a human. Remote cards carry
// agent-controlled Name/Description/Desc
// strings, so every such field is ANSI-stripped before hitting the terminal.
// Nil cards degrade to a placeholder line rather than panicking.
func printCardPretty(w io.Writer, card *iagent.AgentCard) {
	if card == nil {
		fmt.Fprintln(w, "(no card)")
		return
	}
	// Dynamic cards carry a Name; static cards fall back to the provider label.
	name := card.Name
	if name == "" {
		name = card.ProviderLabel
	}
	fmt.Fprintf(w, "%s (%s)\n", stripANSI(name), card.AgentID)
	if card.Description != "" {
		fmt.Fprintf(w, "  %s\n", stripANSI(card.Description))
	}
	if len(card.Identity) > 0 {
		ids := make([]string, 0, len(card.Identity))
		for _, spec := range card.Identity {
			id := string(spec.Type)
			if spec.Precondition != "" {
				id += "（前置: " + stripANSI(spec.Precondition) + "）"
			}
			ids = append(ids, id)
		}
		fmt.Fprintf(w, "  identity: %s\n", strings.Join(ids, ", "))
	}

	fmt.Fprintln(w, "  capabilities:")
	// Capabilities is a closed struct; iterate in fixed alphabetical key order,
	// matching the sorted output of the earlier map-based representation.
	for _, k := range []string{
		iagent.CapArtifactDownload,
		iagent.CapFileInput,
		iagent.CapInputRequired,
		iagent.CapMultiTurn,
		iagent.CapTaskCancel,
		iagent.CapTaskGet,
		iagent.CapTaskList,
	} {
		mark := "no"
		if card.Supports(k) {
			mark = "yes"
		}
		fmt.Fprintf(w, "    %-20s %s\n", k, mark)
	}

	if len(card.Parameters) > 0 {
		fmt.Fprintln(w, "  parameters:")
		for _, pr := range card.Parameters {
			req := ""
			if pr.Required {
				req = " (required)"
			}
			fmt.Fprintf(w, "    %s: %s%s", pr.Name, pr.Type, req)
			if pr.Desc != "" {
				fmt.Fprintf(w, " — %s", stripANSI(pr.Desc))
			}
			fmt.Fprintln(w)
		}
	}

	if len(card.Skills) > 0 {
		fmt.Fprintln(w, "  skills:")
		for _, sk := range card.Skills {
			name := sk.Name
			if name == "" {
				name = sk.ID
			}
			fmt.Fprintf(w, "    %s\n", stripANSI(name))
		}
	}
}

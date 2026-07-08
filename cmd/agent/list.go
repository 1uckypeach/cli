// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/output"
)

// providerInfo describes a registered provider adapter in `agent list` output.
// Every field is sourced from the registered iagent.ProviderInfo (the single
// source of truth).
type providerInfo struct {
	Scheme         string `json:"scheme"`
	Label          string `json:"label"`
	AgentRefFormat string `json:"agent_ref_format"`
	Kind           string `json:"kind"`
	AgentIDSource  string `json:"agent_id_source"`
}

// listOptions holds all inputs for `agent list [scheme]`.
type listOptions struct {
	Factory *cmdutil.Factory
	Cmd     *cobra.Command
	Scheme  string
	Format  string
}

// NewCmdAgentList builds `agent list [scheme]`. Without an argument it
// enumerates the registered provider adapters with their metadata — a
// pure, API-free listing. With a scheme it performs second-level discovery:
// providers implementing Discoverer enumerate their agents;
// others return unsupported_capability with the agent_id_source
// guidance. Risk=read.
func NewCmdAgentList(f *cmdutil.Factory) *cobra.Command {
	opts := &listOptions{Factory: f}
	cmd := &cobra.Command{
		Use:   "list [scheme]",
		Short: "List registered agent providers, or enumerate the agents under one provider",
		Long:  "With no argument, list the built-in provider adapters and their metadata (label / agent_ref format / kind / how to obtain an agent_id) without calling any API. With a scheme, enumerate the agents under that provider (catalog providers must be enumerable; instance providers may not support it).",
		Args:  maximumArgsWithUsage(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(opts.Format); err != nil {
				return err
			}
			opts.Cmd = cmd
			if len(args) == 1 {
				opts.Scheme = args[0]
			}
			return agentListRun(opts)
		},
	}
	cmd.Flags().StringVar(&opts.Format, "format", "json", formatFlagHelp)
	cmd.Flags().String("jq", "", "用 jq 表达式过滤 JSON 输出")
	cmdutil.SetRisk(cmd, cmdutil.RiskRead)
	return cmd
}

// agentListRun dispatches `agent list [scheme]`: with a scheme it lists that
// provider's agents (second-level discovery); without it renders the provider
// listing. JSON envelope is the default; `pretty` is the opt-in human view.
func agentListRun(opts *listOptions) error {
	if opts.Scheme != "" {
		return agentListSchemeRun(opts)
	}

	f := opts.Factory
	providers := listProviders()

	// pretty is a human view only; a --jq expression implies structured JSON.
	if opts.Format == "pretty" && jqExpr(opts.Cmd) == "" {
		fmt.Fprintf(f.IOStreams.Out, "SCHEME\tLABEL\tAGENT_REF_FORMAT\tKIND\n")
		for _, p := range providers {
			fmt.Fprintf(f.IOStreams.Out, "%s\t%s\t%s\t%s\n", p.Scheme, p.Label, p.AgentRefFormat, p.Kind)
		}
		// agent_id_source is a full sentence — a TSV column would blow out the
		// row width, so surface it as a per-provider footer instead. This is the
		// single most important "where do I get an agent_id" cue for newcomers
		// and must not vanish in the human-readable view.
		fmt.Fprintln(f.IOStreams.Out)
		for _, p := range providers {
			fmt.Fprintf(f.IOStreams.Out, "agent_id 获取（%s）: %s\n", p.Scheme, p.AgentIDSource)
		}
		return nil
	}

	env := output.Envelope{
		OK:     true,
		Data:   map[string]interface{}{"providers": providers},
		Notice: output.GetNotice(),
	}
	if jq := jqExpr(opts.Cmd); jq != "" {
		return output.JqFilter(f.IOStreams.Out, env, jq)
	}
	output.PrintJson(f.IOStreams.Out, env)
	return nil
}

// agentListSchemeRun runs `agent list <scheme>`: second-level discovery for one
// provider. The Discoverer probe runs BEFORE any client construction so a
// provider without discovery support returns its precise
// unsupported_capability error even in an unconfigured environment — aligned
// with the validation-before-config-gate principle. Only a provider that
// does implement Discoverer needs a configured client for the real ListAgents
// call.
func agentListSchemeRun(opts *listOptions) error {
	f := opts.Factory
	info, ok := iagent.Info(opts.Scheme)
	if !ok {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"未知的 agent provider '%s'，当前支持: %s",
			opts.Scheme, iagent.KnownSchemes()).
			WithHint("用 lark-cli agent list 查看可用 provider")
	}
	if !probeDiscoverer(info) {
		return errs.NewValidationError(errs.SubtypeUnsupportedCapability,
			"provider '%s' 暂不支持列举 agent", opts.Scheme).
			WithHint("%s", info.AgentIDSource)
	}

	// The real ListAgents call carries the resolved identity, aligned with
	// resolveProvider (common.go) — a provider must never see a zero As on an
	// API-bound instance.
	id := f.ResolveAs(opts.Cmd.Context(), opts.Cmd, "")
	apiClient, err := f.NewAPIClient()
	if err != nil {
		return err
	}
	p, err := info.Factory(iagent.Deps{Client: apiClient, As: id}, "")
	if err != nil {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "%s", err.Error()).WithCause(err)
	}
	agents, err := p.ListAgents(opts.Cmd.Context())
	if err != nil {
		return err
	}

	// pretty is a human view only; a --jq expression implies structured JSON.
	if opts.Format == "pretty" && jqExpr(opts.Cmd) == "" {
		// Name/Description are agent-controlled remote strings — ANSI-strip
		// them before writing to the terminal.
		fmt.Fprintf(f.IOStreams.Out, "AGENT_REF\tNAME\tDESCRIPTION\n")
		for _, a := range agents {
			fmt.Fprintf(f.IOStreams.Out, "%s\t%s\t%s\n", stripANSI(a.AgentRef), stripANSI(a.Name), stripANSI(a.Description))
		}
		return nil
	}

	env := output.Envelope{
		OK:     true,
		Data:   map[string]interface{}{"agents": agents},
		Meta:   &output.Meta{Count: len(agents)},
		Notice: output.GetNotice(),
	}
	if jq := jqExpr(opts.Cmd); jq != "" {
		return output.JqFilter(f.IOStreams.Out, env, jq)
	}
	output.PrintJson(f.IOStreams.Out, env)
	return nil
}

// probeDiscoverer reports whether the provider built by info can enumerate its
// agents (wires ListAgents). The probe instance is constructed with empty Deps
// and an empty agentID — no client is needed to read a field, which keeps the
// probe usable before config init. A factory error means the capability cannot
// be confirmed, so it degrades to not discoverable.
func probeDiscoverer(info iagent.ProviderInfo) bool {
	p, err := info.Factory(iagent.Deps{}, "")
	if err != nil || p == nil {
		return false
	}
	return p.ListAgents != nil
}

// listProviders builds the provider descriptors from the built-in registry so
// the listing stays in sync with whatever adapters are registered.
func listProviders() []providerInfo {
	schemes := iagent.RegisteredSchemes()
	out := make([]providerInfo, 0, len(schemes))
	for _, s := range schemes {
		// s comes from RegisteredSchemes, so Info always succeeds.
		info, _ := iagent.Info(s)
		out = append(out, providerInfo{
			Scheme:         s,
			Label:          info.Label,
			AgentRefFormat: info.AgentRefFormat,
			Kind:           string(info.Kind),
			AgentIDSource:  info.AgentIDSource,
		})
	}
	return out
}

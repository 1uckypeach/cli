// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
)

// providerInfo describes a registered provider adapter in `agent list` output.
// Every field is sourced from the registered iagent.Provider (the single
// source of truth).
type providerInfo struct {
	Scheme         string `json:"scheme"`
	Label          string `json:"label"`
	AgentRefFormat string `json:"agent_ref_format"`
	Kind           string `json:"kind"`
	AgentIDSource  string `json:"agent_id_source"`
	// ListParams documents the business parameters `agent list <scheme>` itself
	// takes — surfaced HERE (the offline, always-reachable provider listing)
	// because at list time the caller holds no agent_ref yet, so a card-based
	// hint would point at an unreachable road.
	ListParams []iagent.CardParam `json:"list_parameters,omitempty"`
}

// listOptions holds all inputs for `agent list [scheme]`.
type listOptions struct {
	Factory *cmdutil.Factory
	Cmd     *cobra.Command
	Scheme  string
	Params  []string
	Format  string
	As      string
}

// NewCmdAgentList builds `agent list [scheme]`. Without an argument it
// enumerates the registered provider adapters with their metadata — a
// pure, API-free listing. With a scheme it performs second-level discovery:
// catalog providers enumerate offline from their static set; instance providers
// enumerate via their optional ListAgents hook (absent ⇒ unsupported_capability
// with the agent_id_source guidance). Risk=read.
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
	addParamFlag(cmd, &opts.Params)
	cmd.Flags().StringVar(&opts.Format, "format", "json", formatFlagHelp)
	cmd.Flags().String("jq", "", "用 jq 表达式过滤 JSON 输出")
	// --as only matters for the online `list <scheme>` enumeration (an instance
	// provider's ListAgents call); the no-scheme provider listing is offline and
	// identity-independent, so it ignores --as.
	addAsFlag(cmd, f, &opts.As)
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
	// The no-scheme form is a pure offline registry listing — business params
	// have no target operation, so reject explicitly rather than silently
	// ignoring what the caller thought they were passing.
	if len(opts.Params) > 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"--param 仅在 agent list <scheme> 时有意义（无 scheme 的列表是纯本地枚举）").
			WithParam("--param").
			WithHint("补充 scheme 重发，如 lark-cli agent list <scheme> --param k=v；各 provider 的 list 参数见本命令输出的 list_parameters")
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
		Meta:   listMeta(len(providers)),
		Notice: output.GetNotice(),
	}
	if jq := jqExpr(opts.Cmd); jq != "" {
		return output.JqFilter(f.IOStreams.Out, env, jq)
	}
	output.PrintJson(f.IOStreams.Out, env)
	return nil
}

// agentListSchemeRun runs `agent list <scheme>`: second-level enumeration for
// one provider. A catalog provider enumerates OFFLINE from its static set
// (prov.ListCatalog). An instance provider enumerates ONLINE via its optional
// ListAgents hook (needs a configured client); an instance provider without that
// hook is not enumerable and returns unsupported_capability + the AgentIDSource
// hint — surfaced before the client is built.
func agentListSchemeRun(opts *listOptions) error {
	f := opts.Factory
	prov, ok := iagent.Info(opts.Scheme)
	if !ok {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"未知的 agent provider '%s'，当前支持: %s",
			opts.Scheme, iagent.KnownSchemes()).
			WithHint("用 lark-cli agent list 查看可用 provider")
	}

	var agents []iagent.AgentSummary
	var identity string // set only on the online (instance) path, which resolves one
	if prov.Kind() == iagent.KindCatalog {
		// Offline catalog enumeration takes no business params (ListParams
		// requires a ListAgents hook); validate against the empty set so a stray
		// --param is rejected with the same teaching error instead of ignored.
		if _, err := validateListParams(opts.Params, nil, opts.Scheme); err != nil {
			return err
		}
		agents = prov.ListCatalog() // offline
	} else {
		// instance: needs the online ListAgents hook. Absent ⇒ not enumerable.
		if prov.ListAgents == nil {
			return errs.NewValidationError(errs.SubtypeUnsupportedCapability,
				"provider '%s' 暂不支持列举 agent", opts.Scheme).
				WithHint("%s", prov.AgentIDSource)
		}
		// Enumeration is a real online call with no agent_id, so it runs the same
		// two gates every ref-addressed online verb runs (via resolveSpec +
		// preflightScopesForRef): the user|bot identity whitelist and the
		// all-or-nothing scope preflight — keyed on the scheme since there is no ref.
		// agentID is empty (enumeration is not scoped to a single agent).
		id := f.ResolveAs(opts.Cmd.Context(), opts.Cmd, core.Identity(opts.As))
		if err := f.CheckIdentity(id, supportedIdentities); err != nil {
			return err
		}
		identity = string(id)
		// list is a provider-level operation: params validate against ListParams
		// (no spec, so no cross-operation reverse lookup); the error hint points
		// at `agent list` output's list_parameters, not at an agent card the
		// caller cannot address yet (it holds no agent_ref at list time).
		vp, err := validateListParams(opts.Params, prov.ListParams, opts.Scheme)
		if err != nil {
			return err
		}
		rt, err := runtimeFor(f, id, "", vp.Resolved)
		if err != nil {
			return err
		}
		if err := preflightScopesForScheme(f, id, opts.Scheme); err != nil {
			return err
		}
		agents, err = prov.ListAgents(opts.Cmd.Context(), rt)
		if err != nil {
			return err
		}
	}
	if agents == nil {
		agents = []iagent.AgentSummary{} // always emit [] not null
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
		OK:       true,
		Identity: identity, // empty for the offline catalog path (omitempty)
		Data:     map[string]interface{}{"agents": agents},
		Meta:     listMeta(len(agents)),
		Notice:   output.GetNotice(),
	}
	if jq := jqExpr(opts.Cmd); jq != "" {
		return output.JqFilter(f.IOStreams.Out, env, jq)
	}
	output.PrintJson(f.IOStreams.Out, env)
	return nil
}

// listProviders builds the provider descriptors from the built-in registry so
// the listing stays in sync with whatever adapters are registered.
func listProviders() []providerInfo {
	schemes := iagent.RegisteredSchemes()
	out := make([]providerInfo, 0, len(schemes))
	for _, s := range schemes {
		// s comes from RegisteredSchemes, so Info always succeeds.
		prov, _ := iagent.Info(s)
		out = append(out, providerInfo{
			Scheme:         s,
			Label:          prov.Label,
			AgentRefFormat: prov.AgentRefFormat(),
			Kind:           string(prov.Kind()),
			AgentIDSource:  prov.AgentIDSource,
			ListParams:     prov.ListParams,
		})
	}
	return out
}

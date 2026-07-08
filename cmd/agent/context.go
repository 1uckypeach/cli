// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"fmt"

	"github.com/spf13/cobra"

	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/output"
)

// contextOptions holds all inputs for the `agent context list|get|delete`
// leaves. A single struct backs all three so the shared fields (Factory, Cmd,
// Ref, As) are wired once; each RunE reads only the fields its verb needs.
type contextOptions struct {
	Factory *cmdutil.Factory
	Cmd     *cobra.Command
	Ref     string
	CtxID   string
	Yes     bool
	As      string
	Format  string
}

// NewCmdAgentContext builds the `agent context` command group: manage a remote
// agent's multi-turn contexts (requires card multi_turn=true). It is a pure group with
// no RunE so an unknown subcommand is reported rather than silently swallowed.
func NewCmdAgentContext(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage a remote agent's multi-turn contexts (sessions)",
		Long:  "context list <agent_ref> lists sessions; context get <agent_ref> <ctx-id> shows session detail; context delete <agent_ref> <ctx-id> deletes a session (high-risk, needs --yes).",
	}
	cmd.AddCommand(NewCmdAgentContextList(f))
	cmd.AddCommand(NewCmdAgentContextGet(f))
	cmd.AddCommand(NewCmdAgentContextDelete(f))
	return cmd
}

// NewCmdAgentContextList builds `agent context list <ref>`: enumerate the
// agent's multi-turn contexts into {contexts:[...]} with a meta.count. Risk=read.
func NewCmdAgentContextList(f *cmdutil.Factory) *cobra.Command {
	opts := &contextOptions{Factory: f}
	cmd := &cobra.Command{
		Use:   "list <agent_ref>",
		Short: "List a remote agent's multi-turn contexts",
		Long:  "List the multi-turn contexts (sessions) of the agent addressed by agent_ref.",
		Args:  exactArgsWithUsage(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(opts.Format); err != nil {
				return err
			}
			opts.Cmd = cmd
			opts.Ref = args[0]
			return agentContextListRun(opts)
		},
	}
	cmd.Flags().StringVar(&opts.Format, "format", "json", formatFlagHelp)
	cmd.Flags().String("jq", "", "用 jq 表达式过滤 JSON 输出")
	addAsFlag(cmd, f, &opts.As)
	cmdutil.SetRisk(cmd, cmdutil.RiskRead)
	return cmd
}

// NewCmdAgentContextGet builds `agent context get <ref> <ctx-id>`: fetch a
// single context's detail. Risk=read.
func NewCmdAgentContextGet(f *cmdutil.Factory) *cobra.Command {
	opts := &contextOptions{Factory: f}
	cmd := &cobra.Command{
		Use:   "get <agent_ref> <ctx-id>",
		Short: "Show the detail of a single multi-turn context",
		Long:  "Show the detail of the multi-turn context ctx-id under the agent addressed by agent_ref.",
		Args:  exactArgsWithUsage(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(opts.Format); err != nil {
				return err
			}
			opts.Cmd = cmd
			opts.Ref = args[0]
			opts.CtxID = args[1]
			return agentContextGetRun(opts)
		},
	}
	cmd.Flags().StringVar(&opts.Format, "format", "json", formatFlagHelp)
	cmd.Flags().String("jq", "", "用 jq 表达式过滤 JSON 输出")
	addAsFlag(cmd, f, &opts.As)
	cmdutil.SetRisk(cmd, cmdutil.RiskRead)
	return cmd
}

// NewCmdAgentContextDelete builds `agent context delete <ref> <ctx-id>`: destroy
// a multi-turn context. Deletion is irreversible, so it is high-risk-write and
// requires --yes; without it the command returns a confirmation_required error
// (exit 10) before touching the API. Risk=high-risk-write.
func NewCmdAgentContextDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &contextOptions{Factory: f}
	cmd := &cobra.Command{
		Use:   "delete <agent_ref> <ctx-id>",
		Short: "Delete a remote agent's multi-turn context (high-risk, needs --yes)",
		Long:  "Delete the multi-turn context ctx-id under the agent addressed by agent_ref. Deletion is irreversible and requires --yes to confirm; otherwise it returns confirmation_required (exit 10).",
		Args:  exactArgsWithUsage(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(opts.Format); err != nil {
				return err
			}
			opts.Cmd = cmd
			opts.Ref = args[0]
			opts.CtxID = args[1]
			return agentContextDeleteRun(opts)
		},
	}
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "确认删除（高危操作，不加则返回 exit 10）")
	cmd.Flags().StringVar(&opts.Format, "format", "json", formatFlagHelp)
	cmd.Flags().String("jq", "", "用 jq 表达式过滤 JSON 输出")
	addAsFlag(cmd, f, &opts.As)
	cmdutil.SetRisk(cmd, cmdutil.RiskHighRiskWrite)
	return cmd
}

// agentContextListRun runs `context list`: resolves the provider, lists contexts
// and emits {contexts:[...]} with meta.count.
func agentContextListRun(opts *contextOptions) error {
	f := opts.Factory
	_, spec, agentID, id, err := resolveSpec(f, opts.Cmd, opts.Ref, opts.As)
	if err != nil {
		return err
	}
	// Capability gate BEFORE the client: multi_turn is derived from ListContexts
	// being wired, so a spec without it returns unsupported_capability offline.
	if spec.ListContexts == nil {
		return capabilityError(opts.Ref, "context list", iagent.CapMultiTurn)
	}
	rt, err := runtimeFor(f, id, agentID)
	if err != nil {
		return err
	}
	// Local scope preflight: after runtimeFor, before the API call.
	if err := preflightScopesForRef(f, id, opts.Ref); err != nil {
		return err
	}
	contexts, err := spec.ListContexts(opts.Cmd.Context(), rt)
	if err != nil {
		return err
	}
	// pretty is a human view only; a --jq expression implies structured JSON.
	if opts.Format == "pretty" && jqExpr(opts.Cmd) == "" {
		printContextsTSV(f.IOStreams.Out, contexts)
		return nil
	}
	env := output.Envelope{
		OK:       true,
		Identity: string(id),
		Data:     map[string]interface{}{"contexts": contexts},
		Meta:     &output.Meta{Count: len(contexts)},
		Notice:   output.GetNotice(),
	}
	if jq := jqExpr(opts.Cmd); jq != "" {
		return output.JqFilter(f.IOStreams.Out, env, jq)
	}
	output.PrintJson(f.IOStreams.Out, env)
	return nil
}

// agentContextGetRun runs `context get`: resolves the provider, fetches the
// context detail and emits it.
func agentContextGetRun(opts *contextOptions) error {
	f := opts.Factory
	_, spec, agentID, id, err := resolveSpec(f, opts.Cmd, opts.Ref, opts.As)
	if err != nil {
		return err
	}
	// Capability gate BEFORE the client.
	if spec.GetContext == nil {
		return capabilityError(opts.Ref, "context get", iagent.CapMultiTurn)
	}
	rt, err := runtimeFor(f, id, agentID)
	if err != nil {
		return err
	}
	// Local scope preflight: after runtimeFor, before the API call.
	if err := preflightScopesForRef(f, id, opts.Ref); err != nil {
		return err
	}
	detail, err := spec.GetContext(opts.Cmd.Context(), rt, opts.CtxID)
	if err != nil {
		return err
	}
	if detail != nil {
		// Derive IsTerminal from State (single source of truth) for the embedded
		// task summaries before emission.
		detail.Tasks = normalizeTaskSummaries(detail.Tasks)
	}
	// pretty is a human view only; a --jq expression implies structured JSON.
	if opts.Format == "pretty" && jqExpr(opts.Cmd) == "" {
		printContextDetailPretty(f.IOStreams.Out, detail)
		return nil
	}
	env := output.Envelope{
		OK:       true,
		Identity: string(id),
		Data:     detail,
		Notice:   output.GetNotice(),
	}
	if jq := jqExpr(opts.Cmd); jq != "" {
		return output.JqFilter(f.IOStreams.Out, env, jq)
	}
	output.PrintJson(f.IOStreams.Out, env)
	return nil
}

// agentContextDeleteRun runs `context delete`. The --yes confirmation guard runs
// first so a missing confirmation returns confirmation_required (exit 10) before
// any provider is built and holds even under a nil Factory. Only a
// confirmed delete reaches resolveProvider + DeleteContext.
func agentContextDeleteRun(opts *contextOptions) error {
	if !opts.Yes {
		return cmdutil.RequireConfirmation("agent context delete")
	}

	f := opts.Factory
	_, spec, agentID, id, err := resolveSpec(f, opts.Cmd, opts.Ref, opts.As)
	if err != nil {
		return err
	}
	// Capability gate BEFORE the client.
	if spec.DeleteContext == nil {
		return capabilityError(opts.Ref, "context delete", iagent.CapMultiTurn)
	}
	rt, err := runtimeFor(f, id, agentID)
	if err != nil {
		return err
	}
	// Local scope preflight: after runtimeFor, before the API call.
	if err := preflightScopesForRef(f, id, opts.Ref); err != nil {
		return err
	}
	if err := spec.DeleteContext(opts.Cmd.Context(), rt, opts.CtxID); err != nil {
		return err
	}
	// pretty is a human view only; a --jq expression implies structured JSON.
	if opts.Format == "pretty" && jqExpr(opts.Cmd) == "" {
		fmt.Fprintf(f.IOStreams.Out, "context_id: %s\ndeleted: true\n", kvValue(opts.CtxID))
		return nil
	}
	env := output.Envelope{
		OK:       true,
		Identity: string(id),
		Data:     map[string]interface{}{"context_id": opts.CtxID, "deleted": true},
		Notice:   output.GetNotice(),
	}
	if jq := jqExpr(opts.Cmd); jq != "" {
		return output.JqFilter(f.IOStreams.Out, env, jq)
	}
	output.PrintJson(f.IOStreams.Out, env)
	return nil
}

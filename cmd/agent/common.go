// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package agent implements the `agent` command tree: a provider-agnostic
// surface over remote A2A agents. This file holds the shared
// command-layer helpers: ref→provider resolution, --param validation against a
// Card, success-envelope emission, capability gating, and wait/watch polling.
package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
)

// supportedIdentities is the identity whitelist enforced for every agent
// command; provider cards advertise (a subset of) the same set.
var supportedIdentities = []string{string(core.AsUser), string(core.AsBot)}

// sleep is the package-level, test-injectable backoff sleep. It blocks for d or
// until ctx is done, returning true if the full duration elapsed and false if
// ctx was canceled first. Tests swap it for a no-op.
var sleep = func(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// resolveSpec is the fully-offline resolution path: it resolves the effective
// identity, enforces the user|bot whitelist, and looks up the AgentSpec
// addressed by ref — WITHOUT constructing a client or touching the network. It
// is the FIRST step of every verb, so a malformed ref, an unknown scheme /
// unknown catalog id, AND a capability gate all surface at exit 2 BEFORE the
// config gate — an unconfigured user still gets the precise error, not
// not_configured. A real API verb then calls runtimeFor to build the client.
func resolveSpec(f *cmdutil.Factory, cmd *cobra.Command, ref, asStr string) (iagent.Provider, *iagent.AgentSpec, string, core.Identity, error) {
	id := f.ResolveAs(cmd.Context(), cmd, core.Identity(asStr))
	if err := f.CheckIdentity(id, supportedIdentities); err != nil {
		return iagent.Provider{}, nil, "", "", err
	}
	prov, spec, agentID, err := iagent.LookupSpec(ref)
	if err != nil {
		// ParseRef / unknown-scheme / unknown-id errors carry the validation
		// wording; promote them to a typed validation error (with a recovery hint)
		// so RunE never returns a bare error and the exit code / subtype are stable.
		return iagent.Provider{}, nil, "", "", wrapRefResolveError(err)
	}
	return prov, spec, agentID, id, nil
}

// runtimeFor builds the identity-pinned Runtime for a verb that actually calls
// the remote API. It requires a configured client (not_configured / exit 3 here
// is correct for a real API call). agentID is the resolved agent this call
// addresses (from the ref), exposed to hooks via rt.AgentID().
func runtimeFor(f *cmdutil.Factory, id core.Identity, agentID string) (iagent.Runtime, error) {
	apiClient, err := f.NewAPIClient()
	if err != nil {
		return nil, err
	}
	return &cmdRuntime{client: apiClient, as: id, agentID: agentID}, nil
}

// wrapRefResolveError promotes a ParseRef / provider-resolution error to a
// validation typed error (subtype invalid_argument, exit 2) and attaches the
// recovery hint keyed to the failure mode: a malformed ref (no ':' / empty
// half — matched via the ErrInvalidRef sentinel) teaches the <scheme>:<agent_id>
// shape; an unknown scheme points at `agent list` to discover the available
// providers. Both hints are copy-pasteable next steps, not just wording.
func wrapRefResolveError(err error) error {
	// LookupSpec's unknown-catalog-id case is ALREADY a typed validation error
	// carrying a scheme-scoped hint (`agent list <scheme>`); pass it through
	// instead of flattening it via err.Error() and overwriting that hint with the
	// generic provider-list one. Only the untyped ParseRef sentinel / unknown-
	// scheme errors need wrapping.
	if _, ok := errs.ProblemOf(err); ok {
		return err
	}
	e := errs.NewValidationError(errs.SubtypeInvalidArgument, "%s", err.Error()).WithCause(err)
	if errors.Is(err, iagent.ErrInvalidRef) {
		return e.WithHint("agent_ref 形如 <scheme>:<agent_id>，如 example:echo")
	}
	return e.WithHint("用 lark-cli agent list 查看可用 provider")
}

// cardHint builds the "check the agent card" hint. The ref is user-echoed
// input: when it passes the safeNextRef whitelist the hint carries the
// copy-pasteable command; otherwise it degrades to plain guidance without any
// interpolated command (a ref containing spaces would make the command
// non-copy-pasteable, and the hint is what an AI copies verbatim).
func cardHint(ref, what string) string {
	if safeNextRef(ref) {
		return fmt.Sprintf("运行 lark-cli agent card %s 查看%s", ref, what)
	}
	return fmt.Sprintf("查看该 agent 的能力卡片（agent card 命令）确认%s", what)
}

// parseAndValidateParams parses `key=value` --param pairs and validates them
// against the card's Parameters declaration: every Required parameter must be
// present, and every provided key must be declared (an undeclared key
// would otherwise be silently dropped by the provider). A pair without '=' (or
// an empty key), a missing required parameter, or an unknown key returns a
// validation typed error (subtype invalid_argument, param "param:<key>")
// whose hint points at `agent card <ref>`. A nil card skips both
// card-driven checks.
func parseAndValidateParams(kvs []string, card *iagent.AgentCard, ref string) (map[string]string, error) {
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
				"--param 格式应为 key=value，得到 %q", kv).
				WithParam("--param").
				WithHint("以 --param key=value 形式重发")
		}
		m[k] = v
	}
	if card != nil {
		declared := make(map[string]bool, len(card.Parameters))
		for _, p := range card.Parameters {
			declared[p.Name] = true
		}
		// Unknown keys are checked in input order so the reported key is
		// deterministic when several are undeclared.
		for _, kv := range kvs {
			k, _, _ := strings.Cut(kv, "=")
			if !declared[k] {
				return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
					"未知参数 %s（该 agent 未声明此参数）", k).
					WithParam("param:"+k).
					WithHint("%s", cardHint(ref, " parameters 声明"))
			}
		}
		for _, p := range card.Parameters {
			if !p.Required {
				continue
			}
			if _, ok := m[p.Name]; !ok {
				return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
					"缺少必填参数 %s（该 agent 要求）", p.Name).
					WithParam("param:"+p.Name).
					WithHint("%s", cardHint(ref, " parameters 声明"))
			}
		}
	}
	return m, nil
}

// emitTask writes a task result: the standard success envelope carrying
// meta.next[] hints for AI callers, or — with format=pretty and no --jq —
// the key:value human view. Because the agent's messages/artifacts are
// untrusted external content, the payload is run through content-safety
// scanning before emission on BOTH paths (and the pretty path additionally
// ANSI-strips agent text). A --jq expression, when the leaf command registers
// one, implies structured JSON and filters stdout.
func emitTask(f *cmdutil.Factory, cmd *cobra.Command, task *iagent.AgentTask, next []output.NextAction, format string) error {
	out := f.IOStreams.Out
	errOut := f.IOStreams.ErrOut

	scan := output.ScanForSafety(cmd.CommandPath(), task, errOut)
	if scan.Blocked {
		return scan.BlockErr
	}

	if format == "pretty" && jqExpr(cmd) == "" {
		if scan.Alert != nil {
			output.WriteAlertWarning(errOut, scan.Alert)
		}
		printTaskPretty(out, task)
		return nil
	}

	env := output.Envelope{
		OK:       true,
		Identity: string(f.ResolvedIdentity),
		Data:     task,
		Notice:   output.GetNotice(),
	}
	if len(next) > 0 {
		env.Meta = &output.Meta{Next: next}
	}
	if scan.Alert != nil {
		env.ContentSafetyAlert = scan.Alert
	}

	if jq := jqExpr(cmd); jq != "" {
		if scan.Alert != nil {
			output.WriteAlertWarning(errOut, scan.Alert)
		}
		return output.JqFilter(out, env, jq)
	}
	output.PrintJson(out, env)
	return nil
}

// scanAndEmitData is the shared scan-then-emit path for the read leaves whose
// payload now carries untrusted agent-authored text — task list
// (TaskSummary.Summary), context list, and context get
// (ContextDetail.ActiveTask.Summary). These used to PrintJson directly and so
// BYPASSED content-safety; like emitTask they now run output.ScanForSafety on
// the payload BEFORE emission on every path: a block returns the typed block
// error, a warn attaches the alert to the JSON envelope (and prints a stderr
// warning on the pretty / jq paths). data is the Envelope.Data payload (and what
// is scanned); meta is an optional *output.Meta (list count, nil for a single
// detail); pretty renders the --format pretty human view and is skipped when a
// --jq expression forces structured JSON.
func scanAndEmitData(f *cmdutil.Factory, cmd *cobra.Command, format string, data any, meta *output.Meta, pretty func(io.Writer)) error {
	out := f.IOStreams.Out
	errOut := f.IOStreams.ErrOut

	scan := output.ScanForSafety(cmd.CommandPath(), data, errOut)
	if scan.Blocked {
		return scan.BlockErr
	}

	if format == "pretty" && jqExpr(cmd) == "" {
		if scan.Alert != nil {
			output.WriteAlertWarning(errOut, scan.Alert)
		}
		pretty(out)
		return nil
	}

	env := output.Envelope{
		OK:       true,
		Identity: string(f.ResolvedIdentity),
		Data:     data,
		Meta:     meta,
		Notice:   output.GetNotice(),
	}
	if scan.Alert != nil {
		env.ContentSafetyAlert = scan.Alert
	}
	if jq := jqExpr(cmd); jq != "" {
		if scan.Alert != nil {
			output.WriteAlertWarning(errOut, scan.Alert)
		}
		return output.JqFilter(out, env, jq)
	}
	output.PrintJson(out, env)
	return nil
}

// jqExpr reads the --jq flag value if the leaf command registered one; absent
// otherwise.
func jqExpr(cmd *cobra.Command) string {
	if cmd == nil { // options structs built directly in tests may carry no Cmd
		return ""
	}
	if f := cmd.Flags().Lookup("jq"); f != nil {
		return f.Value.String()
	}
	return ""
}

// capabilityError returns the unsupported_capability validation error (exit 2)
// used for capability gating: capHuman is the human-facing action (e.g.
// "task cancel"), capKey the Card capability key (e.g. task_cancel). The hint
// interpolates ref only when it passes the whitelist (cardHint).
func capabilityError(ref, capHuman, capKey string) error {
	return errs.NewValidationError(
		errs.SubtypeUnsupportedCapability,
		"agent '%s' 不支持 '%s'（capability %s=false）", ref, capHuman, capKey,
	).WithHint("%s", cardHint(ref, "支持的能力"))
}

// normalizeTask derives the redundant IsTerminal flag from State — the single
// source of truth — the moment a task enters the command layer, so a provider
// that forgets (or mis-fills) the flag can never skew watch exit codes or an
// AI caller's stop-polling decision. nil-safe; returns t for call-site chaining.
func normalizeTask(t *iagent.AgentTask) *iagent.AgentTask {
	if t != nil {
		t.IsTerminal = t.State.IsTerminal()
	}
	return t
}

// normalizeTaskSummaries derives IsTerminal from State for every summary (same
// single-source rule as normalizeTask), returning the slice for chaining.
func normalizeTaskSummaries(ts []iagent.TaskSummary) []iagent.TaskSummary {
	for i := range ts {
		ts[i].IsTerminal = ts[i].State.IsTerminal()
	}
	return ts
}

// pollToStop polls getTask with exponential backoff (1s → 5s cap) until the
// task hits a stop condition (terminal, input_required, or auth_required)
// or ctx is done. A timeout is not a failure: it returns the most recent
// task with a nil error, letting the caller print the current state (exit 0). A
// provider GetTask error is surfaced. getTask is a bound closure over the
// resolved spec + runtime (spec.GetTask(ctx, rt, id)), so pollToStop stays
// provider-neutral and testable.
func pollToStop(ctx context.Context, getTask func(context.Context, string) (*iagent.AgentTask, error), taskID string) (*iagent.AgentTask, error) {
	const (
		initialDelay = time.Second
		maxDelay     = 5 * time.Second
	)
	var last *iagent.AgentTask
	delay := initialDelay
	for {
		task, err := getTask(ctx, taskID)
		if err != nil {
			return last, err
		}
		last = task
		if task.State.ShouldStopPolling() {
			return task, nil
		}
		if ctx.Err() != nil {
			return last, nil //nolint:nilerr // a poll timeout is an observation-window close, not a task failure — return the last task with exit 0
		}
		if !sleep(ctx, delay) {
			// ctx canceled during backoff → observation window closed, not a
			// task failure.
			return last, nil
		}
		if delay < maxDelay {
			if delay *= 2; delay > maxDelay {
				delay = maxDelay
			}
		}
	}
}

// semanticExitError maps a wait/watch terminal task to the semantic exit code:
// a non-successful terminal state (failed/rejected/canceled) yields a
// silent exit-1 signal; any other state (including a successful terminal or a
// non-terminal stop like input_required) yields nil. A nil task yields nil.
func semanticExitError(task *iagent.AgentTask) error {
	if task == nil || !task.IsTerminal {
		return nil
	}
	switch task.State {
	case iagent.StateFailed, iagent.StateRejected, iagent.StateCanceled:
		return output.ErrBare(1)
	default:
		return nil
	}
}

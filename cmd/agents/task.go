// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agents

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagents "github.com/larksuite/cli/internal/agents"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/internal/vfs"
)

// maxArtifactBytes caps a single downloaded artifact to guard against an
// untrusted host streaming an unbounded body onto local disk.
const maxArtifactBytes = 256 << 20 // 256 MiB

// taskOptions holds all inputs for the `agents task get|list|cancel` leaves. A
// single struct backs all three so the shared fields (Factory, Cmd, Ref, As)
// are wired once; each RunE reads only the fields its verb needs.
type taskOptions struct {
	Factory    *cmdutil.Factory
	Cmd        *cobra.Command
	Ref        string
	TaskID     string
	ContextID  string
	ArtifactID string
	Params     []string
	Output     string
	Force      bool
	Watch      bool
	Timeout    time.Duration
	As         string
	Format     string
}

// resolveDownload is the DownloadArtifact seam: it resolves the provider
// addressed by opts under the effective identity, runs the local scope
// preflight, and fetches the artifact descriptor. Tests swap it to return
// inline bytes without a Factory / network.
var resolveDownload = func(opts *taskOptions) (*iagents.ArtifactData, error) {
	_, spec, agentID, id, err := resolveSpec(opts.Factory, opts.Cmd, opts.Ref, opts.As)
	if err != nil {
		return nil, err
	}
	// Capability gate before any network: a spec that does not wire
	// DownloadArtifact (card artifact_download=false) returns unsupported_capability.
	if spec.DownloadArtifact.Handler == nil {
		return nil, capabilityError(opts.Ref, "artifact download", iagents.CapArtifactDownload)
	}
	// --artifact switches this command to the artifact_download operation, so
	// params validate STRICTLY against its declaration (a task_get-only param
	// here gets the cross-operation teaching error), and rt.Params() carries
	// only artifact_download keys — the executing hook's own contract.
	vp, err := validateParams(opts.Params, spec.DownloadArtifact.Params, iagents.VerbArtifactDownload, spec, opts.Ref)
	if err != nil {
		return nil, err
	}
	rt, err := runtimeFor(opts.Factory, id, agentID, vp.Resolved)
	if err != nil {
		return nil, err
	}
	if err := preflightScopesForRef(opts.Factory, id, opts.Ref); err != nil {
		return nil, err
	}
	return spec.DownloadArtifact.Handler(opts.Cmd.Context(), rt, opts.TaskID, opts.ArtifactID)
}

// artifactFetch is the URL-download seam: it SSRF-validates rawURL and fetches
// its bytes with a download-hardened client. Tests swap it to serve a loopback
// httptest server (which the production SSRF guard would otherwise block).
var artifactFetch = fetchArtifactURL

// hardenDownloadClient is the download-client-build seam inside fetchArtifactURL.
// Production wraps the base client with the SSRF-hardened redirect/dial rules;
// tests swap it to pass the (interceptable) base client through unchanged so the
// request/status/read/limit logic can run against an httpmock transport that the
// hardened client's transport clone would otherwise discard.
var hardenDownloadClient = func(base *http.Client) *http.Client {
	return validate.NewDownloadHTTPClient(base, validate.DownloadHTTPClientOptions{})
}

// NewCmdAgentTask builds the `agents task` command group: query, list and cancel
// tasks on a remote agent. It is a pure group with no RunE so an unknown
// subcommand is reported rather than silently swallowed.
func NewCmdAgentTask(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Query / list / cancel a remote agent's tasks",
		Long:  "task get <agent_ref> <task-id> queries a single task (with --watch polling and --artifact download); task list <agent_ref> lists tasks; task cancel <agent_ref> <task-id> cancels (capability-gated).",
	}
	cmd.AddCommand(NewCmdAgentTaskGet(f))
	cmd.AddCommand(NewCmdAgentTaskList(f))
	cmd.AddCommand(NewCmdAgentTaskCancel(f))
	return cmd
}

// NewCmdAgentTaskGet builds `agents task get <ref> <task-id>`: fetch a single
// task's state and artifacts. `--watch` polls until the task reaches a stop
// condition and the terminal state drives the semantic exit code;
// `--timeout` bounds that poll (0 = unbounded, blocking to a stop condition —
// the backward-compatible default). `--artifact <id>` downloads that artifact
// to `-o` instead of printing the task: a URL-type artifact is SSRF-validated
// and fetched, an inline-bytes artifact is written straight to disk.
// Risk=read.
func NewCmdAgentTaskGet(f *cmdutil.Factory) *cobra.Command {
	opts := &taskOptions{Factory: f}
	cmd := &cobra.Command{
		Use:   "get <agent_ref> <task-id>",
		Short: "Query a single task's state and artifacts",
		Long:  "Query the state and artifacts of task-id under the agent addressed by agent_ref. --watch polls until a stop condition and then prints the final state; --timeout bounds the watch (0 = unbounded, blocking to a terminal state). --artifact <id> with -o downloads that artifact to a local file.",
		Args:  exactArgsWithUsage(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(opts.Format); err != nil {
				return err
			}
			opts.Cmd = cmd
			opts.Ref = args[0]
			opts.TaskID = args[1]
			return agentTaskGetRun(opts)
		},
	}
	cmd.Flags().BoolVar(&opts.Watch, "watch", false, "轮询任务直到进入停轮询条件（终态 / 需补输入 / 需补鉴权）再打印最终状态")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 0, "--watch 的最长轮询时长，如 30s；0=无界（阻塞到终态）；到点未终止则返回当前状态+续 watch 命令")
	cmd.Flags().StringVar(&opts.ArtifactID, "artifact", "", "下载指定产物 id（须配合 -o 指定落盘路径），不打印任务详情")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "产物落盘路径（仅 --artifact 时使用）")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "允许覆盖已存在的 -o 目标文件（默认拒绝覆盖，防止误毁本地文件）")
	addParamFlag(cmd, &opts.Params)
	cmd.Flags().StringVar(&opts.Format, "format", "json", formatFlagHelp)
	cmd.Flags().String("jq", "", "用 jq 表达式过滤 JSON 输出")
	addAsFlag(cmd, f, &opts.As)
	cmdutil.SetRisk(cmd, cmdutil.RiskRead)
	return cmd
}

// NewCmdAgentTaskList builds `agents task list <ref>`: enumerate the agent's
// tasks, optionally filtered by `--context-id`, into {tasks:[...]} with a
// meta.count. Risk=read.
func NewCmdAgentTaskList(f *cmdutil.Factory) *cobra.Command {
	opts := &taskOptions{Factory: f}
	cmd := &cobra.Command{
		Use:   "list <agent_ref>",
		Short: "List a remote agent's tasks",
		Long:  "List the tasks of the agent addressed by agent_ref; --context-id filters by multi-turn context.",
		Args:  exactArgsWithUsage(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(opts.Format); err != nil {
				return err
			}
			opts.Cmd = cmd
			opts.Ref = args[0]
			return agentTaskListRun(opts)
		},
	}
	cmd.Flags().StringVar(&opts.ContextID, "context-id", "", "按多轮上下文 id 过滤任务")
	addParamFlag(cmd, &opts.Params)
	cmd.Flags().StringVar(&opts.Format, "format", "json", formatFlagHelp)
	cmd.Flags().String("jq", "", "用 jq 表达式过滤 JSON 输出")
	addAsFlag(cmd, f, &opts.As)
	cmdutil.SetRisk(cmd, cmdutil.RiskRead)
	return cmd
}

// NewCmdAgentTaskCancel builds `agents task cancel <ref> <task-id>`: cancel
// (interrupt) a task. Cancel is capability-gated on the Card's task_cancel: for
// an agent that does not support it (task_cancel=false, e.g. example:echo) the
// command returns unsupported_capability without contacting the API.
// Risk=write.
func NewCmdAgentTaskCancel(f *cmdutil.Factory) *cobra.Command {
	opts := &taskOptions{Factory: f}
	cmd := &cobra.Command{
		Use:   "cancel <agent_ref> <task-id>",
		Short: "Cancel (interrupt) a remote agent's task",
		Long:  "Cancel task-id under the agent addressed by agent_ref. If the agent does not support cancel (card task_cancel=false), it returns unsupported_capability without sending a request.",
		Args:  exactArgsWithUsage(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(opts.Format); err != nil {
				return err
			}
			opts.Cmd = cmd
			opts.Ref = args[0]
			opts.TaskID = args[1]
			return agentTaskCancelRun(opts)
		},
	}
	addParamFlag(cmd, &opts.Params)
	cmd.Flags().StringVar(&opts.Format, "format", "json", formatFlagHelp)
	cmd.Flags().String("jq", "", "用 jq 表达式过滤 JSON 输出")
	addAsFlag(cmd, f, &opts.As)
	cmdutil.SetRisk(cmd, cmdutil.RiskWrite)
	return cmd
}

// addAsFlag registers the identity flag: the real API-identity flag when a
// Factory is present, or a bare --as for construction-time unit tests (f nil).
func addAsFlag(cmd *cobra.Command, f *cmdutil.Factory, as *string) {
	if f != nil {
		cmdutil.AddAPIIdentityFlag(cmd.Context(), cmd, f, as)
		return
	}
	cmd.Flags().StringVar(as, "as", "", "identity type: user | bot")
}

// agentTaskGetRun runs `task get`. The `--artifact` client-side guard (requires
// -o) runs first so it never touches the network and holds under a nil Factory.
// With `--artifact` it downloads the named artifact to -o; otherwise it
// fetches the task, optionally polling it to a stop condition under --watch, and
// emits the task with the terminal state driving the semantic exit code.
func agentTaskGetRun(opts *taskOptions) error {
	if opts.ArtifactID != "" {
		if opts.Output == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"--artifact 需配合 -o/--output 指定落盘路径").
				WithParam("--output").
				WithHint("补充 -o <落盘路径> 后重发")
		}
		return downloadArtifact(opts)
	}

	// --timeout only bounds the --watch poll; without --watch it is meaningless.
	// Guard it client-side (mirrors the send --task-id/--context-id combo check)
	// so it never touches the network and holds under a nil Factory.
	if opts.Timeout > 0 && !opts.Watch {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"--timeout 需与 --watch 一起使用").
			WithParam("--timeout").
			WithHint("加上 --watch（如 --watch --timeout 30s）做有界轮询；或去掉 --timeout 做单次查询")
	}

	f := opts.Factory
	_, spec, agentID, id, err := resolveSpec(f, opts.Cmd, opts.Ref, opts.As)
	if err != nil {
		return err
	}
	vp, err := validateParams(opts.Params, spec.GetTask.Params, iagents.VerbTaskGet, spec, opts.Ref)
	if err != nil {
		return err
	}
	rt, err := runtimeFor(f, id, agentID, vp.Resolved)
	if err != nil {
		return err
	}
	// Local scope preflight: after runtimeFor, before the API call.
	if err := preflightScopesForRef(f, id, opts.Ref); err != nil {
		return err
	}

	ctx := opts.Cmd.Context()
	task, err := spec.GetTask.Handler(ctx, rt, opts.TaskID)
	if err != nil {
		return err
	}
	// A provider that decodes an empty "data" via Call[*AgentTask] legitimately
	// returns (nil, nil) (see internal/agent decodeData). Surface that as a typed
	// error rather than dereferencing task.State below (the --watch branch would
	// otherwise panic; the sibling consumers all nil-guard).
	if task == nil {
		return errs.NewInternalError(errs.SubtypeInvalidResponse,
			"provider 未返回任务数据（响应无 data）")
	}

	if opts.Watch && !task.State.ShouldStopPolling() {
		// A positive --timeout bounds the poll: pollToStop returns the most recent
		// task with a nil error when the deadline fires (a timeout is an
		// observation-window close, not a failure), so a long task degrades to
		// "current state + a fresh watch hint" instead of blocking forever. 0 =
		// unbounded (the backward-compatible default). pollToStop is unchanged.
		pollCtx := ctx
		if opts.Timeout > 0 {
			var cancel context.CancelFunc
			pollCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
			defer cancel()
		}
		final, perr := pollToStop(pollCtx, func(c context.Context, tid string) (*iagents.AgentTask, error) {
			return spec.GetTask.Handler(c, rt, tid)
		}, opts.TaskID)
		if perr != nil {
			return perr
		}
		if final != nil {
			task = final
		}
	}

	// Derive IsTerminal from State (single source of truth) before any consumer
	// — emitTask's output and semanticExitError below both read the flag.
	normalizeTask(task)
	if err := emitTask(f, opts.Cmd, task, nextForTask(opts.Ref, task, spec, vp.Given, iagents.VerbTaskGet), opts.Format); err != nil {
		return err
	}
	// Under --watch a non-successful terminal state signals exit 1; a
	// plain get (or a non-terminal stop) is exit 0.
	if opts.Watch {
		return semanticExitError(task)
	}
	return nil
}

// agentTaskListRun runs `task list`: resolves the provider, lists tasks
// (optionally filtered by --context-id), sorts them newest-first by UpdatedAt,
// and emits {tasks:[...]} with meta.count through content-safety scanning (the
// summaries carry untrusted agent text).
func agentTaskListRun(opts *taskOptions) error {
	f := opts.Factory
	_, spec, agentID, id, err := resolveSpec(f, opts.Cmd, opts.Ref, opts.As)
	if err != nil {
		return err
	}
	// Capability gate BEFORE building the client: a spec that does not wire
	// ListTasks (card task_list=false) returns unsupported_capability offline.
	if spec.ListTasks.Handler == nil {
		return capabilityError(opts.Ref, "task list", iagents.CapTaskList)
	}
	vp, err := validateParams(opts.Params, spec.ListTasks.Params, iagents.VerbTaskList, spec, opts.Ref)
	if err != nil {
		return err
	}
	rt, err := runtimeFor(f, id, agentID, vp.Resolved)
	if err != nil {
		return err
	}
	// Local scope preflight: after runtimeFor, before the API call.
	if err := preflightScopesForRef(f, id, opts.Ref); err != nil {
		return err
	}
	tasks, err := spec.ListTasks.Handler(opts.Cmd.Context(), rt, opts.ContextID)
	if err != nil {
		return err
	}
	tasks = normalizeTaskSummaries(tasks)
	// Newest-first: sort by UpdatedAt (RFC3339 UTC) descending so the most
	// recently active task heads the list; a stable sort preserves the provider's
	// relative order for equal timestamps, and tasks with no timestamp sort last.
	sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].UpdatedAt > tasks[j].UpdatedAt })
	if tasks == nil {
		tasks = []iagents.TaskSummary{} // always emit [] not null (matches the Card.Parameters array convention)
	}
	return scanAndEmitData(f, opts.Cmd, opts.Format,
		map[string]interface{}{"tasks": tasks},
		listMeta(len(tasks)),
		func(w io.Writer) { printTaskSummariesTSV(w, tasks) })
}

// agentTaskCancelRun runs `task cancel`. Cancel is capability-gated offline
// (right after resolveSpec, before the client is built): a spec that does not
// wire CancelTask (card task_cancel=false, e.g. example:echo) returns
// unsupported_capability without any API access. Only a supporting spec reaches
// runtimeFor + CancelTask.
func agentTaskCancelRun(opts *taskOptions) error {
	f := opts.Factory
	_, spec, agentID, id, err := resolveSpec(f, opts.Cmd, opts.Ref, opts.As)
	if err != nil {
		return err
	}
	if spec.CancelTask.Handler == nil {
		return capabilityError(opts.Ref, "task cancel", iagents.CapTaskCancel)
	}
	vp, err := validateParams(opts.Params, spec.CancelTask.Params, iagents.VerbTaskCancel, spec, opts.Ref)
	if err != nil {
		return err
	}
	rt, err := runtimeFor(f, id, agentID, vp.Resolved)
	if err != nil {
		return err
	}
	// Local scope preflight: after runtimeFor, before the API call. A
	// task_cancel=false agent never reaches here (gated above); it is wired so a
	// provider that supports cancel is not silently exempt from the all-or-nothing
	// scope check.
	if err := preflightScopesForRef(f, id, opts.Ref); err != nil {
		return err
	}
	if err := spec.CancelTask.Handler(opts.Cmd.Context(), rt, opts.TaskID); err != nil {
		return err
	}
	// pretty is a human view only; a --jq expression implies structured JSON.
	if opts.Format == "pretty" && jqExpr(opts.Cmd) == "" {
		fmt.Fprintf(f.IOStreams.Out, "task_id: %s\ncanceled: true\n", kvValue(opts.TaskID))
		return nil
	}
	env := output.Envelope{
		OK:       true,
		Identity: string(id),
		Data:     map[string]interface{}{"task_id": opts.TaskID, "canceled": true},
		Notice:   output.GetNotice(),
	}
	if jq := jqExpr(opts.Cmd); jq != "" {
		return output.JqFilter(f.IOStreams.Out, env, jq)
	}
	output.PrintJson(f.IOStreams.Out, env)
	return nil
}

// downloadArtifact resolves the artifact descriptor and writes it to opts.Output
// under vfs. A URL-type artifact is SSRF-validated and fetched over a
// download-hardened client; an inline-bytes artifact is written directly. The
// output path is validated with SafeOutputPath (relative, within the CWD)
// before any write.
func downloadArtifact(opts *taskOptions) error {
	safePath, err := validate.SafeOutputPath(opts.Output)
	if err != nil {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "非法的 -o 路径: %v", err).
			WithParam("--output").WithCause(err)
	}

	// Overwriting a local file destroys its content irreversibly — a high-risk
	// write. It goes through the same confirmation contract as other --force
	// gates (config bind): without --force, a would-be overwrite returns
	// confirmation_required (exit 10) before any download. Lstat (not Stat) so a
	// symlink at the path counts as existing rather than being followed.
	if !opts.Force {
		if _, statErr := vfs.Lstat(safePath); statErr == nil {
			return errs.NewConfirmationRequiredError(errs.RiskHighRiskWrite, "agents task get --artifact -o",
				"目标文件已存在，覆盖会不可逆地毁掉本地内容: %s", safePath).
				WithHint("确认要覆盖后加 --force 重跑，或换一个 -o 路径")
		}
	}

	ctx := opts.Cmd.Context()
	art, err := resolveDownload(opts)
	if err != nil {
		return err
	}
	// A provider decoding an empty "data" via Call[*ArtifactData] can return
	// (nil, nil); and a non-nil descriptor with neither inline bytes nor a URL
	// carries no downloadable content. Both are provider-response defects — fail
	// with a typed error instead of dereferencing nil or writing a 0-byte file
	// (which under --force would clobber an existing local file with emptiness).
	if art == nil {
		return errs.NewInternalError(errs.SubtypeInvalidResponse,
			"provider 未返回产物数据（响应无 data）")
	}
	if len(art.Bytes) == 0 && art.URL == "" {
		return errs.NewInternalError(errs.SubtypeInvalidResponse,
			"产物 '%s' 无可下载内容（provider 既未提供内联字节也未提供下载 URL）", opts.ArtifactID)
	}

	data := art.Bytes
	if art.URL != "" {
		data, err = artifactFetch(ctx, opts.Factory, art.URL)
		if err != nil {
			return err
		}
	}

	if err := vfs.WriteFile(safePath, data, 0o600); err != nil {
		return errs.NewInternalError(errs.SubtypeFileIO, "写产物到 %s 失败: %v", safePath, err).WithCause(err)
	}

	f := opts.Factory
	// pretty is a human view only; a --jq expression implies structured JSON.
	if opts.Format == "pretty" && jqExpr(opts.Cmd) == "" {
		out := f.IOStreams.Out
		fmt.Fprintf(out, "artifact_id: %s\n", kvValue(opts.ArtifactID))
		fmt.Fprintf(out, "path: %s\n", safePath)
		fmt.Fprintf(out, "bytes: %d\n", len(data))
		if art.Mime != "" {
			fmt.Fprintf(out, "mime: %s\n", kvValue(art.Mime))
		}
		// suggested_name is the server-suggested name, for reference only; the
		// actual on-disk path is already the safePath (-o) above.
		if art.Name != "" {
			fmt.Fprintf(out, "suggested_name: %s\n", kvValue(art.Name))
		}
		return nil
	}
	env := output.Envelope{
		OK:       true,
		Identity: string(f.ResolvedIdentity),
		Data: map[string]interface{}{
			"artifact_id":    opts.ArtifactID,
			"path":           safePath,
			"bytes":          len(data),
			"mime":           art.Mime,
			"suggested_name": art.Name,
		},
		Notice: output.GetNotice(),
	}
	if jq := jqExpr(opts.Cmd); jq != "" {
		return output.JqFilter(f.IOStreams.Out, env, jq)
	}
	output.PrintJson(f.IOStreams.Out, env)
	return nil
}

// fetchArtifactURL is the production URL fetch: it SSRF-validates rawURL, builds
// a download-hardened HTTP client from the Factory and reads the body up to
// maxArtifactBytes, refusing anything larger. The artifact host is untrusted
// external content, so both the URL and the redirect chain are guarded.
func fetchArtifactURL(ctx context.Context, f *cmdutil.Factory, rawURL string) ([]byte, error) {
	if err := validate.ValidateDownloadSourceURL(ctx, rawURL); err != nil {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "被拦截的产物 URL: %v", err).
			WithCause(err)
	}
	// Artifact bytes come from an untrusted host over the network; require https
	// so the payload cannot be read or tampered with in transit. The SSRF check
	// above already rejects private/loopback hosts and non-http(s) schemes, so a
	// surviving non-https URL is plain-text http.
	if !strings.HasPrefix(strings.ToLower(rawURL), "https://") {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "产物 URL 必须为 https（拒绝明文下载）")
	}
	base, err := f.HttpClient()
	if err != nil {
		return nil, errs.NewInternalError(errs.SubtypeSDKError, "构造 http client 失败: %v", err).WithCause(err)
	}
	client := hardenDownloadClient(base)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "非法的产物 URL: %v", err).WithCause(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errs.NewNetworkError(errs.SubtypeNetworkTransport, "下载产物失败: %v", err).WithCause(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errs.NewNetworkError(errs.SubtypeNetworkServer, "下载产物失败: HTTP %d", resp.StatusCode)
	}
	// Read ONE byte past the cap so an oversized body is detected rather than
	// silently truncated: io.LimitReader returns EOF (not an error) at the cap, so
	// reading exactly maxArtifactBytes cannot distinguish "fits" from "overflowed".
	// A body over the cap is refused with a typed error instead of writing a
	// corrupt, partial file that would otherwise report success.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxArtifactBytes+1))
	if err != nil {
		return nil, errs.NewNetworkError(errs.SubtypeNetworkTransport, "读取产物响应失败: %v", err).WithCause(err)
	}
	if int64(len(data)) > maxArtifactBytes {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"产物超过大小上限 %d 字节，拒绝下载（避免写入被截断的残缺文件）", int64(maxArtifactBytes))
	}
	return data, nil
}

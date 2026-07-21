// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agents

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagents "github.com/larksuite/cli/internal/agents"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/validate"
)

// sendOptions holds all inputs for `agents send <ref>`.
type sendOptions struct {
	Factory   *cmdutil.Factory
	Cmd       *cobra.Command
	Ref       string
	Text      string
	Files     []string
	Params    []string
	ContextID string
	TaskID    string
	Answers   []string // raw --answer key=value entries, argv order
	DryRun    bool
	Yes       bool
	As        string
	Format    string
}

// NewCmdAgentSend builds `agents send <agent_ref>`: send a message to a remote
// agent, starting a new task or continuing an existing one. `--dry-run`
// validates the inputs against the agent Card and prints the request preview
// without any API call (always available). A send fires and returns the
// current task immediately; poll progress with
// `agents task get <agent_ref> <task-id> --watch` (surfaced via meta.next).
// `--file` uploads local files to the remote agent — the content leaves this
// machine. Risk=write. runF, when non-nil, replaces the production run path
// (test seam).
func NewCmdAgentSend(f *cmdutil.Factory, runF func(*sendOptions) error) *cobra.Command {
	opts := &sendOptions{Factory: f}
	cmd := &cobra.Command{
		Use:   "send <agent_ref>",
		Short: "Send a message to a remote agent (start a new task or continue an existing one)",
		Long: "Send one message to the remote agent addressed by agent_ref. Without --context-id/--task-id it starts a new task; " +
			"with --context-id (optionally --task-id) it continues the same multi-turn context; with --answer it answers the task's pending input_required question group. " +
			"--dry-run only validates locally and prints the request preview without calling the API. A send fires and returns the current task immediately; " +
			"poll progress with agents task get <agent_ref> <task-id> --watch (see meta.next).",
		Args: exactArgsWithUsage(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(opts.Format); err != nil {
				return err
			}
			opts.Cmd = cmd
			opts.Ref = args[0]
			if runF != nil {
				return runF(opts)
			}
			return agentSendRun(opts)
		},
	}
	cmd.Flags().StringVar(&opts.Text, "text", "", "消息的自由文本部分：起任务/续聊的正文，或随 --answer 的整体附言（--text 永远不是某道题的答案）")
	cmd.Flags().StringArrayVar(&opts.Files, "file", nil, "随消息外发的本地文件路径，可重复；文件会被上传到远端 provider（内容离开本机）")
	addParamFlag(cmd, &opts.Params)
	cmd.Flags().StringVar(&opts.ContextID, "context-id", "", "多轮上下文 id（续发同一会话）")
	cmd.Flags().StringVar(&opts.TaskID, "task-id", "", "向已有任务续发（须与 --context-id 一起用）")
	cmd.Flags().StringArrayVar(&opts.Answers, "answer", nil, "回答 input_required 问题组，可重复：给选项键用 <question_id>=<option_id>（多选重复同 key），给文字用 <question_id>.text=<文本>；须与 --context-id/--task-id 一起用")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "只做本地校验并打印请求预览，不调用 API")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "确认用 --file 把本地文件外发上传到远端（不加则 exit 10，不上传）")
	cmd.Flags().StringVar(&opts.Format, "format", "json", formatFlagHelp)
	cmd.Flags().String("jq", "", "用 jq 表达式过滤 JSON 输出")
	if f != nil {
		cmdutil.AddAPIIdentityFlag(cmd.Context(), cmd, f, &opts.As)
	} else {
		// f is nil only in construction-time unit tests; register a bare --as so
		// the flag surface is still assertable without a Factory.
		cmd.Flags().StringVar(&opts.As, "as", "", "identity type: user | bot")
	}
	cmdutil.SetRisk(cmd, cmdutil.RiskWrite)
	return cmd
}

// sendMode is send's semantic mode, derived from the flags by a fixed priority
// (the user never passes a mode). The discriminator formalizes what the guards
// enforce: answer needs the pending group's task+context and takes --answer
// entries (with --text as an optional message-level remark); continue/start
// need --text.
type sendMode string

const (
	modeStart    sendMode = "start"    // no context/task/answers — a fresh task
	modeContinue sendMode = "continue" // has context (optionally task) — same conversation
	modeAnswer   sendMode = "answer"   // has --answer entries — input_required group reply
)

// answerKeyPattern is the offline --answer key grammar: a KeyPattern-conforming
// question id plus at most one case-sensitive ".text" suffix. Anything else —
// ".txt", ".TEXT", a bare ".text", two dots, a '-'-leading flag-lookalike — is
// rejected before any network access, because the only two legal key shapes are
// <qid> and <qid>.text and a near-miss silently becoming an unknown question_id
// at the provider would send the AI down the wrong recovery branch.
var answerKeyPattern = regexp.MustCompile(`^` + iagents.KeyCharsetRE + `(\.text)?$`)

// parseAnswers parses the raw --answer key=value entries into the §10.1 map
// encoding (values in argv order), running every offline guard in one
// collect-all pass so a multi-error submission is fixed in one round-trip:
// key=value shape, key grammar, non-empty value, no duplicate .text entry per
// question. Exact duplicate bare values are deduplicated (an AI retry glitch is
// idempotent, not an error). Semantic validation (does the qid exist, is the
// value a legal option) is deliberately NOT here — the CLI is stateless and
// does not hold the question group; that is the provider's policy (§6.3).
func parseAnswers(raw []string) (map[string][]string, error) {
	answers := make(map[string][]string, len(raw))
	var viols []string
	for _, entry := range raw {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			viols = append(viols, fmt.Sprintf("%s（非 key=value 形）", entry))
			continue
		}
		if !answerKeyPattern.MatchString(key) {
			viols = append(viols, fmt.Sprintf("%s（key 非法：合法形态只有 <question_id> 与 <question_id>.text）", key))
			continue
		}
		if value == "" {
			viols = append(viols, fmt.Sprintf("%s（空答案无意义：选项题给 option_id、文字给非空文本；不想答的题不要带这个 key）", key))
			continue
		}
		if _, isText := iagents.SplitAnswerKey(key); isText && len(answers[key]) > 0 {
			viols = append(viols, fmt.Sprintf("%s（同一题的 .text 只能出现一次，文本不累积）", key))
			continue
		}
		dup := false
		for _, v := range answers[key] {
			if v == value {
				dup = true // exact duplicate → dedupe silently
				break
			}
		}
		if !dup {
			answers[key] = append(answers[key], value)
		}
	}
	if len(viols) > 0 {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"非法的 --answer: %s", strings.Join(viols, "；")).
			WithParam("--answer").
			WithHint("给选项键用 --answer <question_id>=<option_id>（多选重复同 key），给文字用 --answer <question_id>.text=<文本>，逐条修正后整组重发")
	}
	return answers, nil
}

// deriveSendMode classifies the send and runs the per-mode client-side guards
// (all offline, all holding under a nil Factory). Conflicting combinations
// never silently fall back to another mode. Guard PRECEDENCE is deliberate
// mode-first: with several simultaneous mistakes the mode-defining flag's guard
// wins (e.g. --answer without --context-id reports the answer guard, not the
// missing --text) — the caller learns which MODE it got wrong before which
// field it forgot. Returns the parsed answers map for the answer mode (nil
// otherwise).
func deriveSendMode(opts *sendOptions) (sendMode, map[string][]string, error) {
	if len(opts.Answers) > 0 {
		// answer: continues the pending group's own task, so both ids are required.
		if opts.ContextID == "" || opts.TaskID == "" {
			return "", nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
				"回答问题组需同时提供 --context-id 与 --task-id").
				WithParam("--answer").
				WithHint("--answer 必须与该问题组所属任务的 --context-id/--task-id 一起提供（照抄 task get 输出 meta.next 的命令模板）")
		}
		answers, err := parseAnswers(opts.Answers)
		if err != nil {
			return "", nil, err
		}
		// --text stays optional here: it is the message-level remark, never a
		// question's answer.
		return modeAnswer, answers, nil
	}
	if opts.TaskID != "" && opts.ContextID == "" {
		return "", nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
			"--task-id 需与 --context-id 一起使用").
			WithParam("--task-id").
			WithHint("补充 --context-id <ctx-id> 后重发；该任务所属会话可用 lark-cli agents task get <agent_ref> <task-id> 输出的 context_id 确认")
	}
	if opts.Text == "" {
		return "", nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--text 不能为空").
			WithParam("--text").
			WithHint(`补充 --text "<消息内容>" 后重发；若在回答问题组，用 --answer <question_id>=<option_id> 或 --answer <question_id>.text=<文本>`)
	}
	if opts.ContextID != "" {
		return modeContinue, nil, nil
	}
	return modeStart, nil, nil
}

// agentSendRun validates the send inputs, resolves the provider, and either
// prints a dry-run preview or dispatches the message. The mode guards run
// first so they never touch the network and hold even under a nil Factory. A
// send fires once and returns the current task immediately (exit 0); the
// caller polls progress via the meta.next `task get ... --watch` hint.
func agentSendRun(opts *sendOptions) error {
	_, answers, err := deriveSendMode(opts)
	if err != nil {
		return err
	}
	if err := validateSendFiles(opts.Files); err != nil {
		return err
	}

	f := opts.Factory
	// Resolution + --param validation + --dry-run are fully offline, so they work
	// (and surface validation as exit 2) before the config gate. The card is
	// built with rt=nil (capability matrix only) for the file gate; --param
	// validation reads the send operation's own declaration.
	prov, spec, agentID, id, err := resolveSpec(f, opts.Cmd, opts.Ref, opts.As)
	if err != nil {
		return err
	}
	// Whole-agent brand gate (offline), before the card / any network access.
	if err := brandGate(f, spec, opts.Ref); err != nil {
		return err
	}
	// Send is a core op; gate it on its own Brands too (normally empty ⇒ no-op).
	if err := opBrandGate(f, spec.Send.Brands, opts.Ref, "send"); err != nil {
		return err
	}
	card := iagents.BuildCard(opts.Cmd.Context(), prov, spec, agentID, resolvedBrand(f), nil)
	vp, err := validateParams(opts.Params, spec.Send.Params, iagents.VerbSend, spec, opts.Ref)
	if err != nil {
		return err
	}

	in := iagents.SendInput{
		Text:      opts.Text,
		Files:     opts.Files,
		ContextID: opts.ContextID,
		TaskID:    opts.TaskID,
		Answers:   answers,
	}

	// --dry-run is a client-side behavior: always available, never
	// gated by the Card's dry_run capability, and never touches the API.
	if opts.DryRun {
		return emitDryRun(f, opts.Cmd, opts.Ref, in, vp.Resolved, opts.Format)
	}

	// An agent that never enters input_required cannot take a group answer, so
	// --answer against it is unsupported_capability — gated offline (mirrors the
	// --file/file_input gate) to save the caller a doomed round-trip.
	if len(in.Answers) > 0 && !card.Supports(iagents.CapInputRequired) {
		return capabilityError(opts.Ref, "send with --answer", iagents.CapInputRequired)
	}

	if len(in.Files) > 0 {
		// An agent that does not declare file_input cannot take an upload, so
		// --file against it is unsupported_capability — gated before any network
		// access, so the user is not told "confirm the upload" for a send that
		// would be rejected anyway.
		if !card.Supports(iagents.CapFileInput) {
			return capabilityError(opts.Ref, "send with --file", iagents.CapFileInput)
		}
		// --file exfiltrates local file content off this machine (the provider
		// reads the file and uploads it to the remote agent). That is an
		// irreversible, CLI-enforced high-risk write: a real send that would upload
		// requires --yes, returning confirmation_required (exit 10) before any
		// network access. dry-run above is exempt — it never uploads.
		if !opts.Yes {
			return errs.NewConfirmationRequiredError(errs.RiskHighRiskWrite, "agents send --file",
				"--file 会把本地文件外发上传到远端 agent（内容离开本机，不可撤回）").
				WithHint("确认要外发这些文件后，加 --yes 重发")
		}
	}

	// A real send calls the API, so it needs a configured client; build the
	// identity-pinned runtime now (not_configured / exit 3 here is correct).
	rt, err := runtimeFor(f, id, agentID, vp.Resolved)
	if err != nil {
		return err
	}

	// Local scope preflight: after runtimeFor, before the API call. The check is
	// all-or-nothing — any real API verb requires the provider's full scope set.
	if err := preflightScopesForRef(f, id, opts.Ref); err != nil {
		return err
	}

	task, err := spec.Send.Handler(opts.Cmd.Context(), rt, in)
	if err != nil {
		return err
	}
	notice := normalizeTask(task)

	// A send fires and returns the current task immediately (exit 0). Progress is
	// polled separately via the meta.next `task get <agent_ref> <task-id> --watch`
	// hint — send no longer blocks on the task reaching a stop condition.
	return emitTask(f, opts.Cmd, task, nextForTask(opts.Ref, task, spec, vp.Given, iagents.VerbSend), opts.Format, notice)
}

// validateSendFiles is the local gate on --file paths, running before any
// capability/confirmation gate or network access (dry-run included): every
// path must be a relative-within-CWD (the lark-shared safety rule the docs
// promise) EXISTING regular file. Violations are collected and reported in one
// pass, mirroring the --param collect-all style, so a multi-file send is fixed
// in one round-trip. Without this gate a bad path used to be discovered only
// by the provider (or worse, silently "uploaded").
func validateSendFiles(files []string) error {
	var viols []string
	for _, p := range files {
		abs, err := validate.SafeInputPath(p)
		if err != nil {
			viols = append(viols, fmt.Sprintf("%s（仅接受 CWD 内的相对路径）", p))
			continue
		}
		st, err := os.Stat(abs)
		switch {
		case err != nil:
			viols = append(viols, fmt.Sprintf("%s（文件不存在或不可读）", p))
		case st.IsDir():
			viols = append(viols, fmt.Sprintf("%s（是目录，--file 只接受文件）", p))
		}
	}
	if len(viols) == 0 {
		return nil
	}
	return errs.NewValidationError(errs.SubtypeInvalidArgument,
		"非法的 --file 路径: %s", strings.Join(viols, "；")).
		WithParam("--file").
		WithHint("--file 只接受当前目录内的相对路径且文件必须存在，逐条修正后重发")
}

// emitDryRun writes the dry-run preview: {dry_run:true, would_send:{…}}
// reconstructed from the validated input, so a caller can inspect exactly what
// a real send would post without contacting the agent. format=pretty (no --jq)
// renders the same fields as key: value lines instead of the envelope.
func emitDryRun(f *cmdutil.Factory, cmd *cobra.Command, ref string, in iagents.SendInput, params map[string]string, format string) error {
	if format == "pretty" && jqExpr(cmd) == "" {
		out := f.IOStreams.Out
		fmt.Fprintln(out, "dry_run: true")
		fmt.Fprintf(out, "agent_ref: %s\n", kvValue(ref))
		fmt.Fprintf(out, "text: %s\n", truncateRunes(kvValue(in.Text), 120))
		if len(in.Files) > 0 {
			fmt.Fprintf(out, "files: %d\n", len(in.Files))
		}
		if len(params) > 0 {
			fmt.Fprintf(out, "params: %d\n", len(params))
		}
		if in.ContextID != "" {
			fmt.Fprintf(out, "context_id: %s\n", kvValue(in.ContextID))
		}
		if in.TaskID != "" {
			fmt.Fprintf(out, "task_id: %s\n", kvValue(in.TaskID))
		}
		if len(in.Answers) > 0 {
			fmt.Fprintf(out, "answers: %d\n", len(in.Answers))
		}
		return nil
	}

	would := map[string]interface{}{
		"agent_ref": ref,
		"text":      in.Text,
	}
	if len(in.Files) > 0 {
		would["files"] = in.Files
	}
	if len(params) > 0 {
		// Default 回填后的终值：预演即所得。
		would["params"] = params
	}
	if in.ContextID != "" {
		would["context_id"] = in.ContextID
	}
	if in.TaskID != "" {
		would["task_id"] = in.TaskID
	}
	if len(in.Answers) > 0 {
		// §10.1 键编码原样预览：预演即所得。
		would["answers"] = in.Answers
	}
	env := output.Envelope{
		OK:       true,
		Identity: string(f.ResolvedIdentity),
		Data: map[string]interface{}{
			"dry_run":    true,
			"would_send": would,
		},
		Notice: output.GetNotice(),
	}
	if jq := jqExpr(cmd); jq != "" {
		return output.JqFilter(f.IOStreams.Out, env, jq)
	}
	output.PrintJson(f.IOStreams.Out, env)
	return nil
}

// nextIDPattern is the character whitelist for server-supplied identifiers
// (task_id / context_id / question_id) before they are interpolated into a
// meta.next command string: first character alphanumeric, then letters, digits,
// '_' and '-'. It is deliberately stricter than validate.ResourceName — that
// check is a denylist aimed at URL-path safety and would pass shell
// metacharacters (spaces, ';', backticks, quotes), which are exactly what
// matters here: meta.next is defined as "AI executes this verbatim", so a
// server-controlled id is a command-injection surface. The alphanumeric first
// character additionally rejects flag-lookalike ids ("--text", "-o") that would
// survive a bare charset test yet hijack the flag surface when an AI re-composes
// the command. It matches iagents.KeyPattern by construction — the two layers
// must agree or a key accepted at one becomes a dead end at the other.
var nextIDPattern = regexp.MustCompile(`^` + iagents.KeyCharsetRE + `$`)

// safeNextID reports whether s may be interpolated into a meta.next command.
func safeNextID(s string) bool {
	return nextIDPattern.MatchString(s)
}

// nextRefPattern is the whitelist for a user-supplied ref before it is
// interpolated into a meta.next command or a hint command string: the
// safeNextID charset on both sides of exactly one ':' (the <scheme>:<agent_id>
// shape ParseRef accepts, further restricted to command-safe characters). A
// ref is not server-controlled — the threat model is not injection but
// copy-paste breakage (a ref with spaces/quotes yields a command that cannot
// be executed verbatim), so a failing ref simply drops the command hint.
var nextRefPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+:[A-Za-z0-9_-]+$`)

// safeNextRef reports whether ref may be interpolated into a meta.next / hint
// command string.
func safeNextRef(ref string) bool {
	return nextRefPattern.MatchString(ref)
}

// nextForTask builds the meta.next[] hints for a send result: a terminal task
// suggests fetching its artifacts / detail, a still-running task the poll
// command, an input_required task the continue command, and an auth_required
// task the re-authorize flow (auth login, not a text continuation). AI callers use
// these to chain the next step without guessing the command shape, so every
// value interpolated here must pass its whitelist first: the ref (safeNextRef)
// and the task_id (safeNextID) each suppress the whole hint when they fail
// (prefer dropping the hint over risking injection); a failing context_id
// degrades to the <context_id> placeholder,
// which keeps the hint while interpolating nothing untrusted. A hint whose
// command carries <...> placeholders is marked Template so callers know it
// needs substitution before execution.
// nextForTask additionally carries business parameters for the TARGET verb of
// each suggested command per the three-way rule (see paramArgsFor): given
// values that pass the whitelist ride literally, whitelist failures degrade
// required params to placeholders, and target-verb-required params the caller
// never provided are added as placeholders — so a required parameter is
// structurally incapable of falling off the chain. given is what the caller
// explicitly provided this call (never backfilled defaults); spec may be nil
// in construction-time tests (no params are carried then).
// caller is the verb that produced this output: a terminal task viewed via
// task get must NOT suggest the very command the caller just ran (a naive AI
// following meta.next verbatim would loop on itself); the artifact downloads
// remain the only genuine increment there.
func nextForTask(ref string, task *iagents.AgentTask, spec *iagents.AgentSpec, given map[string]string, caller string) []output.NextAction {
	if !safeNextRef(ref) {
		return nil
	}
	if task == nil || task.TaskID == "" || !safeNextID(task.TaskID) {
		return nil
	}
	if task.State.ShouldStopPolling() {
		if task.State == iagents.StateAuthRequired {
			// auth_required is an agent-side task state — the end user must
			// (re)authorize in the agent (see the SKILL state semantics), NOT a CLI scope error and
			// NOT a text continuation like input_required. Point at the auth
			// re-authorize flow instead of a text continuation. The concrete scopes are the
			// agent's declared scope set (see the lark-agents skill's prerequisites), so --scope is a
			// placeholder → Template. ref/task_id are already whitelisted above, so
			// echoing the re-check command in the label is safe.
			// label 内嵌的重查命令按三分规则补 task_get 的参数携带——auth_required
			// 是唯一不指向 agent 子树的 next，链传规则同样不许在这条路上丢必填。
			recheckArgs, _ := paramArgsFor(spec, iagents.VerbTaskGet, given)
			return []output.NextAction{{
				Label:    fmt.Sprintf("完成重新授权后重查任务（据该 agent 所需 scope 定；重查: lark-cli agents task get %s %s%s）", ref, task.TaskID, recheckArgs),
				Command:  `lark-cli auth login --scope "<required_scopes>"`,
				Template: true,
			}}
		}
		if task.State == iagents.StateInputRequired {
			// A task pausing on a question group: expand ONE per-question template
			// (design doc §4.4) so the AI never hand-assembles the answer grammar —
			// the placeholder names the answer form per question type (bare
			// <option_id> for a choice, marked repeatable for multi-select,
			// .text=<文本> for free text). All values are placeholders, so the hint
			// is always a template — which is also why a missing or
			// whitelist-failing context_id can degrade to the <context_id>
			// placeholder instead of dropping the hint. Every question_id is
			// server-supplied and must pass the safeNextID whitelist before
			// interpolation (normalization upstream guarantees this; a violation
			// here degrades to the free-text continuation rather than emitting a
			// key the CLI's own guard would reject).
			ctxID := task.ContextID
			if ctxID == "" || !safeNextID(ctxID) {
				ctxID = "<context_id>"
			}
			sendArgs, _ := paramArgsFor(spec, iagents.VerbSend, given)
			if ir := task.InputRequired; ir != nil && len(ir.Questions) > 0 {
				parts := make([]string, 0, len(ir.Questions))
				for _, q := range ir.Questions {
					if !safeNextID(q.QuestionID) {
						parts = nil
						break
					}
					switch {
					case len(q.Options) == 0:
						parts = append(parts, fmt.Sprintf("--answer %s.text=<文本>", q.QuestionID))
					case q.MultiSelect:
						parts = append(parts, fmt.Sprintf("--answer %s=<option_id 多选可重复>", q.QuestionID))
					default:
						parts = append(parts, fmt.Sprintf("--answer %s=<option_id>", q.QuestionID))
					}
				}
				if parts != nil {
					return []output.NextAction{{
						Label:    "把问题组转达给用户后按其答复提交（用户先前指令已唯一确定答案时可代答，须说明依据）；选项都不合适的题用 <question_id>.text=<文本>",
						Command:  fmt.Sprintf("lark-cli agents send %s --context-id %s --task-id %s %s%s", ref, ctxID, task.TaskID, strings.Join(parts, " "), sendArgs),
						Template: true,
					}}
				}
			}
			// No structured group (provider supplied none and normalization had
			// nothing to synthesize from): plain free-text continuation — the
			// provider treats a message to its paused task as the answer (§6.5).
			return []output.NextAction{{
				Label:    "补充输入后向同一任务续发",
				Command:  fmt.Sprintf("lark-cli agents send %s --context-id %s --task-id %s --text <你的答复>%s", ref, ctxID, task.TaskID, sendArgs),
				Template: true,
			}}
		}
		// Terminal: suggest reading the final detail, plus a ready-made download
		// command per artifact (so the AI never has to hand-craft the
		// `task get --artifact` form itself; -o stays a placeholder → template).
		// When the caller IS task get, the detail suggestion would be a self-loop
		// (the exact command just executed) — drop it and keep only the artifact
		// increments.
		var next []output.NextAction
		if caller != iagents.VerbTaskGet {
			getArgs, getTpl := paramArgsFor(spec, iagents.VerbTaskGet, given)
			next = append(next, output.NextAction{
				Label:    "查看任务详情与产物",
				Command:  fmt.Sprintf("lark-cli agents task get %s %s%s", ref, task.TaskID, getArgs),
				Template: getTpl,
			})
		}
		next = append(next, artifactNext(ref, task, spec, given)...)
		return next
	}
	getArgs, getTpl := paramArgsFor(spec, iagents.VerbTaskGet, given)
	return []output.NextAction{{
		Label:    "轮询任务直到停轮询条件（有界；到点未终止照此再 watch）",
		Command:  fmt.Sprintf("lark-cli agents task get %s %s --watch --timeout %s%s", ref, task.TaskID, defaultWatchTimeout, getArgs),
		Template: getTpl,
	}}
}

// artifactNext builds one ready-made download command per artifact of a
// terminal task: only when the spec wires DownloadArtifact, only for artifact
// ids that pass the whitelist (a failing id skips just that artifact), always
// template (the -o save path is the caller's choice). Params carry per the
// three-way rule against the artifact_download declaration.
func artifactNext(ref string, task *iagents.AgentTask, spec *iagents.AgentSpec, given map[string]string) []output.NextAction {
	if spec == nil || !task.IsTerminal || len(task.Artifacts) == 0 {
		return nil
	}
	if op, ok := spec.Op(iagents.VerbArtifactDownload); !ok || !op.Wired {
		return nil
	}
	dlArgs, _ := paramArgsFor(spec, iagents.VerbArtifactDownload, given)
	var next []output.NextAction
	for _, a := range task.Artifacts {
		if a.ID == "" || !safeNextID(a.ID) {
			continue // 服务端 id 过不了白名单 → 跳过该产物，不冒注入险
		}
		next = append(next, output.NextAction{
			// label 只内插已过白名单的 id；产物名是 agent 可控文本，不进 label。
			Label:    fmt.Sprintf("下载产物 %s", a.ID),
			Command:  fmt.Sprintf("lark-cli agents task get %s %s --artifact %s -o <保存路径>%s", ref, task.TaskID, a.ID, dlArgs),
			Template: true,
		})
	}
	return next
}

// defaultWatchTimeout is the bounded poll window meta.next suggests for a
// still-running task: a safe default that avoids an unbounded --watch blocking
// forever on a long task and stops an AI caller from self-hammering. On expiry
// the poll returns the current state (exit 0) plus a fresh watch hint, so the
// caller re-watches in segments rather than blocking once. `--watch` used alone
// (--timeout 0) stays unbounded for backward compatibility.
const defaultWatchTimeout = 30 * time.Second

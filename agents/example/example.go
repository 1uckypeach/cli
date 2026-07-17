// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package example is the in-repo agent provider onboarding template and offline
// demo backend: a hypothetical example business domain whose data / calls are
// entirely in-memory mocks, with zero network. It has three roles:
//
//  1. A copy-start point for new integrators — copy the package, rename the
//     scheme, write plain hook funcs, add one line to agent/register.go. There is
//     no Factory, no Deps, no probe, no Kind field.
//  2. The command tree's offline demo backend — the full agent
//     list/card/send/task/context chain runs for real without any platform config.
//  3. A stable mock scheme for cmd-layer tests.
//
// The whole provider is a declarative agents.Provider value: metadata + a catalog
// of agents.AgentSpec units. Each spec's capability set is exactly the hooks it
// wires (the framework derives the card matrix from that), so echo (minimal) and
// reporter (full) differ by DATA, not by a Factory branch.
package example

import (
	"context"
	"fmt"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/agents"
	"github.com/larksuite/cli/internal/core"
)

// Provider is the whole declaration. The Catalog set makes this a catalog-type
// provider; the framework derives enumeration (agents list example), the
// unknown-id error, and each agent's card matrix from this data.
func Provider() agents.Provider {
	return agents.Provider{
		Scheme:        "example",
		Label:         "Example 演示 agent（内存 mock，零网络）",
		AgentIDSource: "运行 lark-cli agents list example 查看内置演示 agent 及其 agent_ref（无需任何平台配置）",
		Identities:    []agents.IdentitySpec{{Type: agents.IdentityUser}, {Type: agents.IdentityBot}},
		// RequiredScopes nil: the mock calls no OAPI, so scope preflight always passes.
		Catalog: []agents.AgentSpec{echoSpec, reporterSpec, plannerSpec},
	}
}

// echoSpec is the minimal set: it wires Send/GetTask plus the read verbs and
// NOTHING else, so its card honestly shows task_cancel / artifact_download /
// file_input = false. Capability IS exactly the wired hooks — there is no bool
// matrix and no capability-refusal code (the command layer gates unwired hooks).
var echoSpec = agents.AgentSpec{
	ID:            "echo",
	Name:          "复读机",
	Description:   "把你发的话原样复读一遍（同一会话续发时带轮次，证明上下文记忆）。最小能力集示范。",
	Send:          agents.SendOp{Handler: echoSend},
	GetTask:       agents.TaskGetOp{Handler: getTask},
	ListTasks:     agents.TaskListOp{Handler: listTasks},
	ListContexts:  agents.ContextListOp{Handler: listContexts},
	GetContext:    agents.ContextGetOp{Handler: getContext},
	DeleteContext: agents.ContextDeleteOp{Handler: deleteContext},
}

// reporterSpec is the full set: it additionally wires CancelTask +
// DownloadArtifact and declares the FileInput/InputRequired behavioral flags. The
// difference between the two agents is data you read top-to-bottom, not a branch
// inside a Factory.
// reporterSendParams is reporter's typed view of its send params — the
// BindParams copy-start template. agenttest.CheckParamsBinding locks the tags
// against the declaration below in example_test.go.
type reporterSendParams struct {
	ReportFormat string `param:"report_format"`
	Quarters     int64  `param:"quarters"`
	// Render binds the object param's leaves（点路径/JSON 两通道归一后的
	// "render.*" 键）——嵌套 struct + tag 即完成拼装。
	Render renderOpts `param:"render"`
}

type renderOpts struct {
	Theme     string `param:"theme"`
	Watermark bool   `param:"watermark"`
}

var reporterSpec = agents.AgentSpec{
	ID:            "reporter",
	Name:          "报表生成器",
	Description:   "对任意请求产出一份内联 CSV 报表 artifact，示范 artifact 下载与任务取消链路。",
	FileInput:     true,
	InputRequired: true,
	// Send declares demo business params covering the whole declaration
	// surface: enum + default (report_format), integer + min/max + default
	// (quarters). Both optional with defaults, so a bare send behaves exactly
	// like before — the params exist to be a copy-start template and to make
	// the validation/card/meta.next chain exercisable offline.
	Send: agents.SendOp{
		Params: []agents.CardParam{
			{Name: "report_format", Enum: []string{"csv", "xlsx"}, Default: "csv",
				Desc: "报表输出格式"},
			{Name: "quarters", Type: "integer", Min: agents.Float(1), Max: agents.Float(12), Default: "4",
				Desc: "回溯季度数"},
			// object 参数演示：点路径 --param render.theme=dark 或 JSON 整值
			// --param render='{"theme":"dark"}' 两通道等价，框架归一后 hook 只见
			// 平铺 "render.*" 键。
			{Name: "render", Type: "object", Desc: "渲染选项", Fields: []agents.CardParam{
				{Name: "theme", Enum: []string{"light", "dark"}, Default: "light", Desc: "配色主题"},
				{Name: "watermark", Type: "boolean", Default: "false", Desc: "是否加水印"},
			}},
		},
		Handler: reporterSend,
	},
	GetTask:       agents.TaskGetOp{Handler: getTask},
	ListTasks:     agents.TaskListOp{Handler: listTasks},
	ListContexts:  agents.ContextListOp{Handler: listContexts},
	GetContext:    agents.ContextGetOp{Handler: getContext},
	DeleteContext: agents.ContextDeleteOp{Handler: deleteContext},
	// task_cancel is scoped to feishu — a real brand-scoped capability demo:
	// under lark reporter's card shows task_cancel=false and
	// `agents task cancel example:reporter` is gated with unavailable_for_brand
	// (the whole agent stays visible under both brands — only this op is scoped).
	CancelTask:       agents.TaskCancelOp{Brands: []core.LarkBrand{core.BrandFeishu}, Handler: cancelTask},
	DownloadArtifact: agents.ArtifactDownloadOp{Handler: downloadArtifact},
}

// plannerSpec demonstrates the input_required HITL flow: the first send opens a
// single-select decision and the task stays non-terminal in input_required;
// answering it with --decision-id/--option completes the task, and a second
// answer to the same decision returns a conflict (the "already arbitrated"
// path). It wires the read verbs but not cancel/artifact.
var plannerSpec = agents.AgentSpec{
	ID:            "planner",
	Name:          "报表规划器",
	Description:   "先反问「按什么维度拆」（input_required 单选决策），你用 --decision-id/--option 选定后再出报表。示范 HITL 决策链路。",
	InputRequired: true,
	Send:          agents.SendOp{Handler: plannerSend},
	GetTask:       agents.TaskGetOp{Handler: getTask},
	ListTasks:     agents.TaskListOp{Handler: listTasks},
	ListContexts:  agents.ContextListOp{Handler: listContexts},
	GetContext:    agents.ContextGetOp{Handler: getContext},
	DeleteContext: agents.ContextDeleteOp{Handler: deleteContext},
}

// plannerSend opens a decision on a fresh request, or applies the answer when
// --decision-id + --option is supplied (continuing the decision's task).
func plannerSend(ctx context.Context, rt agents.Runtime, in agents.SendInput) (*agents.AgentTask, error) {
	if in.DecisionID != "" {
		if len(in.OptionIDs) != 1 {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
				"planner 的决策是单选，请用恰好一个 --option").WithParam("--option")
		}
		task, err := store.answerDecision(rt.AgentID(), in.TaskID, in.DecisionID, in.OptionIDs[0])
		if err != nil {
			return nil, err
		}
		return &task, nil
	}
	ctxID := in.ContextID
	if ctxID == "" {
		var err error
		ctxID, err = store.createContext(rt.AgentID(), truncateTitle(in.Text))
		if err != nil {
			return nil, err
		}
	}
	task, err := store.createTask(rt.AgentID(), ctxID, func(int) agents.AgentTask {
		return agents.AgentTask{
			TaskID:    newID("task"),
			ContextID: ctxID,
			State:     agents.StateInputRequired,
			Messages: []agents.Message{
				{Role: "user", Parts: []agents.Part{{Type: "text", Text: in.Text}}},
				{Role: "agent", Parts: []agents.Part{{Type: "text", Text: "先确认口径：报表按什么维度拆分？"}}},
			},
			InputRequired: &agents.InputRequired{
				DecisionID: newID("dec"),
				Prompt:     "报表按什么维度拆分？",
				InputType:  agents.InputTypeSingleSelect,
				Options: []agents.Option{
					{OptionID: "by_region", Label: "按大区"},
					{OptionID: "by_category", Label: "按品类"},
				},
			},
		}
	})
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// ── Hooks: plain funcs. The addressed agent comes from rt.AgentID() (request
//    data, replacing the old state.agentID). The mock ignores rt's network
//    methods (CallAPI/CallMultipart/IsBot). There is NO catalog.Lookup guard
//    anywhere — the framework's LookupSpec validated ref→spec offline before
//    dispatch, so an unknown id never reaches a hook. ──

// echoSend echoes the input; from round 2 on it appends a round marker to prove
// across commands that context memory works.
func echoSend(ctx context.Context, rt agents.Runtime, in agents.SendInput) (*agents.AgentTask, error) {
	return newTurn(rt.AgentID(), in, func(round int) (string, []agents.Artifact) {
		reply := in.Text
		if round > 1 {
			reply = fmt.Sprintf("%s（第 %d 轮）", in.Text, round)
		}
		return reply, nil
	})
}

// reporterSend produces a fixed inline CSV artifact for any request. It reads
// its demo params through BindParams — the typed, compile-checked consumption
// template (rt.Params() raw lookups work too but are typo-prone). With the
// declaration defaults (csv/4) the reply is byte-identical to the historical
// one; a hook invoked outside the framework (unit tests calling it directly)
// sees an empty param map and the same historical reply.
func reporterSend(ctx context.Context, rt agents.Runtime, in agents.SendInput) (*agents.AgentTask, error) {
	p, err := agents.BindParams[reporterSendParams](rt)
	if err != nil {
		return nil, err
	}
	return newTurn(rt.AgentID(), in, func(round int) (string, []agents.Artifact) {
		reply := "报表已生成：quarterly_report.csv（见 artifacts，用 task get --artifact <id> -o <path> 下载）"
		if p.ReportFormat != "" && p.ReportFormat != "csv" {
			reply = fmt.Sprintf("报表已生成（%s 格式，回溯 %d 个季度）：quarterly_report.%s（见 artifacts，用 task get --artifact <id> -o <path> 下载）",
				p.ReportFormat, p.Quarters, p.ReportFormat)
		}
		if p.Render.Watermark {
			reply = fmt.Sprintf("%s（%s 主题，含水印）", reply, p.Render.Theme)
		}
		if n := len(in.Files); n > 0 {
			reply = fmt.Sprintf("已收到 %d 个附件；%s", n, reply)
		}
		// Name/Mime 在 GetTask 阶段就可见（下载前），调用方能直接据此定 -o 后缀，
		// 不必先猜再靠下载后的 suggested_name 纠正——真实 provider 应尽量同样前置。
		ext, mime := "csv", "text/csv"
		if p.ReportFormat == "xlsx" {
			ext, mime = "xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
		return reply, []agents.Artifact{{ID: newID("art"), Kind: "text", Name: "quarterly_report." + ext, Mime: mime}}
	})
}

// newTurn factors the shared store flow: start/continue a context, then create a
// task whose body the caller builds per round. The mock task is instantly
// terminal, so there is no "feed input to a running task" scenario — continuing
// via --task-id returns failed_precondition (the request is valid but the target
// state does not satisfy it, so the AI knows to start a new task instead).
func newTurn(agentID string, in agents.SendInput, build func(round int) (reply string, artifacts []agents.Artifact)) (*agents.AgentTask, error) {
	if in.TaskID != "" {
		return nil, errs.NewValidationError(errs.SubtypeFailedPrecondition,
			"example 的任务发出即完成（终态），无法向已有任务续发").
			WithParam("--task-id").
			WithHint("去掉 --task-id，用 --context-id 在同一会话起新一轮任务")
	}
	ctxID := in.ContextID
	if ctxID == "" {
		var err error
		ctxID, err = store.createContext(agentID, truncateTitle(in.Text))
		if err != nil {
			return nil, err
		}
	}
	// createTask validates context ownership under the lock (an unknown /
	// cross-agents context id is rejected inside with a typed error), computes the
	// round, and inserts atomically.
	task, err := store.createTask(agentID, ctxID, func(round int) agents.AgentTask {
		reply, artifacts := build(round)
		return agents.AgentTask{
			TaskID:     newID("task"),
			ContextID:  ctxID,
			State:      agents.StateCompleted,
			IsTerminal: true,
			Messages: []agents.Message{
				{Role: "user", Parts: []agents.Part{{Type: "text", Text: in.Text}}},
				{Role: "agent", Parts: []agents.Part{{Type: "text", Text: reply}}},
			},
			Artifacts: artifacts,
		}
	})
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func getTask(ctx context.Context, rt agents.Runtime, taskID string) (*agents.AgentTask, error) {
	task, err := store.getTask(rt.AgentID(), taskID)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func listTasks(ctx context.Context, rt agents.Runtime, contextID string, page agents.PageParams) ([]agents.TaskSummary, agents.PageInfo, error) {
	tasks, info := store.listTasks(rt.AgentID(), contextID, page)
	return tasks, info, nil
}

func listContexts(ctx context.Context, rt agents.Runtime, page agents.PageParams) ([]agents.ContextSummary, agents.PageInfo, error) {
	ctxs, info := store.listContexts(rt.AgentID(), page)
	return ctxs, info, nil
}

func getContext(ctx context.Context, rt agents.Runtime, ctxID string) (*agents.ContextDetail, error) {
	return store.getContext(rt.AgentID(), ctxID)
}

func deleteContext(ctx context.Context, rt agents.Runtime, ctxID string) error {
	return store.deleteContext(rt.AgentID(), ctxID)
}

// cancelTask is wired only for reporter, so echo never reaches it (the command
// layer gates echo's cancel on the nil field). The mock task is completed the
// moment it is sent, so canceling a terminal task returns a failed_precondition
// typed error rather than pretending success.
func cancelTask(ctx context.Context, rt agents.Runtime, taskID string) error {
	task, err := store.getTask(rt.AgentID(), taskID)
	if err != nil {
		return err
	}
	if task.State.IsTerminal() {
		return errs.NewValidationError(errs.SubtypeFailedPrecondition,
			"任务 '%s' 已处于终态 %s，无法取消", taskID, task.State).
			WithHint("终态任务不可取消；用 lark-cli agents task get example:%s %s 查看结果", rt.AgentID(), taskID)
	}
	return store.setTaskState(taskID, agents.StateCanceled)
}

// reportCSV is the fixed content of the reporter artifact (inline Bytes type).
const reportCSV = "quarter,revenue,cost,margin\n" +
	"2026Q1,1250,830,0.336\n" +
	"2026Q2,1410,905,0.358\n"

// downloadArtifact is wired only for reporter (echo is gated on the nil field).
// It returns inline Bytes; a real provider would fill URL instead and let the
// command layer SSRF-validate + fetch.
//
// Teaching point (suggested_name): ArtifactData.Name is the server-suggested
// file name, echoed back only as a reference for choosing -o — it is untrusted
// and never participates in constructing the local save path (the save path is
// always -o/SafeOutputPath).
func downloadArtifact(ctx context.Context, rt agents.Runtime, taskID, artifactID string) (*agents.ArtifactData, error) {
	task, err := store.getTask(rt.AgentID(), taskID)
	if err != nil {
		return nil, err
	}
	for _, a := range task.Artifacts {
		if a.ID == artifactID {
			return &agents.ArtifactData{
				Name:  "quarterly_report.csv",
				Mime:  "text/csv",
				Bytes: []byte(reportCSV),
			}, nil
		}
	}
	return nil, errs.NewValidationError(errs.SubtypeInvalidArgument,
		"任务 '%s' 名下没有产物 '%s'", taskID, artifactID).
		WithHint("运行 lark-cli agents task get example:%s %s 查看该任务的 artifacts", rt.AgentID(), taskID)
}

// truncateTitle takes the first few characters of the message as the context
// title (truncated by rune to avoid cutting a character in half).
func truncateTitle(s string) string {
	const max = 20
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

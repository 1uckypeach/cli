// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package base exposes the fixed Base assistant through the provider-neutral
// agents SPI. Adapter-specific wire details intentionally stay in this package.
package base

import (
	"context"

	"github.com/larksuite/cli/errs"
	iagents "github.com/larksuite/cli/internal/agents"
)

// Base Agent scope is checked before every real API operation. Both the Open
// Platform app and the user token must grant it.
const baseAgentExecuteScope = "base:agent:execute"

// adapterAgentID is deliberately private: callers always use base:assistant,
// and a future Adapter-side ID change is isolated to this mapping.
const adapterAgentID = "assistant"

type sendParams struct {
	BaseToken     string `param:"base_token"`
	ActiveTableID string `param:"active_table_id"`
}

type baseTokenParams struct {
	BaseToken string `param:"base_token"`
}

type getTaskParams struct {
	BaseToken string `param:"base_token"`
	ContextID string `param:"context_id"`
}

type listTasksParams struct {
	BaseToken string `param:"base_token"`
	State     string `param:"state"`
}

type listContextsParams struct {
	BaseToken string `param:"base_token"`
	Status    string `param:"status"`
}

func baseTokenParam() []iagents.CardParam {
	return []iagents.CardParam{{Name: "base_token", Required: true, Desc: "Base app token"}}
}

func getTaskParamList() []iagents.CardParam {
	return []iagents.CardParam{
		{Name: "base_token", Required: true, Desc: "Base app token"},
		{Name: "context_id", Desc: "Optional context override used to retrieve the task's message snapshot"},
	}
}

func sendParamList() []iagents.CardParam {
	return []iagents.CardParam{
		{Name: "base_token", Required: true, Desc: "Base app token"},
		{Name: "active_table_id", Desc: "Optional active table for automatic routing"},
	}
}

func listTasksParamList() []iagents.CardParam {
	return []iagents.CardParam{
		{Name: "base_token", Required: true, Desc: "Base app token"},
		{Name: "state", Enum: []string{"running", "done", "failed"}, Desc: "Adapter task state"},
	}
}

func listContextsParamList() []iagents.CardParam {
	return []iagents.CardParam{
		{Name: "base_token", Required: true, Desc: "Base app token"},
		{Name: "status", Desc: "Adapter context status"},
	}
}

var assistantSpec = iagents.AgentSpec{
	ID:          "assistant",
	Name:        "Base Assistant",
	Description: "Automatically routes Base creation, dashboard, workflow, and data questions to the appropriate capability.",
	Skills: []iagents.CardSkill{
		{ID: "base_build", Name: "Build a Base", Examples: []string{"Create a project tracker with owners and due dates"}},
		{ID: "base_analyze", Name: "Analyze Base data", Examples: []string{"Summarize this table and highlight anomalies"}},
	},
	Send:          iagents.SendOp{Params: sendParamList(), Handler: send},
	GetTask:       iagents.TaskGetOp{Params: getTaskParamList(), Handler: getTask},
	ListTasks:     iagents.TaskListOp{Params: listTasksParamList(), Handler: listTasks},
	CancelTask:    iagents.TaskCancelOp{Params: baseTokenParam(), Handler: cancelTask},
	ListContexts:  iagents.ContextListOp{Params: listContextsParamList(), Handler: listContexts},
	GetContext:    iagents.ContextGetOp{Params: baseTokenParam(), Handler: getContext},
	DeleteContext: iagents.ContextDeleteOp{Params: baseTokenParam(), Handler: deleteContext},
	FileInput:     false,
	InputRequired: false,
}

// Provider returns the single offline-discoverable Base assistant.
func Provider() iagents.Provider {
	return iagents.Provider{
		Scheme:         "base",
		Label:          "Base Assistant",
		AgentIDSource:  "Use the fixed agent reference base:assistant",
		RequiredScopes: []string{baseAgentExecuteScope},
		Identities:     []iagents.IdentitySpec{{Type: iagents.IdentityUser}},
		Catalog:        []iagents.AgentSpec{assistantSpec},
	}
}

func validateSendRuntime(rt iagents.Runtime, in iagents.SendInput) error {
	if rt.IsBot() {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"base:assistant currently supports only user identity").WithParam("--as").
			WithHint("run the command with --as user")
	}
	if len(in.Files) > 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"base:assistant does not support file input").WithParam("--file")
	}
	if len(in.Answers) > 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"base:assistant does not support structured input_required answers").WithParam("--answer")
	}
	return nil
}

func send(ctx context.Context, rt iagents.Runtime, in iagents.SendInput) (*iagents.AgentTask, error) {
	if err := validateSendRuntime(rt, in); err != nil {
		return nil, err
	}
	return sendMessage(ctx, rt, in)
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package base

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strconv"

	"github.com/larksuite/cli/errs"
	iagents "github.com/larksuite/cli/internal/agents"
)

const baseAgentServicePath = "/open-apis/base/v3"

func callPayload[T any](ctx context.Context, rt iagents.Runtime, method, path string, query map[string]string, body any) (T, error) {
	return iagents.Call[T](ctx, rt, method, path, query, body)
}

func segment(v string) string { return url.PathEscape(v) }

func agentRoot(baseToken string) string {
	return baseAgentServicePath + "/bases/" + segment(baseToken) + "/ai/agents/" + segment(adapterAgentID)
}

func randomIdempotencyKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", errs.NewInternalError(errs.SubtypeUnknown,
			"generate Base Agent idempotency key: %v", err).WithCause(err)
	}
	return "lark-cli-" + hex.EncodeToString(b[:]), nil
}

func sendMessage(ctx context.Context, rt iagents.Runtime, in iagents.SendInput) (*iagents.AgentTask, error) {
	p, err := iagents.BindParams[sendParams](rt)
	if err != nil {
		return nil, err
	}
	key, err := randomIdempotencyKey()
	if err != nil {
		return nil, err
	}
	params := map[string]string{}
	if p.ActiveTableID != "" {
		params["active_table_id"] = p.ActiveTableID
	}
	req := adapterSendRequest{
		ContextID:      in.ContextID,
		TaskID:         in.TaskID,
		Message:        adapterMessage{Role: "user", Parts: []adapterPart{{Type: "text", Text: in.Text}}},
		Params:         params,
		IdempotencyKey: key,
		Metadata:       map[string]string{"channel": "lark_cli"},
	}
	path := agentRoot(p.BaseToken) + "/messages"
	got, err := callPayload[adapterTask](ctx, rt, "POST", path, nil, req)
	if err != nil {
		return nil, err
	}
	return mapTask(got, true)
}

func getTask(ctx context.Context, rt iagents.Runtime, taskID string) (*iagents.AgentTask, error) {
	p, err := iagents.BindParams[getTaskParams](rt)
	if err != nil {
		return nil, err
	}
	path := agentRoot(p.BaseToken) + "/tasks/" + segment(taskID)
	query := map[string]string{}
	putQuery(query, "context_id", p.ContextID)
	got, err := callPayload[adapterTask](ctx, rt, "GET", path, query, nil)
	if err != nil {
		return nil, err
	}
	return mapTask(got, false)
}

func listTasks(ctx context.Context, rt iagents.Runtime, contextID string, page iagents.PageParams) ([]iagents.TaskSummary, iagents.PageInfo, error) {
	p, err := iagents.BindParams[listTasksParams](rt)
	if err != nil {
		return nil, iagents.PageInfo{}, err
	}
	query := map[string]string{}
	putQuery(query, "context_id", contextID)
	putQuery(query, "cursor", page.Token)
	if page.Size > 0 {
		query["limit"] = strconv.Itoa(page.Size)
	}
	putQuery(query, "state", p.State)
	got, err := callPayload[adapterTaskList](ctx, rt, "GET", agentRoot(p.BaseToken)+"/tasks", query, nil)
	if err != nil {
		return nil, iagents.PageInfo{}, err
	}
	out := make([]iagents.TaskSummary, 0, len(got.Tasks))
	for _, item := range got.Tasks {
		summary, err := mapTaskSummary(item)
		if err != nil {
			return nil, iagents.PageInfo{}, err
		}
		out = append(out, summary)
	}
	return out, iagents.PageInfo{HasMore: got.HasMore, NextToken: got.NextCursor}, nil
}

func cancelTask(ctx context.Context, rt iagents.Runtime, taskID string) error {
	p, err := iagents.BindParams[baseTokenParams](rt)
	if err != nil {
		return err
	}
	path := agentRoot(p.BaseToken) + "/tasks/" + segment(taskID) + "/cancel"
	result, err := callPayload[adapterResult](ctx, rt, "POST", path, nil, map[string]any{
		"metadata": map[string]string{"channel": "lark_cli"},
	})
	if err != nil {
		return err
	}
	return mapResult(result, "cancel task")
}

func listContexts(ctx context.Context, rt iagents.Runtime, page iagents.PageParams) ([]iagents.ContextSummary, iagents.PageInfo, error) {
	p, err := iagents.BindParams[listContextsParams](rt)
	if err != nil {
		return nil, iagents.PageInfo{}, err
	}
	query := map[string]string{}
	putQuery(query, "cursor", page.Token)
	if page.Size > 0 {
		query["limit"] = strconv.Itoa(page.Size)
	}
	putQuery(query, "status", p.Status)
	got, err := callPayload[adapterContextList](ctx, rt, "GET", agentRoot(p.BaseToken)+"/contexts", query, nil)
	if err != nil {
		return nil, iagents.PageInfo{}, err
	}
	out := make([]iagents.ContextSummary, 0, len(got.Contexts))
	for _, item := range got.Contexts {
		mapped, err := mapContextSummary(item)
		if err != nil {
			return nil, iagents.PageInfo{}, err
		}
		out = append(out, mapped)
	}
	return out, iagents.PageInfo{HasMore: got.HasMore, NextToken: got.NextCursor}, nil
}

func getContext(ctx context.Context, rt iagents.Runtime, contextID string) (*iagents.ContextDetail, error) {
	p, err := iagents.BindParams[baseTokenParams](rt)
	if err != nil {
		return nil, err
	}
	path := agentRoot(p.BaseToken) + "/contexts/" + segment(contextID)
	got, err := callPayload[adapterContext](ctx, rt, "GET", path, nil, nil)
	if err != nil {
		return nil, err
	}
	return mapContextDetail(got)
}

func deleteContext(ctx context.Context, rt iagents.Runtime, contextID string) error {
	p, err := iagents.BindParams[baseTokenParams](rt)
	if err != nil {
		return err
	}
	path := agentRoot(p.BaseToken) + "/contexts/" + segment(contextID)
	result, err := callPayload[adapterResult](ctx, rt, "DELETE", path, nil, nil)
	if err != nil {
		return err
	}
	return mapResult(result, "delete context")
}

func putQuery(query map[string]string, key, value string) {
	if value != "" {
		query[key] = value
	}
}

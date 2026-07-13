// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package base

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/larksuite/cli/errs"
	iagents "github.com/larksuite/cli/internal/agents"
)

func callPayload[T any](ctx context.Context, rt iagents.Runtime, method, path string, query map[string]string, body any) (T, error) {
	payload, err := iagents.Call[string](ctx, rt, method, path, query, body)
	if err != nil {
		var zero T
		return zero, err
	}
	if strings.TrimSpace(payload) == "" {
		var zero T
		return zero, errs.NewInternalError(errs.SubtypeInvalidResponse,
			"Base Adapter returned an empty payload for %s %s", method, path)
	}
	var out T
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		return out, errs.NewInternalError(errs.SubtypeInvalidResponse,
			"decode Base Adapter payload for %s %s: %v", method, path, err).WithCause(err)
	}
	return out, nil
}

func segment(v string) string { return url.PathEscape(v) }

func agentRoot(baseToken string) string {
	return "/bases/" + segment(baseToken) + "/ai/agents/" + segment(adapterAgentID)
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
	p, err := iagents.BindParams[baseTokenParams](rt)
	if err != nil {
		return nil, err
	}
	path := agentRoot(p.BaseToken) + "/tasks/" + segment(taskID)
	got, err := callPayload[adapterTask](ctx, rt, "GET", path, nil, nil)
	if err != nil {
		return nil, err
	}
	return mapTask(got, false)
}

func listTasks(ctx context.Context, rt iagents.Runtime, contextID string) ([]iagents.TaskSummary, error) {
	p, err := iagents.BindParams[listTasksParams](rt)
	if err != nil {
		return nil, err
	}
	query := map[string]string{}
	putQuery(query, "context_id", contextID)
	putQuery(query, "cursor", p.Cursor)
	if p.Limit > 0 {
		query["limit"] = strconv.FormatInt(p.Limit, 10)
	}
	putQuery(query, "state", p.State)
	got, err := callPayload[[]adapterTask](ctx, rt, "GET", agentRoot(p.BaseToken)+"/tasks", query, nil)
	if err != nil {
		return nil, err
	}
	out := make([]iagents.TaskSummary, 0, len(got))
	for _, item := range got {
		summary, err := mapTaskSummary(item)
		if err != nil {
			return nil, err
		}
		out = append(out, summary)
	}
	return out, nil
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

func listContexts(ctx context.Context, rt iagents.Runtime) ([]iagents.ContextSummary, error) {
	p, err := iagents.BindParams[listContextsParams](rt)
	if err != nil {
		return nil, err
	}
	query := map[string]string{}
	putQuery(query, "cursor", p.Cursor)
	if p.Limit > 0 {
		query["limit"] = strconv.FormatInt(p.Limit, 10)
	}
	putQuery(query, "status", p.Status)
	got, err := callPayload[[]adapterContext](ctx, rt, "GET", agentRoot(p.BaseToken)+"/contexts", query, nil)
	if err != nil {
		return nil, err
	}
	out := make([]iagents.ContextSummary, 0, len(got))
	for _, item := range got {
		mapped, err := mapContextSummary(item)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped)
	}
	return out, nil
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

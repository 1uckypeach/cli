// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package base

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/larksuite/cli/errs"
	iagents "github.com/larksuite/cli/internal/agents"
)

func taskID(in adapterTask) string {
	if in.TaskID != "" {
		return in.TaskID
	}
	return in.ID
}

func contextID(in adapterContext) string {
	if in.ContextID != "" {
		return in.ContextID
	}
	return in.ID
}

func mapState(raw string, allowEmpty bool) (iagents.TaskState, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "running", "pending", "working":
		return iagents.StateWorking, nil
	case "done", "finish", "finished", "turn_finished", "completed":
		return iagents.StateCompleted, nil
	case "failed", "cancel", "canceled", "cancelled":
		return iagents.StateFailed, nil
	case "":
		if allowEmpty {
			return iagents.StateSubmitted, nil
		}
	}
	return "", errs.NewInternalError(errs.SubtypeInvalidResponse,
		"Base Adapter returned unsupported task state %q", raw)
}

func mapTask(in adapterTask, allowEmptyState bool) (*iagents.AgentTask, error) {
	stateRaw := in.State
	if stateRaw == "" {
		stateRaw = in.Status
	}
	state, err := mapState(stateRaw, allowEmptyState)
	if err != nil {
		return nil, err
	}
	messages, err := mapMessages(in.Messages)
	if err != nil {
		return nil, err
	}
	artifacts, err := mapArtifacts(in.Artifacts)
	if err != nil {
		return nil, err
	}
	createdAt, err := mapTime(in.CreatedAt)
	if err != nil {
		return nil, err
	}
	updatedAt, err := mapTime(in.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &iagents.AgentTask{
		TaskID:     taskID(in),
		ContextID:  in.ContextID,
		State:      state,
		IsTerminal: state.IsTerminal(),
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		Messages:   messages,
		Artifacts:  artifacts,
	}, nil
}

func mapTaskSummary(in adapterTask) (iagents.TaskSummary, error) {
	stateRaw := in.State
	if stateRaw == "" {
		stateRaw = in.Status
	}
	state, err := mapState(stateRaw, false)
	if err != nil {
		return iagents.TaskSummary{}, err
	}
	updatedAt, err := mapTime(in.UpdatedAt)
	if err != nil {
		return iagents.TaskSummary{}, err
	}
	summary := in.Summary
	if summary == "" {
		messages, mapErr := mapMessages(in.Messages)
		if mapErr != nil {
			return iagents.TaskSummary{}, mapErr
		}
		summary = lastText(messages)
	}
	return iagents.TaskSummary{
		TaskID:     taskID(in),
		ContextID:  in.ContextID,
		State:      state,
		IsTerminal: state.IsTerminal(),
		UpdatedAt:  updatedAt,
		Summary:    summary,
	}, nil
}

func mapContextSummary(in adapterContext) (iagents.ContextSummary, error) {
	createdAt, err := mapTime(in.CreatedAt)
	if err != nil {
		return iagents.ContextSummary{}, err
	}
	updatedAt, err := mapTime(in.UpdatedAt)
	if err != nil {
		return iagents.ContextSummary{}, err
	}
	return iagents.ContextSummary{
		ContextID: contextID(in),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Title:     in.Title,
		TaskCount: 0,
	}, nil
}

func mapContextDetail(in adapterContext) (*iagents.ContextDetail, error) {
	summary, err := mapContextSummary(in)
	if err != nil {
		return nil, err
	}
	detail := &iagents.ContextDetail{
		ContextID: summary.ContextID,
		CreatedAt: summary.CreatedAt,
		UpdatedAt: summary.UpdatedAt,
		Title:     summary.Title,
		TaskCount: len(in.Tasks),
	}
	if len(in.Tasks) > 0 {
		active, err := mapTaskSummary(in.Tasks[0])
		if err != nil {
			return nil, err
		}
		detail.ActiveTask = &active
		detail.AwaitingInput = active.State == iagents.StateInputRequired || active.State == iagents.StateAuthRequired
	}
	return detail, nil
}

func mapMessages(in []adapterMessage) ([]iagents.Message, error) {
	out := make([]iagents.Message, 0, len(in))
	for _, message := range in {
		parts := make([]iagents.Part, 0, len(message.Parts)+1)
		if message.Text != "" {
			parts = append(parts, iagents.Part{Type: "text", Text: message.Text})
		}
		for _, part := range message.Parts {
			mapped, err := mapPart(part)
			if err != nil {
				return nil, err
			}
			parts = append(parts, mapped)
		}
		role := message.Role
		if role == "assistant" {
			role = "agent"
		}
		out = append(out, iagents.Message{Role: role, Parts: parts})
	}
	return out, nil
}

func mapPart(in adapterPart) (iagents.Part, error) {
	switch strings.ToLower(in.Type) {
	case "text":
		return iagents.Part{Type: "text", Text: in.Text}, nil
	case "file":
		return iagents.Part{Type: "file", Name: in.Name, URL: in.URL}, nil
	case "data":
		if len(in.Data) > 0 {
			var data any
			if err := json.Unmarshal(in.Data, &data); err != nil {
				return iagents.Part{}, invalidMessage(err)
			}
			return iagents.Part{Type: "data", Data: data}, nil
		}
		if in.Text == "" {
			return iagents.Part{Type: "data"}, nil
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(in.Text), &data); err != nil {
			return iagents.Part{}, invalidMessage(err)
		}
		op, _ := data["operation_type"].(string)
		content, contentIsString := data["content"].(string)
		switch strings.ToLower(op) {
		case "text", "answer", "message", "plain_text", "markdown":
			if contentIsString {
				return iagents.Part{Type: "text", Text: content}, nil
			}
		}
		return iagents.Part{Type: "data", Data: data}, nil
	default:
		return iagents.Part{Type: "data", Data: map[string]any{
			"type": in.Type, "text": in.Text, "name": in.Name, "url": in.URL,
		}}, nil
	}
}

func mapArtifacts(in []adapterArtifact) ([]iagents.Artifact, error) {
	out := make([]iagents.Artifact, 0, len(in))
	for _, item := range in {
		text := item.Text
		if strings.HasPrefix(strings.TrimSpace(text), "{") {
			part, err := mapPart(adapterPart{Type: "data", Text: text})
			if err != nil {
				return nil, err
			}
			if part.Type == "text" {
				text = part.Text
			}
		}
		out = append(out, iagents.Artifact{ID: item.ID, Kind: item.Kind, Name: item.Name, URL: item.URL, Text: text})
	}
	return out, nil
}

func mapTime(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var unix int64
	if err := json.Unmarshal(raw, &unix); err == nil {
		return time.Unix(unix, 0).UTC().Format(time.RFC3339), nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", errs.NewInternalError(errs.SubtypeInvalidResponse,
			"Base Adapter returned invalid timestamp %s", string(raw)).WithCause(err)
	}
	if value == "" {
		return "", nil
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(n, 0).UTC().Format(time.RFC3339), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", errs.NewInternalError(errs.SubtypeInvalidResponse,
			"Base Adapter returned invalid timestamp %q", value).WithCause(err)
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

func invalidMessage(cause error) error {
	return errs.NewInternalError(errs.SubtypeInvalidResponse,
		"Base Adapter returned an invalid embedded CliMessage: %v", cause).WithCause(cause)
}

func lastText(messages []iagents.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		for j := len(messages[i].Parts) - 1; j >= 0; j-- {
			if messages[i].Parts[j].Type == "text" {
				return messages[i].Parts[j].Text
			}
		}
	}
	return ""
}

func mapResult(result adapterResult, action string) error {
	if result.Result {
		return nil
	}
	message := result.Reason
	if message == "" {
		message = result.Error.Message
	}
	if message == "" {
		message = action + " failed"
	}
	switch strings.ToLower(result.Error.Category) {
	case "not_found":
		return errs.NewAPIError(errs.SubtypeNotFound, "%s: %s", action, message)
	case "permission_denied", "forbidden":
		return errs.NewPermissionError(errs.SubtypePermissionDenied, "%s: %s", action, message)
	case "task_terminal", "failed_precondition":
		return errs.NewValidationError(errs.SubtypeFailedPrecondition, "%s: %s", action, message)
	case "conflict", "idempotency_conflict":
		return errs.NewAPIError(errs.SubtypeConflict, "%s: %s", action, message)
	case "rate_limit":
		return errs.NewAPIError(errs.SubtypeRateLimit, "%s: %s", action, message).WithRetryable()
	case "internal_route", "server_error":
		return errs.NewAPIError(errs.SubtypeServerError, "%s: %s", action, message).WithRetryable()
	default:
		return errs.NewInternalError(errs.SubtypeInvalidResponse,
			"Base Adapter returned an unknown business error category %q for %s", result.Error.Category, action)
	}
}

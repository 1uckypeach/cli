// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package base

import "encoding/json"

type adapterSendRequest struct {
	ContextID      string            `json:"context_id,omitempty"`
	TaskID         string            `json:"task_id,omitempty"`
	Message        adapterMessage    `json:"message"`
	Params         map[string]string `json:"params,omitempty"`
	IdempotencyKey string            `json:"idempotency_key"`
	Metadata       map[string]string `json:"metadata"`
}

type adapterMessage struct {
	Role  string        `json:"role"`
	Parts []adapterPart `json:"parts,omitempty"`
	Text  string        `json:"text,omitempty"`
}

type adapterPart struct {
	Type string          `json:"type"`
	Text string          `json:"text,omitempty"`
	Name string          `json:"name,omitempty"`
	URL  string          `json:"url,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

type adapterArtifact struct {
	ID   string `json:"id"`
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
	Text string `json:"text,omitempty"`
}

type adapterTask struct {
	ID        string            `json:"id,omitempty"`
	TaskID    string            `json:"task_id,omitempty"`
	ContextID string            `json:"context_id,omitempty"`
	State     string            `json:"state,omitempty"`
	Status    string            `json:"status,omitempty"`
	CreatedAt json.RawMessage   `json:"created_at,omitempty"`
	UpdatedAt json.RawMessage   `json:"updated_at,omitempty"`
	Summary   string            `json:"summary,omitempty"`
	Messages  []adapterMessage  `json:"messages,omitempty"`
	Artifacts []adapterArtifact `json:"artifacts,omitempty"`
}

type adapterContext struct {
	ID        string          `json:"id,omitempty"`
	ContextID string          `json:"context_id,omitempty"`
	Title     string          `json:"title,omitempty"`
	Status    string          `json:"status,omitempty"`
	CreatedAt json.RawMessage `json:"created_at,omitempty"`
	UpdatedAt json.RawMessage `json:"updated_at,omitempty"`
	Tasks     []adapterTask   `json:"tasks,omitempty"`
}

type adapterBusinessError struct {
	Category string `json:"category,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
}

type adapterResult struct {
	Result bool                 `json:"result"`
	Reason string               `json:"reason,omitempty"`
	Error  adapterBusinessError `json:"error,omitempty"`
}

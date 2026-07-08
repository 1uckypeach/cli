// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import "context"

// SendInput is the input to send (Params has already passed Card validation).
type SendInput struct {
	Text      string
	Files     []string
	Params    map[string]string
	ContextID string
	TaskID    string
}

// Provider is a remote agent adapter: it translates the unified commands into a
// specific vendor's OAPI.
type Provider interface {
	// Card returns the agent's capability description (may be synthesized
	// statically, not necessarily via an API call).
	Card(ctx context.Context) (*AgentCard, error)
	// Send sends one message, starting a new task or continuing an existing one.
	Send(ctx context.Context, in SendInput) (*AgentTask, error)
	// GetTask queries a single task's state and artifacts.
	GetTask(ctx context.Context, taskID string) (*AgentTask, error)
	// ListTasks lists tasks, optionally filtered by contextID (empty string means no filter).
	ListTasks(ctx context.Context, contextID string) ([]TaskSummary, error)
	// CancelTask cancels (interrupts) a task; returns ErrUnsupported when unsupported.
	CancelTask(ctx context.Context, taskID string) error
	// ListContexts lists multi-turn contexts.
	ListContexts(ctx context.Context) ([]ContextSummary, error)
	// GetContext returns a single context's detail.
	GetContext(ctx context.Context, ctxID string) (*ContextDetail, error)
	// DeleteContext deletes a context (a destructive operation).
	DeleteContext(ctx context.Context, ctxID string) error
	// DownloadArtifact fetches artifact data: the URL type returns URL, the inline type returns Bytes.
	DownloadArtifact(ctx context.Context, taskID, artifactID string) (*ArtifactData, error)
}

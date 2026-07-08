// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

// AgentTask is the unified structure that task-family commands put into output.Envelope.Data.
type AgentTask struct {
	TaskID        string         `json:"task_id"`
	ContextID     string         `json:"context_id,omitempty"`
	State         TaskState      `json:"state"`
	IsTerminal    bool           `json:"is_terminal"`
	Messages      []Message      `json:"messages,omitempty"`
	Artifacts     []Artifact     `json:"artifacts,omitempty"`
	InputRequired *InputRequired `json:"input_required,omitempty"`
}

// Message is one turn of an agent or user message, composed of several Parts.
type Message struct {
	Role  string `json:"role"` // "agent" | "user"
	Parts []Part `json:"parts"`
}

// Part is one fragment of a message: text, file, or structured data.
type Part struct {
	Type string `json:"type"` // "text" | "file" | "data"
	Text string `json:"text,omitempty"`
	// File/Data pass-through: file uses URL/Name, data uses Data.
	Name string      `json:"name,omitempty"`
	URL  string      `json:"url,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// Artifact is one artifact produced by a task (file / inline text), downloadable
// via URL.
//
// Its fields align with A2A's Artifact/FilePart, but only what a provider can
// truly deliver is populated (e.g. example only provides ID + Kind — the
// coarse-grained kind at the GetTask stage — plus Name/Mime at the download
// stage). Mime/Description/Size are placeholders under A2A semantics; if a
// provider does not yet supply them they are omitted via omitempty and lit up
// only once the provider can fill them, rather than creating empty shell fields
// that cannot be filled.
type Artifact struct {
	ID          string `json:"id"`
	Kind        string `json:"kind,omitempty"` // coarse-grained kind (image/file/...), a type hint before download
	Name        string `json:"name,omitempty"` // file name (with extension), helps choose the -o save name
	Mime        string `json:"mime,omitempty"` // content type (image/png…), empty if the provider does not supply it
	Description string `json:"description,omitempty"`
	Size        int64  `json:"size,omitempty"` // byte count, 0 if the provider does not supply it
	URL         string `json:"url,omitempty"`
	Text        string `json:"text,omitempty"`
}

// InputRequired describes the input a task requests from the user while in the
// input_required state.
type InputRequired struct {
	Prompt  string   `json:"prompt"`
	Options []string `json:"options,omitempty"`
}

// TaskSummary is a single task summary in the task list output.
type TaskSummary struct {
	TaskID     string    `json:"task_id"`
	ContextID  string    `json:"context_id,omitempty"`
	State      TaskState `json:"state"`
	IsTerminal bool      `json:"is_terminal"`
}

// ContextSummary is a single context summary in the context list output.
type ContextSummary struct {
	ContextID string `json:"context_id"`
	CreatedAt string `json:"created_at,omitempty"`
	Title     string `json:"title,omitempty"`
}

// ContextDetail is the context detail in the context get output (including its task list).
type ContextDetail struct {
	ContextID string        `json:"context_id"`
	CreatedAt string        `json:"created_at,omitempty"`
	Title     string        `json:"title,omitempty"`
	Tasks     []TaskSummary `json:"tasks,omitempty"`
}

// ArtifactData is the return value of DownloadArtifact: the URL type gives URL,
// the inline type gives Bytes. Name is the server-suggested file name (echoed
// back only as a suggested_name reference for the command layer); it is
// untrusted input and must never participate in constructing the local save
// path — the save path is always determined by -o/SafeOutputPath.
type ArtifactData struct {
	Name  string
	Mime  string
	URL   string
	Bytes []byte
}

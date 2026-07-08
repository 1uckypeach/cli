// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

// AgentTask is the unified structure that task-family commands put into output.Envelope.Data.
type AgentTask struct {
	TaskID        string         `json:"task_id"`
	ContextID     string         `json:"context_id,omitempty"`
	State         TaskState      `json:"state"`
	IsTerminal    bool           `json:"is_terminal"`
	CreatedAt     string         `json:"created_at,omitempty"` // ISO 8601; when the task was created (empty if the provider does not supply it)
	UpdatedAt     string         `json:"updated_at,omitempty"` // ISO 8601; when the current status was recorded (aligns with A2A TaskStatus.timestamp)
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

// TaskSummary is a single task summary in the task list output (and in a
// context's active_task). It carries just enough to triage without a full
// task get: state + when it last changed + a one-line content digest.
type TaskSummary struct {
	TaskID     string    `json:"task_id"`
	ContextID  string    `json:"context_id,omitempty"`
	State      TaskState `json:"state"`
	IsTerminal bool      `json:"is_terminal"`
	UpdatedAt  string    `json:"updated_at,omitempty"` // ISO 8601; when the status was last recorded — the key for "most recent"
	Summary    string    `json:"summary,omitempty"`    // last agent message, ANSI-stripped + flattened + truncated; for input_required it is the pending prompt
}

// ContextSummary is a single context summary in the context list output. It is
// the conversation-layer rollup used to pick which conversation needs attention.
type ContextSummary struct {
	ContextID     string `json:"context_id"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"` // ISO 8601; last activity across the context's tasks
	Title         string `json:"title,omitempty"`
	TaskCount     int    `json:"task_count"`               // number of tasks in the context
	AwaitingInput bool   `json:"awaiting_input,omitempty"` // a task is paused in input_required/auth_required (needs the caller)
}

// ContextDetail is the context detail in the context get output. It is the
// conversation overview — metadata + a rollup + the single task the caller would
// most likely act on. The full task enumeration lives in `agent task list
// --context-id`, so ContextDetail deliberately does NOT embed the whole tasks[].
type ContextDetail struct {
	ContextID     string       `json:"context_id"`
	CreatedAt     string       `json:"created_at,omitempty"`
	UpdatedAt     string       `json:"updated_at,omitempty"`
	Title         string       `json:"title,omitempty"`
	TaskCount     int          `json:"task_count"`
	AwaitingInput bool         `json:"awaiting_input,omitempty"`
	ActiveTask    *TaskSummary `json:"active_task,omitempty"` // the task with the latest updated_at (nil for an empty context)
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

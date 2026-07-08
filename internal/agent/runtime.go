// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import "context"

// Runtime is the only thing a verb hook touches for I/O. It is the agent
// analogue of the event/shortcut runtime: the framework has already resolved and
// PINNED the calling identity (user|bot) inside it, so a hook never sees a raw
// *client.APIClient, never resolves a token, and cannot bypass scope preflight.
// The concrete implementation lives in cmd/agent (like event's consumeRuntime in
// cmd/event), which is why internal/agent no longer needs to depend on
// internal/client — the sole reason the old Deps struct existed.
type Runtime interface {
	// AgentID is the agent this call addresses (parsed from the ref by the
	// framework). A catalog hook may ignore it; an instance template reads it to
	// know which runtime agent it serves. Request data, not plumbing.
	AgentID() string

	// IsBot reports the resolved identity kind for the rare hook that must branch
	// on it. Identity resolution itself stays hidden.
	IsBot() bool

	// CallAPI issues one JSON OAPI request under the pinned identity and returns
	// the decoded "data" object (map[string]any) or a typed errs.* error — a hook
	// never does response-envelope unwrapping, identity threading, or error
	// classification. query values are strings (page_token, *_id_type, …).
	CallAPI(ctx context.Context, method, path string, query map[string]string, body any) (map[string]any, error)

	// CallMultipart is the file-upload seam: it reproduces the multipart form
	// upload a real provider would otherwise hand-write (larkcore.NewFormdata +
	// WithFileUpload), but centralized and identity-opaque. The framework
	// SafeInputPath-validates and opens each FilePart.Path, builds the multipart
	// body, pins the identity, and returns the decoded "data" object. This is what
	// makes the FileInput capability actually deliverable — without it a provider
	// declaring FileInput=true but with only a JSON client would silently drop
	// SendInput.Files.
	CallMultipart(ctx context.Context, method, path string, fields map[string]string, files []FilePart) (map[string]any, error)
}

// FilePart is one file to upload. Path comes straight from SendInput.Files and is
// SafeInputPath-validated by the runtime (the security check stays framework-
// owned, not re-implemented per provider).
type FilePart struct {
	Field string // multipart field name, e.g. "file"
	Path  string // local path (framework validates + opens)
}

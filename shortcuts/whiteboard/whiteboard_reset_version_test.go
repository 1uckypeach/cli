// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package whiteboard

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/httpmock"
)

// TestWhiteboardResetVersion_Validate verifies target-revision validation.
func TestWhiteboardResetVersion_Validate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		flags   map[string]string
		wantErr bool
		param   string
	}{
		{
			name:    "valid positive revision",
			flags:   map[string]string{"whiteboard-token": "test-token-123", "target-revision": "10221"},
			wantErr: false,
		},
		{
			name:    "non-numeric revision",
			flags:   map[string]string{"whiteboard-token": "test-token-123", "target-revision": "abc"},
			wantErr: true,
			param:   "--target-revision",
		},
		{
			name:    "non-positive revision",
			flags:   map[string]string{"whiteboard-token": "test-token-123", "target-revision": "0"},
			wantErr: true,
			param:   "--target-revision",
		},
		{
			name:    "empty revision",
			flags:   map[string]string{"whiteboard-token": "test-token-123", "target-revision": ""},
			wantErr: true,
			param:   "--target-revision",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := newTestRuntime(tt.flags, nil)
			err := wbResetVersionValidate(ctx, rt)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wbResetVersionValidate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.param != "" {
				var ve *errs.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("error is not *errs.ValidationError: %T", err)
				}
				if ve.Param != tt.param {
					t.Errorf("Param = %q, want %q", ve.Param, tt.param)
				}
			}
		})
	}
}

// TestWhiteboardResetVersion_DryRun verifies the dry-run targets the reset_version endpoint with the target revision.
func TestWhiteboardResetVersion_DryRun(t *testing.T) {
	ctx := context.Background()
	rt := newTestRuntime(map[string]string{
		"whiteboard-token": "test-token-123456789012",
		"target-revision":  "10221",
	}, nil)

	dry := wbResetVersionDryRun(ctx, rt)
	if dry == nil {
		t.Fatal("wbResetVersionDryRun() returned nil")
	}
	raw, err := json.Marshal(dry)
	if err != nil {
		t.Fatalf("marshal dry-run: %v", err)
	}
	var out struct {
		API []struct {
			Method string                 `json:"method"`
			URL    string                 `json:"url"`
			Body   map[string]interface{} `json:"body"`
		} `json:"api"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode dry-run: %v\nraw=%s", err, raw)
	}
	if len(out.API) != 1 {
		t.Fatalf("api calls = %d, want 1: %s", len(out.API), raw)
	}
	if got := out.API[0].Method; got != "POST" {
		t.Errorf("method = %q, want POST", got)
	}
	// Token is masked in dry-run output but the path shape must be stable.
	if got := out.API[0].URL; got != "/open-apis/board/v1/whiteboards/test...9012/reset_version" {
		t.Errorf("url = %q, want masked reset_version path", got)
	}
	if got := out.API[0].Body["target_revision"]; got != "10221" {
		t.Errorf("target_revision = %#v, want 10221", got)
	}
}

// TestWhiteboardResetVersion_Execute verifies the execute path posts target_revision to the reset_version endpoint.
func TestWhiteboardResetVersion_Execute(t *testing.T) {
	factory, stdout, reg := newUpdateExecuteFactory(t)

	stub := &httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/board/v1/whiteboards/test-token-reset/reset_version",
		Body: map[string]interface{}{
			"code": 0,
			"msg":  "success",
			"data": map[string]interface{}{},
		},
	}
	reg.Register(stub)

	args := []string{"+reset-version", "--whiteboard-token", "test-token-reset", "--target-revision", "10221"}
	if err := runUpdateShortcut(t, WhiteboardResetVersion, args, factory, stdout); err != nil {
		t.Fatalf("err=%v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(stub.CapturedBody, &body); err != nil {
		t.Fatalf("unmarshal captured body: %v\nraw=%s", err, string(stub.CapturedBody))
	}
	if got := body["target_revision"]; got != "10221" {
		t.Fatalf("target_revision = %#v, want 10221; body=%s", got, string(stub.CapturedBody))
	}
}

// TestWhiteboardResetVersion_ExecuteAPIError verifies API failures surface as typed errors carrying the Lark code.
func TestWhiteboardResetVersion_ExecuteAPIError(t *testing.T) {
	factory, stdout, reg := newUpdateExecuteFactory(t)

	reg.Register(&httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/board/v1/whiteboards/test-token-reset-error/reset_version",
		Body: map[string]interface{}{
			"code": 10001,
			"msg":  "revert failed",
		},
	})

	args := []string{"+reset-version", "--whiteboard-token", "test-token-reset-error", "--target-revision", "10221"}
	err := runUpdateShortcut(t, WhiteboardResetVersion, args, factory, stdout)
	if err == nil {
		t.Fatal("expected API error, got none")
	}
	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("error is not a typed errs.* envelope: %T (%v)", err, err)
	}
	if p.Code != 10001 {
		t.Errorf("Problem.Code = %d, want 10001", p.Code)
	}
}

// TestWhiteboardResetVersion_ShortcutRegistration verifies the shortcut metadata.
func TestWhiteboardResetVersion_ShortcutRegistration(t *testing.T) {
	t.Parallel()

	if WhiteboardResetVersion.Command != "+reset-version" {
		t.Errorf("Command = %q, want \"+reset-version\"", WhiteboardResetVersion.Command)
	}
	if WhiteboardResetVersion.Service != "whiteboard" {
		t.Errorf("Service = %q, want \"whiteboard\"", WhiteboardResetVersion.Service)
	}
	if WhiteboardResetVersion.Risk != "write" {
		t.Errorf("Risk = %q, want \"write\"", WhiteboardResetVersion.Risk)
	}

	seen := false
	for _, s := range Shortcuts() {
		if s.Command == "+reset-version" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("missing +reset-version in Shortcuts()")
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package common

import (
	"context"
	"testing"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/spf13/cobra"
)

// TestShortcutMount_FormatFlagAlwaysRegistered verifies that --format is
// injected for every shortcut regardless of the HasFormat field value.
func TestShortcutMount_FormatFlagAlwaysRegistered(t *testing.T) {
	f, _, _, _ := cmdutil.TestFactory(t, nil)
	parent := &cobra.Command{Use: "root"}
	shortcut := Shortcut{
		Service:     "im",
		Command:     "+message-send",
		Description: "send message",
		HasFormat:   false, // explicitly false — format must still be registered
		Execute:     func(context.Context, *RuntimeContext) error { return nil },
	}
	shortcut.Mount(parent, f)

	cmd, _, err := parent.Find([]string{"+message-send"})
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	flag := cmd.Flags().Lookup("format")
	if flag == nil {
		t.Fatal("--format flag not registered; expected it to be injected even when HasFormat is false")
	}
	if flag.DefValue != "json" {
		t.Errorf("--format default = %q, want %q", flag.DefValue, "json")
	}
}

func TestRunShortcutWritePrettyWithoutRendererExecutesAndFallsBackToJSON(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_CONTENT_SAFETY_MODE", "off")
	executeCalls := 0
	f, stdout, stderr, reg := cmdutil.TestFactory(t, &core.CliConfig{
		AppID: "test", AppSecret: "test", Brand: core.BrandFeishu,
	})
	writeStub := &httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/fixture/v1/items",
		Body: map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{"id": "created"},
		},
	}
	reg.Register(writeStub)
	shortcut := &Shortcut{
		Service:   "fixture",
		Command:   "+write",
		Risk:      "write",
		AuthTypes: []string{"bot"},
		Execute: func(_ context.Context, rctx *RuntimeContext) error {
			executeCalls++
			data, err := rctx.CallAPITyped("POST", "/open-apis/fixture/v1/items", nil, map[string]interface{}{"name": "created"})
			if err != nil {
				return err
			}
			rctx.OutFormat(data, nil, nil)
			return nil
		},
	}
	cmd := newTestShortcutCmd(shortcut, f)
	if err := cmd.Flags().Set("as", "bot"); err != nil {
		t.Fatalf("set --as: %v", err)
	}
	if err := cmd.Flags().Set("format", "pretty"); err != nil {
		t.Fatalf("set --format: %v", err)
	}

	if err := runShortcut(cmd, f, shortcut, true); err != nil {
		t.Fatalf("runShortcut() error = %v, want nil", err)
	}
	if executeCalls != 1 {
		t.Fatalf("Execute call count = %d, want 1", executeCalls)
	}
	if len(writeStub.CapturedBodies) != 1 {
		t.Fatalf("API call count = %d, want 1", len(writeStub.CapturedBodies))
	}
	const wantStdout = "{\n  \"ok\": true,\n  \"identity\": \"bot\",\n  \"data\": {\n    \"id\": \"created\"\n  }\n}\n"
	if stdout.String() != wantStdout {
		t.Fatalf("stdout = %q, want %q", stdout.String(), wantStdout)
	}
	const wantStderr = "warning: --format pretty is not supported by this command; showing JSON instead\n"
	if stderr.String() != wantStderr {
		t.Fatalf("stderr = %q, want %q", stderr.String(), wantStderr)
	}
}

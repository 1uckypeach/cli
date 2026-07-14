// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package common

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
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

func TestRuntimeContextOutKeepsJSONEnvelopeForPrettyFormat(t *testing.T) {
	cfg := &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu}
	f, stdout, _, _ := cmdutil.TestFactory(t, cfg)
	rctx := TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "+read"}, cfg, f, core.AsBot)
	rctx.Format = "pretty"

	rctx.Out(map[string]interface{}{
		"items": []interface{}{map[string]interface{}{"name": "Alice"}},
	}, nil)

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Items []map[string]interface{} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("Out should emit a JSON envelope: %v\n%s", err, stdout.String())
	}
	if !envelope.OK || len(envelope.Data.Items) != 1 || envelope.Data.Items[0]["name"] != "Alice" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}

func TestRuntimeContextOutRawKeepsJSONEnvelopeForPrettyFormat(t *testing.T) {
	cfg := &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu}
	f, stdout, _, _ := cmdutil.TestFactory(t, cfg)
	rctx := TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "+read"}, cfg, f, core.AsBot)
	rctx.Format = "pretty"

	rctx.OutRaw(map[string]interface{}{"body": "<p>hello</p>"}, nil)

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Body string `json:"body"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("OutRaw should emit a JSON envelope: %v\n%s", err, stdout.String())
	}
	if !envelope.OK || envelope.Data.Body != "<p>hello</p>" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}

func TestShortcutMount_UnsupportedFormatFailsBeforeExecution(t *testing.T) {
	cfg := &core.CliConfig{AppID: "test-app", AppSecret: "test-secret", Brand: core.BrandFeishu}
	f, _, _, _ := cmdutil.TestFactory(t, cfg)
	parent := &cobra.Command{Use: "root"}
	executed := false
	shortcut := Shortcut{
		Service:     "test",
		Command:     "+read",
		Description: "read data",
		AuthTypes:   []string{"bot"},
		Execute: func(context.Context, *RuntimeContext) error {
			executed = true
			return nil
		},
	}
	shortcut.Mount(parent, f)

	cmd, _, err := parent.Find([]string{"+read"})
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if cmd == nil {
		t.Fatal("expected mounted shortcut command")
	}
	parent.SetArgs([]string{"+read", "--format", "xml"})
	err = parent.Execute()
	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *errs.ValidationError, got %T: %v", err, err)
	}
	if validationErr.Param != "--format" {
		t.Fatalf("param = %q, want --format", validationErr.Param)
	}
	if executed {
		t.Fatal("shortcut must not execute with an unsupported format")
	}
}

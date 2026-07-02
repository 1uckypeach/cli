// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/output"
	"github.com/spf13/cobra"
)

func TestUnknownFlagName(t *testing.T) {
	cases := []struct {
		in   string
		name string
		ok   bool
	}{
		{"unknown flag: --query", "query", true},
		{"unknown flag: --with-styles", "with-styles", true},
		{"unknown shorthand flag: 'z' in -z", "", false},
		{"flag needs an argument: --find", "", false},
		{`invalid argument "x" for "--count"`, "", false},
	}
	for _, c := range cases {
		name, ok := unknownFlagName(errors.New(c.in))
		if name != c.name || ok != c.ok {
			t.Errorf("unknownFlagName(%q) = (%q,%v), want (%q,%v)", c.in, name, ok, c.name, c.ok)
		}
	}
}

func TestFlagDidYouMean_UnknownFlagSuggestsAndListsValid(t *testing.T) {
	c := &cobra.Command{Use: "demo"}
	c.Flags().String("range", "", "")
	c.Flags().String("find", "", "")
	c.Flags().Bool("dry-run", false, "")

	err := flagDidYouMean(c, errors.New("unknown flag: --rang")) // typo of --range
	var verr *errs.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *errs.ValidationError, got %T", err)
	}
	if verr.Subtype != errs.SubtypeInvalidArgument {
		t.Errorf("subtype = %q, want invalid_argument", verr.Subtype)
	}
	if code := output.ExitCodeOf(err); code != output.ExitValidation {
		t.Errorf("exit code = %d, want %d (ExitValidation)", code, output.ExitValidation)
	}
	// The offending flag is carried structurally on Params (replaces the
	// legacy detail map) and named in the message.
	if len(verr.Params) != 1 || verr.Params[0].Name != "--rang" {
		t.Errorf("Params = %v, want one entry named --rang", verr.Params)
	}
	if len(verr.Params) == 1 && verr.Params[0].Reason == "" {
		t.Error("Params[0].Reason must explain the rejection")
	}
	if !strings.Contains(verr.Message, "--rang") {
		t.Errorf("message should name the offending flag, got %q", verr.Message)
	}
	// The ranked candidate rides on the param as a machine-readable suggestion
	// so an agent can retry without parsing prose.
	if len(verr.Params) == 1 {
		found := false
		for _, s := range verr.Params[0].Suggestions {
			if s == "--range" {
				found = true
			}
		}
		if !found {
			t.Errorf("Params[0].Suggestions should include --range, got %v", verr.Params[0].Suggestions)
		}
	}
	// The same candidate is also carried in the human-facing hint.
	if !strings.Contains(verr.Hint, "--range") {
		t.Errorf("hint should suggest --range, got %q", verr.Hint)
	}
}

func TestFlagDidYouMean_WikiNodeGetSuggestsNodeToken(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_REMOTE_META", "off")
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())

	root := Build(context.Background(), cmdutil.InvocationContext{}, WithoutPlugins())
	root.SetArgs([]string{
		"wiki", "+node-get",
		"--node", "https://feishu.cn/wiki/wikcnABC",
		"--as", "user",
	})

	err := root.Execute()
	var verr *errs.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *errs.ValidationError, got %T (%v)", err, err)
	}
	if len(verr.Params) != 1 || verr.Params[0].Name != "--node" {
		t.Fatalf("Params = %v, want one entry named --node", verr.Params)
	}
	found := false
	for _, s := range verr.Params[0].Suggestions {
		if s == "--node-token" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Params[0].Suggestions = %v, want --node-token", verr.Params[0].Suggestions)
	}
	if !strings.Contains(verr.Hint, "--node-token") {
		t.Fatalf("hint = %q, want --node-token", verr.Hint)
	}
}

func TestFlagDidYouMean_OtherErrorStaysGeneric(t *testing.T) {
	c := &cobra.Command{Use: "demo"}
	err := flagDidYouMean(c, errors.New("flag needs an argument: --find"))
	var verr *errs.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *errs.ValidationError, got %T", err)
	}
	// Non-unknown-flag errors stay generic: invalid_argument subtype, no
	// structured param, generic --help hint (no "did you mean" suggestion).
	if verr.Subtype != errs.SubtypeInvalidArgument {
		t.Errorf("subtype = %q, want invalid_argument (non-unknown-flag errors stay generic)", verr.Subtype)
	}
	if code := output.ExitCodeOf(err); code != output.ExitValidation {
		t.Errorf("exit code = %d, want %d (ExitValidation)", code, output.ExitValidation)
	}
	if verr.Param != "" || len(verr.Params) != 0 {
		t.Errorf("Param=%q Params=%v, want both empty for generic flag error", verr.Param, verr.Params)
	}
	if strings.Contains(verr.Hint, "did you mean") {
		t.Errorf("generic flag error must not produce a did-you-mean hint, got %q", verr.Hint)
	}
}

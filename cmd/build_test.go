// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/output"
	"github.com/spf13/cobra"
)

func TestBuildWithoutPluginsStillBuildsBuiltinCommands(t *testing.T) {
	root := Build(context.Background(), cmdutil.InvocationContext{}, WithoutPlugins())

	if root == nil {
		t.Fatal("Build returned nil root")
	}
	if findCommand(root, "api") == nil {
		t.Fatal("builtin api command missing")
	}
	if findCommand(root, "docs +fetch") == nil {
		t.Fatal("builtin docs +fetch shortcut missing")
	}
}

func buildValidationTestRoot(t *testing.T) *cobra.Command {
	t.Helper()
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	return Build(context.Background(), cmdutil.InvocationContext{},
		WithIO(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}),
		WithoutPlugins(),
		WithoutServiceCommands(),
		WithoutStrictMode(),
	)
}

func TestBuiltRoot_TopLevelTypoReturnsStructuredSuggestion(t *testing.T) {
	root := buildValidationTestRoot(t)
	root.SetArgs([]string{"imm"})

	err := root.Execute()
	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *errs.ValidationError, got %T: %v", err, err)
	}
	if len(validationErr.Params) != 1 || validationErr.Params[0].Name != "imm" {
		t.Fatalf("params = %v, want one entry named imm", validationErr.Params)
	}
	found := false
	for _, candidate := range validationErr.Params[0].Suggestions {
		if candidate == "im" {
			found = true
		}
	}
	if !found {
		t.Fatalf("suggestions = %v, want im", validationErr.Params[0].Suggestions)
	}
}

func TestBuiltRoot_SheetsOneRequiredGroupReturnsValidationExit(t *testing.T) {
	root := buildValidationTestRoot(t)
	root.SetArgs([]string{
		"sheets", "+csv-put",
		"--url", "https://example.com/sheets/token",
		"--sheet-name", "Sheet1",
		"--csv", "a,b",
	})

	err := root.Execute()
	problem, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("expected typed problem, got %T: %v", err, err)
	}
	if problem.Category != errs.CategoryValidation || problem.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("problem = %s/%s, want validation/invalid_argument", problem.Category, problem.Subtype)
	}
	if got := output.ExitCodeOf(err); got != output.ExitValidation {
		t.Fatalf("exit code = %d, want %d", got, output.ExitValidation)
	}
}

func TestBuiltRoot_MailUnknownFlagUsesSharedSuggestions(t *testing.T) {
	root := buildValidationTestRoot(t)
	root.SetArgs([]string{"mail", "+send", "--tos", "alice@example.com"})

	err := root.Execute()
	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *errs.ValidationError, got %T: %v", err, err)
	}
	if len(validationErr.Params) != 1 || validationErr.Params[0].Name != "--tos" {
		t.Fatalf("params = %v, want one entry named --tos", validationErr.Params)
	}
	found := false
	for _, candidate := range validationErr.Params[0].Suggestions {
		if candidate == "--to" {
			found = true
		}
	}
	if !found {
		t.Fatalf("suggestions = %v, want --to", validationErr.Params[0].Suggestions)
	}
}

func findCommand(root *cobra.Command, path string) *cobra.Command {
	parts := strings.Fields(path)
	cmd := root
	for _, part := range parts {
		var next *cobra.Command
		for _, child := range cmd.Commands() {
			if child.Name() == part {
				next = child
				break
			}
		}
		if next == nil {
			return nil
		}
		cmd = next
	}
	return cmd
}

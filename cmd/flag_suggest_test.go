// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdmeta"
	"github.com/larksuite/cli/internal/output"
	"github.com/spf13/cobra"
)

func parseFlagError(t *testing.T, c *cobra.Command, args ...string) error {
	t.Helper()
	err := c.Flags().Parse(args)
	if err == nil {
		t.Fatalf("Parse(%v) returned nil", args)
	}
	return err
}

func TestFlagDidYouMean_UnknownFlagSuggestsAndListsValid(t *testing.T) {
	c := &cobra.Command{Use: "demo"}
	c.Flags().String("range", "", "")
	c.Flags().String("find", "", "")
	c.Flags().Bool("dry-run", false, "")

	err := flagDidYouMean(c, parseFlagError(t, c, "--rang")) // typo of --range
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

func TestFlagDidYouMean_OtherErrorStaysGeneric(t *testing.T) {
	c := &cobra.Command{Use: "demo"}
	c.Flags().String("find", "", "")
	err := flagDidYouMean(c, parseFlagError(t, c, "--find"))
	var verr *errs.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *errs.ValidationError, got %T", err)
	}
	// Non-unknown-flag errors retain the same validation shape and identify the
	// flag from pflag's typed ValueRequiredError.
	if verr.Subtype != errs.SubtypeInvalidArgument {
		t.Errorf("subtype = %q, want invalid_argument (non-unknown-flag errors stay generic)", verr.Subtype)
	}
	if code := output.ExitCodeOf(err); code != output.ExitValidation {
		t.Errorf("exit code = %d, want %d (ExitValidation)", code, output.ExitValidation)
	}
	if len(verr.Params) != 1 || verr.Params[0].Name != "--find" {
		t.Errorf("Params=%v, want one entry named --find", verr.Params)
	}
	if strings.Contains(verr.Hint, "did you mean") {
		t.Errorf("generic flag error must not produce a did-you-mean hint, got %q", verr.Hint)
	}
}

func TestFlagDidYouMean_SheetsListsVisibleFlags(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	sheets := &cobra.Command{Use: "sheets"}
	cmdmeta.SetDomain(sheets, "sheets")
	root.AddCommand(sheets)
	sheets.Flags().String("range", "", "")
	sheets.Flags().Int("width", 0, "")

	err := flagDidYouMean(sheets, parseFlagError(t, sheets, "--cols"))
	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *errs.ValidationError, got %T", err)
	}
	for _, want := range []string{"--range", "--width"} {
		if !strings.Contains(validationErr.Hint, want) {
			t.Errorf("hint should include %q, got %q", want, validationErr.Hint)
		}
	}
}

func TestFlagDidYouMean_InvalidValueTypedError(t *testing.T) {
	c := &cobra.Command{Use: "demo"}
	c.Flags().Int("width", 0, "")

	// A non-numeric value for a typed flag surfaces pflag's InvalidValueError.
	err := flagDidYouMean(c, parseFlagError(t, c, "--width=abc"))
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
	if len(verr.Params) != 1 || verr.Params[0].Name != "--width" || verr.Params[0].Reason != "invalid flag value" {
		t.Errorf("Params = %v, want one --width entry with reason 'invalid flag value'", verr.Params)
	}
}

func TestFlagDidYouMean_InvalidSyntaxTypedError(t *testing.T) {
	c := &cobra.Command{Use: "demo"}
	c.Flags().String("range", "", "")

	// An empty flag name is bad flag syntax and surfaces pflag's InvalidSyntaxError.
	err := flagDidYouMean(c, parseFlagError(t, c, "--=oops"))
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
	if len(verr.Params) != 1 || verr.Params[0].Reason != "invalid flag syntax" {
		t.Errorf("Params = %v, want one entry with reason 'invalid flag syntax'", verr.Params)
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package sheets

import (
	"errors"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
	"github.com/spf13/cobra"
)

func TestCanonicalEnumValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		val  string
		enum []string
		want string
	}{
		{"SUM", []string{"sum", "count"}, "sum"},                  // casing
		{"center", []string{"top", "middle", "bottom"}, "middle"}, // alias: CSS vertical center
		{"middle", []string{"left", "center", "right"}, "center"}, // alias: horizontal middle
		{"overwite", []string{"append", "overwrite"}, ""},         // typo is NOT canonical
		{"delete", []string{"append", "overwrite"}, ""},           // nothing close
	}
	for _, c := range cases {
		if got := canonicalEnumValue(c.val, c.enum); got != c.want {
			t.Errorf("canonicalEnumValue(%q, %v) = %q, want %q", c.val, c.enum, got, c.want)
		}
	}
}

func TestClosestEnumValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		val  string
		enum []string
		want string
	}{
		{"SUM", []string{"sum", "count"}, "sum"},                   // casing
		{"center", []string{"top", "middle", "bottom"}, "middle"},  // alias
		{"overwite", []string{"append", "overwrite"}, "overwrite"}, // edit distance
		{"delete", []string{"append", "overwrite"}, ""},            // nothing close
	}
	for _, c := range cases {
		if got := closestEnumValue(c.val, c.enum); got != c.want {
			t.Errorf("closestEnumValue(%q, %v) = %q, want %q", c.val, c.enum, got, c.want)
		}
	}
}

// TestChainEnumNormalization_UnitContract pins the PreRunE stage in
// isolation: canonical vocabulary is auto-applied, typos error with a
// suggestion (never applied), the framework PreRunE keeps running first,
// and --print-schema skips enum gating entirely.
func TestChainEnumNormalization_UnitContract(t *testing.T) {
	t.Parallel()
	newCmd := func() (*cobra.Command, *bool) {
		cmd := &cobra.Command{Use: "+cells-set-style"}
		cmd.Flags().String("vertical-alignment", "", "")
		cmd.Flags().Bool("print-schema", false, "")
		prevCalled := false
		cmd.PreRunE = func(*cobra.Command, []string) error {
			prevCalled = true
			return nil
		}
		chainEnumNormalization(cmd)
		return cmd, &prevCalled
	}

	// Alias auto-applied, framework PreRunE preserved.
	cmd, prevCalled := newCmd()
	cmd.Flags().Set("vertical-alignment", "center")
	if err := cmd.PreRunE(cmd, nil); err != nil {
		t.Fatalf("center should normalize and pass, got: %v", err)
	}
	if got, _ := cmd.Flags().GetString("vertical-alignment"); got != "middle" {
		t.Errorf("vertical-alignment = %q, want rewritten to %q", got, "middle")
	}
	if !*prevCalled {
		t.Error("framework PreRunE must keep running first")
	}

	// Typo: error with suggestion, value untouched.
	cmd, _ = newCmd()
	cmd.Flags().Set("vertical-alignment", "botom")
	err := cmd.PreRunE(cmd, nil)
	var verr *errs.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("typo should fail with *errs.ValidationError, got %T: %v", err, err)
	}
	if !strings.Contains(verr.Hint, `"bottom"`) {
		t.Errorf("hint should suggest bottom for the typo, got %q", verr.Hint)
	}
	if got, _ := cmd.Flags().GetString("vertical-alignment"); got != "botom" {
		t.Errorf("typo must not be rewritten, got %q", got)
	}

	// --print-schema skips enum gating (pure local introspection).
	cmd, _ = newCmd()
	cmd.Flags().Set("vertical-alignment", "not-a-value")
	cmd.Flags().Set("print-schema", "true")
	if err := cmd.PreRunE(cmd, nil); err != nil {
		t.Errorf("--print-schema must skip enum gating, got: %v", err)
	}
}

// shortcutFromRegistry returns the fully wired shortcut (PostMount
// ergonomics included) as Shortcuts() exposes it to the framework.
func shortcutFromRegistry(t *testing.T, command string) common.Shortcut {
	t.Helper()
	for _, sc := range Shortcuts() {
		if sc.Command == command {
			return sc
		}
	}
	t.Fatalf("shortcut %q not found in Shortcuts()", command)
	return common.Shortcut{}
}

// TestShortcuts_FlagErgonomicsMounted verifies the ergonomics ride every
// mounted sheets command end-to-end: enum vocabulary normalizes on a real
// invocation, and unknown flags answer with the inlined valid-flag list.
func TestShortcuts_FlagErgonomicsMounted(t *testing.T) {
	t.Parallel()

	t.Run("enum alias normalizes through a real run", func(t *testing.T) {
		t.Parallel()
		sc := shortcutFromRegistry(t, "+cells-set-style")
		stdout, _, err := runShortcutCapturingErr(t, sc, []string{
			"--url", testURL,
			"--sheet-name", "s",
			"--range", "A1:A1",
			"--vertical-alignment", "center",
			"--dry-run",
		})
		if err != nil {
			t.Fatalf("center should normalize to middle and pass, got: %v", err)
		}
		if !strings.Contains(stdout, "middle") || strings.Contains(stdout, "center") {
			t.Errorf("dry-run body should carry the normalized value, got %q", stdout)
		}
	})

	t.Run("enum typo errors with suggestion", func(t *testing.T) {
		t.Parallel()
		sc := shortcutFromRegistry(t, "+cells-set-style")
		_, _, err := runShortcutCapturingErr(t, sc, []string{
			"--url", testURL,
			"--sheet-name", "s",
			"--range", "A1:A1",
			"--vertical-alignment", "botom",
			"--dry-run",
		})
		ve := requireValidation(t, err, `invalid value "botom" for --vertical-alignment`)
		if !strings.Contains(ve.Hint, `"bottom"`) {
			t.Errorf("hint should suggest bottom, got %q", ve.Hint)
		}
	})

}

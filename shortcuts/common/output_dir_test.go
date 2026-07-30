// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/larksuite/cli/internal/cmdutil"
)

// TestEnsureOutputDirForwards pins that the wrapper is wired: the behaviour it
// forwards to — relative-path validation, containment, owner-only permissions —
// is covered where it lives, in internal/outputdir. A wrapper this thin can only
// fail one way, by not being called.
func TestEnsureOutputDirForwards(t *testing.T) {
	dir := t.TempDir()
	cmdutil.TestChdir(t, dir)

	if err := EnsureOutputDir(filepath.Join("out", "nested")); err != nil {
		t.Fatalf("EnsureOutputDir(relative) error = %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "out", "nested")); err != nil || !info.IsDir() {
		t.Fatalf("EnsureOutputDir did not create the directory: stat error = %v", err)
	}

	if err := EnsureOutputDir(filepath.Join("..", "escaped")); err == nil {
		t.Fatal("EnsureOutputDir(../escaped) = nil, wanted the validation error to propagate")
	}
}

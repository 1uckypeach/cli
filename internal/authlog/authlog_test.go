// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package authlog

import (
	"path/filepath"
	"testing"
)

// TestAuthLogDir_UsesValidatedLogDirEnv verifies that a valid absolute
// LARKSUITE_CLI_LOG_DIR is normalized and used as the auth log directory.
func TestAuthLogDir_UsesValidatedLogDirEnv(t *testing.T) {
	base := t.TempDir()
	base, _ = filepath.EvalSymlinks(base)
	t.Setenv("LARKSUITE_CLI_LOG_DIR", filepath.Join(base, "logs", "..", "auth"))
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", "")

	logger := New(Options{RuntimeDir: func() string { return t.TempDir() }})
	got := logger.logDir()
	want := filepath.Join(base, "auth")
	if got != want {
		t.Fatalf("authLogDir() = %q, want %q", got, want)
	}
}

// TestAuthLogDir_InvalidLogDirFallsBackToConfigDir verifies that an invalid
// LARKSUITE_CLI_LOG_DIR falls back to LARKSUITE_CLI_CONFIG_DIR/logs.
func TestAuthLogDir_InvalidLogDirFallsBackToConfigDir(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_LOG_DIR", "relative-logs")
	configDir := t.TempDir()
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", configDir)

	logger := New(Options{RuntimeDir: func() string { return configDir }})
	got := logger.logDir()
	want := filepath.Join(configDir, "logs")
	if got != want {
		t.Fatalf("authLogDir() = %q, want %q", got, want)
	}
}

// TestShared_ReturnsOneLoggerPerProcess pins the property the callers depend on:
// every call site must observe the same logger, otherwise each one opens its own
// file handle and re-runs the log prune.
func TestShared_ReturnsOneLoggerPerProcess(t *testing.T) {
	restoreShared(t)

	first := Shared()
	if first == nil {
		t.Fatal("Shared() returned nil")
	}
	if second := Shared(); second != first {
		t.Fatalf("Shared() returned a different logger on the second call: %p then %p", first, second)
	}
}

// TestSetShared_InstalledLoggerWins covers the startup wiring: the command
// factory installs a workspace-aware logger and later callers must get it
// instead of the pre-workspace fallback.
func TestSetShared_InstalledLoggerWins(t *testing.T) {
	restoreShared(t)

	installed := New(Options{RuntimeDir: func() string { return "workspace-dir" }})
	SetShared(installed)
	if got := Shared(); got != installed {
		t.Fatalf("Shared() = %p, want the installed logger %p", got, installed)
	}

	// A nil install must not wipe the configured logger.
	SetShared(nil)
	if got := Shared(); got != installed {
		t.Fatalf("SetShared(nil) replaced the logger: got %p, want %p", got, installed)
	}
}

// restoreShared isolates each test from the process-wide logger.
func restoreShared(t *testing.T) {
	t.Helper()

	sharedMu.Lock()
	previous := sharedLog
	sharedLog = nil
	sharedMu.Unlock()

	t.Cleanup(func() {
		sharedMu.Lock()
		sharedLog = previous
		sharedMu.Unlock()
	})
}

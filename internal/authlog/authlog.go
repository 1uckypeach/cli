// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package authlog records authentication response and error diagnostics.
package authlog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/internal/vfs"
)

// Options configures one authentication logger instance.
type Options struct {
	RuntimeDir func() string
	Logger     *log.Logger
	Now        func() time.Time
	Args       func() []string
}

// Logger records authentication diagnostics in a workspace-specific log.
type Logger struct {
	runtimeDir func() string
	logger     *log.Logger
	loggerOnce sync.Once
	now        func() time.Time
	args       func() []string
}

// New creates an authentication logger with explicit dependencies.
func New(options Options) *Logger {
	if options.RuntimeDir == nil {
		options.RuntimeDir = defaultRuntimeDir
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Args == nil {
		options.Args = func() []string { return os.Args }
	}
	return &Logger{
		runtimeDir: options.RuntimeDir,
		logger:     options.Logger,
		now:        options.Now,
		args:       options.Args,
	}
}

func defaultRuntimeDir() string {
	if dir := os.Getenv("LARKSUITE_CLI_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := vfs.UserHomeDir()
	if err != nil || home == "" {
		// Silent fallback to a relative ".lark-cli": this package has no
		// IOStreams in scope, so we cannot surface a warning here without
		// violating the IOStreams injection boundary (enforced by lint).
		// Users who hit this path should set LARKSUITE_CLI_CONFIG_DIR
		// explicitly; the relative path will otherwise surface as an
		// explicit I/O error at first use.
		home = ""
	}
	return filepath.Join(home, ".lark-cli")
}

func (l *Logger) logDir() string {
	// LARKSUITE_CLI_LOG_DIR is the highest-priority override.
	// When set, it bypasses workspace subtree routing entirely.
	if dir := os.Getenv("LARKSUITE_CLI_LOG_DIR"); dir != "" {
		safeDir, err := validate.SafeEnvDirPath(dir, "LARKSUITE_CLI_LOG_DIR")
		if err == nil {
			return safeDir
		}
	}

	return filepath.Join(l.runtimeDir(), "logs")
}

func (l *Logger) init() {
	l.loggerOnce.Do(func() {
		if l.logger != nil {
			return
		}

		dir := l.logDir()
		now := l.now()
		if err := vfs.MkdirAll(dir, 0700); err != nil {
			return
		}

		logName := fmt.Sprintf("auth-%s.log", now.Format("2006-01-02"))
		logPath := filepath.Join(dir, logName)
		if f, err := vfs.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); err == nil {
			l.logger = log.New(f, "", 0)
			cleanupOldLogs(dir, now)
		}
	})
}

func FormatAuthCmdline(args []string) string {
	if len(args) == 0 {
		return ""
	}

	if len(args) <= 3 {
		return strings.Join(args, " ")
	}

	return strings.Join(args[:3], " ") + " ..."
}

// LogResponse records one authentication HTTP response.
func (l *Logger) LogResponse(path string, status int, logID string) {
	if l == nil {
		return
	}
	l.init()
	if l.logger == nil {
		return
	}

	l.logger.Printf(
		"[lark-cli] auth-response: time=%s path=%s status=%d x-tt-logid=%s cmdline=%s",
		l.now().Format(time.RFC3339Nano),
		path,
		status,
		logID,
		FormatAuthCmdline(l.args()),
	)
}

// LogError records one authentication-related local error.
func (l *Logger) LogError(component, op string, err error) {
	if l == nil || err == nil {
		return
	}

	l.init()
	if l.logger == nil {
		return
	}

	l.logger.Printf(
		"[lark-cli] auth-error: time=%s component=%s op=%s error=%q cmdline=%s",
		l.now().Format(time.RFC3339Nano),
		component,
		op,
		err.Error(),
		FormatAuthCmdline(l.args()),
	)
}

func cleanupOldLogs(dir string, now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "[lark-cli] [WARN] background log cleanup panicked: %v\n", r)
		}
	}()

	entries, err := vfs.ReadDir(dir)
	if err != nil {
		return
	}

	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := now.AddDate(0, 0, -7)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "auth-") || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		dateStr := strings.TrimPrefix(entry.Name(), "auth-")
		dateStr = strings.TrimSuffix(dateStr, ".log")

		logDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		logDate = time.Date(logDate.Year(), logDate.Month(), logDate.Day(), 0, 0, 0, 0, now.Location())
		if logDate.Before(cutoff) {
			_ = vfs.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

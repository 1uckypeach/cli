// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/larksuite/cli/shortcuts/common"
)

// initTimer collects ordered, per-step wall-clock timings for a single +init
// run, so a slow init can be broken down into which git/npx/subprocess/API step
// dominated. Spans are kept in call order (duplicates allowed — e.g. the
// two-commit init path records "git commit" twice). Steps skipped by an
// appTypePolicy are recorded explicitly with skipped=true so different
// app_types stay comparable side by side. The zero receiver (nil *initTimer) is
// a no-op, so callers never need a nil check.
type initTimer struct {
	mu    sync.Mutex
	now   func() time.Time
	start time.Time
	spans []initSpan
}

type initSpan struct {
	Name    string
	MS      int64
	Skipped bool
}

func newInitTimer() *initTimer {
	now := time.Now
	return &initTimer{now: now, start: now()}
}

// span starts a timed span and returns a stop func that records the elapsed
// milliseconds under name. Idiomatic use: `defer t.span("git_clone")()` or
// `stop := t.span(...); ...; stop()`.
func (t *initTimer) span(name string) func() {
	if t == nil {
		return func() {}
	}
	start := t.now()
	return func() { t.record(name, t.now().Sub(start).Milliseconds(), false) }
}

// skip records a step that an appTypePolicy skipped (0 ms, skipped=true).
func (t *initTimer) skip(name string) { t.record(name, 0, true) }

func (t *initTimer) record(name string, ms int64, skipped bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.spans = append(t.spans, initSpan{Name: name, MS: ms, Skipped: skipped})
	t.mu.Unlock()
}

// finish records the end-to-end "total" span (elapsed since the timer was
// created). Call once, right before rendering output.
func (t *initTimer) finish() {
	if t == nil {
		return
	}
	t.record("total", t.now().Sub(t.start).Milliseconds(), false)
}

// payload renders the collected spans as an ordered slice for the JSON
// envelope: [{"step":..,"ms":..,"skipped":true?}, ...].
func (t *initTimer) payload() []map[string]interface{} {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]map[string]interface{}, 0, len(t.spans))
	for _, s := range t.spans {
		m := map[string]interface{}{"step": s.Name, "ms": s.MS}
		if s.Skipped {
			m["skipped"] = true
		}
		out = append(out, m)
	}
	return out
}

// logSummary writes the per-step breakdown to stderr (never stdout, which is
// reserved for the JSON envelope). Skipped steps are shown so the reader sees
// what the app_type's policy elided.
func (t *initTimer) logSummary(rctx *common.RuntimeContext) {
	if t == nil {
		return
	}
	t.mu.Lock()
	spans := append([]initSpan(nil), t.spans...)
	t.mu.Unlock()
	if len(spans) == 0 {
		return
	}
	initLogf(rctx, "step timings (ms):")
	for _, s := range spans {
		if s.Skipped {
			initLogf(rctx, "  %-28s skipped", s.Name)
			continue
		}
		initLogf(rctx, "  %-28s %d", s.Name, s.MS)
	}
}

// attachTimings finalizes the timer, embeds the ordered timings in the JSON
// output map, and logs the human-readable summary to stderr. Call it on every
// +init return path just before rctx.OutFormat.
func attachTimings(rctx *common.RuntimeContext, out map[string]interface{}, timer *initTimer) {
	timer.finish()
	out["timings"] = timer.payload()
	timer.logSummary(rctx)
}

// timingRunner decorates a commandRunner, recording each Run's wall-clock
// duration into timer under a human-readable label derived from the command.
// It changes no behavior — stdout/stderr/err pass through untouched — so it can
// wrap the production runner or a test fake transparently.
type timingRunner struct {
	inner commandRunner
	timer *initTimer
}

func (r timingRunner) Run(ctx context.Context, dir, name string, args ...string) (string, string, error) {
	stop := r.timer.span(commandLabel(name, args))
	stdout, stderr, err := r.inner.Run(ctx, dir, name, args...)
	stop()
	return stdout, stderr, err
}

// commandLabel derives a stable, readable timing label from an external command
// invocation. git/npx get their subcommand; a `<self> apps +xxx` subprocess is
// labeled by its shortcut name. Mirrors the routing in fakeCommandRunner.Run.
func commandLabel(name string, args []string) string {
	base := filepath.Base(name)
	switch {
	case base == "git":
		if len(args) > 0 {
			return "git " + args[0]
		}
		return "git"
	case base == "npx":
		if sub := npxSubcommand(args); sub != "" {
			return "npx " + sub
		}
		return "npx"
	case len(args) >= 2 && args[0] == "apps":
		// A `<self> apps +git-credential-init|+env-pull ...` subprocess.
		return "subprocess " + args[1]
	default:
		return base
	}
}

// npxSubcommand extracts the miaoda-cli subcommand ("app init", "app sync",
// "skills sync") from an npx arg list whose leading tokens are flags and the
// package spec. Returns "" when no known subcommand verb is present.
func npxSubcommand(args []string) string {
	for i, a := range args {
		if (a == "app" || a == "skills") && i+1 < len(args) {
			return a + " " + args[i+1]
		}
	}
	return ""
}

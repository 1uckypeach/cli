// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/internal/output"
)

// scopedInfo fetches the registered fakescoped ProviderInfo (4 RequiredScopes,
// see scripted_provider_test.go) — the all-or-nothing preflight requires every
// one of fakescopedAllScopes for any real API verb.
func scopedInfo(t *testing.T) iagent.ProviderInfo {
	t.Helper()
	registerScripted()
	info, ok := iagent.Info("fakescoped")
	if !ok {
		t.Fatal("fakescoped provider should be registered")
	}
	return info
}

// requirePreflightError asserts err is the missing_scope permission error
// (exit 3, mirroring the event-consume scope preflight) and returns the typed
// value for field assertions.
func requirePreflightError(t *testing.T, err error) *errs.PermissionError {
	t.Helper()
	if err == nil {
		t.Fatal("want missing_scope error, got nil")
	}
	var pe *errs.PermissionError
	if !errors.As(err, &pe) {
		t.Fatalf("want *errs.PermissionError, got %T: %v", err, err)
	}
	if pe.Subtype != errs.SubtypeMissingScope {
		t.Fatalf("subtype should be missing_scope, got %q", pe.Subtype)
	}
	if code := output.ExitCodeOf(err); code != 3 {
		t.Fatalf("exit code should be 3, got %d", code)
	}
	return pe
}

// TestPreflightReportsAllMissingWithMergedHint is the all-or-nothing pin: the
// check is all-or-nothing, so a user token holding only some of the provider's scopes fails
// with EVERY missing scope named (sorted) in both the message and
// missing_scopes, and a re-auth hint that merges the stored token scopes with
// the provider's FULL RequiredScopes set (sorted, so re-running the login
// command never drops an existing grant).
func TestPreflightReportsAllMissingWithMergedHint(t *testing.T) {
	err := preflightScopes(preflightInput{
		Identity:    core.AsUser,
		TokenScopes: []string{"im:message", "fakescoped:agent_chat:write"},
		Info:        scopedInfo(t),
	})
	ve := requirePreflightError(t, err)

	wantMissing := []string{"fakescoped:agent_artifact:read", "fakescoped:agent_attachment:write", "fakescoped:agent_chat:read"}
	if !strings.Contains(ve.Message, "当前 user 身份缺少本命令所需 scope: "+strings.Join(wantMissing, ", ")) {
		t.Errorf("message should list all missing scopes, got %q", ve.Message)
	}
	if !reflect.DeepEqual(ve.MissingScopes, wantMissing) {
		t.Errorf("missing_scopes should be %v (all missing, stable sort), got %v", wantMissing, ve.MissingScopes)
	}
	// Merged hint: existing token scopes ∪ FULL provider RequiredScopes, sorted.
	wantScopeArg := `lark-cli auth login --scope "fakescoped:agent_artifact:read fakescoped:agent_attachment:write fakescoped:agent_chat:read fakescoped:agent_chat:write im:message"`
	if !strings.Contains(ve.Hint, wantScopeArg) {
		t.Errorf("hint should contain the merged full-scope command %q, got %q", wantScopeArg, ve.Hint)
	}
}

// TestPreflightBotSkipped pins that a bot token has no scope list concept, so
// preflight is skipped entirely regardless of TokenScopes.
func TestPreflightBotSkipped(t *testing.T) {
	err := preflightScopes(preflightInput{
		Identity:    core.AsBot,
		TokenScopes: nil,
		Info:        scopedInfo(t),
	})
	if err != nil {
		t.Fatalf("bot identity should skip preflight, got %v", err)
	}
}

// TestPreflightNoTokenScopesReturnsNil pins that no local token (or a token
// without a scope list) yields nil so the downstream not_configured /
// need-authorization path owns the error.
func TestPreflightNoTokenScopesReturnsNil(t *testing.T) {
	err := preflightScopes(preflightInput{
		Identity:    core.AsUser,
		TokenScopes: nil,
		Info:        scopedInfo(t),
	})
	if err != nil {
		t.Fatalf("no token scope list should return nil, got %v", err)
	}
}

// TestPreflightAllScopesPresent pins the happy path: a token carrying all four
// fakescoped scopes passes the all-or-nothing check.
func TestPreflightAllScopesPresent(t *testing.T) {
	if err := preflightScopes(preflightInput{
		Identity: core.AsUser, TokenScopes: fakescopedAllScopes, Info: scopedInfo(t),
	}); err != nil {
		t.Errorf("should pass when all scopes present, got %v", err)
	}
}

// TestPreflightMissingAnyScopeFails pins the all-or-nothing rule: a token that
// is missing even a single scope fails, and the reported missing set is exactly
// the scopes it lacks (not just this-verb scopes — the per-verb concept is
// gone).
func TestPreflightMissingAnyScopeFails(t *testing.T) {
	// Missing exactly one scope (attachment) → that one scope is reported.
	ve := requirePreflightError(t, preflightScopes(preflightInput{
		Identity: core.AsUser,
		TokenScopes: []string{
			"fakescoped:agent_chat:write", "fakescoped:agent_chat:read", "fakescoped:agent_artifact:read",
		},
		Info: scopedInfo(t),
	}))
	if !reflect.DeepEqual(ve.MissingScopes, []string{"fakescoped:agent_attachment:write"}) {
		t.Errorf("when only attachment is missing, missing_scopes should be [fakescoped:agent_attachment:write], got %v", ve.MissingScopes)
	}

	// Only the write scope → the other three are all reported.
	ve = requirePreflightError(t, preflightScopes(preflightInput{
		Identity: core.AsUser, TokenScopes: []string{"fakescoped:agent_chat:write"}, Info: scopedInfo(t),
	}))
	wantMissing := []string{"fakescoped:agent_artifact:read", "fakescoped:agent_attachment:write", "fakescoped:agent_chat:read"}
	if !reflect.DeepEqual(ve.MissingScopes, wantMissing) {
		t.Errorf("with only the write scope, missing_scopes should be %v, got %v", wantMissing, ve.MissingScopes)
	}
}

// ---------------------------------------------------------------------------
// Command wiring: each verb runs preflight after resolveProvider and before
// any real API call. The stored-scope read goes through the storedUserScopes
// seam so no test touches the real keychain; zero httpmock stubs are
// registered, so any HTTP request would fail the test with a transport error
// instead of the asserted missing_scope.
// ---------------------------------------------------------------------------

// swapStoredScopes swaps the storedUserScopes seam for the test's scope list.
func swapStoredScopes(t *testing.T, scopes []string) {
	t.Helper()
	old := storedUserScopes
	storedUserScopes = func(*cmdutil.Factory) []string { return scopes }
	t.Cleanup(func() { storedUserScopes = old })
}

// userLeafCmd builds a leaf command under lark-cli/agent/... with --as
// explicitly set to user so ResolveAs honors it verbatim.
func userLeafCmd(t *testing.T, names ...string) *cobra.Command {
	t.Helper()
	parent := &cobra.Command{Use: "lark-cli"}
	for _, name := range names {
		child := &cobra.Command{Use: name}
		parent.AddCommand(child)
		parent = child
	}
	parent.Flags().String("as", "", "identity")
	if err := parent.Flags().Set("as", "user"); err != nil {
		t.Fatal(err)
	}
	parent.SetContext(context.Background())
	return parent
}

// userFactory builds a test Factory + registry for a user-identity run.
func userFactory(t *testing.T) (*cmdutil.Factory, *httpmock.Registry) {
	t.Helper()
	f, _, _, reg := cmdutil.TestFactory(t, &core.CliConfig{AppID: "cli_x", AppSecret: "fake-secret", Brand: core.BrandFeishu})
	return f, reg
}

// TestSendPreflightBlocksMissingScope pins the send wiring: a user token that
// holds none of the provider's scopes fails with missing_scope
// (reporting the full set) and no request.
func TestSendPreflightBlocksMissingScope(t *testing.T) {
	swapStoredScopes(t, []string{"im:message"})
	f, _ := userFactory(t)
	err := agentSendRun(&sendOptions{
		Factory: f, Cmd: userLeafCmd(t, "agent", "send"),
		Ref: "fakescoped:agt_x", Text: "hi", As: "user",
	})
	ve := requirePreflightError(t, err)
	if !reflect.DeepEqual(ve.MissingScopes, fakescopedAllScopes) {
		t.Errorf("with no provider scope, send should report all missing %v, got %v", fakescopedAllScopes, ve.MissingScopes)
	}
}

// TestSendPreflightPartialTokenBlocked pins that a partial token (write only)
// still fails the all-or-nothing check, reporting the three scopes it lacks.
func TestSendPreflightPartialTokenBlocked(t *testing.T) {
	swapStoredScopes(t, []string{"fakescoped:agent_chat:write"})
	f, _ := userFactory(t)
	err := agentSendRun(&sendOptions{
		Factory: f, Cmd: userLeafCmd(t, "agent", "send"),
		Ref: "fakescoped:agt_x", Text: "hi", As: "user",
	})
	ve := requirePreflightError(t, err)
	wantMissing := []string{"fakescoped:agent_artifact:read", "fakescoped:agent_attachment:write", "fakescoped:agent_chat:read"}
	if !reflect.DeepEqual(ve.MissingScopes, wantMissing) {
		t.Errorf("write-only token should report missing %v, got %v", wantMissing, ve.MissingScopes)
	}
}

// TestSendDryRunSkipsPreflight pins that --dry-run stays API-free AND
// scope-free — it succeeds even when the token has none of the provider scopes.
func TestSendDryRunSkipsPreflight(t *testing.T) {
	swapStoredScopes(t, []string{"im:message"})
	f, _ := userFactory(t)
	err := agentSendRun(&sendOptions{
		Factory: f, Cmd: userLeafCmd(t, "agent", "send"),
		Ref: "fakescoped:agt_x", Text: "hi", As: "user", DryRun: true,
	})
	if err != nil {
		t.Fatalf("--dry-run should not run scope preflight: %v", err)
	}
}

// TestTaskGetPreflightBlocksMissingScope pins the task get wiring.
func TestTaskGetPreflightBlocksMissingScope(t *testing.T) {
	swapStoredScopes(t, []string{"fakescoped:agent_chat:write"})
	f, _ := userFactory(t)
	err := agentTaskGetRun(&taskOptions{
		Factory: f, Cmd: userLeafCmd(t, "agent", "task", "get"),
		Ref: "fakescoped:agt_x", TaskID: "t1", As: "user",
	})
	ve := requirePreflightError(t, err)
	if !contains(ve.MissingScopes, "fakescoped:agent_chat:read") {
		t.Errorf("task get missing scope should include fakescoped:agent_chat:read, got %v", ve.MissingScopes)
	}
}

// TestTaskGetArtifactPreflightFires pins the --artifact download wiring
// (resolveDownload path): it too runs the all-or-nothing preflight before the
// API call.
func TestTaskGetArtifactPreflightFires(t *testing.T) {
	swapStoredScopes(t, []string{"fakescoped:agent_chat:read"})
	f, _ := userFactory(t)
	err := agentTaskGetRun(&taskOptions{
		Factory: f, Cmd: userLeafCmd(t, "agent", "task", "get"),
		Ref: "fakescoped:agt_x", TaskID: "t1", As: "user",
		ArtifactID: "art_1", Output: "out.bin",
	})
	ve := requirePreflightError(t, err)
	if !contains(ve.MissingScopes, "fakescoped:agent_artifact:read") {
		t.Errorf("task get --artifact missing scope should include fakescoped:agent_artifact:read, got %v", ve.MissingScopes)
	}
}

// TestTaskListPreflightBlocksMissingScope pins the task list wiring.
func TestTaskListPreflightBlocksMissingScope(t *testing.T) {
	swapStoredScopes(t, []string{"fakescoped:agent_chat:write"})
	f, _ := userFactory(t)
	err := agentTaskListRun(&taskOptions{
		Factory: f, Cmd: userLeafCmd(t, "agent", "task", "list"),
		Ref: "fakescoped:agt_x", As: "user",
	})
	requirePreflightError(t, err)
}

// TestContextVerbsPreflightBlocksMissingScope pins the context list/get/delete
// wiring: all three run the all-or-nothing preflight.
func TestContextVerbsPreflightBlocksMissingScope(t *testing.T) {
	runs := []struct {
		name string
		run  func(f *cmdutil.Factory) error
	}{
		{"list", func(f *cmdutil.Factory) error {
			return agentContextListRun(&contextOptions{
				Factory: f, Cmd: userLeafCmd(t, "agent", "context", "list"),
				Ref: "fakescoped:agt_x", As: "user", Format: "pretty",
			})
		}},
		{"get", func(f *cmdutil.Factory) error {
			return agentContextGetRun(&contextOptions{
				Factory: f, Cmd: userLeafCmd(t, "agent", "context", "get"),
				Ref: "fakescoped:agt_x", CtxID: "ctx_1", As: "user",
			})
		}},
		{"delete", func(f *cmdutil.Factory) error {
			return agentContextDeleteRun(&contextOptions{
				Factory: f, Cmd: userLeafCmd(t, "agent", "context", "delete"),
				Ref: "fakescoped:agt_x", CtxID: "ctx_1", As: "user", Yes: true,
			})
		}},
	}
	for _, tc := range runs {
		t.Run(tc.name, func(t *testing.T) {
			swapStoredScopes(t, []string{"fakescoped:agent_chat:write"})
			f, _ := userFactory(t)
			requirePreflightError(t, tc.run(f))
		})
	}
}

// TestSendPreflightPassesWithScopeAndSends pins that a token holding the full
// provider scope set lets the real send proceed (the scripted Send hook fires,
// proving preflight did not false-positive).
func TestSendPreflightPassesWithScopeAndSends(t *testing.T) {
	swapStoredScopes(t, fakescopedAllScopes)
	f, _ := userFactory(t)
	sent := false
	setScripted(t, scriptedHooks{send: func(iagent.SendInput) (*iagent.AgentTask, error) {
		sent = true
		return &iagent.AgentTask{TaskID: "chat_1", ContextID: "sess_1", State: iagent.StateWorking}, nil
	}})
	err := agentSendRun(&sendOptions{
		Factory: f, Cmd: userLeafCmd(t, "agent", "send"),
		Ref: "fakescoped:agt_x", Text: "hi", As: "user",
	})
	if err != nil {
		t.Fatalf("a send with all scopes should pass preflight and send: %v", err)
	}
	if !sent {
		t.Fatal("provider.Send should actually be called after preflight passes")
	}
}

// TestTaskCancelPreflightWired pins the task cancel wiring: the capability
// gate (fakescoped card declares task_cancel=false) answers before
// provider/preflight, so a scope-missing user token yields
// unsupported_capability, not missing_scope — proving the wired
// preflight does not change the gate-first ordering.
func TestTaskCancelPreflightWired(t *testing.T) {
	swapStoredScopes(t, []string{"im:message"})
	f, _ := userFactory(t)
	err := agentTaskCancelRun(&taskOptions{
		Factory: f, Cmd: userLeafCmd(t, "agent", "task", "cancel"),
		Ref: "fakescoped:agt_x", TaskID: "t1", As: "user",
	})
	if err == nil {
		t.Fatal("task cancel with task_cancel=false should be blocked by the capability gate")
	}
	p, ok := errs.ProblemOf(err)
	if !ok || p.Subtype != errs.Subtype("unsupported_capability") {
		t.Fatalf("want unsupported_capability (capability gate answers first), got %+v", p)
	}
}

// contains reports whether s appears in the slice.
func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

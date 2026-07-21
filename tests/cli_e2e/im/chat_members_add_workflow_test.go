// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const chatMembersAddWriteScope = "im:chat.members:write_only"

func TestIM_ChatMembersAddWorkflow(t *testing.T) {
	clie2e.SkipWithoutTenantAccessToken(t)
	clie2e.SkipWithoutUserToken(t)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)
	parentT := t
	suffix := clie2e.GenerateSuffix()

	selfOpenID := getSelfOpenIDForChatMembersAdd(t, ctx)
	botChatID := createChatAs(t, parentT, ctx, "lark-cli-e2e-member-add-bot-"+suffix, "bot")
	botAppID := getBotAppIDFromChat(t, ctx, botChatID)
	userChatID := createChatAs(t, parentT, ctx, "lark-cli-e2e-member-add-user-"+suffix, "user")

	t.Run("add user as bot and read back", func(t *testing.T) {
		baseline := listChatMembers(t, ctx, botChatID, "user", "bot")
		assertCompleteChatMemberList(t, baseline)
		require.False(t, chatMemberListContains(baseline, "users", "member_id", selfOpenID), "target user must be absent before add")

		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args: []string{
				"im", "+chat-members-add",
				"--chat-id", botChatID,
				"--users", selfOpenID,
			},
			DefaultAs: "bot",
			Yes:       true,
		})
		require.NoError(t, err)
		skipIfMissingChatMemberPermission(t, result)
		assertChatMembersAddSuccess(t, result)

		readback := waitForChatMember(t, ctx, botChatID, "user", "bot", "users", "member_id", selfOpenID)
		assertCompleteChatMemberList(t, readback)
	})

	t.Run("add bot as user and read back", func(t *testing.T) {
		baseline := listChatMembers(t, ctx, userChatID, "bot", "user")
		assertCompleteChatMemberList(t, baseline)
		require.False(t, chatMemberListContains(baseline, "bots", "app_id", botAppID), "target bot must be absent before add")

		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args: []string{
				"im", "+chat-members-add",
				"--chat-id", userChatID,
				"--bots", botAppID,
			},
			DefaultAs: "user",
			Yes:       true,
		})
		require.NoError(t, err)
		skipIfMissingChatMemberPermission(t, result)
		assertChatMembersAddSuccess(t, result)

		readback := waitForChatMember(t, ctx, userChatID, "bot", "user", "bots", "app_id", botAppID)
		assertCompleteChatMemberList(t, readback)
	})
}

func getSelfOpenIDForChatMembersAdd(t *testing.T, ctx context.Context) string {
	t.Helper()

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args:      []string{"contact", "+get-user"},
		DefaultAs: "user",
	})
	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode, "contact lookup failed: %s", resultErrorSummary(result))
	require.True(t, gjson.Get(result.Stdout, "ok").Bool(), "contact lookup returned ok=false")

	openID := gjson.Get(result.Stdout, "data.user.open_id").String()
	require.NotEmpty(t, openID, "contact lookup returned an empty open_id")
	return openID
}

func getBotAppIDFromChat(t *testing.T, ctx context.Context, chatID string) string {
	t.Helper()

	result, err := clie2e.RunCmdWithRetry(ctx, chatMembersListRequest(chatID, "bot", "bot"), clie2e.RetryOptions{
		Attempts:     6,
		InitialDelay: time.Second,
		MaxDelay:     4 * time.Second,
		ShouldRetry: func(result *clie2e.Result) bool {
			if result == nil {
				return true
			}
			if result.ExitCode != 0 {
				return clie2e.ResultHasRetryableError(result)
			}
			for _, bot := range gjson.Get(result.Stdout, "data.bots").Array() {
				if bot.Get("app_id").String() != "" {
					return false
				}
			}
			return true
		},
	})
	require.NoError(t, err)
	skipIfMissingChatMemberPermission(t, result)
	require.Equal(t, 0, result.ExitCode, "bot member discovery failed: %s", resultErrorSummary(result))
	require.True(t, gjson.Get(result.Stdout, "ok").Bool(), "bot member discovery returned ok=false")
	assertCompleteChatMemberList(t, result)

	for _, bot := range gjson.Get(result.Stdout, "data.bots").Array() {
		if appID := bot.Get("app_id").String(); appID != "" {
			return appID
		}
	}
	t.Fatal("bot member discovery returned no app_id")
	return ""
}

func chatMembersListRequest(chatID, memberType, defaultAs string) clie2e.Request {
	return clie2e.Request{
		Args: []string{
			"im", "+chat-members-list",
			"--chat-id", chatID,
			"--member-types", memberType,
			"--page-all",
			"--page-limit", "0",
		},
		DefaultAs: defaultAs,
	}
}

func listChatMembers(t *testing.T, ctx context.Context, chatID, memberType, defaultAs string) *clie2e.Result {
	t.Helper()

	result, err := clie2e.RunCmd(ctx, chatMembersListRequest(chatID, memberType, defaultAs))
	require.NoError(t, err)
	skipIfMissingChatMemberPermission(t, result)
	require.Equal(t, 0, result.ExitCode, "member list failed: %s", resultErrorSummary(result))
	require.True(t, gjson.Get(result.Stdout, "ok").Bool(), "member list returned ok=false")
	return result
}

func waitForChatMember(
	t *testing.T,
	ctx context.Context,
	chatID string,
	memberType string,
	defaultAs string,
	bucket string,
	idField string,
	id string,
) *clie2e.Result {
	t.Helper()

	result, err := clie2e.RunCmdWithRetry(ctx, chatMembersListRequest(chatID, memberType, defaultAs), clie2e.RetryOptions{
		Attempts:     8,
		InitialDelay: time.Second,
		MaxDelay:     5 * time.Second,
		ShouldRetry: func(result *clie2e.Result) bool {
			if result == nil {
				return true
			}
			if result.ExitCode != 0 {
				return clie2e.ResultHasRetryableError(result)
			}
			return !chatMemberListContains(result, bucket, idField, id)
		},
	})
	require.NoError(t, err)
	skipIfMissingChatMemberPermission(t, result)
	require.Equal(t, 0, result.ExitCode, "member readback failed: %s", resultErrorSummary(result))
	require.True(t, gjson.Get(result.Stdout, "ok").Bool(), "member readback returned ok=false")
	require.True(t, chatMemberListContains(result, bucket, idField, id), "added member was absent after bounded readback")
	return result
}

func chatMemberListContains(result *clie2e.Result, bucket, idField, id string) bool {
	if result == nil {
		return false
	}
	for _, member := range gjson.Get(result.Stdout, "data."+bucket).Array() {
		if member.Get(idField).String() == id {
			return true
		}
	}
	return false
}

func assertCompleteChatMemberList(t *testing.T, result *clie2e.Result) {
	t.Helper()
	require.False(t, gjson.Get(result.Stdout, "data.has_more").Bool(), "member list must include all pages")
	require.Empty(t, gjson.Get(result.Stdout, "data.truncations").Array(), "member list must not contain truncation markers")
}

func assertChatMembersAddSuccess(t *testing.T, result *clie2e.Result) {
	t.Helper()
	require.Equal(t, 0, result.ExitCode, "member add failed: %s", resultErrorSummary(result))
	require.True(t, gjson.Get(result.Stdout, "ok").Bool(), "member add returned ok=false")
	require.Equal(t, int64(1), gjson.Get(result.Stdout, "data.success_count").Int())

	for _, field := range []string{"invalid_id_list", "not_existed_id_list", "pending_approval_id_list"} {
		value := gjson.Get(result.Stdout, "data."+field)
		require.True(t, value.Exists(), "%s must be present", field)
		require.True(t, value.IsArray(), "%s must be an array", field)
		require.Empty(t, value.Array(), "%s must be empty", field)
	}
}

func skipIfMissingChatMemberPermission(t *testing.T, result *clie2e.Result) {
	t.Helper()
	if result == nil || result.ExitCode == 0 {
		return
	}

	scopes := missingPermissionNames(result)
	if len(scopes) == 0 {
		return
	}
	t.Skipf("skipped: missing IM member permissions: %s", strings.Join(scopes, ", "))
}

func missingPermissionNames(result *clie2e.Result) []string {
	if result == nil {
		return nil
	}

	names := map[string]struct{}{}
	fallbackNames := map[string]struct{}{}
	missingScopeFailure := false
	for _, raw := range []string{result.Stdout, result.Stderr} {
		payload := strings.TrimSpace(raw)
		if !gjson.Valid(payload) {
			continue
		}
		subtype := gjson.Get(payload, "error.subtype").String()
		code := gjson.Get(payload, "error.code").Int()
		if !isMissingIMMemberScope(subtype, code) {
			continue
		}

		missingScopeFailure = true
		if isMissingIMMemberScopeCode(code) {
			fallbackNames["scope code "+strconv.FormatInt(code, 10)] = struct{}{}
		} else {
			fallbackNames["IM chat member scope"] = struct{}{}
		}
		for _, scope := range gjson.Get(payload, "error.missing_scopes").Array() {
			if name := scope.String(); name != "" {
				names[name] = struct{}{}
			}
		}
	}

	if !missingScopeFailure {
		return nil
	}
	if len(names) == 0 {
		for name := range fallbackNames {
			names[name] = struct{}{}
		}
	}

	resultNames := make([]string, 0, len(names))
	for name := range names {
		resultNames = append(resultNames, name)
	}
	sort.Strings(resultNames)
	return resultNames
}

func isMissingIMMemberScope(subtype string, code int64) bool {
	switch subtype {
	case "missing_scope", "app_scope_not_applied", "token_scope_insufficient":
		return true
	default:
		return isMissingIMMemberScopeCode(code)
	}
}

func isMissingIMMemberScopeCode(code int64) bool {
	switch code {
	case 99991672, 99991676, 99991679:
		return true
	default:
		return false
	}
}

func TestMissingIMMemberPermissionNames(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		stdout string
		want   []string
	}{
		{
			name:   "missing scope subtype",
			stderr: `{"error":{"type":"authorization","subtype":"missing_scope","missing_scopes":["im:chat.members:read"]}}`,
			want:   []string{"im:chat.members:read"},
		},
		{
			name:   "app scope not applied subtype",
			stderr: `{"error":{"type":"authorization","subtype":"app_scope_not_applied","missing_scopes":["im:chat.members:write_only"]}}`,
			want:   []string{chatMembersAddWriteScope},
		},
		{
			name:   "token scope insufficient subtype",
			stderr: `{"error":{"type":"authorization","subtype":"token_scope_insufficient"}}`,
			want:   []string{"IM chat member scope"},
		},
		{
			name:   "app scope not applied code",
			stderr: `{"error":{"type":"api_error","code":99991672}}`,
			want:   []string{"scope code 99991672"},
		},
		{
			name:   "token scope insufficient code",
			stderr: `{"error":{"type":"api_error","code":99991676}}`,
			want:   []string{"scope code 99991676"},
		},
		{
			name:   "missing scope code on stdout",
			stdout: `{"error":{"type":"api_error","code":99991679}}`,
			want:   []string{"scope code 99991679"},
		},
		{
			name:   "authorization category alone",
			stderr: `{"error":{"type":"authorization","subtype":"unknown"}}`,
		},
		{
			name:   "ordinary permission denied",
			stderr: `{"error":{"type":"authorization","subtype":"permission_denied"}}`,
		},
		{
			name:   "not in chat resource state",
			stderr: `{"error":{"type":"validation","subtype":"failed_precondition","message":"not in chat"}}`,
		},
		{
			name:   "no invite permission",
			stderr: `{"error":{"type":"authorization","subtype":"permission_denied","message":"no invite permission"}}`,
		},
		{
			name:   "scope substring is not accepted",
			stderr: `{"error":{"type":"authorization","subtype":"unknown_scope_state"}}`,
		},
		{
			name:   "unknown error",
			stderr: `{"error":{"type":"internal","subtype":"unknown"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &clie2e.Result{
				ExitCode: 3,
				Stdout:   tt.stdout,
				Stderr:   tt.stderr,
			}
			require.Equal(t, tt.want, missingPermissionNames(result))
		})
	}
}

func resultErrorSummary(result *clie2e.Result) string {
	if result == nil {
		return "result=nil"
	}
	for _, raw := range []string{result.Stderr, result.Stdout} {
		payload := strings.TrimSpace(raw)
		if !gjson.Valid(payload) {
			continue
		}
		return "type=" + gjson.Get(payload, "error.type").String() +
			" subtype=" + gjson.Get(payload, "error.subtype").String()
	}
	return "structured error unavailable"
}

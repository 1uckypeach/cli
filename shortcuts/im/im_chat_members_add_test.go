// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
	"github.com/spf13/cobra"
)

func TestReadChatMembersAddSpec(t *testing.T) {
	userIDsOverLimit := make([]string, imChatMembersAddUserLimit+1)
	for i := range userIDsOverLimit {
		userIDsOverLimit[i] = "ou_user_" + strings.Repeat("a", i+1)
	}
	botIDsOverLimit := make([]string, imChatMembersAddBotLimit+1)
	for i := range botIDsOverLimit {
		botIDsOverLimit[i] = "cli_bot_" + strings.Repeat("a", i+1)
	}

	tests := []struct {
		name        string
		chatID      string
		users       string
		bots        string
		wantParam   string
		wantParams  []string
		wantMessage string
	}{
		{
			name:      "missing chat",
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:        "missing members",
			chatID:      "oc_chat_a",
			wantParams:  []string{"--users", "--bots"},
			wantMessage: "specify at least one of --users or --bots",
		},
		{
			name:      "chat control character",
			chatID:    "oc_chat\x00a",
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:      "chat unicode suffix",
			chatID:    "oc_群聊",
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:      "chat URL metacharacter",
			chatID:    "oc_chat?mode=admin",
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:      "chat identifier too long",
			chatID:    "oc_" + strings.Repeat("a", imChatMembersAddIDMaxBytes-2),
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:      "chat URL token receives local validation",
			chatID:    "https://tenant.feishu.cn/messenger/chat/oc_chat?mode=admin",
			users:     "ou_user_a",
			wantParam: "--chat-id",
		},
		{
			name:      "invalid user prefix",
			chatID:    "oc_chat_a",
			users:     "user_a",
			wantParam: "--users",
		},
		{
			name:      "empty user suffix",
			chatID:    "oc_chat_a",
			users:     "ou_",
			wantParam: "--users",
		},
		{
			name:      "invalid bot prefix",
			chatID:    "oc_chat_a",
			bots:      "bot_a",
			wantParam: "--bots",
		},
		{
			name:      "empty bot suffix",
			chatID:    "oc_chat_a",
			bots:      "cli_",
			wantParam: "--bots",
		},
		{
			name:      "user control character",
			chatID:    "oc_chat_a",
			users:     "ou_user\x00a",
			wantParam: "--users",
		},
		{
			name:      "bot control character",
			chatID:    "oc_chat_a",
			bots:      "cli_bot\x1fa",
			wantParam: "--bots",
		},
		{
			name:      "user identifier too long",
			chatID:    "oc_chat_a",
			users:     "ou_" + strings.Repeat("a", imChatMembersAddIDMaxBytes-2),
			wantParam: "--users",
		},
		{
			name:      "bot identifier too long",
			chatID:    "oc_chat_a",
			bots:      "cli_" + strings.Repeat("a", imChatMembersAddIDMaxBytes-3),
			wantParam: "--bots",
		},
		{
			name:        "too many users",
			chatID:      "oc_chat_a",
			users:       strings.Join(userIDsOverLimit, ","),
			wantParam:   "--users",
			wantMessage: "--users accepts at most 50 unique IDs",
		},
		{
			name:        "too many bots",
			chatID:      "oc_chat_a",
			bots:        strings.Join(botIDsOverLimit, ","),
			wantParam:   "--bots",
			wantMessage: "--bots accepts at most 5 unique IDs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := newChatMembersAddTestRuntime(t, tt.chatID, tt.users, tt.bots)
			_, err := readChatMembersAddSpec(runtime)
			assertChatMembersAddValidationError(t, err, tt.wantParam, tt.wantParams, tt.wantMessage)
		})
	}
}

func TestReadChatMembersAddSpecAcceptsNormalizedChatURL(t *testing.T) {
	runtime := newChatMembersAddTestRuntime(
		t,
		"https://tenant.feishu.cn/messenger/chat/oc_chat_a",
		"ou_user_a",
		"",
	)

	got, err := readChatMembersAddSpec(runtime)
	if err != nil {
		t.Fatalf("readChatMembersAddSpec() error = %v", err)
	}
	if got.ChatID != "oc_chat_a" {
		t.Fatalf("ChatID = %q, want %q", got.ChatID, "oc_chat_a")
	}
}

func TestReadChatMembersAddSpecCountsUniqueIDs(t *testing.T) {
	usersAtLimit := makeChatMembersAddIDs("ou_user_", imChatMembersAddUserLimit)
	botsAtLimit := makeChatMembersAddIDs("cli_bot_", imChatMembersAddBotLimit)

	tests := []struct {
		name      string
		users     []string
		bots      []string
		wantUsers int
		wantBots  int
	}{
		{
			name:      "51 user entries with one duplicate",
			users:     append(append([]string(nil), usersAtLimit...), usersAtLimit[0]),
			wantUsers: imChatMembersAddUserLimit,
		},
		{
			name:     "6 bot entries with one duplicate",
			bots:     append(append([]string(nil), botsAtLimit...), botsAtLimit[0]),
			wantBots: imChatMembersAddBotLimit,
		},
		{
			name:      "exact user and bot limits",
			users:     usersAtLimit,
			bots:      botsAtLimit,
			wantUsers: imChatMembersAddUserLimit,
			wantBots:  imChatMembersAddBotLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := newChatMembersAddTestRuntime(
				t,
				"oc_chat_a",
				strings.Join(tt.users, ","),
				strings.Join(tt.bots, ","),
			)
			got, err := readChatMembersAddSpec(runtime)
			if err != nil {
				t.Fatalf("readChatMembersAddSpec() error = %v", err)
			}
			if len(got.Users) != tt.wantUsers || len(got.Bots) != tt.wantBots {
				t.Fatalf(
					"member counts = users %d bots %d, want users %d bots %d",
					len(got.Users),
					len(got.Bots),
					tt.wantUsers,
					tt.wantBots,
				)
			}
		})
	}
}

func TestReadChatMembersAddSpecErrorsDoNotEchoMemberIDs(t *testing.T) {
	tests := []struct {
		name  string
		users string
		bots  string
		rawID string
	}{
		{
			name:  "invalid user characters",
			users: "ou_private@example.com",
			rawID: "ou_private@example.com",
		},
		{
			name:  "invalid bot characters",
			bots:  "cli_private@example.com",
			rawID: "cli_private@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := newChatMembersAddTestRuntime(t, "oc_chat_a", tt.users, tt.bots)
			_, err := readChatMembersAddSpec(runtime)
			if err == nil {
				t.Fatal("readChatMembersAddSpec() error = nil, want validation error")
			}
			if strings.Contains(err.Error(), tt.rawID) {
				t.Fatalf("error message contains the member identifier: %q", err.Error())
			}
		})
	}
}

func TestReadChatMembersAddSpecDeduplicatesInFirstSeenOrder(t *testing.T) {
	runtime := newChatMembersAddTestRuntime(
		t,
		"oc_chat_a",
		"ou_b,ou_a,ou_b",
		"cli_b,cli_a,cli_b",
	)

	got, err := readChatMembersAddSpec(runtime)
	if err != nil {
		t.Fatalf("readChatMembersAddSpec() error = %v", err)
	}

	want := chatMembersAddSpec{
		ChatID: "oc_chat_a",
		Users:  []string{"ou_b", "ou_a"},
		Bots:   []string{"cli_b", "cli_a"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readChatMembersAddSpec() = %#v, want %#v", got, want)
	}
}

func TestDedupeChatMemberIDs(t *testing.T) {
	got := dedupeChatMemberIDs([]string{"ou_b", "ou_a", "ou_b", "ou_a", "ou_c"})
	want := []string{"ou_b", "ou_a", "ou_c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dedupeChatMemberIDs() = %#v, want %#v", got, want)
	}
}

func TestBuildChatMembersAddDryRun(t *testing.T) {
	tests := []struct {
		name       string
		spec       chatMembersAddSpec
		wantTypes  []string
		wantBodies [][]string
	}{
		{
			name: "users only",
			spec: chatMembersAddSpec{
				ChatID: "oc_test/slash",
				Users:  []string{"ou_b", "ou_a"},
			},
			wantTypes:  []string{"open_id"},
			wantBodies: [][]string{{"ou_b", "ou_a"}},
		},
		{
			name: "bots only",
			spec: chatMembersAddSpec{
				ChatID: "oc_test/slash",
				Bots:   []string{"cli_b", "cli_a"},
			},
			wantTypes:  []string{"app_id"},
			wantBodies: [][]string{{"cli_b", "cli_a"}},
		},
		{
			name: "users before bots",
			spec: chatMembersAddSpec{
				ChatID: "oc_test/slash",
				Users:  []string{"ou_b", "ou_a"},
				Bots:   []string{"cli_b", "cli_a"},
			},
			wantTypes:  []string{"open_id", "app_id"},
			wantBodies: [][]string{{"ou_b", "ou_a"}, {"cli_b", "cli_a"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dryRun := buildChatMembersAddDryRun(tt.spec)
			encoded, err := json.Marshal(dryRun)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			var payload struct {
				API []struct {
					Method string                 `json:"method"`
					URL    string                 `json:"url"`
					Params map[string]interface{} `json:"params"`
					Body   struct {
						IDList []string `json:"id_list"`
					} `json:"body"`
				} `json:"api"`
			}
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if len(payload.API) != len(tt.wantTypes) {
				t.Fatalf("api call count = %d, want %d", len(payload.API), len(tt.wantTypes))
			}
			for i, call := range payload.API {
				if call.Method != http.MethodPost {
					t.Errorf("api[%d].method = %q, want %q", i, call.Method, http.MethodPost)
				}
				if call.URL != "/open-apis/im/v1/chats/oc_test%2Fslash/members" {
					t.Errorf("api[%d].url = %q, want safely encoded chat ID", i, call.URL)
				}
				if got := call.Params["member_id_type"]; got != tt.wantTypes[i] {
					t.Errorf("api[%d].params.member_id_type = %#v, want %q", i, got, tt.wantTypes[i])
				}
				if got := call.Params["succeed_type"]; got != "1" {
					t.Errorf("api[%d].params.succeed_type = %#v, want %q", i, got, "1")
				}
				if !reflect.DeepEqual(call.Body.IDList, tt.wantBodies[i]) {
					t.Errorf("api[%d].body.id_list = %#v, want %#v", i, call.Body.IDList, tt.wantBodies[i])
				}
			}
		})
	}
}

func TestExecuteChatMembersAddUsersBeforeBots(t *testing.T) {
	type recordedCall struct {
		method       string
		escapedPath  string
		memberIDType string
		succeedType  string
		body         struct {
			IDList []string `json:"id_list"`
		}
	}
	var calls []recordedCall
	runtime := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var call recordedCall
		call.method = req.Method
		call.escapedPath = req.URL.EscapedPath()
		call.memberIDType = req.URL.Query().Get("member_id_type")
		call.succeedType = req.URL.Query().Get("succeed_type")
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("io.ReadAll() error = %v", err)
		}
		if err := json.Unmarshal(body, &call.body); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		calls = append(calls, call)
		return shortcutJSONResponse(http.StatusOK, map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{},
		}), nil
	}))
	setChatMembersAddTestFlags(t, runtime, "oc_test", "ou_b,ou_a,ou_b", "cli_b,cli_a,cli_b")

	err := executeChatMembersAdd(runtime, chatMembersAddSpec{
		ChatID: "oc_test",
		Users:  []string{"ou_b", "ou_a"},
		Bots:   []string{"cli_b", "cli_a"},
	})
	if err != nil {
		t.Fatalf("executeChatMembersAdd() error = %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("call count = %d, want 2", len(calls))
	}
	for i, call := range calls {
		if call.method != http.MethodPost {
			t.Errorf("calls[%d].method = %q, want %q", i, call.method, http.MethodPost)
		}
		if call.escapedPath != "/open-apis/im/v1/chats/oc_test/members" {
			t.Errorf("calls[%d].escapedPath = %q", i, call.escapedPath)
		}
		if got := call.succeedType; got != "1" {
			t.Errorf("calls[%d].succeed_type = %q, want %q", i, got, "1")
		}
	}
	if got, want := []string{calls[0].memberIDType, calls[1].memberIDType}, []string{"open_id", "app_id"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("member_id_type order = %#v, want %#v", got, want)
	}
	if got, want := calls[0].body.IDList, []string{"ou_b", "ou_a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("user id_list = %#v, want %#v", got, want)
	}
	if got, want := calls[1].body.IDList, []string{"cli_b", "cli_a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("bot id_list = %#v, want %#v", got, want)
	}
}

func TestExecuteChatMembersAddUsersOnly(t *testing.T) {
	got := executeSingleChatMembersAddRequest(t, "user", chatMembersAddSpec{
		ChatID: "oc_test/slash",
		Users:  []string{"ou_b", "ou_a"},
	})
	assertSingleChatMembersAddRequest(t, got, "open_id", []string{"ou_b", "ou_a"})
}

func TestExecuteChatMembersAddBotsOnly(t *testing.T) {
	got := executeSingleChatMembersAddRequest(t, "bot", chatMembersAddSpec{
		ChatID: "oc_test/slash",
		Bots:   []string{"cli_b", "cli_a"},
	})
	assertSingleChatMembersAddRequest(t, got, "app_id", []string{"cli_b", "cli_a"})
}

type chatMembersAddRequestRecord struct {
	method       string
	escapedPath  string
	memberIDType string
	succeedType  string
	idList       []string
}

func executeSingleChatMembersAddRequest(t *testing.T, identity string, spec chatMembersAddSpec) chatMembersAddRequestRecord {
	t.Helper()

	var requests []chatMembersAddRequestRecord
	transport := shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body struct {
			IDList []string `json:"id_list"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("json.Decode() error = %v", err)
		}
		requests = append(requests, chatMembersAddRequestRecord{
			method:       req.Method,
			escapedPath:  req.URL.EscapedPath(),
			memberIDType: req.URL.Query().Get("member_id_type"),
			succeedType:  req.URL.Query().Get("succeed_type"),
			idList:       body.IDList,
		})
		return shortcutJSONResponse(http.StatusOK, map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{},
		}), nil
	})

	var runtime *common.RuntimeContext
	switch identity {
	case "user":
		runtime = newUserShortcutRuntime(t, transport)
	case "bot":
		runtime = newBotShortcutRuntime(t, transport)
	default:
		t.Fatalf("unsupported test identity %q", identity)
	}
	setChatMembersAddTestFlags(t, runtime, spec.ChatID, strings.Join(spec.Users, ","), strings.Join(spec.Bots, ","))

	if err := executeChatMembersAdd(runtime, spec); err != nil {
		t.Fatalf("executeChatMembersAdd() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(requests))
	}
	return requests[0]
}

func assertSingleChatMembersAddRequest(t *testing.T, got chatMembersAddRequestRecord, wantMemberIDType string, wantIDs []string) {
	t.Helper()

	if got.method != http.MethodPost {
		t.Errorf("method = %q, want %q", got.method, http.MethodPost)
	}
	if got.escapedPath != "/open-apis/im/v1/chats/oc_test%2Fslash/members" {
		t.Errorf("escaped path = %q, want encoded chat ID", got.escapedPath)
	}
	if got.memberIDType != wantMemberIDType {
		t.Errorf("member_id_type = %q, want %q", got.memberIDType, wantMemberIDType)
	}
	if got.succeedType != "1" {
		t.Errorf("succeed_type = %q, want %q", got.succeedType, "1")
	}
	if !reflect.DeepEqual(got.idList, wantIDs) {
		t.Errorf("id_list = %#v, want %#v", got.idList, wantIDs)
	}
}

func TestExecuteChatMembersAddPassesThroughUserAPIError(t *testing.T) {
	requestCount := 0
	runtime := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if got := req.URL.Query().Get("member_id_type"); got != "open_id" {
			t.Fatalf("member_id_type = %q, want first request to use open_id", got)
		}
		return shortcutJSONResponse(http.StatusOK, map[string]interface{}{
			"code": 123456789,
			"msg":  "member request rejected",
			"error": map[string]interface{}{
				"log_id": "log-chat-members-add",
			},
		}), nil
	}))
	setChatMembersAddTestFlags(t, runtime, "oc_test", "ou_a", "cli_a")

	err := executeChatMembersAdd(runtime, chatMembersAddSpec{
		ChatID: "oc_test",
		Users:  []string{"ou_a"},
		Bots:   []string{"cli_a"},
	})
	if err == nil {
		t.Fatal("executeChatMembersAdd() error = nil, want typed API error")
	}
	var apiErr *errs.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("executeChatMembersAdd() error = %T, want *errs.APIError", err)
	}
	problem, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("ProblemOf(%v) returned ok=false", err)
	}
	if problem.Category != errs.CategoryAPI || problem.Subtype != errs.SubtypeUnknown {
		t.Errorf("problem category/subtype = %q/%q, want %q/%q", problem.Category, problem.Subtype, errs.CategoryAPI, errs.SubtypeUnknown)
	}
	if problem.Code != 123456789 {
		t.Errorf("problem code = %d, want %d", problem.Code, 123456789)
	}
	if problem.LogID != "log-chat-members-add" {
		t.Errorf("problem log_id = %q, want %q", problem.LogID, "log-chat-members-add")
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1; bot request must not execute", requestCount)
	}
}

func TestProjectChatMembersAddResponseUsesEmptySlices(t *testing.T) {
	got, err := projectChatMembersAddResponse(map[string]interface{}{}, []string{"ou_a"})
	if err != nil {
		t.Fatalf("projectChatMembersAddResponse() error = %v", err)
	}
	if got.InvalidIDList == nil || got.NotExistedIDList == nil || got.PendingApprovalIDList == nil {
		t.Fatalf("projectChatMembersAddResponse() = %#v, want non-nil empty slices", got)
	}
	if len(got.InvalidIDList) != 0 || len(got.NotExistedIDList) != 0 || len(got.PendingApprovalIDList) != 0 {
		t.Fatalf("projectChatMembersAddResponse() = %#v, want empty slices", got)
	}
}

func TestProjectChatMembersAddResponsePreservesAllLists(t *testing.T) {
	data := map[string]interface{}{
		"invalid_id_list":          []interface{}{"ou_invalid_a", "ou_invalid_b"},
		"not_existed_id_list":      []string{"ou_missing_a", "ou_missing_b"},
		"pending_approval_id_list": []interface{}{"ou_pending_a", "ou_pending_b"},
	}
	requested := []string{
		"ou_invalid_a", "ou_invalid_b",
		"ou_missing_a", "ou_missing_b",
		"ou_pending_a", "ou_pending_b",
	}

	got, err := projectChatMembersAddResponse(data, requested)
	if err != nil {
		t.Fatalf("projectChatMembersAddResponse() error = %v", err)
	}
	want := chatMembersAddResponse{
		InvalidIDList:         []string{"ou_invalid_a", "ou_invalid_b"},
		NotExistedIDList:      []string{"ou_missing_a", "ou_missing_b"},
		PendingApprovalIDList: []string{"ou_pending_a", "ou_pending_b"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projectChatMembersAddResponse() = %#v, want %#v", got, want)
	}
}

func TestExecuteChatMembersAddRejectsInvalidResponseLists(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
	}{
		{
			name: "field is not an array",
			data: map[string]interface{}{"invalid_id_list": "invalid-shape"},
		},
		{
			name: "array element is not a string",
			data: map[string]interface{}{"invalid_id_list": []interface{}{"ou_a", float64(1)}},
		},
		{
			name: "duplicate inside one list",
			data: map[string]interface{}{"invalid_id_list": []interface{}{"ou_a", "ou_a"}},
		},
		{
			name: "duplicate across lists",
			data: map[string]interface{}{
				"invalid_id_list":     []interface{}{"ou_a"},
				"not_existed_id_list": []interface{}{"ou_a"},
			},
		},
		{
			name: "identifier was not requested",
			data: map[string]interface{}{"pending_approval_id_list": []interface{}{"ou_unexpected"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := newUserShortcutRuntime(t, shortcutRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return shortcutJSONResponse(http.StatusOK, map[string]interface{}{
					"code": 0,
					"data": tt.data,
				}), nil
			}))
			setChatMembersAddTestFlags(t, runtime, "oc_test", "ou_a,ou_b", "")

			err := executeChatMembersAdd(runtime, chatMembersAddSpec{
				ChatID: "oc_test",
				Users:  []string{"ou_a", "ou_b"},
			})
			if err == nil {
				t.Fatal("executeChatMembersAdd() error = nil, want invalid response error")
			}
			problem, ok := errs.ProblemOf(err)
			if !ok {
				t.Fatalf("ProblemOf(%v) returned ok=false", err)
			}
			if problem.Category != errs.CategoryInternal || problem.Subtype != errs.SubtypeInvalidResponse {
				t.Fatalf(
					"problem = category %q subtype %q, want category %q subtype %q",
					problem.Category,
					problem.Subtype,
					errs.CategoryInternal,
					errs.SubtypeInvalidResponse,
				)
			}
			if errors.Unwrap(err) == nil {
				t.Fatal("invalid response error has no cause")
			}
			for _, id := range []string{"ou_a", "ou_b", "ou_unexpected"} {
				if strings.Contains(err.Error(), id) {
					t.Fatalf("error message contains member identifier %q", id)
				}
			}
			out, ok := runtime.Factory.IOStreams.Out.(*bytes.Buffer)
			if !ok {
				t.Fatalf("stdout buffer has type %T", runtime.Factory.IOStreams.Out)
			}
			if out.Len() != 0 {
				t.Fatalf("stdout = %q, want no success data", out.String())
			}
		})
	}
}

func TestMergeChatMembersAddResponsesPreservesRequestOrder(t *testing.T) {
	users := chatMembersAddResponse{
		InvalidIDList:         []string{"ou_invalid"},
		NotExistedIDList:      []string{"ou_missing"},
		PendingApprovalIDList: []string{"ou_pending"},
	}
	bots := chatMembersAddResponse{
		InvalidIDList:         []string{"cli_invalid"},
		NotExistedIDList:      []string{"cli_missing"},
		PendingApprovalIDList: []string{"cli_pending"},
	}

	got := mergeChatMembersAddResponse(users, bots)
	want := chatMembersAddResponse{
		InvalidIDList:         []string{"ou_invalid", "cli_invalid"},
		NotExistedIDList:      []string{"ou_missing", "cli_missing"},
		PendingApprovalIDList: []string{"ou_pending", "cli_pending"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeChatMembersAddResponse() = %#v, want %#v", got, want)
	}
}

func TestChatMembersAddResultSuccessCount(t *testing.T) {
	tests := []struct {
		name      string
		requested int
		response  chatMembersAddResponse
		want      int
	}{
		{name: "all confirmed", requested: 4, response: chatMembersAddResponse{}, want: 4},
		{
			name:      "subtracts all unfinished lists",
			requested: 5,
			response: chatMembersAddResponse{
				InvalidIDList:         []string{"invalid"},
				NotExistedIDList:      []string{"missing"},
				PendingApprovalIDList: []string{"pending"},
			},
			want: 2,
		},
		{
			name:      "clamps excessive unfinished count to zero",
			requested: 2,
			response: chatMembersAddResponse{
				InvalidIDList:         []string{"invalid-a", "invalid-b"},
				NotExistedIDList:      []string{"missing"},
				PendingApprovalIDList: []string{"pending"},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := confirmedChatMembersAddCount(tt.requested, tt.response); got != tt.want {
				t.Fatalf("confirmedChatMembersAddCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func newChatMembersAddTestRuntime(t *testing.T, chatID, users, bots string) *common.RuntimeContext {
	t.Helper()
	runtime := &common.RuntimeContext{}
	setChatMembersAddTestFlags(t, runtime, chatID, users, bots)
	return runtime
}

func setChatMembersAddTestFlags(t *testing.T, runtime *common.RuntimeContext, chatID, users, bots string) {
	t.Helper()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("chat-id", "", "")
	cmd.Flags().String("users", "", "")
	cmd.Flags().String("bots", "", "")
	for name, value := range map[string]string{
		"chat-id": chatID,
		"users":   users,
		"bots":    bots,
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	runtime.Cmd = cmd
}

func makeChatMembersAddIDs(prefix string, count int) []string {
	ids := make([]string, count)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s%d", prefix, i)
	}
	return ids
}

func assertChatMembersAddValidationError(t *testing.T, err error, wantParam string, wantParams []string, wantMessage string) {
	t.Helper()

	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %T (%v), want *errs.ValidationError", err, err)
	}
	problem, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("ProblemOf(%v) returned ok=false", err)
	}
	if problem.Category != errs.CategoryValidation || problem.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf(
			"problem = category %q subtype %q, want category %q subtype %q",
			problem.Category,
			problem.Subtype,
			errs.CategoryValidation,
			errs.SubtypeInvalidArgument,
		)
	}
	if validationErr.Param != wantParam {
		t.Fatalf("Param = %q, want %q", validationErr.Param, wantParam)
	}
	var gotParams []string
	for _, param := range validationErr.Params {
		gotParams = append(gotParams, param.Name)
	}
	if !reflect.DeepEqual(gotParams, wantParams) {
		t.Fatalf("Params = %#v, want %#v", gotParams, wantParams)
	}
	if wantMessage != "" && validationErr.Error() != wantMessage {
		t.Fatalf("error message = %q, want %q", validationErr.Error(), wantMessage)
	}
}

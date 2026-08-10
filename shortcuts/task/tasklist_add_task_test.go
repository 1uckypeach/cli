// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package task

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/recovery"
	"github.com/larksuite/cli/internal/surface"
)

func TestAddTaskToTasklist_UserMissingScopeProjectsInlineHint(t *testing.T) {
	tests := []struct {
		name string
		plan *surface.Plan
	}{
		{name: "visible"},
		{
			name: "concealed",
			plan: surface.NewPlan(map[surface.CommandID]surface.CommandState{
				surface.CommandAuthLogin: surface.CommandConcealed,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, stdout, _, reg := taskShortcutTestFactory(t)
			f.Recovery = recovery.NewProjector(func() *surface.Plan { return tt.plan })
			reg.Register(&httpmock.Stub{
				Method: "POST",
				URL:    "/open-apis/task/v2/tasks/task-scope/add_tasklist",
				Body: map[string]interface{}{
					"code": 99991679,
					"msg":  "missing scope",
					"error": map[string]interface{}{
						"permission_violations": []interface{}{
							map[string]interface{}{"subject": "task:task:write"},
						},
					},
				},
			})

			s := AddTaskToTasklist
			s.AuthTypes = []string{"bot", "user"}
			err := runMountedTaskShortcut(t, s, []string{
				"+tasklist-task-add", "--tasklist-id", "tl-123", "--task-id", "task-scope",
				"--as", "user", "--format", "json",
			}, f, stdout)
			var partial *output.PartialFailureError
			if !errors.As(err, &partial) {
				t.Fatalf("err = %T, want *output.PartialFailureError: %v", err, err)
			}

			var envelope struct {
				OK   bool `json:"ok"`
				Data struct {
					Failed []map[string]interface{} `json:"failed_tasks"`
				} `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("unmarshal stdout: %v\n%s", err, stdout.String())
			}
			if envelope.OK || len(envelope.Data.Failed) != 1 {
				t.Fatalf("envelope = %#v, want ok:false with one failure", envelope)
			}
			failed := envelope.Data.Failed[0]
			if got, want := failed["type"], string(errs.SubtypeMissingScope); got != want {
				t.Errorf("failed type = %v, want %v", got, want)
			}
			if got, want := failed["hint"], recovery.UserAuthorization("task:task:write").Render(tt.plan); got != want {
				t.Errorf("failed hint = %q, want %q", got, want)
			}
			if tt.plan != nil && strings.Contains(failed["hint"].(string), "auth login") {
				t.Errorf("concealed hint leaked auth command: %q", failed["hint"])
			}
		})
	}
}

func TestAddTaskToTasklist_DryRunPreviewsOnlyFirstTask(t *testing.T) {
	f, stdout, _, _ := taskShortcutTestFactory(t)
	args := []string{
		"+tasklist-task-add",
		"--tasklist-id", "https://applink.feishu.cn/client/todo/task_list?guid=tl-from-url&extra=ignored",
		"--task-id", " first/task? , second-task ",
		"--section-guid", " sec-456 ",
		"--dry-run",
		"--as", "bot",
	}
	if err := runMountedTaskShortcut(t, AddTaskToTasklist, args, f, stdout); err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	var envelope struct {
		Data struct {
			API []struct {
				Method string         `json:"method"`
				URL    string         `json:"url"`
				Params map[string]any `json:"params"`
				Body   map[string]any `json:"body"`
			} `json:"api"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if len(envelope.Data.API) != 1 {
		t.Fatalf("api calls = %d, want one-call legacy preview", len(envelope.Data.API))
	}
	call := envelope.Data.API[0]
	if call.Method != http.MethodPost || call.URL != "/open-apis/task/v2/tasks/first%2Ftask%3F/add_tasklist" {
		t.Fatalf("call = %s %s, want first task only", call.Method, call.URL)
	}
	if call.Params["user_id_type"] != "open_id" {
		t.Fatalf("params = %#v, want user_id_type=open_id", call.Params)
	}
	wantBody := map[string]any{"tasklist_guid": "tl-from-url", "section_guid": "sec-456"}
	if !taskJSONEqual(call.Body, wantBody) {
		t.Fatalf("body = %#v, want %#v", call.Body, wantBody)
	}
}

func TestAddTaskToTasklist_Success(t *testing.T) {
	f, stdout, _, reg := taskShortcutTestFactory(t)
	warmTenantToken(t, f, reg)

	reg.Register(&httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/task/v2/tasks/task-1/add_tasklist",
		Body: map[string]interface{}{
			"code": 0, "msg": "success",
			"data": map[string]interface{}{
				"task": map[string]interface{}{
					"guid": "task-1",
				},
			},
		},
	})

	s := AddTaskToTasklist
	s.AuthTypes = []string{"bot", "user"}

	args := []string{"+tasklist-task-add", "--tasklist-id", "tl-123", "--task-id", "task-1", "--section-guid", "sec-456", "--as", "bot", "--format", "json"}
	err := runMountedTaskShortcut(t, s, args, f, stdout)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, `"tasklist_guid":"tl-123"`) && !strings.Contains(out, `"tasklist_guid": "tl-123"`) {
		t.Errorf("expected tasklist_guid in output, got: %s", out)
	}
}

// TestAddTaskToTasklist_PartialFailure exercises the batch path: some tasks
// succeed, others fail with typed API errors. Successful and failed tasks both
// land in stdout as an ok:false envelope, and the command returns the typed
// partial-failure exit signal (exit 1) via runtime.OutPartialFailure. The
// failed_tasks[].type carries the typed subtype (e.g. "permission_denied",
// "not_found") read off errs.ProblemOf.
func TestAddTaskToTasklist_SuccessFanOutPreservesOrderAndOutput(t *testing.T) {
	f, stdout, _, reg := taskShortcutTestFactory(t)
	warmTenantToken(t, f, reg)

	first := &httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/task/v2/tasks/task-one/add_tasklist",
		Body: map[string]any{
			"code": 0,
			"data": map[string]any{"task": map[string]any{
				"guid": "task-one",
				"url":  "https://applink.feishu.cn/client/todo/task?guid=task-one&from=server",
			}},
		},
	}
	second := &httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/task/v2/tasks/task-two/add_tasklist",
		Body: map[string]any{
			"code": 0,
			"data": map[string]any{"task": map[string]any{
				"guid": "task-two",
				"url":  "https://applink.feishu.cn/client/todo/task?guid=task-two",
			}},
		},
	}
	reg.Register(first)
	reg.Register(second)

	args := []string{
		"+tasklist-task-add",
		"--tasklist-id", "tl-123",
		"--task-id", " task-one, ,task-two ",
		"--section-guid", " sec-456 ",
		"--as", "bot",
	}
	if err := runMountedTaskShortcut(t, AddTaskToTasklist, args, f, stdout); err != nil {
		t.Fatalf("execute: %v", err)
	}

	for i, stub := range []*httpmock.Stub{first, second} {
		var body map[string]any
		if err := json.Unmarshal(stub.CapturedBody, &body); err != nil {
			t.Fatalf("decode body %d: %v", i, err)
		}
		want := map[string]any{"tasklist_guid": "tl-123", "section_guid": "sec-456"}
		if !taskJSONEqual(body, want) {
			t.Fatalf("body %d = %#v, want %#v", i, body, want)
		}
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			TasklistGUID string `json:"tasklist_guid"`
			Successful   []struct {
				GUID string `json:"guid"`
				URL  string `json:"url"`
			} `json:"successful_tasks"`
			Failed []any `json:"failed_tasks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !envelope.OK || envelope.Data.TasklistGUID != "tl-123" || len(envelope.Data.Successful) != 2 || len(envelope.Data.Failed) != 0 {
		t.Fatalf("envelope = %#v", envelope)
	}
	if envelope.Data.Successful[0].GUID != "task-one" || envelope.Data.Successful[1].GUID != "task-two" {
		t.Fatalf("successful order = %#v", envelope.Data.Successful)
	}
	if envelope.Data.Successful[0].URL != "https://applink.feishu.cn/client/todo/task?guid=task-one" {
		t.Fatalf("truncated URL = %q", envelope.Data.Successful[0].URL)
	}
}

func TestAddTaskToTasklist_AllBlankItemsReturnLegacyZeroItemSuccess(t *testing.T) {
	f, stdout, _, reg := taskShortcutTestFactory(t)
	warmTenantToken(t, f, reg)

	err := runMountedTaskShortcut(t, AddTaskToTasklist, []string{
		"+tasklist-task-add", "--tasklist-id", "tl-123", "--task-id", " , , ", "--as", "bot",
	}, f, stdout)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode stdout: %v", err)
	}
	if envelope["ok"] != true {
		t.Fatalf("ok = %#v, want true", envelope["ok"])
	}
	data, _ := envelope["data"].(map[string]any)
	if data["successful_tasks"] != nil || data["failed_tasks"] != nil {
		t.Fatalf("zero-item legacy slices = %#v/%#v, want null/null", data["successful_tasks"], data["failed_tasks"])
	}
}

func TestAddTaskToTasklist_PartialFailure(t *testing.T) {
	f, stdout, _, reg := taskShortcutTestFactory(t)
	warmTenantToken(t, f, reg)

	reg.Register(&httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/task/v2/tasks/task-ok/add_tasklist",
		Body: map[string]interface{}{
			"code": 0, "msg": "success",
			"data": map[string]interface{}{
				"task": map[string]interface{}{
					"guid": "task-ok",
				},
			},
		},
	})
	reg.Register(&httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/task/v2/tasks/task-perm/add_tasklist",
		Body: map[string]interface{}{
			"code": ErrCodeTaskPermissionDenied, "msg": "no permission",
		},
	})
	reg.Register(&httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/task/v2/tasks/task-missing/add_tasklist",
		Body: map[string]interface{}{
			"code": ErrCodeTaskNotFound, "msg": "task not found",
		},
	})

	s := AddTaskToTasklist
	s.AuthTypes = []string{"bot", "user"}

	args := []string{"+tasklist-task-add", "--tasklist-id", "tl-123", "--task-id", "task-ok,task-perm,task-missing", "--as", "bot", "--format", "json"}
	err := runMountedTaskShortcut(t, s, args, f, stdout)
	// Partial failure now surfaces as a non-zero exit (ok:false), not nil.
	var pfErr *output.PartialFailureError
	if !errors.As(err, &pfErr) {
		t.Fatalf("expected *output.PartialFailureError on partial failure, got %T: %v", err, err)
	}
	if pfErr.Code != output.ExitAPI {
		t.Errorf("exit code = %d, want %d (ExitAPI)", pfErr.Code, output.ExitAPI)
	}

	out := stdout.String()

	// Successful task is in stdout.
	if !strings.Contains(out, "task-ok") {
		t.Errorf("expected successful task-ok in output, got: %s", out)
	}

	// Failed tasks carry the typed subtype, not the legacy Detail.Type.
	if !strings.Contains(out, string(errs.SubtypePermissionDenied)) {
		t.Errorf("expected typed subtype %q in failed_tasks, got: %s", errs.SubtypePermissionDenied, out)
	}
	if !strings.Contains(out, string(errs.SubtypeNotFound)) {
		t.Errorf("expected typed subtype %q in failed_tasks, got: %s", errs.SubtypeNotFound, out)
	}

	// The legacy shapes must not leak.
	if strings.Contains(out, "permission_error") {
		t.Errorf("legacy type \"permission_error\" leaked into output: %s", out)
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Successful []map[string]any `json:"successful_tasks"`
			Failed     []map[string]any `json:"failed_tasks"`
		} `json:"data"`
	}
	if decodeErr := json.Unmarshal(stdout.Bytes(), &envelope); decodeErr != nil {
		t.Fatalf("decode partial stdout: %v", decodeErr)
	}
	if envelope.OK || len(envelope.Data.Successful) != 1 || len(envelope.Data.Failed) != 2 {
		t.Fatalf("partial envelope = %#v", envelope)
	}
	if envelope.Data.Failed[0]["guid"] != "task-perm" || envelope.Data.Failed[0]["type"] != string(errs.SubtypePermissionDenied) {
		t.Fatalf("first failed item = %#v", envelope.Data.Failed[0])
	}
	if envelope.Data.Failed[1]["guid"] != "task-missing" || envelope.Data.Failed[1]["type"] != string(errs.SubtypeNotFound) {
		t.Fatalf("second failed item = %#v", envelope.Data.Failed[1])
	}
}

func taskJSONEqual(a, b any) bool {
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(aa) == string(bb)
}

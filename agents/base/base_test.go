// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package base

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	iagents "github.com/larksuite/cli/internal/agents"
	"github.com/larksuite/cli/internal/agents/agenttest"
)

func init() { iagents.Register(Provider()) }

type apiCall struct {
	method string
	path   string
	query  map[string]string
	body   any
}

type fakeRuntime struct {
	agentID   string
	bot       bool
	params    map[string]string
	responses []json.RawMessage
	errs      []error
	calls     []apiCall
}

func (f *fakeRuntime) AgentID() string           { return f.agentID }
func (f *fakeRuntime) IsBot() bool               { return f.bot }
func (f *fakeRuntime) Params() map[string]string { return f.params }
func (f *fakeRuntime) CallMultipart(context.Context, string, string, map[string]string, []iagents.FilePart) (json.RawMessage, error) {
	panic("base provider must not upload files")
}
func (f *fakeRuntime) CallAPI(_ context.Context, method, path string, query map[string]string, body any) (json.RawMessage, error) {
	q := make(map[string]string, len(query))
	for k, v := range query {
		q[k] = v
	}
	f.calls = append(f.calls, apiCall{method: method, path: path, query: q, body: body})
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(f.responses) == 0 {
		return nil, nil
	}
	raw := f.responses[0]
	f.responses = f.responses[1:]
	return raw, nil
}

func payloadResponse(t *testing.T, payload string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func bodyMap(t *testing.T, body any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func problem(t *testing.T, err error, category errs.Category, subtype errs.Subtype) {
	t.Helper()
	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("want typed error, got %T: %v", err, err)
	}
	if p.Category != category || p.Subtype != subtype {
		t.Fatalf("problem=(%s,%s), want (%s,%s)", p.Category, p.Subtype, category, subtype)
	}
}

func TestProviderConformance(t *testing.T) {
	agenttest.RunConformance(t, "base", "assistant")
	p := Provider()
	if !reflect.DeepEqual(p.RequiredScopes, []string{baseAgentScope}) {
		t.Fatalf("RequiredScopes=%v", p.RequiredScopes)
	}
	if len(p.Catalog) != 1 || p.Catalog[0].ID != "assistant" {
		t.Fatalf("catalog=%+v", p.Catalog)
	}
	caps := iagents.DeriveCapabilities(&p.Catalog[0])
	if !caps.TaskGet || !caps.TaskList || !caps.TaskCancel || !caps.ContextList || !caps.ContextGet || !caps.ContextDelete {
		t.Fatalf("missing required capability: %+v", caps)
	}
	if caps.FileInput || caps.InputRequired || caps.ArtifactDownload {
		t.Fatalf("unsupported capability advertised: %+v", caps)
	}
	agenttest.CheckParamsBinding[sendParams](t, &p.Catalog[0], iagents.VerbSend)
	agenttest.CheckParamsBinding[listTasksParams](t, &p.Catalog[0], iagents.VerbTaskList)
	agenttest.CheckParamsBinding[listContextsParams](t, &p.Catalog[0], iagents.VerbContextList)
}

func TestSendBuildsAdapterRequest(t *testing.T) {
	rt := &fakeRuntime{
		agentID:   "assistant",
		params:    map[string]string{"base_token": "basc/token", "active_table_id": "tbl1"},
		responses: []json.RawMessage{payloadResponse(t, `{"task_id":"task-1","context_id":"ctx-1"}`)},
	}
	task, err := assistantSpec.Send.Handler(context.Background(), rt, iagents.SendInput{Text: "build a dashboard"})
	if err != nil {
		t.Fatal(err)
	}
	if task.TaskID != "task-1" || task.ContextID != "ctx-1" || task.State != iagents.StateSubmitted || task.IsTerminal {
		t.Fatalf("task=%+v", task)
	}
	if len(rt.calls) != 1 {
		t.Fatalf("calls=%d", len(rt.calls))
	}
	call := rt.calls[0]
	if call.method != "POST" || call.path != "/bases/basc%2Ftoken/ai/agents/assistant/messages" {
		t.Fatalf("call=%s %s", call.method, call.path)
	}
	body := bodyMap(t, call.body)
	if body["context_id"] != nil || body["task_id"] != nil {
		t.Fatalf("fresh send must omit context/task: %#v", body)
	}
	if body["idempotency_key"] == "" || body["idempotency_key"] == nil {
		t.Fatalf("missing idempotency key: %#v", body)
	}
	params := body["params"].(map[string]any)
	if params["active_table_id"] != "tbl1" {
		t.Fatalf("params=%#v", params)
	}
	metadata := body["metadata"].(map[string]any)
	if metadata["channel"] != "lark_cli" {
		t.Fatalf("metadata=%#v", metadata)
	}
	encoded, _ := json.Marshal(body)
	for _, forbidden := range []string{"preJobId", "skill_id", "action_code", "scene_code", "sidebar_tools", "memberId", "UserID", "TenantID", "AppID"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("request leaked forbidden field %q: %s", forbidden, encoded)
		}
	}
}

func TestSendContinuationAndIdempotency(t *testing.T) {
	rt := &fakeRuntime{
		agentID: "assistant",
		params:  map[string]string{"base_token": "b1"},
		responses: []json.RawMessage{
			payloadResponse(t, `{"task_id":"t1","context_id":"c1","state":"running"}`),
			payloadResponse(t, `{"task_id":"t1","context_id":"c1","state":"running"}`),
		},
	}
	in := iagents.SendInput{Text: "more", ContextID: "c1", TaskID: "t1"}
	if _, err := assistantSpec.Send.Handler(context.Background(), rt, in); err != nil {
		t.Fatal(err)
	}
	if _, err := assistantSpec.Send.Handler(context.Background(), rt, in); err != nil {
		t.Fatal(err)
	}
	b1, b2 := bodyMap(t, rt.calls[0].body), bodyMap(t, rt.calls[1].body)
	if b1["context_id"] != "c1" || b1["task_id"] != "t1" {
		t.Fatalf("continuation body=%#v", b1)
	}
	if b1["idempotency_key"] == b2["idempotency_key"] {
		t.Fatalf("logical sends reused key %q", b1["idempotency_key"])
	}
}

func TestSendRejectsDisabledInputs(t *testing.T) {
	tests := []iagents.SendInput{
		{Text: "x", Files: []string{"a.txt"}},
		{Text: "x", DecisionID: "d1"},
		{Text: "x", OptionIDs: []string{"o1"}},
	}
	for _, in := range tests {
		rt := &fakeRuntime{params: map[string]string{"base_token": "b1"}}
		_, err := assistantSpec.Send.Handler(context.Background(), rt, in)
		problem(t, err, errs.CategoryValidation, errs.SubtypeInvalidArgument)
		if len(rt.calls) != 0 {
			t.Fatalf("disabled input made API call: %+v", in)
		}
	}
}

func TestSendRejectsBotIdentity(t *testing.T) {
	rt := &fakeRuntime{bot: true, params: map[string]string{"base_token": "b1"}}
	_, err := assistantSpec.Send.Handler(context.Background(), rt, iagents.SendInput{Text: "x"})
	problem(t, err, errs.CategoryValidation, errs.SubtypeInvalidArgument)
	if len(rt.calls) != 0 {
		t.Fatal("bot identity must be rejected before an API call")
	}
}

func TestTaskHooksAndMapping(t *testing.T) {
	rt := &fakeRuntime{
		params: map[string]string{"base_token": "b1", "cursor": "next", "limit": "20", "state": "running"},
		responses: []json.RawMessage{
			payloadResponse(t, `{"task_id":"t/1","context_id":"c1","state":"running","created_at":1710000000,"updated_at":1710000060,"messages":[{"role":"agent","parts":[{"type":"data","text":"{\"operation_type\":\"text\",\"content\":\"ready\"}"}]}]}`),
			payloadResponse(t, `[{"task_id":"t1","context_id":"c1","state":"done","updated_at":1710000060,"summary":"done"}]`),
		},
	}
	task, err := assistantSpec.GetTask.Handler(context.Background(), rt, "t/1")
	if err != nil {
		t.Fatal(err)
	}
	if task.State != iagents.StateWorking || task.IsTerminal || task.Messages[0].Parts[0].Text != "ready" {
		t.Fatalf("task=%+v", task)
	}
	if rt.calls[0].path != "/bases/b1/ai/agents/assistant/tasks/t%2F1" {
		t.Fatalf("path=%s", rt.calls[0].path)
	}
	list, err := assistantSpec.ListTasks.Handler(context.Background(), rt, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].State != iagents.StateCompleted || !list[0].IsTerminal {
		t.Fatalf("list=%+v", list)
	}
	wantQuery := map[string]string{"context_id": "c1", "cursor": "next", "limit": "20", "state": "running"}
	if !reflect.DeepEqual(rt.calls[1].query, wantQuery) {
		t.Fatalf("query=%v want %v", rt.calls[1].query, wantQuery)
	}
}

func TestUnknownStateAndInvalidPayloadAreTyped(t *testing.T) {
	for _, payload := range []string{`{"task_id":"t1","state":"paused"}`, `{not-json`} {
		rt := &fakeRuntime{params: map[string]string{"base_token": "b1"}, responses: []json.RawMessage{payloadResponse(t, payload)}}
		_, err := assistantSpec.GetTask.Handler(context.Background(), rt, "t1")
		problem(t, err, errs.CategoryInternal, errs.SubtypeInvalidResponse)
	}
}

func TestEmbeddedCliMessageMapping(t *testing.T) {
	t.Run("unknown operation stays data", func(t *testing.T) {
		part, err := mapPart(adapterPart{Type: "data", Text: `{"operation_type":"run_shell","content":"rm -rf /"}`})
		if err != nil {
			t.Fatal(err)
		}
		if part.Type != "data" || part.Data == nil || part.Text != "" {
			t.Fatalf("part=%+v", part)
		}
	})
	t.Run("invalid embedded json is typed", func(t *testing.T) {
		_, err := mapPart(adapterPart{Type: "data", Text: `{bad`})
		problem(t, err, errs.CategoryInternal, errs.SubtypeInvalidResponse)
	})
}

func TestContextHooks(t *testing.T) {
	rt := &fakeRuntime{
		params: map[string]string{"base_token": "b1", "cursor": "next", "limit": "10", "status": "active"},
		responses: []json.RawMessage{
			payloadResponse(t, `[{"context_id":"c1","title":"Quarterly plan","created_at":1710000000,"updated_at":1710000060}]`),
			payloadResponse(t, `{"context_id":"c1","title":"Quarterly plan","tasks":[{"task_id":"new","state":"running","updated_at":1710000060},{"task_id":"old","state":"done","updated_at":1710000000}]}`),
			payloadResponse(t, `{"result":true}`),
		},
	}
	contexts, err := assistantSpec.ListContexts.Handler(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(contexts) != 1 || contexts[0].TaskCount != 0 {
		t.Fatalf("contexts=%+v", contexts)
	}
	wantQuery := map[string]string{"cursor": "next", "limit": "10", "status": "active"}
	if !reflect.DeepEqual(rt.calls[0].query, wantQuery) {
		t.Fatalf("query=%v", rt.calls[0].query)
	}
	detail, err := assistantSpec.GetContext.Handler(context.Background(), rt, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.TaskCount != 2 || detail.ActiveTask == nil || detail.ActiveTask.TaskID != "new" {
		t.Fatalf("detail=%+v", detail)
	}
	if err := assistantSpec.DeleteContext.Handler(context.Background(), rt, "c1"); err != nil {
		t.Fatal(err)
	}
	if rt.calls[2].method != "DELETE" {
		t.Fatalf("delete call=%+v", rt.calls[2])
	}
}

func TestResultFalseUsesTypedCategory(t *testing.T) {
	rt := &fakeRuntime{
		params:    map[string]string{"base_token": "b1"},
		responses: []json.RawMessage{payloadResponse(t, `{"result":false,"reason":"task is terminal","error":{"category":"task_terminal"}}`)},
	}
	err := assistantSpec.CancelTask.Handler(context.Background(), rt, "t1")
	problem(t, err, errs.CategoryValidation, errs.SubtypeFailedPrecondition)
}

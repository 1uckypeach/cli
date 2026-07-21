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

// dataResponse mirrors Runtime.CallAPI: the OpenAPI envelope has already been
// checked and its data field is returned as raw JSON. Base's api.map="data"
// response is an object/array here, not a JSON-encoded string.
func dataResponse(t *testing.T, data string) json.RawMessage {
	t.Helper()
	return json.RawMessage(data)
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
	wantScopes := []string{"base:agent:execute"}
	if !reflect.DeepEqual(p.RequiredScopes, wantScopes) {
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
	agenttest.CheckParamsBinding[getTaskParams](t, &p.Catalog[0], iagents.VerbTaskGet)
	agenttest.CheckParamsBinding[listTasksParams](t, &p.Catalog[0], iagents.VerbTaskList)
	agenttest.CheckParamsBinding[listContextsParams](t, &p.Catalog[0], iagents.VerbContextList)
}

func TestAgentRootUsesPublicBaseV3Prefix(t *testing.T) {
	got := agentRoot("basc/token")
	want := "/open-apis/base/v3/bases/basc%2Ftoken/ai/agents/assistant"
	if got != want {
		t.Fatalf("agentRoot()=%q want %q", got, want)
	}
}

func TestSendBuildsAdapterRequest(t *testing.T) {
	rt := &fakeRuntime{
		agentID:   "assistant",
		params:    map[string]string{"base_token": "basc/token", "active_table_id": "tbl1"},
		responses: []json.RawMessage{dataResponse(t, `{"schema_version":1,"task_id":"task-1","context_id":"ctx-1","status":"pending","outputs":[]}`)},
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
	if call.method != "POST" || call.path != "/open-apis/base/v3/bases/basc%2Ftoken/ai/agents/assistant/messages" {
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

func TestSendRejectsJSONStringData(t *testing.T) {
	encoded, err := json.Marshal(`{"schema_version":1,"task_id":"task-1","context_id":"ctx-1","status":"pending","outputs":[]}`)
	if err != nil {
		t.Fatal(err)
	}
	rt := &fakeRuntime{
		agentID:   "assistant",
		params:    map[string]string{"base_token": "b1"},
		responses: []json.RawMessage{encoded},
	}
	_, err = assistantSpec.Send.Handler(context.Background(), rt, iagents.SendInput{Text: "hello"})
	problem(t, err, errs.CategoryInternal, errs.SubtypeInvalidResponse)
}

func TestSendContinuationAndIdempotency(t *testing.T) {
	rt := &fakeRuntime{
		agentID: "assistant",
		params:  map[string]string{"base_token": "b1"},
		responses: []json.RawMessage{
			dataResponse(t, `{"schema_version":1,"task_id":"t1","context_id":"c1","status":"running","outputs":[]}`),
			dataResponse(t, `{"schema_version":1,"task_id":"t1","context_id":"c1","status":"running","outputs":[]}`),
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

func TestGetTaskForwardsContextIDAcrossPolls(t *testing.T) {
	rt := &fakeRuntime{
		params: map[string]string{"base_token": "b1", "context_id": "c1"},
		responses: []json.RawMessage{
			dataResponse(t, `{"schema_version":1,"task_id":"t1","context_id":"c1","status":"running","outputs":[]}`),
			dataResponse(t, `{"schema_version":1,"task_id":"t1","context_id":"c1","status":"completed","outputs":[]}`),
		},
	}

	for range 2 {
		if _, err := assistantSpec.GetTask.Handler(context.Background(), rt, "t1"); err != nil {
			t.Fatal(err)
		}
	}
	if len(rt.calls) != 2 {
		t.Fatalf("calls=%d", len(rt.calls))
	}
	wantQuery := map[string]string{"context_id": "c1"}
	for i, call := range rt.calls {
		if !reflect.DeepEqual(call.query, wantQuery) {
			t.Fatalf("calls[%d].query=%v want %v", i, call.query, wantQuery)
		}
	}
}

func TestTaskHooksAndMapping(t *testing.T) {
	rt := &fakeRuntime{
		params: map[string]string{"base_token": "b1", "state": "running"},
		responses: []json.RawMessage{
			dataResponse(t, `{"schema_version":1,"task_id":"t/1","context_id":"c1","status":"running","outputs":[{"id":"1:text:1","type":"text","text":"ready"}]}`),
			dataResponse(t, `[{"task_id":"t1","context_id":"c1","state":"done","updated_at":1710000060,"summary":"done"}]`),
		},
	}
	task, err := assistantSpec.GetTask.Handler(context.Background(), rt, "t/1")
	if err != nil {
		t.Fatal(err)
	}
	if task.State != iagents.StateWorking || task.IsTerminal || task.Messages[0].Parts[0].Text != "ready" {
		t.Fatalf("task=%+v", task)
	}
	if rt.calls[0].path != "/open-apis/base/v3/bases/b1/ai/agents/assistant/tasks/t%2F1" {
		t.Fatalf("path=%s", rt.calls[0].path)
	}
	list, pageInfo, err := assistantSpec.ListTasks.Handler(context.Background(), rt, "c1", iagents.PageParams{Token: "next", Size: 20})
	if err != nil {
		t.Fatal(err)
	}
	if pageInfo != (iagents.PageInfo{}) {
		t.Fatalf("pageInfo=%+v", pageInfo)
	}
	if len(list) != 1 || list[0].State != iagents.StateCompleted || !list[0].IsTerminal {
		t.Fatalf("list=%+v", list)
	}
	wantQuery := map[string]string{"context_id": "c1", "cursor": "next", "limit": "20", "state": "running"}
	if !reflect.DeepEqual(rt.calls[1].query, wantQuery) {
		t.Fatalf("query=%v want %v", rt.calls[1].query, wantQuery)
	}
}

func TestListHooksMapPaginationEnvelope(t *testing.T) {
	rt := &fakeRuntime{
		params: map[string]string{"base_token": "b1", "state": "done", "status": "active"},
		responses: []json.RawMessage{
			dataResponse(t, `{"tasks":[{"task_id":"t1","context_id":"c1","state":"done","updated_at":1710000060}],"has_more":true,"next_cursor":"task-next"}`),
			dataResponse(t, `{"contexts":[{"context_id":"c1","title":"Quarterly plan","created_at":1710000000,"updated_at":1710000060}],"has_more":true,"next_cursor":"context-next"}`),
		},
	}

	tasks, taskPage, err := assistantSpec.ListTasks.Handler(context.Background(), rt, "c1", iagents.PageParams{Size: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || taskPage != (iagents.PageInfo{HasMore: true, NextToken: "task-next"}) {
		t.Fatalf("tasks=%+v page=%+v", tasks, taskPage)
	}

	contexts, contextPage, err := assistantSpec.ListContexts.Handler(context.Background(), rt, iagents.PageParams{Size: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(contexts) != 1 || contextPage != (iagents.PageInfo{HasMore: true, NextToken: "context-next"}) {
		t.Fatalf("contexts=%+v page=%+v", contexts, contextPage)
	}
}

func TestListHooksRejectMalformedPaginationEnvelope(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		contexts bool
	}{
		{name: "null task response", payload: `null`},
		{name: "missing tasks", payload: `{"has_more":false}`},
		{name: "missing task has_more", payload: `{"tasks":[]}`},
		{name: "task cursor missing", payload: `{"tasks":[],"has_more":true}`},
		{name: "unexpected task cursor", payload: `{"tasks":[],"has_more":false,"next_cursor":"next"}`},
		{name: "null contexts", payload: `{"contexts":null,"has_more":false}`, contexts: true},
		{name: "missing contexts", payload: `{"has_more":false}`, contexts: true},
		{name: "missing context has_more", payload: `{"contexts":[]}`, contexts: true},
		{name: "context cursor missing", payload: `{"contexts":[],"has_more":true}`, contexts: true},
		{name: "null context cursor", payload: `{"contexts":[],"has_more":false,"next_cursor":null}`, contexts: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := &fakeRuntime{
				params:    map[string]string{"base_token": "b1"},
				responses: []json.RawMessage{dataResponse(t, test.payload)},
			}
			var err error
			if test.contexts {
				_, _, err = assistantSpec.ListContexts.Handler(context.Background(), rt, iagents.PageParams{})
			} else {
				_, _, err = assistantSpec.ListTasks.Handler(context.Background(), rt, "c1", iagents.PageParams{})
			}
			problem(t, err, errs.CategoryInternal, errs.SubtypeInvalidResponse)
		})
	}
}

func TestUnknownStateAndInvalidPayloadAreTyped(t *testing.T) {
	for _, payload := range []string{`{"schema_version":1,"task_id":"t1","status":"paused","outputs":[]}`, `{not-json`} {
		rt := &fakeRuntime{params: map[string]string{"base_token": "b1"}, responses: []json.RawMessage{dataResponse(t, payload)}}
		_, err := assistantSpec.GetTask.Handler(context.Background(), rt, "t1")
		problem(t, err, errs.CategoryInternal, errs.SubtypeInvalidResponse)
	}
}

func TestVersionedTaskMapsOutputsAndClarification(t *testing.T) {
	rt := &fakeRuntime{
		params: map[string]string{"base_token": "b1"},
		responses: []json.RawMessage{dataResponse(t, `{
  "schema_version": 1,
  "task_id": "t1",
  "context_id": "c1",
  "status": "waiting_for_input",
  "outputs": [
    {"id":"100:text:1","type":"text","source":"base_agent","group_id":"grp_1","text":"先给出结论"},
    {"id":"101:data_qa_chart:1","type":"data","source":"base_agent","group_id":"grp_1","data":{"kind":"qa_chart","schema_version":1,"payload":{"chartId":"chart_1","baseId":9007199254740993,"vchartSpec":{"type":"bar"}}}},
    {"id":"102:question:1","type":"clarification","source":"base_agent","group_id":"grp_1","clarification":{
      "id":"clarify_1","title":"请补充信息","required":true,"submitted":false,
      "questions":[{"id":"q_done","type":"text","prompt":"已回答","required":true,"answered":true,"answer":{"value":"ok"}}],
      "forms":[{"id":"form_1","title":"高级设置","questions":[{"id":"q_scene","type":"single_select","prompt":"请选择场景","required":true,"options":[{"id":"opt_1","label":"新建","description":"创建新表"}]}],"buttons":[{"id":"btn_skip","kind":"custom","label":"跳过","action_params":"{\"action\":\"skip\"}"}]}],
      "buttons":[{"id":"btn_submit","kind":"custom","style":"primary","label":"确认","action_params":"{\"action\":\"submit\"}"}]
    }},
    {"id":"103:artifact:1","type":"artifact","source":"table_agent","group_id":"grp_2","artifact":{"id":"artifact_1","type":"table","title":"销售表","status":"ready","resource":{"block_id":"block_1","view_id":"view_1"},"revision":128,"metadata":{"base_id":9007199254740993,"init_type":2}}}
  ]
}`)},
	}

	task, err := assistantSpec.GetTask.Handler(context.Background(), rt, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if task.State != iagents.StateInputRequired || task.IsTerminal {
		t.Fatalf("state=%s terminal=%t", task.State, task.IsTerminal)
	}
	if len(task.Messages) != 1 || len(task.Messages[0].Parts) != 2 {
		t.Fatalf("messages=%+v", task.Messages)
	}
	if got := task.Messages[0].Parts[0]; got.Type != "text" || got.Text != "先给出结论" ||
		got.OutputID != "100:text:1" || got.Source != "base_agent" || got.GroupID != "grp_1" {
		t.Fatalf("text part=%+v", got)
	}
	data, ok := task.Messages[0].Parts[1].Data.(map[string]interface{})
	if !ok || data["kind"] != "qa_chart" {
		t.Fatalf("data part=%+v", task.Messages[0].Parts[1])
	}
	payload, ok := data["payload"].(json.RawMessage)
	if !ok || !strings.Contains(string(payload), `"chartId":"chart_1"`) ||
		!strings.Contains(string(payload), `"baseId":9007199254740993`) {
		t.Fatalf("payload=%#v", data["payload"])
	}
	if got := task.Messages[0].Parts[1]; got.OutputID != "101:data_qa_chart:1" || got.Source != "base_agent" || got.GroupID != "grp_1" {
		t.Fatalf("data part metadata=%+v", got)
	}
	if task.InputRequired == nil || task.InputRequired.DecisionID != "q_scene" ||
		task.InputRequired.InputType != iagents.InputTypeSingleSelect || len(task.InputRequired.Options) != 1 {
		t.Fatalf("input_required=%+v", task.InputRequired)
	}
	if task.InputRequired.Prompt != "请补充信息：高级设置：请选择场景" {
		t.Fatalf("prompt=%q", task.InputRequired.Prompt)
	}
	if task.InputRequired.Options[0].Description != "创建新表" || task.InputRequired.Data == nil {
		t.Fatalf("input_required details=%+v", task.InputRequired)
	}
	if task.InputRequired.OutputID != "102:question:1" || task.InputRequired.Source != "base_agent" || task.InputRequired.GroupID != "grp_1" {
		t.Fatalf("input_required metadata=%+v", task.InputRequired)
	}
	if len(task.Artifacts) != 1 {
		t.Fatalf("artifacts=%+v", task.Artifacts)
	}
	artifact := task.Artifacts[0]
	if artifact.ID != "artifact_1" || artifact.Kind != "table" || artifact.Name != "销售表" || artifact.Status != "ready" {
		t.Fatalf("artifact=%+v", artifact)
	}
	if artifact.OutputID != "103:artifact:1" || artifact.Source != "table_agent" || artifact.GroupID != "grp_2" {
		t.Fatalf("artifact metadata=%+v", artifact)
	}
	details, ok := artifact.Data.(map[string]interface{})
	if !ok || details["revision"] != int64(128) {
		t.Fatalf("artifact data=%#v", artifact.Data)
	}
	encoded, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	for _, exact := range []string{`"baseId":9007199254740993`, `"base_id":9007199254740993`} {
		if !strings.Contains(string(encoded), exact) {
			t.Fatalf("large integer %s was not preserved: %s", exact, encoded)
		}
	}
}

func TestVersionedTaskUsesLatestPendingClarification(t *testing.T) {
	task, err := mapTask(adapterTask{
		SchemaVersion: 1,
		TaskID:        "t1",
		Status:        "waiting_for_input",
		Outputs: []adapterOutput{
			{Type: "clarification", Clarification: &adapterClarification{ID: "old", Title: "旧问题", Required: true}},
			{Type: "text", Text: "处理中"},
			{Type: "clarification", Clarification: &adapterClarification{ID: "new", Title: "新问题", Required: true}},
		},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if task.InputRequired == nil || task.InputRequired.DecisionID != "new" || task.InputRequired.Prompt != "新问题" {
		t.Fatalf("input_required=%+v", task.InputRequired)
	}
}

func TestVersionedTaskTerminalStateIgnoresHistoricalClarification(t *testing.T) {
	for _, status := range []string{"completed", "failed", "canceled"} {
		t.Run(status, func(t *testing.T) {
			task, err := mapTask(adapterTask{
				SchemaVersion: 1,
				TaskID:        "t1",
				Status:        status,
				Outputs: []adapterOutput{{
					Type:          "clarification",
					Clarification: &adapterClarification{ID: "old", Required: true},
				}},
			}, false)
			if err != nil {
				t.Fatal(err)
			}
			if !task.IsTerminal || task.InputRequired != nil {
				t.Fatalf("task=%+v", task)
			}
			if status == "canceled" && task.State != iagents.StateCanceled {
				t.Fatalf("canceled state=%s", task.State)
			}
		})
	}
}

func TestVersionedTaskProtocolInconsistenciesAreTyped(t *testing.T) {
	tests := []adapterTask{
		{SchemaVersion: 2, TaskID: "t1", Status: "running"},
		{SchemaVersion: 1, TaskID: "t1", Status: "waiting_for_input"},
		{SchemaVersion: 1, TaskID: "t1", Status: "running", Outputs: []adapterOutput{{
			Type: "clarification", Clarification: &adapterClarification{ID: "q1", Required: true},
		}}},
		{SchemaVersion: 1, TaskID: "t1", Status: "running", Outputs: []adapterOutput{{ID: "x", Type: "data"}}},
	}
	for _, input := range tests {
		_, err := mapTask(input, false)
		problem(t, err, errs.CategoryInternal, errs.SubtypeInvalidResponse)
	}
}

func TestVersionedTaskUnknownOutputIsPreservedAsData(t *testing.T) {
	var wire adapterTask
	if err := json.Unmarshal([]byte(`{"schema_version":1,"task_id":"t1","status":"running","outputs":[{"id":"x","type":"future_output","source":"future_agent","group_id":"grp_x","future":{"value":9007199254740993}}]}`), &wire); err != nil {
		t.Fatal(err)
	}
	task, err := mapTask(wire, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Messages) != 1 || len(task.Messages[0].Parts) != 1 || task.Messages[0].Parts[0].Type != "data" {
		t.Fatalf("messages=%+v", task.Messages)
	}
	part := task.Messages[0].Parts[0]
	if part.OutputID != "x" || part.Source != "future_agent" || part.GroupID != "grp_x" {
		t.Fatalf("metadata=%+v", part)
	}
	encoded, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"future":{"value":9007199254740993}`) {
		t.Fatalf("unknown output was not preserved: %s", encoded)
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
		params: map[string]string{"base_token": "b1", "status": "active"},
		responses: []json.RawMessage{
			dataResponse(t, `[{"context_id":"c1","title":"Quarterly plan","created_at":1710000000,"updated_at":1710000060}]`),
			dataResponse(t, `{"context_id":"c1","title":"Quarterly plan","tasks":[{"task_id":"new","state":"running","updated_at":1710000060},{"task_id":"old","state":"done","updated_at":1710000000}]}`),
			dataResponse(t, `{"result":true}`),
		},
	}
	contexts, pageInfo, err := assistantSpec.ListContexts.Handler(context.Background(), rt, iagents.PageParams{Token: "next", Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	if pageInfo != (iagents.PageInfo{}) {
		t.Fatalf("pageInfo=%+v", pageInfo)
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

func TestMapContextDetailRollsUpTasks(t *testing.T) {
	tests := []struct {
		name          string
		tasks         []adapterTask
		activeID      string
		awaitingInput bool
	}{
		{
			name: "latest task is active while any task can await input",
			tasks: []adapterTask{
				{TaskID: "old", State: "input_required", UpdatedAt: json.RawMessage(`1710000000`), Summary: "choose one"},
				{TaskID: "latest", State: "done", UpdatedAt: json.RawMessage(`1710000060`), Summary: "complete"},
			},
			activeID:      "latest",
			awaitingInput: true,
		},
		{
			name: "waiting task wins an updated at tie regardless of order",
			tasks: []adapterTask{
				{TaskID: "20", State: "done", UpdatedAt: json.RawMessage(`1710000060`)},
				{TaskID: "10", State: "input_required", UpdatedAt: json.RawMessage(`1710000060`)},
			},
			activeID:      "10",
			awaitingInput: true,
		},
		{
			name: "numeric task id breaks a complete task tie",
			tasks: []adapterTask{
				{TaskID: "9", State: "done", UpdatedAt: json.RawMessage(`1710000060`)},
				{TaskID: "10", State: "done", UpdatedAt: json.RawMessage(`1710000060`)},
			},
			activeID: "10",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, reverse := range []bool{false, true} {
				tasks := append([]adapterTask(nil), test.tasks...)
				if reverse {
					for left, right := 0, len(tasks)-1; left < right; left, right = left+1, right-1 {
						tasks[left], tasks[right] = tasks[right], tasks[left]
					}
				}
				detail, err := mapContextDetail(adapterContext{ContextID: "c1", Tasks: tasks})
				if err != nil {
					t.Fatal(err)
				}
				if detail.ActiveTask == nil || detail.ActiveTask.TaskID != test.activeID || detail.AwaitingInput != test.awaitingInput {
					t.Fatalf("reverse=%v detail=%+v", reverse, detail)
				}
			}
		})
	}
}

func TestResultFalseUsesTypedCategory(t *testing.T) {
	rt := &fakeRuntime{
		params:    map[string]string{"base_token": "b1"},
		responses: []json.RawMessage{dataResponse(t, `{"result":false,"reason":"task is terminal","error":{"category":"task_terminal"}}`)},
	}
	err := assistantSpec.CancelTask.Handler(context.Background(), rt, "t1")
	problem(t, err, errs.CategoryValidation, errs.SubtypeFailedPrecondition)
}

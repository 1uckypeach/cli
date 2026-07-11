// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
)

// paramSpec builds a spec with a send declaration (required ws + enum/default
// priority + ranged integer) and a task_list declaration sharing ws — the
// cross-operation reverse-lookup and three-way-carry test bed.
func paramSpec() *iagent.AgentSpec {
	ws := iagent.CardParam{Name: "workspace_id", Type: "string", Required: true, Desc: "目标工作区"}
	return &iagent.AgentSpec{
		Send: iagent.SendOp{
			Params: []iagent.CardParam{
				ws,
				{Name: "priority", Type: "string", Enum: []string{"low", "normal", "high"}, Default: "normal"},
				{Name: "max_results", Type: "integer", Min: iagent.Float(1), Max: iagent.Float(100), Default: "20"},
			},
			Handler: func(context.Context, iagent.Runtime, iagent.SendInput) (*iagent.AgentTask, error) { return nil, nil },
		},
		GetTask: iagent.TaskGetOp{Handler: func(context.Context, iagent.Runtime, string) (*iagent.AgentTask, error) { return nil, nil }},
		ListTasks: iagent.TaskListOp{
			Params:  []iagent.CardParam{ws},
			Handler: func(context.Context, iagent.Runtime, string) ([]iagent.TaskSummary, error) { return nil, nil },
		},
	}
}

// TestValidateParams_CollectAll pins the batch contract: every violation in ONE
// error — two missing requireds are impossible on one decl set, so mix missing
// required + unknown key + enum violation and assert all three violations
// surface with self-contained specs.
func TestValidateParams_CollectAll(t *testing.T) {
	spec := paramSpec()
	_, err := validateParams(
		[]string{"priority=urgent", "bogus=1"},
		spec.Send.Params, iagent.VerbSend, spec, "acme:reporter")
	if err == nil {
		t.Fatal("should fail with collected violations")
	}
	var verr *errs.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("want *errs.ValidationError, got %T", err)
	}
	if len(verr.Params) != 3 {
		t.Fatalf("want 3 violations (enum + unknown + missing required), got %d: %+v", len(verr.Params), verr.Params)
	}
	byName := map[string]errs.InvalidParam{}
	for _, v := range verr.Params {
		byName[v.Name] = v
	}
	// enum violation lists the full set and embeds the spec
	if v := byName["priority"]; !strings.Contains(v.Reason, "low|normal|high") || v.Spec == nil {
		t.Errorf("priority violation should list the enum set and embed spec, got %+v", v)
	}
	// unknown key lists this operation's available params
	if v := byName["bogus"]; !strings.Contains(v.Reason, "workspace_id") {
		t.Errorf("unknown-key violation should list available params, got %+v", v)
	}
	// missing required embeds the full declaration so the caller can fix without
	// a discovery round-trip
	v := byName["workspace_id"]
	if !strings.Contains(v.Reason, "缺少必填参数") || v.Spec == nil {
		t.Fatalf("missing-required violation should embed spec, got %+v", v)
	}
	if sp, ok := v.Spec.(iagent.CardParam); !ok || sp.Desc != "目标工作区" {
		t.Errorf("embedded spec should be the full CardParam, got %+v", v.Spec)
	}
	// multi-violation message is a count summary; hint points at --operation
	if !strings.Contains(verr.Message, "3 处问题") {
		t.Errorf("multi-violation message should carry the count, got %q", verr.Message)
	}
	if !strings.Contains(verr.Hint, "--operation send") {
		t.Errorf("hint should point at card --operation send, got %q", verr.Hint)
	}
}

// TestValidateParams_CrossOpReverseLookup pins the "它声明在" teaching error: a
// param declared on send but passed to task_get names where it lives.
func TestValidateParams_CrossOpReverseLookup(t *testing.T) {
	spec := paramSpec()
	_, err := validateParams([]string{"priority=high"}, spec.GetTask.Params, iagent.VerbTaskGet, spec, "acme:reporter")
	if err == nil || !strings.Contains(err.Error(), "不适用于 task_get") || !strings.Contains(err.Error(), "它声明在: send") {
		t.Fatalf("cross-op teaching error expected, got %v", err)
	}
}

// TestValidateParams_RulesTable covers the remaining violation kinds one by one.
func TestValidateParams_RulesTable(t *testing.T) {
	spec := paramSpec()
	base := []string{"workspace_id=ws_42"}
	cases := []struct {
		name string
		kvs  []string
		want string
	}{
		{"duplicate", append(base, "workspace_id=ws_43"), "重复提供"},
		{"empty required", []string{"workspace_id="}, "不能为空值"},
		{"malformed", append(base, "noequals"), "key=value"},
		{"type mismatch", append(base, "max_results=abc"), "integer"},
		{"range violation", append(base, "max_results=500"), "1..100"},
		{"zero-param op given a param", nil, ""},
	}
	for _, tc := range cases[:5] {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateParams(tc.kvs, spec.Send.Params, iagent.VerbSend, spec, "acme:reporter")
			if err == nil || !strings.Contains(err.Error()+errHint(err), tc.want) {
				t.Fatalf("want %q in error, got %v", tc.want, err)
			}
		})
	}
	// value containing '=' splits on the first '=' only
	vp, err := validateParams(append(base, "priority=high"), spec.Send.Params, iagent.VerbSend, spec, "acme:reporter")
	if err != nil || vp.Given["workspace_id"] != "ws_42" {
		t.Fatalf("valid set should pass: %v %v", vp, err)
	}
}

// TestValidateParams_EmptyOptionalTreatedAsAbsent pins the review fix (blocker):
// `k=` on an OPTIONAL param counts as not provided — no violation, no entry in
// Given, and the declared Default still backfills Resolved, so no unvalidated
// "" can ever reach a hook (the rt.Params() contract).
func TestValidateParams_EmptyOptionalTreatedAsAbsent(t *testing.T) {
	spec := paramSpec()
	vp, err := validateParams([]string{"workspace_id=ws_42", "max_results="}, spec.Send.Params, iagent.VerbSend, spec, "acme:reporter")
	if err != nil {
		t.Fatalf("empty optional should not violate: %v", err)
	}
	if got := vp.Resolved["max_results"]; got != "20" {
		t.Errorf("empty optional must not shadow the default (backfill still applies), got %q", got)
	}
	if _, ok := vp.Given["max_results"]; ok {
		t.Errorf("empty optional must not enter Given, got %v", vp.Given)
	}
	// empty on a declared optional with default: default wins in Resolved
	vp2, err := validateParams([]string{"workspace_id=ws_42", "priority="}, spec.Send.Params, iagent.VerbSend, spec, "acme:reporter")
	if err != nil {
		t.Fatalf("empty optional should not violate: %v", err)
	}
	if vp2.Resolved["priority"] != "normal" {
		t.Errorf("empty optional must not shadow the default, got %q", vp2.Resolved["priority"])
	}
	// duplicate detection still sees the empty occurrence
	_, err = validateParams([]string{"workspace_id=ws_42", "priority=", "priority=high"}, spec.Send.Params, iagent.VerbSend, spec, "acme:reporter")
	if err == nil || !strings.Contains(err.Error()+errHint(err), "重复提供") {
		t.Fatalf("duplicate after empty occurrence must be reported, got %v", err)
	}
}

// TestValidateParams_NoFalseMissingOnInvalidValue pins the review fix: a
// required param given an INVALID value reports exactly the value violation —
// never an additional contradictory "缺少必填参数"; and a duplicate after an
// invalid first value is reported as duplicate, not as the same violation twice.
func TestValidateParams_NoFalseMissingOnInvalidValue(t *testing.T) {
	spec := paramSpec()
	// make workspace_id enum-constrained for this test via a local declaration
	decl := []iagent.CardParam{{Name: "mode", Type: "string", Required: true, Enum: []string{"a", "b"}}}
	_, err := validateParams([]string{"mode=zzz"}, decl, iagent.VerbSend, spec, "acme:reporter")
	var verr *errs.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("want validation error, got %T", err)
	}
	if len(verr.Params) != 1 {
		t.Fatalf("invalid value must yield exactly 1 violation (no false missing-required), got %d: %+v", len(verr.Params), verr.Params)
	}
	if !strings.Contains(verr.Params[0].Reason, "a|b") {
		t.Errorf("the one violation should be the enum violation, got %+v", verr.Params[0])
	}
	// duplicate after invalid first value → enum violation + duplicate violation
	_, err = validateParams([]string{"mode=zzz", "mode=zzz"}, decl, iagent.VerbSend, spec, "acme:reporter")
	if !errors.As(err, &verr) {
		t.Fatalf("want validation error, got %T", err)
	}
	if len(verr.Params) != 2 {
		t.Fatalf("want enum violation + duplicate violation, got %d: %+v", len(verr.Params), verr.Params)
	}
	kinds := verr.Params[0].Reason + verr.Params[1].Reason
	if !strings.Contains(kinds, "a|b") || !strings.Contains(kinds, "重复提供") {
		t.Errorf("want one enum + one duplicate violation, got %+v", verr.Params)
	}
}

func errHint(err error) string {
	if p, ok := errs.ProblemOf(err); ok {
		return p.Hint
	}
	return ""
}

// TestValidateParams_DefaultBackfill pins Resolved vs Given: defaults land in
// Resolved (what the hook sees) but never in Given (what meta.next carries).
func TestValidateParams_DefaultBackfill(t *testing.T) {
	spec := paramSpec()
	vp, err := validateParams([]string{"workspace_id=ws_42"}, spec.Send.Params, iagent.VerbSend, spec, "acme:reporter")
	if err != nil {
		t.Fatalf("should pass: %v", err)
	}
	if vp.Resolved["priority"] != "normal" || vp.Resolved["max_results"] != "20" {
		t.Errorf("defaults should backfill Resolved, got %v", vp.Resolved)
	}
	if _, ok := vp.Given["priority"]; ok {
		t.Errorf("defaults must NOT appear in Given (meta.next noise), got %v", vp.Given)
	}
	// an explicitly provided value overrides the default in Resolved
	vp2, _ := validateParams([]string{"workspace_id=ws_42", "priority=high"}, spec.Send.Params, iagent.VerbSend, spec, "acme:reporter")
	if vp2.Resolved["priority"] != "high" || vp2.Given["priority"] != "high" {
		t.Errorf("explicit value should override default, got %v / %v", vp2.Resolved, vp2.Given)
	}
}

// TestParamArgsFor pins the three-way carry rule.
func TestParamArgsFor(t *testing.T) {
	spec := paramSpec()
	// 1) given + whitelisted → literal carry (declaration order)
	args, tpl := paramArgsFor(spec, iagent.VerbSend, map[string]string{"workspace_id": "ws_42", "priority": "high"})
	if args != " --param workspace_id=ws_42 --param priority=high" || tpl {
		t.Errorf("literal carry wrong: %q tpl=%v", args, tpl)
	}
	// 2) given but whitelist-failing → required degrades to placeholder,
	//    optional drops
	args, tpl = paramArgsFor(spec, iagent.VerbSend, map[string]string{"workspace_id": "ws 42; rm", "priority": "值 带 空格"})
	if !strings.Contains(args, "--param workspace_id=<workspace_id>") || strings.Contains(args, "priority") || !tpl {
		t.Errorf("degrade rule wrong: %q tpl=%v", args, tpl)
	}
	// 3) absent but required on the target verb → placeholder (cross-verb hole)
	args, tpl = paramArgsFor(spec, iagent.VerbTaskList, map[string]string{})
	if args != " --param workspace_id=<workspace_id>" || !tpl {
		t.Errorf("required-absent placeholder wrong: %q tpl=%v", args, tpl)
	}
	// nil spec / unknown verb carry nothing
	if a, _ := paramArgsFor(nil, iagent.VerbSend, nil); a != "" {
		t.Errorf("nil spec should carry nothing, got %q", a)
	}
}

// TestNextForTaskCarriesParams pins the wired outcome: a send with given params
// yields a poll hint carrying them literally.
func TestNextForTaskCarriesParams(t *testing.T) {
	spec := paramSpec()
	task := &iagent.AgentTask{TaskID: "task_1", State: iagent.StateWorking}
	// task_get declares no params on this spec → nothing to carry for the poll
	next := nextForTask("acme:reporter", task, spec, map[string]string{"workspace_id": "ws_42"})
	if len(next) != 1 || strings.Contains(next[0].Command, "--param") {
		t.Fatalf("task_get declares no params, poll hint should carry none: %+v", next)
	}
	// give task_get a required param → the poll hint must carry it
	spec.GetTask.Params = []iagent.CardParam{{Name: "workspace_id", Type: "string", Required: true}}
	next = nextForTask("acme:reporter", task, spec, map[string]string{"workspace_id": "ws_42"})
	if !strings.Contains(next[0].Command, "--param workspace_id=ws_42") {
		t.Fatalf("poll hint should carry the given required param: %+v", next)
	}
	// absent → placeholder + template
	next = nextForTask("acme:reporter", task, spec, nil)
	if !strings.Contains(next[0].Command, "--param workspace_id=<workspace_id>") || !next[0].Template {
		t.Fatalf("absent required should degrade to placeholder+template: %+v", next)
	}
}

// TestArtifactNext pins the per-artifact download hints: terminal task +
// wired DownloadArtifact → one template hint per whitelisted artifact id;
// whitelist-failing ids are skipped (never interpolated).
func TestArtifactNext(t *testing.T) {
	spec := paramSpec()
	spec.DownloadArtifact = iagent.ArtifactDownloadOp{
		Params:  []iagent.CardParam{{Name: "workspace_id", Type: "string", Required: true}},
		Handler: func(context.Context, iagent.Runtime, string, string) (*iagent.ArtifactData, error) { return nil, nil },
	}
	task := &iagent.AgentTask{
		TaskID: "task_1", State: iagent.StateCompleted, IsTerminal: true,
		Artifacts: []iagent.Artifact{{ID: "art_1"}, {ID: "bad;id"}, {ID: "art_2"}},
	}
	next := nextForTask("acme:reporter", task, spec, map[string]string{"workspace_id": "ws_42"})
	var downloads []string
	for _, n := range next {
		if strings.Contains(n.Command, "--artifact") {
			downloads = append(downloads, n.Command)
			if !n.Template {
				t.Errorf("download hint has a -o placeholder, must be template: %+v", n)
			}
		}
	}
	if len(downloads) != 2 {
		t.Fatalf("want 2 download hints (bad;id skipped), got %d: %v", len(downloads), downloads)
	}
	for _, c := range downloads {
		if !strings.Contains(c, "--param workspace_id=ws_42") || !strings.Contains(c, "-o <保存路径>") {
			t.Errorf("download hint should carry params and the -o placeholder: %q", c)
		}
		if strings.Contains(c, "bad;id") {
			t.Errorf("whitelist-failing artifact id leaked: %q", c)
		}
	}
	// unwired DownloadArtifact → no hints
	spec.DownloadArtifact = iagent.ArtifactDownloadOp{}
	if n := artifactNext("acme:reporter", task, spec, nil); n != nil {
		t.Errorf("unwired artifact_download should produce no hints, got %+v", n)
	}
}

// TestCardOperationSubquery pins `card --operation <verb>` against the real
// example provider: reporter's send contract carries command + parameters;
// unknown verb lists the vocabulary; unwired verb answers supported:false; a
// wired zero-param verb answers parameters:[].
func TestCardOperationSubquery(t *testing.T) {
	decode := func(t *testing.T, opts *cardOptions) map[string]any {
		t.Helper()
		out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })
		if err := agentCardRun(opts); err != nil {
			t.Fatalf("card --operation should not error: %v", err)
		}
		var env struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(out.Bytes(), &env); err != nil {
			t.Fatalf("invalid envelope: %v", err)
		}
		return env.Data
	}

	opts, _ := cardTestOpts(t, "example:reporter")
	opts.Operation = "send"
	data := decode(t, opts)
	if data["operation"] != "send" || data["supported"] != true {
		t.Fatalf("send contract wrong: %v", data)
	}
	if cmdStr, _ := data["command"].(string); !strings.Contains(cmdStr, "lark-cli agent send") {
		t.Errorf("contract should carry the command template, got %v", data["command"])
	}
	params, _ := data["parameters"].([]any)
	if len(params) != 2 {
		t.Fatalf("reporter send declares 2 demo params, got %v", data["parameters"])
	}
	first, _ := params[0].(map[string]any)
	if first["name"] != "report_format" || first["default"] != "csv" {
		t.Errorf("first param should be report_format with default csv, got %v", first)
	}

	// unwired verb → supported:false
	opts2, _ := cardTestOpts(t, "example:echo")
	opts2.Operation = "task_cancel"
	data = decode(t, opts2)
	if data["supported"] != false {
		t.Errorf("echo task_cancel should be supported:false, got %v", data)
	}

	// wired zero-param verb → parameters []
	opts3, _ := cardTestOpts(t, "example:echo")
	opts3.Operation = "context_delete"
	data = decode(t, opts3)
	if data["supported"] != true {
		t.Fatalf("echo context_delete should be supported, got %v", data)
	}
	if ps, ok := data["parameters"].([]any); !ok || len(ps) != 0 {
		t.Errorf("zero-param op should answer parameters:[], got %v", data["parameters"])
	}

	// unknown verb → invalid_argument listing the vocabulary
	opts4, _ := cardTestOpts(t, "example:echo")
	opts4.Operation = "sennd"
	err := agentCardRun(opts4)
	if err == nil || !strings.Contains(err.Error(), "task_get") || !strings.Contains(err.Error(), "all") {
		t.Fatalf("unknown verb should list the vocabulary, got %v", err)
	}
	if p, ok := errs.ProblemOf(err); !ok || p.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("unknown verb should be invalid_argument, got %+v", p)
	}
}

// TestCardOperationInstanceShape pins the review fix: on an INSTANCE provider
// (fakeflow), the single-verb --operation output reuses the struct — an
// unwired verb carries NO command key (omitempty, not command:"") and every
// response carries parameters_source:"template".
func TestCardOperationInstanceShape(t *testing.T) {
	registerScripted()
	opts, _ := cardTestOpts(t, "fakeflow:agt_x")
	opts.Operation = "task_cancel" // scriptedSpec deliberately leaves CancelTask unwired
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })
	if err := agentCardRun(opts); err != nil {
		t.Fatalf("card --operation should not error: %v", err)
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	if env.Data["supported"] != false {
		t.Fatalf("task_cancel should be unsupported on the scripted spec, got %v", env.Data)
	}
	if _, present := env.Data["command"]; present {
		t.Errorf("unwired verb must not carry a command key (omitempty), got %v", env.Data["command"])
	}
	if env.Data["parameters_source"] != "template" {
		t.Errorf("instance provider --operation should carry parameters_source:template, got %v", env.Data)
	}
}

// TestCardOperationAll pins the one-shot full map: every verb present, wired
// ones carrying command+parameters.
func TestCardOperationAll(t *testing.T) {
	opts, _ := cardTestOpts(t, "example:reporter")
	opts.Operation = "all"
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })
	if err := agentCardRun(opts); err != nil {
		t.Fatalf("card --operation all should not error: %v", err)
	}
	var env struct {
		Data struct {
			Operations map[string]map[string]any `json:"operations"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	if len(env.Data.Operations) != 8 {
		t.Fatalf("all should enumerate 8 operations, got %d", len(env.Data.Operations))
	}
	send := env.Data.Operations["send"]
	if send["supported"] != true {
		t.Errorf("reporter send should be supported, got %v", send)
	}
	if ps, _ := send["parameters"].([]any); len(ps) != 2 {
		t.Errorf("reporter send should carry its 2 demo params, got %v", send["parameters"])
	}
}

// TestCardLeanHasParameters pins the lean card cue on the real reporter: send
// appears in has_parameters (it declares demo params), context_delete does not.
func TestCardLeanHasParameters(t *testing.T) {
	opts, _ := cardTestOpts(t, "example:reporter")
	out := opts.Factory.IOStreams.Out.(interface{ Bytes() []byte })
	if err := agentCardRun(opts); err != nil {
		t.Fatalf("card should not error: %v", err)
	}
	var env struct {
		Data struct {
			HasParameters []string `json:"has_parameters"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	if len(env.Data.HasParameters) != 1 || env.Data.HasParameters[0] != "send" {
		t.Fatalf("reporter has_parameters should be [send], got %v", env.Data.HasParameters)
	}
}

// TestSendValidatesDeclaredParams drives the full send path against the real
// reporter declaration: enum violation fails offline; a valid --param passes
// through to dry-run with defaults backfilled.
func TestSendValidatesDeclaredParams(t *testing.T) {
	opts := sendTestOpts(t)
	opts.Ref = "example:reporter"
	opts.Text = "报表"
	opts.Params = []string{"report_format=pdf"}
	err := agentSendRun(opts)
	if err == nil || !strings.Contains(err.Error(), "csv|xlsx") {
		t.Fatalf("enum violation should fail offline listing the set, got %v", err)
	}

	opts2 := sendTestOpts(t)
	opts2.Ref = "example:reporter"
	opts2.Text = "报表"
	opts2.Params = []string{"report_format=xlsx"}
	opts2.DryRun = true
	out := opts2.Factory.IOStreams.Out.(interface{ Bytes() []byte })
	if err := agentSendRun(opts2); err != nil {
		t.Fatalf("valid param should pass: %v", err)
	}
	var env struct {
		Data struct {
			WouldSend struct {
				Params map[string]string `json:"params"`
			} `json:"would_send"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	if env.Data.WouldSend.Params["report_format"] != "xlsx" || env.Data.WouldSend.Params["quarters"] != "4" {
		t.Fatalf("dry-run should show the resolved params (default quarters=4 backfilled), got %v", env.Data.WouldSend.Params)
	}
}

// TestListRejectsParams pins the two list guards: --param without a scheme is
// rejected outright; --param on a catalog scheme validates against the empty
// set with the list-specific hint.
func TestListRejectsParams(t *testing.T) {
	opts, _ := listFactory()
	opts.Params = []string{"env=boe"}
	err := agentListRun(opts)
	if err == nil || !strings.Contains(err.Error(), "仅在 agent list <scheme>") {
		t.Fatalf("no-scheme --param should be rejected, got %v", err)
	}

	opts2, _ := listFactory()
	opts2.Scheme = "example"
	opts2.Params = []string{"env=boe"}
	err = agentListRun(opts2)
	if err == nil {
		t.Fatal("catalog scheme with --param should be rejected (zero-param op)")
	}
	if p, ok := errs.ProblemOf(err); !ok || !strings.Contains(p.Hint, "list_parameters") {
		t.Fatalf("list param error hint should point at providers[].list_parameters, got %+v", p)
	}
}

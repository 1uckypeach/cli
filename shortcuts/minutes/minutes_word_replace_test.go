// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package minutes

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/spf13/cobra"
)

const minutesWordReplaceTestToken = "obcnexampleminute"

func TestMinutesWordReplace_Validate(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	f, _, _, _ := cmdutil.TestFactory(t, defaultConfig())
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing minute token",
			args:    []string{"+word-replace", "--replace-words", `[{"source_word":"a","target_word":"b"}]`, "--as", "user"},
			wantErr: "required flag(s) \"minute-token\" not set",
		},
		{
			name:    "missing replace words",
			args:    []string{"+word-replace", "--minute-token", minutesWordReplaceTestToken, "--as", "user"},
			wantErr: "required flag(s) \"replace-words\" not set",
		},
		{
			name:    "invalid json",
			args:    []string{"+word-replace", "--minute-token", minutesWordReplaceTestToken, "--replace-words", "not-json", "--as", "user"},
			wantErr: "JSON array",
		},
		{
			name:    "empty source word",
			args:    []string{"+word-replace", "--minute-token", minutesWordReplaceTestToken, "--replace-words", `[{"source_word":"","target_word":"b"}]`, "--as", "user"},
			wantErr: "source_word is required",
		},
		{
			name:    "duplicate source word",
			args:    []string{"+word-replace", "--minute-token", minutesWordReplaceTestToken, "--replace-words", `[{"source_word":"a","target_word":"b"},{"source_word":"a","target_word":"c"}]`, "--as", "user"},
			wantErr: "duplicate source_word",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := &cobra.Command{Use: "minutes"}
			MinutesWordReplace.Mount(parent, f)
			parent.SetArgs(tt.args)
			parent.SilenceErrors = true
			parent.SilenceUsage = true
			err := parent.Execute()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error should contain %q, got: %s", tt.wantErr, err.Error())
			}
		})
	}
}

func TestMinutesWordReplace_DryRun(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	f, stdout, _, _ := cmdutil.TestFactory(t, defaultConfig())
	warmTokenCache(t)

	err := mountAndRun(t, MinutesWordReplace, []string{
		"+word-replace",
		"--minute-token", minutesWordReplaceTestToken,
		"--replace-words", `[{"source_word":"foo","target_word":"bar"}]`,
		"--dry-run", "--as", "user",
	}, f, stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "PUT") {
		t.Errorf("expected PUT method, got:\n%s", out)
	}
	if !strings.Contains(out, "/open-apis/minutes/v1/minutes/"+minutesWordReplaceTestToken+"/transcript/word") {
		t.Errorf("expected word endpoint, got:\n%s", out)
	}
	if !strings.Contains(out, "foo") || !strings.Contains(out, "bar") {
		t.Errorf("expected replace_words in body, got:\n%s", out)
	}
}

func TestMinutesWordReplace_Execute(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
	warmTokenCache(t)

	reg.Register(&httpmock.Stub{
		Method: http.MethodPut,
		URL:    "/open-apis/minutes/v1/minutes/" + minutesWordReplaceTestToken + "/transcript/word",
		Body: map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{
				"replace_word_counts": []map[string]interface{}{
					{"source_word": "foo", "replace_count": "2"},
				},
			},
		},
	})

	err := mountAndRun(t, MinutesWordReplace, []string{
		"+word-replace",
		"--minute-token", minutesWordReplaceTestToken,
		"--replace-words", `[{"source_word":"foo","target_word":"bar"}]`,
		"--format", "json", "--as", "user",
	}, f, stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			MinuteToken string `json:"minute_token"`
			Message     string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("ok = false, want true; stdout=%s", stdout.String())
	}
	if envelope.Data.MinuteToken != minutesWordReplaceTestToken {
		t.Errorf("data.minute_token = %q, want %q", envelope.Data.MinuteToken, minutesWordReplaceTestToken)
	}
	wantMsg := "Succeeded: foo; Failed: none. " + minutesWordReplaceDoNotRetrySucceeded
	if envelope.Data.Message != wantMsg {
		t.Errorf("message = %q, want %q", envelope.Data.Message, wantMsg)
	}
}

func TestMinutesWordReplace_PartialSuccess(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
	warmTokenCache(t)

	reg.Register(&httpmock.Stub{
		Method: http.MethodPut,
		URL:    "/open-apis/minutes/v1/minutes/" + minutesWordReplaceTestToken + "/transcript/word",
		Body: map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{
				"replace_word_counts": []map[string]interface{}{
					{"source_word": "missing", "replace_count": "0"},
					{"source_word": "hello", "replace_count": "1"},
				},
			},
		},
	})

	err := mountAndRun(t, MinutesWordReplace, []string{
		"+word-replace",
		"--minute-token", minutesWordReplaceTestToken,
		"--replace-words", `[{"source_word":"missing","target_word":"Lark"},{"source_word":"hello","target_word":"hi"}]`,
		"--format", "json", "--as", "user",
	}, f, stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal stdout: %v\nraw=%s", err, stdout.String())
	}
	if !envelope.OK {
		t.Fatalf("ok = false, want true for partial success (aligned with OAPI); stdout=%s", stdout.String())
	}
	wantMsg := "Succeeded: hello; Failed: missing. " + minutesWordReplaceDoNotRetrySucceeded
	if envelope.Data.Message != wantMsg {
		t.Errorf("message = %q, want %q", envelope.Data.Message, wantMsg)
	}
}

func TestMinutesWordReplace_MissingCountsIsNotAllSuccess(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
	warmTokenCache(t)

	// Server returns code=0 without replace_word_counts (old / incomplete deploy).
	reg.Register(&httpmock.Stub{
		Method: http.MethodPut,
		URL:    "/open-apis/minutes/v1/minutes/" + minutesWordReplaceTestToken + "/transcript/word",
		Body: map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{},
		},
	})

	err := mountAndRun(t, MinutesWordReplace, []string{
		"+word-replace",
		"--minute-token", minutesWordReplaceTestToken,
		"--replace-words", `[{"source_word":"hello","target_word":"hi"},{"source_word":"missing","target_word":"x"}]`,
		"--format", "json", "--as", "user",
	}, f, stdout)
	if err == nil {
		t.Fatal("expected failed_precondition when replace_word_counts is missing, got nil")
	}
	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("want typed errs.*, got %T: %v", err, err)
	}
	if p.Subtype != errs.SubtypeFailedPrecondition {
		t.Errorf("subtype = %q, want %q", p.Subtype, errs.SubtypeFailedPrecondition)
	}
	if strings.Contains(stdout.String(), "Failed: none") {
		t.Fatalf("must not invent all-success message, stdout=%s", stdout.String())
	}
}

func TestMinutesWordReplace_AllCountsZero(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
	warmTokenCache(t)

	reg.Register(&httpmock.Stub{
		Method: http.MethodPut,
		URL:    "/open-apis/minutes/v1/minutes/" + minutesWordReplaceTestToken + "/transcript/word",
		Body: map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{
				"replace_word_counts": []map[string]interface{}{
					{"source_word": "foo", "replace_count": "0"},
					{"source_word": "bar", "replace_count": "0"},
				},
			},
		},
	})

	err := mountAndRun(t, MinutesWordReplace, []string{
		"+word-replace",
		"--minute-token", minutesWordReplaceTestToken,
		"--replace-words", `[{"source_word":"foo","target_word":"x"},{"source_word":"bar","target_word":"y"}]`,
		"--format", "json", "--as", "user",
	}, f, stdout)
	if err == nil {
		t.Fatal("expected failure exit signal, got nil")
	}

	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("want typed errs.*, got %T: %v", err, err)
	}
	if p.Subtype != errs.SubtypeNotFound {
		t.Errorf("subtype = %q, want %q", p.Subtype, errs.SubtypeNotFound)
	}
	if p.Code != minutesWordReplaceWordsNotFound {
		t.Errorf("code = %d, want %d", p.Code, minutesWordReplaceWordsNotFound)
	}
	if !strings.Contains(p.Message, "Failed: foo, bar") {
		t.Errorf("message should list failed words, got: %s", p.Message)
	}
	if !strings.Contains(p.Message, "nothing was replaced") {
		t.Errorf("message should state nothing was replaced, got: %s", p.Message)
	}
}

func TestMinutesWordReplace_NoEditPermission(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
	warmTokenCache(t)

	reg.Register(&httpmock.Stub{
		Method: http.MethodPut,
		URL:    "/open-apis/minutes/v1/minutes/" + minutesWordReplaceTestToken + "/transcript/word",
		Body: map[string]interface{}{
			"code": minutesWordReplaceNoEditPermission,
			"msg":  "permission deny",
		},
	})

	err := mountAndRun(t, MinutesWordReplace, []string{
		"+word-replace",
		"--minute-token", minutesWordReplaceTestToken,
		"--replace-words", `[{"source_word":"foo","target_word":"bar"}]`,
		"--format", "json", "--as", "user",
	}, f, stdout)
	if err == nil {
		t.Fatal("expected no-edit-permission error, got nil")
	}

	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("want typed errs.*, got %T: %v", err, err)
	}
	if p.Subtype != errs.SubtypePermissionDenied {
		t.Errorf("subtype = %q, want %q", p.Subtype, errs.SubtypePermissionDenied)
	}
}

func TestMinutesWordReplace_OthersAreEditing(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
	warmTokenCache(t)

	reg.Register(&httpmock.Stub{
		Method: http.MethodPut,
		URL:    "/open-apis/minutes/v1/minutes/" + minutesWordReplaceTestToken + "/transcript/word",
		Body: map[string]interface{}{
			"code": minutesWordReplaceOthersEditing,
			"msg":  "others are editing",
		},
	})

	err := mountAndRun(t, MinutesWordReplace, []string{
		"+word-replace",
		"--minute-token", minutesWordReplaceTestToken,
		"--replace-words", `[{"source_word":"foo","target_word":"bar"}]`,
		"--format", "json", "--as", "user",
	}, f, stdout)
	if err == nil {
		t.Fatal("expected others-are-editing error, got nil")
	}

	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("want typed errs.*, got %T: %v", err, err)
	}
	if p.Subtype != errs.SubtypeConflict {
		t.Errorf("subtype = %q, want %q", p.Subtype, errs.SubtypeConflict)
	}
}

// The stub reproduces a real gateway response observed against a live minute:
// the bare internal code with no msg. The code is written as a literal so that
// changing minutesWordReplaceWordsNotFound to a value the gateway never sends
// fails this test instead of passing vacuously.
func TestMinutesWordReplace_WordsNotFound(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
	warmTokenCache(t)

	reg.Register(&httpmock.Stub{
		Method: http.MethodPut,
		URL:    "/open-apis/minutes/v1/minutes/" + minutesWordReplaceTestToken + "/transcript/word",
		Body: map[string]interface{}{
			"code": 40013,
			"msg":  "",
		},
	})

	err := mountAndRun(t, MinutesWordReplace, []string{
		"+word-replace",
		"--minute-token", minutesWordReplaceTestToken,
		"--replace-words", `[{"source_word":"foo","target_word":"bar"}]`,
		"--format", "json", "--as", "user",
	}, f, stdout)
	if err == nil {
		t.Fatal("expected words-not-found error, got nil")
	}

	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("want typed errs.*, got %T: %v", err, err)
	}
	if p.Subtype != errs.SubtypeNotFound {
		t.Errorf("subtype = %q, want %q", p.Subtype, errs.SubtypeNotFound)
	}
	if !strings.Contains(p.Message, minutesWordReplaceTestToken) {
		t.Errorf("message should include minute token, got: %s", p.Message)
	}
	if !strings.Contains(p.Message, "nothing was replaced") {
		t.Errorf("message should state nothing was replaced, got: %s", p.Message)
	}
	if !strings.Contains(p.Hint, "source_word") || !strings.Contains(p.Hint, "+detail") {
		t.Errorf("hint should point at source_word and +detail, got: %s", p.Hint)
	}
}

// Only the dedicated words-not-found code earns the not_found rewrite. A generic
// invalid-params failure must surface as-is, even when its message happens to
// mention the transcript.
func TestMinutesWordReplace_GenericInvalidParamsNotRewritten(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
	warmTokenCache(t)

	reg.Register(&httpmock.Stub{
		Method: http.MethodPut,
		URL:    "/open-apis/minutes/v1/minutes/" + minutesWordReplaceTestToken + "/transcript/word",
		Body: map[string]interface{}{
			"code": 40001,
			"msg":  "replace words not found in transcript",
		},
	})

	err := mountAndRun(t, MinutesWordReplace, []string{
		"+word-replace",
		"--minute-token", minutesWordReplaceTestToken,
		"--replace-words", `[{"source_word":"foo","target_word":"bar"}]`,
		"--format", "json", "--as", "user",
	}, f, stdout)
	if err == nil {
		t.Fatal("expected invalid-params error, got nil")
	}

	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("want typed errs.*, got %T: %v", err, err)
	}
	if p.Subtype == errs.SubtypeNotFound && strings.Contains(p.Message, "None of the source words were found") {
		t.Fatalf("generic 40001 must not be rewritten as not_found, got subtype=%q message=%q", p.Subtype, p.Message)
	}
}

func TestFormatWordReplaceMessage(t *testing.T) {
	got := formatWordReplaceMessage([]string{"hello"}, []string{"missing"})
	want := "Succeeded: hello; Failed: missing. " + minutesWordReplaceDoNotRetrySucceeded
	if got != want {
		t.Fatalf("formatWordReplaceMessage() = %q, want %q", got, want)
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package vc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/shortcuts/common"
)

func newMeetingCountdownRuntime() *common.RuntimeContext {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-id", "", "")
	cmd.Flags().String("action", "", "")
	cmd.Flags().Int("duration", 0, "")
	cmd.Flags().Bool("need-play-audio-at-end", false, "")
	cmd.Flags().IntSlice("reminders-before-end-in-second", nil, "")
	return common.TestNewRuntimeContext(cmd, defaultConfig())
}

func mustSetMeetingCountdownFlag(t *testing.T, runtime *common.RuntimeContext, name, value string) {
	t.Helper()
	if err := runtime.Cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("Flags().Set(%q, %q) error = %v", name, value, err)
	}
}

func assertMeetingCountdownValidationError(t *testing.T, err error, wantParam string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation error")
	}
	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("expected typed problem, got %T: %v", err, err)
	}
	if p.Category != errs.CategoryValidation {
		t.Errorf("Category = %q, want %q", p.Category, errs.CategoryValidation)
	}
	if p.Subtype != errs.SubtypeInvalidArgument {
		t.Errorf("Subtype = %q, want %q", p.Subtype, errs.SubtypeInvalidArgument)
	}
	var ve *errs.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *errs.ValidationError, got %T: %v", err, err)
	}
	if ve.Param != wantParam {
		t.Errorf("Param = %q, want %q", ve.Param, wantParam)
	}
}

func TestMeetingCountdownBuildBody_Set(t *testing.T) {
	runtime := newMeetingCountdownRuntime()
	mustSetMeetingCountdownFlag(t, runtime, "action", "set")
	mustSetMeetingCountdownFlag(t, runtime, "duration", "5")
	mustSetMeetingCountdownFlag(t, runtime, "need-play-audio-at-end", "true")
	mustSetMeetingCountdownFlag(t, runtime, "reminders-before-end-in-second", "60,30")

	body, err := buildMeetingCountdownBody(runtime)
	if err != nil {
		t.Fatalf("buildMeetingCountdownBody() error = %v", err)
	}
	if body["action"] != meetingCountdownActionSet {
		t.Fatalf("action = %v, want set", body["action"])
	}
	if body["duration"] != 5 {
		t.Fatalf("duration = %v, want 5", body["duration"])
	}
	if body["need_play_audio_at_end"] != true {
		t.Fatalf("need_play_audio_at_end = %v, want true", body["need_play_audio_at_end"])
	}
	reminders, ok := body["reminders_before_end_in_second"].([]int)
	if !ok {
		t.Fatalf("reminders_before_end_in_second type = %T, want []int", body["reminders_before_end_in_second"])
	}
	if len(reminders) != 2 || reminders[0] != 60 || reminders[1] != 30 {
		t.Fatalf("reminders_before_end_in_second = %#v, want [60 30]", reminders)
	}
}

func TestMeetingCountdownBuildBody_Prolong(t *testing.T) {
	runtime := newMeetingCountdownRuntime()
	mustSetMeetingCountdownFlag(t, runtime, "action", "prolong")
	mustSetMeetingCountdownFlag(t, runtime, "duration", "3")

	body, err := buildMeetingCountdownBody(runtime)
	if err != nil {
		t.Fatalf("buildMeetingCountdownBody() error = %v", err)
	}
	if body["action"] != meetingCountdownActionProlong {
		t.Fatalf("action = %v, want prolong", body["action"])
	}
	if body["duration"] != 3 {
		t.Fatalf("duration = %v, want 3", body["duration"])
	}
	if _, ok := body["reminders_before_end_in_second"]; ok {
		t.Fatalf("reminders_before_end_in_second should be omitted for prolong")
	}
}

func TestMeetingCountdownBuildBody_Close(t *testing.T) {
	runtime := newMeetingCountdownRuntime()
	mustSetMeetingCountdownFlag(t, runtime, "action", "close")

	body, err := buildMeetingCountdownBody(runtime)
	if err != nil {
		t.Fatalf("buildMeetingCountdownBody() error = %v", err)
	}
	if body["action"] != meetingCountdownActionClose {
		t.Fatalf("action = %v, want close", body["action"])
	}
	if _, ok := body["duration"]; ok {
		t.Fatalf("duration should be omitted for close")
	}
}

func TestMeetingCountdownValidateRejectsMeetingNumber(t *testing.T) {
	runtime := newMeetingCountdownRuntime()
	mustSetMeetingCountdownFlag(t, runtime, "meeting-id", "123456789")
	mustSetMeetingCountdownFlag(t, runtime, "action", "close")

	err := VCMeetingCountdown.Validate(context.Background(), runtime)
	assertMeetingCountdownValidationError(t, err, "--meeting-id")
	if !strings.Contains(err.Error(), "9-digit meeting number") {
		t.Fatalf("error = %v, want 9-digit meeting number hint", err)
	}
}

func TestMeetingCountdownValidateRejectsMissingDurationForSet(t *testing.T) {
	runtime := newMeetingCountdownRuntime()
	mustSetMeetingCountdownFlag(t, runtime, "meeting-id", "7651377260537433044")
	mustSetMeetingCountdownFlag(t, runtime, "action", "set")

	err := VCMeetingCountdown.Validate(context.Background(), runtime)
	assertMeetingCountdownValidationError(t, err, "--duration")
	if !strings.Contains(err.Error(), "positive number of minutes") {
		t.Fatalf("error = %v, want duration minute hint", err)
	}
}

func TestMeetingCountdownValidateRejectsReminderNotLessThanDurationSeconds(t *testing.T) {
	runtime := newMeetingCountdownRuntime()
	mustSetMeetingCountdownFlag(t, runtime, "meeting-id", "7651377260537433044")
	mustSetMeetingCountdownFlag(t, runtime, "action", "set")
	mustSetMeetingCountdownFlag(t, runtime, "duration", "1")
	mustSetMeetingCountdownFlag(t, runtime, "reminders-before-end-in-second", "60")

	err := VCMeetingCountdown.Validate(context.Background(), runtime)
	assertMeetingCountdownValidationError(t, err, "--reminders-before-end-in-second")
	if !strings.Contains(err.Error(), "converted to seconds") {
		t.Fatalf("error = %v, want duration seconds hint", err)
	}
}

func TestMeetingCountdownValidateRejectsSetOnlyFieldsForClose(t *testing.T) {
	runtime := newMeetingCountdownRuntime()
	mustSetMeetingCountdownFlag(t, runtime, "meeting-id", "7651377260537433044")
	mustSetMeetingCountdownFlag(t, runtime, "action", "close")
	mustSetMeetingCountdownFlag(t, runtime, "need-play-audio-at-end", "true")

	err := VCMeetingCountdown.Validate(context.Background(), runtime)
	assertMeetingCountdownValidationError(t, err, "--need-play-audio-at-end")
}

func TestMeetingCountdownDryRun_Set(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, defaultConfig())
	err := mountAndRun(t, VCMeetingCountdown, []string{
		"+meeting-countdown", "--dry-run", "--as", "user",
		"--meeting-id", "7651377260537433044",
		"--action", "set",
		"--duration", "5",
		"--need-play-audio-at-end",
		"--reminders-before-end-in-second", "60,30",
	}, f, stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{
		"/open-apis/vc/v1/meetings/7651377260537433044/countdown",
		"\"action\": \"set\"",
		"\"duration\": 5",
		"\"need_play_audio_at_end\": true",
		"\"reminders_before_end_in_second\": [",
		"60",
		"30",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run output missing %q: %s", want, out)
		}
	}
}

func TestMeetingCountdownDryRun_ValidationErrorEnvelope(t *testing.T) {
	runtime := newMeetingCountdownRuntime()
	mustSetMeetingCountdownFlag(t, runtime, "action", "prolong")

	dryRun := VCMeetingCountdown.DryRun(context.Background(), runtime)
	if got := dryRun.Format(); !strings.Contains(got, "--duration must be a positive number of minutes") {
		t.Fatalf("dry-run error = %v, want duration required", got)
	}
}

func TestMeetingCountdownExecute_Set(t *testing.T) {
	f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
	stub := &httpmock.Stub{
		Method: "POST",
		URL:    buildMeetingCountdownPath("7651377260537433044"),
		Body: map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{},
		},
	}
	reg.Register(stub)

	err := mountAndRun(t, VCMeetingCountdown, []string{
		"+meeting-countdown", "--as", "bot",
		"--format", "pretty",
		"--meeting-id", "7651377260537433044",
		"--action", "set",
		"--duration", "5",
		"--reminders-before-end-in-second", "60,30",
	}, f, stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reg.Verify(t)

	var req map[string]interface{}
	if err := json.Unmarshal(stub.CapturedBody, &req); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if req["action"] != "set" {
		t.Errorf("action = %v, want set", req["action"])
	}
	if req["duration"] != float64(5) {
		t.Errorf("duration = %v, want 5", req["duration"])
	}
	reminders, _ := req["reminders_before_end_in_second"].([]interface{})
	if len(reminders) != 2 || reminders[0] != float64(60) || reminders[1] != float64(30) {
		t.Errorf("reminders_before_end_in_second = %#v, want [60 30]", reminders)
	}
	out := stdout.String()
	for _, want := range []string{
		"Meeting countdown operated.",
		"Action:  set",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package vc

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/shortcuts/common"
)

func TestBuildMeetingEndBody(t *testing.T) {
	tests := []struct {
		name      string
		meetingID string
		wantErr   string
	}{
		{name: "trimmed numeric ID", meetingID: " 69999999 "},
		{name: "missing ID", meetingID: "  ", wantErr: "--meeting-id is required"},
		{name: "non-numeric ID", meetingID: "699a9999", wantErr: "--meeting-id must be numeric"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			cmd.Flags().String("meeting-id", "", "")
			_ = cmd.Flags().Set("meeting-id", tt.meetingID)

			body, err := buildMeetingEndBody(common.TestNewRuntimeContext(cmd, defaultConfig()))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("buildMeetingEndBody() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildMeetingEndBody() error = %v", err)
			}
			if body["meeting_id"] != "69999999" {
				t.Fatalf("body = %#v", body)
			}
		})
	}
}

func TestIsDigits(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{value: "", want: false},
		{value: "69999999", want: true},
		{value: "699a9999", want: false},
	} {
		if got := isDigits(tt.value); got != tt.want {
			t.Fatalf("isDigits(%q) = %t, want %t", tt.value, got, tt.want)
		}
	}
}

func TestMeetingEnd_DryRunAndExecuteValidationFailures(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-id", "", "")
	_ = cmd.Flags().Set("meeting-id", "invalid")
	runtime := common.TestNewRuntimeContext(cmd, defaultConfig())

	if got := VCMeetingEnd.DryRun(context.Background(), runtime).Format(); !strings.Contains(got, "--meeting-id must be numeric") {
		t.Fatalf("dry-run error = %q", got)
	}
	if err := VCMeetingEnd.Execute(context.Background(), runtime); err == nil || !strings.Contains(err.Error(), "--meeting-id must be numeric") {
		t.Fatalf("execute error = %v", err)
	}
}

func TestMeetingEnd_DryRunAndExecutePrettyOutput(t *testing.T) {
	t.Run("dry run", func(t *testing.T) {
		f, stdout, _, _ := cmdutil.TestFactory(t, defaultConfig())
		err := mountAndRun(t, VCMeetingEnd, []string{
			"+meeting-end",
			"--meeting-id", "69999999",
			"--dry-run",
			"--as", "bot",
		}, f, stdout)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, want := range []string{meetingBotEndPath, "69999999"} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("dry-run output missing %q: %s", want, stdout.String())
			}
		}
	})

	t.Run("execute", func(t *testing.T) {
		f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
		stub := &httpmock.Stub{
			Method: http.MethodPost,
			URL:    meetingBotEndPath,
			Body: map[string]interface{}{
				"code": 0,
				"msg":  "ok",
				"data": map[string]interface{}{},
			},
		}
		reg.Register(stub)

		err := mountAndRun(t, VCMeetingEnd, []string{
			"+meeting-end",
			"--meeting-id", " 69999999 ",
			"--format", "pretty",
			"--as", "bot",
		}, f, stdout)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		reg.Verify(t)

		var body map[string]interface{}
		if err := json.Unmarshal(stub.CapturedBody, &body); err != nil {
			t.Fatalf("decode captured request body: %v", err)
		}
		if !reflect.DeepEqual(body, map[string]interface{}{"meeting_id": "69999999"}) {
			t.Fatalf("request body = %#v", body)
		}
		if !strings.Contains(stdout.String(), "Ended meeting 69999999.") {
			t.Fatalf("pretty output = %s", stdout.String())
		}
	})
}

func TestMeetingEnd_ExecuteHandlesAPIErrorAndEmptyData(t *testing.T) {
	t.Run("API error", func(t *testing.T) {
		f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
		reg.Register(&httpmock.Stub{
			Method: http.MethodPost,
			URL:    meetingBotEndPath,
			Body:   map[string]interface{}{"code": 121005, "msg": "no permission"},
		})

		err := mountAndRun(t, VCMeetingEnd, []string{
			"+meeting-end",
			"--meeting-id", "69999999",
			"--as", "bot",
		}, f, stdout)
		if err == nil {
			t.Fatalf("execute error = %v", err)
		}
		reg.Verify(t)
	})

	t.Run("empty data", func(t *testing.T) {
		f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
		reg.Register(&httpmock.Stub{
			Method: http.MethodPost,
			URL:    meetingBotEndPath,
			Body:   map[string]interface{}{"code": 0, "msg": "ok"},
		})

		err := mountAndRun(t, VCMeetingEnd, []string{
			"+meeting-end",
			"--meeting-id", "69999999",
			"--format", "pretty",
			"--as", "bot",
		}, f, stdout)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		reg.Verify(t)
		if !strings.Contains(stdout.String(), "Ended meeting 69999999.") {
			t.Fatalf("pretty output = %s", stdout.String())
		}
	})
}

func TestVCMeetingEndUsesManageScope(t *testing.T) {
	if !reflect.DeepEqual(VCMeetingEnd.Scopes, []string{"vc:meeting.bot.manage:write"}) {
		t.Fatalf("VCMeetingEnd.Scopes = %v", VCMeetingEnd.Scopes)
	}
}

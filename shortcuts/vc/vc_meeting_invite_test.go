// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package vc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/shortcuts/common"
)

func TestVCMeetingInviteNormalizesTypeBeforeEnumValidation(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("type", "", "")
	_ = cmd.Flags().Set("type", " selected ")
	runtime := common.TestNewRuntimeContext(cmd, defaultConfig())

	if err := VCMeetingInvite.Normalize(context.Background(), runtime.FlagContext()); err != nil {
		t.Fatalf("VCMeetingInvite.Normalize() error = %v", err)
	}
	if got := runtime.Str("type"); got != meetingInviteTypeSelected {
		t.Fatalf("normalized type = %q, want %q", got, meetingInviteTypeSelected)
	}
}

func TestBuildMeetingInviteBodySelectedUsesInvitees(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-id", "", "")
	cmd.Flags().String("type", "", "")
	cmd.Flags().StringSlice("open-ids", nil, "")
	_ = cmd.Flags().Set("meeting-id", " 69999999 ")
	_ = cmd.Flags().Set("type", "selected")
	_ = cmd.Flags().Set("open-ids", "ou_a,ou_b,ou_a")

	body, err := buildMeetingInviteBody(common.TestNewRuntimeContext(cmd, defaultConfig()))

	if err != nil {
		t.Fatalf("buildMeetingInviteBody() error = %v", err)
	}
	if body["meeting_id"] != "69999999" || body["invite_type"] != meetingInviteTypeSelectedValue {
		t.Fatalf("body = %#v", body)
	}
	wantInvitees := []map[string]interface{}{
		{"id": "ou_a", "user_type": 1},
		{"id": "ou_b", "user_type": 1},
	}
	if !reflect.DeepEqual(body["invitees"], wantInvitees) {
		t.Fatalf("invitees = %#v", body["invitees"])
	}
	if !reflect.DeepEqual(buildMeetingInviteParams(), map[string]interface{}{"user_id_type": "open_id"}) {
		t.Fatalf("invite params = %#v", buildMeetingInviteParams())
	}
}

func TestBuildMeetingInviteBodyAllSuggestedOmitsOpenIDs(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-id", "", "")
	cmd.Flags().String("type", "", "")
	cmd.Flags().StringSlice("open-ids", nil, "")
	_ = cmd.Flags().Set("meeting-id", "69999999")
	_ = cmd.Flags().Set("type", meetingInviteTypeAllSuggested)

	body, err := buildMeetingInviteBody(common.TestNewRuntimeContext(cmd, defaultConfig()))

	if err != nil {
		t.Fatalf("buildMeetingInviteBody() error = %v", err)
	}
	if body["invite_type"] != meetingInviteTypeAllValue {
		t.Fatalf("invite_type = %#v", body["invite_type"])
	}
	if _, ok := body["invitees"]; ok {
		t.Fatalf("ALL_SUGGESTED body must omit invitees: %#v", body)
	}
}

func TestBuildMeetingInviteBodyRejectsNonUserOpenID(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-id", "", "")
	cmd.Flags().String("type", "", "")
	cmd.Flags().StringSlice("open-ids", nil, "")
	_ = cmd.Flags().Set("meeting-id", "69999999")
	_ = cmd.Flags().Set("type", meetingInviteTypeSelected)
	_ = cmd.Flags().Set("open-ids", "oc_chat")

	_, err := buildMeetingInviteBody(common.TestNewRuntimeContext(cmd, defaultConfig()))

	if err == nil || !strings.Contains(err.Error(), "ou_xxx") {
		t.Fatalf("error = %v, want user open_id validation", err)
	}
}

func TestBuildMeetingInviteBodyRejectsInvalidCombinations(t *testing.T) {
	tooManyOpenIDs := make([]string, meetingInviteeLimit+1)
	for i := range tooManyOpenIDs {
		tooManyOpenIDs[i] = fmt.Sprintf("ou_%d", i)
	}

	tests := []struct {
		name       string
		meetingID  string
		inviteType string
		openIDs    []string
		wantErr    string
	}{
		{
			name:       "missing meeting ID",
			meetingID:  "  ",
			inviteType: meetingInviteTypeSelected,
			openIDs:    []string{"ou_a"},
			wantErr:    "--meeting-id is required",
		},
		{
			name:       "non-numeric meeting ID",
			meetingID:  "6999a999",
			inviteType: meetingInviteTypeSelected,
			openIDs:    []string{"ou_a"},
			wantErr:    "--meeting-id must be numeric",
		},
		{
			name:      "missing type",
			meetingID: "69999999",
			wantErr:   "--type is required",
		},
		{
			name:       "unsupported type",
			meetingID:  "69999999",
			inviteType: "OTHER",
			wantErr:    "ALL_SUGGESTED or SELECTED",
		},
		{
			name:       "selected without open IDs",
			meetingID:  "69999999",
			inviteType: meetingInviteTypeSelected,
			wantErr:    "--open-ids is required",
		},
		{
			name:       "selected with too many open IDs",
			meetingID:  "69999999",
			inviteType: meetingInviteTypeSelected,
			openIDs:    tooManyOpenIDs,
			wantErr:    "at most 200 users",
		},
		{
			name:       "all suggested with open IDs",
			meetingID:  "69999999",
			inviteType: meetingInviteTypeAllSuggested,
			openIDs:    []string{"ou_a"},
			wantErr:    "must not be set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			cmd.Flags().String("meeting-id", "", "")
			cmd.Flags().String("type", "", "")
			cmd.Flags().StringSlice("open-ids", nil, "")
			_ = cmd.Flags().Set("meeting-id", tt.meetingID)
			_ = cmd.Flags().Set("type", tt.inviteType)
			if tt.openIDs != nil {
				_ = cmd.Flags().Set("open-ids", strings.Join(tt.openIDs, ","))
			}

			_, err := buildMeetingInviteBody(common.TestNewRuntimeContext(cmd, defaultConfig()))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("buildMeetingInviteBody() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeMeetingInviteOpenIDs(t *testing.T) {
	got := normalizeMeetingInviteOpenIDs([]string{"", " ou_a ", "ou_a", "ou_b", "  "})
	want := []string{"ou_a", "ou_b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeMeetingInviteOpenIDs() = %#v, want %#v", got, want)
	}
}

func TestMeetingInvite_DryRun(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, defaultConfig())

	err := mountAndRun(t, VCMeetingInvite, []string{
		"+meeting-invite",
		"--meeting-id", "69999999",
		"--type", " selected ",
		"--open-ids", "ou_a,ou_b,ou_a",
		"--dry-run",
		"--as", "bot",
	}, f, stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{meetingBotInvitePath, "user_id_type", "open_id", "69999999", "invite_type"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run output missing %q: %s", want, out)
		}
	}
}

func TestMeetingInvite_ExecuteSelectedPrettyOutput(t *testing.T) {
	f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
	var gotUserIDType string
	stub := &httpmock.Stub{
		Method: http.MethodPost,
		URL:    meetingBotInvitePath,
		OnMatch: func(req *http.Request) {
			gotUserIDType = req.URL.Query().Get("user_id_type")
		},
		Body: map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{
				"failed_count":  1,
				"invited_count": 2,
				"has_more":      true,
				"invite_results": []interface{}{
					map[string]interface{}{"id": "ou_a", "status": 1},
					map[string]interface{}{"id": "ou_b", "status": 2},
				},
			},
		},
	}
	reg.Register(stub)

	err := mountAndRun(t, VCMeetingInvite, []string{
		"+meeting-invite",
		"--meeting-id", "69999999",
		"--type", "selected",
		"--open-ids", "ou_a,ou_b,ou_a",
		"--format", "pretty",
		"--as", "bot",
	}, f, stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reg.Verify(t)

	if gotUserIDType != "open_id" {
		t.Fatalf("user_id_type = %q, want open_id", gotUserIDType)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(stub.CapturedBody, &body); err != nil {
		t.Fatalf("decode captured request body: %v", err)
	}
	if body["meeting_id"] != "69999999" || body["invite_type"] != float64(meetingInviteTypeSelectedValue) {
		t.Fatalf("request body = %#v", body)
	}
	invitees, ok := body["invitees"].([]interface{})
	if !ok || len(invitees) != 2 {
		t.Fatalf("request invitees = %#v, want two deduplicated users", body["invitees"])
	}
	for _, want := range []string{"Invite request sent.", "Failed:   1", "Invited:  2", "Has more: true", "Results:  2 users"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("pretty output missing %q: %s", want, stdout.String())
		}
	}
}

func TestPrintMeetingInviteResultWithoutOptionalFields(t *testing.T) {
	var out strings.Builder
	printMeetingInviteResult(&out, map[string]interface{}{})
	if got, want := out.String(), "Invite request sent.\n"; got != want {
		t.Fatalf("printMeetingInviteResult() = %q, want %q", got, want)
	}
}

func TestMeetingInvite_DryRunAndExecuteValidationFailures(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-id", "", "")
	cmd.Flags().String("type", "", "")
	cmd.Flags().StringSlice("open-ids", nil, "")
	_ = cmd.Flags().Set("meeting-id", "invalid")
	_ = cmd.Flags().Set("type", meetingInviteTypeSelected)
	_ = cmd.Flags().Set("open-ids", "ou_a")
	runtime := common.TestNewRuntimeContext(cmd, defaultConfig())

	if got := VCMeetingInvite.DryRun(context.Background(), runtime).Format(); !strings.Contains(got, "--meeting-id must be numeric") {
		t.Fatalf("dry-run error = %q", got)
	}
	if err := VCMeetingInvite.Execute(context.Background(), runtime); err == nil || !strings.Contains(err.Error(), "--meeting-id must be numeric") {
		t.Fatalf("execute error = %v", err)
	}
}

func TestMeetingInvite_ExecuteHandlesAPIErrorAndEmptyData(t *testing.T) {
	t.Run("API error", func(t *testing.T) {
		f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
		reg.Register(&httpmock.Stub{
			Method: http.MethodPost,
			URL:    meetingBotInvitePath,
			Body:   map[string]interface{}{"code": 121005, "msg": "no permission"},
		})

		err := mountAndRun(t, VCMeetingInvite, []string{
			"+meeting-invite",
			"--meeting-id", "69999999",
			"--type", meetingInviteTypeAllSuggested,
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
			URL:    meetingBotInvitePath,
			Body:   map[string]interface{}{"code": 0, "msg": "ok"},
		})

		err := mountAndRun(t, VCMeetingInvite, []string{
			"+meeting-invite",
			"--meeting-id", "69999999",
			"--type", meetingInviteTypeAllSuggested,
			"--format", "pretty",
			"--as", "bot",
		}, f, stdout)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		reg.Verify(t)
		if !strings.Contains(stdout.String(), "Invite request sent.") {
			t.Fatalf("pretty output = %s", stdout.String())
		}
	})
}

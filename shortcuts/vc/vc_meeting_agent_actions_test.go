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

func TestVCMeetingInviteNormalizeRejectsBlankType(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("type", "", "")
	_ = cmd.Flags().Set("type", " ")

	err := VCMeetingInvite.Normalize(context.Background(), common.TestNewRuntimeContext(cmd, defaultConfig()).FlagContext())
	if err == nil || !strings.Contains(err.Error(), "--type is required") {
		t.Fatalf("Normalize() error = %v, want --type is required", err)
	}
}

func TestBuildMeetingInviteBodySelectedUsesInvitees(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-id", "", "")
	cmd.Flags().String("type", "", "")
	cmd.Flags().StringSlice("open-ids", nil, "")
	_ = cmd.Flags().Set("meeting-id", " 69999999 ")
	_ = cmd.Flags().Set("type", meetingInviteTypeSelected)
	_ = cmd.Flags().Set("open-ids", "ou_a,ou_b,ou_a")

	body := buildMeetingInviteBody(common.TestNewRuntimeContext(cmd, defaultConfig()))
	if body.MeetingID != "69999999" || body.InviteType != meetingInviteTypeSelectedValue {
		t.Fatalf("body = %#v", body)
	}
	wantInvitees := []meetingInvitee{
		{ID: "ou_a", UserType: meetingInviteeUserType},
		{ID: "ou_b", UserType: meetingInviteeUserType},
	}
	if !reflect.DeepEqual(body.Invitees, wantInvitees) {
		t.Fatalf("invitees = %#v", body.Invitees)
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

	body := buildMeetingInviteBody(common.TestNewRuntimeContext(cmd, defaultConfig()))
	if body.InviteType != meetingInviteTypeAllValue {
		t.Fatalf("invite_type = %#v", body.InviteType)
	}
	if len(body.Invitees) != 0 {
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

	err := validateMeetingInviteFlags(common.TestNewRuntimeContext(cmd, defaultConfig()))

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
			wantErr:    "--meeting-id must be a positive integer",
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

			runtime := common.TestNewRuntimeContext(cmd, defaultConfig())
			err := validateMeetingIDFlag(runtime.Str("meeting-id"))
			if err == nil {
				err = validateMeetingInviteFlags(runtime)
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validation error = %v, want %q", err, tt.wantErr)
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
	f, stdout, _, reg := cmdutil.TestFactory(t, defaultConfig())
	apiCalled := false
	reg.Register(&httpmock.Stub{
		Method:   http.MethodPost,
		URL:      meetingBotInvitePath,
		Optional: true,
		OnMatch: func(*http.Request) {
			apiCalled = true
		},
	})

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
	if apiCalled {
		t.Fatal("dry-run must not call the invite API")
	}

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-id", "", "")
	cmd.Flags().String("type", "", "")
	cmd.Flags().StringSlice("open-ids", nil, "")
	_ = cmd.Flags().Set("meeting-id", "69999999")
	_ = cmd.Flags().Set("type", meetingInviteTypeSelected)
	_ = cmd.Flags().Set("open-ids", "ou_a,ou_b")
	raw, marshalErr := json.Marshal(VCMeetingInvite.DryRun(context.Background(), common.TestNewRuntimeContext(cmd, defaultConfig())))
	if marshalErr != nil {
		t.Fatalf("marshal dry-run: %v", marshalErr)
	}
	var payload struct {
		API []struct {
			Method string                 `json:"method"`
			URL    string                 `json:"url"`
			Params map[string]interface{} `json:"params"`
			Body   meetingInviteRequest   `json:"body"`
		} `json:"api"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}
	if len(payload.API) != 1 {
		t.Fatalf("dry-run API count = %d, want 1", len(payload.API))
	}
	call := payload.API[0]
	if call.Method != http.MethodPost || call.URL != meetingBotInvitePath {
		t.Fatalf("dry-run endpoint = %s %s", call.Method, call.URL)
	}
	if call.Params["user_id_type"] != "open_id" {
		t.Fatalf("dry-run params = %#v", call.Params)
	}
	if call.Body.MeetingID != "69999999" || call.Body.InviteType != meetingInviteTypeSelectedValue || len(call.Body.Invitees) != 2 {
		t.Fatalf("dry-run body = %#v", call.Body)
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
	for _, want := range []string{
		"Invite request sent.",
		"Failed:   1",
		"Invited:  2",
		meetingInviteCandidateLimitNotice,
		"ou_a: invited",
		"ou_b: failed",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("pretty output missing %q: %s", want, stdout.String())
		}
	}
}

func TestBuildMeetingInviteOutputReplacesHasMoreWithLimitNotice(t *testing.T) {
	data := map[string]interface{}{
		"failed_count":  1,
		"has_more":      true,
		"invited_count": 2,
	}

	output := buildMeetingInviteOutput(data)
	if _, ok := output["has_more"]; ok {
		t.Fatalf("output must not expose has_more: %#v", output)
	}
	if got := common.GetString(output, "notice"); got != meetingInviteCandidateLimitNotice {
		t.Fatalf("output notice = %q, want %q", got, meetingInviteCandidateLimitNotice)
	}
	if !common.GetBool(data, "has_more") {
		t.Fatalf("source data must not be mutated: %#v", data)
	}
}

func TestPrintMeetingInviteResultWithoutOptionalFields(t *testing.T) {
	var out strings.Builder
	printMeetingInviteResult(&out, map[string]interface{}{})
	if got, want := out.String(), "Invite request sent.\n"; got != want {
		t.Fatalf("printMeetingInviteResult() = %q, want %q", got, want)
	}
}

func TestMeetingInviteValidateRejectsInvalidMeetingID(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-id", "", "")
	cmd.Flags().String("type", "", "")
	cmd.Flags().StringSlice("open-ids", nil, "")
	_ = cmd.Flags().Set("meeting-id", "invalid")
	_ = cmd.Flags().Set("type", meetingInviteTypeSelected)
	_ = cmd.Flags().Set("open-ids", "ou_a")
	runtime := common.TestNewRuntimeContext(cmd, defaultConfig())

	if err := VCMeetingInvite.Validate(context.Background(), runtime); err == nil || !strings.Contains(err.Error(), "--meeting-id must be a positive integer") {
		t.Fatalf("validate error = %v", err)
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

func TestBuildMeetingEndBody(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-id", "", "")
	_ = cmd.Flags().Set("meeting-id", " 69999999 ")

	body := buildMeetingEndBody(common.TestNewRuntimeContext(cmd, defaultConfig()))
	if body.MeetingID != "69999999" {
		t.Fatalf("body = %#v", body)
	}
}

func TestValidateMeetingIDFlag(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "positive integer", value: "69999999"},
		{name: "leading zero positive integer", value: "0001"},
		{name: "missing", value: " ", wantErr: "--meeting-id is required"},
		{name: "zero", value: "0", wantErr: "--meeting-id must be a positive integer"},
		{name: "non-numeric", value: "699a9999", wantErr: "--meeting-id must be a positive integer"},
		{name: "overflow", value: "9223372036854775808", wantErr: "--meeting-id is out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMeetingIDFlag(tt.value)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateMeetingIDFlag(%q) error = %v", tt.value, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateMeetingIDFlag(%q) error = %v, want %q", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestMeetingEndValidateRejectsInvalidMeetingID(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-id", "", "")
	_ = cmd.Flags().Set("meeting-id", "invalid")
	runtime := common.TestNewRuntimeContext(cmd, defaultConfig())

	if err := VCMeetingEnd.Validate(context.Background(), runtime); err == nil || !strings.Contains(err.Error(), "--meeting-id must be a positive integer") {
		t.Fatalf("validate error = %v", err)
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

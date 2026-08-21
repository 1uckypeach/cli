// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package vc

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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

func TestVCMeetingEndUsesManageScope(t *testing.T) {
	if !reflect.DeepEqual(VCMeetingEnd.Scopes, []string{"vc:meeting.bot.manage:write"}) {
		t.Fatalf("VCMeetingEnd.Scopes = %v", VCMeetingEnd.Scopes)
	}
}

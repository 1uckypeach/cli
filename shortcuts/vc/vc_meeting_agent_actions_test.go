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

func TestBuildMeetingStartBodyUsesTypedAction(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("meeting-number", "", "")
	cmd.Flags().String("password", "", "")
	cmd.Flags().String("call-id", "", "")
	_ = cmd.Flags().Set("meeting-number", "123456789")

	body, err := buildMeetingStartBody(common.TestNewRuntimeContext(cmd, defaultConfig()))

	if err != nil {
		t.Fatalf("buildMeetingStartBody() error = %v", err)
	}
	if body["action"] != 2 {
		t.Fatalf("action = %#v, want START_AND_JOIN(2)", body["action"])
	}
	if _, ok := body["meeting_action"]; ok {
		t.Fatalf("body must not contain legacy meeting_action: %#v", body)
	}
}

func TestBuildMeetingInviteBodySelectedUsesOpenIDs(t *testing.T) {
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
	if body["meeting_id"] != "69999999" || body["type"] != meetingInviteTypeSelectedValue {
		t.Fatalf("body = %#v", body)
	}
	if !reflect.DeepEqual(body["open_ids"], []string{"ou_a", "ou_b"}) {
		t.Fatalf("open_ids = %#v", body["open_ids"])
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
	if body["type"] != meetingInviteTypeAllValue {
		t.Fatalf("type = %#v", body["type"])
	}
	if _, ok := body["open_ids"]; ok {
		t.Fatalf("ALL_SUGGESTED body must omit open_ids: %#v", body)
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

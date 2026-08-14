// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package mail

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/shortcuts/common"
)

func TestMailSenderListShortcuts_Metadata(t *testing.T) {
	if MailSenderAllowlist.Command != "+sender-allowlist" {
		t.Fatalf("allowlist command = %q", MailSenderAllowlist.Command)
	}
	if MailSenderBlocklist.Command != "+sender-blocklist" {
		t.Fatalf("blocklist command = %q", MailSenderBlocklist.Command)
	}
	if MailSenderAllowlist.Risk != "write" || MailSenderBlocklist.Risk != "write" {
		t.Fatalf("risk = %q/%q, want write", MailSenderAllowlist.Risk, MailSenderBlocklist.Risk)
	}
	if MailSenderAllowlist.Scopes[0] != "mail:user_mailbox:readonly" {
		t.Fatalf("read scope = %v", MailSenderAllowlist.Scopes)
	}
	if MailSenderAllowlist.ConditionalScopes[0] != "mail:user_mailbox" {
		t.Fatalf("conditional write scope = %v", MailSenderAllowlist.ConditionalScopes)
	}
	for _, shortcut := range []struct {
		name  string
		flags []common.Flag
	}{
		{name: MailSenderAllowlist.Command, flags: MailSenderAllowlist.Flags},
		{name: MailSenderBlocklist.Command, flags: MailSenderBlocklist.Flags},
	} {
		for _, flag := range shortcut.flags {
			if flag.Name == "yes" {
				t.Fatalf("%s must not register shortcut-specific --yes flag", shortcut.name)
			}
		}
	}
}

func TestMailSenderListShortcut_DefaultListsOrSearches(t *testing.T) {
	f, stdout, _, reg := mailShortcutTestFactory(t)
	stub := &httpmock.Stub{
		Method: "GET",
		URL:    "/user_mailboxes/me/allow_senders?keyword=fixture&page_size=20",
		Body: map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"has_more": false,
				"items": []map[string]interface{}{
					{"sender": "fixture.one@sender.test"},
					{"sender": "fixture.two@sender.test"},
				},
			},
		},
	}
	reg.Register(stub)

	err := runMountedMailShortcut(t, MailSenderAllowlist, []string{"+sender-allowlist", "--query", "fixture"}, f, stdout)
	if err != nil {
		t.Fatalf("execute shortcut: %v", err)
	}
	data := decodeShortcutEnvelopeData(t, stdout)
	items, ok := data["items"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("items = %#v, want 2 items", data["items"])
	}
	reg.Verify(t)
}

func TestMailSenderListShortcut_AddBuildsItemsBody(t *testing.T) {
	f, stdout, _, reg := mailShortcutTestFactory(t)
	stub := &httpmock.Stub{
		Method: "POST",
		URL:    "/user_mailboxes/me/blocked_senders/batch_create",
		Body: map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{"failed_items": []interface{}{}},
		},
	}
	reg.Register(stub)

	err := runMountedMailShortcut(t, MailSenderBlocklist, []string{
		"+sender-blocklist",
		"--add", "bad.example,spam.example",
		"--type", "domain",
	}, f, stdout)
	if err != nil {
		t.Fatalf("execute shortcut: %v", err)
	}
	body := decodeCapturedSenderListJSONBody(t, stub)
	items, ok := body["items"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("items = %#v, want two item objects", body["items"])
	}
	first := items[0].(map[string]interface{})
	if first["sender"] != "bad.example" || first["sender_type"].(float64) != 2 {
		t.Fatalf("first item = %#v", first)
	}
}

func TestMailSenderListShortcut_RemoveBuildsSendersBody(t *testing.T) {
	f, stdout, _, reg := mailShortcutTestFactory(t)
	stub := &httpmock.Stub{
		Method: "POST",
		URL:    "/user_mailboxes/me/allow_senders/batch_remove",
		Body: map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{"deleted_count": 2},
		},
	}
	reg.Register(stub)

	err := runMountedMailShortcut(t, MailSenderAllowlist, []string{
		"+sender-allowlist",
		"--remove", "fixture.one@sender.test",
		"--remove", "fixture.two@sender.test",
	}, f, stdout)
	if err != nil {
		t.Fatalf("execute shortcut: %v", err)
	}
	body := decodeCapturedSenderListJSONBody(t, stub)
	senders, ok := body["senders"].([]interface{})
	if !ok || len(senders) != 2 {
		t.Fatalf("senders = %#v, want two senders", body["senders"])
	}
}

func TestMailSenderListShortcut_ValidateAddresses(t *testing.T) {
	f, stdout, _, _ := mailShortcutTestFactory(t)
	err := runMountedMailShortcut(t, MailSenderAllowlist, []string{
		"+sender-allowlist",
		"--add", "valid@example.com,",
	}, f, stdout)
	requireSenderListValidationParam(t, err, "--add")

	err = runMountedMailShortcut(t, MailSenderAllowlist, []string{
		"+sender-allowlist",
		"--add", "a@example.com",
		"--remove", "b@example.com",
	}, f, stdout)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v, want mutually exclusive", err)
	}
}

func decodeCapturedSenderListJSONBody(t *testing.T, stub *httpmock.Stub) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(stub.CapturedBody, &body); err != nil {
		t.Fatalf("decode captured body: %v\n%s", err, string(stub.CapturedBody))
	}
	return body
}

func requireSenderListValidationParam(t *testing.T, err error, param string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error for %s", param)
	}
	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %T", err)
	}
	if validationErr.Param != param {
		t.Fatalf("param = %q, want %q", validationErr.Param, param)
	}
}

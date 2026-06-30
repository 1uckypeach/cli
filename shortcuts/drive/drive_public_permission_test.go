// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package drive

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/httpmock"
)

func TestResolveDrivePublicPermissionTarget_BareTokenNeedsType(t *testing.T) {
	t.Parallel()

	_, _, err := resolveDrivePublicPermissionTarget("doxTok123", "")
	assertDrivePublicPermissionValidationError(t, err, "--type", "--type is required")
}

func TestResolveDrivePublicPermissionTarget_URLInference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		wantTok  string
		wantType string
	}{
		{"docx", "https://example.feishu.cn/docx/doxTok123?from=share", "doxTok123", "docx"},
		{"docx trailing path", "https://example.feishu.cn/docx/doxTok123/extra/path?from=share", "doxTok123", "docx"},
		{"sheet", "https://example.feishu.cn/sheets/shtTok456?sheet=abc", "shtTok456", "sheet"},
		{"base", "https://example.feishu.cn/base/bscTok789", "bscTok789", "bitable"},
		{"file", "https://example.feishu.cn/file/boxTok111", "boxTok111", "file"},
		{"wiki", "https://example.feishu.cn/wiki/wikTok222", "wikTok222", "wiki"},
		{"legacy doc", "https://example.feishu.cn/doc/docTok333", "docTok333", "doc"},
		{"mindnote", "https://example.feishu.cn/mindnotes/mnTok444", "mnTok444", "mindnote"},
		{"minutes", "https://example.feishu.cn/minutes/obcnTok555", "obcnTok555", "minutes"},
		{"slides", "https://example.feishu.cn/slides/slTok666", "slTok666", "slides"},
	}
	for _, temp := range tests {
		tt := temp
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token, docType, err := resolveDrivePublicPermissionTarget(tt.raw, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token != tt.wantTok || docType != tt.wantType {
				t.Fatalf("got target (%q, %q), want (%q, %q)", token, docType, tt.wantTok, tt.wantType)
			}
		})
	}
}

func TestResolveDrivePublicPermissionTarget_ExplicitTypeOverridesURL(t *testing.T) {
	t.Parallel()

	token, docType, err := resolveDrivePublicPermissionTarget("https://example.feishu.cn/docx/doxTok123", "wiki")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "doxTok123" || docType != "wiki" {
		t.Fatalf("got target (%q, %q), want (doxTok123, wiki)", token, docType)
	}
}

func TestResolveDrivePublicPermissionTarget_RejectsMarkerOutsidePath(t *testing.T) {
	t.Parallel()

	tests := []string{
		"https://example.feishu.cn/share?redirect=/docx/doxTok123",
		"https://example.feishu.cn/share#/docx/doxTok123",
		"https://example.feishu.cn/space/docx/doxTok123",
		"https://example.feishu.cn/foo/bitable/bscTok789",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			_, _, err := resolveDrivePublicPermissionTarget(raw, "")
			assertDrivePublicPermissionValidationError(t, err, "--token", "could not infer token from URL")
		})
	}
}

func TestDrivePublicPermissionUpdate_DryRun(t *testing.T) {
	t.Parallel()

	f, stdout, _, _ := cmdutil.TestFactory(t, driveTestConfig())
	err := mountAndRunDrive(t, DrivePublicPermissionUpdate, []string{
		"+public-permission-update",
		"--token", "https://example.feishu.cn/docx/doxTok123?from=share",
		"--external-access-entity", "open",
		"--link-share-entity", "anyone_readable",
		"--perm-type", "single_page",
		"--dry-run", "--as", "user",
	}, f, stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{
		"/open-apis/drive/v2/permissions/doxTok123/public",
		`"PATCH"`,
		`"type": "docx"`,
		`"external_access_entity": "open"`,
		`"link_share_entity": "anyone_readable"`,
		`"perm_type": "single_page"`,
		`"` + "to" + `ken": "doxTok123"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, out)
		}
	}
}

func TestDrivePublicPermissionUpdate_ValidateRejectsNoBodyFields(t *testing.T) {
	t.Parallel()

	f, stdout, _, _ := cmdutil.TestFactory(t, driveTestConfig())
	err := mountAndRunDrive(t, DrivePublicPermissionUpdate, []string{
		"+public-permission-update",
		"--token", "doxTok123",
		"--type", "docx",
		"--as", "user",
	}, f, stdout)
	assertDrivePublicPermissionValidationError(t, err, "", "nothing to update")
}

func TestDrivePublicPermissionUpdate_ValidateRejectsPermTypeOnly(t *testing.T) {
	t.Parallel()

	f, stdout, _, _ := cmdutil.TestFactory(t, driveTestConfig())
	err := mountAndRunDrive(t, DrivePublicPermissionUpdate, []string{
		"+public-permission-update",
		"--token", "doxTok123",
		"--type", "docx",
		"--perm-type", "single_page",
		"--as", "user",
	}, f, stdout)
	assertDrivePublicPermissionValidationError(t, err, "", "nothing to update")
}

func TestDrivePublicPermissionUpdate_ValidateRejectsSinglePageForUnsupportedField(t *testing.T) {
	t.Parallel()

	f, stdout, _, _ := cmdutil.TestFactory(t, driveTestConfig())
	err := mountAndRunDrive(t, DrivePublicPermissionUpdate, []string{
		"+public-permission-update",
		"--token", "doxTok123",
		"--type", "docx",
		"--security-entity", "only_full_access",
		"--perm-type", "single_page",
		"--as", "user",
	}, f, stdout)
	assertDrivePublicPermissionValidationError(t, err, "--perm-type", "--perm-type single_page")
}

func TestDrivePublicPermissionUpdate_ValidateRejectsSinglePageMixedFields(t *testing.T) {
	t.Parallel()

	f, stdout, _, _ := cmdutil.TestFactory(t, driveTestConfig())
	err := mountAndRunDrive(t, DrivePublicPermissionUpdate, []string{
		"+public-permission-update",
		"--token", "doxTok123",
		"--type", "docx",
		"--link-share-entity", "closed",
		"--copy-entity", "only_full_access",
		"--perm-type", "single_page",
		"--as", "user",
	}, f, stdout)
	assertDrivePublicPermissionValidationError(t, err, "--perm-type", "remove --copy-entity")
}

func TestValidateExternalLinkShareCombo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		external string
		link     string
		wantErr  bool
		contains string
	}{
		// Both empty: no validation needed.
		{"both empty", "", "", false, ""},
		// Only one set: no combo to check.
		{"only external", "open", "", false, ""},
		{"only link", "", "anyone_readable", false, ""},

		// external=open
		{"open + anyone_readable", "open", "anyone_readable", false, ""},
		{"open + tenant_readable", "open", "tenant_readable", false, ""},
		{"open + closed", "open", "closed", false, ""},
		{"open + partner_tenant_readable", "open", "partner_tenant_readable", false, ""},

		// external=allow_share_partner_tenant
		{"partner + partner_tenant_readable", "allow_share_partner_tenant", "partner_tenant_readable", false, ""},
		{"partner + tenant_readable", "allow_share_partner_tenant", "tenant_readable", false, ""},
		{"partner + closed", "allow_share_partner_tenant", "closed", false, ""},
		{"partner + anyone_readable", "allow_share_partner_tenant", "anyone_readable", true, "anyone_* requires external=open"},

		// external=closed
		{"closed + tenant_readable", "closed", "tenant_readable", false, ""},
		{"closed + closed", "closed", "closed", false, ""},
		{"closed + anyone_readable", "closed", "anyone_readable", true, "only tenant_* or closed are allowed"},
		{"closed + partner_tenant_readable", "closed", "partner_tenant_readable", true, "only tenant_* or closed are allowed"},
	}
	for _, temp := range tests {
		tt := temp
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateExternalLinkShareCombo(tt.external, tt.link)
			if tt.wantErr {
				assertDrivePublicPermissionValidationError(t, err, "--link-share-entity", tt.contains)
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDrivePublicPermissionUpdate_ValidateRejectsExternalLinkCombo(t *testing.T) {
	t.Parallel()

	f, stdout, _, _ := cmdutil.TestFactory(t, driveTestConfig())
	err := mountAndRunDrive(t, DrivePublicPermissionUpdate, []string{
		"+public-permission-update",
		"--token", "doxTok123",
		"--type", "docx",
		"--external-access-entity", "closed",
		"--link-share-entity", "anyone_readable",
		"--as", "user",
	}, f, stdout)
	assertDrivePublicPermissionValidationError(t, err, "--link-share-entity", "only tenant_* or closed are allowed")
}

func TestDrivePublicPermissionUpdate_HighRiskRequiresYes(t *testing.T) {
	t.Parallel()

	if DrivePublicPermissionUpdate.Risk != "high-risk-write" {
		t.Fatalf("Risk = %q, want high-risk-write", DrivePublicPermissionUpdate.Risk)
	}
	f, stdout, _, _ := cmdutil.TestFactory(t, driveTestConfig())
	err := mountAndRunDrive(t, DrivePublicPermissionUpdate, []string{
		"+public-permission-update",
		"--token", "doxTok123",
		"--type", "docx",
		"--link-share-entity", "closed",
		"--as", "user",
	}, f, stdout)
	if err == nil || !strings.Contains(err.Error(), "requires confirmation") {
		t.Fatalf("expected confirmation error, got: %v", err)
	}
}

func TestDrivePublicPermissionUpdate_ExecuteSuccess(t *testing.T) {
	f, stdout, _, reg := cmdutil.TestFactory(t, driveTestConfig())
	stub := &httpmock.Stub{
		Method: "PATCH",
		URL:    "/open-apis/drive/v2/permissions/doxTok123/public?type=docx",
		Body: map[string]interface{}{
			"code": 0, "msg": "success",
			"data": map[string]interface{}{
				"permission_public": map[string]interface{}{
					"link_share_entity": "closed",
				},
			},
		},
	}
	reg.Register(stub)

	err := mountAndRunDrive(t, DrivePublicPermissionUpdate, []string{
		"+public-permission-update",
		"--token", "doxTok123",
		"--type", "docx",
		"--link-share-entity", "closed",
		"--copy-entity", "only_full_access",
		"--yes",
		"--as", "user",
	}, f, stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(stub.CapturedBody, &body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if body["link_share_entity"] != "closed" || body["copy_entity"] != "only_full_access" {
		t.Fatalf("unexpected request body: %#v", body)
	}
	if _, ok := body["permission_public"]; ok {
		t.Fatalf("body must be flat, got nested permission_public: %#v", body)
	}
}

func assertDrivePublicPermissionValidationError(t *testing.T, err error, wantParam, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("expected typed error, got %T: %v", err, err)
	}
	if p.Category != errs.CategoryValidation {
		t.Fatalf("category = %q, want %q; err=%v", p.Category, errs.CategoryValidation, err)
	}
	if p.Subtype != errs.SubtypeInvalidArgument {
		t.Fatalf("subtype = %q, want %q; err=%v", p.Subtype, errs.SubtypeInvalidArgument, err)
	}
	if wantMessage != "" && !strings.Contains(err.Error(), wantMessage) {
		t.Fatalf("error %q does not contain %q", err.Error(), wantMessage)
	}
	var validationErr *errs.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *errs.ValidationError, got %T: %v", err, err)
	}
	if wantParam != "" && validationErr.Param != wantParam {
		t.Fatalf("param = %q, want %q; err=%v", validationErr.Param, wantParam, err)
	}
}

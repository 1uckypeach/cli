// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package drive

import (
	"context"
	"strings"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestDrive_PublicPermissionUpdateDryRun(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "app")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

	tests := []struct {
		name     string
		args     []string
		wantURL  string
		wantType string
		assert   func(t *testing.T, out string)
	}{
		{
			name: "URL input infers type and sends v2 fields",
			args: []string{
				"drive", "+public-permission-update",
				"--token", "https://example.feishu.cn/docx/doxcnE2E001?from=share",
				"--external-access-entity", "open",
				"--link-share-entity", "anyone_readable",
				"--perm-type", "single_page",
				"--dry-run",
			},
			wantURL:  "/open-apis/drive/v2/permissions/doxcnE2E001/public",
			wantType: "docx",
			assert: func(t *testing.T, out string) {
				if got := gjson.Get(out, "api.0.body.external_access_entity").String(); got != "open" {
					t.Fatalf("body.external_access_entity = %q, want open\nstdout:\n%s", got, out)
				}
				if got := gjson.Get(out, "api.0.body.link_share_entity").String(); got != "anyone_readable" {
					t.Fatalf("body.link_share_entity = %q, want anyone_readable\nstdout:\n%s", got, out)
				}
				if got := gjson.Get(out, "api.0.body.perm_type").String(); got != "single_page" {
					t.Fatalf("body.perm_type = %q, want single_page\nstdout:\n%s", got, out)
				}
			},
		},
		{
			name: "bare token requires explicit type and sends flat body",
			args: []string{
				"drive", "+public-permission-update",
				"--token", "shtcnE2E002",
				"--type", "sheet",
				"--security-entity", "only_full_access",
				"--comment-entity", "anyone_can_edit",
				"--manage-collaborator-entity", "collaborator_full_access",
				"--copy-entity", "anyone_can_view",
				"--dry-run",
			},
			wantURL:  "/open-apis/drive/v2/permissions/shtcnE2E002/public",
			wantType: "sheet",
			assert: func(t *testing.T, out string) {
				if got := gjson.Get(out, "api.0.body.security_entity").String(); got != "only_full_access" {
					t.Fatalf("body.security_entity = %q, want only_full_access\nstdout:\n%s", got, out)
				}
				if got := gjson.Get(out, "api.0.body.comment_entity").String(); got != "anyone_can_edit" {
					t.Fatalf("body.comment_entity = %q, want anyone_can_edit\nstdout:\n%s", got, out)
				}
				if gjson.Get(out, "api.0.body.permission_public").Exists() {
					t.Fatalf("body must be flat, stdout:\n%s", out)
				}
			},
		},
	}

	for _, temp := range tests {
		tt := temp
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			t.Cleanup(cancel)

			result, err := clie2e.RunCmd(ctx, clie2e.Request{
				Args:      tt.args,
				DefaultAs: "user",
			})
			require.NoError(t, err)
			result.AssertExitCode(t, 0)

			out := result.Stdout
			if got := gjson.Get(out, "api.0.method").String(); got != "PATCH" {
				t.Fatalf("method = %q, want PATCH\nstdout:\n%s", got, out)
			}
			if got := gjson.Get(out, "api.0.url").String(); got != tt.wantURL {
				t.Fatalf("url = %q, want %q\nstdout:\n%s", got, tt.wantURL, out)
			}
			if got := gjson.Get(out, "api.0.params.type").String(); got != tt.wantType {
				t.Fatalf("params.type = %q, want %q\nstdout:\n%s", got, tt.wantType, out)
			}
			tt.assert(t, out)
		})
	}
}

func TestDrive_PublicPermissionUpdateDryRunRejectsMissingTypeForBareToken(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "app")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"drive", "+public-permission-update",
			"--token", "doxcnE2E999",
			"--link-share-entity", "closed",
			"--dry-run",
		},
		DefaultAs: "user",
	})
	require.NoError(t, err)
	requireDrivePublicPermissionValidationEnvelope(t, result, "--type", "--type is required")
}

func TestDrive_PublicPermissionUpdateDryRunRejectsSinglePageMixedFields(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "app")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"drive", "+public-permission-update",
			"--token", "doxcnE2E999",
			"--type", "docx",
			"--link-share-entity", "closed",
			"--copy-entity", "only_full_access",
			"--perm-type", "single_page",
			"--dry-run",
		},
		DefaultAs: "user",
	})
	require.NoError(t, err)
	requireDrivePublicPermissionValidationEnvelope(t, result, "--perm-type", "remove --copy-entity")
}

func requireDrivePublicPermissionValidationEnvelope(t *testing.T, result *clie2e.Result, wantParam, wantMessage string) {
	t.Helper()
	if result.ExitCode == 0 {
		t.Fatalf("command must be rejected, stdout:\n%s", result.Stdout)
	}
	if got := gjson.Get(result.Stderr, "error.type").String(); got != "validation" {
		t.Fatalf("error.type = %q, want validation\nstdout:\n%s\nstderr:\n%s", got, result.Stdout, result.Stderr)
	}
	if got := gjson.Get(result.Stderr, "error.subtype").String(); got != "invalid_argument" {
		t.Fatalf("error.subtype = %q, want invalid_argument\nstdout:\n%s\nstderr:\n%s", got, result.Stdout, result.Stderr)
	}
	if got := gjson.Get(result.Stderr, "error.param").String(); got != wantParam {
		t.Fatalf("error.param = %q, want %q\nstdout:\n%s\nstderr:\n%s", got, wantParam, result.Stdout, result.Stderr)
	}
	message := gjson.Get(result.Stderr, "error.message").String()
	if !strings.Contains(message, wantMessage) {
		t.Fatalf("error.message %q does not contain %q\nstdout:\n%s\nstderr:\n%s", message, wantMessage, result.Stdout, result.Stderr)
	}
}

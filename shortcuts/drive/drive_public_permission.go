// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package drive

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

const drivePublicPermissionScope = "docs:permission.setting:write_only"

var drivePublicPermissionTypes = []string{
	"doc", "sheet", "file", "wiki", "bitable", "docx",
	"mindnote", "minutes", "slides",
}

var drivePublicPermissionURLPathToType = []struct {
	Prefix string
	Type   string
}{
	{"/mindnotes/", "mindnote"},
	{"/bitable/", "bitable"},
	{"/sheets/", "sheet"},
	{"/minutes/", "minutes"},
	{"/slides/", "slides"},
	{"/docx/", "docx"},
	{"/wiki/", "wiki"},
	{"/base/", "bitable"},
	{"/file/", "file"},
	{"/doc/", "doc"},
}

var (
	drivePublicPermissionSecurityEntities = []string{"anyone_can_view", "anyone_can_edit", "only_full_access"}
	drivePublicPermissionCommentEntities  = []string{"anyone_can_view", "anyone_can_edit"}
	drivePublicPermissionShareEntities    = []string{"anyone", "same_tenant"}
	drivePublicPermissionManageEntities   = []string{"collaborator_can_view", "collaborator_can_edit", "collaborator_full_access"}
	drivePublicPermissionLinkEntities     = []string{
		"tenant_readable", "tenant_editable",
		"anyone_readable", "anyone_editable",
		"partner_tenant_readable", "partner_tenant_editable",
		"closed",
	}
	drivePublicPermissionCopyEntities     = drivePublicPermissionSecurityEntities
	drivePublicPermissionExternalEntities = []string{"open", "closed", "allow_share_partner_tenant"}
	drivePublicPermissionPermTypes        = []string{"container", "single_page"}
)

var drivePublicPermissionBodyFlags = []struct {
	Flag string
	JSON string
}{
	{"security-entity", "security_entity"},
	{"comment-entity", "comment_entity"},
	{"share-entity", "share_entity"},
	{"manage-collaborator-entity", "manage_collaborator_entity"},
	{"link-share-entity", "link_share_entity"},
	{"copy-entity", "copy_entity"},
	{"external-access-entity", "external_access_entity"},
}

// DrivePublicPermissionUpdate updates public permission settings using the
// drive/v2 public-permission PATCH endpoint.
var DrivePublicPermissionUpdate = common.Shortcut{
	Service:     "drive",
	Command:     "+public-permission-update",
	Description: "Update public permission settings on a Drive document or file",
	Risk:        "high-risk-write",
	Scopes:      []string{drivePublicPermissionScope},
	AuthTypes:   []string{"user", "bot"},
	Flags: []common.Flag{
		{Name: "token", Desc: "target token or document URL (docx/sheets/base/file/wiki/doc/mindnotes/minutes/slides)", Required: true},
		{Name: "type", Desc: "target type; auto-inferred from URL when omitted", Enum: drivePublicPermissionTypes},
		{Name: "security-entity", Desc: "who can copy, duplicate, print, and download", Enum: drivePublicPermissionSecurityEntities},
		{Name: "comment-entity", Desc: "who can comment", Enum: drivePublicPermissionCommentEntities},
		{Name: "share-entity", Desc: "who can view, add, or remove collaborators at the org level", Enum: drivePublicPermissionShareEntities},
		{Name: "manage-collaborator-entity", Desc: "who can manage collaborators", Enum: drivePublicPermissionManageEntities},
		{Name: "link-share-entity", Desc: "link sharing setting", Enum: drivePublicPermissionLinkEntities},
		{Name: "copy-entity", Desc: "who can create copies", Enum: drivePublicPermissionCopyEntities},
		{Name: "external-access-entity", Desc: "external sharing setting", Enum: drivePublicPermissionExternalEntities},
		{Name: "perm-type", Desc: "permission scope for link/external access changes", Enum: drivePublicPermissionPermTypes},
	},
	Tips: []string{
		"Calls PATCH /open-apis/drive/v2/permissions/:token/public; use --dry-run first to inspect the exact body.",
		"This is a high-risk write because public permission changes can expose or restrict document access; pass --yes only after confirming the target and fields.",
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		_, _, err := resolveDrivePublicPermissionTarget(runtime.Str("token"), runtime.Str("type"))
		if err != nil {
			return err
		}
		if err := validateDrivePublicPermissionBody(runtime); err != nil {
			return err
		}
		return nil
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		token, docType, err := resolveDrivePublicPermissionTarget(runtime.Str("token"), runtime.Str("type"))
		if err != nil {
			return common.NewDryRunAPI().Set("error", err.Error())
		}
		if err := validateDrivePublicPermissionBody(runtime); err != nil {
			return common.NewDryRunAPI().Set("error", err.Error())
		}
		return common.NewDryRunAPI().
			Desc("Update Drive public permission settings").
			PATCH("/open-apis/drive/v2/permissions/:token/public").
			Params(map[string]interface{}{"type": docType}).
			Body(buildDrivePublicPermissionBody(runtime)).
			Set("token", token)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		token, docType, err := resolveDrivePublicPermissionTarget(runtime.Str("token"), runtime.Str("type"))
		if err != nil {
			return err
		}
		body := buildDrivePublicPermissionBody(runtime)

		fmt.Fprintf(runtime.IO().ErrOut, "Updating public permission settings on %s %s...\n",
			docType, common.MaskToken(token))

		data, err := runtime.CallAPITyped("PATCH",
			fmt.Sprintf("/open-apis/drive/v2/permissions/%s/public", validate.EncodePathSegment(token)),
			map[string]interface{}{"type": docType},
			body,
		)
		if err != nil {
			return err
		}
		runtime.Out(data, nil)
		return nil
	},
}

func resolveDrivePublicPermissionTarget(raw, explicitType string) (resourceID, docType string, err error) {
	raw = strings.TrimSpace(raw)
	explicitType = strings.TrimSpace(explicitType)
	if raw == "" {
		return "", "", errs.NewValidationError(errs.SubtypeInvalidArgument, "--token is required").WithParam("--token")
	}

	if strings.Contains(raw, "://") {
		parsed, parseErr := url.Parse(raw)
		if parseErr != nil {
			return "", "", errs.NewValidationError(errs.SubtypeInvalidArgument,
				"invalid URL %q: %v",
				raw,
				parseErr,
			).WithParam("--token").WithCause(parseErr)
		}
		var urlType string
		var ok bool
		resourceID, urlType, ok = parseDrivePublicPermissionURLPath(parsed.Path)
		if resourceID == "" {
			return "", "", errs.NewValidationError(errs.SubtypeInvalidArgument,
				"could not infer token from URL %q: supported paths are /docx/, /sheets/, /base/, /bitable/, /file/, /wiki/, /doc/, /mindnotes/, /minutes/, /slides/. Pass a bare token with --type instead if the URL shape is unusual",
				raw,
			).WithParam("--token")
		}
		if ok && explicitType == "" {
			docType = urlType
		}
	} else {
		resourceID = raw
	}

	if explicitType != "" {
		docType = explicitType
	}
	if docType == "" {
		return "", "", errs.NewValidationError(errs.SubtypeInvalidArgument,
			"--type is required when --token is a bare token; accepted values: %s",
			strings.Join(drivePublicPermissionTypes, ", "),
		).WithParam("--type")
	}
	if err := validate.ResourceName(resourceID, "--token"); err != nil {
		return "", "", errs.NewValidationError(errs.SubtypeInvalidArgument, "%s", err).WithParam("--token").WithCause(err)
	}
	return resourceID, docType, nil
}

func parseDrivePublicPermissionURLPath(path string) (resourceID, docType string, ok bool) {
	for _, mapping := range drivePublicPermissionURLPathToType {
		if !strings.HasPrefix(path, mapping.Prefix) {
			continue
		}
		candidate := path[len(mapping.Prefix):]
		candidate = strings.TrimRight(candidate, "/")
		if idx := strings.IndexByte(candidate, '/'); idx >= 0 {
			candidate = candidate[:idx]
		}
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return "", "", false
		}
		return candidate, mapping.Type, true
	}
	return "", "", false
}

func validateDrivePublicPermissionBody(runtime *common.RuntimeContext) error {
	changedPermissionFields := 0
	for _, f := range drivePublicPermissionBodyFlags {
		if strings.TrimSpace(runtime.Str(f.Flag)) != "" {
			changedPermissionFields++
		}
	}
	if changedPermissionFields == 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"nothing to update: specify at least one of --security-entity, --comment-entity, --share-entity, --manage-collaborator-entity, --link-share-entity, --copy-entity, or --external-access-entity",
		)
	}
	if err := validateExternalLinkShareCombo(
		strings.TrimSpace(runtime.Str("external-access-entity")),
		strings.TrimSpace(runtime.Str("link-share-entity")),
	); err != nil {
		return err
	}
	if runtime.Str("perm-type") == "single_page" {
		hasLinkOrExternal := strings.TrimSpace(runtime.Str("link-share-entity")) != "" ||
			strings.TrimSpace(runtime.Str("external-access-entity")) != ""
		if !hasLinkOrExternal {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"--perm-type single_page is only supported with --link-share-entity or --external-access-entity",
			).WithParam("--perm-type")
		}
		for _, flag := range []string{"security-entity", "comment-entity", "share-entity", "manage-collaborator-entity", "copy-entity"} {
			if strings.TrimSpace(runtime.Str(flag)) != "" {
				return errs.NewValidationError(errs.SubtypeInvalidArgument,
					"--perm-type single_page only supports --link-share-entity and --external-access-entity; remove --%s",
					flag,
				).WithParam("--perm-type")
			}
		}
	}
	return nil
}

// validateExternalLinkShareCombo checks that link_share_entity does not exceed
// the external sharing boundary set by external_access_entity.
//
//	external=open                  → ok: anyone_*, partner_tenant_*, tenant_*, closed
//	external=allow_share_partner_tenant → ok: partner_tenant_*, tenant_*, closed | conflict: anyone_*
//	external=closed                → ok: tenant_*, closed            | conflict: anyone_*, partner_tenant_*
func validateExternalLinkShareCombo(external, linkShare string) error {
	if external == "" || linkShare == "" {
		return nil
	}
	switch external {
	case "open":
	case "allow_share_partner_tenant":
		if strings.HasPrefix(linkShare, "anyone_") {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"--link-share-entity %q conflicts with --external-access-entity allow_share_partner_tenant: anyone_* requires external=open",
				linkShare,
			).WithParam("--link-share-entity")
		}
	case "closed":
		if strings.HasPrefix(linkShare, "anyone_") || strings.HasPrefix(linkShare, "partner_tenant_") {
			return errs.NewValidationError(errs.SubtypeInvalidArgument,
				"--link-share-entity %q conflicts with --external-access-entity closed: only tenant_* or closed are allowed",
				linkShare,
			).WithParam("--link-share-entity")
		}
	}
	return nil
}

func buildDrivePublicPermissionBody(runtime *common.RuntimeContext) map[string]interface{} {
	body := make(map[string]interface{})
	for _, f := range drivePublicPermissionBodyFlags {
		if value := strings.TrimSpace(runtime.Str(f.Flag)); value != "" {
			body[f.JSON] = value
		}
	}
	if value := strings.TrimSpace(runtime.Str("perm-type")); value != "" {
		body["perm_type"] = value
	}
	return body
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/recovery"
	"github.com/larksuite/cli/shortcuts/common"
)

// docsSceneContextKey lets in-process embedders pass a server-owned docs_ai
// scene without exposing it as a user-controlled CLI flag.
const docsSceneContextKey = "lark_cli_docs_scene"

type documentRef struct {
	Kind     string
	Token    string
	Fragment string
}

func parseDocumentRef(input string) (documentRef, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return documentRef{}, errs.NewValidationError(errs.SubtypeInvalidArgument, "--doc cannot be empty").WithParam("--doc")
	}

	if token, ok := extractDocumentToken(raw, "/wiki/"); ok {
		return documentRef{Kind: "wiki", Token: token, Fragment: extractDocumentFragment(raw)}, nil
	}
	if token, ok := extractDocumentToken(raw, "/docx/"); ok {
		return documentRef{Kind: "docx", Token: token, Fragment: extractDocumentFragment(raw)}, nil
	}
	if token, ok := extractDocumentToken(raw, "/doc/"); ok {
		return documentRef{Kind: "doc", Token: token, Fragment: extractDocumentFragment(raw)}, nil
	}
	if strings.Contains(raw, "://") {
		return documentRef{}, errs.NewValidationError(errs.SubtypeInvalidArgument, "unsupported --doc input %q: use a docx URL/token or a wiki URL that resolves to docx", raw).WithParam("--doc")
	}
	if strings.ContainsAny(raw, "/?#") {
		return documentRef{}, errs.NewValidationError(errs.SubtypeInvalidArgument, "unsupported --doc input %q: use a docx token or a wiki URL", raw).WithParam("--doc")
	}

	return documentRef{Kind: "docx", Token: raw}, nil
}

func extractDocumentToken(raw, marker string) (string, bool) {
	idx := strings.Index(raw, marker)
	if idx < 0 {
		return "", false
	}
	token := raw[idx+len(marker):]
	if end := strings.IndexAny(token, "/?#"); end >= 0 {
		token = token[:end]
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}

func extractDocumentFragment(raw string) string {
	idx := strings.Index(raw, "#")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(raw[idx+1:])
}

// doDocAPI executes an OpenAPI request against the docs_ai endpoints and returns
// the parsed "data" field from the standard Lark response envelope {code, msg, data}.
// CallAPITyped lifts the x-tt-logid response header onto the typed error so log_id
// surfaces for support escalations even when the body omits it.
func doDocAPI(runtime *common.RuntimeContext, method, apiPath string, body interface{}) (map[string]interface{}, error) {
	data, err := runtime.CallAPITyped(method, apiPath, nil, body)
	if err != nil {
		return data, withDocAPIRecovery(err, runtime.IsBot())
	}
	if data == nil {
		return nil, errs.NewInternalError(errs.SubtypeInvalidResponse, "document API returned an empty data object")
	}
	return data, nil
}

type docWriteOperation string

const (
	docWriteCreate docWriteOperation = "create"
	docWriteUpdate docWriteOperation = "update"
)

func withDocAPIRecovery(err error, bot bool) error {
	if err == nil {
		return nil
	}
	problem, ok := errs.ProblemOf(err)
	if !ok {
		return err
	}
	switch problem.Code {
	case 3380002, 3380004, -32011, 99991668:
	default:
		return err
	}
	clone, ok := recovery.CloneTyped(err)
	if !ok {
		return err
	}
	problem, ok = errs.ProblemOf(clone)
	if !ok {
		return err
	}
	switch problem.Code {
	case 3380002:
		problem.Hint = "stop retrying the same document reference; verify the original docx/wiki URL, resource type, deletion state, and access for the selected identity"
	case 3380004:
		problem.Hint = "ask the document owner to grant the selected identity access and check sharing or tenant policy; re-authentication alone does not fix document ACLs"
	case -32011, 99991668:
		if bot {
			problem.Hint = "verify the selected identity and bot app credentials; do not run user auth login for a bot credential"
		} else {
			problem.Hint = "run `lark-cli auth status --verify`; if the user credential is invalid, refresh user login and retry with `--as user`"
		}
	}
	return clone
}

func withDocWriteRecovery(err error, operation docWriteOperation) error {
	if err == nil {
		return nil
	}
	clone, ok := recovery.CloneTyped(err)
	if !ok {
		return err
	}
	networkErr, ok := clone.(*errs.NetworkError)
	if !ok {
		return err
	}
	switch networkErr.Subtype {
	case errs.SubtypeNetworkServer, errs.SubtypeNetworkTimeout, errs.SubtypeNetworkTransport:
	default:
		return err
	}

	networkErr.Retryable = false
	networkErr.RetryAfterSeconds = 0
	networkErr.OutcomeUnknown = true
	if operation == docWriteCreate {
		networkErr.Hint = "the create request may already have succeeded; inspect the target folder for the document before creating another one"
	} else {
		networkErr.Hint = "the update request may already have succeeded; fetch the affected document scope before applying another update"
	}
	return networkErr
}

func docsAPIOperationFailed(data map[string]interface{}) bool {
	return strings.EqualFold(strings.TrimSpace(common.GetString(data, "result")), "failed")
}

func docsSceneFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	scene, _ := ctx.Value(docsSceneContextKey).(string)
	return strings.TrimSpace(scene)
}

func injectDocsScene(runtime *common.RuntimeContext, body map[string]interface{}) {
	if scene := docsSceneFromContext(runtime.Ctx()); scene != "" {
		body["scene"] = scene
	}
}

func buildDriveRouteExtra(docID string) (string, error) {
	extra, err := json.Marshal(map[string]string{"drive_route_token": docID})
	if err != nil {
		return "", errs.NewInternalError(errs.SubtypeUnknown, "failed to marshal upload extra data: %v", err).WithCause(err)
	}
	return string(extra), nil
}

func appendDocWarning(data map[string]interface{}, warning string) {
	if data == nil {
		return
	}
	if strings.TrimSpace(warning) == "" {
		return
	}
	switch existing := data["warnings"].(type) {
	case []interface{}:
		data["warnings"] = append(existing, warning)
	case []string:
		data["warnings"] = append(existing, warning)
	case nil:
		data["warnings"] = []string{warning}
	default:
		data["warnings"] = []interface{}{existing, warning}
	}
}

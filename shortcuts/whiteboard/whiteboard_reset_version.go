// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package whiteboard

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

var wbResetVersionScopes = []string{"board:whiteboard:node:create"}
var wbResetVersionAuthTypes = []string{"user", "bot"}
var wbResetVersionFlags = []common.Flag{
	{Name: "whiteboard-token", Desc: "whiteboard token of the whiteboard to revert. You will need edit permission.", Required: true},
	{Name: "target-revision", Desc: "target history revision to revert the whiteboard to. Positive integer string.", Required: true},
}

type resetVersionReq struct {
	TargetRevision string `json:"target_revision"`
}

func wbResetVersionValidate(ctx context.Context, runtime *common.RuntimeContext) error {
	token := runtime.Str("whiteboard-token")
	if err := common.RejectDangerousCharsTyped("--whiteboard-token", token); err != nil {
		return err
	}
	target := strings.TrimSpace(runtime.Str("target-revision"))
	if err := common.RejectDangerousCharsTyped("--target-revision", target); err != nil {
		return err
	}
	// The API types target_revision as an int64 in string form; reject anything
	// that is not a positive integer before spending a network round-trip.
	rev, err := strconv.ParseInt(target, 10, 64)
	if err != nil {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--target-revision must be a positive integer string").WithParam("--target-revision").WithCause(err)
	}
	if rev <= 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--target-revision must be a positive integer string").WithParam("--target-revision")
	}
	return nil
}

func wbResetVersionDryRun(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
	token := runtime.Str("whiteboard-token")
	reqBody := resetVersionReq{TargetRevision: strings.TrimSpace(runtime.Str("target-revision"))}
	return common.NewDryRunAPI().
		POST(fmt.Sprintf("/open-apis/board/v1/whiteboards/%s/reset_version", common.MaskToken(url.PathEscape(token)))).
		Body(reqBody).
		Desc("revert the whiteboard to the target history revision.")
}

func wbResetVersionExecute(ctx context.Context, runtime *common.RuntimeContext) error {
	token := runtime.Str("whiteboard-token")
	reqBody := resetVersionReq{TargetRevision: strings.TrimSpace(runtime.Str("target-revision"))}

	_, err := runtime.CallAPITyped(
		http.MethodPost,
		fmt.Sprintf("/open-apis/board/v1/whiteboards/%s/reset_version", url.PathEscape(token)),
		nil,
		reqBody,
	)
	if err != nil {
		return err
	}

	outData := map[string]string{
		"whiteboard_token": token,
		"target_revision":  reqBody.TargetRevision,
	}
	runtime.OutFormat(outData, nil, func(w io.Writer) {
		fmt.Fprintf(w, "Whiteboard reverted to revision %s", reqBody.TargetRevision)
	})
	return nil
}

// WhiteboardResetVersionDescription describes the whiteboard reset-version shortcut.
const WhiteboardResetVersionDescription = "Revert a whiteboard to a specified history revision. Use it to roll back an abnormal or unwanted edit."

// WhiteboardResetVersion registers the `whiteboard +reset-version` shortcut.
var WhiteboardResetVersion = common.Shortcut{
	Service:     "whiteboard",
	Command:     "+reset-version",
	Description: WhiteboardResetVersionDescription,
	Risk:        "write",
	Scopes:      wbResetVersionScopes,
	AuthTypes:   wbResetVersionAuthTypes,
	Flags:       wbResetVersionFlags,
	Validate:    wbResetVersionValidate,
	DryRun:      wbResetVersionDryRun,
	Execute:     wbResetVersionExecute,
}

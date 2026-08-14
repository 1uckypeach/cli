// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package wiki

import (
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/shortcuts/common"
)

// wikiNodeURL returns the user-facing link for a wiki node. The create/copy
// OpenAPI responses carry a real `url` (undocumented in the server-docs schema
// but present in practice); prefer it so the CLI surfaces the canonical link.
// Fall back to BuildResourceURL synthesis only when the response omits it.
//
// Shared by +node-create and +node-copy, hence kept here rather than in either
// command's file.
func wikiNodeURL(brand core.LarkBrand, node *wikiNodeRecord) string {
	if node == nil {
		return ""
	}
	if u := strings.TrimSpace(node.URL); u != "" {
		return u
	}
	return common.BuildResourceURL(brand, "wiki", node.NodeToken)
}

func appendWikiProblemHint(err error, hint string) error {
	if strings.TrimSpace(hint) == "" {
		return err
	}
	if p, ok := errs.ProblemOf(err); ok {
		if strings.TrimSpace(p.Hint) != "" {
			p.Hint = p.Hint + "\n" + hint
		} else {
			p.Hint = hint
		}
	}
	return err
}

// wikiPermissionDeniedHint provides stable recovery for read-path 131006
// (node-get / node-list). The service uses one public code for both node and
// space ACL failures, while the upstream message is informational and
// normalized by the shared classifier. Keep the command hint accurate without
// branching on that unstable text.
func wikiPermissionDeniedHint() string {
	return "The current user or app/bot identity lacks access to the target wiki space or node. This is resource access, not app scope authorization. Do not retry the same request, reauthorize, or switch identity as trial and error; ask the resource owner or wiki administrator to grant read access, or use an accessible resource."
}

// wikiWritePermissionDeniedHint provides stable recovery for write-path 131006
// (node-copy / move). Official copy/move APIs require container edit permission
// on the relevant parent nodes; copying or moving to a space root can also
// require wiki space membership or administrator permission.
func wikiWritePermissionDeniedHint() string {
	return "The current user or app/bot identity lacks Wiki container permission for this write. This is resource access, not app scope authorization. Do not retry the same request, reauthorize, or switch identity as trial and error; ask the resource owner or wiki administrator to grant container edit permission on the relevant source or destination parent node. Copying or moving to a space root can also require wiki space membership or administrator permission. Use an accessible resource if that access cannot be granted."
}

func annotateWikiPermissionDenied(err error) error {
	return annotateWikiPermissionDeniedWith(err, wikiPermissionDeniedHint())
}

func annotateWikiWritePermissionDenied(err error) error {
	return annotateWikiPermissionDeniedWith(err, wikiWritePermissionDeniedHint())
}

// annotateWikiPermissionDeniedWith marks wiki 131006 as a terminal
// resource-access failure and attaches the given recovery hint. Other errors
// pass through.
func annotateWikiPermissionDeniedWith(err error, hint string) error {
	p, ok := errs.ProblemOf(err)
	if !ok || p == nil || p.Code != 131006 {
		return err
	}
	p.Retryable = false
	return appendWikiProblemHint(err, hint)
}

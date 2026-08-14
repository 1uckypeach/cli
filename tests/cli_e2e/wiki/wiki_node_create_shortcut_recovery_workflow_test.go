// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package wiki

import (
	"context"
	"strings"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
)

// TestWiki_NodeCreateShortcutSourceRecovery verifies the real API rejects a
// shortcut whose source is another shortcut and that +node-create converts the
// generic 131002 response into a terminal, actionable recovery hint. The CLI
// must not silently replay the write with different input.
func TestWiki_NodeCreateShortcutSourceRecovery(t *testing.T) {
	clie2e.SkipWithoutTenantAccessToken(t)
	parentT := t
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	suffix := clie2e.GenerateSuffix()
	_, parentNode := createWikiNodeUnderAnyHost(t, parentT, ctx, "lark-cli-e2e-wiki-shortcut-recovery-parent-"+suffix)
	spaceID := parentNode.Get("space_id").String()
	parentNodeToken := parentNode.Get("node_token").String()
	require.NotEmpty(t, spaceID)
	require.NotEmpty(t, parentNodeToken)

	originNode, originResult, err := createWikiNode(t, parentT, ctx, spaceID, map[string]any{
		"node_type":         "origin",
		"obj_type":          "docx",
		"title":             "lark-cli-e2e-wiki-shortcut-recovery-origin-" + suffix,
		"parent_node_token": parentNodeToken,
	})
	require.NoError(t, err)
	originResult.AssertExitCode(t, 0)
	originNodeToken := originNode.Get("node_token").String()
	require.NotEmpty(t, originNodeToken)

	shortcutNode, shortcutResult, err := createWikiNode(t, parentT, ctx, spaceID, map[string]any{
		"node_type":         "shortcut",
		"obj_type":          "docx",
		"title":             "lark-cli-e2e-wiki-shortcut-recovery-source-" + suffix,
		"parent_node_token": parentNodeToken,
		"origin_node_token": originNodeToken,
	})
	require.NoError(t, err)
	shortcutResult.AssertExitCode(t, 0)
	shortcutNodeToken := shortcutNode.Get("node_token").String()
	require.NotEmpty(t, shortcutNodeToken)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"wiki", "+node-create",
			"--space-id", spaceID,
			"--parent-node-token", parentNodeToken,
			"--node-type", "shortcut",
			"--obj-type", "docx",
			"--origin-node-token", shortcutNodeToken,
			"--title", "lark-cli-e2e-wiki-shortcut-recovery-target-" + suffix,
		},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	require.NotEqual(t, 0, result.ExitCode, "shortcut-to-shortcut create must fail; stdout=%s stderr=%s", result.Stdout, result.Stderr)

	combined := result.Stdout + "\n" + result.Stderr
	for _, fragment := range []string{
		"131002",
		"points to another Wiki shortcut",
		"Do not retry with the same parameters",
		"--origin-node-token " + originNodeToken,
		"--obj-type docx",
	} {
		require.True(t, strings.Contains(combined, fragment), "output missing %q; stdout=%s stderr=%s", fragment, result.Stdout, result.Stderr)
	}
}

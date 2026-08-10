// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package docs

import (
	"context"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
)

func TestDocsMediaUploadDryRun(t *testing.T) {
	setDocsDryRunEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args:      []string{"docs", "+media-upload", "--file", "fixture.bin", "--parent-type", "docx_file", "--parent-node", "blk_parent", "--doc-id", "doxcnDoc", "--dry-run"},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	out := result.Stdout
	require.Equal(t, "POST", clie2e.DryRunGet(out, "api.0.method").String(), out)
	require.Equal(t, "/open-apis/drive/v1/medias/upload_all", clie2e.DryRunGet(out, "api.0.url").String(), out)
	require.Equal(t, "fixture.bin", clie2e.DryRunGet(out, "api.0.body.file_name").String(), out)
	require.Equal(t, "docx_file", clie2e.DryRunGet(out, "api.0.body.parent_type").String(), out)
	require.Equal(t, "blk_parent", clie2e.DryRunGet(out, "api.0.body.parent_node").String(), out)
	require.Contains(t, clie2e.DryRunGet(out, "api.0.body.extra").String(), "doxcnDoc")
}

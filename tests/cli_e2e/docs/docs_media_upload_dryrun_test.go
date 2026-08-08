// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package docs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
)

// TestDocsMediaUploadDryRunParentType pins the public CLI request contract:
// ordinary document parents retain the caller-provided parent_type, while a
// 28-character local Office token carrying the interleaved OFL0X marker and a
// trailing Word type enum (3) uses office_docx_file.
func TestDocsMediaUploadDryRunParentType(t *testing.T) {
	setDocsDryRunEnv(t)

	workDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "image.png"), []byte("png-bytes"), 0o600))

	tests := []struct {
		name           string
		parentNode     string
		wantParentType string
	}{
		{
			name:           "native docx parent",
			parentNode:     "blkcnDryRunNative",
			wantParentType: "docx_image",
		},
		{
			name:           "local office docx parent",
			parentNode:     "KvLqOjiJMFwICuLfVeK0z3LTXNf3",
			wantParentType: "office_docx_file",
		},
		{
			name:           "local office excel parent is not docx",
			parentNode:     "KvLqOjiJMFwICuLfVeK0z3LTXNf1",
			wantParentType: "docx_image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			t.Cleanup(cancel)

			result, err := clie2e.RunCmd(ctx, clie2e.Request{
				Args: []string{
					"docs", "+media-upload",
					"--file", "image.png",
					"--parent-type", "docx_image",
					"--parent-node", tt.parentNode,
					"--dry-run",
				},
				DefaultAs: "user",
				WorkDir:   workDir,
			})
			require.NoError(t, err)
			result.AssertExitCode(t, 0)

			out := result.Stdout
			require.Equal(t, "POST", clie2e.DryRunGet(out, "api.0.method").String(), "stdout:\n%s", out)
			require.Equal(t, "/open-apis/drive/v1/medias/upload_all",
				clie2e.DryRunGet(out, "api.0.url").String(), "stdout:\n%s", out)
			require.Equal(t, tt.wantParentType, clie2e.DryRunGet(out, "api.0.body.parent_type").String(),
				"parent_type for parent_node %q must be %q; stdout:\n%s", tt.parentNode, tt.wantParentType, out)
			require.Equal(t, tt.parentNode, clie2e.DryRunGet(out, "api.0.body.parent_node").String(),
				"parent_node must be preserved; stdout:\n%s", out)
		})
	}
}

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

func TestDocsMediaDownloadDryRun_UsesMediaDownloadAsAuthoritativePermissionCheck(t *testing.T) {
	setDocsDryRunEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"docs", "+media-download",
			"--token", "mediaDryRunDownload",
			"--output", "./artifacts/media.bin",
			"--dry-run",
		},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)

	out := result.Stdout
	if got := clie2e.DryRunGet(out, "api.#").Int(); got != 1 {
		t.Fatalf("api count=%d, want 1\nstdout:\n%s", got, out)
	}
	if got := clie2e.DryRunGet(out, "api.0.url").String(); got != "/open-apis/drive/v1/medias/mediaDryRunDownload/download" {
		t.Fatalf("api.0.url=%q, want media download\nstdout:\n%s", got, out)
	}
}

func TestDocsMediaDownloadDryRun_WhiteboardSkipsExportAuth(t *testing.T) {
	setDocsDryRunEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"docs", "+media-download",
			"--token", "boardDryRunDownload",
			"--type", "whiteboard",
			"--output", "./artifacts/board.png",
			"--dry-run",
		},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)

	out := result.Stdout
	if got := clie2e.DryRunGet(out, "api.#").Int(); got != 1 {
		t.Fatalf("api count=%d, want 1\nstdout:\n%s", got, out)
	}
	if got := clie2e.DryRunGet(out, "api.0.url").String(); got != "/open-apis/board/v1/whiteboards/boardDryRunDownload/download_as_image" {
		t.Fatalf("api.0.url=%q, want whiteboard download only\nstdout:\n%s", got, out)
	}
}

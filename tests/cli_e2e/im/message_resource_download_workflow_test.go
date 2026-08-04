// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"context"
	"crypto/md5"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// resourceDownloadFixtureSize is deliberately larger than the 128 KiB probe
// chunk the downloader uses, so a live run exercises the ranged path — the
// probe, the follow-up range requests, and the Content-Range and validator
// checks that hold them together — rather than the single-stream path.
const resourceDownloadFixtureSize = 320 * 1024

// TestIM_MessageResourceDownloadWorkflowAsBot uploads a file through
// `im +messages-send`, reads its file_key back off the message, downloads it
// with `im +messages-resources-download`, and compares the bytes. It is the
// only coverage that exercises the real endpoint's Content-Range, ETag and
// Content-Disposition behaviour; unit tests can only assert what a fake server
// was told to send.
func TestIM_MessageResourceDownloadWorkflowAsBot(t *testing.T) {
	clie2e.SkipWithoutTenantAccessToken(t)
	parentT := t
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)

	suffix := clie2e.GenerateSuffix()
	chatID := createChat(t, parentT, ctx, "im-resource-download-"+suffix)

	workDir := t.TempDir()
	fixtureRelPath := filepath.Join("fixture", "resource.bin")
	payload := make([]byte, resourceDownloadFixtureSize)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	require.NoError(t, os.MkdirAll(filepath.Join(workDir, "fixture"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workDir, fixtureRelPath), payload, 0o600))

	messageID := sendFileMessageOrSkipPermission(t, ctx, chatID, workDir, fixtureRelPath)
	fileKey := fileKeyOfMessage(t, ctx, messageID)

	t.Run("download file resource larger than the probe chunk", func(t *testing.T) {
		downloadDir := t.TempDir()
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args: []string{"im", "+messages-resources-download",
				"--message-id", messageID,
				"--file-key", fileKey,
				"--type", "file",
				"--output", "./downloaded.bin",
			},
			WorkDir:   downloadDir,
			DefaultAs: "bot",
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		result.AssertStdoutStatus(t, true)

		require.Equal(t, int64(len(payload)), gjson.Get(result.Stdout, "data.size_bytes").Int(),
			"stdout:\n%s", result.Stdout)
		require.NotEmpty(t, gjson.Get(result.Stdout, "data.saved_path").String(), "stdout:\n%s", result.Stdout)

		got, readErr := os.ReadFile(filepath.Join(downloadDir, "downloaded.bin"))
		require.NoError(t, readErr)
		require.Equal(t, md5.Sum(payload), md5.Sum(got),
			"downloaded bytes must match the uploaded fixture byte for byte")
	})
}

// sendFileMessageOrSkipPermission sends workDir-relative relPath as a file
// message and returns the message id, skipping when the test account cannot
// upload IM resources.
func sendFileMessageOrSkipPermission(t *testing.T, ctx context.Context, chatID, workDir, relPath string) string {
	t.Helper()

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{"im", "+messages-send",
			"--chat-id", chatID,
			"--file", "./" + filepath.ToSlash(relPath),
		},
		WorkDir:   workDir,
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	if result.ExitCode != 0 {
		combined := strings.ToLower(result.Stdout + "\n" + result.Stderr)
		if strings.Contains(combined, "app scope not enabled") ||
			strings.Contains(combined, "im:resource") ||
			strings.Contains(combined, "99991672") {
			t.Skipf("skip IM resource download workflow due to missing bot scope: %s",
				strings.TrimSpace(result.Stdout+"\n"+result.Stderr))
		}
	}
	result.AssertExitCode(t, 0)
	result.AssertStdoutStatus(t, true)

	messageID := gjson.Get(result.Stdout, "data.message_id").String()
	require.NotEmpty(t, messageID, "message_id should not be empty\nstdout:\n%s", result.Stdout)
	return messageID
}

// fileKeyOfMessage reads the file_key out of a file message's content, proving
// the key the download uses came from the platform rather than the test.
func fileKeyOfMessage(t *testing.T, ctx context.Context, messageID string) string {
	t.Helper()

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args:      []string{"im", "+messages-mget", "--message-ids", messageID},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	result.AssertStdoutStatus(t, true)

	content := gjson.Get(result.Stdout, "data.messages.0.content").String()
	fileKey := gjson.Get(content, "file_key").String()
	require.NotEmpty(t, fileKey, "file message content should carry file_key\nstdout:\n%s", result.Stdout)
	return fileKey
}

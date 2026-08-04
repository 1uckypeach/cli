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

// resourceDownloadFixtureSize is larger than the 128 KiB probe chunk, so the
// download sends a Range header and the ranged path runs whenever the endpoint
// honours it.
//
// What this test can prove is that a real upload/download round trip returns the
// exact bytes. It cannot prove which path ran: nothing in the command output
// says whether the endpoint answered 206 or ignored the Range and answered 200,
// so a green run is not evidence that the Content-Range and validator checks
// executed. Those are pinned by the unit tests in shortcuts/im.
const resourceDownloadFixtureSize = 320 * 1024

// TestIM_MessageResourceDownloadWorkflowAsBot uploads a file through
// `im +messages-send`, reads its file_key back off the message, downloads it
// with `im +messages-resources-download`, and compares the bytes. It is the only
// coverage that runs the shortcut against the real endpoint rather than a fake
// server told what to send.
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
			"downloaded size should match the fixture")
		require.NotEmpty(t, gjson.Get(result.Stdout, "data.saved_path").String(),
			"result should report where the resource was saved")

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
			// Only the classification is logged; the envelope carries live tenant
			// and chat identifiers and this job's logs are public.
			t.Skipf("skip IM resource download workflow due to missing bot scope (exit %d, error.subtype=%q)",
				result.ExitCode, gjson.Get(result.Stderr, "error.subtype").String())
		}
	}
	result.AssertExitCode(t, 0)
	result.AssertStdoutStatus(t, true)

	messageID := gjson.Get(result.Stdout, "data.message_id").String()
	require.NotEmpty(t, messageID, "message_id should not be empty")
	return messageID
}

// fileKeyOfMessage reads the resource key out of a file message, proving the key
// the download uses came from the platform rather than from the test.
//
// `im +messages-mget` does not hand back the platform's raw JSON content: it
// converts each message to the CLI's display form, so a file message arrives as
// `<file key="file_v3_..." name="resource.bin"/>` rather than
// `{"file_key":"..."}`. The key is read out of that attribute.
func fileKeyOfMessage(t *testing.T, ctx context.Context, messageID string) string {
	t.Helper()

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args:      []string{"im", "+messages-mget", "--message-ids", messageID},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	result.AssertStdoutStatus(t, true)

	msgType := gjson.Get(result.Stdout, "data.messages.0.msg_type").String()
	require.Equal(t, "file", msgType, "message should be a file message")

	content := gjson.Get(result.Stdout, "data.messages.0.content").String()
	fileKey, ok := fileKeyFromMessageContent(content)
	// Deliberately not echoing content or stdout: this job's logs are public and
	// the payload carries live chat, message, sender and tenant identifiers.
	require.True(t, ok, "file message content should carry a key attribute (content length %d)", len(content))
	require.NotEmpty(t, fileKey, "file key should not be empty")
	return fileKey
}

// fileKeyFromMessageContent extracts the key attribute from a converted file
// message such as `<file key="file_v3_x" name="resource.bin"/>`.
func fileKeyFromMessageContent(content string) (string, bool) {
	const marker = `key="`
	start := strings.Index(content, marker)
	if start < 0 {
		return "", false
	}
	rest := content[start+len(marker):]
	end := strings.Index(rest, `"`)
	if end <= 0 {
		return "", false
	}
	return rest[:end], true
}

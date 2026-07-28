// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package docs

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/larksuite/cli/tests/cli_e2e/drive"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestDocs_LocalResourcesWorkflowAsBot(t *testing.T) {
	testDocsLocalResourcesWorkflow(t, "bot")
}

func TestDocs_LocalResourcesWorkflowAsUser(t *testing.T) {
	clie2e.SkipWithoutUserToken(t)
	testDocsLocalResourcesWorkflow(t, "user")
}

func testDocsLocalResourcesWorkflow(t *testing.T, defaultAs string) {
	t.Helper()
	if os.Getenv("LARK_DOC_LOCAL_RESOURCES_E2E") != "1" {
		t.Skip("set LARK_DOC_LOCAL_RESOURCES_E2E=1 and use a server lane with local-resource placeholder support")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)

	workDir := t.TempDir()
	createdSource := []byte("created source fixture\n")
	appendedNegativeSource := []byte("appended negative source fixture\n")
	appendedNonNumericSource := []byte("appended nonnumeric source fixture\n")
	writeLocalResourceFixture(t, workDir, "created.png", onePixelPNG)
	writeLocalResourceFixture(t, workDir, "created.txt", createdSource)
	writeLocalResourceFixture(t, workDir, "appended.png", onePixelPNG)
	writeLocalResourceFixture(t, workDir, "appended-negative.txt", appendedNegativeSource)
	writeLocalResourceFixture(t, workDir, "appended-nonnumeric.txt", appendedNonNumericSource)

	suffix := clie2e.GenerateSuffix()
	parentT := t
	folderToken := ""
	cleanupAs := defaultAs
	if defaultAs == "bot" {
		// Bot-created documents grant the current CLI user full access, while
		// the shared PPE bot intentionally lacks Drive delete scopes.
		cleanupAs = "user"
	} else {
		folderToken = drive.CreateDriveFolder(t, parentT, ctx, "lark-cli-e2e-local-resources-"+suffix, defaultAs, "")
	}
	var docToken string
	var roundTripDocToken string
	var roundTripContent string

	t.Run("create image and source", func(t *testing.T) {
		args := []string{
			"docs", "+create",
			"--title", "lark-cli local resources " + suffix,
			"--content", `<p>created resources</p><img path="@created.png" caption="created image" width="0" height="-1"/><source path="@created.txt" name="created-report.txt" size="0"/>`,
		}
		if folderToken != "" {
			args = append(args, "--parent-token", folderToken)
		}
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args:      args,
			DefaultAs: defaultAs,
			WorkDir:   workDir,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		result.AssertStdoutStatus(t, true)
		assertBoundLocalResourceBlocks(t, result.Stdout, 1, 1)

		docToken = gjson.Get(result.Stdout, "data.document.document_id").String()
		require.NotEmpty(t, docToken, "stdout:\n%s", result.Stdout)
		parentT.Cleanup(func() {
			cleanupCtx, cleanupCancel := clie2e.CleanupContext()
			defer cleanupCancel()
			deleteResult, deleteErr := drive.DeleteDriveResourceAndVerify(cleanupCtx, docToken, "docx", cleanupAs)
			clie2e.ReportCleanupFailure(parentT, "delete doc "+docToken, deleteResult, deleteErr)
		})
	})

	t.Run("append image and source", func(t *testing.T) {
		require.NotEmpty(t, docToken, "document token should be created before update")
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args: []string{
				"docs", "+update",
				"--doc", docToken,
				"--command", "append",
				"--content", `<p>appended resources</p><img path="@appended.png" caption="appended image" width="invalid" height="0"/><source path="@appended-negative.txt" name="appended-negative-report.txt" size="-2"/><source path="@appended-nonnumeric.txt" name="appended-nonnumeric-report.txt" size="invalid"/>`,
			},
			DefaultAs: defaultAs,
			WorkDir:   workDir,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		result.AssertStdoutStatus(t, true)
		assertBoundLocalResourceBlocks(t, result.Stdout, 1, 2)
	})

	t.Run("fetch verifies persisted resources", func(t *testing.T) {
		require.NotEmpty(t, docToken, "document token should be created before fetch")
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args: []string{
				"docs", "+fetch",
				"--doc", docToken,
				"--doc-format", "xml",
				"--detail", "full",
			},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		result.AssertStdoutStatus(t, true)

		content := gjson.Get(result.Stdout, "data.document.content").String()
		for _, want := range []string{
			"created image",
			"appended image",
			"created-report.txt",
			"appended-negative-report.txt",
			"appended-nonnumeric-report.txt",
		} {
			require.Contains(t, content, want, "fetched XML:\n%s", content)
		}
		require.NotContains(t, content, "@lcli_", "fetched XML leaked internal correlation marker")
		require.NotContains(t, content, "@created.", "fetched XML leaked create fixture path")
		require.NotContains(t, content, "@appended.", "fetched XML leaked append fixture path")
	})

	t.Run("fetch markdown preserves resource metadata", func(t *testing.T) {
		require.NotEmpty(t, docToken, "document token should be created before fetch")
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args: []string{
				"docs", "+fetch",
				"--doc", docToken,
				"--doc-format", "markdown",
				"--detail", "full",
			},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		result.AssertStdoutStatus(t, true)

		content := gjson.Get(result.Stdout, "data.document.content").String()
		for _, want := range []string{"![created image](", "![appended image]("} {
			require.Contains(t, content, want, "fetched Markdown:\n%s", content)
		}
		assertMarkdownSourceMetadata(t, content, "created-report.txt", len(createdSource))
		assertMarkdownSourceMetadata(t, content, "appended-negative-report.txt", len(appendedNegativeSource))
		assertMarkdownSourceMetadata(t, content, "appended-nonnumeric-report.txt", len(appendedNonNumericSource))
		require.NotContains(t, content, "@lcli_", "fetched Markdown leaked internal correlation marker")
		require.NotContains(t, content, "@created.", "fetched Markdown leaked create fixture path")
		require.NotContains(t, content, "@appended.", "fetched Markdown leaked append fixture path")

		roundTripContent = content
	})

	t.Run("create from exported markdown restores image captions", func(t *testing.T) {
		require.NotEmpty(t, roundTripContent, "Markdown content should be fetched before replay")
		args := []string{
			"docs", "+create",
			"--title", "lark-cli markdown replay " + suffix,
			"--doc-format", "markdown",
			"--content", "-",
		}
		if folderToken != "" {
			args = append(args, "--parent-token", folderToken)
		}
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args:      args,
			DefaultAs: defaultAs,
			Stdin:     []byte(roundTripContent),
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		result.AssertStdoutStatus(t, true)

		roundTripDocToken = gjson.Get(result.Stdout, "data.document.document_id").String()
		require.NotEmpty(t, roundTripDocToken, "stdout:\n%s", result.Stdout)
		parentT.Cleanup(func() {
			cleanupCtx, cleanupCancel := clie2e.CleanupContext()
			defer cleanupCancel()
			deleteResult, deleteErr := drive.DeleteDriveResourceAndVerify(cleanupCtx, roundTripDocToken, "docx", cleanupAs)
			clie2e.ReportCleanupFailure(parentT, "delete markdown replay doc "+roundTripDocToken, deleteResult, deleteErr)
		})
	})

	t.Run("fetch markdown replay verifies captions and source metadata", func(t *testing.T) {
		require.NotEmpty(t, roundTripDocToken, "Markdown replay document should be created before fetch")
		result, err := clie2e.RunCmd(ctx, clie2e.Request{
			Args: []string{
				"docs", "+fetch",
				"--doc", roundTripDocToken,
				"--doc-format", "xml",
				"--detail", "full",
			},
			DefaultAs: defaultAs,
		})
		require.NoError(t, err)
		result.AssertExitCode(t, 0)
		result.AssertStdoutStatus(t, true)

		content := gjson.Get(result.Stdout, "data.document.content").String()
		for _, want := range []string{
			`caption="created image"`,
			`caption="appended image"`,
			"created-report.txt",
			"appended-negative-report.txt",
			"appended-nonnumeric-report.txt",
		} {
			require.Contains(t, content, want, "replayed XML:\n%s", content)
		}
	})
}

var markdownSourceTagPattern = regexp.MustCompile(`(?s)<source\b[^>]*>`)

func assertMarkdownSourceMetadata(t *testing.T, content, wantName string, wantSize int) {
	t.Helper()
	wantNameAttr := fmt.Sprintf(`name="%s"`, wantName)
	for _, tag := range markdownSourceTagPattern.FindAllString(content, -1) {
		if !strings.Contains(tag, wantNameAttr) {
			continue
		}
		require.Contains(t, tag, fmt.Sprintf(`size="%d"`, wantSize), "source tag in fetched Markdown:\n%s", tag)
		return
	}
	require.Failf(t, "source metadata not found", "fetched Markdown has no source tag with %s:\n%s", wantNameAttr, content)
}

func assertBoundLocalResourceBlocks(t *testing.T, stdout string, wantImages, wantFiles int) {
	t.Helper()
	counts := map[string]int{"image": 0, "file": 0}
	blockIDs := make(map[string]struct{}, wantImages+wantFiles)
	for _, block := range gjson.Get(stdout, "data.document.new_blocks").Array() {
		blockType := block.Get("block_type").String()
		if _, tracked := counts[blockType]; !tracked {
			continue
		}
		counts[blockType]++
		blockID := block.Get("block_id").String()
		require.NotEmpty(t, blockID, "%s block has no block_id: %s", blockType, block.Raw)
		require.NotContains(t, blockIDs, blockID, "multiple local resources reused block_id %s: %s", blockID, stdout)
		blockIDs[blockID] = struct{}{}
		token := block.Get("block_token").String()
		require.NotEmpty(t, token, "%s block has no bound token: %s", blockType, block.Raw)
		require.False(t, strings.HasPrefix(token, "@lcli_"), "%s block leaked marker: %s", blockType, block.Raw)
	}
	require.Equal(t, wantImages, counts["image"], "image blocks in stdout:\n%s", stdout)
	require.Equal(t, wantFiles, counts["file"], "file blocks in stdout:\n%s", stdout)
}

func writeLocalResourceFixture(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

var onePixelPNG = func() []byte {
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		panic(fmt.Sprintf("decode embedded PNG fixture: %v", err))
	}
	return data
}()

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/shortcuts/common"
)

func TestPrepareLocalDocResourcesXMLImageAndSource(t *testing.T) {
	runtime := newLocalDocResourceTestRuntime(t, map[string]string{
		"diagram.png": localDocResourcePNG(t, 100, 80),
		"report.pdf":  "pdf-data",
	})
	content := `<p>before</p><img path="@diagram.png" alt="diagram" width="50" height="40" align="right" scale="0.5"/><source path="@report.pdf" name="report.pdf"></source>`

	got, resources, err := prepareLocalDocResources(runtime, "xml", content)
	if err != nil {
		t.Fatalf("prepareLocalDocResources() error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("resources len = %d, want 2: %#v", len(resources), resources)
	}
	if resources[0].Kind != localDocResourceImage || resources[0].Path != "diagram.png" {
		t.Fatalf("image resource = %#v", resources[0])
	}
	if resources[1].Kind != localDocResourceFile || resources[1].Path != "report.pdf" {
		t.Fatalf("file resource = %#v", resources[1])
	}
	if strings.Contains(got, "@diagram.png") || strings.Contains(got, "@report.pdf") {
		t.Fatalf("rewritten content leaks local path: %s", got)
	}
	for _, want := range []string{resources[0].Marker, resources[1].Marker, `width="100"`, `height="80"`, `align="right"`, `scale="0.500000"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("rewritten content missing %q: %s", want, got)
		}
	}
}

func TestPrepareLocalDocResourcesNormalizesImageDimensionsAndScale(t *testing.T) {
	tests := []struct {
		name          string
		nativeWidth   int
		nativeHeight  int
		attrs         string
		wantWidth     int
		wantHeight    int
		wantScale     float64
		wantHasScale  bool
		wantScaleText string
	}{
		{
			name:         "intrinsic dimensions without model display size",
			nativeWidth:  100,
			nativeHeight: 80,
			wantWidth:    100,
			wantHeight:   80,
		},
		{
			name:          "model width becomes scale",
			nativeWidth:   100,
			nativeHeight:  80,
			attrs:         ` width="50"`,
			wantWidth:     100,
			wantHeight:    80,
			wantScale:     0.5,
			wantHasScale:  true,
			wantScaleText: `scale="0.500000"`,
		},
		{
			name:          "model height becomes scale",
			nativeWidth:   100,
			nativeHeight:  80,
			attrs:         ` height="20"`,
			wantWidth:     100,
			wantHeight:    80,
			wantScale:     0.25,
			wantHasScale:  true,
			wantScaleText: `scale="0.250000"`,
		},
		{
			name:          "model width wins when both dimensions exist",
			nativeWidth:   100,
			nativeHeight:  80,
			attrs:         ` width="25" height="70"`,
			wantWidth:     100,
			wantHeight:    80,
			wantScale:     0.25,
			wantHasScale:  true,
			wantScaleText: `scale="0.250000"`,
		},
		{
			name:          "explicit scale wins over model dimensions",
			nativeWidth:   100,
			nativeHeight:  80,
			attrs:         ` width="50" scale="0.75"`,
			wantWidth:     100,
			wantHeight:    80,
			wantScale:     0.75,
			wantHasScale:  true,
			wantScaleText: `scale="0.750000"`,
		},
		{
			name:          "percentage width becomes scale",
			nativeWidth:   100,
			nativeHeight:  80,
			attrs:         ` width="80%"`,
			wantWidth:     100,
			wantHeight:    80,
			wantScale:     0.8,
			wantHasScale:  true,
			wantScaleText: `scale="0.800000"`,
		},
		{
			name:         "invalid model dimensions are ignored",
			nativeWidth:  100,
			nativeHeight: 80,
			attrs:        ` width="invalid" height="0" scale="-1"`,
			wantWidth:    100,
			wantHeight:   80,
		},
		{
			name:          "wide image is capped below page width",
			nativeWidth:   1200,
			nativeHeight:  800,
			wantWidth:     1200,
			wantHeight:    800,
			wantScale:     0.849999,
			wantHasScale:  true,
			wantScaleText: `scale="0.849999"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := newLocalDocResourceTestRuntime(t, map[string]string{
				"diagram.png": localDocResourcePNG(t, tt.nativeWidth, tt.nativeHeight),
			})
			content := `<img path="@diagram.png"` + tt.attrs + `/>`

			got, resources, err := prepareLocalDocResources(runtime, "xml", content)
			if err != nil {
				t.Fatalf("prepareLocalDocResources() error: %v", err)
			}
			if len(resources) != 1 {
				t.Fatalf("resources len = %d, want 1: %#v", len(resources), resources)
			}
			resource := resources[0]
			if resource.ImageWidth != tt.wantWidth || resource.ImageHeight != tt.wantHeight {
				t.Fatalf("intrinsic dimensions = %dx%d, want %dx%d; content=%s", resource.ImageWidth, resource.ImageHeight, tt.wantWidth, tt.wantHeight, got)
			}
			if resource.HasScale != tt.wantHasScale || resource.ImageScale != tt.wantScale {
				t.Fatalf("scale = %v (present=%v), want %v (present=%v); content=%s", resource.ImageScale, resource.HasScale, tt.wantScale, tt.wantHasScale, got)
			}
			for _, want := range []string{
				fmt.Sprintf(`width="%d"`, tt.wantWidth),
				fmt.Sprintf(`height="%d"`, tt.wantHeight),
			} {
				if !strings.Contains(got, want) {
					t.Fatalf("rewritten content missing %q: %s", want, got)
				}
			}
			if tt.wantHasScale {
				if !strings.Contains(got, tt.wantScaleText) {
					t.Fatalf("rewritten content missing %q: %s", tt.wantScaleText, got)
				}
			} else if strings.Contains(got, ` scale=`) {
				t.Fatalf("rewritten content unexpectedly contains scale: %s", got)
			}
		})
	}
}

// BUG_MAP #1: Markdown alt is persisted by docx_engine through caption, then
// exported back as Markdown alt. Sending only the SDK-only alt attribute loses
// the text when the placeholder reaches the engine.
func TestPrepareLocalDocResourcesMarkdownAltUsesCaption(t *testing.T) {
	runtime := newLocalDocResourceTestRuntime(t, map[string]string{"diagram.png": "png-data"})

	got, resources, err := prepareLocalDocResources(runtime, "markdown", `![architecture diagram](@diagram.png)`)
	if err != nil {
		t.Fatalf("prepareLocalDocResources() error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("resources len = %d, want 1", len(resources))
	}
	if !strings.Contains(got, `caption="architecture diagram"`) {
		t.Fatalf("Markdown alt must be mapped to engine caption, got: %s", got)
	}
	if strings.Contains(got, ` alt=`) {
		t.Fatalf("rewritten Markdown image must not rely on non-persisted alt: %s", got)
	}
}

func TestPrepareLocalDocResourcesMarkdownTitleConsumesQuotedClosingParen(t *testing.T) {
	runtime := newLocalDocResourceTestRuntime(t, map[string]string{"diagram.png": "png-data"})
	content := `before ![architecture diagram](@diagram.png "v2 (final)") after`

	got, resources, err := prepareLocalDocResources(runtime, "markdown", content)
	if err != nil {
		t.Fatalf("prepareLocalDocResources() error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("resources len = %d, want 1", len(resources))
	}
	want := `before <img path="` + resources[0].Marker + `" caption="architecture diagram" title="v2 (final)"/> after`
	if got != want {
		t.Fatalf("rewritten Markdown image = %q, want %q", got, want)
	}
}

func TestParseMarkdownImageDestinationRejectsUnclosedTitle(t *testing.T) {
	content := `![diagram](@diagram.png "v2 (final)) trailing`
	if image, ok := parseMarkdownImageAt(content, 0); ok {
		t.Fatalf("parseMarkdownImageAt() = %#v, true; want invalid unclosed title", image)
	}
}

// BUG_MAP #2: image-looking text in inert Markdown contexts must remain text;
// otherwise the CLI plans an upload for a block the Markdown parser never
// creates and reports a partial failure after the document write succeeded.
func TestPrepareLocalDocResourcesMarkdownIgnoresInertContexts(t *testing.T) {
	runtime := newLocalDocResourceTestRuntime(t, map[string]string{"diagram.png": "png-data"})
	content := "<!-- ![comment](@diagram.png) -->\n    ![indented code](@diagram.png)\n"

	got, resources, err := prepareLocalDocResources(runtime, "markdown", content)
	if err != nil {
		t.Fatalf("prepareLocalDocResources() error: %v", err)
	}
	if got != content {
		t.Fatalf("inert Markdown was rewritten:\n got: %q\nwant: %q", got, content)
	}
	if len(resources) != 0 {
		t.Fatalf("inert Markdown planned %d resources, want 0: %#v", len(resources), resources)
	}
}

func TestPrepareLocalDocResourcesMarkdownListIndentIsRelativeToList(t *testing.T) {
	runtime := newLocalDocResourceTestRuntime(t, map[string]string{"diagram.png": "png-data"})
	content := "- report:\n\n    ![chart](@diagram.png)\n"

	got, resources, err := prepareLocalDocResources(runtime, "markdown", content)
	if err != nil {
		t.Fatalf("prepareLocalDocResources() error: %v", err)
	}
	if len(resources) != 1 || !strings.Contains(got, resources[0].Marker) {
		t.Fatalf("four-space list content was not rewritten: got=%q resources=%#v", got, resources)
	}
	if strings.Contains(got, "@diagram.png") {
		t.Fatalf("rewritten list content leaked local path: %q", got)
	}
}

func TestPrepareLocalDocResourcesMarkdownListIndentedCodeRemainsInert(t *testing.T) {
	runtime := newLocalDocResourceTestRuntime(t, map[string]string{"diagram.png": "png-data"})
	content := "- report:\n\n      ![code](@diagram.png)\n"

	got, resources, err := prepareLocalDocResources(runtime, "markdown", content)
	if err != nil {
		t.Fatalf("prepareLocalDocResources() error: %v", err)
	}
	if got != content || len(resources) != 0 {
		t.Fatalf("six-space list code was rewritten: got=%q resources=%#v", got, resources)
	}
}

func TestPrepareLocalDocResourcesMarkdownThematicBreakDoesNotOpenList(t *testing.T) {
	for _, thematicBreak := range []string{"* * *", "- - -"} {
		t.Run(thematicBreak, func(t *testing.T) {
			runtime := newLocalDocResourceTestRuntime(t, map[string]string{"diagram.png": "png-data"})
			content := thematicBreak + "\n\n    ![code](@diagram.png)\n"

			got, resources, err := prepareLocalDocResources(runtime, "markdown", content)
			if err != nil {
				t.Fatalf("prepareLocalDocResources() error: %v", err)
			}
			if got != content || len(resources) != 0 {
				t.Fatalf("indented code after thematic break was rewritten: got=%q resources=%#v", got, resources)
			}
		})
	}
}

// BUG_MAP #3: internal correlation markers are an implementation detail and
// must never escape in a partial-failure result, even if no document ID was
// returned and cleanup therefore cannot run.
func TestFinalizeLocalDocResourcesScrubsMarkerWhenDocumentIDMissing(t *testing.T) {
	marker := "@lcli_img_0123456789abcdef0123456789abcdef"
	factory, _, _, _ := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-resource-marker-scrub"))
	runtime := common.TestNewRuntimeContextForAPI(
		context.Background(),
		&cobra.Command{Use: "docs +create"},
		docsTestConfigWithAppID("local-resource-marker-scrub"),
		factory,
		core.AsUser,
	)
	block := map[string]interface{}{
		"block_id":    "blk_placeholder",
		"block_type":  "image",
		"block_token": marker,
	}
	data := map[string]interface{}{
		"document": map[string]interface{}{
			"new_blocks": []interface{}{block},
		},
	}

	err := finalizeLocalDocResources(runtime, "", data, []localDocResource{{
		Occurrence: 1,
		Kind:       localDocResourceImage,
		Marker:     marker,
	}})
	if err == nil {
		t.Fatal("finalizeLocalDocResources() error = nil, want partial failure")
	}
	if token := common.GetString(block, "block_token"); token != "" {
		t.Fatalf("partial-failure response leaked marker %q", token)
	}
}

func TestNewLocalDocResourceRejectsTraversalAndSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatalf("MkdirAll(work): %v", err)
	}
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatalf("MkdirAll(outside): %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.png"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFile(secret): %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.png"), filepath.Join(work, "escape.png")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	cmdutil.TestChdir(t, work)
	factory, _, _, _ := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-resource-path"))
	runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-resource-path"), factory, core.AsUser)

	for _, path := range []string{"@../outside/secret.png", "@escape.png"} {
		if _, err := newLocalDocResource(runtime, localDocResourceImage, path, 1); err == nil {
			t.Fatalf("newLocalDocResource(%q) error = nil, want unsafe path rejection", path)
		}
	}
}

func TestCorrelateLocalDocResourcesMatchesTypeAndBlockID(t *testing.T) {
	marker := "@lcli_file_0123456789abcdef0123456789abcdef"
	block := map[string]interface{}{
		"block_id":    "blk_file",
		"block_type":  "file",
		"block_token": marker,
	}
	data := map[string]interface{}{
		"document": map[string]interface{}{"new_blocks": []interface{}{block}},
	}

	outcomes := correlateLocalDocResources(data, []localDocResource{{Occurrence: 1, Kind: localDocResourceFile, Marker: marker}})
	if len(outcomes) != 1 {
		t.Fatalf("outcomes len = %d, want 1", len(outcomes))
	}
	if outcomes[0].Status != "pending" || outcomes[0].BlockID != "blk_file" || !outcomes[0].SafeToCleanup {
		t.Fatalf("outcome = %#v", outcomes[0])
	}
}

func TestCorrelateLocalDocResourcesTypeMismatchIsNotSafeToCleanup(t *testing.T) {
	marker := "@lcli_img_0123456789abcdef0123456789abcdef"
	outcomes := correlateLocalDocResources(map[string]interface{}{
		"document": map[string]interface{}{"new_blocks": []interface{}{
			map[string]interface{}{"block_id": "blk_mismatch", "block_type": "file", "block_token": marker},
		}},
	}, []localDocResource{{Occurrence: 1, Kind: localDocResourceImage, Marker: marker}})

	if len(outcomes) != 1 || outcomes[0].Status != "correlation_failed" || outcomes[0].SafeToCleanup {
		t.Fatalf("mismatched block outcome = %#v", outcomes)
	}
	if len(outcomes[0].CleanupBlockIDs) != 0 || outcomes[0].CleanupStatus != "skipped_ambiguous" {
		t.Fatalf("mismatched block was scheduled for cleanup: %#v", outcomes[0])
	}
}

func TestCorrelateLocalDocResourcesUnknownMarkerDisablesCleanup(t *testing.T) {
	expectedMarker := "@lcli_img_0123456789abcdef0123456789abcdef"
	unknownMarker := "@lcli_file_fedcba9876543210fedcba9876543210"
	outcomes := correlateLocalDocResources(map[string]interface{}{
		"document": map[string]interface{}{"new_blocks": []interface{}{
			map[string]interface{}{"block_id": "blk_expected", "block_type": "image", "block_token": expectedMarker},
			map[string]interface{}{"block_id": "blk_unknown", "block_type": "file", "block_token": unknownMarker},
		}},
	}, []localDocResource{{Occurrence: 1, Kind: localDocResourceImage, Marker: expectedMarker}})

	if len(outcomes) != 1 || outcomes[0].Status != "correlation_failed" || outcomes[0].SafeToCleanup {
		t.Fatalf("unknown marker outcome = %#v", outcomes)
	}
	if outcomes[0].CleanupStatus != "skipped_ambiguous" {
		t.Fatalf("unknown marker cleanup status = %q, want skipped_ambiguous", outcomes[0].CleanupStatus)
	}
	for _, blockID := range outcomes[0].CleanupBlockIDs {
		if blockID == "blk_unknown" {
			t.Fatalf("unknown marker block was scheduled for cleanup: %#v", outcomes[0])
		}
	}
}

func TestBuildLocalDocResourceBatchUpdatePreservesImagePresentation(t *testing.T) {
	image := &localDocResourceOutcome{
		Resource: localDocResource{
			Kind:        localDocResourceImage,
			ImageWidth:  640,
			ImageHeight: 480,
			ImageAlign:  "right",
			ImageScale:  0.5,
			HasScale:    true,
		},
		BlockID:   "blk_image",
		FileToken: "file_image",
	}
	file := &localDocResourceOutcome{
		Resource:  localDocResource{Kind: localDocResourceFile},
		BlockID:   "blk_file",
		FileToken: "file_attachment",
	}

	body := buildLocalDocResourceBatchUpdate([]*localDocResourceOutcome{image, file})
	requests, _ := body["requests"].([]interface{})
	if len(requests) != 2 {
		t.Fatalf("requests len = %d, want 2: %#v", len(requests), body)
	}
	imageReq, _ := requests[0].(map[string]interface{})
	replaceImage, _ := imageReq["replace_image"].(map[string]interface{})
	for key, want := range map[string]interface{}{
		"token":  "file_image",
		"width":  640,
		"height": 480,
		"align":  alignMap["right"],
		"scale":  0.5,
	} {
		if got := replaceImage[key]; got != want {
			t.Fatalf("replace_image[%s] = %#v, want %#v; body=%#v", key, got, want, body)
		}
	}
	fileReq, _ := requests[1].(map[string]interface{})
	replaceFile, _ := fileReq["replace_file"].(map[string]interface{})
	if got := replaceFile["token"]; got != "file_attachment" {
		t.Fatalf("replace_file token = %#v, want file_attachment", got)
	}
}

func TestPrepareLocalDocResourcesSourceUsesExplicitUploadName(t *testing.T) {
	runtime := newLocalDocResourceTestRuntime(t, map[string]string{"report.pdf": "pdf-data"})

	got, resources, err := prepareLocalDocResources(runtime, "xml", `<source path="@report.pdf" name="  自定义报告.pdf  "/>`)
	if err != nil {
		t.Fatalf("prepareLocalDocResources() error: %v", err)
	}
	if len(resources) != 1 || resources[0].FileName != "自定义报告.pdf" {
		t.Fatalf("resources = %#v, want trimmed explicit file name", resources)
	}
	if !strings.Contains(got, `name="自定义报告.pdf"`) {
		t.Fatalf("rewritten source did not preserve trimmed name: %s", got)
	}
}

func TestPrepareLocalDocResourcesRejectsInvalidSourceName(t *testing.T) {
	for _, name := range []string{"   ", "../secret.pdf", `folder\secret.pdf`} {
		t.Run(strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			runtime := newLocalDocResourceTestRuntime(t, map[string]string{"report.pdf": "pdf-data"})
			_, _, err := prepareLocalDocResources(runtime, "xml", `<source path="@report.pdf" name="`+name+`"/>`)
			if err == nil {
				t.Fatalf("source name %q was accepted", name)
			}
		})
	}
}

func TestUploadLocalDocResourceUsesExplicitSourceName(t *testing.T) {
	dir := t.TempDir()
	cmdutil.TestChdir(t, dir)
	if err := os.WriteFile("report.pdf", []byte("pdf-data"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	factory, _, _, reg := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-source-name"))
	runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-source-name"), factory, core.AsUser)
	_, resources, err := prepareLocalDocResources(runtime, "xml", `<source path="@report.pdf" name="自定义报告.pdf"/>`)
	if err != nil {
		t.Fatalf("prepareLocalDocResources: %v", err)
	}
	upload := &httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/drive/v1/medias/upload_all",
		Body: map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{"file_token": "file_custom_name"},
		},
	}
	reg.Register(upload)
	outcome := &localDocResourceOutcome{Resource: resources[0], BlockID: "blk_file", Status: "pending"}
	uploadLocalDocResources(runtime, "doxcn_source_name", []*localDocResourceOutcome{outcome})
	if outcome.Status != "uploaded" || outcome.FileToken != "file_custom_name" {
		t.Fatalf("outcome = %#v", outcome)
	}
	body := string(upload.CapturedBody)
	if !strings.Contains(body, "自定义报告.pdf") || strings.Contains(body, "\r\n\r\nreport.pdf\r\n") {
		t.Fatalf("upload body did not use explicit source name: %s", body)
	}
}

func TestUploadLocalDocResourcesRetriesConflictAndSerializes(t *testing.T) {
	dir := t.TempDir()
	cmdutil.TestChdir(t, dir)
	for _, name := range []string{"first.png", "second.png"} {
		if err := os.WriteFile(name, []byte(name), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	factory, _, _, reg := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-upload-serial"))
	runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-upload-serial"), factory, core.AsUser)
	reg.Register(&httpmock.Stub{Method: "POST", URL: "/open-apis/drive/v1/medias/upload_all", Body: map[string]interface{}{"code": localDocResourceUploadConflictCode, "msg": "material transaction conflict"}})
	reg.Register(&httpmock.Stub{Method: "POST", URL: "/open-apis/drive/v1/medias/upload_all", Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"file_token": "file_first"}}})
	reg.Register(&httpmock.Stub{Method: "POST", URL: "/open-apis/drive/v1/medias/upload_all", Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"file_token": "file_second"}}})
	outcomes := []*localDocResourceOutcome{
		{Resource: localDocResource{Occurrence: 1, Kind: localDocResourceImage, Path: "first.png", FileName: "first.png", Size: int64(len("first.png"))}, BlockID: "blk_first", Status: "pending"},
		{Resource: localDocResource{Occurrence: 2, Kind: localDocResourceImage, Path: "second.png", FileName: "second.png", Size: int64(len("second.png"))}, BlockID: "blk_second", Status: "pending"},
	}
	var waits []time.Duration
	originalWait := waitLocalDocResourceRequest
	waitLocalDocResourceRequest = func(_ context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}
	t.Cleanup(func() { waitLocalDocResourceRequest = originalWait })
	uploadLocalDocResources(runtime, "doxcn_upload_serial", outcomes)
	if outcomes[0].FileToken != "file_first" || outcomes[1].FileToken != "file_second" {
		t.Fatalf("outcomes = %#v", outcomes)
	}
	if len(waits) != 2 || waits[0] != localDocResourceUploadInterval || waits[1] != localDocResourceUploadInterval {
		t.Fatalf("upload pacing waits = %#v", waits)
	}
}

func TestPrepareLocalDocResourcesRejectsDuplicateAttributes(t *testing.T) {
	runtime := newLocalDocResourceTestRuntime(t, map[string]string{
		"diagram.png": "png-data",
		"secret.png":  "secret-data",
	})
	_, resources, err := prepareLocalDocResources(runtime, "xml", `<img path="@diagram.png" PATH="@secret.png"/>`)
	if err == nil {
		t.Fatal("duplicate path attributes were accepted")
	}
	if len(resources) != 0 {
		t.Fatalf("duplicate attributes planned resources: %#v", resources)
	}
	if strings.Contains(err.Error(), "secret.png") {
		t.Fatalf("duplicate-attribute error leaked the second local path: %v", err)
	}
}

func TestCorrelateLocalDocResourcesRejectsDuplicateBlockID(t *testing.T) {
	markers := []string{
		"@lcli_img_0123456789abcdef0123456789abcdef",
		"@lcli_img_fedcba9876543210fedcba9876543210",
	}
	blocks := []interface{}{
		map[string]interface{}{"block_id": "blk_shared", "block_type": "image", "block_token": markers[0]},
		map[string]interface{}{"block_id": "blk_shared", "block_type": "image", "block_token": markers[1]},
	}
	outcomes := correlateLocalDocResources(map[string]interface{}{
		"document": map[string]interface{}{"new_blocks": blocks},
	}, []localDocResource{
		{Occurrence: 1, Kind: localDocResourceImage, Marker: markers[0]},
		{Occurrence: 2, Kind: localDocResourceImage, Marker: markers[1]},
	})
	cleanupCount := 0
	for _, outcome := range outcomes {
		if outcome.Status != "correlation_failed" {
			t.Fatalf("duplicate block outcome = %#v", outcome)
		}
		cleanupCount += len(outcome.CleanupBlockIDs)
	}
	if cleanupCount != 1 {
		t.Fatalf("cleanup block IDs = %d, want one deduplicated delete", cleanupCount)
	}
}

func TestPrepareLocalDocResourcesMarkdownIgnoresNestedFencesAndRawHTML(t *testing.T) {
	runtime := newLocalDocResourceTestRuntime(t, map[string]string{"diagram.png": "png-data"})
	content := strings.Join([]string{
		"> ```xml",
		"> <img path=\"@diagram.png\"/>",
		"> ```",
		"- ```markdown",
		"  ![list code](@diagram.png)",
		"  ```",
		"<pre>",
		"```",
		"<img path=\"@diagram.png\"/>",
		"```",
		"</pre>",
		"<script>const sample = '<img path=\"@diagram.png\"/>';</script>",
		"<style>/* ![style](@diagram.png) */</style>",
		"<textarea><source path=\"@diagram.png\"/></textarea>",
		`\<img path="@diagram.png"/>`,
	}, "\n")

	got, resources, err := prepareLocalDocResources(runtime, "markdown", content)
	if err != nil {
		t.Fatalf("prepareLocalDocResources: %v", err)
	}
	if got != content || len(resources) != 0 {
		t.Fatalf("inert Markdown changed: resources=%#v\n got=%q\nwant=%q", resources, got, content)
	}
}

func TestPrepareLocalDocResourcesXMLBackticksAreNotInert(t *testing.T) {
	runtime := newLocalDocResourceTestRuntime(t, map[string]string{"diagram.png": "png-data"})
	got, resources, err := prepareLocalDocResources(runtime, "xml", "`<img path=\"@diagram.png\"/>`")
	if err != nil {
		t.Fatalf("prepareLocalDocResources: %v", err)
	}
	if len(resources) != 1 || !strings.Contains(got, resources[0].Marker) {
		t.Fatalf("XML backticks incorrectly hid resource: got=%q resources=%#v", got, resources)
	}
}

func TestMarkdownUnescapePreservesNonPunctuationBackslashes(t *testing.T) {
	if got, want := unescapeMarkdownText(`C:\temp\photo \*draft\* \\`), `C:\temp\photo *draft* \`; got != want {
		t.Fatalf("unescapeMarkdownText() = %q, want %q", got, want)
	}
	image, ok := parseMarkdownImageAt(`![x](@images\photo.png)`, 0)
	if !ok || image.Destination != `@images\photo.png` {
		t.Fatalf("parsed destination = %q, ok=%v", image.Destination, ok)
	}
}

func TestBindLocalDocResourcesBatchesTwentyAndPaces(t *testing.T) {
	factory, _, _, reg := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-bind-batch"))
	runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-bind-batch"), factory, core.AsUser)
	first := &httpmock.Stub{Method: "PATCH", URL: "/open-apis/docx/v1/documents/doxcn_batch/blocks/batch_update", Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"document_revision_id": 1}}}
	second := &httpmock.Stub{Method: "PATCH", URL: "/open-apis/docx/v1/documents/doxcn_batch/blocks/batch_update", Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"document_revision_id": 2}}}
	reg.Register(first)
	reg.Register(second)
	outcomes := make([]*localDocResourceOutcome, 21)
	for i := range outcomes {
		outcomes[i] = &localDocResourceOutcome{
			Resource:  localDocResource{Occurrence: i + 1, Kind: localDocResourceImage},
			BlockID:   fmt.Sprintf("blk_%d", i),
			FileToken: fmt.Sprintf("file_%d", i),
			Status:    "uploaded",
		}
	}
	var waits []time.Duration
	originalWait := waitLocalDocResourceRequest
	waitLocalDocResourceRequest = func(_ context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}
	t.Cleanup(func() { waitLocalDocResourceRequest = originalWait })

	revision, revisionKnown := bindLocalDocResources(runtime, "doxcn_batch", outcomes)
	if !revisionKnown || revision != int64(2) {
		t.Fatalf("revision = %#v, want 2", revision)
	}
	if got := requestCountFromLocalDocBatchBody(t, first.CapturedBody); got != 20 {
		t.Fatalf("first batch size = %d, want 20", got)
	}
	if got := requestCountFromLocalDocBatchBody(t, second.CapturedBody); got != 1 {
		t.Fatalf("second batch size = %d, want 1", got)
	}
	if len(waits) != 1 || waits[0] != localDocResourceBindInterval {
		t.Fatalf("bind pacing waits = %#v", waits)
	}
}

func TestNormalizeLocalDocResourceRevision(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value interface{}
		want  interface{}
	}{
		{name: "integer", value: 7, want: int64(7)},
		{name: "float integer", value: float64(8), want: int64(8)},
		{name: "json number", value: json.Number("9"), want: int64(9)},
		{name: "numeric string", value: "10", want: int64(10)},
		{name: "negative sentinel", value: -1},
		{name: "fraction", value: 1.5},
		{name: "invalid string", value: "latest"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeLocalDocResourceRevision(tt.value); got != tt.want {
				t.Fatalf("normalizeLocalDocResourceRevision(%#v) = %#v, want %#v", tt.value, got, tt.want)
			}
		})
	}
}

func TestBindLocalDocResourceRetryUsesStableTokenAndBackoff(t *testing.T) {
	factory, _, _, reg := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-bind-retry"))
	runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-bind-retry"), factory, core.AsUser)
	var clientTokens []string
	for _, stub := range []*httpmock.Stub{
		{Method: "PATCH", URL: "/open-apis/docx/v1/documents/doxcn_retry/blocks/batch_update", Body: map[string]interface{}{"code": localDocResourceUploadRateLimitCode, "msg": "rate limited"}, OnMatch: func(req *http.Request) { clientTokens = append(clientTokens, req.URL.Query().Get("client_token")) }},
		{Method: "GET", URL: "/open-apis/docx/v1/documents/doxcn_retry/blocks/blk_retry", Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"block": map[string]interface{}{"image": map[string]interface{}{"token": ""}}}}},
		{Method: "PATCH", URL: "/open-apis/docx/v1/documents/doxcn_retry/blocks/batch_update", Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{}}, OnMatch: func(req *http.Request) { clientTokens = append(clientTokens, req.URL.Query().Get("client_token")) }},
	} {
		reg.Register(stub)
	}
	var waits []time.Duration
	originalWait := waitLocalDocResourceRequest
	waitLocalDocResourceRequest = func(_ context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}
	t.Cleanup(func() { waitLocalDocResourceRequest = originalWait })
	outcome := &localDocResourceOutcome{Resource: localDocResource{Kind: localDocResourceImage}, BlockID: "blk_retry", FileToken: "file_retry", Status: "uploaded"}
	_, _ = bindLocalDocResourceChunk(runtime, "doxcn_retry", []*localDocResourceOutcome{outcome})
	if outcome.Status != "bound" || len(clientTokens) != 2 || clientTokens[0] == "" || clientTokens[0] != clientTokens[1] {
		t.Fatalf("outcome=%#v client_tokens=%#v", outcome, clientTokens)
	}
	if len(waits) != 1 || waits[0] != localDocResourceBindInterval {
		t.Fatalf("retry waits = %#v", waits)
	}
}

func TestBindLocalDocResourceAmbiguousSuccessDropsStaleRevision(t *testing.T) {
	factory, _, _, reg := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-bind-ambiguous-success"))
	runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-bind-ambiguous-success"), factory, core.AsUser)
	reg.Register(&httpmock.Stub{Method: "PATCH", URL: "/open-apis/docx/v1/documents/doxcn_ambiguous/blocks/batch_update", Body: map[string]interface{}{"code": localDocResourceUploadRateLimitCode, "msg": "rate limited"}})
	reg.Register(&httpmock.Stub{Method: "GET", URL: "/open-apis/docx/v1/documents/doxcn_ambiguous/blocks/blk_ambiguous", Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"block": map[string]interface{}{"image": map[string]interface{}{"token": "file_ambiguous"}}}}})
	outcome := &localDocResourceOutcome{Resource: localDocResource{Kind: localDocResourceImage}, BlockID: "blk_ambiguous", FileToken: "file_ambiguous", Status: "uploaded"}
	revision, revisionKnown := bindLocalDocResourceChunk(runtime, "doxcn_ambiguous", []*localDocResourceOutcome{outcome})
	if revision != nil || revisionKnown || outcome.Status != "bound" {
		t.Fatalf("revision=%#v known=%v outcome=%#v", revision, revisionKnown, outcome)
	}
}

func TestCleanupLocalDocResourcePlaceholdersRevalidatesToken(t *testing.T) {
	tests := []struct {
		name         string
		getStatus    int
		block        map[string]interface{}
		wantDelete   bool
		wantStatus   string
		wantCleanup  string
		wantKnown    bool
		wantRevision interface{}
	}{
		{name: "empty", block: map[string]interface{}{"block_type": 27, "image": map[string]interface{}{"token": ""}}, wantDelete: true, wantStatus: "bind_failed", wantCleanup: "succeeded", wantKnown: true, wantRevision: int64(8)},
		{name: "ours", block: map[string]interface{}{"block_type": 27, "image": map[string]interface{}{"token": "file_ours"}}, wantStatus: "bound", wantCleanup: "not_needed"},
		{name: "other", block: map[string]interface{}{"block_type": 27, "image": map[string]interface{}{"token": "file_other"}}, wantStatus: "bind_conflict", wantCleanup: "skipped_conflict"},
		{name: "other_kind_token", block: map[string]interface{}{"block_type": 27, "image": map[string]interface{}{"token": ""}, "file": map[string]interface{}{"token": "file_hidden"}}, wantStatus: "bind_conflict", wantCleanup: "skipped_conflict"},
		{name: "type_mismatch", block: map[string]interface{}{"block_type": 23, "file": map[string]interface{}{"token": ""}}, wantStatus: "bind_ambiguous", wantCleanup: "skipped_ambiguous"},
		{name: "type_unknown", block: map[string]interface{}{"block_type": 999, "image": map[string]interface{}{"token": ""}}, wantStatus: "bind_ambiguous", wantCleanup: "skipped_ambiguous"},
		{name: "get_fail", getStatus: 503, wantStatus: "bind_ambiguous", wantCleanup: "skipped_ambiguous"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, _, _, reg := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-cleanup-"+tt.name))
			runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-cleanup-"+tt.name), factory, core.AsUser)
			getStub := &httpmock.Stub{Method: "GET", URL: "/open-apis/docx/v1/documents/doxcn_cleanup/blocks/blk_cleanup"}
			if tt.getStatus != 0 {
				getStub.Status = tt.getStatus
				getStub.RawBody = []byte("temporary")
			} else {
				getStub.Body = map[string]interface{}{"code": 0, "data": map[string]interface{}{"block": tt.block}}
			}
			reg.Register(getStub)
			deleteCalls := 0
			var deleteStub *httpmock.Stub
			if tt.wantDelete {
				deleteStub = &httpmock.Stub{Method: "PUT", URL: "/open-apis/docs_ai/v1/documents/doxcn_cleanup", Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"document": map[string]interface{}{"revision_id": 8}}}, OnMatch: func(*http.Request) { deleteCalls++ }}
				reg.Register(deleteStub)
			}
			outcome := &localDocResourceOutcome{
				Resource:        localDocResource{Occurrence: 1, Kind: localDocResourceImage},
				BlockID:         "blk_cleanup",
				CleanupBlockIDs: []string{"blk_cleanup"},
				FileToken:       "file_ours",
				Status:          "bind_failed",
				CleanupStatus:   "pending",
				SafeToCleanup:   true,
			}
			revision, revisionKnown := cleanupLocalDocResourcePlaceholders(runtime, "doxcn_cleanup", []*localDocResourceOutcome{outcome}, float64(7))
			if got := deleteCalls > 0; got != tt.wantDelete {
				t.Fatalf("delete called=%v, want %v", got, tt.wantDelete)
			}
			if revisionKnown != tt.wantKnown || revision != tt.wantRevision {
				t.Fatalf("revision=%#v known=%v, want revision=%#v known=%v", revision, revisionKnown, tt.wantRevision, tt.wantKnown)
			}
			if outcome.Status != tt.wantStatus || outcome.CleanupStatus != tt.wantCleanup || outcome.SafeToCleanup {
				t.Fatalf("outcome=%#v, want status=%s cleanup=%s safe=false", outcome, tt.wantStatus, tt.wantCleanup)
			}
			if tt.wantDelete {
				var body map[string]interface{}
				if err := json.Unmarshal(deleteStub.CapturedBody, &body); err != nil {
					t.Fatalf("decode cleanup body: %v", err)
				}
				if body["revision_id"] != float64(7) {
					t.Fatalf("cleanup revision_id=%#v, want 7", body["revision_id"])
				}
			}
		})
	}
}

func TestCleanupLocalDocResourcePlaceholdersRequiresKnownRevision(t *testing.T) {
	factory, _, _, _ := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-cleanup-no-revision"))
	runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-cleanup-no-revision"), factory, core.AsUser)
	outcome := &localDocResourceOutcome{Resource: localDocResource{Kind: localDocResourceImage}, CleanupBlockIDs: []string{"blk_cleanup"}, Status: "upload_failed", CleanupStatus: "pending", SafeToCleanup: true}
	revision, revisionKnown := cleanupLocalDocResourcePlaceholders(runtime, "doxcn_cleanup", []*localDocResourceOutcome{outcome}, nil)
	if revision != nil || revisionKnown || outcome.CleanupStatus != "skipped_ambiguous" || outcome.SafeToCleanup {
		t.Fatalf("revision=%#v known=%v outcome=%#v", revision, revisionKnown, outcome)
	}
}

func TestCleanupLocalDocResourceFileDeletesFigureParent(t *testing.T) {
	factory, _, _, reg := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-cleanup-file-parent"))
	runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-cleanup-file-parent"), factory, core.AsUser)
	reg.Register(&httpmock.Stub{
		Method: "GET",
		URL:    "/open-apis/docx/v1/documents/doxcn_cleanup/blocks/blk_file",
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"block": map[string]interface{}{
			"block_id":   "blk_file",
			"block_type": 23,
			"parent_id":  "blk_figure",
			"file":       map[string]interface{}{"token": ""},
		}}},
	})
	deleteStub := &httpmock.Stub{
		Method: "PUT",
		URL:    "/open-apis/docs_ai/v1/documents/doxcn_cleanup",
		Body:   map[string]interface{}{"code": 0, "data": map[string]interface{}{"document": map[string]interface{}{"revision_id": 8}}},
	}
	reg.Register(deleteStub)
	outcome := &localDocResourceOutcome{
		Resource:        localDocResource{Occurrence: 1, Kind: localDocResourceFile},
		BlockID:         "blk_file",
		CleanupBlockIDs: []string{"blk_file"},
		Status:          "upload_failed",
		CleanupStatus:   "pending",
		SafeToCleanup:   true,
	}

	revision, revisionKnown := cleanupLocalDocResourcePlaceholders(runtime, "doxcn_cleanup", []*localDocResourceOutcome{outcome}, float64(7))

	if !revisionKnown || revision != int64(8) || outcome.CleanupStatus != "succeeded" {
		t.Fatalf("revision=%#v known=%v outcome=%#v", revision, revisionKnown, outcome)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(deleteStub.CapturedBody, &body); err != nil {
		t.Fatalf("decode cleanup body: %v", err)
	}
	if body["block_id"] != "blk_figure" {
		t.Fatalf("cleanup block_id=%#v, want figure parent", body["block_id"])
	}
}

func TestCleanupLocalDocResourceFileWithoutFigureParentIsPreserved(t *testing.T) {
	factory, _, _, reg := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-cleanup-file-no-parent"))
	runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-cleanup-file-no-parent"), factory, core.AsUser)
	reg.Register(&httpmock.Stub{
		Method: "GET",
		URL:    "/open-apis/docx/v1/documents/doxcn_cleanup/blocks/blk_file",
		Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"block": map[string]interface{}{
			"block_id":   "blk_file",
			"block_type": 23,
			"file":       map[string]interface{}{"token": ""},
		}}},
	})
	outcome := &localDocResourceOutcome{
		Resource:        localDocResource{Occurrence: 1, Kind: localDocResourceFile},
		BlockID:         "blk_file",
		CleanupBlockIDs: []string{"blk_file"},
		Status:          "upload_failed",
		CleanupStatus:   "pending",
		SafeToCleanup:   true,
	}

	revision, revisionKnown := cleanupLocalDocResourcePlaceholders(runtime, "doxcn_cleanup", []*localDocResourceOutcome{outcome}, float64(7))

	if revision != nil || revisionKnown || outcome.Status != "bind_ambiguous" || outcome.CleanupStatus != "skipped_ambiguous" || outcome.SafeToCleanup {
		t.Fatalf("revision=%#v known=%v outcome=%#v", revision, revisionKnown, outcome)
	}
}

func TestDocAPINullDataIsTypedInvalidResponse(t *testing.T) {
	factory, _, _, reg := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-null-data"))
	runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-null-data"), factory, core.AsUser)
	reg.Register(&httpmock.Stub{Method: "PUT", URL: "/open-apis/docs_ai/v1/documents/doxcn_null", Body: map[string]interface{}{"code": 0, "data": nil}})
	_, err := doDocAPI(runtime, "PUT", "/open-apis/docs_ai/v1/documents/doxcn_null", map[string]interface{}{})
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Subtype != errs.SubtypeInvalidResponse {
		t.Fatalf("error = %T %v, want typed invalid_response", err, err)
	}
}

func TestFinalizeLocalDocResourcesTOCTOUErrorDoesNotLeakCWD(t *testing.T) {
	dir := t.TempDir()
	cmdutil.TestChdir(t, dir)
	if err := os.WriteFile("vanished.png", []byte("png-data"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	factory, stdout, _, reg := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-toctou"))
	runtime := common.TestNewRuntimeContextForAPI(context.Background(), &cobra.Command{Use: "test"}, docsTestConfigWithAppID("local-toctou"), factory, core.AsUser)
	_, resources, err := prepareLocalDocResources(runtime, "xml", `<img path="@vanished.png"/>`)
	if err != nil {
		t.Fatalf("prepareLocalDocResources: %v", err)
	}
	if err := os.Remove("vanished.png"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	block := map[string]interface{}{"block_id": "blk_vanished", "block_type": "image", "block_token": resources[0].Marker}
	data := map[string]interface{}{"document": map[string]interface{}{"revision_id": 1, "new_blocks": []interface{}{block}}}
	reg.Register(&httpmock.Stub{Method: "GET", URL: "/open-apis/docx/v1/documents/doxcn_toctou/blocks/blk_vanished", Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"block": map[string]interface{}{"block_type": 27, "image": map[string]interface{}{"token": ""}}}}})
	reg.Register(&httpmock.Stub{Method: "PUT", URL: "/open-apis/docs_ai/v1/documents/doxcn_toctou", Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{"document": map[string]interface{}{"revision_id": 2}}}})
	if err := finalizeLocalDocResources(runtime, "doxcn_toctou", data, resources); err == nil {
		t.Fatal("finalizeLocalDocResources error = nil, want partial failure")
	}
	public, _ := json.Marshal(data)
	public = append(public, stdout.Bytes()...)
	for _, secret := range []string{dir, "vanished.png", resources[0].Marker} {
		if strings.Contains(string(public), secret) {
			t.Fatalf("partial response leaked %q: %s", secret, public)
		}
	}
}

func TestLocalDocResourceDryRunIncludesRouteExtraPartSizeAndTwentyBatch(t *testing.T) {
	resources := make([]localDocResource, 21)
	for i := range resources {
		resources[i] = localDocResource{Occurrence: i + 1, Kind: localDocResourceImage, Size: common.MaxDriveMediaUploadSinglePartSize + 1}
	}
	dry := appendLocalDocResourcesDryRun(common.NewDryRunAPI(), "doc/with space", resources)
	decoded := decodeDocDryRun(t, dry)
	patches := 0
	for _, api := range decoded.API {
		switch {
		case api.URL == "/open-apis/drive/v1/medias/upload_prepare":
			if !strings.Contains(fmt.Sprint(api.Body["extra"]), "drive_route_token") || api.Body["size"] == nil {
				t.Fatalf("upload_prepare body = %#v", api.Body)
			}
		case api.URL == "/open-apis/drive/v1/medias/upload_part":
			if api.Body["size"] == nil {
				t.Fatalf("upload_part body missing size: %#v", api.Body)
			}
		case strings.Contains(api.URL, "/blocks/batch_update"):
			patches++
			if !strings.Contains(api.URL, "doc%2Fwith%20space") {
				t.Fatalf("batch URL is not encoded: %s", api.URL)
			}
		}
	}
	if patches != 2 {
		t.Fatalf("batch PATCH count = %d, want 2", patches)
	}
}

func TestDocsUpdateLocalResourceWikiDryRunResolvesDocxFirst(t *testing.T) {
	dir := t.TempDir()
	cmdutil.TestChdir(t, dir)
	if err := os.WriteFile("diagram.png", []byte("png-data"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runtime := newUpdateShortcutTestRuntime(t, "", map[string]string{
		"doc":     "https://example.larksuite.com/wiki/wikcn_local",
		"command": "append",
		"content": `<img path="@diagram.png"/>`,
	})
	dry := decodeDocDryRun(t, dryRunUpdateV2(context.Background(), runtime))
	if len(dry.API) < 2 || dry.API[0].URL != "/open-apis/wiki/v2/spaces/get_node" {
		t.Fatalf("dry-run must resolve wiki first: %#v", dry.API)
	}
	if got := dry.API[1].URL; got != "/open-apis/docs_ai/v1/documents/%3Cresolved_docx_token%3E" {
		t.Fatalf("docs_ai URL = %q", got)
	}
	for _, api := range dry.API[2:] {
		if strings.Contains(api.URL, "wikcn_local") {
			t.Fatalf("post-resolve API still uses wiki node token: %s", api.URL)
		}
	}
}

func TestLocalDocResourceUpdateCommandsAreNonDestructive(t *testing.T) {
	resources := []localDocResource{{Kind: localDocResourceImage}}
	for _, command := range []string{"str_replace", "block_replace", "overwrite"} {
		if err := validateLocalDocResourceUpdateCommand(command, resources); err == nil {
			t.Fatalf("command %s accepted local resources", command)
		}
	}
	for _, command := range []string{"append", "block_insert_after"} {
		if err := validateLocalDocResourceUpdateCommand(command, resources); err != nil {
			t.Fatalf("command %s rejected: %v", command, err)
		}
	}
}

func requestCountFromLocalDocBatchBody(t *testing.T, raw []byte) int {
	t.Helper()
	var body struct {
		Requests []interface{} `json:"requests"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode batch body: %v; %s", err, raw)
	}
	return len(body.Requests)
}

func newLocalDocResourceTestRuntime(t *testing.T, files map[string]string) *common.RuntimeContext {
	t.Helper()
	dir := t.TempDir()
	cmdutil.TestChdir(t, dir)
	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	factory, _, _, _ := cmdutil.TestFactory(t, docsTestConfigWithAppID("local-resource-test"))
	return common.TestNewRuntimeContextForAPI(
		context.Background(),
		&cobra.Command{Use: "docs local-resource-test"},
		docsTestConfigWithAppID("local-resource-test"),
		factory,
		core.AsUser,
	)
}

func localDocResourcePNG(t *testing.T, width, height int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("encode %dx%d PNG: %v", width, height, err)
	}
	return buf.String()
}

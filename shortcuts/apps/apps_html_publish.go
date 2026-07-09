// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/larksuite/cli/extension/fileio"
	"github.com/larksuite/cli/internal/client"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

// AppsHTMLPublish packs --path as tar.gz and uploads + publishes via one multipart POST.
var AppsHTMLPublish = common.Shortcut{
	Service:     appsService,
	Command:     "+html-publish",
	Description: "Publish HTML to an app (single multipart POST returns the access URL)",
	Risk:        "write",
	Tips: []string{
		"Example: lark-cli apps +html-publish --app-id <app_id> --path ./dist",
		"Example: lark-cli apps +html-publish --app-id <app_id> --path ./site --dry-run",
	},
	Scopes:    []string{"spark:app:write"},
	AuthTypes: []string{"user"},
	HasFormat: true,
	Flags: []common.Flag{
		{Name: "app-id", Desc: "app ID", Required: true},
		{Name: "path", Desc: "path to HTML file or directory", Required: true},
		{Name: "allow-sensitive", Type: "bool", Desc: "skip the credential-file scan (allow .env / .npmrc / .aws/credentials / etc. in the publish payload)"},
	},
	Validate: func(ctx context.Context, rctx *common.RuntimeContext) error {
		if strings.TrimSpace(rctx.Str("app-id")) == "" {
			return appsValidationParamError("--app-id", "--app-id is required")
		}
		path := strings.TrimSpace(rctx.Str("path"))
		if path == "" {
			return appsValidationParamError("--path", "--path is required")
		}
		// Block well-known credential files in the publish payload unless the
		// caller explicitly opts in. Lives in Validate (not DryRun) so that
		// `--dry-run` returns non-zero on hit — the framework runs Validate
		// before branching to DryRun/Execute, so both paths share this gate.
		if rctx.Bool("allow-sensitive") {
			return nil
		}
		candidates, err := walkHTMLPublishCandidates(rctx.FileIO(), path)
		if err != nil {
			// Don't fail Validate on walk errors (bad --path, etc.) — let
			// DryRun/Execute surface them in their own (richer) envelopes.
			return nil
		}
		var hits []string
		for _, c := range candidates {
			if isSensitiveCandidate(path, c) {
				hits = append(hits, c.RelPath)
			}
		}
		if len(hits) > 0 {
			return sensitiveCandidatesError(hits)
		}
		return nil
	},
	DryRun: func(ctx context.Context, rctx *common.RuntimeContext) *common.DryRunAPI {
		appID := strings.TrimSpace(rctx.Str("app-id"))
		path := strings.TrimSpace(rctx.Str("path"))
		dry := common.NewDryRunAPI()
		dry.Desc("Upload tar.gz + publish HTML (multipart, returns url)")
		dry.POST(fmt.Sprintf("%s/apps/%s/upload_and_release_html_code", apiBasePath, validate.EncodePathSegment(appID))).
			Set("content_type", "multipart/form-data")

		candidates, err := walkHTMLPublishCandidates(rctx.FileIO(), path)
		if err != nil {
			dry.Set("path_error", err.Error())
			return dry
		}
		if err := ensureIndexHTML(candidates); err != nil {
			// Surface the same failure Execute would hit, but as a structured
			// envelope field so dry-run still exits 0 (matches repo convention
			// for dry-run "advisory preview" semantics).
			dry.Set("validation_error", err.Error())
		}
		if hits := oversizeHTMLFiles(candidates); len(hits) > 0 {
			dry.Set("oversize_html", hits)
		}
		dry.Set("file_count", len(candidates))
		var totalSize int64
		names := make([]string, 0, len(candidates))
		for _, c := range candidates {
			totalSize += c.Size
			names = append(names, c.RelPath)
		}
		dry.Set("total_size_bytes", totalSize)
		dry.Set("files", names)
		// Sensitive-file rejection lives in Validate (so dry-run exits non-zero
		// on hit). When --allow-sensitive is set, still surface the list here
		// as an info field so the caller sees what was waived.
		if rctx.Bool("allow-sensitive") {
			var waived []string
			for _, c := range candidates {
				if isSensitiveCandidate(path, c) {
					waived = append(waived, c.RelPath)
				}
			}
			if len(waived) > 0 {
				dry.Set("sensitive_waived", waived)
				dry.Set("sensitive_waived_summary", fmt.Sprintf("%d credential file(s) included because --allow-sensitive is set", len(waived)))
			}
		}
		return dry
	},
	Execute: func(ctx context.Context, rctx *common.RuntimeContext) error {
		spec := appsHTMLPublishSpec{
			AppID: strings.TrimSpace(rctx.Str("app-id")),
			Path:  strings.TrimSpace(rctx.Str("path")),
		}

		appType := queryAppType(ctx, rctx, spec.AppID)

		var out map[string]interface{}
		var err error
		if appType == "modern_html" {
			out, err = runHTMLPublishTOS(ctx, rctx, spec)
		} else {
			client := appsHTMLPublishAPI{runtime: rctx}
			out, err = runHTMLPublish(ctx, rctx.FileIO(), client, spec)
		}
		if err != nil {
			return err
		}
		out["app_id"] = spec.AppID
		rctx.OutFormat(out, nil, func(w io.Writer) {
			fmt.Fprintf(w, "app_id: %s\n", spec.AppID)
			if url, ok := out["url"].(string); ok && url != "" {
				fmt.Fprintf(w, "url: %s\n", url)
			}
			if tosPath, ok := out["tos_path"].(string); ok && tosPath != "" {
				fmt.Fprintf(w, "tos_path: %s\n", tosPath)
			}
		})
		return nil
	},
}

type appsHTMLPublishSpec struct {
	AppID string
	Path  string
}

// maxSensitiveListInError caps how many credential-file matches we list inline
// in the validation error, so the message stays readable when a misconfigured
// payload has many hits (e.g. a directory tree accidentally containing
// per-environment .env.* files for every stage).
const maxSensitiveListInError = 5

// truncatedJoin joins items with ", ", capping at max entries and appending
// "(and N more)" for the remainder, so an inline error list stays readable when
// a payload has many hits.
func truncatedJoin(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + fmt.Sprintf(" (and %d more)", len(items)-max)
}

// sensitiveCandidatesError builds the Validate-time rejection when --path
// contains credential files and --allow-sensitive was not set.
func sensitiveCandidatesError(hits []string) error {
	return appsValidationParamError("--path",
		"--path contains %d credential file(s) that should not be published: %s",
		len(hits), truncatedJoin(hits, maxSensitiveListInError)).
		WithHint("remove these files from the publish payload, OR pass --allow-sensitive if shipping them is intentional (e.g. a docs site demoing credential-file formats)")
}

// maxHTMLPublishTarballBytes 是 client 端 tar.gz 包体上限，对齐 OAPI 设计 20MB 约束。
// 用 var 而非 const，便于单测调小覆盖拦截路径。
var maxHTMLPublishTarballBytes int64 = 20 * 1024 * 1024

// maxHTMLPublishRawBytes caps the total UNCOMPRESSED candidate size before
// tar+gzip writes them into the in-memory buffer. Defends against
// highly-compressible "decompression bomb" inputs (e.g. 50GB of zeros)
// that would balloon process memory before the gzip-after check fires.
// 200MB is much higher than any plausible legitimate HTML/static-site
// payload but low enough to stay well under typical container memory.
// Mutable for tests.
var maxHTMLPublishRawBytes int64 = 200 * 1024 * 1024

// maxHTMLPublishSingleHTMLFileBytes 单个 .html 文件上限，对齐妙搭服务端 10MB 约束。
// 用 var 而非 const，便于单测调小覆盖拦截路径。
var maxHTMLPublishSingleHTMLFileBytes int64 = 10 * 1024 * 1024

// oversizeHTMLFiles 返回 candidates 中扩展名为 .html（大小写不敏感）且单个 Size 超过
// maxHTMLPublishSingleHTMLFileBytes 的 RelPath 列表。只针对 .html 文件，不波及图片/字体/JS。
func oversizeHTMLFiles(candidates []htmlPublishCandidate) []string {
	var hits []string
	for _, c := range candidates {
		if strings.EqualFold(filepath.Ext(c.RelPath), ".html") && c.Size > maxHTMLPublishSingleHTMLFileBytes {
			hits = append(hits, c.RelPath)
		}
	}
	return hits
}

// oversizeHTMLFilesError 构造单文件超限的 Validate 风格拒绝。
func oversizeHTMLFilesError(hits []string) error {
	return appsValidationParamError("--path",
		"--path contains %d HTML file(s) exceeding the %d bytes (10MB) per-file limit: %s",
		len(hits), maxHTMLPublishSingleHTMLFileBytes, truncatedJoin(hits, maxSensitiveListInError)).
		WithHint("split or trim oversized HTML file(s); the 10MB cap applies to each single .html file")
}

// ensureIndexHTML 要求 walker 抓到的 candidates 里必须含 index.html。
// 目录形态：根目录下必须有 index.html。
// 单文件形态：文件名必须就是 index.html。
// 妙搭服务端用 index.html 作为应用入口。
func ensureIndexHTML(candidates []htmlPublishCandidate) error {
	for _, c := range candidates {
		if c.RelPath == "index.html" {
			return nil
		}
	}
	return appsFailedPreconditionParamError("--path", "--path is missing index.html").
		WithHint("index.html is the app entrypoint; for a directory put index.html at the root, or pass a single file named index.html")
}

func runHTMLPublish(ctx context.Context, fio fileio.FileIO, publisher appsHTMLPublishClient, spec appsHTMLPublishSpec) (map[string]interface{}, error) {
	candidates, err := walkHTMLPublishCandidates(fio, spec.Path)
	if err != nil {
		return nil, err
	}
	if err := ensureIndexHTML(candidates); err != nil {
		return nil, err
	}
	if hits := oversizeHTMLFiles(candidates); len(hits) > 0 {
		return nil, oversizeHTMLFilesError(hits)
	}
	var rawTotal int64
	for _, c := range candidates {
		rawTotal += c.Size
	}
	if rawTotal > maxHTMLPublishRawBytes {
		return nil, appsValidationParamError("--path",
			"--path total raw bytes %d exceeds %d bytes limit (uncompressed pre-pack cap)", rawTotal, maxHTMLPublishRawBytes).
			WithHint("reduce --path contents or choose a smaller subdirectory before packaging")
	}
	tarball, err := buildHTMLPublishTarball(fio, candidates)
	if err != nil {
		return nil, err
	}

	if tarball.Size > maxHTMLPublishTarballBytes {
		return nil, appsValidationParamError("--path",
			"packed tar.gz size %d bytes exceeds %d bytes limit", tarball.Size, maxHTMLPublishTarballBytes).
			WithHint("reduce --path contents, remove unrelated large files, then retry")
	}

	resp, err := publisher.HTMLPublish(ctx, spec.AppID, tarball)
	if err != nil {
		return nil, client.WrapDoAPIError(err)
	}

	out := map[string]interface{}{}
	if resp.URL != "" {
		out["url"] = resp.URL
	}
	return out, nil
}

// runHTMLPublishTOS handles the modern_html publish path: validate → tar.gz →
// call pre_release to get TOS upload URL → upload tar.gz to TOS → return
// tos_path for +release-create --tos-path.
func runHTMLPublishTOS(ctx context.Context, rctx *common.RuntimeContext, spec appsHTMLPublishSpec) (map[string]interface{}, error) {
	candidates, err := walkHTMLPublishCandidates(rctx.FileIO(), spec.Path)
	if err != nil {
		return nil, err
	}
	if err := ensureIndexHTML(candidates); err != nil {
		return nil, err
	}
	if hits := oversizeHTMLFiles(candidates); len(hits) > 0 {
		return nil, oversizeHTMLFilesError(hits)
	}
	var rawTotal int64
	for _, c := range candidates {
		rawTotal += c.Size
	}
	if rawTotal > maxHTMLPublishRawBytes {
		return nil, appsValidationParamError("--path",
			"--path total raw bytes %d exceeds %d bytes limit (uncompressed pre-pack cap)", rawTotal, maxHTMLPublishRawBytes).
			WithHint("reduce --path contents or choose a smaller subdirectory before packaging")
	}

	tarball, err := buildHTMLPublishTarball(rctx.FileIO(), candidates)
	if err != nil {
		return nil, err
	}
	if tarball.Size > maxHTMLPublishTarballBytes {
		return nil, appsValidationParamError("--path",
			"packed tar.gz size %d bytes exceeds %d bytes limit", tarball.Size, maxHTMLPublishTarballBytes).
			WithHint("reduce --path contents, remove unrelated large files, then retry")
	}

	// Step 1: call pre_release to get TOS upload URL and tos_path.
	preReleasePath := fmt.Sprintf("%s/apps/%s/pre_release", apiBasePath, validate.EncodePathSegment(spec.AppID))
	preData, err := rctx.CallAPITyped("POST", preReleasePath, nil, nil)
	if err != nil {
		return nil, err
	}
	params, _ := preData["params"].(map[string]interface{})
	if params == nil {
		return nil, appsSubprocessEnvelopeError("pre_release returned no params")
	}
	uploadURL, _ := params["upload_url"].(string)
	tosPath, _ := params["tos_path"].(string)
	if uploadURL == "" || tosPath == "" {
		return nil, appsSubprocessEnvelopeError("pre_release params missing upload_url or tos_path")
	}

	// Step 2: upload tar.gz to TOS.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(tarball.Body))
	if err != nil {
		return nil, appsFileIOError(err, "create TOS upload request: %v", err)
	}
	req.Header.Set("Content-Type", "application/gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, appsExternalToolError(err, "TOS upload failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, appsExternalToolError(nil, "TOS upload returned HTTP %d", resp.StatusCode)
	}

	return map[string]interface{}{
		"app_id":   spec.AppID,
		"tos_path": tosPath,
	}, nil
}

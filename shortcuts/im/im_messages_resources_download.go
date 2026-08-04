// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/extension/fileio"
	"github.com/larksuite/cli/internal/client"
	"github.com/larksuite/cli/shortcuts/common"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

var ImMessagesResourcesDownload = common.Shortcut{
	Service:     "im",
	Command:     "+messages-resources-download",
	Description: "Download images/files from a message; user/bot; downloads image/file resources by message-id and file-key to a safe relative output path",
	Risk:        "write",
	Scopes:      []string{"im:message:readonly"},
	AuthTypes:   []string{"user", "bot"},
	Flags: []common.Flag{
		{Name: "message-id", Desc: "message ID (om_xxx)", Required: true},
		{Name: "file-key", Desc: "resource key (img_xxx or file_xxx)", Required: true},
		{Name: "type", Desc: "resource type (image or file)", Required: true, Enum: []string{"image", "file"}},
		{Name: "output", Desc: "local save path (relative only, no .. traversal); when omitted, uses the server's Content-Disposition filename if available, otherwise file_key; extension is inferred from Content-Disposition or Content-Type if not provided"},
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		fileKey := runtime.Str("file-key")
		outputPath := runtime.Str("output")
		if outputPath == "" {
			outputPath = fileKey
		}
		return common.NewDryRunAPI().
			GET("/open-apis/im/v1/messages/:message_id/resources/:file_key").
			Params(map[string]interface{}{"type": runtime.Str("type")}).
			Set("message_id", runtime.Str("message-id")).Set("file_key", fileKey).
			Set("type", runtime.Str("type")).Set("output", outputPath)
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		if messageId := runtime.Str("message-id"); messageId == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "--message-id is required (om_xxx)").WithParam("--message-id")
		} else if _, err := validateMessageID(messageId); err != nil {
			return err
		}
		relPath, err := normalizeDownloadOutputPath(runtime.Str("file-key"), runtime.Str("output"))
		if err != nil {
			return err
		}
		if _, err := runtime.ResolveSavePath(relPath); err != nil {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "unsafe output path: %s", err).WithParam("--output").WithCause(err)
		}
		return nil
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		messageId := runtime.Str("message-id")
		fileKey := runtime.Str("file-key")
		fileType := runtime.Str("type")
		relPath, err := normalizeDownloadOutputPath(fileKey, runtime.Str("output"))
		if err != nil {
			return err
		}
		if _, err := runtime.ResolveSavePath(relPath); err != nil {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "unsafe output path: %s", err).WithParam("--output").WithCause(err)
		}

		// With an explicit --output, keep that basename (append only an
		// extension); without it, adopt the server's original filename.
		preserveBasename := runtime.Str("output") != ""
		finalPath, sizeBytes, err := downloadIMResourceToPath(ctx, runtime, messageId, fileKey, fileType, relPath, preserveBasename)
		if err != nil {
			return err
		}

		runtime.Out(map[string]interface{}{"saved_path": finalPath, "size_bytes": sizeBytes}, nil)
		return nil
	},
}

func normalizeDownloadOutputPath(fileKey, outputPath string) (string, error) {
	fileKey = strings.TrimSpace(fileKey)
	if fileKey == "" {
		return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "file-key cannot be empty").WithParam("--file-key")
	}
	if strings.ContainsAny(fileKey, "/\\") {
		return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "file-key cannot contain path separators").WithParam("--file-key")
	}
	if outputPath == "" {
		return fileKey, nil
	}
	outputPath = filepath.Clean(strings.TrimSpace(outputPath))
	if outputPath == "." {
		return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "path cannot be empty").WithParam("--output")
	}
	if filepath.IsAbs(outputPath) {
		return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "absolute paths are not allowed").WithParam("--output")
	}
	if outputPath == ".." || strings.HasPrefix(outputPath, ".."+string(filepath.Separator)) {
		return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "path cannot escape the current working directory").WithParam("--output")
	}
	return outputPath, nil
}

const (
	defaultIMResourceDownloadTimeout = 120 * time.Second
	probeChunkSize                   = int64(128 * 1024)
	normalChunkSize                  = int64(8 * 1024 * 1024)
	imDownloadRequestRetries         = 2
	imDownloadRetryDelay             = 300 * time.Millisecond
)

var imMimeToExt = map[string]string{
	"image/png":                    ".png",
	"image/jpeg":                   ".jpg",
	"image/gif":                    ".gif",
	"image/webp":                   ".webp",
	"image/svg+xml":                ".svg",
	"application/pdf":              ".pdf",
	"video/mp4":                    ".mp4",
	"video/3gpp":                   ".3gp",
	"video/x-msvideo":              ".avi",
	"audio/mpeg":                   ".mp3",
	"audio/ogg":                    ".ogg",
	"audio/wav":                    ".wav",
	"text/plain":                   ".txt",
	"text/html":                    ".html",
	"text/css":                     ".css",
	"text/csv":                     ".csv",
	"application/zip":              ".zip",
	"application/x-zip-compressed": ".zip",
	"application/x-rar-compressed": ".rar",
	"application/json":             ".json",
	"application/xml":              ".xml",
	"application/octet-stream":     ".bin",
	"application/msword":           ".doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
	"application/vnd.ms-excel": ".xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
	"application/vnd.ms-powerpoint":                                             ".ppt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
}

// rangeChunkReader streams a resource that the server serves in byte ranges.
//
// Every offset it tracks comes from the Content-Range the server sent, never
// from the range this client asked for. Deriving nextOffset from the request
// is what allowed a 206 carrying the wrong slice to be written at the wrong
// place: the byte count still added up, so the final size check passed and the
// command reported success on a corrupt file.
type rangeChunkReader struct {
	ctx       context.Context
	runtime   *common.RuntimeContext
	messageID string
	fileKey   string
	fileType  string
	// validator pins later requests to the representation the probe read, via
	// If-Range. Empty when the server offered no usable validator.
	validator  string
	totalSize  int64
	delivered  int64
	current    io.ReadCloser
	chunkWant  int64
	chunkRead  int64
	nextOffset int64
}

func newRangeChunkReader(
	ctx context.Context,
	runtime *common.RuntimeContext,
	messageID, fileKey, fileType string,
	probeBody io.ReadCloser,
	probeRange contentRange,
	validator string,
) *rangeChunkReader {
	return &rangeChunkReader{
		ctx:        ctx,
		runtime:    runtime,
		messageID:  messageID,
		fileKey:    fileKey,
		fileType:   fileType,
		validator:  validator,
		totalSize:  probeRange.total,
		current:    probeBody,
		chunkWant:  probeRange.length(),
		nextOffset: probeRange.end + 1,
	}
}

func (r *rangeChunkReader) Read(p []byte) (int, error) {
	for {
		if r.current != nil {
			n, err := r.current.Read(p)
			r.delivered += int64(n)
			r.chunkRead += int64(n)

			if r.delivered > r.totalSize {
				if err == io.EOF {
					closeErr := r.current.Close()
					r.current = nil
					if closeErr != nil {
						return 0, closeErr
					}
				}
				return 0, errs.NewNetworkError(errs.SubtypeNetworkProtocol, "chunk overflow: delivered %d, expected %d", r.delivered, r.totalSize)
			}
			// The body outran the Content-Range it came with, so the response
			// contradicts itself and we cannot place these bytes.
			if r.chunkRead > r.chunkWant {
				_ = r.current.Close()
				r.current = nil
				return 0, errs.NewNetworkError(errs.SubtypeNetworkProtocol, "range response delivered more than the %d bytes its Content-Range declared", r.chunkWant)
			}

			switch err {
			case nil:
				return n, nil
			case io.EOF:
				closeErr := r.current.Close()
				r.current = nil
				// A short body is an integrity failure, so it outranks any
				// error from closing the connection.
				if r.chunkRead != r.chunkWant {
					return 0, errs.NewNetworkError(errs.SubtypeNetworkProtocol, "range response delivered %d bytes, want the %d bytes its Content-Range declared", r.chunkRead, r.chunkWant)
				}
				if closeErr != nil {
					return n, closeErr
				}
				if r.delivered == r.totalSize {
					if n > 0 {
						return n, nil
					}
					return 0, io.EOF
				}
				r.chunkRead = 0
				r.chunkWant = 0
				if n > 0 {
					return n, nil
				}
			default:
				return n, err
			}
		}

		if r.nextOffset >= r.totalSize {
			if r.delivered == r.totalSize {
				return 0, io.EOF
			}
			return 0, errs.NewNetworkError(errs.SubtypeNetworkTransport, "file size mismatch: expected %d, got %d", r.totalSize, r.delivered)
		}

		end := min(r.nextOffset+normalChunkSize-1, r.totalSize-1)
		headers := map[string]string{
			"Range": fmt.Sprintf("bytes=%d-%d", r.nextOffset, end),
		}
		if r.validator != "" {
			headers["If-Range"] = r.validator
		}
		resp, err := doIMResourceDownloadRequest(r.ctx, r.runtime, r.messageID, r.fileKey, r.fileType, headers)
		if err != nil {
			return 0, err
		}
		if resp.StatusCode >= 400 {
			defer resp.Body.Close()
			return 0, downloadResponseError(resp)
		}
		// If-Range did not match, so the server answered with the whole current
		// representation instead of the range we asked for: the resource changed
		// under us. Splicing this body onto what is already on disk would mix two
		// versions of the file, so stop instead.
		if resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return 0, errs.NewNetworkError(errs.SubtypeNetworkProtocol, "resource changed while downloading; run the command again to download the current version")
		}
		if resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			return 0, errs.NewNetworkError(errs.SubtypeNetworkTransport, "unexpected status code: %d", resp.StatusCode)
		}
		got, err := parseContentRange(resp.Header.Get("Content-Range"))
		if err != nil {
			resp.Body.Close()
			return 0, errs.NewNetworkError(errs.SubtypeNetworkProtocol, "invalid Content-Range header on range response: %s", err)
		}
		// Resume from what the server says it sent. A different end is fine — a
		// server may serve a shorter or a longer slice than requested, and the
		// next request simply continues from there. A different start would
		// silently skip or duplicate bytes, and a different total means this is
		// no longer the same file.
		if got.start != r.nextOffset {
			resp.Body.Close()
			return 0, errs.NewNetworkError(errs.SubtypeNetworkProtocol, "range response is %s, want it to resume at byte %d", got, r.nextOffset)
		}
		if got.total != r.totalSize {
			resp.Body.Close()
			return 0, errs.NewNetworkError(errs.SubtypeNetworkProtocol, "resource size changed while downloading: range response is %s, want total %d", got, r.totalSize)
		}

		r.current = resp.Body
		r.chunkWant = got.length()
		r.chunkRead = 0
		r.nextOffset = got.end + 1
	}
}

func (r *rangeChunkReader) Close() error {
	if r.current == nil {
		return nil
	}
	err := r.current.Close()
	r.current = nil
	return err
}

func initialIMResourceDownloadHeaders(fileType string) map[string]string {
	if fileType != "file" {
		return nil
	}
	return map[string]string{
		"Range": fmt.Sprintf("bytes=0-%d", probeChunkSize-1),
	}
}

func downloadIMResourceToPath(ctx context.Context, runtime *common.RuntimeContext, messageID, fileKey, fileType, outputPath string, preserveBasename bool) (string, int64, error) {
	downloadResp, err := doIMResourceDownloadRequest(ctx, runtime, messageID, fileKey, fileType, initialIMResourceDownloadHeaders(fileType))
	if err != nil {
		return "", 0, err
	}
	if downloadResp == nil {
		return "", 0, errs.NewNetworkError(errs.SubtypeNetworkTransport, "download failed: empty response")
	}

	if downloadResp.StatusCode >= 400 {
		defer downloadResp.Body.Close()
		return "", 0, downloadResponseError(downloadResp)
	}

	finalPath := resolveIMResourceDownloadPath(outputPath, downloadResp.Header.Get("Content-Type"), downloadResp.Header.Get("Content-Disposition"), preserveBasename)

	var (
		body      io.ReadCloser
		sizeBytes int64
	)
	switch downloadResp.StatusCode {
	case http.StatusPartialContent:
		firstRange, err := parseContentRange(downloadResp.Header.Get("Content-Range"))
		if err != nil {
			downloadResp.Body.Close()
			return "", 0, errs.NewNetworkError(errs.SubtypeNetworkProtocol, "invalid Content-Range header on range response: %s", err)
		}
		// The probe asked to start at byte 0; the same start-must-match rule that
		// guards every later chunk applies here. How far the first slice reaches
		// is up to the server.
		if firstRange.start != 0 {
			downloadResp.Body.Close()
			return "", 0, errs.NewNetworkError(errs.SubtypeNetworkProtocol, "range response is %s, want it to start at byte 0", firstRange)
		}
		body = newRangeChunkReader(ctx, runtime, messageID, fileKey, fileType, downloadResp.Body, firstRange, rangeValidator(downloadResp.Header))
		sizeBytes = firstRange.total

	case http.StatusOK:
		body = downloadResp.Body
		sizeBytes = downloadResp.ContentLength

	default:
		downloadResp.Body.Close()
		return "", 0, errs.NewNetworkError(errs.SubtypeNetworkTransport, "unexpected status code: %d", downloadResp.StatusCode)
	}
	defer body.Close()

	result, err := runtime.FileIO().Save(finalPath, fileio.SaveOptions{
		ContentType:   downloadResp.Header.Get("Content-Type"),
		ContentLength: sizeBytes,
	}, body)
	if err != nil {
		return "", 0, common.WrapSaveErrorTyped(err)
	}
	if sizeBytes >= 0 && result.Size() != sizeBytes {
		return "", 0, errs.NewNetworkError(errs.SubtypeNetworkTransport, "file size mismatch: expected %d, got %d", sizeBytes, result.Size())
	}
	savedPath, resolveErr := runtime.ResolveSavePath(finalPath)
	if resolveErr != nil || savedPath == "" {
		savedPath = finalPath
	}
	return savedPath, result.Size(), nil
}

// resolveIMResourceDownloadPath decides the on-disk path for a downloaded
// resource. preserveBasename controls how a server-provided
// Content-Disposition filename is used when safePath has no extension:
//   - false: adopt the server's original filename (replace the basename) — the
//     friendly single-file behavior for an explicit `+messages-resources-download`
//     with no --output.
//   - true: keep safePath's basename and only borrow the extension. Used both
//     when the user pinned --output and for batch --download-resources, where
//     safePath is keyed by the unique (file_key) and the basename MUST stay
//     unique — otherwise two resources whose servers return the same
//     Content-Disposition filename (e.g. download.bin) would resolve to the
//     same path and clobber each other concurrently.
func resolveIMResourceDownloadPath(safePath, contentType, contentDisposition string, preserveBasename bool) string {
	if filepath.Ext(safePath) != "" {
		return safePath
	}
	if cdFilename := parseContentDispositionFilename(contentDisposition); cdFilename != "" {
		if !preserveBasename {
			// Adopt the server's original filename.
			dir := filepath.Dir(safePath)
			if dir == "." {
				return cdFilename
			}
			return filepath.Join(dir, cdFilename)
		}
		// Keep the basename; only append the extension from the CD filename.
		if ext := filepath.Ext(cdFilename); ext != "" {
			return safePath + ext
		}
	}
	mimeType := strings.TrimSpace(strings.Split(contentType, ";")[0])
	if ext, ok := imMimeToExt[mimeType]; ok {
		return safePath + ext
	}
	return safePath
}

// parseContentDispositionFilename extracts and sanitizes the filename from a
// Content-Disposition header. It handles RFC 5987 encoded filenames (filename*)
// with priority over plain filename via the standard mime package.
// Returns an empty string if no valid filename can be extracted.
func parseContentDispositionFilename(header string) string {
	if header == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(params["filename"])
	if name == "" {
		return ""
	}
	// Strip any path component (Unix or Windows style) to prevent path traversal.
	if i := strings.LastIndexAny(name, "/\\"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" || name == "." || name == ".." {
		return ""
	}
	// Reject control characters (including null bytes).
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return name
}

func doIMResourceDownloadRequest(ctx context.Context, runtime *common.RuntimeContext, messageID, fileKey, fileType string, headers map[string]string) (*http.Response, error) {
	query := larkcore.QueryParams{}
	query.Set("type", fileType)

	headerValues := make(http.Header, len(headers))
	for key, value := range headers {
		headerValues.Set(key, value)
	}

	req := &larkcore.ApiReq{
		HttpMethod: http.MethodGet,
		ApiPath:    "/open-apis/im/v1/messages/:message_id/resources/:file_key",
		PathParams: larkcore.PathParams{
			"message_id": messageID,
			"file_key":   fileKey,
		},
		QueryParams: query,
	}

	var lastErr error
	for attempt := 0; attempt <= imDownloadRequestRetries; attempt++ {
		resp, err := runtime.DoAPIStream(ctx, req, client.WithTimeout(defaultIMResourceDownloadTimeout), client.WithHeaders(headerValues))
		if err == nil {
			return resp, nil
		}
		if ctx.Err() != nil {
			return nil, imContextError(ctx.Err())
		}
		lastErr = err
		if attempt == imDownloadRequestRetries {
			break
		}
		sleepIMDownloadRetry(ctx, attempt)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errs.NewNetworkError(errs.SubtypeNetworkTransport, "download request failed")
}

func sleepIMDownloadRetry(ctx context.Context, attempt int) {
	delay := imDownloadRetryDelay * (1 << uint(attempt))
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func downloadResponseError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if len(body) > 0 {
		return errs.NewNetworkError(errs.SubtypeNetworkTransport, "download failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return errs.NewNetworkError(errs.SubtypeNetworkTransport, "download failed: HTTP %d", resp.StatusCode)
}

// contentRange is a parsed `Content-Range: bytes start-end/total` header.
type contentRange struct {
	start int64
	end   int64
	total int64
}

func (cr contentRange) String() string {
	return fmt.Sprintf("bytes %d-%d/%d", cr.start, cr.end, cr.total)
}

// length is how many bytes the response body must carry to match this header.
func (cr contentRange) length() int64 {
	return cr.end - cr.start + 1
}

// rangeValidator returns the validator to pin later range requests to with
// If-Range, or "" when the server offers none usable. A weak entity-tag must
// not be sent in If-Range (RFC 9110 13.1.5), so it falls back to Last-Modified.
func rangeValidator(header http.Header) string {
	if etag := strings.TrimSpace(header.Get("ETag")); etag != "" && !strings.HasPrefix(etag, "W/") {
		return etag
	}
	return strings.TrimSpace(header.Get("Last-Modified"))
}

func parseContentRange(header string) (contentRange, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return contentRange{}, fmt.Errorf("content-range is empty") //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}
	if !strings.HasPrefix(header, "bytes ") {
		return contentRange{}, fmt.Errorf("unsupported content-range: %q", header) //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}

	parts := strings.SplitN(strings.TrimPrefix(header, "bytes "), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return contentRange{}, fmt.Errorf("unsupported content-range: %q", header) //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}
	if parts[0] == "*" {
		return contentRange{}, fmt.Errorf("unsupported content-range: %q", header) //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}
	if parts[1] == "*" {
		return contentRange{}, fmt.Errorf("unknown total size in content-range: %q", header) //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}

	bounds := strings.SplitN(parts[0], "-", 2)
	if len(bounds) != 2 || bounds[0] == "" || bounds[1] == "" {
		return contentRange{}, fmt.Errorf("unsupported content-range: %q", header) //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}

	start, err := strconv.ParseInt(bounds[0], 10, 64)
	if err != nil {
		return contentRange{}, fmt.Errorf("parse range start: %w", err) //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}
	end, err := strconv.ParseInt(bounds[1], 10, 64)
	if err != nil {
		return contentRange{}, fmt.Errorf("parse range end: %w", err) //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}
	total, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return contentRange{}, fmt.Errorf("parse total size: %w", err) //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}
	if total <= 0 {
		return contentRange{}, fmt.Errorf("invalid total size: %d", total) //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}
	if start > end {
		return contentRange{}, fmt.Errorf("invalid content range: start %d is after end %d", start, end) //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}
	if end >= total {
		return contentRange{}, fmt.Errorf("invalid content range: end %d is outside total %d", end, total) //nolint:forbidigo // intermediate Content-Range parse; caller wraps it as a typed network error
	}
	return contentRange{start: start, end: end, total: total}, nil
}

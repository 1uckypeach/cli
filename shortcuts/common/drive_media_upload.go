// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package common

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/extension/fileio"
	"github.com/larksuite/cli/internal/client"
)

const MaxDriveMediaUploadSinglePartSize int64 = 20 * 1024 * 1024 // 20MB

const (
	driveMediaUploadAllAction    = "upload media failed"
	driveMediaUploadPartAction   = "upload media part failed"
	driveMediaUploadFinishAction = "upload media finish failed"
)

type DriveMediaMultipartUploadSession struct {
	UploadID  string
	BlockSize int64
	BlockNum  int
}

type DriveMediaUploadAllConfig struct {
	FilePath   string
	FileName   string
	FileSize   int64
	ParentType string
	ParentNode *string
	Extra      string
	// Reader, when non-nil, is used as the upload source instead of opening
	// FilePath. Callers own the reader lifetime.
	Reader io.Reader
}

type DriveMediaMultipartUploadConfig struct {
	FilePath   string
	FileName   string
	FileSize   int64
	ParentType string
	ParentNode string
	Extra      string
	Reader     io.Reader
}

type driveMediaUploadBoundary interface {
	FileIO() fileio.FileIO
	Stderr() io.Writer
	UploadForm(context.Context, string, *larkcore.Formdata) (map[string]interface{}, error)
	CallAPI(context.Context, string, map[string]interface{}) (map[string]interface{}, error)
}

type legacyDriveMediaUploadBoundary struct{ runtime *RuntimeContext }

func (boundary legacyDriveMediaUploadBoundary) FileIO() fileio.FileIO {
	return boundary.runtime.FileIO()
}
func (boundary legacyDriveMediaUploadBoundary) Stderr() io.Writer {
	return boundary.runtime.IO().ErrOut
}
func (boundary legacyDriveMediaUploadBoundary) UploadForm(ctx context.Context, path string, form *larkcore.Formdata) (map[string]interface{}, error) {
	response, err := boundary.runtime.DoAPIWithContext(ctx, &larkcore.ApiReq{HttpMethod: http.MethodPost, ApiPath: path, Body: form}, larkcore.WithFileUpload())
	if err != nil {
		return nil, client.WrapDoAPIError(err)
	}
	return boundary.runtime.ClassifyAPIResponse(response)
}
func (boundary legacyDriveMediaUploadBoundary) CallAPI(_ context.Context, path string, body map[string]interface{}) (map[string]interface{}, error) {
	return boundary.runtime.CallAPITyped(http.MethodPost, path, nil, body)
}

type commandDriveMediaUploadBoundary struct{ command CommandContext }

func (boundary commandDriveMediaUploadBoundary) FileIO() fileio.FileIO {
	return boundary.command.FileIO()
}
func (boundary commandDriveMediaUploadBoundary) Stderr() io.Writer { return boundary.command.Stderr() }
func (boundary commandDriveMediaUploadBoundary) UploadForm(ctx context.Context, path string, form *larkcore.Formdata) (map[string]interface{}, error) {
	return DoTypedAPIJSONWithOptions(ctx, boundary.command, http.MethodPost, path, nil, form, larkcore.WithFileUpload())
}
func (boundary commandDriveMediaUploadBoundary) CallAPI(ctx context.Context, path string, body map[string]interface{}) (map[string]interface{}, error) {
	return CallTypedAPI(ctx, boundary.command, http.MethodPost, path, nil, body)
}

// UploadDriveMediaAllTyped preserves the Legacy RuntimeContext entry point.
func UploadDriveMediaAllTyped(runtime *RuntimeContext, cfg DriveMediaUploadAllConfig) (string, error) {
	ctx := runtime.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return uploadDriveMediaAll(ctx, legacyDriveMediaUploadBoundary{runtime: runtime}, cfg)
}

// UploadDriveMediaAllCommand is the Typed CommandContext entry point.
func UploadDriveMediaAllCommand(ctx context.Context, command CommandContext, cfg DriveMediaUploadAllConfig) (string, error) {
	return uploadDriveMediaAll(ctx, commandDriveMediaUploadBoundary{command: command}, cfg)
}

func uploadDriveMediaAll(ctx context.Context, boundary driveMediaUploadBoundary, cfg DriveMediaUploadAllConfig) (string, error) {
	reader, closeReader, err := driveMediaUploadReader(boundary.FileIO(), cfg.FilePath, cfg.Reader)
	if err != nil {
		return "", err
	}
	if closeReader != nil {
		defer closeReader.Close()
	}

	form := larkcore.NewFormdata()
	form.AddField("file_name", cfg.FileName)
	form.AddField("parent_type", cfg.ParentType)
	form.AddField("size", fmt.Sprintf("%d", cfg.FileSize))
	if cfg.ParentNode != nil {
		form.AddField("parent_node", *cfg.ParentNode)
	}
	if cfg.Extra != "" {
		form.AddField("extra", cfg.Extra)
	}
	form.AddFile("file", reader)

	data, err := boundary.UploadForm(ctx, "/open-apis/drive/v1/medias/upload_all", form)
	if err != nil {
		return "", prefixDriveMediaUploadProblem(err, driveMediaUploadAllAction)
	}
	return extractDriveMediaUploadFileTokenTyped(data, driveMediaUploadAllAction)
}

// UploadDriveMediaMultipartTyped preserves the Legacy RuntimeContext entry point.
func UploadDriveMediaMultipartTyped(runtime *RuntimeContext, cfg DriveMediaMultipartUploadConfig) (string, error) {
	ctx := runtime.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return uploadDriveMediaMultipart(ctx, legacyDriveMediaUploadBoundary{runtime: runtime}, cfg)
}

// UploadDriveMediaMultipartCommand is the Typed CommandContext entry point.
func UploadDriveMediaMultipartCommand(ctx context.Context, command CommandContext, cfg DriveMediaMultipartUploadConfig) (string, error) {
	return uploadDriveMediaMultipart(ctx, commandDriveMediaUploadBoundary{command: command}, cfg)
}

func uploadDriveMediaMultipart(ctx context.Context, boundary driveMediaUploadBoundary, cfg DriveMediaMultipartUploadConfig) (string, error) {
	prepareBody := map[string]interface{}{
		"file_name": cfg.FileName, "parent_type": cfg.ParentType,
		"parent_node": cfg.ParentNode, "size": cfg.FileSize,
	}
	if cfg.Extra != "" {
		prepareBody["extra"] = cfg.Extra
	}
	data, err := boundary.CallAPI(ctx, "/open-apis/drive/v1/medias/upload_prepare", prepareBody)
	if err != nil {
		return "", err
	}
	session, err := parseDriveMediaMultipartUploadSessionTyped(data)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(boundary.Stderr(), "Multipart upload initialized: %d chunks x %s\n", session.BlockNum, FormatSize(session.BlockSize))
	if err := uploadDriveMediaMultipartParts(ctx, boundary, cfg, session); err != nil {
		return "", err
	}
	return finishDriveMediaMultipartUpload(ctx, boundary, session.UploadID, session.BlockNum)
}

func driveMediaUploadReader(fio fileio.FileIO, filePath string, supplied io.Reader) (io.Reader, io.ReadCloser, error) {
	if supplied != nil {
		return supplied, nil, nil
	}
	file, err := fio.Open(filePath)
	if err != nil {
		return nil, nil, WrapInputStatErrorTyped(err)
	}
	return file, file, nil
}

func prefixDriveMediaUploadProblem(err error, action string) error {
	if problem, ok := errs.ProblemOf(err); ok {
		problem.Message = action + ": " + problem.Message
	}
	return err
}

func parseDriveMediaMultipartUploadSessionTyped(data map[string]interface{}) (DriveMediaMultipartUploadSession, error) {
	session := DriveMediaMultipartUploadSession{
		UploadID: GetString(data, "upload_id"), BlockSize: int64(GetFloat(data, "block_size")), BlockNum: int(GetFloat(data, "block_num")),
	}
	if session.UploadID == "" {
		return DriveMediaMultipartUploadSession{}, errs.NewInternalError(errs.SubtypeInvalidResponse, "upload prepare failed: no upload_id returned")
	}
	if session.BlockSize <= 0 {
		return DriveMediaMultipartUploadSession{}, errs.NewInternalError(errs.SubtypeInvalidResponse, "upload prepare failed: invalid block_size returned")
	}
	if session.BlockNum <= 0 {
		return DriveMediaMultipartUploadSession{}, errs.NewInternalError(errs.SubtypeInvalidResponse, "upload prepare failed: invalid block_num returned")
	}
	return session, nil
}

func extractDriveMediaUploadFileTokenTyped(data map[string]interface{}, action string) (string, error) {
	fileToken := GetString(data, "file_token")
	if fileToken == "" {
		return "", errs.NewInternalError(errs.SubtypeInvalidResponse, "%s: no file_token returned", action)
	}
	return fileToken, nil
}

func uploadDriveMediaMultipartParts(ctx context.Context, boundary driveMediaUploadBoundary, cfg DriveMediaMultipartUploadConfig, session DriveMediaMultipartUploadSession) error {
	reader, closeReader, err := driveMediaUploadReader(boundary.FileIO(), cfg.FilePath, cfg.Reader)
	if err != nil {
		return err
	}
	if closeReader != nil {
		defer closeReader.Close()
	}
	maxInt := int64(^uint(0) >> 1)
	if session.BlockSize <= 0 || session.BlockSize > maxInt {
		return errs.NewInternalError(errs.SubtypeInvalidResponse, "upload prepare failed: invalid block_size returned")
	}
	buffer := make([]byte, int(session.BlockSize))
	remaining := cfg.FileSize
	for sequence := 0; sequence < session.BlockNum; sequence++ {
		chunkSize := session.BlockSize
		if remaining > 0 && chunkSize > remaining {
			chunkSize = remaining
		}
		read, readErr := io.ReadFull(reader, buffer[:int(chunkSize)])
		if readErr != nil {
			return WrapInputStatErrorTyped(readErr)
		}
		if err := uploadDriveMediaMultipartPart(ctx, boundary, session.UploadID, sequence, buffer[:read]); err != nil {
			return err
		}
		fmt.Fprintf(boundary.Stderr(), "  Block %d/%d uploaded (%s)\n", sequence+1, session.BlockNum, FormatSize(int64(read)))
		remaining -= int64(read)
	}
	return nil
}

func uploadDriveMediaMultipartPart(ctx context.Context, boundary driveMediaUploadBoundary, uploadID string, sequence int, chunk []byte) error {
	form := larkcore.NewFormdata()
	form.AddField("upload_id", uploadID)
	form.AddField("seq", fmt.Sprintf("%d", sequence))
	form.AddField("size", fmt.Sprintf("%d", len(chunk)))
	form.AddFile("file", bytes.NewReader(chunk))
	if _, err := boundary.UploadForm(ctx, "/open-apis/drive/v1/medias/upload_part", form); err != nil {
		return prefixDriveMediaUploadProblem(err, driveMediaUploadPartAction)
	}
	return nil
}

func finishDriveMediaMultipartUpload(ctx context.Context, boundary driveMediaUploadBoundary, uploadID string, blockNum int) (string, error) {
	data, err := boundary.CallAPI(ctx, "/open-apis/drive/v1/medias/upload_finish", map[string]interface{}{"upload_id": uploadID, "block_num": blockNum})
	if err != nil {
		return "", err
	}
	return extractDriveMediaUploadFileTokenTyped(data, driveMediaUploadFinishAction)
}

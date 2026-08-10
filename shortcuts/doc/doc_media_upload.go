// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/extension/fileio"
	"github.com/larksuite/cli/shortcuts/common"
)

type docMediaUploadArgs struct {
	File       string `flag:"file" schema:"required" doc:"local file path (files > 20MB use multipart upload automatically)"`
	ParentType string `flag:"parent-type" schema:"required" doc:"parent type: docx_image | docx_file | whiteboard | mindnote_image"`
	ParentNode string `flag:"parent-node" schema:"required" doc:"parent node ID (block_id for docx, board_token for whiteboard, mindnote token for mindnote)"`
	DocID      string `flag:"doc-id" schema:"optional" doc:"document ID (for drive_route_token)"`
}

type docMediaUploadData struct {
	FileName  string `json:"file_name" schema:"required;minLength=1" doc:"uploaded file name"`
	FileToken string `json:"file_token" schema:"required;minLength=1" doc:"uploaded media file token"`
	Size      int64  `json:"size" schema:"required;minimum=0" doc:"uploaded file size in bytes"`
}

var DocMediaUpload = common.Define(common.Definition[docMediaUploadArgs, docMediaUploadData]{
	Metadata: common.CommandMetadata{
		Service: "docs", Command: "+media-upload", Description: "Upload media file (image/attachment) to a document block", Risk: common.RiskWrite,
		Authorization: common.AuthorizationDefinition{Identities: map[common.Identity]common.IdentityAuthorization{
			common.IdentityUser: {RequiredScopes: []string{"docs:document.media:upload"}},
			common.IdentityBot:  {RequiredScopes: []string{"docs:document.media:upload"}},
		}},
	},
	Output: common.OutputDefinition{Mode: common.OutputFixedJSON},
	Hooks: common.Hooks[docMediaUploadArgs, docMediaUploadData]{
		DryRun: func(_ context.Context, command common.CommandContext, args *docMediaUploadArgs) *common.DryRunAPI {
			body := map[string]interface{}{"file_name": filepath.Base(args.File), "parent_type": args.ParentType, "parent_node": args.ParentNode}
			if args.DocID != "" {
				body["extra"] = fmt.Sprintf(`{"drive_route_token":"%s"}`, args.DocID)
			}
			dry := common.NewDryRunAPI()
			if docMediaShouldUseMultipart(command.FileIO(), args.File) {
				prepareBody := map[string]interface{}{"file_name": filepath.Base(args.File), "parent_type": args.ParentType, "parent_node": args.ParentNode, "size": "<file_size>"}
				if extra, ok := body["extra"]; ok {
					prepareBody["extra"] = extra
				}
				return dry.Desc("chunked media upload (files > 20MB)").POST("/open-apis/drive/v1/medias/upload_prepare").Body(prepareBody).
					POST("/open-apis/drive/v1/medias/upload_part").Body(map[string]interface{}{"upload_id": "<upload_id>", "seq": "<chunk_index>", "size": "<chunk_size>", "file": "<chunk_binary>"}).
					POST("/open-apis/drive/v1/medias/upload_finish").Body(map[string]interface{}{"upload_id": "<upload_id>", "block_num": "<block_num>"})
			}
			body["file"], body["size"] = "@"+args.File, "<file_size>"
			return dry.Desc("multipart/form-data upload").POST("/open-apis/drive/v1/medias/upload_all").Body(body)
		},
		Execute: func(ctx context.Context, command common.CommandContext, args *docMediaUploadArgs) (common.Result[docMediaUploadData], error) {
			stat, err := command.FileIO().Stat(args.File)
			if err != nil {
				return common.Result[docMediaUploadData]{}, wrapDocInputFileErr(err, "file not found")
			}
			if !stat.Mode().IsRegular() {
				return common.Result[docMediaUploadData]{}, errs.NewValidationError(errs.SubtypeInvalidArgument, "file must be a regular file: %s", args.File).WithParam("--file")
			}
			fileName := filepath.Base(args.File)
			fmt.Fprintf(command.Stderr(), "Uploading: %s (%d bytes)\n", fileName, stat.Size())
			if stat.Size() > common.MaxDriveMediaUploadSinglePartSize {
				fmt.Fprintln(command.Stderr(), "File exceeds 20MB, using multipart upload")
			}
			fileToken, err := uploadDocMediaFileCommand(ctx, command, UploadDocMediaFileConfig{
				FilePath: args.File, FileName: fileName, FileSize: stat.Size(), ParentType: args.ParentType, ParentNode: args.ParentNode, DocID: args.DocID,
			})
			if err != nil {
				return common.Result[docMediaUploadData]{}, err
			}
			return common.Success(docMediaUploadData{FileName: fileName, FileToken: fileToken, Size: stat.Size()}), nil
		},
	},
})

// UploadDocMediaFileConfig groups the inputs to uploadDocMediaFile so the
// call site names each value at call time, avoiding the "8 positional
// params of mostly string/int64" ambiguity and mirroring the config-struct
// style already used by DriveMediaUploadAllConfig /
// DriveMediaMultipartUploadConfig downstream.
//
// Exactly one of FilePath (on-disk source) or Reader (in-memory source for
// the clipboard flow) should be set. Leave Reader at its zero value (nil
// interface) when the caller only has FilePath — passing a typed-nil
// pointer like (*bytes.Reader)(nil) here would make Reader compare
// non-nil downstream and skip the FilePath open, so the field type is
// deliberately an interface and the clipboard caller builds it only when
// it actually has bytes.
type UploadDocMediaFileConfig struct {
	FilePath   string
	Reader     io.Reader
	FileName   string
	FileSize   int64
	ParentType string
	ParentNode string
	DocID      string
}

func uploadDocMediaFile(runtime *common.RuntimeContext, cfg UploadDocMediaFileConfig) (string, error) {
	extra, err := docMediaRouteExtra(cfg.DocID)
	if err != nil {
		return "", err
	}
	if cfg.FileSize <= common.MaxDriveMediaUploadSinglePartSize {
		return common.UploadDriveMediaAllTyped(runtime, docMediaUploadAllConfig(cfg, extra))
	}
	return common.UploadDriveMediaMultipartTyped(runtime, docMediaMultipartConfig(cfg, extra))
}

func uploadDocMediaFileCommand(ctx context.Context, command common.CommandContext, cfg UploadDocMediaFileConfig) (string, error) {
	extra, err := docMediaRouteExtra(cfg.DocID)
	if err != nil {
		return "", err
	}
	if cfg.FileSize <= common.MaxDriveMediaUploadSinglePartSize {
		return common.UploadDriveMediaAllCommand(ctx, command, docMediaUploadAllConfig(cfg, extra))
	}
	return common.UploadDriveMediaMultipartCommand(ctx, command, docMediaMultipartConfig(cfg, extra))
}

func docMediaRouteExtra(docID string) (string, error) {
	if docID == "" {
		return "", nil
	}
	return buildDriveRouteExtra(docID)
}

func docMediaUploadAllConfig(cfg UploadDocMediaFileConfig, extra string) common.DriveMediaUploadAllConfig {
	return common.DriveMediaUploadAllConfig{
		FilePath: cfg.FilePath, Reader: cfg.Reader, FileName: cfg.FileName, FileSize: cfg.FileSize,
		ParentType: cfg.ParentType, ParentNode: &cfg.ParentNode, Extra: extra,
	}
}

func docMediaMultipartConfig(cfg UploadDocMediaFileConfig, extra string) common.DriveMediaMultipartUploadConfig {
	return common.DriveMediaMultipartUploadConfig{
		FilePath: cfg.FilePath, Reader: cfg.Reader, FileName: cfg.FileName, FileSize: cfg.FileSize,
		ParentType: cfg.ParentType, ParentNode: cfg.ParentNode, Extra: extra,
	}
}

func docMediaShouldUseMultipart(fio fileio.FileIO, filePath string) bool {
	// Dry-run uses local stat as a best-effort planning hint. Execute re-validates
	// the file before choosing the actual upload path.
	info, err := fio.Stat(filePath)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Size() > common.MaxDriveMediaUploadSinglePartSize
}

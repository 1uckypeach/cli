// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package drive

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/extension/fileio"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

const driveMetadataReadScope = "drive:drive.metadata:readonly"

type driveDownloadOutputPathValidator func(string) error

func driveDownloadNormalizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	if name == "" || name == "." || name == ".." || strings.Trim(name, "/") == "" {
		return ""
	}
	return name
}

func driveDownloadFallbackFileName(title, fileToken string) string {
	if name := driveDownloadNormalizeFileName(title); name != "" {
		return name
	}
	return fileToken
}

func driveDownloadCandidateOutputPath(header http.Header, candidate string) (string, bool) {
	fileName := driveDownloadNormalizeFileName(candidate)
	if fileName == "" {
		return "", false
	}

	fileName = sanitizeExportFileName(fileName, "")
	if fileName == "" {
		return "", false
	}

	fileName, _ = common.AutoAppendDownloadExtension(fileName, header, "")
	if strings.TrimSpace(fileName) == "" || fileName == "." || fileName == ".." || strings.Trim(fileName, "/") == "" {
		return "", false
	}
	return fileName, true
}

func driveDownloadDefaultOutputPath(header http.Header, title, fileToken string, validatePath driveDownloadOutputPathValidator) (string, error) {
	candidates := []string{
		larkcore.FileNameByHeader(header),
		title,
		fileToken,
	}

	var lastErr error
	for _, candidate := range candidates {
		fileName, ok := driveDownloadCandidateOutputPath(header, candidate)
		if !ok {
			continue
		}
		if validatePath != nil {
			if err := validatePath(fileName); err != nil {
				lastErr = err
				continue
			}
		}
		return fileName, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return fileToken, nil
}

func driveDownloadShouldFailOnMetadataTitleError(ctx context.Context, err error) bool {
	if ctx != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return true
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if problem, ok := errs.ProblemOf(err); ok {
		if problem.Category == errs.CategoryAuthorization {
			return true
		}
	}
	return false
}

type driveDownloadArgs struct {
	FileToken string `flag:"file-token" schema:"required" doc:"file token"`
	Output    string `flag:"output" schema:"optional" doc:"local save path"`
	Overwrite bool   `flag:"overwrite" schema:"optional" doc:"overwrite existing output file"`
}

type driveDownloadData struct {
	SavedPath string `json:"saved_path" schema:"required;minLength=1" doc:"resolved local path of the saved file"`
	SizeBytes int64  `json:"size_bytes" schema:"required;minimum=0" doc:"number of bytes written"`
}

var DriveDownload = common.Define(common.Definition[driveDownloadArgs, driveDownloadData]{
	Metadata: common.CommandMetadata{
		Service: "drive", Command: "+download", Description: "Download a file from Drive to local", Risk: common.RiskRead,
		Authorization: common.AuthorizationDefinition{Identities: map[common.Identity]common.IdentityAuthorization{
			common.IdentityUser: {RequiredScopes: []string{"drive:file:download"}, ConditionalScopes: []common.ConditionalScope{{Scopes: []string{driveMetadataReadScope}, When: "--output is omitted and the default filename needs metadata", Params: []string{"output"}}}},
			common.IdentityBot:  {RequiredScopes: []string{"drive:file:download"}, ConditionalScopes: []common.ConditionalScope{{Scopes: []string{driveMetadataReadScope}, When: "--output is omitted and the default filename needs metadata", Params: []string{"output"}}}},
		}},
	},
	Output: common.OutputDefinition{
		Mode:      common.OutputFixedJSON,
		Artifacts: []common.ArtifactDefinition{{Name: "download", ItemsPath: "", PathField: "/saved_path", SizeField: "/size_bytes"}},
	},
	Hooks: common.Hooks[driveDownloadArgs, driveDownloadData]{
		Validate: func(_ context.Context, command common.CommandContext, args *driveDownloadArgs) error {
			if err := validate.ResourceName(args.FileToken, "--file-token"); err != nil {
				return errs.NewValidationError(errs.SubtypeInvalidArgument, "%s", err).WithParam("--file-token")
			}
			if args.Output == "" {
				return command.RequireConditionalScopes(driveMetadataReadScope)
			}
			if _, err := command.ResolveSavePath(args.Output); err != nil {
				return errs.NewValidationError(errs.SubtypeInvalidArgument, "unsafe output path: %s", err).WithParam("--output")
			}
			return nil
		},
		DryRun: func(_ context.Context, _ common.CommandContext, args *driveDownloadArgs) *common.DryRunAPI {
			outputPath := args.Output
			plan := common.NewDryRunAPI()
			downloadDesc := "[1] Download file bytes to the explicit output path"
			if outputPath == "" {
				outputPath = "<Content-Disposition filename | metadata title | token>"
				downloadDesc = "[2] Download file bytes; Content-Disposition filename wins over metadata title when present"
				plan.POST("/open-apis/drive/v1/metas/batch_query").
					Desc("[1] Resolve metadata title before downloading; fails before the download request if metadata scope is missing").
					Body(map[string]interface{}{"request_docs": []map[string]interface{}{{"doc_token": args.FileToken, "doc_type": "file"}}})
			}
			return plan.GET("/open-apis/drive/v1/files/:file_token/download").Desc(downloadDesc).
				Set("file_token", args.FileToken).Set("output", outputPath)
		},
		Execute: func(ctx context.Context, command common.CommandContext, args *driveDownloadArgs) (common.Result[driveDownloadData], error) {
			outputPath := args.Output
			if outputPath != "" {
				if _, err := command.ResolveSavePath(outputPath); err != nil {
					return common.Result[driveDownloadData]{}, errs.NewValidationError(errs.SubtypeInvalidArgument, "unsafe output path: %s", err).WithParam("--output")
				}
				if _, err := command.FileIO().Stat(outputPath); err == nil && !args.Overwrite {
					return common.Result[driveDownloadData]{}, errs.NewValidationError(errs.SubtypeInvalidArgument, "output file already exists: %s (use --overwrite to replace)", outputPath).WithParam("--output")
				}
			}

			var metadataTitle string
			if outputPath == "" {
				title, err := common.FetchDriveMetaTitleCommand(ctx, command, args.FileToken, "file")
				if err != nil {
					if driveDownloadShouldFailOnMetadataTitleError(ctx, err) {
						if contextErr := ctx.Err(); contextErr != nil {
							return common.Result[driveDownloadData]{}, contextErr
						}
						return common.Result[driveDownloadData]{}, err
					}
					fmt.Fprintf(command.Stderr(), "warning: metadata title lookup failed; continuing with Content-Disposition or token filename: %v\n", err)
				} else {
					metadataTitle = title
				}
			}

			fmt.Fprintf(command.Stderr(), "Downloading: %s\n", common.MaskToken(args.FileToken))
			response, err := common.DoTypedAPIStream(ctx, command, &larkcore.ApiReq{
				HttpMethod: http.MethodGet,
				ApiPath:    fmt.Sprintf("/open-apis/drive/v1/files/%s/download", validate.EncodePathSegment(args.FileToken)),
			})
			if err != nil {
				wrapped := withDriveDownloadForbiddenPreviewHint(wrapDriveNetworkErr(err, "download failed: %s", err), args.FileToken)
				return common.Result[driveDownloadData]{}, wrapped
			}
			defer response.Body.Close()

			if outputPath == "" {
				var resolveErr error
				outputPath, resolveErr = driveDownloadDefaultOutputPath(response.Header, metadataTitle, args.FileToken, func(candidate string) error {
					_, err := command.ResolveSavePath(candidate)
					return err
				})
				if resolveErr != nil {
					return common.Result[driveDownloadData]{}, errs.NewInternalError(errs.SubtypeFileIO, "cannot derive a safe default output path: %s", resolveErr).WithCause(resolveErr)
				}
			}
			if _, err := command.FileIO().Stat(outputPath); err == nil && !args.Overwrite {
				return common.Result[driveDownloadData]{}, errs.NewValidationError(errs.SubtypeInvalidArgument, "output file already exists: %s (use --overwrite to replace)", outputPath).WithParam("--output")
			}
			saved, err := command.FileIO().Save(outputPath, fileio.SaveOptions{ContentType: response.Header.Get("Content-Type"), ContentLength: response.ContentLength}, response.Body)
			if err != nil {
				return common.Result[driveDownloadData]{}, driveSaveError(err)
			}
			savedPath, _ := command.ResolveSavePath(outputPath)
			if savedPath == "" {
				savedPath = outputPath
			}
			return common.Success(driveDownloadData{SavedPath: savedPath, SizeBytes: saved.Size()}), nil
		},
	},
})

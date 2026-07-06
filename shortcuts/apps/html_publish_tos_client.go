// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

type deployResponse struct {
	Status    string
	URL       string
	ReleaseID string
}

type appsHTMLPublishTOSClient interface {
	GetUploadURL(ctx context.Context, appID string) (uploadURL string, tosKey string, err error)
	UploadToTOS(ctx context.Context, uploadURL string, archive *htmlPublishArchive) error
	Deploy(ctx context.Context, appID string, tosKey string) (*deployResponse, error)
}

type appsHTMLPublishTOSAPI struct {
	runtime *common.RuntimeContext
}

func (api appsHTMLPublishTOSAPI) GetUploadURL(ctx context.Context, appID string) (string, string, error) {
	path := fmt.Sprintf("%s/apps/%s/pre_release_html_code", apiBasePath, validate.EncodePathSegment(appID))
	data, err := api.runtime.CallAPITyped("POST", path, nil, nil)
	if err != nil {
		return "", "", err
	}
	uploadURL, _ := data["upload_url"].(string)
	tosKey, _ := data["tos_key"].(string)
	if uploadURL == "" || tosKey == "" {
		return "", "", appsSubprocessEnvelopeError("pre_release_html_code returned empty upload_url or tos_key")
	}
	if !strings.HasPrefix(uploadURL, "https://") {
		return "", "", appsSubprocessEnvelopeError("upload_url must be https, got %q", uploadURL)
	}
	return uploadURL, tosKey, nil
}

func (api appsHTMLPublishTOSAPI) UploadToTOS(ctx context.Context, uploadURL string, archive *htmlPublishArchive) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(archive.Body))
	if err != nil {
		return appsFileIOError(err, "create TOS upload request: %v", err)
	}
	req.Header.Set("Content-Type", "application/zip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return appsExternalToolError(err, "TOS upload failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return appsExternalToolError(nil, "TOS upload returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (api appsHTMLPublishTOSAPI) Deploy(ctx context.Context, appID string, tosKey string) (*deployResponse, error) {
	path := fmt.Sprintf("%s/apps/%s/release_html_code", apiBasePath, validate.EncodePathSegment(appID))
	body := map[string]interface{}{"tos_key": tosKey}
	data, err := api.runtime.CallAPITyped("POST", path, nil, body)
	if err != nil {
		return nil, err
	}
	status, _ := data["status"].(string)
	url, _ := data["online_url"].(string)
	releaseID, _ := data["release_id"].(string)
	return &deployResponse{Status: status, URL: url, ReleaseID: releaseID}, nil
}

var (
	pollInterval = 10 * time.Second
	pollTimeout  = 60 * time.Second
)

func pollDeployStatus(ctx context.Context, rctx *common.RuntimeContext, appID, releaseID string) (*deployResponse, error) {
	path := fmt.Sprintf("%s/apps/%s/releases/%s",
		apiBasePath,
		validate.EncodePathSegment(appID),
		validate.EncodePathSegment(releaseID))
	deadline := time.Now().Add(pollTimeout)
	for time.Now().Before(deadline) {
		data, err := rctx.CallAPITyped("GET", path, nil, nil)
		if err != nil {
			return nil, err
		}
		status, _ := data["status"].(string)
		switch status {
		case "finished":
			url, _ := data["online_url"].(string)
			return &deployResponse{Status: "finished", URL: url, ReleaseID: releaseID}, nil
		case "failed":
			return &deployResponse{Status: "failed", ReleaseID: releaseID}, nil
		}
		time.Sleep(pollInterval)
	}
	return &deployResponse{Status: "building", ReleaseID: releaseID}, nil
}

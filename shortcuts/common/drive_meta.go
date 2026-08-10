// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package common

import "context"

// DriveMeta is the subset of drive metas/batch_query fields used by shortcuts.
type DriveMeta struct {
	Title string
	URL   string
}

// FetchDriveMeta looks up document metadata via the drive metas batch_query API.
func FetchDriveMeta(runtime *RuntimeContext, token, docType string, withURL bool) (DriveMeta, error) {
	data, err := runtime.CallAPITyped("POST", "/open-apis/drive/v1/metas/batch_query", nil, driveMetaRequestBody(token, docType, withURL))
	if err != nil {
		return DriveMeta{}, err
	}
	return driveMetaFromData(data), nil
}

// FetchDriveMetaCommand is the Typed counterpart of FetchDriveMeta.
func FetchDriveMetaCommand(ctx context.Context, command CommandContext, token, docType string, withURL bool) (DriveMeta, error) {
	data, err := CallTypedAPI(ctx, command, "POST", "/open-apis/drive/v1/metas/batch_query", nil, driveMetaRequestBody(token, docType, withURL))
	if err != nil {
		return DriveMeta{}, err
	}
	return driveMetaFromData(data), nil
}

func driveMetaRequestBody(token, docType string, withURL bool) map[string]interface{} {
	body := map[string]interface{}{"request_docs": []map[string]interface{}{{"doc_token": token, "doc_type": docType}}}
	if withURL {
		body["with_url"] = true
	}
	return body
}

func driveMetaFromData(data map[string]interface{}) DriveMeta {
	metas := GetSlice(data, "metas")
	if len(metas) == 0 {
		return DriveMeta{}
	}
	meta, _ := metas[0].(map[string]interface{})
	return DriveMeta{Title: GetString(meta, "title"), URL: GetString(meta, "url")}
}

// FetchDriveMetaTitleCommand looks up a title through CommandContext.
func FetchDriveMetaTitleCommand(ctx context.Context, command CommandContext, token, docType string) (string, error) {
	meta, err := FetchDriveMetaCommand(ctx, command, token, docType, false)
	if err != nil {
		return "", err
	}
	return meta.Title, nil
}

// FetchDriveMetaTitle looks up the document title via the drive metas batch_query API.
func FetchDriveMetaTitle(runtime *RuntimeContext, token, docType string) (string, error) {
	meta, err := FetchDriveMeta(runtime, token, docType, false)
	if err != nil {
		return "", err
	}
	return meta.Title, nil
}

// FetchDriveMetaURL looks up the document access URL via the drive metas batch_query API.
func FetchDriveMetaURL(runtime *RuntimeContext, token, docType string) (string, error) {
	meta, err := FetchDriveMeta(runtime, token, docType, true)
	if err != nil {
		return "", err
	}
	return meta.URL, nil
}

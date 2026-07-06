// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeTOSClient struct {
	uploadURL  string
	tosKey     string
	deployResp *deployResponse
	uploadErr  error
	deployErr  error
	getURLErr  error

	// recorded calls for assertions
	uploadedURL string
	deployedKey string
}

func (f *fakeTOSClient) GetUploadURL(ctx context.Context, appID string) (string, string, error) {
	if f.getURLErr != nil {
		return "", "", f.getURLErr
	}
	return f.uploadURL, f.tosKey, nil
}

func (f *fakeTOSClient) UploadToTOS(ctx context.Context, uploadURL string, archive *htmlPublishArchive) error {
	f.uploadedURL = uploadURL
	return f.uploadErr
}

func (f *fakeTOSClient) Deploy(ctx context.Context, appID string, tosKey string) (*deployResponse, error) {
	f.deployedKey = tosKey
	if f.deployErr != nil {
		return nil, f.deployErr
	}
	return f.deployResp, nil
}

func setupHTMLPublishTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir
}

func TestRunHTMLPublishTOS_SyncSuccess(t *testing.T) {
	dir := setupHTMLPublishTestDir(t)
	fio := newTestFIO()
	client := &fakeTOSClient{
		uploadURL:  "https://tos.example.com/upload",
		tosKey:     "tos/key/123",
		deployResp: &deployResponse{Status: "finished", URL: "https://app.example.com/app_x"},
	}
	spec := appsHTMLPublishSpec{AppID: "app_x", Path: dir}
	out, err := runHTMLPublishTOS(context.Background(), fio, client, nil, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["status"] != "finished" {
		t.Errorf("status = %v, want finished", out["status"])
	}
	if out["online_url"] != "https://app.example.com/app_x" {
		t.Errorf("online_url = %v", out["online_url"])
	}
	if out["app_id"] != "app_x" {
		t.Errorf("app_id = %v, want app_x", out["app_id"])
	}
}

func TestRunHTMLPublishTOS_GetUploadURLError(t *testing.T) {
	dir := setupHTMLPublishTestDir(t)
	fio := newTestFIO()
	wantErr := errors.New("get upload url failed")
	client := &fakeTOSClient{getURLErr: wantErr}
	spec := appsHTMLPublishSpec{AppID: "app_x", Path: dir}
	_, err := runHTMLPublishTOS(context.Background(), fio, client, nil, spec)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestRunHTMLPublishTOS_UploadError(t *testing.T) {
	dir := setupHTMLPublishTestDir(t)
	fio := newTestFIO()
	wantErr := errors.New("upload failed")
	client := &fakeTOSClient{
		uploadURL: "https://tos.example.com/upload",
		tosKey:    "tos/key/123",
		uploadErr: wantErr,
	}
	spec := appsHTMLPublishSpec{AppID: "app_x", Path: dir}
	_, err := runHTMLPublishTOS(context.Background(), fio, client, nil, spec)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestRunHTMLPublishTOS_DeployError(t *testing.T) {
	dir := setupHTMLPublishTestDir(t)
	fio := newTestFIO()
	wantErr := errors.New("deploy failed")
	client := &fakeTOSClient{
		uploadURL: "https://tos.example.com/upload",
		tosKey:    "tos/key/123",
		deployErr: wantErr,
	}
	spec := appsHTMLPublishSpec{AppID: "app_x", Path: dir}
	_, err := runHTMLPublishTOS(context.Background(), fio, client, nil, spec)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestRunHTMLPublishTOS_MissingIndexHTML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "foo.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	fio := newTestFIO()
	client := &fakeTOSClient{
		uploadURL:  "https://tos.example.com/upload",
		tosKey:     "tos/key/123",
		deployResp: &deployResponse{Status: "finished", URL: "https://app.example.com/app_x"},
	}
	spec := appsHTMLPublishSpec{AppID: "app_x", Path: dir}
	_, err := runHTMLPublishTOS(context.Background(), fio, client, nil, spec)
	if err == nil {
		t.Fatalf("expected error for missing index.html")
	}
	problem := requireAppsValidationProblem(t, err)
	if !strings.Contains(problem.Message, "index.html") {
		t.Fatalf("message missing 'index.html': %v", problem.Message)
	}
}

func TestRunHTMLPublishTOS_RejectsOversizeZip(t *testing.T) {
	orig := maxHTMLPublishTarballBytes
	maxHTMLPublishTarballBytes = 100
	defer func() { maxHTMLPublishTarballBytes = orig }()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "big.html"),
		[]byte(strings.Repeat("x", 4096)), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fio := newTestFIO()
	client := &fakeTOSClient{
		uploadURL:  "https://tos.example.com/upload",
		tosKey:     "tos/key/123",
		deployResp: &deployResponse{Status: "finished"},
	}
	spec := appsHTMLPublishSpec{AppID: "app_x", Path: dir}
	_, err := runHTMLPublishTOS(context.Background(), fio, client, nil, spec)
	if err == nil {
		t.Fatalf("expected oversize error")
	}
	problem := requireAppsValidationProblem(t, err)
	if !strings.Contains(problem.Message, "exceeds") {
		t.Fatalf("message missing 'exceeds': %v", problem.Message)
	}
}

func TestRunHTMLPublishTOS_PassesURLAndKeyToClient(t *testing.T) {
	dir := setupHTMLPublishTestDir(t)
	fio := newTestFIO()
	client := &fakeTOSClient{
		uploadURL:  "https://tos.example.com/upload/abc",
		tosKey:     "tos/key/456",
		deployResp: &deployResponse{Status: "finished", URL: "https://app.example.com/app_x"},
	}
	spec := appsHTMLPublishSpec{AppID: "app_x", Path: dir}
	_, err := runHTMLPublishTOS(context.Background(), fio, client, nil, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.uploadedURL != "https://tos.example.com/upload/abc" {
		t.Errorf("uploadedURL = %q, want https://tos.example.com/upload/abc", client.uploadedURL)
	}
	if client.deployedKey != "tos/key/456" {
		t.Errorf("deployedKey = %q, want tos/key/456", client.deployedKey)
	}
}

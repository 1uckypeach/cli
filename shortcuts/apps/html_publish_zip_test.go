// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildHTMLPublishZip_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "style.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fio := newTestFIO()
	candidates, err := walkHTMLPublishCandidates(fio, dir)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	archive, err := buildHTMLPublishZip(fio, candidates)
	if err != nil {
		t.Fatalf("buildHTMLPublishZip: %v", err)
	}

	if len(archive.SHA256) != 64 {
		t.Fatalf("SHA256 wrong len: %d", len(archive.SHA256))
	}
	if archive.Size <= 0 || int64(len(archive.Body)) != archive.Size {
		t.Fatalf("size=%d body=%d", archive.Size, len(archive.Body))
	}

	r, err := zip.NewReader(bytes.NewReader(archive.Body), archive.Size)
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	names := make(map[string]bool)
	for _, f := range r.File {
		names[f.Name] = true
	}
	if !names["index.html"] || !names["style.css"] {
		t.Errorf("zip entries = %v, want index.html and style.css", names)
	}

	// Verify content round-trip for index.html.
	for _, f := range r.File {
		if f.Name == "index.html" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open entry: %v", err)
			}
			body, err := io.ReadAll(rc)
			rc.Close()
			if err != nil || string(body) != "<html></html>" {
				t.Fatalf("body=%q err=%v", body, err)
			}
		}
	}
}

func TestBuildHTMLPublishZip_EmptyCandidates(t *testing.T) {
	if _, err := buildHTMLPublishZip(newTestFIO(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildHTMLPublishZip_SHA256Stable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>hi</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidates := []htmlPublishCandidate{
		{RelPath: "index.html", AbsPath: filepath.Join(dir, "index.html"), Size: 11},
	}
	fio := newTestFIO()
	a1, err := buildHTMLPublishZip(fio, candidates)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := buildHTMLPublishZip(fio, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if a1.SHA256 != a2.SHA256 {
		t.Errorf("SHA256 not stable: %s vs %s", a1.SHA256, a2.SHA256)
	}
}

func TestWriteHTMLPublishZipEntry_OpenFailure(t *testing.T) {
	zw := zip.NewWriter(io.Discard)
	defer zw.Close()
	err := writeHTMLPublishZipEntry(newTestFIO(), zw, htmlPublishCandidate{
		RelPath: "x.html",
		AbsPath: "/nonexistent-path-for-test/x.html",
		Size:    0,
	})
	if err == nil {
		t.Fatalf("expected error for nonexistent abs path")
	}
	if !strings.Contains(err.Error(), "open") {
		t.Fatalf("expected open error, got %v", err)
	}
}

func TestWriteHTMLPublishZipEntry_CopyFailure(t *testing.T) {
	fio := readFailingFIO{readErr: errors.New("synthetic read failure")}
	zw := zip.NewWriter(io.Discard)
	defer zw.Close()

	err := writeHTMLPublishZipEntry(fio, zw, htmlPublishCandidate{
		RelPath: "x.html",
		AbsPath: "fixtures/x.html",
		Size:    7,
	})
	if err == nil {
		t.Fatalf("expected error when underlying Read fails")
	}
	if !strings.Contains(err.Error(), "copy") {
		t.Fatalf("expected copy-stage error, got %v", err)
	}
}

func TestBuildHTMLPublishZip_EntryWriteFailureReturnsError(t *testing.T) {
	candidates := []htmlPublishCandidate{
		{RelPath: "x.html", AbsPath: "/nonexistent-path-for-test/x.html", Size: 0},
	}

	archive, err := buildHTMLPublishZip(newTestFIO(), candidates)
	if err == nil {
		t.Fatalf("expected error, got archive=%+v", archive)
	}
	if archive != nil {
		t.Fatalf("expected nil archive on error, got %+v", archive)
	}
}

func TestWriteHTMLPublishZipEntry_RejectsPathTraversal(t *testing.T) {
	zw := zip.NewWriter(io.Discard)
	defer zw.Close()

	cases := []struct {
		name string
		rel  string
	}{
		{"parent traversal", "../etc/passwd"},
		{"absolute path", "/etc/passwd"},
		{"embedded traversal", "a/../../etc/passwd"},
		{"null byte", "evil\x00.html"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := writeHTMLPublishZipEntry(newTestFIO(), zw, htmlPublishCandidate{
				RelPath: c.rel,
				AbsPath: "fixtures/whatever",
				Size:    0,
			})
			if err == nil {
				t.Fatalf("expected error for RelPath=%q", c.rel)
			}
			if !strings.Contains(err.Error(), "invalid zip entry name") {
				t.Fatalf("expected 'invalid zip entry name' error, got %v", err)
			}
		})
	}
}

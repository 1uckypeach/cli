// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/extension/fileio"
)

// htmlPublishArchive is the in-memory zip ready for TOS upload.
type htmlPublishArchive struct {
	Body   []byte
	Size   int64
	SHA256 string
}

func buildHTMLPublishZip(fio fileio.FileIO, candidates []htmlPublishCandidate) (*htmlPublishArchive, error) {
	if len(candidates) == 0 {
		return nil, appsValidationParamError("--path", "no files to pack")
	}

	var buf bytes.Buffer
	hasher := sha256.New()
	multi := io.MultiWriter(&buf, hasher)
	zw := zip.NewWriter(multi)

	for _, c := range candidates {
		if err := writeHTMLPublishZipEntry(fio, zw, c); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, appsFileIOError(err, "zip close: %v", err)
	}

	return &htmlPublishArchive{
		Body:   buf.Bytes(),
		Size:   int64(buf.Len()),
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func writeHTMLPublishZipEntry(fio fileio.FileIO, zw *zip.Writer, c htmlPublishCandidate) error {
	if isUnsafeRelPath(c.RelPath) {
		return errs.NewInternalError(errs.SubtypeUnknown, "invalid zip entry name %q", c.RelPath)
	}

	src, err := fio.Open(c.AbsPath)
	if err != nil {
		return appsInputPathEntryError(c.AbsPath, err)
	}
	defer src.Close()

	w, err := zw.Create(c.RelPath)
	if err != nil {
		return appsFileIOError(err, "create zip entry %s: %v", c.RelPath, err)
	}
	if _, err := io.Copy(w, src); err != nil {
		return appsFileIOError(err, "copy %s: %v", c.RelPath, err)
	}
	return nil
}

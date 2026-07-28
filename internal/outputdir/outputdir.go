// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package outputdir validates and creates directories used for command output.
package outputdir

import (
	"path/filepath"

	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/internal/vfs"
)

// Ensure creates an output directory with owner-only permissions.
func Ensure(path string) error {
	if !filepath.IsAbs(path) {
		resolved, err := validate.SafeOutputPath(path)
		if err != nil {
			return err
		}
		path = resolved
	}
	return vfs.MkdirAll(path, 0700)
}

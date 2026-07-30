// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package common

import (
	"path/filepath"

	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/internal/vfs"
)

// EnsureOutputDir creates an output directory with owner-only permissions.
// Relative paths are validated and resolved within the working directory.
// Absolute paths are accepted for callers that already resolved them through
// SafeOutputPath or RuntimeContext.ResolveSavePath.
//
// The body sits here rather than in a package of its own: shortcuts/common is
// the runtime gate that shortcuts-runtime-gate exempts, so it already holds vfs
// and validate, and a package below it would add a hop that holds nothing the
// gate does not.
func EnsureOutputDir(path string) error {
	if !filepath.IsAbs(path) {
		resolved, err := validate.SafeOutputPath(path)
		if err != nil {
			return err
		}
		path = resolved
	}
	return vfs.MkdirAll(path, 0700)
}

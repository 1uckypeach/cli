// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package common

import "github.com/larksuite/cli/internal/outputdir"

// EnsureOutputDir creates an output directory with owner-only permissions.
// Relative paths are validated and resolved within the working directory.
// Absolute paths are accepted for callers that already resolved them through
// SafeOutputPath or RuntimeContext.ResolveSavePath.
func EnsureOutputDir(path string) error {
	return outputdir.Ensure(path)
}

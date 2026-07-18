// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package sheets

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/larksuite/cli/extension/fileio"
	"github.com/larksuite/cli/shortcuts/common"
)

// ─── lark_sheet read → file offload ───────────────────────────────────
//
// Shared plumbing for +cells-get / +csv-get / +table-get behind the
// --output-path flag: when a caller redirects a read to a file, the char cap
// should default to unlimited so the whole sheet lands on disk instead of being
// clipped by the stdout-oriented max_chars safety cap.

// readOutputPath returns the trimmed --output-path flag value ("" when unset).
func readOutputPath(runtime *common.RuntimeContext) string {
	return strings.TrimSpace(runtime.Str("output-path"))
}

// maxCharsInput resolves the max_chars value to send to the underlying read
// tool. With --output-path set the cap is lifted (unbounded sentinel) so the
// full result is written to the file; otherwise the --max-chars value binds.
// The second return is false when nothing should be sent (max-chars <= 0), in
// which case the tool's own default applies. Note the tool truncates at ~50000
// even when max_chars is omitted, so callers that want an explicit cap should
// pass a positive default.
func maxCharsInput(runtime *common.RuntimeContext) (int, bool) {
	if readOutputPath(runtime) != "" {
		return unboundedReadLimit, true
	}
	if n := runtime.Int("max-chars"); n > 0 {
		return n, true
	}
	return 0, false
}

// emitReadResult delivers a read shortcut's result. When --output-path is set it
// writes the data payload to that path as pretty JSON and prints a small
// confirmation envelope to stdout (path + byte count); otherwise it prints the
// full result envelope to stdout as usual.
func emitReadResult(runtime *common.RuntimeContext, out interface{}) error {
	path := readOutputPath(runtime)
	if path == "" {
		runtime.Out(out, nil)
		return nil
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err := runtime.FileIO().Save(path, fileio.SaveOptions{}, bytes.NewReader(b)); err != nil {
		return err
	}
	resolved, err := runtime.FileIO().ResolvePath(path)
	if err != nil {
		resolved = path
	}
	runtime.Out(map[string]interface{}{
		"output_path":   resolved,
		"bytes_written": len(b),
	}, nil)
	return nil
}

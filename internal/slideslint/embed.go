// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package slideslint runs the slides XML linter (skills/lark-slides/scripts/xml_lint.py)
// in-process via a CPython interpreter compiled to wasm32-wasi, executed by wazero.
// No system Python, no cgo, no on-disk extraction: the interpreter (stdlib frozen in),
// the lint scripts and the schema/iconpark data are all embedded and mounted read-only;
// slide XML goes in via stdin and the JSON verdict comes back on stdout.
package slideslint

import "embed"

// assetsFS holds the gzip'd CPython-wasi interpreter plus the lint scripts and data.
// The .py files are the unmodified originals from skills/lark-slides/scripts, so the
// lint verdict is byte-identical to running them under native python3.
//
//go:embed assets/python.wasm.gz assets/scripts/*.py assets/references/xml/*.xml assets/references/xml/*.json
var assetsFS embed.FS

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package apimeta lets Go module integrators install embedded API metadata
// for this process, equivalent to the meta_data.json compiled in via
// go:embed in official lark-cli builds.
//
// Binaries built from the bare Go module embed only an empty metadata stub,
// so schema resolution and generated service commands have nothing to work
// with offline. Integrators that ship their own metadata (typically embedded
// into their binary with their own go:embed directive) call SetEmbedded at
// process start to make the CLI treat those bytes as the compiled-in
// metadata, with no behavioral difference from an official build.
package apimeta

import "github.com/larksuite/cli/internal/registry"

// ErrAlreadyLoaded reports that SetEmbedded was called after the embedded
// metadata had already been parsed, so the injection was rejected. Use
// errors.Is(err, ErrAlreadyLoaded) to detect it.
var ErrAlreadyLoaded = registry.ErrMetaAlreadyLoaded

// SetEmbedded installs data as this process's embedded API metadata.
//
// It must be called before any registry consumption — cmd.Build, cmd.Execute,
// schema resolution, or scope discovery — typically at the top of main() or
// from an init() function; any package init in the embedder's dependency tree
// runs early enough. Calling it after the metadata has been parsed returns
// ErrAlreadyLoaded.
//
// data must parse as lark-cli API metadata and declare at least one service;
// otherwise an error is returned and the existing state (the empty stub, or
// the compiled-in metadata of an official build) is left unchanged. Calling
// SetEmbedded multiple times before the first parse is allowed: the last
// successful call wins, mirroring ordinary Go process-init trust — whoever
// links code into the binary controls its metadata.
func SetEmbedded(data []byte) error {
	return registry.SetEmbeddedMeta(data)
}

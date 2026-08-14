// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slideslint

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/build"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/vfs"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	wsys "github.com/tetratelabs/wazero/sys"
)

// engine holds the one-time-compiled interpreter and the read-only guest FS.
// CompileModule is the ~3s cost; it is paid once per process and persisted to a
// disk cache under the CLI config dir so it is skipped on later invocations.
type engine struct {
	rt       wazero.Runtime
	compiled wazero.CompiledModule
	guestFS  fs.FS
}

var (
	engOnce sync.Once
	eng     *engine
	engErr  error
)

func compileCacheDir() string {
	// Same root/override semantics as the rest of the CLI (LARKSUITE_CLI_CONFIG_DIR,
	// workspaces). Keyed by version so a new binary/wasm recompiles once.
	return filepath.Join(core.GetConfigDir(), "cache", "slides-lint-wasm", build.Version)
}

func getEngine(ctx context.Context) (*engine, error) {
	engOnce.Do(func() {
		gz, err := assetsFS.ReadFile("assets/python.wasm.gz")
		if err != nil {
			engErr = errs.NewInternalError(errs.SubtypeSDKError, "read embedded wasm: %s", err).WithCause(err)
			return
		}
		zr, err := gzip.NewReader(bytes.NewReader(gz))
		if err != nil {
			engErr = errs.NewInternalError(errs.SubtypeSDKError, "open wasm gzip: %s", err).WithCause(err)
			return
		}
		wasm, err := io.ReadAll(zr) // ~26MB, lives in RAM only; never written to disk
		if err != nil {
			engErr = errs.NewInternalError(errs.SubtypeSDKError, "decompress wasm: %s", err).WithCause(err)
			return
		}

		cfg := wazero.NewRuntimeConfig()
		if dir := compileCacheDir(); vfs.MkdirAll(dir, 0o700) == nil {
			if cache, cerr := wazero.NewCompilationCacheWithDir(dir); cerr == nil {
				cfg = cfg.WithCompilationCache(cache)
			}
		}
		rt := wazero.NewRuntimeWithConfig(ctx, cfg)
		wasi_snapshot_preview1.MustInstantiate(ctx, rt)

		compiled, err := rt.CompileModule(ctx, wasm)
		if err != nil {
			_ = rt.Close(ctx)
			engErr = errs.NewInternalError(errs.SubtypeSDKError, "compile wasm: %s", err).WithCause(err)
			return
		}
		sub, err := fs.Sub(assetsFS, "assets") // mount so /w/scripts and /w/references are siblings
		if err != nil {
			_ = rt.Close(ctx)
			engErr = errs.NewInternalError(errs.SubtypeSDKError, "sub assets fs: %s", err).WithCause(err)
			return
		}
		eng = &engine{rt: rt, compiled: compiled, guestFS: sub}
	})
	return eng, engErr
}

// runBatch feeds the slide XML strings to lint_batch.py (JSON array on stdin) and
// returns the decoded per-slide results (JSON array on stdout).
func runBatch(ctx context.Context, xmls []string) ([]Result, error) {
	e, err := getEngine(ctx)
	if err != nil {
		return nil, err
	}
	in, err := json.Marshal(xmls)
	if err != nil {
		return nil, errs.NewInternalError(errs.SubtypeSDKError, "marshal lint input: %s", err).WithCause(err)
	}

	var out, errBuf bytes.Buffer
	modCfg := wazero.NewModuleConfig().
		WithName(""). // anonymous → module can be instantiated repeatedly in one process
		WithStdin(bytes.NewReader(in)).
		WithStdout(&out).
		WithStderr(&errBuf).
		WithFSConfig(wazero.NewFSConfig().WithFSMount(e.guestFS, "/w")).
		WithArgs("python", "/w/scripts/lint_batch.py").
		WithEnv("PYTHONHOME", "/usr/local").
		WithEnv("PYTHONPATH", "/w/scripts").
		WithEnv("PYTHONDONTWRITEBYTECODE", "1").
		WithSysWalltime().
		WithSysNanotime()

	mod, err := e.rt.InstantiateModule(ctx, e.compiled, modCfg)
	if mod != nil {
		defer func() { _ = mod.Close(ctx) }()
	}
	if err != nil {
		// The entry never calls sys.exit(nonzero); a nonzero exit means the
		// interpreter itself failed. Surface stderr for diagnosis.
		if ee, ok := err.(*wsys.ExitError); ok && ee.ExitCode() != 0 {
			return nil, errs.NewInternalError(errs.SubtypeSDKError,
				"slides lint interpreter exited %d: %s", ee.ExitCode(), truncate(errBuf.String(), 500))
		}
		return nil, errs.NewInternalError(errs.SubtypeSDKError, "run slides lint: %s", err).WithCause(err)
	}

	var results []Result
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		return nil, errs.NewInternalError(errs.SubtypeSDKError,
			"parse lint output: %s (stderr: %s)", err, truncate(errBuf.String(), 300)).WithCause(err)
	}
	return results, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package envelope exposes lark-cli's error-dispatch decision to embedders.
//
// Integrators that call cmd.Build and drive Execute themselves must render
// errors exactly like the official binary so agents can parse stderr
// uniformly. DispatchError is the same function the official root dispatcher
// consumes — classification and envelope bytes cannot drift.
package envelope

import "github.com/larksuite/cli/internal/output"

// DispatchError classifies err exactly like lark-cli's own root dispatcher
// and returns the stderr envelope bytes (if any) together with the process
// exit code. identity is the resolved identity string ("user", "bot", or ""
// to omit the field). Typical embedder epilogue:
//
//	env, code, has := envelope.DispatchError(err, "user")
//	if has {
//		_, _ = os.Stderr.Write(env)
//	}
//	os.Exit(code)
func DispatchError(err error, identity string) (envelope []byte, exitCode int, hasEnvelope bool) {
	return output.DispatchError(err, identity)
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/larksuite/cli/errs"
)

// NoticeProvider supplies the notice attached to a structured envelope.
// The provider is captured by an Emitter so emission never reads the global
// PendingNotice hook implicitly.
type NoticeProvider func() map[string]interface{}

// PrettyRenderer writes the human-readable representation of one result.
// colorEnabled is the terminal capability captured when the Emitter is built.
type PrettyRenderer func(w io.Writer, colorEnabled bool) error

// EmitterConfig contains command-scoped dependencies. A command constructs one
// Emitter and reuses it for its success result or streamed pages.
type EmitterConfig struct {
	Out            io.Writer
	ErrOut         io.Writer
	CommandPath    string
	Identity       string
	ColorEnabled   bool
	NoticeProvider NoticeProvider
}

// EmitOptions describes one result's wire representation.
//
// The format contract is explicit: FormatJSON (the zero value) uses an
// Envelope; pretty, table, csv, and ndjson render naked business data. JQ takes
// precedence over Format and filters the JSON Envelope. Raw affects only JSON
// envelope encoding and jq's complex-value encoding. Format is a canonical
// typed value — boundaries reject unknown formats via ParseFormatStrict, so the
// Emitter never sees one and never falls back.
type EmitOptions struct {
	Raw    bool
	Meta   *Meta
	Format Format
	JQ     string
	DryRun bool
	Pretty PrettyRenderer
}

// StreamOptions describes one streamed page's wire representation. Streaming
// carries page items directly, so it deliberately exposes only the fields that
// affect a single page: the format and, for pretty, its renderer. It has no
// OK/Meta/DryRun/JQ — an ok:false envelope, metadata, dry-run, and jq all need
// the aggregated result, which the caller's pagination layer owns before it
// streams pages.
type StreamOptions struct {
	Format Format
	Pretty PrettyRenderer
}

// Emitter owns all command-scoped output dependencies and pagination state.
// It deliberately has no dependency on client or cmdutil.
type Emitter struct {
	out            io.Writer
	errOut         io.Writer
	commandPath    string
	identity       string
	colorEnabled   bool
	noticeProvider NoticeProvider
	scanCtx        scanContextFactory

	streamFormat    Format
	streamFormatter *PaginatedFormatter
}

// NewEmitter constructs a command-scoped output emitter.
func NewEmitter(config EmitterConfig) *Emitter {
	errOut := config.ErrOut
	if errOut == nil {
		errOut = io.Discard
	}
	return &Emitter{
		out:            config.Out,
		errOut:         errOut,
		commandPath:    config.CommandPath,
		identity:       config.Identity,
		colorEnabled:   config.ColorEnabled,
		noticeProvider: config.NoticeProvider,
		scanCtx:        defaultContentSafetyContext,
	}
}

// Success scans and emits one command result by composing the package's leaf
// primitives. JSON and jq use the standard envelope; pretty, table, csv, and
// ndjson render the business value directly.
func (e *Emitter) Success(data interface{}, opts EmitOptions) error {
	if !opts.Format.Valid() {
		return errs.NewInternalError(errs.SubtypeUnknown,
			"internal: unknown output format %d", int(opts.Format))
	}
	if err := e.requireOutput(); err != nil {
		return err
	}

	if opts.JQ != "" {
		return e.emitEnvelope(data, true, opts)
	}

	switch opts.Format {
	case FormatJSON:
		return e.emitEnvelope(data, true, opts)
	case FormatPretty:
		return e.emitPretty(data, opts)
	default:
		return e.emitFormatted(data, opts.Format)
	}
}

// PartialFailure emits a multi-status result whose envelope honestly reports
// ok:false. It is the typed counterpart to Success for batch operations where
// some items failed but the per-item outcomes are the primary stdout output.
// Like the legacy OutPartialFailure it produces only the JSON/jq envelope; the
// caller owns the non-zero exit signal, keeping the Emitter free of exit
// semantics.
func (e *Emitter) PartialFailure(data interface{}, opts EmitOptions) error {
	if !opts.Format.Valid() {
		return errs.NewInternalError(errs.SubtypeUnknown,
			"internal: unknown output format %d", int(opts.Format))
	}
	if err := e.requireOutput(); err != nil {
		return err
	}
	return e.emitEnvelope(data, false, opts)
}

// StreamPage scans and emits one page while retaining table/csv columns from
// the first page. Streamed output carries page items directly, so it takes a
// StreamOptions (format + optional pretty renderer) rather than the full
// EmitOptions: ok/meta/dry-run/jq all need the aggregated result and are the
// caller's pagination-layer responsibility, not a per-page concern. Excluding
// jq from the type makes "jq requires aggregated output" a compile-time fact
// instead of a runtime rejection.
func (e *Emitter) StreamPage(data interface{}, opts StreamOptions) error {
	if !opts.Format.Valid() {
		return errs.NewInternalError(errs.SubtypeUnknown,
			"internal: unknown output format %d", int(opts.Format))
	}
	if err := e.requireOutput(); err != nil {
		return err
	}

	if opts.Format == FormatPretty {
		if opts.Pretty != nil {
			return e.emitPrettyRenderer(opts.Pretty)
		}
		if e.streamFormatter == nil {
			fmt.Fprintln(e.errOut, "warning: --format pretty is not supported by this command; showing NDJSON instead")
		}
		opts.Format = FormatNDJSON
	}

	if e.streamFormatter == nil {
		e.streamFormat = opts.Format
		e.streamFormatter = NewPaginatedFormatter(nil, opts.Format)
	} else if opts.Format != e.streamFormat {
		return errs.NewInternalError(errs.SubtypeUnknown,
			"stream output format changed from %q to %q", e.streamFormat, opts.Format)
	}

	// Render this page, then scan the exact bytes before writing: a rule match
	// can form in the rendered page (joined table cells, adjacent objects) even
	// when no single value matches.
	var buf bytes.Buffer
	e.streamFormatter.W = &buf
	if err := e.streamFormatter.WritePage(data); err != nil {
		return wrapOutputError("render", err)
	}
	return e.emitScannedBuffer(&buf)
}

func (e *Emitter) emitEnvelope(data interface{}, ok bool, opts EmitOptions) error {
	scanResult := e.scanForSafety(data, false)
	if scanResult.Blocked {
		return scanResult.BlockErr
	}

	env := Envelope{
		OK:       ok,
		Identity: e.identity,
		DryRun:   opts.DryRun,
		Data:     data,
		Meta:     opts.Meta,
		Notice:   e.notice(),
	}
	if scanResult.Alert != nil {
		env.ContentSafetyAlert = scanResult.Alert
	}

	if opts.JQ != "" {
		if scanResult.Alert != nil {
			if err := WriteAlertWarning(e.errOut, scanResult.Alert); err != nil {
				return wrapOutputError("write", err)
			}
		}
		// Buffer the jq output manually so jq's own typed error (a validation
		// error for a bad expression, an api error for a runtime failure) is
		// returned unchanged; only a genuine stdout write failure is wrapped as
		// an internal output error.
		var buf bytes.Buffer
		var jqErr error
		if opts.Raw {
			jqErr = JqFilterRaw(&buf, env, opts.JQ)
		} else {
			jqErr = JqFilter(&buf, env, opts.JQ)
		}
		if jqErr != nil {
			return jqErr
		}
		// A jq expression can concatenate fields into a rule match that no
		// single value contains; scan the rendered bytes before writing.
		if err := e.blockIfRenderedUnsafe(&buf); err != nil {
			return err
		}
		if _, err := io.Copy(e.out, &buf); err != nil {
			return wrapOutputError("write", err)
		}
		return nil
	}

	// The JSON envelope is scanned via its data above: JSON serialization keeps
	// per-field punctuation between values, so a whitespace-joined cross-field
	// match cannot form the way it does for table/CSV or a jq concatenation.
	var buf bytes.Buffer
	var renderErr error
	if opts.Raw {
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		renderErr = enc.Encode(env)
	} else {
		renderErr = WriteJSON(&buf, env)
	}
	if renderErr != nil {
		return wrapOutputError("render", renderErr)
	}
	if _, err := io.Copy(e.out, &buf); err != nil {
		return wrapOutputError("write", err)
	}
	return nil
}

func (e *Emitter) emitPretty(data interface{}, opts EmitOptions) error {
	if opts.Pretty != nil {
		return e.emitPrettyRenderer(opts.Pretty)
	}

	// No pretty renderer: the command cannot render --format pretty. Rather than
	// failing after a write may already have mutated remote state (which would
	// make automation retry and duplicate resources), warn and fall back to JSON.
	fmt.Fprintln(e.errOut, "warning: --format pretty is not supported by this command; showing JSON instead")
	return e.emitEnvelope(data, true, opts)
}

func (e *Emitter) emitPrettyRenderer(renderer PrettyRenderer) error {
	// Buffer pretty output so the safety scan sees the exact text that will be
	// written to stdout, including anything captured by the opaque renderer.
	var buf bytes.Buffer
	if err := renderer(&buf, e.colorEnabled); err != nil {
		return wrapOutputError("render", err)
	}
	return e.emitScannedBuffer(&buf)
}

// emitFormatted renders naked business data for the non-envelope formats
// (ndjson, table, csv). Success routes FormatJSON to the envelope and
// FormatPretty to the pretty renderer, and boundaries reject unknown formats,
// so emitFormatted only ever receives a canonical non-envelope Format.
func (e *Emitter) emitFormatted(data interface{}, format Format) error {
	var buf bytes.Buffer
	if err := WriteFormatted(&buf, data, format); err != nil {
		return wrapOutputError("render", err)
	}
	return e.emitScannedBuffer(&buf)
}

// emitScannedBuffer scans the exact bytes that will be written to stdout and
// writes them only if the scan does not block. Scanning the rendered buffer —
// not the structured data — is what stops a rule match that only forms in the
// output (table cells joined by spaces, a jq string concatenation, adjacent
// NDJSON objects) from slipping past block mode. A warn-mode alert goes to
// stderr.
func (e *Emitter) emitScannedBuffer(buf *bytes.Buffer) error {
	scanResult := e.scanForSafety(buf.String(), true)
	if scanResult.Blocked {
		return scanResult.BlockErr
	}
	if scanResult.Alert != nil {
		if err := WriteAlertWarning(e.errOut, scanResult.Alert); err != nil {
			return wrapOutputError("write", err)
		}
	}
	if _, err := io.Copy(e.out, buf); err != nil {
		return wrapOutputError("write", err)
	}
	return nil
}

// blockIfRenderedUnsafe scans the exact rendered bytes and, in block mode,
// returns the block error when a rule matches text that only forms in the
// rendered output. The envelope's data scan owns warn-mode reporting (the
// embedded alert / stderr warning), so this adds no second alert.
func (e *Emitter) blockIfRenderedUnsafe(buf *bytes.Buffer) error {
	scanResult := e.scanForSafety(buf.String(), true)
	if scanResult.Blocked {
		return scanResult.BlockErr
	}
	return nil
}

func (e *Emitter) scanForSafety(data interface{}, fullText bool) ScanResult {
	return scanForSafety(e.commandPath, data, e.errOut, fullText, e.scanCtx)
}

func wrapOutputError(op string, err error) error {
	return errs.NewInternalError(errs.SubtypeUnknown, "failed to %s command output", op).WithCause(err)
}

func (e *Emitter) notice() map[string]interface{} {
	if e.noticeProvider == nil {
		return nil
	}
	return e.noticeProvider()
}

func (e *Emitter) requireOutput() error {
	if e == nil || e.out == nil {
		return errs.NewInternalError(errs.SubtypeUnknown,
			"success output writer is not configured")
	}
	return nil
}

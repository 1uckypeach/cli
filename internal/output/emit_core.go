// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package output

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	extcs "github.com/larksuite/cli/extension/contentsafety"
	"github.com/larksuite/cli/internal/envvars"
)

type mode uint8

const (
	modeOff mode = iota
	modeWarn
	modeBlock
)

// scanTimeout also bounds untruncated rendered-text scans.
const scanTimeout = 100 * time.Millisecond

var newContentSafetyContext = func() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), scanTimeout)
}

// modeFromEnv reads LARKSUITE_CLI_CONTENT_SAFETY_MODE.
func modeFromEnv(errOut io.Writer) mode {
	raw := strings.TrimSpace(os.Getenv(envvars.CliContentSafetyMode))
	if raw == "" {
		return modeOff
	}
	switch strings.ToLower(raw) {
	case "off":
		return modeOff
	case "warn":
		return modeWarn
	case "block":
		return modeBlock
	default:
		fmt.Fprintf(errOut,
			"warning: unknown %s value %q, falling back to off\n",
			envvars.CliContentSafetyMode, raw)
		return modeOff
	}
}

// normalizeCommandPath converts cobra CommandPath() to dotted form.
// "lark-cli im +messages-search" -> "im.messages_search"
func normalizeCommandPath(cobraPath string) string {
	segs := strings.Fields(cobraPath)
	if len(segs) <= 1 {
		return ""
	}
	segs = segs[1:]
	for i, s := range segs {
		s = strings.TrimPrefix(s, "+")
		s = strings.ReplaceAll(s, "-", "_")
		segs[i] = s
	}
	return strings.Join(segs, ".")
}

var (
	errBlocked        = errors.New("content safety blocked")
	errScanIncomplete = errors.New("content safety scan incomplete")
)

func runContentSafety(cobraPath string, data any, errOut io.Writer, fullText bool) (*extcs.Alert, error) {
	m := modeFromEnv(errOut)
	if m == modeOff {
		return nil, nil
	}

	p := extcs.GetProvider()
	if p == nil {
		return nil, nil
	}

	cmdPath := normalizeCommandPath(cobraPath)
	if cmdPath == "" {
		return nil, nil
	}

	type result struct {
		alert *extcs.Alert
		err   error
	}
	ch := make(chan result, 1)
	ctx, cancel := newContentSafetyContext()
	defer cancel()

	// A timed-out provider may outlive this call, so it cannot share errOut.
	scanErrBuf := &bytes.Buffer{}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{nil, fmt.Errorf("content safety panic: %v", r)}
			}
		}()
		a, e := p.Scan(ctx, extcs.ScanRequest{
			Path:     cmdPath,
			Data:     data,
			ErrOut:   scanErrBuf,
			FullText: fullText,
		})
		ch <- result{a, e}
	}()

	var res result
	select {
	case res = <-ch:
		if scanErrBuf.Len() > 0 {
			_, _ = io.Copy(errOut, scanErrBuf)
		}
		if ctx.Err() != nil && m == modeBlock {
			return nil, fmt.Errorf("%w: %w", errScanIncomplete, ctx.Err())
		}
	case <-ctx.Done():
		if m == modeBlock {
			return nil, fmt.Errorf("%w: %w", errScanIncomplete, ctx.Err())
		}
		return nil, nil
	}

	if res.err != nil {
		fmt.Fprintf(errOut, "warning: content safety scan error: %v\n", res.err)
		if m == modeBlock {
			return nil, fmt.Errorf("%w: %w", errScanIncomplete, res.err)
		}
		return nil, nil
	}
	if res.alert == nil {
		return nil, nil
	}

	if m == modeBlock {
		return res.alert, errBlocked
	}
	return res.alert, nil
}

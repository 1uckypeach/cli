// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"io"

	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
)

// PaginateToOutput fetches all requested pages and emits them in the selected format.
func PaginateToOutput(ctx context.Context, ac *APIClient, request RawApiRequest, format output.Format, jqExpr string, out, errOut io.Writer, commandPath string, pagOpts PaginationOptions, checkErr func(interface{}, core.Identity) error, markErr func(error) error) error {
	if markErr == nil {
		markErr = func(err error) error { return err }
	}
	if pagOpts.Identity == "" {
		pagOpts.Identity = request.As
	}
	// When jq is set, always aggregate all pages then filter.
	if jqExpr != "" {
		result, err := ac.PaginateAll(ctx, request, pagOpts)
		if err != nil {
			return markErr(err)
		}
		if apiErr := checkErr(result, pagOpts.Identity); apiErr != nil {
			output.FormatValue(out, result, output.FormatJSON)
			return markErr(apiErr)
		}
		return output.WriteSuccessEnvelope(output.SuccessEnvelopeData(result), output.SuccessEnvelopeOptions{
			CommandPath: commandPath,
			Identity:    string(pagOpts.Identity),
			JqExpr:      jqExpr,
			Out:         out,
			ErrOut:      errOut,
		})
	}

	switch format {
	case output.FormatNDJSON, output.FormatTable, output.FormatCSV:
		emitter := output.NewEmitter(output.EmitterConfig{
			Out:            out,
			ErrOut:         errOut,
			CommandPath:    commandPath,
			Identity:       string(pagOpts.Identity),
			NoticeProvider: output.GetNotice,
		})
		result, hasItems, err := ac.StreamPages(ctx, request, func(items []interface{}) error {
			// Streaming formats intentionally emit each page after that page has
			// passed safety scanning. A later page may still fail, so callers
			// must use the exit code to distinguish complete vs partial output.
			return emitter.StreamPage(items, output.StreamOptions{Format: format})
		}, pagOpts)
		if err != nil {
			return markErr(err)
		}
		if apiErr := checkErr(result, pagOpts.Identity); apiErr != nil {
			return markErr(apiErr)
		}
		if !hasItems {
			fmt.Fprintf(errOut, "warning: this API does not return a list, format %q is not supported, falling back to json\n", format)
			return output.WriteSuccessEnvelope(output.SuccessEnvelopeData(result), output.SuccessEnvelopeOptions{
				CommandPath: commandPath,
				Identity:    string(pagOpts.Identity),
				Out:         out,
				ErrOut:      errOut,
			})
		}
		return nil
	default:
		result, err := ac.PaginateAll(ctx, request, pagOpts)
		if err != nil {
			return markErr(err)
		}
		if apiErr := checkErr(result, pagOpts.Identity); apiErr != nil {
			output.FormatValue(out, result, output.FormatJSON)
			return markErr(apiErr)
		}
		return output.WriteSuccessEnvelope(output.SuccessEnvelopeData(result), output.SuccessEnvelopeOptions{
			CommandPath: commandPath,
			Identity:    string(pagOpts.Identity),
			Out:         out,
			ErrOut:      errOut,
		})
	}
}

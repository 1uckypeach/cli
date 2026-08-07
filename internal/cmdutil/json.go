// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/extension/fileio"
)

// ParseOptionalBody parses --data JSON for methods that accept a request body.
// Supports stdin (-), @file, @@-escape, and single-quote stripping via ResolveInput.
// Returns (nil, nil) if the method has no body or data is empty.
func ParseOptionalBody(httpMethod, data string, stdin io.Reader, fileIO fileio.FileIO) (interface{}, error) {
	switch httpMethod {
	case "POST", "PUT", "PATCH", "DELETE":
	default:
		return nil, nil
	}
	resolved, err := ResolveInput(data, stdin, fileIO)
	if err != nil {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--data: %s", err).
			WithParam("--data").
			WithCause(err)
	}
	if resolved == "" {
		return nil, nil
	}
	var body interface{}
	if err := decodeJSONPreserveNumbers([]byte(resolved), &body); err != nil {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--data invalid JSON format").
			WithParam("--data").
			WithCause(err)
	}
	return body, nil
}

// ParseJSONMap parses a JSON string into a map. Returns an empty (never nil) map
// for empty input or the JSON literal null, so callers can always overlay onto
// the result without a nil-map panic.
// Supports stdin (-), @file, @@-escape, and single-quote stripping via ResolveInput.
func ParseJSONMap(input, label string, stdin io.Reader, fileIO fileio.FileIO) (map[string]any, error) {
	resolved, err := ResolveInput(input, stdin, fileIO)
	if err != nil {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "%s: %s", label, err).
			WithParam(label).
			WithCause(err)
	}
	if resolved == "" {
		return map[string]any{}, nil
	}
	var result map[string]any
	if err := decodeJSONPreserveNumbers([]byte(resolved), &result); err != nil {
		return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "%s invalid format, expected JSON object", label).
			WithParam(label).
			WithCause(err)
	}
	if result == nil {
		// `null` unmarshals into a nil map without error; normalize it so the
		// returned map is always writable, matching the empty-input case.
		return map[string]any{}, nil
	}
	return result, nil
}

// decodeJSONPreserveNumbers keeps JSON number literals as json.Number so large
// integers survive dry-run rendering and request serialization unchanged.
// The initial RawMessage unmarshal preserves json.Unmarshal's strict
// single-value/trailing-data validation before the UseNumber decode.
func decodeJSONPreserveNumbers(data []byte, dst any) error {
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(dst)
}

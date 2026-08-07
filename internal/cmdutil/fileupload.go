// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/extension/fileio"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

// ParseFileFlag parses a --file flag value into its components.
// The format is either "path" or "field=path". When no explicit "field="
// prefix is present, defaultField is used as the field name.
// A path of "-" indicates stdin; in that case filePath is empty and isStdin is true.
func ParseFileFlag(raw, defaultField string) (fieldName, filePath string, isStdin bool) {
	if idx := strings.IndexByte(raw, '='); idx > 0 {
		fieldName = raw[:idx]
		filePath = raw[idx+1:]
	} else {
		fieldName = defaultField
		filePath = raw
	}
	if filePath == "-" {
		return fieldName, "", true
	}
	return fieldName, filePath, false
}

// ValidateFileFlag checks mutual exclusion rules for the --file flag.
// Returns nil if file is empty (flag not provided).
func ValidateFileFlag(file, params, data, outputPath string, pageAll bool, httpMethod string) error {
	if file == "" {
		return nil
	}

	_, filePath, isStdin := ParseFileFlag(file, "file")
	if !isStdin && filePath == "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--file: empty file path").
			WithParam("--file")
	}

	if outputPath != "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--file and --output are mutually exclusive").WithParams(
			errs.InvalidParam{Name: "--file", Reason: "mutually exclusive with --output"},
			errs.InvalidParam{Name: "--output", Reason: "mutually exclusive with --file"},
		)
	}
	if pageAll {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--file and --page-all are mutually exclusive").WithParams(
			errs.InvalidParam{Name: "--file", Reason: "mutually exclusive with --page-all"},
			errs.InvalidParam{Name: "--page-all", Reason: "mutually exclusive with --file"},
		)
	}
	if isStdin && data == "-" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--file and --data cannot both read from stdin").WithParams(
			errs.InvalidParam{Name: "--file", Reason: "only one flag may read from stdin"},
			errs.InvalidParam{Name: "--data", Reason: "only one flag may read from stdin"},
		)
	}
	if isStdin && params == "-" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--file and --params cannot both read from stdin").WithParams(
			errs.InvalidParam{Name: "--file", Reason: "only one flag may read from stdin"},
			errs.InvalidParam{Name: "--params", Reason: "only one flag may read from stdin"},
		)
	}

	switch httpMethod {
	case "POST", "PUT", "PATCH", "DELETE":
	default:
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--file requires POST, PUT, PATCH, or DELETE method").
			WithParam("--file").
			WithHint("file upload only applies to write methods; remove --file for read methods")
	}

	return nil
}

// FileUploadMeta holds file upload metadata for dry-run display.
// Returned by request builders when dry-run mode skips actual file reading.
type FileUploadMeta struct {
	FieldName  string
	FilePath   string
	FormFields any
}

// BuildFormdata constructs a multipart form data payload for file upload.
// If isStdin is true, the file content is read from stdin.
// Top-level keys from dataJSON are added as text form fields.
func BuildFormdata(fileIO fileio.FileIO, fieldName, filePath string, isStdin bool, stdin io.Reader, dataJSON any) (*larkcore.Formdata, error) {
	fd := larkcore.NewFormdata()

	if isStdin {
		if stdin == nil {
			return nil, errs.NewValidationError(errs.SubtypeFailedPrecondition, "--file: stdin is not available").
				WithParam("--file").
				WithHint("pipe the file content to stdin, or pass a file path instead of \"-\"")
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--file: failed to read stdin: %v", err).
				WithParam("--file").
				WithCause(err)
		}
		if len(data) == 0 {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--file: stdin is empty").
				WithParam("--file").
				WithHint("pipe non-empty file content to stdin")
		}
		fd.AddFile(fieldName, bytes.NewReader(data))
	} else {
		f, err := fileIO.Open(filePath)
		if err != nil {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "cannot open file: %s", filePath).
				WithParam("--file").
				WithCause(err)
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--file: failed to read %s: %v", filePath, err).
				WithParam("--file").
				WithCause(err)
		}
		fd.AddFileWithName(fieldName, filepath.Base(filePath), bytes.NewReader(data))
	}

	// Add top-level JSON keys as text form fields.
	if m, ok := dataJSON.(map[string]any); ok {
		const maxMultipartNumberExpansionBytes = 1 << 20
		totalNumberExpansionBytes := 0
		for k, v := range m {
			value := formatFormFieldValue(v)
			if n, ok := v.(json.Number); ok && len(value) > len(n.String()) {
				expansionBytes := len(value) - len(n.String())
				if expansionBytes > maxMultipartNumberExpansionBytes-totalNumberExpansionBytes {
					return nil, errs.NewValidationError(
						errs.SubtypeInvalidArgument,
						"--data numeric expansion exceeds the %d-byte multipart limit",
						maxMultipartNumberExpansionBytes,
					).
						WithParam("--data").
						WithHint("use ordinary decimal notation and smaller numeric exponents")
				}
				totalNumberExpansionBytes += expansionBytes
			}
			fd.AddField(k, value)
		}
	}

	return fd, nil
}

// formatFormFieldValue renders a JSON-unmarshalled value as a multipart form
// field string. Numeric values are handled specially: scientific notation
// (e.g. "1.185356e+06") is expanded to exact decimal notation because some
// backends reject exponents when parsing integer form fields. json.Number must
// not pass through float64, or large integers would lose precision. All other
// types fall through to %v.
func formatFormFieldValue(v any) string {
	switch n := v.(type) {
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	case json.Number:
		return expandJSONNumber(n)
	}
	return fmt.Sprintf("%v", v)
}

// expandJSONNumber converts a valid scientific-notation JSON number to exact
// decimal notation by moving the decimal point in its original digits. Values
// that already use decimal notation are returned unchanged.
func expandJSONNumber(n json.Number) string {
	raw := n.String()
	expAt := strings.IndexAny(raw, "eE")
	if expAt < 0 {
		return raw
	}

	exponent, err := strconv.ParseInt(raw[expAt+1:], 10, 32)
	if err != nil {
		return raw
	}
	mantissa := raw[:expAt]
	sign := ""
	if strings.HasPrefix(mantissa, "-") {
		sign = "-"
		mantissa = mantissa[1:]
	}

	dot := strings.IndexByte(mantissa, '.')
	integerDigits := len(mantissa)
	digits := mantissa
	if dot >= 0 {
		integerDigits = dot
		digits = mantissa[:dot] + mantissa[dot+1:]
	}
	if strings.Trim(digits, "0") == "" {
		return sign + "0"
	}
	decimalPos := int64(integerDigits) + exponent

	// Avoid turning a compact but pathological exponent into an unbounded
	// allocation. Such a value is not a practical multipart form field; leave
	// it unchanged so the server can reject it.
	const maxExpandedDigits = int64(1 << 20)
	if decimalPos > maxExpandedDigits || decimalPos < -maxExpandedDigits {
		return raw
	}

	var out string
	switch {
	case decimalPos <= 0:
		out = "0." + strings.Repeat("0", int(-decimalPos)) + digits
	case decimalPos >= int64(len(digits)):
		out = digits + strings.Repeat("0", int(decimalPos)-len(digits))
	default:
		pos := int(decimalPos)
		out = digits[:pos] + "." + digits[pos:]
	}
	if strings.Contains(out, ".") {
		out = strings.TrimRight(out, "0")
		out = strings.TrimRight(out, ".")
	}
	if !strings.Contains(out, ".") {
		out = strings.TrimLeft(out, "0")
		if out == "" {
			out = "0"
		}
	}
	return sign + out
}

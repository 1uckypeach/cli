// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package errs

import (
	"encoding/json"
	"errors"
)

const (
	paginationCompletedPagesField = "completed_pages"
	paginationNextPageTokenField  = "next_page_token"
)

// PaginationError adds resumable pagination progress to an existing typed
// error without changing its category, subtype, extension fields, or cause
// chain. The failed page's response is never represented as completed work.
type PaginationError struct {
	Problem
	Cause          error  `json:"-"`
	CompletedPages int    `json:"-"`
	NextPageToken  string `json:"-"`
	encoded        []byte
}

// NewPaginationError validates and snapshots the underlying typed error's
// complete JSON object before returning the wrapper. If the underlying object
// already owns a reserved progress field, it returns a serializable typed
// internal error instead of creating a wrapper whose MarshalJSON would fail at
// the final stderr boundary.
func NewPaginationError(cause error, completedPages int, nextPageToken string) error {
	problem, ok := ProblemOf(cause)
	if !ok {
		return NewInternalError(SubtypeUnknown,
			"cannot attach pagination progress to an untyped error").WithCause(cause)
	}
	paginationErr := &PaginationError{
		Problem:        *problem,
		Cause:          cause,
		CompletedPages: completedPages,
		NextPageToken:  nextPageToken,
	}
	encoded, err := paginationErr.buildJSON()
	if err != nil {
		return err
	}
	paginationErr.encoded = encoded
	return paginationErr
}

func (e *PaginationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return ""
	}
	return e.Cause.Error()
}

func (e *PaginationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *PaginationError) ProblemDetail() *Problem {
	if e == nil {
		return nil
	}
	return &e.Problem
}

// IsPagination reports whether err contains pagination recovery progress.
func IsPagination(err error) bool {
	var paginationErr *PaginationError
	return errors.As(err, &paginationErr)
}

func (e *PaginationError) MarshalJSON() ([]byte, error) {
	if e != nil && e.encoded != nil {
		return append([]byte(nil), e.encoded...), nil
	}
	return e.buildJSON()
}

func (e *PaginationError) buildJSON() ([]byte, error) {
	if e == nil || e.Cause == nil {
		return nil, NewInternalError(SubtypeUnknown, "cannot serialize pagination progress without an underlying error")
	}
	typed, ok := UnwrapTypedError(e.Cause)
	if !ok {
		return nil, NewInternalError(SubtypeUnknown,
			"cannot serialize pagination progress for untyped error").WithCause(e.Cause)
	}
	encoded, err := json.Marshal(typed)
	if err != nil {
		return nil, NewInternalError(SubtypeInvalidResponse,
			"failed to serialize underlying pagination error: %v", err).WithCause(e.Cause)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, NewInternalError(SubtypeInvalidResponse,
			"underlying pagination error did not serialize as a JSON object: %v", err).WithCause(e.Cause)
	}
	if fields == nil {
		return nil, NewInternalError(SubtypeInvalidResponse,
			"underlying pagination error did not serialize as a JSON object").WithCause(e.Cause)
	}
	for _, reserved := range []string{paginationCompletedPagesField, paginationNextPageTokenField} {
		if _, exists := fields[reserved]; exists {
			return nil, NewInternalError(SubtypeInvalidResponse,
				"cannot add pagination progress: underlying error already contains reserved field %q", reserved).WithCause(e.Cause)
		}
	}

	completed, err := json.Marshal(e.CompletedPages)
	if err != nil {
		return nil, NewInternalError(SubtypeInvalidResponse,
			"failed to serialize completed pagination page count: %v", err).WithCause(e.Cause)
	}
	fields[paginationCompletedPagesField] = completed
	if e.NextPageToken != "" {
		next, err := json.Marshal(e.NextPageToken)
		if err != nil {
			return nil, NewInternalError(SubtypeInvalidResponse,
				"failed to serialize pagination resume token: %v", err).WithCause(e.Cause)
		}
		fields[paginationNextPageTokenField] = next
	}
	encoded, err = json.Marshal(fields)
	if err != nil {
		return nil, NewInternalError(SubtypeInvalidResponse,
			"failed to serialize pagination progress: %v", err).WithCause(e.Cause)
	}
	return encoded, nil
}

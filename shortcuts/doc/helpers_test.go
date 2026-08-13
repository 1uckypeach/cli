// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/errclass"
)

func TestParseDocumentRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantKind     string
		wantToken    string
		wantFragment string
		wantErr      string
	}{
		{
			name:      "docx url",
			input:     "https://example.larksuite.com/docx/xxxxxx?from=wiki",
			wantKind:  "docx",
			wantToken: "xxxxxx",
		},
		{
			name:      "wiki url",
			input:     "https://example.larksuite.com/wiki/xxxxxx?from=wiki",
			wantKind:  "wiki",
			wantToken: "xxxxxx",
		},
		{
			name:         "wiki url with selection anchor",
			input:        "https://example.larksuite.com/wiki/xxxxxx#share-CUE3d6Ykno2fkexEvt8cGF8Wnse",
			wantKind:     "wiki",
			wantToken:    "xxxxxx",
			wantFragment: "share-CUE3d6Ykno2fkexEvt8cGF8Wnse",
		},
		{
			name:      "doc url",
			input:     "https://example.larksuite.com/doc/xxxxxx",
			wantKind:  "doc",
			wantToken: "xxxxxx",
		},
		{
			name:      "raw token",
			input:     "xxxxxx",
			wantKind:  "docx",
			wantToken: "xxxxxx",
		},
		{
			name:    "unsupported url",
			input:   "https://example.com/not-a-doc",
			wantErr: "unsupported --doc input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseDocumentRef(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Kind != tt.wantKind {
				t.Fatalf("parseDocumentRef(%q) kind = %q, want %q", tt.input, got.Kind, tt.wantKind)
			}
			if got.Token != tt.wantToken {
				t.Fatalf("parseDocumentRef(%q) token = %q, want %q", tt.input, got.Token, tt.wantToken)
			}
			if got.Fragment != tt.wantFragment {
				t.Fatalf("parseDocumentRef(%q) fragment = %q, want %q", tt.input, got.Fragment, tt.wantFragment)
			}
		})
	}
}

func TestBuildDriveRouteExtraEscapesJSON(t *testing.T) {
	t.Parallel()

	got, err := buildDriveRouteExtra(`doc-"quoted"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"drive_route_token":"doc-\"quoted\""}`
	if got != want {
		t.Fatalf("buildDriveRouteExtra() = %q, want %q", got, want)
	}
}

func TestAppendDocWarning(t *testing.T) {
	t.Parallel()

	appendDocWarning(nil, "ignored")

	empty := map[string]interface{}{}
	appendDocWarning(empty, "   ")
	if _, ok := empty["warnings"]; ok {
		t.Fatalf("blank warning should be ignored: %#v", empty)
	}

	tests := []struct {
		name string
		data map[string]interface{}
		want interface{}
	}{
		{
			name: "missing warnings",
			data: map[string]interface{}{},
			want: []string{"new warning"},
		},
		{
			name: "string slice warnings",
			data: map[string]interface{}{"warnings": []string{"old warning"}},
			want: []string{"old warning", "new warning"},
		},
		{
			name: "interface slice warnings",
			data: map[string]interface{}{"warnings": []interface{}{"old warning"}},
			want: []interface{}{"old warning", "new warning"},
		},
		{
			name: "scalar warning",
			data: map[string]interface{}{"warnings": "old warning"},
			want: []interface{}{"old warning", "new warning"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appendDocWarning(tt.data, "new warning")
			if got := tt.data["warnings"]; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("warnings = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestWithDocAPIRecovery(t *testing.T) {
	tests := []struct {
		name         string
		code         int
		bot          bool
		wantType     any
		wantCategory errs.Category
		wantSubtype  errs.Subtype
		wantHint     string
	}{
		{"document missing", 3380002, false, (*errs.APIError)(nil), errs.CategoryAPI, errs.SubtypeNotFound, "stop retrying"},
		{"document permission", 3380004, false, (*errs.PermissionError)(nil), errs.CategoryAuthorization, errs.SubtypePermissionDenied, "document owner"},
		{"user token invalid", -32011, false, (*errs.AuthenticationError)(nil), errs.CategoryAuthentication, errs.SubtypeTokenInvalid, "auth status --verify"},
		{"bot token guidance", -32011, true, (*errs.AuthenticationError)(nil), errs.CategoryAuthentication, errs.SubtypeTokenInvalid, "do not run user auth login"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sentinel := errors.New("sentinel cause")
			classified := errclass.BuildAPIError(map[string]any{"code": test.code, "msg": "upstream failure"}, errclass.ClassifyContext{Identity: "user"})
			attachDocTestCause(t, classified, sentinel)
			got := withDocAPIRecovery(classified, test.bot)
			problem, ok := errs.ProblemOf(got)
			if !ok || problem.Category != test.wantCategory || problem.Subtype != test.wantSubtype || problem.Code != test.code {
				t.Fatalf("problem = %+v, want %s/%s code %d", problem, test.wantCategory, test.wantSubtype, test.code)
			}
			if !strings.Contains(problem.Hint, test.wantHint) {
				t.Fatalf("hint = %q, want text containing %q", problem.Hint, test.wantHint)
			}
			if !errors.Is(got, sentinel) {
				t.Fatal("recovery must preserve the source cause")
			}
			switch test.wantType.(type) {
			case *errs.APIError:
				var target *errs.APIError
				if !errors.As(got, &target) {
					t.Fatalf("error = %T, want *errs.APIError", got)
				}
			case *errs.PermissionError:
				var target *errs.PermissionError
				if !errors.As(got, &target) {
					t.Fatalf("error = %T, want *errs.PermissionError", got)
				}
			case *errs.AuthenticationError:
				var target *errs.AuthenticationError
				if !errors.As(got, &target) {
					t.Fatalf("error = %T, want *errs.AuthenticationError", got)
				}
			}
		})
	}
}

func attachDocTestCause(t *testing.T, err error, cause error) {
	t.Helper()
	var apiErr *errs.APIError
	if errors.As(err, &apiErr) {
		apiErr.WithCause(cause)
		return
	}
	var permissionErr *errs.PermissionError
	if errors.As(err, &permissionErr) {
		permissionErr.WithCause(cause)
		return
	}
	var authenticationErr *errs.AuthenticationError
	if errors.As(err, &authenticationErr) {
		authenticationErr.WithCause(cause)
		return
	}
	t.Fatalf("unsupported source error type %T", err)
}

func TestWithDocAPIRecoveryPreservesUnrecognizedError(t *testing.T) {
	original := errs.NewAPIError(errs.SubtypeUnknown, "upstream failure").WithCode(12345)
	if got := withDocAPIRecovery(original, false); got != original {
		t.Fatalf("unrecognized error = %T %v, want original error", got, got)
	}
}

func TestWithDocWriteRecoveryPreventsUnsafeReplay(t *testing.T) {
	operations := []struct {
		operation docWriteOperation
		wantHint  string
	}{
		{docWriteCreate, "inspect the target folder"},
		{docWriteUpdate, "fetch the affected document scope"},
	}
	for _, subtype := range []errs.Subtype{errs.SubtypeNetworkServer, errs.SubtypeNetworkTimeout, errs.SubtypeNetworkTransport} {
		for _, test := range operations {
			t.Run(string(subtype)+"/"+string(test.operation), func(t *testing.T) {
				sentinel := errors.New("sentinel cause")
				original := errs.NewNetworkError(subtype, "request failed").
					WithRetryable().
					WithRetryAfterSeconds(5).
					WithCause(sentinel)
				got := withDocWriteRecovery(original, test.operation)
				problem, ok := errs.ProblemOf(got)
				if !ok || problem.Category != errs.CategoryNetwork || problem.Subtype != subtype {
					t.Fatalf("problem = %+v, want network/%s", problem, subtype)
				}
				var networkErr *errs.NetworkError
				if !errors.As(got, &networkErr) {
					t.Fatalf("error = %T, want *errs.NetworkError", got)
				}
				if problem.Retryable || networkErr.RetryAfterSeconds != 0 {
					t.Fatalf("problem = %+v, want retry metadata cleared", problem)
				}
				if !strings.Contains(problem.Hint, test.wantHint) {
					t.Fatalf("hint = %q, want text containing %q", problem.Hint, test.wantHint)
				}
				if !errors.Is(got, sentinel) {
					t.Fatal("recovery must preserve the source cause")
				}
				if !original.Retryable || original.RetryAfterSeconds != 5 {
					t.Fatalf("source error mutated: %+v", original)
				}
			})
		}
	}
}

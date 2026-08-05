// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	internalauth "github.com/larksuite/cli/internal/auth"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/errclass"
	"github.com/larksuite/cli/internal/recovery"
	"github.com/larksuite/cli/internal/surface"
)

func TestFactoryPresentErrorEnrichesPaginationCauseAndRebuildsSnapshot(t *testing.T) {
	f := &Factory{Recovery: recovery.NewProjector(nil)}
	source := errs.NewPaginationError(internalauth.NewNeedUserAuthorizationError("ou_test"), 1, "resume-page-2")

	rendered := f.PresentError(source, ErrorPresentationOptions{
		DeclaredScopes: func() []string { return []string{"docx:document:create"} },
	})
	var paginationErr *errs.PaginationError
	if !errors.As(rendered, &paginationErr) {
		t.Fatalf("PresentError() = %T, want PaginationError", rendered)
	}
	encoded, err := json.Marshal(rendered)
	if err != nil {
		t.Fatalf("json.Marshal(rendered): %v", err)
	}
	var wire map[string]interface{}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("json.Unmarshal(rendered): %v", err)
	}
	if got, want := wire["hint"], recovery.UserAuthorization("docx:document:create").String(); got != want {
		t.Fatalf("hint = %#v, want %q", got, want)
	}
	if wire["completed_pages"] != float64(1) || wire["next_page_token"] != "resume-page-2" {
		t.Fatalf("pagination progress = %#v", wire)
	}
}

func TestFactoryPresentErrorClonesAndPreservesPermissionMachineFields(t *testing.T) {
	cause := errors.New("permission cause")
	source := errs.NewPermissionError(errs.SubtypeMissingScope, "missing scope").
		WithCode(99991679).
		WithLogID("log-123").
		WithMissingScopes("docx:document").
		WithRequestedScopes("docx:document", "drive:drive").
		WithGrantedScopes("drive:drive").
		WithIdentity("user").
		WithCause(cause)
	plan := surface.NewPlan(map[surface.CommandID]surface.CommandState{
		surface.CommandAuthLogin: surface.CommandConcealed,
	})
	f := &Factory{
		ResolvedIdentity: core.AsUser,
		Recovery: recovery.NewProjector(func() *surface.Plan {
			return plan
		}),
	}

	rendered := f.PresentError(source, ErrorPresentationOptions{})
	presented, ok := rendered.(*errs.PermissionError)
	if !ok {
		t.Fatalf("PresentError() = %T, want *errs.PermissionError", rendered)
	}
	if presented == source {
		t.Fatal("PresentError returned the producer instead of a clone")
	}
	if !errors.Is(rendered, cause) {
		t.Fatalf("PresentError did not preserve cause %v: %v", cause, rendered)
	}
	problem, ok := errs.ProblemOf(rendered)
	if !ok {
		t.Fatalf("PresentError() = %T, want typed problem", rendered)
	}
	if problem.Category != errs.CategoryAuthorization || problem.Subtype != errs.SubtypeMissingScope {
		t.Fatalf("problem = %s/%s, want authorization/missing_scope", problem.Category, problem.Subtype)
	}
	if presented.Code != source.Code || presented.LogID != source.LogID ||
		presented.Identity != source.Identity || presented.Subtype != source.Subtype {
		t.Fatalf("presented machine fields = %+v, source = %+v", presented, source)
	}
	if strings.Join(presented.MissingScopes, ",") != strings.Join(source.MissingScopes, ",") ||
		strings.Join(presented.RequestedScopes, ",") != strings.Join(source.RequestedScopes, ",") ||
		strings.Join(presented.GrantedScopes, ",") != strings.Join(source.GrantedScopes, ",") {
		t.Fatalf("presented scope fields = %+v, source = %+v", presented, source)
	}
	if strings.Contains(presented.Hint, "auth login") ||
		!strings.Contains(presented.Hint, "supported authorization flow") {
		t.Fatalf("presented concealed hint = %q", presented.Hint)
	}
	if source.Hint != "" {
		t.Fatalf("PresentError mutated producer hint: %q", source.Hint)
	}
}

func TestFactoryPresentErrorRebuildsUnannotatedCanonicalHintWithInvocationContext(t *testing.T) {
	canonical := errclass.PermissionHint(nil, "user", errs.SubtypeMissingScope, "")
	source := errs.NewPermissionError(errs.SubtypeMissingScope, "missing scope").
		WithIdentity("user").
		WithHint("%s", canonical)
	projector := recovery.NewProjectorWithContext(nil, recovery.RenderContext{Profile: "team-beta"})
	f := &Factory{ResolvedIdentity: core.AsUser, Recovery: projector}

	rendered := f.PresentError(source, ErrorPresentationOptions{
		DeclaredScopes: func() []string { return []string{"calendar:calendar.event:read"} },
	})
	presented, ok := rendered.(*errs.PermissionError)
	if !ok {
		t.Fatalf("PresentError() = %T, want *errs.PermissionError", rendered)
	}
	for _, want := range []string{
		`--profile='team-beta'`,
		`--scope "calendar:calendar.event:read"`,
		"--no-wait --json",
		"--device-code",
	} {
		if !strings.Contains(presented.Hint, want) {
			t.Fatalf("presented hint %q does not contain %q", presented.Hint, want)
		}
	}
	if strings.Contains(presented.Hint, "--recommend") {
		t.Fatalf("presented hint retained generic recovery: %q", presented.Hint)
	}
	if source.Hint != canonical {
		t.Fatalf("PresentError mutated producer hint: got %q, want %q", source.Hint, canonical)
	}
}

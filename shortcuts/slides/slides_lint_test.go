// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slides

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/httpmock"
)

// TestMain disables the slides lint gate for this package by default. The gate's
// own logic is tested in internal/slideslint; the shortcut tests here predate it
// and use minimal fixtures the linter would reject, so they exercise the
// content-handling paths with the operational kill-switch on. Individual tests
// that want the gate re-enable it with t.Setenv.
func TestMain(m *testing.M) {
	os.Setenv("LARKSUITE_CLI_SLIDES_LINT", "off")
	os.Exit(m.Run())
}

// TestUpdateSlideLintGateBlocksOverlap is the in-repo regression guard that the
// gate is actually wired into +update-slide: an overlapping page is rejected
// before any API call. Runs the real wasm linter, so it is slower than the
// content-handling tests.
func TestUpdateSlideLintGateBlocksOverlap(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_SLIDES_LINT", "") // re-enable the gate for this test

	f, stdout, _, reg := cmdutil.TestFactory(t, slidesTestConfig(t, ""))
	called := false
	reg.Register(&httpmock.Stub{
		Method:   "POST",
		URL:      "/open-apis/slides_ai/v1/xml_presentations/pres_abc/slide/replace",
		Body:     map[string]interface{}{"code": 0, "data": map[string]interface{}{"revision_id": 1}},
		Optional: true,
		OnMatch:  func(*http.Request) { called = true },
	})

	overlap := `<slide id="pYw"><data>` +
		`<shape type="text" topLeftX="80" topLeftY="80" width="300" height="40"><content textType="title"><p>a very long overflowing title</p></content></shape>` +
		`<shape type="text" topLeftX="90" topLeftY="85" width="300" height="40"><content textType="body"><p>overlaps</p></content></shape>` +
		`</data></slide>`

	err := runSlidesShortcut(t, f, stdout, SlidesUpdateSlide, []string{
		"+update-slide", "--presentation", "pres_abc", "--slide-id", "pYw",
		"--content", overlap, "--as", "user",
	})
	if err == nil {
		t.Fatal("expected lint gate to block overlapping page, got nil error")
	}
	if !strings.Contains(err.Error(), "bbox_overlap") {
		t.Fatalf("expected bbox_overlap in error, got: %v", err)
	}
	if called {
		t.Fatal("a lint-blocked edit must not reach the API")
	}
}

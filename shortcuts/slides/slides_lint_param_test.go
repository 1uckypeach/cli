// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slides

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/shortcuts/common"
)

// The lint switch is asserted on the wire rather than on the query-builder
// return value. What matters is the parameter the backend receives, and only a
// real request proves the flag reached it: a builder test would still pass if a
// command stopped calling its own builder.

// TestAddSlideLintXMLTravels pins both directions on +add-slide.
func TestAddSlideLintXMLTravels(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		extra []string
		want  string
	}{
		{name: "default lints", want: "true"},
		{name: "--no-lint disables", extra: []string{"--no-lint"}, want: "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f, stdout, _, reg := cmdutil.TestFactory(t, slidesTestConfig(t, ""))
			var query url.Values
			reg.Register(&httpmock.Stub{
				Method:  "POST",
				URL:     "/open-apis/slides_ai/v1/xml_presentations/pres_abc/slide",
				Body:    map[string]interface{}{"code": 0, "data": map[string]interface{}{"slide_id": "slide_001"}},
				OnMatch: func(req *http.Request) { query = req.URL.Query() },
			})

			args := append([]string{
				"+add-slide",
				"--presentation", "pres_abc",
				"--slide", testPageXML,
				"--as", "user",
			}, tc.extra...)
			if err := runSlidesShortcut(t, f, stdout, SlidesAddSlide, args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := query.Get(lintXMLParamKey); got != tc.want {
				t.Fatalf("%s = %q, want %q", lintXMLParamKey, got, tc.want)
			}
		})
	}
}

// TestUpdateSlideLintXMLTravels pins both directions on +update-slide, and that
// the switch is added to the query rather than replacing what was already there.
func TestUpdateSlideLintXMLTravels(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		extra []string
		want  string
	}{
		{name: "default lints", want: "true"},
		{name: "--no-lint disables", extra: []string{"--no-lint"}, want: "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f, stdout, _, reg := cmdutil.TestFactory(t, slidesTestConfig(t, ""))
			var query url.Values
			reg.Register(&httpmock.Stub{
				Method:  "POST",
				URL:     "/open-apis/slides_ai/v1/xml_presentations/pres_abc/slide/replace",
				Body:    map[string]interface{}{"code": 0, "data": map[string]interface{}{"revision_id": 9}},
				OnMatch: func(req *http.Request) { query = req.URL.Query() },
			})

			args := append([]string{
				"+update-slide",
				"--presentation", "pres_abc",
				"--slide-id", "bUn",
				"--content", testPageXML,
				"--as", "user",
			}, tc.extra...)
			if err := runSlidesShortcut(t, f, stdout, SlidesUpdateSlide, args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := query.Get(lintXMLParamKey); got != tc.want {
				t.Fatalf("%s = %q, want %q", lintXMLParamKey, got, tc.want)
			}
			if got := query.Get("slide_id"); got != "bUn" {
				t.Fatalf("slide_id = %q, want bUn — the switch must be added to the query, not replace it", got)
			}
		})
	}
}

// TestCreateLintXMLTravels pins both directions on +create, and that the switch
// rides every page call rather than only the first.
func TestCreateLintXMLTravels(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		extra []string
		want  string
	}{
		{name: "default lints", want: "true"},
		{name: "--no-lint disables", extra: []string{"--no-lint"}, want: "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f, stdout, _, reg := cmdutil.TestFactory(t, slidesTestConfig(t, ""))
			var createQuery url.Values
			var slideQueries []url.Values
			reg.Register(&httpmock.Stub{
				Method: "POST",
				URL:    "/open-apis/slides_ai/v1/xml_presentations",
				Body: map[string]interface{}{"code": 0, "data": map[string]interface{}{
					"xml_presentation_id": "pres_lint",
					"revision_id":         1,
				}},
				OnMatch: func(req *http.Request) { createQuery = req.URL.Query() },
			})
			for i, slideID := range []string{"slide_001", "slide_002"} {
				reg.Register(&httpmock.Stub{
					Method:  "POST",
					URL:     "/open-apis/slides_ai/v1/xml_presentations/pres_lint/slide",
					Body:    map[string]interface{}{"code": 0, "data": map[string]interface{}{"slide_id": slideID, "revision_id": i + 2}},
					OnMatch: func(req *http.Request) { slideQueries = append(slideQueries, req.URL.Query()) },
				})
			}

			args := append([]string{
				"+create",
				"--title", "Lint",
				"--slide", testPageXML,
				"--slide", testPageXML,
				"--as", "user",
			}, tc.extra...)
			if err := runSlidesCreateShortcut(t, f, stdout, args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(slideQueries) != 2 {
				t.Fatalf("page calls = %d, want 2", len(slideQueries))
			}
			for i, q := range slideQueries {
				if got := q.Get(lintXMLParamKey); got != tc.want {
					t.Fatalf("page %d: %s = %q, want %q", i+1, lintXMLParamKey, got, tc.want)
				}
			}
			// The presentation-create body is the title-only <presentation> shell,
			// so there is no page in it to lint and the switch has no meaning there.
			if got := createQuery.Get(lintXMLParamKey); got != "" {
				t.Fatalf("presentation create carried %s = %q, want it absent", lintXMLParamKey, got)
			}
		})
	}
}

// TestLintXMLFlagIsOnEveryWholePageWriter guards the set. A new whole-page
// writer that forgets the flag ships pages the backend never checked, and the
// omission is invisible until something renders wrong.
func TestLintXMLFlagIsOnEveryWholePageWriter(t *testing.T) {
	t.Parallel()

	for _, sc := range []struct {
		name string
		want bool
		have []string
	}{
		{name: "+create", want: true, have: lintFlagNames(SlidesCreate.Flags)},
		{name: "+add-slide", want: true, have: lintFlagNames(SlidesAddSlide.Flags)},
		{name: "+update-slide", want: true, have: lintFlagNames(SlidesUpdateSlide.Flags)},
		// +replace-slide sends block fragments, not a whole <slide>, so a
		// page-level lint has nothing to measure and the flag stays off it.
		{name: "+replace-slide", want: false, have: lintFlagNames(SlidesReplaceSlide.Flags)},
	} {
		if got := flagListHas(sc.have, noLintFlagName); got != sc.want {
			t.Fatalf("%s has --%s = %v, want %v", sc.name, noLintFlagName, got, sc.want)
		}
	}
}

func lintFlagNames(flags []common.Flag) []string {
	names := make([]string, 0, len(flags))
	for _, f := range flags {
		names = append(names, f.Name)
	}
	return names
}

func flagListHas(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

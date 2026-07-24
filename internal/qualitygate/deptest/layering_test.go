// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package deptest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/larksuite/cli/internal/vfs"
)

const modulePath = "github.com/larksuite/cli"

// Mode selects whether a rule checks direct or transitive dependencies.
type Mode int

const (
	// Direct checks package imports.
	Direct Mode = iota
	// Transitive checks the complete dependency set.
	Transitive
)

// Rule defines one package-layer dependency restriction.
type Rule struct {
	Name       string
	Mode       Mode
	FromPrefix string
	Denied     []string
	ExceptFrom []string
	SkipFrom   []string
}

var rules = []Rule{
	{
		Name:       "extension-zero-internal",
		Mode:       Transitive,
		FromPrefix: modulePath + "/extension",
		Denied:     []string{modulePath + "/internal/"},
		SkipFrom:   []string{"/examples/"},
	},
	{
		Name:       "events-no-shortcuts",
		Mode:       Transitive,
		FromPrefix: modulePath + "/events",
		Denied:     []string{modulePath + "/shortcuts/"},
	},
	{
		Name:       "shortcuts-runtime-gate",
		Mode:       Direct,
		FromPrefix: modulePath + "/shortcuts",
		Denied: []string{
			modulePath + "/internal/auth",
			modulePath + "/internal/keychain",
			modulePath + "/internal/credential",
			modulePath + "/internal/client",
			modulePath + "/internal/vfs",
		},
		ExceptFrom: []string{
			modulePath + "/shortcuts/common",
			modulePath + "/shortcuts/apps/gitcred",
		},
	},
	{
		Name:       "cmd-assembly-only",
		Mode:       Direct,
		FromPrefix: modulePath + "/cmd",
		Denied:     []string{modulePath + "/shortcuts"},
		ExceptFrom: []string{
			modulePath + "/cmd",
			modulePath + "/cmd/auth",
		},
	},
	{
		Name:       "errs-leaf",
		Mode:       Direct,
		FromPrefix: modulePath + "/errs",
		Denied:     []string{modulePath + "/"},
	},
	{
		Name:       "internal-no-upper",
		Mode:       Direct,
		FromPrefix: modulePath + "/internal",
		Denied: []string{
			modulePath + "/cmd",
			modulePath + "/shortcuts",
			modulePath + "/events",
		},
		ExceptFrom: []string{
			modulePath + "/internal/qualitygate/cmd/manifest-export",
		},
	},
}

type listedPackage struct {
	ImportPath string
	Imports    []string
	Deps       []string
}

type layeringEdge struct {
	From   string
	Denied string
}

type layeringViolation struct {
	layeringEdge
	Rule string
}

type seededLayeringEdge struct {
	layeringEdge
	Owner   string
	Reason  string
	AddedAt time.Time
	Line    int
}

func TestPackageLayering(t *testing.T) {
	root := repoRoot(t)
	packages := goListPackageGraph(t, root)
	seeded := readLayeringEdges(t, filepath.Join(root, "internal/qualitygate/deptest/layering-edges.txt"))
	seededByEdge := indexSeededLayeringEdges(t, seeded)

	actualByRule := make(map[string][]layeringViolation, len(rules))
	actualEdges := make(map[layeringEdge]struct{})
	for _, rule := range rules {
		violations := evaluateLayeringRule(packages, rule)
		actualByRule[rule.Name] = violations
		for _, violation := range violations {
			actualEdges[violation.layeringEdge] = struct{}{}
		}
	}

	for _, rule := range rules {
		t.Run(rule.Name, func(t *testing.T) {
			for _, violation := range actualByRule[rule.Name] {
				if _, ok := seededByEdge[violation.layeringEdge]; ok {
					continue
				}
				t.Errorf(
					"new layering violation: from=%s denied=%s rule=%s; use the approved dependency gate or fix the dependency; do not add rows to layering-edges.txt",
					violation.From,
					violation.Denied,
					violation.Rule,
				)
			}
		})
	}

	t.Run("stale-layering-edges", func(t *testing.T) {
		for _, edge := range seeded {
			if _, ok := actualEdges[edge.layeringEdge]; ok {
				continue
			}
			t.Errorf(
				"stale layering edge: from=%s denied=%s line=%d; this violation has been removed; delete this row from layering-edges.txt",
				edge.From,
				edge.Denied,
				edge.Line,
			)
		}
	})
}

func TestParseLayeringEdges(t *testing.T) {
	t.Run("valid-rows", func(t *testing.T) {
		input := strings.NewReader(
			"# from\tdenied\towner\treason\tadded_at\n" +
				"\n" +
				"example.com/from\texample.com/denied\towner\tlegacy dependency\t2026-07-24\r\n",
		)
		edges, err := parseLayeringEdges(input)
		if err != nil {
			t.Fatalf("parseLayeringEdges returned an error: %v", err)
		}
		if len(edges) != 1 {
			t.Fatalf("parseLayeringEdges returned %d rows, want 1", len(edges))
		}
		edge := edges[0]
		if edge.From != "example.com/from" || edge.Denied != "example.com/denied" {
			t.Fatalf("parseLayeringEdges returned unexpected edge: %+v", edge)
		}
		if edge.Owner != "owner" || edge.Reason != "legacy dependency" {
			t.Fatalf("parseLayeringEdges returned unexpected metadata: %+v", edge)
		}
		if got := edge.AddedAt.Format(time.DateOnly); got != "2026-07-24" {
			t.Fatalf("parseLayeringEdges returned added_at %q, want 2026-07-24", got)
		}
		if edge.Line != 3 {
			t.Fatalf("parseLayeringEdges returned line %d, want 3", edge.Line)
		}
	})

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "wrong-field-count",
			input: "from\tdenied\towner\treason\n",
		},
		{
			name:  "blank-field",
			input: "from\tdenied\t\treason\t2026-07-24\n",
		},
		{
			name:  "invalid-date",
			input: "from\tdenied\towner\treason\t2026-02-30\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseLayeringEdges(strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("parseLayeringEdges returned nil error for a malformed row")
			}
			if !strings.Contains(err.Error(), "line 1") {
				t.Fatalf("parseLayeringEdges error %q does not identify line 1", err)
			}
		})
	}
}

func TestMatchesPackagePrefix(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		importPath string
		want       bool
	}{
		{
			name:       "exact-package",
			prefix:     modulePath + "/errs",
			importPath: modulePath + "/errs",
			want:       true,
		},
		{
			name:       "child-package",
			prefix:     modulePath + "/internal/vfs",
			importPath: modulePath + "/internal/vfs/localfileio",
			want:       true,
		},
		{
			name:       "trailing-slash-prefix",
			prefix:     modulePath + "/internal/",
			importPath: modulePath + "/internal/vfs",
			want:       true,
		},
		{
			name:       "adjacent-package-name",
			prefix:     modulePath + "/errs",
			importPath: modulePath + "/errclass",
			want:       false,
		},
		{
			name:       "unrelated-package",
			prefix:     modulePath + "/events/",
			importPath: modulePath + "/shortcuts/im",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesPackagePrefix(tt.prefix, tt.importPath); got != tt.want {
				t.Fatalf(
					"matchesPackagePrefix(%q, %q) = %t, want %t",
					tt.prefix,
					tt.importPath,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestEvaluateLayeringRuleUsesExactExceptions(t *testing.T) {
	rule := Rule{
		Name:       "exact-exception",
		Mode:       Direct,
		FromPrefix: modulePath + "/cmd",
		Denied:     []string{modulePath + "/shortcuts"},
		ExceptFrom: []string{modulePath + "/cmd"},
	}
	packages := []listedPackage{
		{
			ImportPath: modulePath + "/cmd",
			Imports:    []string{modulePath + "/shortcuts"},
		},
		{
			ImportPath: modulePath + "/cmd/service",
			Imports:    []string{modulePath + "/shortcuts"},
		},
	}

	violations := evaluateLayeringRule(packages, rule)
	if len(violations) != 1 {
		t.Fatalf("evaluateLayeringRule returned %d violations, want 1", len(violations))
	}
	if got := violations[0].From; got != modulePath+"/cmd/service" {
		t.Fatalf("evaluateLayeringRule returned source %q, want %q", got, modulePath+"/cmd/service")
	}
}

func goListPackageGraph(t *testing.T, root string) []listedPackage {
	t.Helper()
	args := []string{"list", "-json", "-tags", "authsidecar", "./..."}
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}

	decoder := json.NewDecoder(bytes.NewReader(out))
	var packages []listedPackage
	for {
		var pkg listedPackage
		err := decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		packages = append(packages, pkg)
	}
	return packages
}

func evaluateLayeringRule(packages []listedPackage, rule Rule) []layeringViolation {
	var violations []layeringViolation
	for _, pkg := range packages {
		if !matchesPackagePrefix(rule.FromPrefix, pkg.ImportPath) {
			continue
		}
		if slices.Contains(rule.ExceptFrom, pkg.ImportPath) {
			continue
		}
		if containsAny(pkg.ImportPath, rule.SkipFrom) {
			continue
		}

		dependencies := pkg.Imports
		if rule.Mode == Transitive {
			dependencies = pkg.Deps
		}
		for _, dependency := range dependencies {
			if !matchesAnyPackagePrefix(rule.Denied, dependency) {
				continue
			}
			violations = append(violations, layeringViolation{
				layeringEdge: layeringEdge{
					From:   pkg.ImportPath,
					Denied: dependency,
				},
				Rule: rule.Name,
			})
		}
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].From != violations[j].From {
			return violations[i].From < violations[j].From
		}
		return violations[i].Denied < violations[j].Denied
	})
	return violations
}

func matchesPackagePrefix(prefix, importPath string) bool {
	if importPath == prefix {
		return true
	}
	if !strings.HasPrefix(importPath, prefix) {
		return false
	}
	return strings.HasSuffix(prefix, "/") || importPath[len(prefix)] == '/'
}

func matchesAnyPackagePrefix(prefixes []string, importPath string) bool {
	for _, prefix := range prefixes {
		if matchesPackagePrefix(prefix, importPath) {
			return true
		}
	}
	return false
}

func containsAny(value string, substrings []string) bool {
	for _, substring := range substrings {
		if strings.Contains(value, substring) {
			return true
		}
	}
	return false
}

func readLayeringEdges(t *testing.T, path string) []seededLayeringEdge {
	t.Helper()
	file, err := vfs.Open(path)
	if err != nil {
		t.Fatalf("open layering edges: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close layering edges: %v", err)
		}
	}()

	edges, err := parseLayeringEdges(file)
	if err != nil {
		t.Fatalf("parse layering edges: %v", err)
	}
	return edges
}

func parseLayeringEdges(r io.Reader) ([]seededLayeringEdge, error) {
	scanner := bufio.NewScanner(r)
	var edges []seededLayeringEdge
	var problems []string
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimRight(scanner.Text(), "\r")
		if skipLayeringEdgeLine(text) {
			continue
		}
		parts := strings.Split(text, "\t")
		if len(parts) != 5 {
			problems = append(problems, malformedLayeringEdge(line))
			continue
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		addedAt, dateErr := time.Parse(time.DateOnly, parts[4])
		if hasBlank(parts...) || dateErr != nil {
			problems = append(problems, malformedLayeringEdge(line))
			continue
		}
		edges = append(edges, seededLayeringEdge{
			layeringEdge: layeringEdge{
				From:   parts[0],
				Denied: parts[1],
			},
			Owner:   parts[2],
			Reason:  parts[3],
			AddedAt: addedAt,
			Line:    line,
		})
	}
	if err := scanner.Err(); err != nil {
		problems = append(problems, "failed to scan layering edges: "+err.Error())
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return edges, nil
}

func skipLayeringEdgeLine(text string) bool {
	trimmed := strings.TrimSpace(text)
	return trimmed == "" || strings.HasPrefix(trimmed, "#")
}

func hasBlank(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func malformedLayeringEdge(line int) string {
	return fmt.Sprintf(
		"line %d: layering edge row must have five tab-separated non-empty fields with added_at in YYYY-MM-DD format",
		line,
	)
}

func indexSeededLayeringEdges(t *testing.T, edges []seededLayeringEdge) map[layeringEdge]seededLayeringEdge {
	t.Helper()
	indexed := make(map[layeringEdge]seededLayeringEdge, len(edges))
	for _, edge := range edges {
		if previous, ok := indexed[edge.layeringEdge]; ok {
			t.Fatalf(
				"duplicate layering edge at lines %d and %d: from=%s denied=%s",
				previous.Line,
				edge.Line,
				edge.From,
				edge.Denied,
			)
		}
		indexed[edge.layeringEdge] = edge
	}
	return indexed
}

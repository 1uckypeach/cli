// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package deptest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
		FromPrefix: modulePath + "/extension/",
		Denied:     []string{modulePath + "/internal/"},
		SkipFrom:   []string{"/examples/"},
	},
	{
		Name:       "events-no-shortcuts",
		Mode:       Transitive,
		FromPrefix: modulePath + "/events/",
		Denied:     []string{modulePath + "/shortcuts/"},
	},
	{
		Name:       "shortcuts-runtime-gate",
		Mode:       Direct,
		FromPrefix: modulePath + "/shortcuts/",
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
		FromPrefix: modulePath + "/internal/",
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

type goListTarget struct {
	GOOS   string
	GOARCH string
}

type commandFactory func(name string, args ...string) *exec.Cmd

var layeringBuildTargets = []goListTarget{
	{GOOS: "linux", GOARCH: "amd64"},
	{GOOS: "linux", GOARCH: "arm64"},
	{GOOS: "linux", GOARCH: "riscv64"},
	{GOOS: "darwin", GOARCH: "amd64"},
	{GOOS: "darwin", GOARCH: "arm64"},
	{GOOS: "windows", GOARCH: "amd64"},
	{GOOS: "windows", GOARCH: "arm64"},
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
			for _, violation := range findUnseededLayeringViolations(actualByRule[rule.Name], seededByEdge) {
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
		for _, edge := range findStaleLayeringEdges(seeded, actualEdges) {
			t.Errorf(
				"stale layering edge: from=%s denied=%s line=%d; this violation has been removed; delete this row from layering-edges.txt",
				edge.From,
				edge.Denied,
				edge.Line,
			)
		}
	})
}

func TestLayeringEdgeClassification(t *testing.T) {
	known := layeringEdge{From: "example.com/from", Denied: "example.com/denied"}
	added := layeringEdge{From: "example.com/new", Denied: "example.com/upper"}
	removed := layeringEdge{From: "example.com/old", Denied: "example.com/legacy"}
	seeded := []seededLayeringEdge{
		{layeringEdge: known, Line: 1},
		{layeringEdge: removed, Line: 2},
	}
	seededByEdge := map[layeringEdge]seededLayeringEdge{
		known:   seeded[0],
		removed: seeded[1],
	}
	actual := []layeringViolation{
		{layeringEdge: known, Rule: "rule"},
		{layeringEdge: added, Rule: "rule"},
	}
	actualEdges := map[layeringEdge]struct{}{
		known: {},
		added: {},
	}

	unseeded := findUnseededLayeringViolations(actual, seededByEdge)
	if len(unseeded) != 1 || unseeded[0].layeringEdge != added {
		t.Fatalf("findUnseededLayeringViolations returned %+v, want only %+v", unseeded, added)
	}
	stale := findStaleLayeringEdges(seeded, actualEdges)
	if len(stale) != 1 || stale[0].layeringEdge != removed {
		t.Fatalf("findStaleLayeringEdges returned %+v, want only %+v", stale, removed)
	}
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

func TestLayeringRuleContracts(t *testing.T) {
	wantRules := []Rule{
		{
			Name:       "extension-zero-internal",
			Mode:       Transitive,
			FromPrefix: modulePath + "/extension/",
			Denied:     []string{modulePath + "/internal/"},
			SkipFrom:   []string{"/examples/"},
		},
		{
			Name:       "events-no-shortcuts",
			Mode:       Transitive,
			FromPrefix: modulePath + "/events/",
			Denied:     []string{modulePath + "/shortcuts/"},
		},
		{
			Name:       "shortcuts-runtime-gate",
			Mode:       Direct,
			FromPrefix: modulePath + "/shortcuts/",
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
			FromPrefix: modulePath + "/internal/",
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
	if !reflect.DeepEqual(rules, wantRules) {
		t.Fatalf("layering rules differ from the enforced contract:\ngot:  %#v\nwant: %#v", rules, wantRules)
	}

	tests := []struct {
		name       string
		ruleName   string
		packages   []listedPackage
		wantFrom   string
		wantDenied string
	}{
		{
			name:     "extension-transitive-denial-and-example-scope-out",
			ruleName: "extension-zero-internal",
			packages: []listedPackage{
				{
					ImportPath: modulePath + "/extension/sdk",
					Deps:       []string{modulePath + "/internal/core"},
				},
				{
					ImportPath: modulePath + "/extension/platform/examples/demo",
					Deps:       []string{modulePath + "/internal/core"},
				},
			},
			wantFrom:   modulePath + "/extension/sdk",
			wantDenied: modulePath + "/internal/core",
		},
		{
			name:     "events-transitive-denial",
			ruleName: "events-no-shortcuts",
			packages: []listedPackage{
				{
					ImportPath: modulePath + "/events/im",
					Deps:       []string{modulePath + "/shortcuts/common"},
				},
				{
					ImportPath: modulePath + "/events/calendar",
					Deps:       []string{modulePath + "/internal/core"},
				},
			},
			wantFrom:   modulePath + "/events/im",
			wantDenied: modulePath + "/shortcuts/common",
		},
		{
			name:     "shortcuts-direct-denial-and-exceptions",
			ruleName: "shortcuts-runtime-gate",
			packages: []listedPackage{
				{
					ImportPath: modulePath + "/shortcuts/im",
					Imports:    []string{modulePath + "/internal/auth"},
				},
				{
					ImportPath: modulePath + "/shortcuts/common",
					Imports:    []string{modulePath + "/internal/auth"},
				},
				{
					ImportPath: modulePath + "/shortcuts/apps/gitcred",
					Imports:    []string{modulePath + "/internal/keychain"},
				},
			},
			wantFrom:   modulePath + "/shortcuts/im",
			wantDenied: modulePath + "/internal/auth",
		},
		{
			name:     "cmd-direct-denial-and-assembly-exceptions",
			ruleName: "cmd-assembly-only",
			packages: []listedPackage{
				{
					ImportPath: modulePath + "/cmd/service",
					Imports:    []string{modulePath + "/shortcuts/im"},
				},
				{
					ImportPath: modulePath + "/cmd",
					Imports:    []string{modulePath + "/shortcuts"},
				},
				{
					ImportPath: modulePath + "/cmd/auth",
					Imports:    []string{modulePath + "/shortcuts/auth"},
				},
			},
			wantFrom:   modulePath + "/cmd/service",
			wantDenied: modulePath + "/shortcuts/im",
		},
		{
			name:     "errs-direct-denial",
			ruleName: "errs-leaf",
			packages: []listedPackage{
				{
					ImportPath: modulePath + "/errs",
					Imports:    []string{modulePath + "/internal/core"},
				},
				{
					ImportPath: modulePath + "/errclass",
					Imports:    []string{modulePath + "/internal/core"},
				},
			},
			wantFrom:   modulePath + "/errs",
			wantDenied: modulePath + "/internal/core",
		},
		{
			name:     "internal-direct-denial-and-collector-exception",
			ruleName: "internal-no-upper",
			packages: []listedPackage{
				{
					ImportPath: modulePath + "/internal/core",
					Imports:    []string{modulePath + "/events/im"},
				},
				{
					ImportPath: modulePath + "/internal/qualitygate/cmd/manifest-export",
					Imports:    []string{modulePath + "/cmd"},
				},
			},
			wantFrom:   modulePath + "/internal/core",
			wantDenied: modulePath + "/events/im",
		},
	}

	rulesByName := make(map[string]Rule, len(rules))
	for _, rule := range rules {
		rulesByName[rule.Name] = rule
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, ok := rulesByName[tt.ruleName]
			if !ok {
				t.Fatalf("missing rule %q", tt.ruleName)
			}
			violations := evaluateLayeringRule(tt.packages, rule)
			if len(violations) != 1 {
				t.Fatalf("evaluateLayeringRule returned %d violations, want 1: %+v", len(violations), violations)
			}
			if violations[0].From != tt.wantFrom || violations[0].Denied != tt.wantDenied {
				t.Fatalf(
					"evaluateLayeringRule returned edge (%q, %q), want (%q, %q)",
					violations[0].From,
					violations[0].Denied,
					tt.wantFrom,
					tt.wantDenied,
				)
			}
		})
	}
}

func TestLayeringBuildTargets(t *testing.T) {
	want := []goListTarget{
		{GOOS: "linux", GOARCH: "amd64"},
		{GOOS: "linux", GOARCH: "arm64"},
		{GOOS: "linux", GOARCH: "riscv64"},
		{GOOS: "darwin", GOARCH: "amd64"},
		{GOOS: "darwin", GOARCH: "arm64"},
		{GOOS: "windows", GOARCH: "amd64"},
		{GOOS: "windows", GOARCH: "arm64"},
	}
	if !reflect.DeepEqual(layeringBuildTargets, want) {
		t.Fatalf("layering build targets = %#v, want %#v", layeringBuildTargets, want)
	}
}

func TestDecodeAndMergeListedPackages(t *testing.T) {
	input := strings.NewReader(
		`{"ImportPath":"example.com/a","Imports":["example.com/b"],"Deps":["example.com/c"]}` + "\n" +
			`{"ImportPath":"example.com/d","Imports":[],"Deps":[]}` + "\n",
	)
	packages, err := decodeListedPackages(input)
	if err != nil {
		t.Fatalf("decodeListedPackages returned an error: %v", err)
	}
	if len(packages) != 2 || packages[0].ImportPath != "example.com/a" || packages[1].ImportPath != "example.com/d" {
		t.Fatalf("decodeListedPackages returned unexpected packages: %+v", packages)
	}

	got := mergeStrings([]string{"example.com/b", "example.com/c"}, []string{"example.com/a", "example.com/b"})
	want := []string{"example.com/a", "example.com/b", "example.com/c"}
	if !slices.Equal(got, want) {
		t.Fatalf("mergeStrings returned %q, want %q", got, want)
	}
}

func TestGoListPackagesSeparatesStderr(t *testing.T) {
	target := goListTarget{GOOS: "linux", GOARCH: "amd64"}
	packages, stderr, err := loadPackagesForTarget("", target, func(_ string, _ ...string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestGoListCommandHelperProcess$")
		cmd.Env = append(os.Environ(), "GO_LIST_COMMAND_HELPER=1")
		return cmd
	})
	if err != nil {
		t.Fatalf("loadPackagesForTarget returned an error: %v", err)
	}
	if stderr != "go: downloading example.com/module\n" {
		t.Fatalf("loadPackagesForTarget stderr = %q, want module download diagnostic", stderr)
	}
	if len(packages) != 1 || packages[0].ImportPath != "example.com/package" {
		t.Fatalf("loadPackagesForTarget returned unexpected packages: %+v", packages)
	}
}

func TestGoListCommandHelperProcess(t *testing.T) {
	if os.Getenv("GO_LIST_COMMAND_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, `{"ImportPath":"example.com/package","Imports":[],"Deps":[]}`)
	fmt.Fprintln(os.Stderr, "go: downloading example.com/module")
	os.Exit(0)
}

func goListPackageGraph(t *testing.T, root string) []listedPackage {
	t.Helper()
	packagesByPath := make(map[string]listedPackage)
	for _, target := range layeringBuildTargets {
		for _, pkg := range goListPackages(t, root, target) {
			merged := packagesByPath[pkg.ImportPath]
			merged.ImportPath = pkg.ImportPath
			merged.Imports = mergeStrings(merged.Imports, pkg.Imports)
			merged.Deps = mergeStrings(merged.Deps, pkg.Deps)
			packagesByPath[pkg.ImportPath] = merged
		}
	}

	packages := make([]listedPackage, 0, len(packagesByPath))
	for _, pkg := range packagesByPath {
		packages = append(packages, pkg)
	}
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].ImportPath < packages[j].ImportPath
	})
	return packages
}

func goListPackages(t *testing.T, root string, target goListTarget) []listedPackage {
	t.Helper()
	packages, stderr, err := loadPackagesForTarget(root, target, exec.Command)
	if err != nil {
		t.Fatalf(
			"GOOS=%s GOARCH=%s go list -json -tags authsidecar ./... failed: %v\n%s",
			target.GOOS,
			target.GOARCH,
			err,
			stderr,
		)
	}
	return packages
}

func loadPackagesForTarget(
	root string,
	target goListTarget,
	newCommand commandFactory,
) ([]listedPackage, string, error) {
	args := []string{"list", "-json", "-tags", "authsidecar", "./..."}
	cmd := newCommand("go", args...)
	cmd.Dir = root
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = append(
		env,
		"GOOS="+target.GOOS,
		"GOARCH="+target.GOARCH,
		"CGO_ENABLED=0",
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, stderr.String(), err
	}

	packages, err := decodeListedPackages(&stdout)
	if err != nil {
		return nil, stderr.String(), fmt.Errorf("decode go list output: %w", err)
	}
	return packages, stderr.String(), nil
}

func decodeListedPackages(r io.Reader) ([]listedPackage, error) {
	decoder := json.NewDecoder(r)
	var packages []listedPackage
	for {
		var pkg listedPackage
		err := decoder.Decode(&pkg)
		if err == io.EOF {
			return packages, nil
		}
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
}

func mergeStrings(left, right []string) []string {
	values := make(map[string]struct{}, len(left)+len(right))
	for _, value := range left {
		values[value] = struct{}{}
	}
	for _, value := range right {
		values[value] = struct{}{}
	}
	merged := make([]string, 0, len(values))
	for value := range values {
		merged = append(merged, value)
	}
	sort.Strings(merged)
	return merged
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

func findUnseededLayeringViolations(
	actual []layeringViolation,
	seeded map[layeringEdge]seededLayeringEdge,
) []layeringViolation {
	var unseeded []layeringViolation
	for _, violation := range actual {
		if _, ok := seeded[violation.layeringEdge]; !ok {
			unseeded = append(unseeded, violation)
		}
	}
	return unseeded
}

func findStaleLayeringEdges(
	seeded []seededLayeringEdge,
	actual map[layeringEdge]struct{},
) []seededLayeringEdge {
	var stale []seededLayeringEdge
	for _, edge := range seeded {
		if _, ok := actual[edge.layeringEdge]; !ok {
			stale = append(stale, edge)
		}
	}
	return stale
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

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

// Output budgets for the surfaces a caller discovers commands through. Each
// ceiling is the measured maximum plus headroom, so an accidental regression
// (a listing that stops being a listing and starts rendering full contracts)
// fails here instead of silently blowing past a consumer's context.
const (
	maxDomainHelpMedian = 2560  // 2.5 KB — overall health, resistant to outliers
	maxSingleDomainHelp = 12288 // 12 KB — measured max 9,965 B (sheets)
	maxRootHelp         = 4096  // 4 KB — measured 3,673 B
	maxServiceIndex     = 4096  // 4 KB — measured 1,705 B
	maxMethodIndex      = 16384 // 16 KB — measured max 11,284 B (mail)
)

// buildCLI compiles the real binary once per test run. These budgets are about
// what a caller actually receives, so they assert on the rendered output of the
// built binary rather than on a hand-assembled command tree.
func buildCLI(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lark-cli-budget")
	build := exec.Command("go", "build", "-o", path, ".")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the CLI under test: %v\n%s", err, out)
	}
	return path
}

func runCLI(t *testing.T, cli string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(cli, args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	// A non-zero exit with output still tells us the rendered size; only a
	// silent failure is fatal.
	if err := cmd.Run(); err != nil && out.Len() == 0 {
		t.Fatalf("%v: %v\nstderr: %s", args, err, errBuf.String())
	}
	return out.Bytes()
}

var sectionHeadRe = regexp.MustCompile(`^[A-Za-z].*:$`)

// domainNames parses the domain list out of root help. Root help groups commands
// under headed sections ("Lark domains:", "Agent tooling:", "CLI management:"),
// and the quickstart above them also uses indented lines — so the walk has to
// stay inside the domain section rather than collecting every indented pair.
func domainNames(t *testing.T, cli string) []string {
	t.Helper()
	var names []string
	inSection := false
	for _, line := range strings.Split(string(runCLI(t, cli, "--help")), "\n") {
		if strings.HasPrefix(line, "Lark domains:") {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if sectionHeadRe.MatchString(line) {
			break // the next section starts; the domain list is done
		}
		fields := strings.Fields(line)
		if !strings.HasPrefix(line, "  ") || len(fields) < 2 {
			continue
		}
		names = append(names, fields[0])
	}
	if len(names) == 0 {
		t.Fatal("could not parse any domain names from root help")
	}
	return names
}

func TestOutputBudget(t *testing.T) {
	cli := buildCLI(t)
	domains := domainNames(t, cli)

	t.Run("HelpSizes", func(t *testing.T) {
		if root := len(runCLI(t, cli, "--help")); root > maxRootHelp {
			t.Errorf("root help is %d bytes, want <= %d", root, maxRootHelp)
		}
		var sizes []int
		for _, d := range domains {
			n := len(runCLI(t, cli, d, "--help"))
			if n > maxSingleDomainHelp {
				t.Errorf("%s help is %d bytes, want <= %d", d, n, maxSingleDomainHelp)
			}
			sizes = append(sizes, n)
		}
		sort.Ints(sizes)
		if median := sizes[len(sizes)/2]; median > maxDomainHelpMedian {
			t.Errorf("domain help median is %d bytes, want <= %d", median, maxDomainHelpMedian)
		}
	})

	t.Run("SchemaSizes", func(t *testing.T) {
		index := runCLI(t, cli, "schema")
		if n := len(index); n > maxServiceIndex {
			t.Errorf("service_index is %d bytes, want <= %d", n, maxServiceIndex)
		}
		// The service index is the authoritative list of services that have a
		// Meta API. Root help is a wider set — it also lists shortcut-only
		// domains, which `schema <domain>` rejects by design — so driving this
		// loop from the index asserts the right set instead of skipping failures.
		var parsed struct {
			Services []struct {
				Name string `json:"name"`
			} `json:"services"`
		}
		if err := json.Unmarshal(index, &parsed); err != nil {
			t.Fatalf("service_index is not valid JSON: %v", err)
		}
		if len(parsed.Services) == 0 {
			t.Fatal("service_index listed no services")
		}
		for _, svc := range parsed.Services {
			out := runCLI(t, cli, "schema", svc.Name)
			if !bytes.Contains(out, []byte(`"method_index"`)) {
				t.Errorf("schema %s did not render a method_index", svc.Name)
				continue
			}
			if len(out) > maxMethodIndex {
				t.Errorf("method_index for %s is %d bytes, want <= %d", svc.Name, len(out), maxMethodIndex)
			}
		}
	})

	// ListingLineWidths reports the display-width distribution of domain listing
	// lines (flattened method rows included). It asserts nothing: no truncation
	// rule is in force, and these numbers are what a truncation decision would
	// have to be based on.
	t.Run("ListingLineWidths", func(t *testing.T) {
		var widths []int
		for _, d := range domains {
			for _, line := range strings.Split(string(runCLI(t, cli, d, "--help")), "\n") {
				if !strings.HasPrefix(line, "  ") || strings.TrimSpace(line) == "" {
					continue
				}
				// runewidth, not len(): CJK descriptions occupy two columns each.
				widths = append(widths, runewidth.StringWidth(strings.TrimRight(line, " ")))
			}
		}
		if len(widths) == 0 {
			t.Fatal("no listing lines collected")
		}
		sort.Ints(widths)
		t.Logf("listing line display width: n=%d p50=%d p90=%d max=%d",
			len(widths), widths[len(widths)/2], widths[len(widths)*9/10], widths[len(widths)-1])
	})
}

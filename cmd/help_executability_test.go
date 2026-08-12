// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/larksuite/cli/cmd/service"
	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/registry"
	"github.com/spf13/cobra"
)

// A discovery surface that names a command the tree cannot run is worse than one
// that names nothing: the reader (an agent especially) copies the row verbatim
// and spends its turns on a name that never existed. The tests here hold the one
// invariant that prevents the whole class — everything domain help lists must
// run, and a name that does not must say so — over the real command tree, so a
// newly onboarded domain or a newly nested resource is covered without anyone
// remembering to extend a fixture.

// buildExecutabilityTree assembles the full command tree over the embedded
// catalog, with the guards and help func the real binary installs.
func buildExecutabilityTree(t *testing.T) *cobra.Command {
	t.Helper()
	f, _, _ := newStrictModeDefaultFactory(t, "default", core.StrictModeOff)
	snapshot, err := registry.OpenSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := snapshot.FullCatalog()
	if err != nil {
		t.Fatal(err)
	}
	f.APICatalog = catalog
	root := buildIntegrationRootCmd(t, f)
	installUnknownSubcommandGuard(root)
	installTipsHelpFunc(root, catalog, nil, nil, nil)
	return root
}

// domainAPIListings renders every domain's help and returns the method paths each
// one lists, keyed by domain name. Domains with no Meta API surface are omitted.
func domainAPIListings(t *testing.T, root *cobra.Command) map[string][]string {
	t.Helper()
	listings := map[string][]string{}
	for _, domain := range root.Commands() {
		if !service.PrepareDomainHelp(domain, nil) {
			continue
		}
		if !strings.Contains(domain.Long, "API methods (") {
			continue // a shortcut-only domain
		}
		if paths := listedAPIMethodPaths(t, domain.Long); len(paths) > 0 {
			listings[domain.Name()] = paths
		}
	}
	if len(listings) == 0 {
		t.Fatal("no domain listed any API method, so this fixture cannot detect a regression")
	}
	return listings
}

func TestDomainHelpListsOnlyRunnableMethods(t *testing.T) {
	root := buildExecutabilityTree(t)
	for domain, paths := range domainAPIListings(t, root) {
		for _, path := range paths {
			args := append([]string{domain}, strings.Fields(path)...)
			// cobra's Find returns the deepest command it matched plus the
			// leftovers, so resolving without error is not enough: a listing that
			// rendered a non-executable name resolves to the domain itself. The
			// method-schema-path annotation is what makes a command a method leaf.
			target, rest, err := root.Find(args)
			if err != nil {
				t.Errorf("%s help lists %q, which does not resolve: %v", domain, path, err)
				continue
			}
			if len(rest) > 0 {
				t.Errorf("%s help lists %q, but %q is left unresolved — the listed name is not runnable as written",
					domain, path, strings.Join(rest, " "))
				continue
			}
			if target.Annotations["method-schema-path"] == "" {
				t.Errorf("%s help lists %q, which resolves to %q — not a method leaf",
					domain, path, target.CommandPath())
			}
		}
	}
}

func TestDottedMethodNameFailsStructuredOnHelpPath(t *testing.T) {
	root := buildExecutabilityTree(t)
	for domain, paths := range domainAPIListings(t, root) {
		for _, path := range paths {
			dotted := strings.ReplaceAll(path, " ", ".")
			if dotted == path {
				continue // a method directly under the domain has no dotted form
			}
			// The pre-fix behaviour: cobra's ErrHelp path bypassed RunE, so this
			// invocation printed the domain's own help and exited 0, leaving the
			// caller no signal that the name was wrong — while the same name
			// without --help failed structured. Both paths must reject it.
			err := runHelpPath(t, root, domain, dotted)
			if err == nil {
				t.Errorf("`%s %s --help` was accepted; a dotted method name must be rejected on the help path too",
					domain, dotted)
				continue
			}
			assertRejectsWithRewrite(t, err, dotted, path)
		}
	}
}

func TestDottedMethodNameFailsStructuredOnRunPath(t *testing.T) {
	root := buildExecutabilityTree(t)
	for domain, paths := range domainAPIListings(t, root) {
		for _, path := range paths {
			dotted := strings.ReplaceAll(path, " ", ".")
			if dotted == path {
				continue
			}
			domainCmd, _, err := root.Find([]string{domain})
			if err != nil {
				t.Fatalf("domain %q does not resolve: %v", domain, err)
			}
			assertRejectsWithRewrite(t, unknownSubcommandError(domainCmd, dotted), dotted, path)
		}
	}
}

// runHelpPath executes `<domain> <name> --help` against the tree and returns the
// rejection the help func recorded, or nil when help rendered.
func runHelpPath(t *testing.T, root *cobra.Command, domain, name string) error {
	t.Helper()
	pendingHelpRejection = nil
	t.Cleanup(func() { pendingHelpRejection = nil })
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{domain, name, "--help"})
	if err := root.Execute(); err != nil {
		return err
	}
	return pendingHelpRejection
}

// assertRejectsWithRewrite checks the rejection is the typed invalid_argument a
// caller can act on: exit 2, and a machine-readable suggestion holding the
// runnable form rather than prose the caller has to parse.
func assertRejectsWithRewrite(t *testing.T, err error, dotted, want string) {
	t.Helper()
	if code := output.ExitCodeOf(err); code != 2 {
		t.Errorf("rejecting %q: exit code = %d, want 2 (%v)", dotted, code, err)
	}
	var verr *errs.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("rejecting %q: error = %v, want a *errs.ValidationError", dotted, err)
	}
	if verr.Subtype != errs.SubtypeInvalidArgument {
		t.Errorf("rejecting %q: subtype = %q, want %q", dotted, verr.Subtype, errs.SubtypeInvalidArgument)
	}
	var suggested []string
	for _, p := range verr.Params {
		if p.Name == dotted {
			suggested = p.Suggestions
		}
	}
	if len(suggested) == 0 {
		t.Errorf("rejecting %q: no suggestion offered; the caller has nothing to retry with", dotted)
		return
	}
	// The rewrite is certain (the tree holds this exact path), so it must be the
	// single suggestion, not one guess among ranked neighbours.
	if len(suggested) != 1 || suggested[0] != want {
		t.Errorf("rejecting %q: suggestions = %v, want exactly [%q]", dotted, suggested, want)
	}
}

// TestSchemaMethodNameIsRunnable pins the two discovery surfaces to each other:
// `schema <dotted-path>` reports a method's name, and that name must be the exact
// argv the command tree accepts. Both derive from the same catalog, so a drift
// here means one of them started composing the path its own way.
func TestSchemaMethodNameIsRunnable(t *testing.T) {
	root := buildExecutabilityTree(t)
	cli := buildCLI(t)
	for domain, paths := range domainAPIListings(t, root) {
		// One method per domain: each case is a process launch, and the property
		// under test is per-surface, not per-method.
		path := paths[0]
		schemaPath := domain + "." + strings.ReplaceAll(path, " ", ".")
		out := runCLI(t, cli, "schema", schemaPath)
		var env struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(out, &env); err != nil {
			t.Errorf("schema %s: output is not JSON: %v\n%s", schemaPath, err, out)
			continue
		}
		want := domain + " " + path
		if env.Name != want {
			t.Errorf("schema %s reports name %q, but the command tree accepts %q", schemaPath, env.Name, want)
		}
	}
}

// TestHelpPathRejectionExitsTwo runs the built binary so the wiring from the help
// func's recorded rejection through Execute to the process exit code is covered
// end to end, not just the recording of it.
func TestHelpPathRejectionExitsTwo(t *testing.T) {
	cli := buildCLI(t)
	root := buildExecutabilityTree(t)
	listings := domainAPIListings(t, root)

	var domain, dotted, runnable string
	for d, paths := range listings {
		for _, p := range paths {
			if strings.Contains(p, " ") {
				domain, dotted, runnable = d, strings.ReplaceAll(p, " ", "."), p
				break
			}
		}
		if dotted != "" {
			break
		}
	}
	if dotted == "" {
		t.Fatal("no domain exposes a method under a resource group, so this fixture cannot exercise the help path")
	}

	cmd := exec.Command(cli, domain, dotted, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("`lark-cli %s %s --help` exited 0; it must fail structured\nstdout: %s", domain, dotted, stdout.String())
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("running the CLI: %v", err)
	}
	if code := exitErr.ExitCode(); code != 2 {
		t.Errorf("`lark-cli %s %s --help` exit code = %d, want 2\nstderr: %s", domain, dotted, code, stderr.String())
	}
	var envelope struct {
		OK    bool `json:"ok"`
		Error struct {
			Subtype string `json:"subtype"`
			Hint    string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not a typed envelope: %v\n%s", err, stderr.String())
	}
	if envelope.OK || envelope.Error.Subtype != string(errs.SubtypeInvalidArgument) {
		t.Errorf("envelope = %+v, want ok=false subtype=%s", envelope, errs.SubtypeInvalidArgument)
	}
	// The hint has to carry the runnable form verbatim; sending the caller back to
	// --help is what made the original silence self-sustaining. Note the runnable
	// form is not the dotted name with its dots swapped for spaces — a resource
	// segment keeps its own dots — so the listing's form is the expectation.
	if !strings.Contains(envelope.Error.Hint, runnable) {
		t.Errorf("hint must name the runnable form %q, got %q", runnable, envelope.Error.Hint)
	}
}

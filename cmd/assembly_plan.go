// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"

	"github.com/larksuite/cli/internal/core"
)

// AssemblySelection describes how much of the command tree must be assembled.
type AssemblySelection string

const (
	AssemblyNone AssemblySelection = "none"
	// AssemblyIndex names every service without reading a single shard. It
	// serves the bare `schema`, whose service index is built from the manifest
	// names and the embedded descriptions alone.
	AssemblyIndex  AssemblySelection = "index"
	AssemblyTarget AssemblySelection = "target"
	AssemblyFull   AssemblySelection = "full"
)

// AssemblyPlan is the catalog and Shortcut subset needed for one invocation.
type AssemblyPlan struct {
	Mode            AssemblySelection
	CatalogServices []string
	ShortcutDomains []string
}

// PlanAssembly conservatively determines which command domains an argv can
// reach. It only recognizes root-global flags and top-level command positions;
// Cobra remains the authority for parsing and validating the actual command.
//
// strictMode resolves the active identity restriction. It is a getter rather
// than a value because only one branch consults it, so an invocation that never
// asks never pays for the credential lookup. A nil getter means the caller
// cannot answer yet, and every branch that would consult it falls back to the
// complete tree. The decision is made once — a branch never retries as Full
// after choosing a narrower selection.
//
// pluginRestricts reports whether any installed plugin declares it will register
// a command restriction. Identity is not the only channel that hides commands,
// and the two must be gated alike: a restriction conceals its denied paths, but
// concealment is recorded only against commands present in the tree, so a
// selection that registers no service commands cannot observe it. Same getter
// and nil semantics as strictMode.
func PlanAssembly(
	argv []string,
	catalogServices []string,
	shortcutDomains []string,
	strictMode func() core.StrictMode,
	pluginRestricts func() bool,
) AssemblyPlan {
	catalog := nameSet(catalogServices)
	shortcuts := nameSet(shortcutDomains)
	longGlobals, shortGlobals := registeredGlobalFlagArities()
	sawVersion := false

	for i := 0; i < len(argv); i++ {
		token := argv[i]
		// An explicit separator (`lark-cli -- drive`) or malformed empty
		// token leaves the top-level command position ambiguous, so let the
		// complete Cobra tree diagnose it.
		if token == "" || token == "--" {
			return fullAssemblyPlan()
		}

		// `lark-cli --version` and `lark-cli -v` are the only invocations
		// whose answer comes entirely from the root command.
		if token == "--version" || token == "-v" {
			sawVersion = true
			continue
		}

		if strings.HasPrefix(token, "--") {
			name, hasValue := splitLongFlag(token)
			requiresValue, known := longGlobals[name]
			// Root help (`--help`) is not part of registeredGlobalFlagArities,
			// so it reaches this branch and requests the full tree. Unknown
			// flags such as `--tenant` do the same, allowing Cobra to produce
			// its normal error against the complete command surface.
			if !known {
				return fullAssemblyPlan()
			}
			if hasValue {
				if requiresValue && strings.HasSuffix(token, "=") {
					return fullAssemblyPlan()
				}
				continue
			}
			if requiresValue {
				if i+1 >= len(argv) || argv[i+1] == "" || strings.HasPrefix(argv[i+1], "-") {
					return fullAssemblyPlan()
				}
				i++
			}
			continue
		}

		if strings.HasPrefix(token, "-") {
			// Short clusters and unknown shorthands (for example `-vh` or
			// `-x`) are deliberately left to the complete Cobra tree.
			if len(token) != 2 {
				return fullAssemblyPlan()
			}
			requiresValue, known := shortGlobals[token[1:]]
			if !known {
				return fullAssemblyPlan()
			}
			if requiresValue {
				if i+1 >= len(argv) || argv[i+1] == "" || strings.HasPrefix(argv[i+1], "-") {
					return fullAssemblyPlan()
				}
				i++
			}
			continue
		}

		// A command combined with version (`--version drive`) is not the
		// deterministic root-only version case; Cobra must validate it.
		if sawVersion {
			return fullAssemblyPlan()
		}
		if token == "schema" {
			return planSchema(argv[i+1:], catalog, strictMode, pluginRestricts)
		}
		// The first positional token in ordinary invocations is the domain,
		// as in `drive files list` or `docs +fetch`.
		return planDomain(token, catalog, shortcuts)
	}

	if sawVersion {
		return AssemblyPlan{Mode: AssemblyNone}
	}
	// Bare `lark-cli` and root help (`lark-cli --help`) enumerate the
	// command surface and therefore require the complete tree.
	return fullAssemblyPlan()
}

func splitLongFlag(token string) (name string, hasValue bool) {
	name = strings.TrimPrefix(token, "--")
	if before, _, ok := strings.Cut(name, "="); ok {
		return before, true
	}
	return name, false
}

func planSchema(
	argv []string,
	catalog map[string]struct{},
	strictMode func() core.StrictMode,
	pluginRestricts func() bool,
) AssemblyPlan {
	// The bare form renders the service index, which needs service names and
	// their embedded descriptions but no shard body. That holds only while both
	// channels that hide commands are known to be inactive.
	//
	// Strict mode: an active mode drops services whose every method the identity
	// cannot reach, and deciding that reads each shard.
	//
	// Plugin restriction: the index registers no service commands, and a
	// concealment is recorded only for a command found in the tree. With nothing
	// registered the concealment is never recorded, the surface plan defaults to
	// available, and the listing would name services the same build hides from
	// root help — the one outcome the renderer's own invariant forbids.
	if len(argv) == 0 {
		if strictModeIsOff(strictMode) && noPluginRestricts(pluginRestricts) {
			return AssemblyPlan{
				Mode:            AssemblyIndex,
				CatalogServices: []string{},
				ShortcutDomains: []string{},
			}
		}
		return fullAssemblyPlan()
	}
	// Flag-first forms (`schema --format json`) reach the same listing, but
	// recognizing them means parsing schema's own flags. The planner stays out
	// of local flag semantics and leaves them to the complete tree.
	if argv[0] == "" || strings.HasPrefix(argv[0], "-") {
		return fullAssemblyPlan()
	}
	parts := strings.Split(argv[0], ".")
	for _, part := range parts {
		// Empty components (`schema drive..list`) are invalid but remain
		// Cobra/schema concerns, so retain the full tree for diagnostics.
		if part == "" {
			return fullAssemblyPlan()
		}
	}
	service := parts[0]
	// An unknown service must see the full catalog so the schema command can
	// report the authoritative error. This is also what routes a pure-Shortcut
	// domain such as `schema docs` to the full tree, where the resolve failure
	// can name the real alternatives.
	if _, ok := catalog[service]; !ok {
		return fullAssemblyPlan()
	}
	// Every resolvable target names its service first, so the leading shard is
	// enough for all of them: `schema drive`, `schema drive.file`, and
	// `schema drive.file.comments.list` alike. The layered output makes the
	// first two the ordinary way to browse rather than a curiosity, so routing
	// them to the full catalog would forfeit the saving on the common path.
	return AssemblyPlan{
		Mode:            AssemblyTarget,
		CatalogServices: []string{service},
		ShortcutDomains: []string{},
	}
}

func planDomain(domain string, catalog, shortcuts map[string]struct{}) AssemblyPlan {
	plan := AssemblyPlan{Mode: AssemblyTarget, ShortcutDomains: []string{}}
	// Known generated domains (`drive files list`) select their Catalog
	// service, while known Shortcut domains (`docs +fetch`) select their
	// Shortcut registrations. A domain may legitimately select both.
	if _, ok := catalog[domain]; ok {
		plan.CatalogServices = []string{domain}
	}
	if _, ok := shortcuts[domain]; ok {
		plan.ShortcutDomains = []string{domain}
	}
	if len(plan.CatalogServices) == 0 && len(plan.ShortcutDomains) == 0 {
		// Unknown or hand-authored roots (`driev`, `auth`, `completion`)
		// need the full tree for dispatch, suggestions, and help.
		return fullAssemblyPlan()
	}
	return plan
}

func nameSet(names []string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	return set
}

func fullAssemblyPlan() AssemblyPlan {
	return AssemblyPlan{Mode: AssemblyFull}
}

// strictModeIsOff reports whether the identity restriction is known to be
// inactive. An absent getter is not an answer, so it reads as "not known to be
// off" and the caller keeps the complete tree.
func strictModeIsOff(resolve func() core.StrictMode) bool {
	if resolve == nil {
		return false
	}
	return !resolve().IsActive()
}

// noPluginRestricts reports whether the build is known to carry no plugin
// command restriction. An absent getter is not an answer, so it reads as "not
// known to be free of them" and the caller keeps the complete tree.
func noPluginRestricts(resolve func() bool) bool {
	if resolve == nil {
		return false
	}
	return !resolve()
}

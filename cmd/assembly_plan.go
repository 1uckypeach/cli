// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import "strings"

// AssemblySelection describes how much of the command tree must be assembled.
type AssemblySelection string

const (
	AssemblyNone   AssemblySelection = "none"
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
func PlanAssembly(
	argv []string,
	catalogServices []string,
	shortcutDomains []string,
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
			return planSchema(argv[i+1:], catalog)
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

func planSchema(argv []string, catalog map[string]struct{}) AssemblyPlan {
	// Bare schema, schema help, and flag-first forms do not identify one
	// service shard (for example `schema` or `schema --format json`).
	if len(argv) == 0 || argv[0] == "" || strings.HasPrefix(argv[0], "-") {
		return fullAssemblyPlan()
	}
	parts := strings.Split(argv[0], ".")
	// Only a precise dotted method such as
	// `schema drive.file.comments.list` can select a shard. Broad forms
	// such as `schema drive` need the complete catalog.
	if len(parts) < 3 {
		return fullAssemblyPlan()
	}
	for _, part := range parts {
		// Empty components (`schema drive..list`) are invalid but remain
		// Cobra/schema concerns, so retain the full tree for diagnostics.
		if part == "" {
			return fullAssemblyPlan()
		}
	}
	service := parts[0]
	// An unknown service in an otherwise dotted path must see the full
	// catalog so the schema command can report the authoritative error.
	if _, ok := catalog[service]; !ok {
		return fullAssemblyPlan()
	}
	// A valid precise method only needs its leading service shard.
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

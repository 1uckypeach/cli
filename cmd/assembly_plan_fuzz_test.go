// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"
	"testing"
)

func FuzzPlanAssemblyNeverPanics(f *testing.F) {
	services := []string{"calendar", "drive", "im"}
	shortcuts := []string{"docs", "drive", "im"}
	for _, seed := range []string{
		"",
		"drive files list",
		"--profile prod drive",
		"--unknown drive",
		"-- drive",
		"-vh drive",
		"schema drive.file.comments.list",
		"\x00 drive",
		"驱动 --配置 drive",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		plan := PlanAssembly(strings.Fields(raw), services, shortcuts)
		switch plan.Mode {
		case AssemblyNone, AssemblyTarget, AssemblyFull:
		default:
			t.Fatalf("PlanAssembly(%q) returned invalid mode %q", raw, plan.Mode)
		}
		if plan.Mode == AssemblyTarget && len(plan.CatalogServices)+len(plan.ShortcutDomains) == 0 {
			t.Fatalf("PlanAssembly(%q) returned empty Target plan", raw)
		}
		for _, service := range plan.CatalogServices {
			if !containsName(services, service) {
				t.Fatalf("PlanAssembly(%q) selected unknown catalog service %q", raw, service)
			}
		}
		for _, domain := range plan.ShortcutDomains {
			if !containsName(shortcuts, domain) {
				t.Fatalf("PlanAssembly(%q) selected unknown Shortcut domain %q", raw, domain)
			}
		}
	})
}

func FuzzPlanAssemblyUnknownFlagValueIsNotDomain(f *testing.F) {
	for _, seed := range []string{"tenant", "profiled", "配置", "\x00", "x=y"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, flagName string) {
		if flagName == "" || flagName == "profile" || strings.ContainsAny(flagName, "=\t\r\n ") {
			t.Skip()
		}
		plan := PlanAssembly([]string{"--" + flagName, "drive"}, []string{"drive"}, []string{"drive"})
		if plan.Mode != AssemblyFull {
			t.Fatalf("unknown flag %q produced %#v, want Full", flagName, plan)
		}
	})
}

func containsName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

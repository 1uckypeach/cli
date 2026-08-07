// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"reflect"
	"testing"
)

func TestPlanAssembly(t *testing.T) {
	t.Parallel()

	services := []string{"approval", "calendar", "drive", "im"}
	shortcuts := []string{"docs", "drive", "im"}

	tests := []struct {
		name string
		argv []string
		want AssemblyPlan
	}{
		{name: "long version", argv: []string{"--version"}, want: AssemblyPlan{Mode: AssemblyNone}},
		{name: "short version", argv: []string{"-v"}, want: AssemblyPlan{Mode: AssemblyNone}},
		{name: "version after global", argv: []string{"--profile", "prod", "--version"}, want: AssemblyPlan{Mode: AssemblyNone}},
		{name: "version before global", argv: []string{"-v", "--profile=prod"}, want: AssemblyPlan{Mode: AssemblyNone}},
		{name: "repeated version", argv: []string{"-v", "--version"}, want: AssemblyPlan{Mode: AssemblyNone}},
		{
			name: "catalog domain",
			argv: []string{"drive", "files", "list"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"drive"}, ShortcutDomains: []string{"drive"}},
		},
		{
			name: "domain help",
			argv: []string{"drive", "--help"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"drive"}, ShortcutDomains: []string{"drive"}},
		},
		{
			name: "pure shortcut domain",
			argv: []string{"docs", "+fetch"},
			want: AssemblyPlan{Mode: AssemblyTarget, ShortcutDomains: []string{"docs"}},
		},
		{
			name: "catalog and shortcut domain",
			argv: []string{"drive", "+search"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"drive"}, ShortcutDomains: []string{"drive"}},
		},
		{
			name: "schema method",
			argv: []string{"schema", "drive.file.comments.list"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"drive"}, ShortcutDomains: []string{}},
		},
		// The layered output makes the service and resource forms the ordinary
		// way to browse, and each still resolves inside one shard.
		{
			name: "schema service",
			argv: []string{"schema", "drive"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"drive"}, ShortcutDomains: []string{}},
		},
		{
			name: "schema resource",
			argv: []string{"schema", "drive.file"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"drive"}, ShortcutDomains: []string{}},
		},
		{
			name: "schema spaced path",
			argv: []string{"schema", "drive", "file", "list"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"drive"}, ShortcutDomains: []string{}},
		},
		{
			name: "catalog-only domain",
			argv: []string{"approval", "instances", "list"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"approval"}, ShortcutDomains: []string{}},
		},
		{
			name: "global separated before domain",
			argv: []string{"--profile", "prod", "im", "messages", "list"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"im"}, ShortcutDomains: []string{"im"}},
		},
		{
			name: "global equals before domain",
			argv: []string{"--profile=prod", "calendar", "--help"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"calendar"}, ShortcutDomains: []string{}},
		},
		{
			name: "repeated globals",
			argv: []string{"--profile=old", "--profile", "prod", "drive"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"drive"}, ShortcutDomains: []string{"drive"}},
		},
		{
			name: "global after domain is local parsing concern",
			argv: []string{"drive", "--profile", "prod"},
			want: AssemblyPlan{Mode: AssemblyTarget, CatalogServices: []string{"drive"}, ShortcutDomains: []string{"drive"}},
		},
		{name: "bare root", argv: nil, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "root help long", argv: []string{"--help"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "root help short", argv: []string{"-h"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "bare schema", argv: []string{"schema"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "schema unknown service", argv: []string{"schema", "unknown"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "schema unknown service", argv: []string{"schema", "unknown.file.list"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "schema empty segment", argv: []string{"schema", "drive..list"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "schema flag before path", argv: []string{"schema", "--format", "json", "drive.file.list"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "completion", argv: []string{"completion", "bash"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "handwritten auth", argv: []string{"auth", "login"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "handwritten config", argv: []string{"config", "show"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "handwritten profile", argv: []string{"profile", "list"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "handwritten api", argv: []string{"api", "get"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "unknown command", argv: []string{"driev", "files", "list"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "unknown long flag", argv: []string{"--tenant", "drive"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "unknown long flag equals", argv: []string{"--tenant=drive"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "missing global value", argv: []string{"--profile"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "empty global value", argv: []string{"--profile=", "drive"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "next flag is not a global value", argv: []string{"--profile", "--version"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "global value is never domain", argv: []string{"--profile", "drive"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "double dash", argv: []string{"--", "drive"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "short cluster", argv: []string{"-vh", "drive"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "unknown short flag", argv: []string{"-x", "drive"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "version plus command", argv: []string{"--version", "drive"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "empty token", argv: []string{"", "drive"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "unicode unknown command", argv: []string{"驱动", "list"}, want: AssemblyPlan{Mode: AssemblyFull}},
		{name: "unicode unknown flag", argv: []string{"--配置", "drive"}, want: AssemblyPlan{Mode: AssemblyFull}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := PlanAssembly(tt.argv, services, shortcuts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("PlanAssembly(%q) = %#v, want %#v", tt.argv, got, tt.want)
			}
		})
	}
}

func TestPlanAssemblyDoesNotInterpretLocalFlags(t *testing.T) {
	t.Parallel()

	want := AssemblyPlan{
		Mode:            AssemblyTarget,
		CatalogServices: []string{"drive"},
		ShortcutDomains: []string{"drive"},
	}
	for _, argv := range [][]string{
		{"drive", "--unknown", "calendar"},
		{"drive", "--", "calendar"},
		{"drive", "-xyz", "calendar"},
		{"drive", ""},
	} {
		if got := PlanAssembly(argv, []string{"calendar", "drive"}, []string{"drive"}); !reflect.DeepEqual(got, want) {
			t.Errorf("PlanAssembly(%q) = %#v, want %#v", argv, got, want)
		}
	}
}

func TestRegisteredGlobalFlagAritiesMatchRegistration(t *testing.T) {
	t.Parallel()

	long, short := registeredGlobalFlagArities()
	if requiresValue, ok := long["profile"]; !ok || !requiresValue {
		t.Fatalf("long[profile] = (%v, %v), want (true, true)", requiresValue, ok)
	}
	if len(short) != 0 {
		t.Fatalf("short = %v, want no registered shorthand flags", short)
	}
}

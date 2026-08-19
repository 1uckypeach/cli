// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"errors"
	"testing"

	"github.com/larksuite/cli/internal/core"
)

type staticWorkspaceConfig struct {
	config *core.MultiAppConfig
	err    error
}

func (s staticWorkspaceConfig) MultiAppConfig() (*core.MultiAppConfig, error) {
	return s.config, s.err
}

func TestResolveSDKHostSignalSource(t *testing.T) {
	disabled := false
	enabled := true
	tests := []struct {
		name       string
		config     workspaceConfigSource
		wantSource bool
	}{
		{name: "workspace default on", config: staticWorkspaceConfig{config: &core.MultiAppConfig{}}, wantSource: true},
		{name: "workspace explicit on", config: staticWorkspaceConfig{config: &core.MultiAppConfig{RiskControl: &enabled}}, wantSource: true},
		{name: "workspace opt-out", config: staticWorkspaceConfig{config: &core.MultiAppConfig{RiskControl: &disabled}}},
		// A config that cannot be loaded exposes no opt-out to honor, so
		// collection defaults on instead of failing closed.
		{name: "missing config", config: staticWorkspaceConfig{err: errors.New("file does not exist")}, wantSource: true},
		{name: "unreadable config", config: staticWorkspaceConfig{err: errors.New("permission denied")}, wantSource: true},
		// A nil config value without an error carries no opt-out but also no
		// usable policy; keep suppressing rather than inventing a policy.
		{name: "nil config value", config: staticWorkspaceConfig{}},
		{name: "nil config source"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resolveSDKHostSignalSource(test.config)
			if (got != nil) != test.wantSource {
				t.Fatalf("resolveSDKHostSignalSource() = %T, wantSource %t", got, test.wantSource)
			}
		})
	}
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/riskcontrol"
)

type workspaceConfigSource interface {
	MultiAppConfig() (*core.MultiAppConfig, error)
}

// resolveSDKHostSignalSource applies workspace policy at the SDK transport
// boundary. Account protection is default-on: host-signal collection is
// suppressed only when a readable config carries an explicit opt-out
// (risk-control off). A config that is missing, unreadable, or malformed
// exposes no opt-out to honor, so collection defaults on rather than failing
// closed — otherwise env-only sandboxes without a config.json would silently
// drop every risk-control signal.
func resolveSDKHostSignalSource(config workspaceConfigSource) riskcontrol.Source {
	if config == nil {
		return nil
	}
	workspace, configErr := config.MultiAppConfig()
	// Only an explicit opt-out in a successfully loaded config suppresses
	// collection. A load failure falls through to default-on.
	if configErr == nil && !workspace.RiskControlEnabled() {
		return nil
	}
	return riskcontrol.NewHostSource()
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"fmt"

	"github.com/larksuite/cli/internal/core"
	"github.com/spf13/cobra"
)

const riskLevelAnnotationKey = "risk_level"

// Risk level constants — aliases of the canonical core.Risk* values, re-exported
// here so command code gets the risk vocabulary and the SetRisk/GetRisk helpers
// from one package. core is the single source of truth.
const (
	RiskRead          = core.RiskRead
	RiskWrite         = core.RiskWrite
	RiskHighRiskWrite = core.RiskHighRiskWrite
)

// SetRisk stores a command's static risk level on cobra annotations so the
// help renderer (cmd/root.go) can surface a Risk: line without importing
// shortcuts/common. Levels follow the three-tier convention: RiskRead |
// RiskWrite | RiskHighRiskWrite. Framework-level confirmation gating only
// acts on RiskHighRiskWrite.
func SetRisk(cmd *cobra.Command, level string) {
	if level == "" {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[riskLevelAnnotationKey] = level
}

// GetRisk returns the static risk level. ok is true when the command has a
// risk annotation.
func GetRisk(cmd *cobra.Command) (level string, ok bool) {
	if cmd.Annotations == nil {
		return "", false
	}
	level, ok = cmd.Annotations[riskLevelAnnotationKey]
	return level, ok && level != ""
}

// RiskLine renders the "Risk: <level>" line shown in help. ok is false when the
// command carries no risk annotation.
//
// The confirmation warning only applies when the command actually gates on
// --yes: the sentence asserts that passing --yes means the USER confirmed, and
// that assertion is only true where the flag exists and RunE checks it. Some
// commands carry the high-risk-write annotation for documentation purposes
// without wiring a --yes gate (e.g. `update`, which has no confirmation step at
// all); warning those callers about --yes would describe a flag that doesn't
// exist and a gate that isn't there. So the guardrail sentence is conditioned on
// cmd.Flags().Lookup("yes") != nil — a command without that flag gets the bare
// "Risk: <level>" line instead.
//
// The returned line has no surrounding whitespace; callers add their own
// separators.
func RiskLine(cmd *cobra.Command) (line string, ok bool) {
	level, ok := GetRisk(cmd)
	if !ok {
		return "", false
	}
	if level == RiskHighRiskWrite && cmd.Flags().Lookup("yes") != nil {
		return fmt.Sprintf("Risk: %s (requires explicit user confirmation to execute; the agent must NOT add --yes on its own — only pass --yes after the user has confirmed)", level), true
	}
	return fmt.Sprintf("Risk: %s", level), true
}

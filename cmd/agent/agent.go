// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/spf13/cobra"

	// example is the in-repo onboarding template and offline demo provider
	// (in-memory mock, zero network). To add a new provider, add one matching
	// blank import here.
	_ "github.com/larksuite/cli/internal/agent/example" // registers the "example" demo/template provider scheme via init()

	"github.com/larksuite/cli/internal/cmdutil"
)

// NewCmdAgent builds the `agent` command group: a provider-agnostic surface
// that drives remote A2A agents with constant verbs. It is a pure group with
// no RunE, so an unknown subcommand is reported rather than silently
// swallowed. All five verbs (list/card/send/task/context) are wired here; task
// and context are themselves nested groups.
func NewCmdAgent(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Drive first-party remote agents (A2A: send / start task / poll / fetch result)",
		Long:  "Drive Feishu first-party remote agents with a constant verb set. An agent_ref looks like <scheme>:<agent_id> (e.g. example:echo). Read capabilities with `agent card <agent_ref>` first, then pick verbs by capability.",
	}
	cmd.AddCommand(NewCmdAgentList(f))
	cmd.AddCommand(NewCmdAgentCard(f))
	cmd.AddCommand(NewCmdAgentSend(f, nil))
	cmd.AddCommand(NewCmdAgentTask(f))
	cmd.AddCommand(NewCmdAgentContext(f))
	return cmd
}

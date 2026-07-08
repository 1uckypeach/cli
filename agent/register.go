// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package agent is the top-level business layer that wires the in-repo agent
// providers into the framework registry (internal/agent). It mirrors the events
// layering: the framework/SPI lives in internal/agent, each concrete provider is
// a declarative agent.Provider value exposed by a package under agent/<scheme>/,
// and this package's init aggregates and registers them. Blank-import this
// package from cmd to populate the provider registry.
//
// To onboard a new provider: add agent/<scheme>/ exposing a Provider() value,
// then add one line to the slice below.
package agent

import (
	"github.com/larksuite/cli/agent/example"
	iagent "github.com/larksuite/cli/internal/agent"
)

func init() {
	for _, p := range []iagent.Provider{
		example.Provider(),
	} {
		iagent.Register(p)
	}
}

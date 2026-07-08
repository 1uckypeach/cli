// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package agent is the top-level business layer that wires the in-repo agent
// providers into the framework registry (internal/agent). It mirrors the events
// layering: the framework/SPI lives in internal/agent, the concrete providers
// live under agent/<scheme>/, and this package blank-imports each so their
// init() self-registration runs. Blank-import this package from cmd to populate
// the provider registry.
//
// To onboard a new provider: add its package under agent/<scheme>/ and add one
// matching blank import below.
package agent

import (
	// example is the in-repo onboarding template and offline demo provider
	// (in-memory mock, zero network); its init() registers the "example" scheme.
	_ "github.com/larksuite/cli/agent/example"
)

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package envvars

import (
	"os"
	"strings"
	"unicode"
)

const (
	agentNameEnv     = "LARKSUITE_CLI_AGENT_NAME"
	agentNameMaxLen  = 128
	agentTraceMaxLen = 1024
)

func AgentName() string {
	return sanitizeSingleLine(os.Getenv(agentNameEnv), agentNameMaxLen)
}

func AgentTrace() string {
	return sanitizeSingleLine(os.Getenv(CliAgentTrace), agentTraceMaxLen)
}

func sanitizeSingleLine(raw string, maxLen int) string {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > maxLen {
		return ""
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return v
}

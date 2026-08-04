// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package envvars

import (
	"os"
	"strings"
	"unicode"
)

const (
	agentNameMaxLen  = 128
	agentTraceMaxLen = 1024
	xTtEnvMaxLen     = 128
)

func AgentName() string {
	return sanitizeSingleLine(os.Getenv(CliAgentName), agentNameMaxLen)
}

func AgentTrace() string {
	return sanitizeSingleLine(os.Getenv(CliAgentTrace), agentTraceMaxLen)
}

func XTtEnv() string {
	return sanitizeSingleLine(os.Getenv(CliXTtEnv), xTtEnvMaxLen)
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

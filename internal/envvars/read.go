// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package envvars

import (
	"net/url"
	"os"
	"strings"
	"unicode"
)

const (
	agentNameMaxLen       = 128
	agentTraceMaxLen      = 1024
	ttEnvMaxLen           = 128
	endpointBaseURLMaxLen = 256
)

func AgentName() string {
	return sanitizeSingleLine(os.Getenv(CliAgentName), agentNameMaxLen)
}

func AgentTrace() string {
	return sanitizeSingleLine(os.Getenv(CliAgentTrace), agentTraceMaxLen)
}

func TTEnv() string {
	return sanitizeSingleLine(os.Getenv(CliTTEnv), ttEnvMaxLen)
}

func OpenBaseURL() string {
	return sanitizeHTTPSBaseURL(os.Getenv(CliOpenBaseURL), endpointBaseURLMaxLen)
}

func AccountsBaseURL() string {
	return sanitizeHTTPSBaseURL(os.Getenv(CliAccountsBaseURL), endpointBaseURLMaxLen)
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

func sanitizeHTTPSBaseURL(raw string, maxLen int) string {
	v := sanitizeSingleLine(raw, maxLen)
	if v == "" {
		return ""
	}
	parsed, err := url.Parse(v)
	if err != nil ||
		parsed.Scheme != "https" ||
		parsed.Host == "" ||
		parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return ""
	}
	return strings.TrimRight(parsed.String(), "/")
}

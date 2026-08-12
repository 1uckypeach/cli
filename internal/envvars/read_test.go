// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package envvars

import (
	"strings"
	"testing"
)

func TestAgentName_EmptyWhenEnvUnset(t *testing.T) {
	t.Setenv(CliAgentName, "")
	if got := AgentName(); got != "" {
		t.Fatalf("AgentName() = %q, want empty when env unset", got)
	}
}

func TestAgentName_ReturnsCleanValue(t *testing.T) {
	const agentName = "sample-agent"
	t.Setenv(CliAgentName, agentName)
	if got := AgentName(); got != agentName {
		t.Fatalf("AgentName() = %q, want %q", got, agentName)
	}
}

func TestAgentName_TrimsWhitespace(t *testing.T) {
	const agentName = "sample-agent"
	t.Setenv(CliAgentName, "  "+agentName+"  ")
	if got := AgentName(); got != agentName {
		t.Fatalf("AgentName() = %q, want %q (whitespace trimmed)", got, agentName)
	}
}

func TestAgentName_RejectsCRLFInjection(t *testing.T) {
	t.Setenv(CliAgentName, "agent\r\nX-Evil: attack")
	if got := AgentName(); got != "" {
		t.Fatalf("AgentName() = %q, want empty for CR/LF value", got)
	}
}

func TestAgentName_RejectsControlChar(t *testing.T) {
	t.Setenv(CliAgentName, "agent\x01injected")
	if got := AgentName(); got != "" {
		t.Fatalf("AgentName() = %q, want empty for control char value", got)
	}
}

func TestAgentName_RejectsOverlongValue(t *testing.T) {
	longVal := strings.Repeat("a", agentNameMaxLen+1)
	t.Setenv(CliAgentName, longVal)
	if got := AgentName(); got != "" {
		t.Fatalf("AgentName() returned non-empty for %d-byte value (max %d)", len(longVal), agentNameMaxLen)
	}
}

func TestAgentTrace_EmptyWhenEnvUnset(t *testing.T) {
	t.Setenv(CliAgentTrace, "")
	if got := AgentTrace(); got != "" {
		t.Fatalf("AgentTrace() = %q, want empty when env unset", got)
	}
}

func TestAgentTrace_ReturnsCleanValue(t *testing.T) {
	t.Setenv(CliAgentTrace, "trace-abc-123")
	if got := AgentTrace(); got != "trace-abc-123" {
		t.Fatalf("AgentTrace() = %q, want %q", got, "trace-abc-123")
	}
}

func TestAgentTrace_TrimsWhitespace(t *testing.T) {
	t.Setenv(CliAgentTrace, "  trace-trim  ")
	if got := AgentTrace(); got != "trace-trim" {
		t.Fatalf("AgentTrace() = %q, want %q (whitespace trimmed)", got, "trace-trim")
	}
}

func TestAgentTrace_OnlyWhitespace_ReturnsEmpty(t *testing.T) {
	t.Setenv(CliAgentTrace, "   ")
	if got := AgentTrace(); got != "" {
		t.Fatalf("AgentTrace() = %q, want empty for whitespace-only value", got)
	}
}

func TestAgentTrace_RejectsCRLF(t *testing.T) {
	t.Setenv(CliAgentTrace, "val\r\nX-Evil: attack")
	if got := AgentTrace(); got != "" {
		t.Fatalf("AgentTrace() = %q, want empty for CR/LF value", got)
	}
}

func TestAgentTrace_RejectsLF(t *testing.T) {
	t.Setenv(CliAgentTrace, "val\nX-Evil: attack")
	if got := AgentTrace(); got != "" {
		t.Fatalf("AgentTrace() = %q, want empty for LF value", got)
	}
}

func TestAgentTrace_RejectsTab(t *testing.T) {
	t.Setenv(CliAgentTrace, "val\tinjected")
	if got := AgentTrace(); got != "" {
		t.Fatalf("AgentTrace() = %q, want empty for tab value", got)
	}
}

func TestAgentTrace_RejectsControlChar(t *testing.T) {
	t.Setenv(CliAgentTrace, "val\x01injected")
	if got := AgentTrace(); got != "" {
		t.Fatalf("AgentTrace() = %q, want empty for control char value", got)
	}
}

func TestAgentTrace_RejectsDEL(t *testing.T) {
	t.Setenv(CliAgentTrace, "val\x7finjected")
	if got := AgentTrace(); got != "" {
		t.Fatalf("AgentTrace() = %q, want empty for DEL value", got)
	}
}

func TestAgentTrace_RejectsOverlongValue(t *testing.T) {
	longVal := strings.Repeat("a", agentTraceMaxLen+1)
	t.Setenv(CliAgentTrace, longVal)
	if got := AgentTrace(); got != "" {
		t.Fatalf("AgentTrace() returned non-empty for %d-byte value (max %d)", len(longVal), agentTraceMaxLen)
	}
}

func TestAgentTrace_AcceptsMaxLengthValue(t *testing.T) {
	val := strings.Repeat("a", agentTraceMaxLen)
	t.Setenv(CliAgentTrace, val)
	if got := AgentTrace(); got != val {
		t.Fatalf("AgentTrace() = %q, want %d-byte value accepted", got, agentTraceMaxLen)
	}
}

func TestTTEnv_EmptyWhenEnvUnset(t *testing.T) {
	t.Setenv(CliTTEnv, "")
	if got := TTEnv(); got != "" {
		t.Fatalf("TTEnv() = %q, want empty when env unset", got)
	}
}

func TestTTEnv_ReturnsCleanValue(t *testing.T) {
	t.Setenv(CliTTEnv, "boe_f_agent_calendar_meeti")
	if got := TTEnv(); got != "boe_f_agent_calendar_meeti" {
		t.Fatalf("TTEnv() = %q, want %q", got, "boe_f_agent_calendar_meeti")
	}
}

func TestTTEnv_TrimsWhitespace(t *testing.T) {
	t.Setenv(CliTTEnv, "  boe_f_agent_calendar_meeti  ")
	if got := TTEnv(); got != "boe_f_agent_calendar_meeti" {
		t.Fatalf("TTEnv() = %q, want trimmed value", got)
	}
}

func TestTTEnv_RejectsCRLF(t *testing.T) {
	t.Setenv(CliTTEnv, "boe\r\nX-Evil: attack")
	if got := TTEnv(); got != "" {
		t.Fatalf("TTEnv() = %q, want empty for CR/LF value", got)
	}
}

func TestTTEnv_RejectsControlChar(t *testing.T) {
	t.Setenv(CliTTEnv, "boe\x01env")
	if got := TTEnv(); got != "" {
		t.Fatalf("TTEnv() = %q, want empty for control char value", got)
	}
}

func TestTTEnv_RejectsOverlongValue(t *testing.T) {
	longVal := strings.Repeat("a", ttEnvMaxLen+1)
	t.Setenv(CliTTEnv, longVal)
	if got := TTEnv(); got != "" {
		t.Fatalf("TTEnv() returned non-empty for %d-byte value (max %d)", len(longVal), ttEnvMaxLen)
	}
}

func TestOpenBaseURL_ReturnsHTTPSHostBase(t *testing.T) {
	t.Setenv(CliOpenBaseURL, "https://open.feishu-boe.cn/")
	if got := OpenBaseURL(); got != "https://open.feishu-boe.cn" {
		t.Fatalf("OpenBaseURL() = %q, want host base", got)
	}
}

func TestAccountsBaseURL_ReturnsHTTPSHostBase(t *testing.T) {
	t.Setenv(CliAccountsBaseURL, " https://accounts.feishu-boe.cn ")
	if got := AccountsBaseURL(); got != "https://accounts.feishu-boe.cn" {
		t.Fatalf("AccountsBaseURL() = %q, want host base", got)
	}
}

func TestOpenBaseURL_RejectsPathQueryAndNonHTTPS(t *testing.T) {
	for _, value := range []string{
		"http://open.feishu-boe.cn",
		"https://open.feishu-boe.cn/open-apis",
		"https://open.feishu-boe.cn?x=1",
		"https://user@open.feishu-boe.cn",
		"https://open.feishu-boe.cn\r\nX-Evil: attack",
	} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(CliOpenBaseURL, value)
			if got := OpenBaseURL(); got != "" {
				t.Fatalf("OpenBaseURL() = %q, want empty for invalid base %q", got, value)
			}
		})
	}
}

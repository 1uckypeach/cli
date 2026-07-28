// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package brand

import "strings"

// Brand represents the Lark platform brand.
// "feishu" targets China-mainland, "lark" targets international.
// ParseBrand and ResolveEndpoints map unrecognized values to Feishu.
type Brand string

const (
	Feishu Brand = "feishu"
	Lark   Brand = "lark"
)

// ParseBrand normalizes a brand string (case-insensitive, whitespace-tolerant);
// anything other than "lark" normalizes to Feishu.
func ParseBrand(value string) Brand {
	if strings.ToLower(strings.TrimSpace(value)) == "lark" {
		return Lark
	}
	return Feishu
}

// OAuthTokenV3Path is the unified OAuth 2.0 Token Endpoint path on the accounts
// domain. It serves every grant type (client_credentials for TAT,
// authorization_code / device_code / refresh_token for UAT) and replaces the
// legacy per-token endpoints (e.g. /open-apis/auth/v3/tenant_access_token/internal).
const OAuthTokenV3Path = "/oauth/v3/token"

// Endpoints holds resolved endpoint URLs for different Lark services.
type Endpoints struct {
	Open     string // e.g. "https://open.feishu.cn"
	Accounts string // e.g. "https://accounts.feishu.cn"
	MCP      string // e.g. "https://mcp.feishu.cn"
	AppLink  string // e.g. "https://applink.feishu.cn"
}

// ResolveEndpoints resolves endpoint URLs for the brand, normalizing its
// input so stored values with unusual casing still resolve correctly.
func ResolveEndpoints(brand Brand) Endpoints {
	switch ParseBrand(string(brand)) {
	case Lark:
		return Endpoints{
			Open:     "https://open.larksuite.com",
			Accounts: "https://accounts.larksuite.com",
			MCP:      "https://mcp.larksuite.com",
			AppLink:  "https://applink.larksuite.com",
		}
	default:
		return Endpoints{
			Open:     "https://open.feishu.cn",
			Accounts: "https://accounts.feishu.cn",
			MCP:      "https://mcp.feishu.cn",
			AppLink:  "https://applink.feishu.cn",
		}
	}
}

// ResolveOpenBaseURL returns the Open API base URL for the given brand.
func ResolveOpenBaseURL(brand Brand) string {
	return ResolveEndpoints(brand).Open
}

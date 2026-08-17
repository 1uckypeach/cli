// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package event

import (
	"testing"

	"github.com/larksuite/cli/internal/core"
	lark "github.com/larksuite/oapi-sdk-go/v3"
)

// The CLI passes the resolver's Open host into the SDK explicitly. Feishu is
// intentionally pinned to the pre OpenAPI domain for countdown PPE testing.
func TestWSDomainMatchesResolver(t *testing.T) {
	if got, want := core.ResolveEndpoints(core.BrandFeishu).Open, "https://open.feishu-boe.cn"; got != want {
		t.Errorf("feishu WS domain = %q, want %q", got, want)
	}
	if got, want := core.ResolveEndpoints(core.BrandLark).Open, lark.LarkBaseUrl; got != want {
		t.Errorf("lark WS domain = %q, want SDK %q", got, want)
	}
}

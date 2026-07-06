// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/shortcuts/common"
)

// newMetaTestRuntime creates a RuntimeContext wired to an httpmock.Registry
// so queryAppMeta can be tested without a real server.
func newMetaTestRuntime(t *testing.T) (*common.RuntimeContext, *httpmock.Registry) {
	t.Helper()
	cfg := &core.CliConfig{Brand: core.BrandFeishu, AppID: "cli_meta_test"}
	f, _, _, reg := cmdutil.TestFactory(t, cfg)
	rt := common.TestNewRuntimeContextForAPI(
		context.Background(),
		&cobra.Command{Use: "+meta-test"},
		cfg, f, core.AsUser,
	)
	return rt, reg
}

func TestQueryAppMeta_Success(t *testing.T) {
	rt, reg := newMetaTestRuntime(t)
	reg.Register(&httpmock.Stub{
		Method: "GET",
		URL:    "/open-apis/spark/v1/apps/app_test123",
		Body: map[string]interface{}{
			"code": float64(0),
			"data": map[string]interface{}{
				"app": map[string]interface{}{
					"app_type":  float64(7),
					"arch_type": float64(4),
					"name":      "Test App",
				},
			},
		},
	})

	meta := queryAppMeta(context.Background(), rt, "app_test123")
	if meta == nil {
		t.Fatal("expected non-nil appMeta")
	}
	if meta.AppType != 7 {
		t.Errorf("AppType = %d, want 7", meta.AppType)
	}
	if meta.ArchType != 4 {
		t.Errorf("ArchType = %d, want 4", meta.ArchType)
	}
}

func TestQueryAppMeta_APIError(t *testing.T) {
	rt, reg := newMetaTestRuntime(t)
	reg.Register(&httpmock.Stub{
		Method: "GET",
		URL:    "/open-apis/spark/v1/apps/app_err",
		Body: map[string]interface{}{
			"code": float64(500100),
			"msg":  "internal server error",
		},
	})

	meta := queryAppMeta(context.Background(), rt, "app_err")
	if meta != nil {
		t.Fatalf("expected nil for API error, got %+v", meta)
	}
}

func TestQueryAppMeta_MissingFields(t *testing.T) {
	rt, reg := newMetaTestRuntime(t)
	reg.Register(&httpmock.Stub{
		Method: "GET",
		URL:    "/open-apis/spark/v1/apps/app_partial",
		Body: map[string]interface{}{
			"code": float64(0),
			"data": map[string]interface{}{
				"app": map[string]interface{}{
					"name": "Partial App",
				},
			},
		},
	})

	meta := queryAppMeta(context.Background(), rt, "app_partial")
	if meta != nil {
		t.Fatalf("expected nil when app_type/arch_type are missing, got %+v", meta)
	}
}

func TestQueryAppMeta_NoAppObject(t *testing.T) {
	rt, reg := newMetaTestRuntime(t)
	reg.Register(&httpmock.Stub{
		Method: "GET",
		URL:    "/open-apis/spark/v1/apps/app_noobj",
		Body: map[string]interface{}{
			"code": float64(0),
			"data": map[string]interface{}{},
		},
	})

	meta := queryAppMeta(context.Background(), rt, "app_noobj")
	if meta != nil {
		t.Fatalf("expected nil when app object is absent, got %+v", meta)
	}
}

func TestIsStaticHtml(t *testing.T) {
	tests := []struct {
		name string
		meta *appMeta
		want bool
	}{
		{"nil meta", nil, false},
		{"static html (7,3)", &appMeta{AppType: 7, ArchType: 3}, true},
		{"full stack (7,4)", &appMeta{AppType: 7, ArchType: 4}, false},
		{"other type (3,0)", &appMeta{AppType: 3, ArchType: 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isStaticHtml(tt.meta); got != tt.want {
				t.Errorf("isStaticHtml(%+v) = %v, want %v", tt.meta, got, tt.want)
			}
		})
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		wantN  int
		wantOK bool
	}{
		{"float64", float64(42), 42, true},
		{"int", 7, 7, true},
		{"json.Number", json.Number("99"), 99, true},
		{"json.Number non-int", json.Number("abc"), 0, false},
		{"string", "7", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, ok := toInt(tt.input)
			if n != tt.wantN || ok != tt.wantOK {
				t.Errorf("toInt(%v) = (%d, %v), want (%d, %v)", tt.input, n, ok, tt.wantN, tt.wantOK)
			}
		})
	}
}

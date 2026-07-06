// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/shortcuts/common"
)

// newMetaTestRuntime creates a RuntimeContext wired to an httpmock.Registry
// so queryAppType can be tested without a real server.
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

func TestQueryAppType_Success(t *testing.T) {
	rt, reg := newMetaTestRuntime(t)
	reg.Register(&httpmock.Stub{
		Method: "GET",
		URL:    "/open-apis/spark/v1/apps/app_test/type",
		Body: map[string]interface{}{
			"code": float64(0),
			"data": map[string]interface{}{
				"app_type": "modern_html",
			},
		},
	})

	result := queryAppType(context.Background(), rt, "app_test")
	if result != "modern_html" {
		t.Errorf("queryAppType = %q, want modern_html", result)
	}
}

func TestQueryAppType_APIError(t *testing.T) {
	rt, reg := newMetaTestRuntime(t)
	reg.Register(&httpmock.Stub{
		Method: "GET",
		URL:    "/open-apis/spark/v1/apps/app_bad/type",
		Status: 500,
		Body:   map[string]interface{}{"code": float64(99999), "msg": "internal error"},
	})

	result := queryAppType(context.Background(), rt, "app_bad")
	if result != "" {
		t.Errorf("queryAppType = %q, want empty on error", result)
	}
}

func TestQueryAppType_MissingField(t *testing.T) {
	rt, reg := newMetaTestRuntime(t)
	reg.Register(&httpmock.Stub{
		Method: "GET",
		URL:    "/open-apis/spark/v1/apps/app_no/type",
		Body: map[string]interface{}{
			"code": float64(0),
			"data": map[string]interface{}{},
		},
	})

	result := queryAppType(context.Background(), rt, "app_no")
	if result != "" {
		t.Errorf("queryAppType = %q, want empty when field missing", result)
	}
}

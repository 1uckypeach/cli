// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

import (
	"context"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
)

func TestAppsDBEnvDiffDryRun(t *testing.T) {
	setAppsDryRunEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args:      []string{"apps", "+db-env-diff", "--app-id", "app_x", "--dry-run"},
		DefaultAs: "user",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)
	out := result.Stdout
	require.Equal(t, "POST", clie2e.DryRunGet(out, "api.0.method").String(), out)
	require.Equal(t, "/open-apis/spark/v1/apps/app_x/db/env_migrate", clie2e.DryRunGet(out, "api.0.url").String(), out)
	require.True(t, clie2e.DryRunGet(out, "api.0.body.dry_run").Bool(), out)
}

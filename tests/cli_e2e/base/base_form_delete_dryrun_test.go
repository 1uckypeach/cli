// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package base

import (
	"context"
	"strings"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseFormDeleteDryRun(t *testing.T) {
	setBaseDryRunConfigEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"base", "+form-delete",
			"--base-token", "basXXXX",
			"--table-id", "tblXXXX",
			"--form-id", "vewXXXX",
			"--dry-run",
		},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)

	output := strings.TrimSpace(result.Stdout)
	assert.Contains(t, output, `"method": "DELETE"`)
	assert.Contains(t, output, "/open-apis/base/v3/bases/basXXXX/tables/tblXXXX/forms/vewXXXX")
}

func TestBaseFormDeleteDryRunMissingFormID(t *testing.T) {
	setBaseDryRunConfigEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"base", "+form-delete",
			"--base-token", "basXXXX",
			"--table-id", "tblXXXX",
			"--yes",
			"--dry-run",
		},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	assert.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, result.Stderr, "form-id")
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package wiki

import (
	"context"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestWikiIdentityValidationDryRun(t *testing.T) {
	setWikiNodeCreateDryRunEnv(t)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "member add rejects bot department member",
			args: []string{
				"wiki", "+member-add",
				"--space-id", "space_42",
				"--member-id", "od_department",
				"--member-type", "opendepartmentid",
				"--member-role", "member",
				"--dry-run",
			},
		},
		{
			name: "move rejects bot personal library",
			args: []string{
				"wiki", "+move",
				"--obj-type", "docx",
				"--obj-token", "doccn_test",
				"--target-space-id", "my_library",
				"--dry-run",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			t.Cleanup(cancel)

			result, err := clie2e.RunCmd(ctx, clie2e.Request{Args: tt.args, DefaultAs: "bot"})
			require.NoError(t, err)
			result.AssertExitCode(t, 2)
			assert.Equal(t, "identity_not_supported", wikiErrorSubtype(result),
				"identity incompatibility must be machine-readable; stdout=%s stderr=%s", result.Stdout, result.Stderr)
		})
	}
}

func wikiErrorSubtype(result *clie2e.Result) string {
	if subtype := gjson.Get(result.Stdout, "error.subtype").String(); subtype != "" {
		return subtype
	}
	return gjson.Get(result.Stderr, "error.subtype").String()
}

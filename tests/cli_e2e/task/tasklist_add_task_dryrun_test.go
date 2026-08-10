// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package task

import (
	"context"
	"testing"
	"time"

	clie2e "github.com/larksuite/cli/tests/cli_e2e"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestTaskTasklistAddTaskDryRun pins the legacy batch preview contract through
// the built binary. Execute fans out over every task ID, but DryRun currently
// previews only the first task; Typed migration must not change that silently.
func TestTaskTasklistAddTaskDryRun(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("LARKSUITE_CLI_APP_ID", "tasklist_add_dryrun_test")
	t.Setenv("LARKSUITE_CLI_APP_SECRET", "tasklist_add_dryrun_secret")
	t.Setenv("LARKSUITE_CLI_BRAND", "feishu")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := clie2e.RunCmd(ctx, clie2e.Request{
		Args: []string{
			"task", "+tasklist-task-add",
			"--tasklist-id", "https://applink.feishu.cn/client/todo/task_list?guid=tl-dryrun&extra=ignored",
			"--task-id", " first/task? ,second-task",
			"--section-guid", " section-dryrun ",
			"--dry-run",
		},
		DefaultAs: "bot",
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 0)

	out := result.Stdout
	require.Equal(t, int64(1), gjson.Get(out, "data.api.#").Int(), out)
	require.Equal(t, "POST", gjson.Get(out, "data.api.0.method").String(), out)
	require.Equal(t,
		"/open-apis/task/v2/tasks/first%2Ftask%3F/add_tasklist",
		gjson.Get(out, "data.api.0.url").String(),
		out,
	)
	require.Equal(t, "open_id", gjson.Get(out, "data.api.0.params.user_id_type").String(), out)
	require.Equal(t, "tl-dryrun", gjson.Get(out, "data.api.0.body.tasklist_guid").String(), out)
	require.Equal(t, "section-dryrun", gjson.Get(out, "data.api.0.body.section_guid").String(), out)
}

// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package task

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

type tasklistAddArgs struct {
	TasklistID   string `flag:"tasklist-id" schema:"required" doc:"tasklist ID or tasklist URL"`
	TaskID       string `flag:"task-id" schema:"required" doc:"task ID; comma-separated for multiple tasks"`
	SectionGUID  string `flag:"section-guid" schema:"optional" doc:"section GUID"`
	TasklistGUID string `arg:"local"`
}

type tasklistAddSuccessfulTask struct {
	GUID string `json:"guid" schema:"required" doc:"task GUID"`
	URL  string `json:"url" schema:"required" doc:"task URL"`
}
type tasklistAddFailedTask struct {
	Code    int    `json:"code" schema:"required" doc:"Lark API error code"`
	GUID    string `json:"guid" schema:"required" doc:"task GUID"`
	Hint    string `json:"hint" schema:"required" doc:"recovery hint"`
	Message string `json:"message" schema:"required" doc:"failure message"`
	Type    string `json:"type" schema:"required" doc:"failure subtype"`
}
type tasklistAddData struct {
	FailedTasks     []tasklistAddFailedTask     `json:"failed_tasks" schema:"required;nullable" doc:"tasks that could not be added"`
	SuccessfulTasks []tasklistAddSuccessfulTask `json:"successful_tasks" schema:"required;nullable" doc:"tasks added successfully"`
	TasklistGUID    string                      `json:"tasklist_guid" schema:"required" doc:"target tasklist GUID"`
}

var AddTaskToTasklist = common.Define(common.Definition[tasklistAddArgs, tasklistAddData]{
	Metadata: common.CommandMetadata{
		Service: "task", Command: "+tasklist-task-add", Description: "add tasks to a tasklist", Risk: common.RiskWrite,
		Authorization: common.AuthorizationDefinition{Identities: map[common.Identity]common.IdentityAuthorization{
			common.IdentityUser: {RequiredScopes: []string{"task:task:write"}}, common.IdentityBot: {RequiredScopes: []string{"task:task:write"}},
		}},
	},
	Output: common.OutputDefinition{Outcomes: common.OutcomeDefinition{PartialFailure: &common.PartialFailureDefinition{
		ExitCode: 1, FailedItems: &common.FailedItemDefinition{ItemsPath: "/failed_tasks", IdentityPaths: []string{"/guid"}, AllItems: true},
	}}},
	Hooks: common.Hooks[tasklistAddArgs, tasklistAddData]{
		Normalize: func(_ context.Context, _ common.CommandContext, args *tasklistAddArgs) error {
			args.TasklistGUID = extractTasklistGuid(args.TasklistID)
			args.SectionGUID = strings.TrimSpace(args.SectionGUID)
			return nil
		},
		DryRun: func(_ context.Context, _ common.CommandContext, args *tasklistAddArgs) *common.DryRunAPI {
			taskIDs := strings.Split(args.TaskID, ",")
			taskID := url.PathEscape(strings.TrimSpace(taskIDs[0]))
			body := map[string]interface{}{"tasklist_guid": args.TasklistGUID}
			if args.SectionGUID != "" {
				body["section_guid"] = args.SectionGUID
			}
			return common.NewDryRunAPI().POST("/open-apis/task/v2/tasks/" + taskID + "/add_tasklist").
				Params(map[string]interface{}{"user_id_type": "open_id"}).Body(body)
		},
		Execute: func(ctx context.Context, command common.CommandContext, args *tasklistAddArgs) (common.Result[tasklistAddData], error) {
			params := map[string]interface{}{"user_id_type": "open_id"}
			body := map[string]interface{}{"tasklist_guid": args.TasklistGUID}
			if args.SectionGUID != "" {
				body["section_guid"] = args.SectionGUID
			}
			var successful []tasklistAddSuccessfulTask
			var failed []tasklistAddFailedTask
			for _, rawTaskID := range strings.Split(args.TaskID, ",") {
				taskID := strings.TrimSpace(rawTaskID)
				if taskID == "" {
					continue
				}
				data, err := callTaskAPITypedCommand(ctx, command, http.MethodPost, "/open-apis/task/v2/tasks/"+url.PathEscape(taskID)+"/add_tasklist", params, body)
				if err != nil {
					presented := command.PresentError(err)
					failure := tasklistAddFailedTask{GUID: taskID, Type: "api_error", Message: presented.Error()}
					if problem, ok := errs.ProblemOf(presented); ok {
						failure.Type = string(problem.Subtype)
						failure.Code = problem.Code
						failure.Message = problem.Message
						failure.Hint = problem.Hint
					}
					failed = append(failed, failure)
					continue
				}
				task, _ := data["task"].(map[string]interface{})
				successful = append(successful, tasklistAddSuccessfulTask{GUID: common.GetString(task, "guid"), URL: truncateTaskURL(common.GetString(task, "url"))})
			}
			result := tasklistAddData{FailedTasks: failed, SuccessfulTasks: successful, TasklistGUID: args.TasklistGUID}
			if len(failed) > 0 {
				return common.Partial(result), nil
			}
			return common.Success(result), nil
		},
		Renderers: map[string]common.Renderer[tasklistAddData]{"pretty": func(w io.Writer, data tasklistAddData) error {
			fmt.Fprintf(w, "✅ Tasks added to tasklist %s!\n", data.TasklistGUID)
			fmt.Fprintf(w, "Successful: %d, Failed: %d\n", len(data.SuccessfulTasks), len(data.FailedTasks))
			if len(data.SuccessfulTasks) > 0 {
				fmt.Fprintln(w, "Successful Tasks:")
				for _, task := range data.SuccessfulTasks {
					fmt.Fprintf(w, "  - ID: %s", task.GUID)
					if task.URL != "" {
						fmt.Fprintf(w, ", URL: %s", task.URL)
					}
					fmt.Fprintln(w)
				}
			}
			if len(data.FailedTasks) > 0 {
				fmt.Fprintln(w, "Failed Tasks:")
				for _, failure := range data.FailedTasks {
					fmt.Fprintf(w, "  - %s: %s\n", failure.GUID, failure.Message)
				}
			}
			return nil
		}},
	},
})

# Docs CLI E2E Coverage

## Metrics
- Denominator: 11 leaf commands
- Covered: 6
- Coverage: 54.5%

## Summary
- TestDocs_CreateAndFetchWorkflow: proves `docs +create` and `docs +fetch`; key `t.Run(...)` proof points are `create as bot` and `fetch as bot`.
- TestDocs_CreateAndFetchWorkflowAsUser: proves the same shortcut pair with UAT injection via `create as user` and `fetch as user`; creates its own Drive folder fixture first, then reads back the created doc by token.
- TestDocs_UpdateWorkflow: proves `docs +update` via `update-title-and-content as bot`, then re-fetches the same doc in `verify as bot` to assert persisted title/content changes.
- TestDocs_LocalResourcesWorkflowAsBot / AsUser: prove the full local image + file lifecycle for `docs +create` and `docs +update --command append`: placeholder correlation, distinct media block IDs, invalid `width`/`height`/`size` normalization, media upload, token binding, response scrubbing, XML/Markdown fetch verification, exported-Markdown replay with image caption restoration, and cleanup.
- TestDocs_LocalResourcesDryRun: proves both `docs +create` and `docs +update --command append` expose the complete no-network request plan for local images and files: placeholder content, media uploads, batch binding, conditional verification, and failure cleanup.
- TestDocs_DryRunDefaultsToV2OpenAPI: proves `docs +create`, `docs +fetch`, and `docs +update` dry-run all emit `/open-apis/docs_ai/v1/...` requests without MCP or `--api-version` guidance; its fetch case asserts fetch sends the default `extra_param`, and its update case asserts `--reference-map` is sent as request body `reference_map`.
- TestDocs_CreateTitleDryRunPrependsContent: proves `docs +create --title` dry-run prepends an escaped `<title>...</title>` tag to request body `content`.
- TestDocs_DryRunDefaultsToV2OpenAPI also proves `docs +history-list`, `docs +history-revert`, and `docs +history-revert-status` dry-run endpoint and query/body shapes.
- TestDocs_HistoryWorkflow proves the guarded live history flow (`LARK_DOC_HISTORY_E2E=1`): create, update, list prior revisions, revert, poll status when needed, and fetch to verify reverted content.
- Setup note: docs workflows create a Drive folder through `drive files create_folder` in `helpers_test.go`; that helper is external to the docs domain and is not counted here.
- Blocked area: standalone media and search shortcuts still need dedicated deterministic workflows; local resource authoring through create/update is covered.

## Command Table

| Status | Cmd | Type | Testcase | Key parameter shapes | Notes / uncovered reason |
| --- | --- | --- | --- | --- | --- |
| ✓ | docs +create | shortcut | docs/helpers_test.go::createDocWithRetry; docs_create_fetch_test.go::TestDocs_CreateAndFetchWorkflowAsUser/create as user; docs_local_resources_workflow_test.go::TestDocs_LocalResourcesWorkflowAsBot/create image and source; docs_local_resources_workflow_test.go::TestDocs_LocalResourcesWorkflowAsUser/create image and source; docs_local_resources_dryrun_test.go::TestDocs_LocalResourcesDryRun/create; docs_update_dryrun_test.go::TestDocs_DryRunDefaultsToV2OpenAPI/create; docs_update_dryrun_test.go::TestDocs_CreateTitleDryRunPrependsContent | `--parent-token`; `--doc-format markdown`; `--content`; `--title`; XML `<img path="@relative">` + `<source path="@relative">` | local-resource workflows assert returned image/file block IDs and bound tokens |
| ✓ | docs +fetch | shortcut | docs_fetch_dryrun_test.go::TestDocsFetchDryRunIgnoresAPIVersionCompatFlag; docs_create_fetch_test.go::TestDocs_CreateAndFetchWorkflow/fetch as bot; docs_update_test.go::TestDocs_UpdateWorkflow/verify as bot; docs_create_fetch_test.go::TestDocs_CreateAndFetchWorkflowAsUser/fetch as user; docs_local_resources_workflow_test.go::testDocsLocalResourcesWorkflow/fetch verifies persisted resources; docs_update_dryrun_test.go::TestDocs_DryRunDefaultsToV2OpenAPI/fetch | `--doc <docToken>`; `--doc-format markdown|xml`; `--detail full`; default `extra_param.enable_user_cite_reference_map=true`; `--api-version v1` compatibility flag still dry-runs the v2 fetch endpoint | local-resource fetch asserts captions/file names persist and internal markers/paths do not leak |
| ✓ | docs +history-list | shortcut | docs_update_dryrun_test.go::TestDocs_DryRunDefaultsToV2OpenAPI/history list; docs_history_workflow_test.go::TestDocs_HistoryWorkflow | `--doc`; `--page-size`; `--page-token` | live workflow gated by `LARK_DOC_HISTORY_E2E=1` |
| ✓ | docs +history-revert | shortcut | docs_update_dryrun_test.go::TestDocs_DryRunDefaultsToV2OpenAPI/history revert; docs_history_workflow_test.go::TestDocs_HistoryWorkflow | `--doc`; `--history-version-id`; `--wait-timeout-ms` | live workflow gated by `LARK_DOC_HISTORY_E2E=1` |
| ✓ | docs +history-revert-status | shortcut | docs_update_dryrun_test.go::TestDocs_DryRunDefaultsToV2OpenAPI/history revert status; docs_history_workflow_test.go::TestDocs_HistoryWorkflow | `--doc`; `--task-id` | live workflow polls only when revert returns `running` |
| ✕ | docs +media-download | shortcut |  | none | no media fixture workflow yet |
| ✕ | docs +media-insert | shortcut |  | none | requires deterministic upload fixture and rollback assertions |
| ✕ | docs +media-preview | shortcut |  | none | requires deterministic media fixture |
| ✕ | docs +search | shortcut |  | none | search results are ambient and not yet stabilized for E2E |
| ✓ | docs +update | shortcut | docs_update_test.go::TestDocs_UpdateWorkflow/update-title-and-content as bot; docs_local_resources_workflow_test.go::testDocsLocalResourcesWorkflow/append image and source; docs_local_resources_dryrun_test.go::TestDocs_LocalResourcesDryRun/update append; docs_update_dryrun_test.go::TestDocs_DryRunDefaultsToV2OpenAPI/update | `--doc`; `--command overwrite|append`; `--doc-format markdown|xml`; `--content`; local `<img path>` / `<source path>`; optional `--reference-map` -> body `reference_map` | local resources are covered under both bot and user identities |
| ✕ | docs +whiteboard-update | shortcut |  | none | requires whiteboard fixture and DSL-specific assertions |

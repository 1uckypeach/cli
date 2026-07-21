# im +chat-members-add

> **Prerequisite:** Read [`../lark-shared/SKILL.md`](../../lark-shared/SKILL.md) first to understand authentication, global parameters, and safety rules.

Add users and/or bots to a group chat in one command. `--users` (open_id) and `--bots` (app_id) map to two independent underlying calls — `chat.members.create` only accepts one `member_id_type` per request, so a single call cannot mix user and bot IDs. This shortcut issues both calls as needed and merges the results into one ledger.

This skill maps to the shortcut: `lark-cli im +chat-members-add` (internally calls `POST /open-apis/im/v1/chats/{chat_id}/members` up to twice, with `succeed_type=1`/best-effort).

## Commands

```bash
# Add users only
lark-cli im +chat-members-add --chat-id oc_xxx --users ou_a,ou_b

# Add bots only
lark-cli im +chat-members-add --chat-id oc_xxx --bots cli_x

# Add both (two API calls internally, merged into one result)
lark-cli im +chat-members-add --chat-id oc_xxx --users ou_a,ou_b --bots cli_x

# JSON output / preview the request
lark-cli im +chat-members-add --chat-id oc_xxx --users ou_a --format json
lark-cli im +chat-members-add --chat-id oc_xxx --users ou_a --bots cli_x --dry-run
```

## Parameters

| Parameter | Required | Limits | Description |
|------|------|------|------|
| `--chat-id <id>` | Yes | `oc_xxx` | Target chat |
| `--users <ids>` | No* | `ou_xxx` (comma-separated or repeated), max 50 | User open_ids to invite |
| `--bots <ids>` | No* | `cli_xxx` (comma-separated or repeated), max 5 | Bot app_ids to invite |
| `--format json` | No | - | Output as JSON |
| `--dry-run` | No | - | Preview the request(s) without executing them |

\* At least one of `--users`/`--bots` is required.

> Supports both `--as user` (default) and `--as bot`. The caller must be in the target chat; for bot calls, invited users must be within the app's availability; for internal chats the operator must belong to the same tenant.

## Output Fields

| Field | Description |
|------|------|
| `chat_id` | The target chat ID |
| `succeeded_id_list` | IDs (users + bots merged) that were actually added to the chat |
| `invalid_id_list` | IDs that are resigned / invisible / from a disabled app |
| `not_existed_id_list` | IDs that don't exist |
| `pending_approval_id_list` | IDs submitted but awaiting owner/admin approval — **not yet actually in the chat** |
| `call_errors` | Whole-call failures (not per-ID) — each entry has `member_type`, the `id_list` that call carried, and `error` |
| `success_count` / `failure_count` / `total` | `total` = requested ID count (post-validation, per-flag deduped); `failure_count` sums the three failure buckets plus every `call_errors[].id_list`; `success_count + failure_count == total` always holds |

## Partial failure: valid IDs still get added

The shortcut always uses best-effort semantics (`succeed_type=1`): a single resigned/nonexistent/unapprovable ID does not block the rest of the batch. When `failure_count > 0`, the command exits non-zero (`ok: false` in the JSON envelope) even though some IDs did succeed — always check `success_count`/`failure_count`, not just the exit code, to know exactly what happened.

`pending_approval_id_list` is a distinct case: those members were **not** added — they're waiting on an owner/admin decision. Don't treat that bucket as "succeeded".

## Common Errors and Troubleshooting

| Symptom | Root Cause | Solution |
|---------|---------|---------|
| `invalid --chat-id ...: must be an open_chat_id starting with oc_` | `--chat-id` missing or malformed | Provide the `oc_xxx` chat ID |
| `at least one of --users or --bots is required` | Both flags omitted | Provide at least one |
| `invalid --users value ...: must start with "ou_"` | Wrong ID type in `--users` | Use `open_id` (`ou_xxx`), not `union_id`/`user_id`/`app_id` |
| `invalid --bots value ...: must start with "cli_"` | Wrong ID type in `--bots` | Use the app's `app_id` (`cli_xxx`) |
| `--users exceeds the maximum of 50` / `--bots exceeds the maximum of 5` | Batch too large | Split into multiple calls |
| Permission denied | Missing `im:chat.members:write_only`, or caller not in the chat / not owner-admin when restricted | Bot: enable the scope in the console. User: `lark-cli auth login --scope "im:chat.members:write_only"`; confirm the caller is in the chat |
```

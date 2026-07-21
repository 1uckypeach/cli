# im +chat-members-add

> **Prerequisite:** Read [`../lark-shared/SKILL.md`](../../lark-shared/SKILL.md) first to understand authentication, global parameters, and safety rules.

Add user and bot members to an existing group chat. User members are identified by `open_id`; bot members are identified by application `app_id`.

This skill maps to the shortcut: `lark-cli im +chat-members-add` (internally calls `POST /open-apis/im/v1/chats/{chat_id}/members`). Prefer this shortcut over the native Meta API command because it validates identifiers, separates user and bot requests, and returns one stable result.

## Command

```bash
lark-cli im +chat-members-add \
  --chat-id oc_xxx \
  --users ou_a,ou_b \
  --bots cli_a \
  --as user \
  --yes
```

To preview both requests without changing the chat, replace `--yes` with `--dry-run`. Dry-run does not require `--yes`.

## Parameters

| Parameter | Required | Limits | Description |
|------|------|------|------|
| `--chat-id <id-or-url>` | Yes | `oc_xxx` or a supported chat link | Target group ID or supported group link |
| `--users <ids>` | At least one member type is required | Up to 50 unique values | Comma-separated user `open_id` values in `ou_xxx` form |
| `--bots <ids>` | At least one member type is required | Up to 5 unique values | Comma-separated bot application `app_id` values in `cli_xxx` form |
| `--as <identity>` | No | `user` or `bot` | Identity used for both requests |
| `--yes` | Required for execution | - | Confirms the high-impact write operation |
| `--dry-run` | No | - | Prints the request preview without executing it or requiring `--yes` |

At least one of `--users` or `--bots` must be present. Duplicate values are removed automatically while preserving first-seen order. Limits are applied after deduplication.

Both `--as user` and `--as bot` are supported. The selected identity requires the `im:chat.members:write_only` scope and sufficient permission to add members to the target chat.

## Request Behavior

User and bot members are always sent in separate requests because they use different identifier types. The user request runs first with `member_id_type=open_id`; the bot request follows with `member_id_type=app_id`. Every request fixes `succeed_type=1`, so members that can be added continue to be added while member-level failures are returned in the response.

If the user request fails for any reason, execution stops immediately and the bot request is not sent. A network, transport, or invalid-response failure in this first request is non-retryable until the current user members have been read back with `+chat-members-list`; retry only users that are not confirmed present. If the user request succeeds and the bot request fails, confirmed user results remain in the output and in the chat. The operation has no transaction or automatic rollback.

## Output

The `data` object contains one combined result. It reports only the chat identifier, the total `success_count`, and three member-level failure arrays; counts are not separated by user and bot type.

| Field | Description |
|------|------|
| `chat_id` | Target chat ID |
| `success_count` | Total number of members confirmed as added after deduplication |
| `invalid_id_list` | Identifiers rejected as invalid |
| `not_existed_id_list` | Identifiers that do not exist |
| `pending_approval_id_list` | Identifiers awaiting approval and not yet confirmed as added |

A successful response exits `0` with `ok:true`:

```json
{"ok":true,"data":{"chat_id":"oc_xxx","success_count":3,"invalid_id_list":[],"not_existed_id_list":[],"pending_approval_id_list":[]},"meta":{"count":3}}
```

If any member-level failure array is non-empty, stdout carries `ok:false` with the complete result and the process exits `1`. Members already accepted by the service remain in the chat:

```json
{"ok":false,"data":{"chat_id":"oc_xxx","success_count":1,"invalid_id_list":["ou_invalid"],"not_existed_id_list":["cli_missing"],"pending_approval_id_list":["ou_pending"]}}
```

Scripts and agents must read `success_count` and all three arrays even when the process exits `1`.

## Bot Request Failure

When the user request has completed and the bot request returns an error, the partial result also contains `failed_member_type:"bot"`, `outcome_unknown`, and a structured `error` object. `success_count` and the three arrays describe only the confirmed user request result; the shortcut does not guess the bot outcome.

```json
{"ok":false,"data":{"chat_id":"oc_xxx","success_count":1,"invalid_id_list":[],"not_existed_id_list":[],"pending_approval_id_list":[],"failed_member_type":"bot","outcome_unknown":true,"error":{"type":"network","subtype":"network_transport","message":"member request failed","hint":"List current chat members before retrying; retry only bots not confirmed present.","retryable":false}}}
```

`outcome_unknown:true` covers network or transport failures and responses that cannot be parsed or validated. First read the current members with the same identity:

```bash
lark-cli im +chat-members-list \
  --chat-id oc_xxx \
  --page-all \
  --as <same-identity>
```

After the readback, process only bots that are not confirmed present. Do not repeat the complete `+chat-members-add` command, because the service may have accepted the bot request before the response became unavailable.

Deterministic permission, authentication, and API errors use `outcome_unknown:false`. Handle these through the structured `error` fields. The confirmed user additions remain in the chat.

## References

- [List chat members](lark-im-chat-members-list.md)
- [Create a chat](lark-im-chat-create.md)
- [lark-im](../SKILL.md)
- [lark-shared](../../lark-shared/SKILL.md)

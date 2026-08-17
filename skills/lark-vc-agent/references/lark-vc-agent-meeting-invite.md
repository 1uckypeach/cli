# vc +meeting-invite

Invite selected users or all eligible Calendar attendees through the Agent bot API.

```bash
lark-cli vc +meeting-invite --as bot --meeting-id 69999999 --type SELECTED --open-ids ou_xxx,ou_yyy
lark-cli vc +meeting-invite --as bot --meeting-id 69999999 --type ALL_SUGGESTED
lark-cli vc +meeting-invite --as bot --meeting-id 69999999 --type ALL_SUGGESTED --dry-run
```

## Parameters

| Parameter | Required | Description |
| --- | --- | --- |
| `--meeting-id` | Yes | Meeting ID, not the 9-digit meeting number. |
| `--type` | Yes | `SELECTED` or `ALL_SUGGESTED` (case-insensitive). |
| `--open-ids` | For `SELECTED` | User `open_id` values (`ou_xxx`), comma-separated or repeated, maximum 200. Do not set for `ALL_SUGGESTED`. |

This shortcut only accepts bot identity and calls `POST /open-apis/vc/v1/bots/invite`.

- `SELECTED` sends explicit user `open_id` values and rejects more than 200 IDs before sending.
- `ALL_SUGGESTED` sends only the type. The server builds the Calendar one-click-invite candidate set, consumes all Calendar pages, filters resources, bots, the actor itself, declined and removed attendees, applies the 200-person cap, and then reuses the shared InviteParticipant policy chain.
- Wire contract: `SELECTED` sends `invite_type=2`, `invitees=[{"id":"ou_xxx","user_type":1}]`, and query `user_id_type=open_id`; `ALL_SUGGESTED` sends `invite_type=1` and omits `invitees`.
- Response contract: `SELECTED` may return `invite_results` for explicit invitees; `ALL_SUGGESTED` returns aggregate fields such as `failed_count`, `invited_count`, and `has_more`, without per-user `invite_results`.

## Permission Notes

- The meeting must be a Calendar VC meeting and the app bot must already be in the meeting.
- Agent Invite depends on the meeting's Agent join capability. If the Calendar meeting did not enable the AI/Agent meeting setting, invite calls fail before candidates are resolved.
- `SELECTED` with one invitee follows the normal single-invite policy, so a regular in-meeting participant may be able to invite that user.
- `ALL_SUGGESTED` and multi-user `SELECTED` use the batch/suggested-list invite policy. In practice, the bot should be the current host or co-host before calling them; a regular participant bot can be rejected or have candidates filtered by the shared meeting permission chain.

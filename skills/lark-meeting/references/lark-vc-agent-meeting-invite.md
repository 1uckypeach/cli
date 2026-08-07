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

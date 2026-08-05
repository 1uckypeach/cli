# vc +meeting-invite

Invite selected users or all eligible Calendar attendees through the Agent bot API.

```bash
lark-cli vc +meeting-invite --as bot --meeting-id 69999999 --scope SELECTED --invitee-user-ids 1001,1002
lark-cli vc +meeting-invite --as bot --meeting-id 69999999 --scope ALL
lark-cli vc +meeting-invite --as bot --meeting-id 69999999 --scope ALL --dry-run
```

## Parameters

| Parameter | Required | Description |
| --- | --- | --- |
| `--meeting-id` | Yes | Meeting ID, not the 9-digit meeting number. |
| `--scope` | Yes | `SELECTED` or `ALL` (case-insensitive). |
| `--invitee-user-ids` | For `SELECTED` | Comma-separated numeric user IDs, maximum 200. Do not set for `ALL`. |

This shortcut only accepts bot identity and calls `POST /open-apis/vc/v1/bots/invite`.

- `SELECTED` sends the explicit user list and rejects more than 200 IDs before sending.
- `ALL` sends only the scope. The server builds the complete candidate set, consumes all Calendar pages, filters resources, bots, the actor itself, declined and removed attendees, applies the 200-person cap, and then reuses the shared InviteParticipant policy chain.

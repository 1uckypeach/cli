# vc +meeting-start

Start and join a Calendar meeting through the external `START_AND_JOIN` API as the app bot.

```bash
lark-cli vc +meeting-start --as bot --meeting-number 123456789
lark-cli vc +meeting-start --as bot --meeting-number 123456789 --dry-run
```

## Parameters

| Parameter | Required | Description |
| --- | --- | --- |
| `--meeting-number` | Yes | 9-digit Calendar meeting number. |
| `--password` | No | Meeting password when required. |
| `--call-id` | No | Invite correlation ID when the start follows an invite event. |

This shortcut only accepts bot identity and sends `action=START_AND_JOIN`. It does not accept or send `owner_user_id`: the service derives the Bot, App, tenant and Agent Owner from the authenticated application and verifies the Calendar relationship before creating the VC meeting. Use `+meeting-join` when the caller only wants to join an already-running meeting.

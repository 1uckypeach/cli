# vc +meeting-start

Probe the external Calendar `START_AND_JOIN` API as the app bot.

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

This shortcut only accepts bot identity and sends `meeting_action=start_and_join`. It does not accept or send `owner_user_id`, and it cannot establish a trusted Owner handoff. Production Agent-created meetings are started by the Agent runtime using its authoritative current-human Owner; an external START request without that trusted runtime context fails closed when the Calendar meeting has not started. Use this shortcut for dry-run/manual diagnostics, and use `+meeting-join` for ordinary joining of an already-running meeting.

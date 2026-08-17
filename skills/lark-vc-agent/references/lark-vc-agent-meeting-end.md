# vc +meeting-end

End a meeting as the app bot when that bot is the current host.

```bash
lark-cli vc +meeting-end --as bot --meeting-id 69999999
lark-cli vc +meeting-end --as bot --meeting-id 69999999 --dry-run
```

## Parameters

| Parameter | Required | Description |
| --- | --- | --- |
| `--meeting-id` | Yes | Meeting ID, not the 9-digit meeting number. |

The service rejects non-host bots, including the case where a non-host bot is the last remaining participant.

This shortcut only accepts bot identity and calls `POST /open-apis/vc/v1/bots/end`. Billing, only-one-participant, offline and share-screen conditions do not authorize a non-Host bot to end the meeting.

Required application scope: `vc:meeting.bot.manage:write`.

## Error Guidance

- `bot is not in the meeting`: the app bot is not currently in the meeting. Start or join the Calendar meeting with the same app bot before ending it.
- `bot is not host`: the app bot is in the meeting but is not the current host. Transfer host to the app bot, or let the current host/owner end the meeting.
- `switch for allowing agents to join meetings is disabled`: the meeting does not currently enable the Agent meeting capability, or the required rollout gate is closed for the meeting owner.

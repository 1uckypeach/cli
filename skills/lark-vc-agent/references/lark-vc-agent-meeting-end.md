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

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

## Error Guidance

- `switch for allowing agents to join meetings is disabled`: the Calendar meeting did not enable the Agent meeting capability, or the owner rollout gate is closed.
- `no permission` before any meeting is created can mean the regular Calendar start permission chain denied creation, or that the target core service did not accept the Calendar Agent proof and treated the app bot as a regular meeting bot. If logs show `CreateConfig.DisableCreate=true` or `CreateOrJoinByID deny create for meeting bot`, this is not an Agent FG miss. Ask the Calendar owner, organizer, or another start-permitted user to start the meeting, then use `+meeting-join`; or adjust the Calendar meeting start permissions/proof deployment before retrying `+meeting-start`.
- `meeting not started`: the target Calendar meeting cannot currently be joined as an existing active meeting. Use `+meeting-start` for eligible Calendar VC meetings, or confirm the meeting is already active before using `+meeting-join`.

# vc
> skill: lark-meeting

## +search

### Skills
- lark-meeting/references/lark-vc-search.md

## +detail

### Skills
- lark-meeting/references/lark-vc-detail.md

## meeting get

## +recording

### Skills
- lark-meeting/references/lark-vc-recording.md

## +meeting-list-active

### Skills
- lark-meeting/references/lark-vc-meeting-list-active.md

## +meeting-events

### Skills
- lark-meeting/references/lark-vc-meeting-events.md

## +meeting-message-send

### Skills
- lark-meeting/references/lark-vc-meeting-message-send.md

## +meeting-end
Use this only when the user explicitly asks to end the entire ongoing meeting. This is a user-only, high-risk write: preview uncertain targets with `--dry-run`, and pass `--yes` only after explicit confirmation.

### Avoid when
- Only the application bot should leave the meeting → use [[+meeting-leave]].
- Only specific participants should be removed → use [[+meeting-participant-kickout]].

### Prerequisites
- Obtain the exact `meeting_id` for the target ongoing meeting from [[+meeting-list-active]] or [[meeting get]].

### Examples

**Preview ending the meeting without making the API call**
```bash
lark-cli vc +meeting-end --as user --meeting-id <meeting_id> --dry-run
```

### Skills
- lark-meeting/references/lark-vc-meeting-end.md

## +meeting-participant-kickout
Use this only when the user explicitly asks a host or cohost to remove specific participants. This is a user-only, high-risk write: preview uncertain targets with `--dry-run`, and pass `--yes` only after explicit confirmation.

### Avoid when
- The entire meeting should end → use [[+meeting-end]].
- Only the application bot should leave by itself → use [[+meeting-leave]].

### Prerequisites
- Obtain the exact `meeting_id` plus each participant tuple from the target meeting's participant snapshot via [[meeting get]]; do not infer `user_type` from an ID.
- Use the dedicated reference for tuple syntax, dry-run shape, and result interpretation instead of re-encoding that schema here.

### Examples

**Preview removing one participant without making the API call**
```bash
lark-cli vc +meeting-participant-kickout --as user --meeting-id <meeting_id> --participant '<participant_id>=<user_type>' --dry-run
```

### Skills
- lark-meeting/references/lark-vc-meeting-participant-kickout.md

## +meeting-join

### Skills
- lark-meeting/references/lark-vc-agent-meeting-join.md

## +meeting-leave

### Skills
- lark-meeting/references/lark-vc-agent-meeting-leave.md

# minutes
> skill: lark-minutes

## +todo
Write AI todos that live inside a minute — not Lark Task list items.

### Avoid when
- Personal or shared Task lists → use the task domain (`lark-cli task`), not this shortcut

### Prerequisites
- `minute_token` from [[+search]], a minutes URL, or VC recording
- For update/delete: `todo_id` from [[+detail]] with `--todo`

### Examples

**Add one unfinished todo**
```bash
lark-cli minutes +todo --minute-token obcnxxxxxxxxxxxxxxxxxxxx --operation add --todo "跟进预算审批" --is-done=false --as user
```

### Skills
- lark-minutes/references/lark-minutes-todo.md

## +word-replace
Batch-replace exact keywords in a minute transcript.

### Prerequisites
- Confirm each `source_word` against [[+detail]] with `--transcript` before replacing

### Examples

**Replace several keywords in one call**
```bash
lark-cli minutes +word-replace --minute-token obcnxxxxxxxxxxxxxxxxxxxx --replace-words '[{"source_word":"旧词","target_word":"新词"},{"source_word":"Foo","target_word":"Bar"}]' --as user
```

## +summary
Replace the minute's AI summary text in full.

### Prerequisites
- Prefer reading the current summary via [[+detail]] with `--summary` before overwriting

### Examples

**Replace the AI summary**
```bash
lark-cli minutes +summary --minute-token obcnxxxxxxxxxxxxxxxxxxxx --summary "**会议结论**\n- 方案 A 通过\n- 下周跟进排期" --as user
```

### Skills
- lark-minutes/references/lark-minutes-summary.md

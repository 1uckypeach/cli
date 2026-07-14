# Edit existing PPT: read-modify-write closed loop

For partial editing, use **shortcut [`+replace-slide`](lark-slides-replace-slide.md)** (block-level replacement/insertion), and use `xml_presentation.slide.get` to read the original page and get `block_id`. Rebuild multiple full pages of existing Slides using **[`+replace-pages`](lark-slides-replace-pages.md)**, keeping the original presentation link unchanged.

> **Must read** before generating XML [xml-schema-quick-ref.md](xml-schema-quick-ref.md).

## Decision tree: block_replace vs block_insert

| Requirements | Recommended actions | Reasons |
|------|------------|------|
| Knowing the `block_id` of a certain block, you need to replace the content of this block (change the title, change the picture, move the coordinates) | `block_replace` | Accurate replacement, good atomicity; `replacement` root `id` is automatically injected into `block_id` by CLI |
| Only add 1~N elements, leaving the existing layout unchanged | `block_insert` | Add new elements without overwriting, optional `insert_before_block_id` specifies the position |
| Move multiple elements at one time (e.g. change title + add picture) | Move multiple elements in a single `--parts` | The entire batch is treated as an atomic transaction, and the entire batch will not take effect if any one fails; `block_replace` and `block_insert` can be mixed |
| Multi-page layout reconstruction, whole-page coordinate rearrangement | `+replace-pages` | Batch create-before/delete-old in the original presentation, without generating new Slides links |

> **No field-level patch**: Even if you only want to change `topLeftX` of a `shape`, you have to write out the new XML of the entire block and use `block_replace`. This isn't "tweaking", it's a block-level rewrite.

## Minimum read-modify-write closed loop

```bash
PID="xml_presentation_id_here"
SID="slide_id_here"

# 1. Read the original page and pick out the 3-digit short id of the block to be changed from the XML (such as bUn / bab)
lark-cli slides xml_presentation.slide get --as user \
  --params "{\"xml_presentation_id\":\"$PID\",\"slide_id\":\"$SID\"}"

# 2. Use +replace-slide to directly change that block (no need to move the original XML)
lark-cli slides +replace-slide --as user \
  --presentation "$PID" --slide-id "$SID" \
--parts '[{"action":"block_replace","block_id":"bUn","replacement":"<shape type=\"text\" topLeftX=\"80\" topLeftY=\"80\" width=\"800\" height=\"120\"><content textType=\"title\"><p>New title</p></content></shape>"}]'
```

`slide_id` / The page order will not change. The `replacement` root element `id` of `block_replace` will be automatically injected as `block_id`, and users do not need to add it themselves when handwriting XML.

## `revision_id` parameter

`--revision-id` defaults to `-1`, which means execution based on the latest version. When passing a specific version number, the server uses this version as the base to apply the changes:

```bash
# Get the current revision_id when reading
REV=$(lark-cli slides xml_presentation.slide get --as user \
  --params "{\"xml_presentation_id\":\"$PID\",\"slide_id\":\"$SID\"}" \
  | jq '.data.revision_id')

# Pass the version number when writing, and the server will use this as the base
lark-cli slides +replace-slide --as user \
  --presentation "$PID" --slide-id "$SID" --revision-id "$REV" \
  --parts '[{"action":"block_replace","block_id":"bUn","replacement":"<shape type=\"rect\" topLeftX=\"100\" topLeftY=\"100\" width=\"200\" height=\"100\"/>"}]'
```

Note: Passing a version number that does not exist (exceeds the current revision) will return 3350002 not found; use `-1` when unsure.

## `--tid` transaction lock

The cross-request concurrent transaction ID is only useful when multiple people collaborate on long transactions. **Single person single call can be left blank**.

## Detailed explanation of two actions

### block_replace — Whole block replacement

Suitable for scenarios where "the block ID is known and the entire content of this block needs to be changed". The `id="<block_id>"` of the `replacement` root element is automatically injected by the CLI (if the user's handwritten XML does not contain `id`, it can be omitted directly; if it contains the wrong one, it will be overwritten with the correct value).

```bash
lark-cli slides +replace-slide --as user \
  --presentation "$PID" --slide-id "$SID" \
--parts '[{"action":"block_replace","block_id":"bab","replacement":"<shape type=\"text\" topLeftX=\"80\" topLeftY=\"80\" width=\"800\" height=\"120\"><content textType=\"title\"><p>New title</p></content></shape>"}]'
```

Field description:

| Field | Required | Description |
|------|------|------|
| `action` | Yes | Fixed to `block_replace` |
| `block_id` | Yes | The 3-digit short element ID of the target block (read from the XML returned by `slide.get`) |
| `replacement` | Yes | New XML fragment; the root element `id` will be automatically injected by the CLI as `block_id` |

### block_insert — Whole block insertion

Suitable for scenarios where "you only want to add one element and leave the existing elements unchanged" (typical: adding a picture to an existing page).

```bash
lark-cli slides +replace-slide --as user \
  --presentation "$PID" --slide-id "$SID" \
  --parts "$(jq -n --arg token "$FILE_TOKEN" \
    '[{action:"block_insert",insertion:("<img src=\""+$token+"\" topLeftX=\"500\" topLeftY=\"100\" width=\"200\" height=\"150\"/>"),insert_before_block_id:"baa"}]')"
```

Field description:

| Field | Required | Description |
|------|------|------|
| `action` | Yes | Fixed to `block_insert` |
| `insertion` | Yes | The complete XML fragment to insert |
| `insert_before_block_id` | No | Insert before this block; if omitted (this field is not provided), it will be appended to the end of the page |

> **`<img>` must use `file_token`**, and external link URL cannot be used - first `slides +media-upload --file ./pic.png --presentation $PID` to get the token.

### Batch parts

`--parts` can run up to 200 items at a time, executed serially in array order. `block_replace` and `block_insert` can be mixed in the same batch. Example: Replace the title block at once, and then add a decorative image at the end.

```bash
lark-cli slides +replace-slide --as user \
  --presentation "$PID" --slide-id "$SID" \
  --parts '[
{"action":"block_replace","block_id":"bab","replacement":"<shape type=\"text\" topLeftX=\"80\" topLeftY=\"80\" width=\"800\" height=\"120\"><content textType=\"title\"><p>New title</p></content></shape>"},
    {"action":"block_insert","insertion":"<img src=\"<file_token>\" topLeftX=\"700\" topLeftY=\"400\" width=\"180\" height=\"100\"/>"}
  ]'
```

The entire batch is treated as an atomic transaction: if any transaction fails, the entire batch will not take effect. The backend usually returns 3350001 when it fails; if the response contains the `failed_part_index` / `failed_reason` fields, shortcut will be transparently transmitted as is.

## Large --parts assembled with jq or stdin

`--parts` supports `@file` (reading files) and `-` (stdin) as value sources, suitable for batch XML scenarios:

```bash
# Read from file
lark-cli slides +replace-slide --as user --presentation "$PID" --slide-id "$SID" \
  --parts @parts.json

# Read from stdin
cat parts.json | lark-cli slides +replace-slide --as user --presentation "$PID" --slide-id "$SID" \
  --parts -
```

## Error troubleshooting

| Phenomenon | Cause | Countermeasures |
|------|------|------|
| 3350001, hint contains "block_id not found" | `parts[i].block_id` does not exist in the current page | Re-slide.get` to get the latest XML, press the short ID inside and fill in |
| 3350002 not found | `--revision-id` passed a version number that does not exist | Use `-1` or an actual `revision_id` |
| `<img>` does not display / displays broken images | `src` writes the external link URL | Replace with `file_token` obtained through `+media-upload` |
| 3350001 (returned by block_replace) | Under normal circumstances, CLI has automatically injected `id` and `<content/>`; if an error is still reported, confirm that `block_id` exists in the current page (retry `slide.get`), check whether the XML structure is legal; whether the coordinates exceed the 960×540 range | — |

## Related documents

- [lark-slides-replace-slide.md](lark-slides-replace-slide.md) — +replace-slide shortcut parameter details
- [lark-slides-replace-pages.md](lark-slides-replace-pages.md) — Multi-page full page reconstruction shortcut
- [lark-slides-xml-presentation-slide-get.md](lark-slides-xml-presentation-slide-get.md) — slide.get reference (get `block_id` / `revision_id`)
- [lark-slides-xml-presentation-slide-replace.md](lark-slides-xml-presentation-slide-replace.md) — Low-level replace API reference (generally use shortcut directly)
- [lark-slides-media-upload.md](lark-slides-media-upload.md) — Upload pictures and get file_token
- [xml-schema-quick-ref.md](xml-schema-quick-ref.md) — A quick look at XML elements and attributes

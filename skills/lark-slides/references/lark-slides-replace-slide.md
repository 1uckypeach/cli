# slides +replace-slide (Block-Level Replace / Insert)

> **Prerequisite:** First read [`../lark-shared/SKILL.md`](../../lark-shared/SKILL.md) for authentication, global parameters, and safety rules.

Performs block-level replacement or insertion on a specified slide. This is the primary path for editing an existing deck — `slide_id` stays the same, page order is untouched, and only the specified blocks are affected.

Compared to calling `xml_presentation.slide.replace` directly, this shortcut adds four extra benefits:

1. `--presentation` accepts an `xml_presentation_id` / `/slides/` URL / `/wiki/` URL (wiki URLs are resolved automatically);
2. For `block_replace`, the `replacement` root element's `id="<block_id>"` is injected automatically by the CLI — a hard constraint of the underlying API (missing it returns 3350001); when calling the native API directly you must add it yourself, with the shortcut it is injected automatically;
3. When a `<shape>` element is missing its `<content/>` child, the CLI injects it automatically — the SML 2.0 schema requires every `<shape>` to have a `<content/>` child, and a missing one also triggers 3350001; self-closing `<shape .../>` is likewise auto-expanded to `<shape ...><content/></shape>`;
4. On 3350001 errors it provides context-aware hints, helping AI agents and users quickly locate the cause.

## Command

```bash
# block_insert: append a new element at the end of the page
lark-cli slides +replace-slide --as user \
  --presentation slidesXXXXXXXXXXXXXXXXXXXXXX \
  --slide-id pfG \
  --parts '[{"action":"block_insert","insertion":"<shape type=\"rect\" topLeftX=\"500\" topLeftY=\"100\" width=\"200\" height=\"100\"/>"}]'

# block_replace: with a known block id, replace the whole block (replacement root id auto-injected as bUn)
lark-cli slides +replace-slide --as user \
  --presentation slidesXXXXXXXXXXXXXXXXXXXXXX \
  --slide-id pfG \
  --parts '[{"action":"block_replace","block_id":"bUn","replacement":"<shape type=\"text\" topLeftX=\"80\" topLeftY=\"80\" width=\"800\" height=\"120\"><content textType=\"title\"><p>New title</p></content></shape>"}]'

# Large --parts via file or stdin (auto-gen commands do not support @file, but shortcuts do)
lark-cli slides +replace-slide --as user \
  --presentation $PID --slide-id $SID --parts @parts.json
cat parts.json | lark-cli slides +replace-slide --as user \
  --presentation $PID --slide-id $SID --parts -

# Pass a wiki URL directly (CLI auto get_node -> resolves the real xml_presentation_id)
lark-cli slides +replace-slide --as user \
  --presentation "https://xxx.feishu.cn/wiki/wikcnXXXXXX" --slide-id pfG \
  --parts '[{"action":"block_insert","insertion":"<shape type=\"rect\" width=\"100\" height=\"100\"/>"}]'

# Preview (does not execute)
lark-cli slides +replace-slide --as user \
  --presentation $PID --slide-id $SID --parts "$PARTS" --dry-run
```

## Parameters

| Parameter | Required | Description |
|------|------|------|
| `--presentation` | Yes | `xml_presentation_id`, `/slides/<token>` URL, or `/wiki/<token>` URL |
| `--slide-id` | Yes | Slide ID (available from `xml_presentation.slide.get` / `slides +xml-get`) |
| `--parts` | Yes | JSON array (`[{...}, ...]`), at most 200 items per call. Supports `@<file>` and `-` (stdin) input |
| `--revision-id` | No | Base revision number; default `-1` means execute against the latest revision; when a specific revision is passed, the server executes with that revision as base; **passing a nonexistent revision (beyond the current one) returns 3350002** |
| `--tid` | No | Concurrent transaction ID; only for long multi-user collaborative transactions, leave empty for single one-off calls |

## parts Element Structure

> **Limits**: at most 200 items; `block_replace` and `block_insert` can be mixed in the same batch. **Any other action (including `str_replace`) is rejected outright by the CLI.**

Each part takes different fields depending on `action`:

### action = `block_replace`

| Field | Required | Description |
|------|------|------|
| `action` | Yes | `"block_replace"` |
| `block_id` | Yes | The target block's 3-character short element ID (read from the XML returned by `slide.get`) |
| `replacement` | Yes | New XML fragment; **the root element's `id` is auto-injected by the CLI as `block_id`** — you do not need to add it yourself (if you added one that differs, it is overwritten with the correct value) |

### action = `block_insert`

| Field | Required | Description |
|------|------|------|
| `action` | Yes | `"block_insert"` |
| `insertion` | Yes | The XML fragment to insert |
| `insert_before_block_id` | No | Insert before this block; when omitted (field not provided), append at the end of the page |

## Valid Root Element Cheat Sheet

`block_replace.replacement` and `block_insert.insertion` must be rooted at an element that SML 2.0 defines as valid. See [`slides_xml_schema_definition.xml`](slides_xml_schema_definition.xml) for the full authoritative definition; here we only list the types that can act as the **root**, plus a minimal working fragment for each type.

| Element | Purpose | Key points |
|---|---|---|
| `<shape>` | All shapes: rectangle/ellipse/triangle/text box, etc. | `type` is required; the CLI auto-injects `<content/>` when missing |
| `<line>` | Straight line | Requires `startX/startY/endX/endY` |
| `<polyline>` | Polyline | `points` is normalized away by the server on read-back (geometry already stored) |
| `<img>` | Image | `src` must be a `file_token` returned by [`+media-upload`](lark-slides-media-upload.md), not a URL |
| `<icon>` | Icon | `iconType` comes from iconpark assets; search semantic icons first with `scripts/iconpark_tool.py search` |
| `<table>` | Table | Replacing a whole table **rebuilds internal td ids**; old td block_ids become invalid immediately |
| `<td>` | Partial cell replacement | Only `block_replace`, not `block_insert`; `block_id` must be a td id from the latest `slide.get` |
| `<chart>` | Chart (line/bar/column/pie/area/radar/combo) | Must nest `<chartPlotArea>` + `<chartData>` + `<dim1>/<dim2>/<chartField>` |
| `<whiteboard>` | Whiteboard (SVG or Mermaid) | Embeds `<svg>` or `<mermaid>`; the structure returned by `slide.get` omits the internal data, but you can write a complete new XML directly as a `block_replace` overwrite; see [`lark-slides-whiteboard.md`](lark-slides-whiteboard.md) |

**Cannot be a root element**:

- `<video>` / `<audio>` — SML 2.0 has no such native elements; `<undefined type="video|audio">` is an **export-time** placeholder (used by the server when it encounters an unsupported type) and **cannot be written**. Attempting insert/replace returns 3350001.

### Minimal XML Fragments (remember to escape `"` as `\"` when embedding in JSON)

`<shape>` (text box; `type` can also be `rect`/`ellipse`/`triangle`/`custom`, etc.):
```xml
<shape type="text" topLeftX="80" topLeftY="80" width="800" height="120">
  <content textType="title"><p>Title</p></content>
</shape>
```

`<img>`:
```xml
<img src="{file_token}" topLeftX="600" topLeftY="20" width="80" height="80"/>
```

`<polyline>`:
```xml
<polyline topLeftX="10" topLeftY="10" width="100" height="50" points="0,0 50,50 100,0"/>
```

`<table>` (2x2):
```xml
<table topLeftX="30" topLeftY="80">
  <colgroup><col span="2" width="110"/></colgroup>
  <tr><td><content><p>A</p></content></td><td><content><p>B</p></content></td></tr>
  <tr><td><content><p>C</p></content></td><td><content><p>D</p></content></td></tr>
</table>
```

`<td>` (`block_replace` a single cell; `block_id` must be a td id from the latest `slide.get`):
```xml
<td><content><p>New content</p></content></td>
```

`<chart>` (change `type` to `bar`/`column`/`pie`/`area`/`radar`/`combo` to switch chart type):
```xml
<chart topLeftX="30" topLeftY="300" width="300" height="200">
  <chartPlotArea><chartPlot type="line"/></chartPlotArea>
  <chartData>
    <dim1><chartField name="x" valueType="string">Q1,Q2,Q3,Q4</chartField></dim1>
    <dim2><chartField name="Sales" valueType="number">10,20,15,30</chartField></dim2>
  </chartData>
</chart>
```

## Return Value

```json
{
  "xml_presentation_id": "slidesXXXXXXXXXXXXXXXXXXXXXX",
  "slide_id": "pfG",
  "parts_count": 1,
  "revision_id": 102
}
```

| Field | Description |
|------|------|
| `xml_presentation_id` | The resolved real token (changes after wiki URL resolution) |
| `slide_id` | Same as the input |
| `parts_count` | Number of parts submitted in this call |
| `revision_id` | The new revision number after success; use it for optimistic locking next time |
| `failed_part_index` | Present when some part failed; points to which part failed |
| `failed_reason` | Textual description of the failure reason |

The whole batch runs as an atomic transaction: if any part fails, the whole batch takes no effect, and the server tells you which one via `failed_part_index` / `failed_reason`; fix accordingly and resubmit.

## Usage Workflows

### Adding an Image to an Existing Page (Typical Scenario)

```bash
PID=xxx
SID=yyy

# 1) Upload the image
TOKEN=$(lark-cli slides +media-upload --as user \
  --file ./pic.png --presentation "$PID" | jq -r '.data.file_token')

# 2) block_insert at the end of the page
lark-cli slides +replace-slide --as user \
  --presentation "$PID" --slide-id "$SID" \
  --parts "$(jq -n --arg token "$TOKEN" \
    '[{action:"block_insert",insertion:("<img src=\""+$token+"\" topLeftX=\"500\" topLeftY=\"100\" width=\"200\" height=\"150\"/>")}]')"
```

### Changing a Title (block_replace)

```bash
# First fetch the page XML and find the title block's 3-character short id (e.g. bUn)
lark-cli slides xml_presentation.slide get --as user \
  --params "{\"xml_presentation_id\":\"$PID\",\"slide_id\":\"$SID\"}"

# block_replace the whole title block (id auto-injected)
lark-cli slides +replace-slide --as user \
  --presentation "$PID" --slide-id "$SID" \
  --parts '[{"action":"block_replace","block_id":"bUn","replacement":"<shape type=\"text\" topLeftX=\"80\" topLeftY=\"80\" width=\"800\" height=\"120\"><content textType=\"title\"><p>New title</p></content></shape>"}]'
```

### Batch: Replace a Title + Append a Decorative Image in One Call

`block_replace` and `block_insert` can be mixed in the same `--parts`; the whole batch executes atomically.

```bash
lark-cli slides +replace-slide --as user \
  --presentation "$PID" --slide-id "$SID" \
  --parts '[
    {"action":"block_replace","block_id":"bab","replacement":"<shape type=\"text\" topLeftX=\"80\" topLeftY=\"80\" width=\"800\" height=\"120\"><content textType=\"title\"><p>New title</p></content></shape>"},
    {"action":"block_insert","insertion":"<img src=\"<file_token>\" topLeftX=\"700\" topLeftY=\"400\" width=\"180\" height=\"100\"/>"}
  ]'
```

### Optimistic Locking

```bash
# Record revision_id at read time
REV=$(lark-cli slides xml_presentation.slide get --as user \
  --params "{\"xml_presentation_id\":\"$PID\",\"slide_id\":\"$SID\"}" \
  | jq '.data.revision_id')

# Pass --revision-id at write time; a nonexistent revision (beyond the current one) returns 3350002
lark-cli slides +replace-slide --as user \
  --presentation "$PID" --slide-id "$SID" --revision-id "$REV" \
  --parts "$PARTS"
```

## Common Errors

| Symptom | Cause | Fix |
|------|------|------|
| 3350001 + hint "block_id not found" | `parts[i].block_id` does not exist on the current page | Re-run `slide.get` for the latest XML and refill using the short IDs in it |
| 3350002 not found | `--revision-id` was a nonexistent revision (beyond the current one) | Use `-1` or a valid `revision_id` from `slide.get` |
| `--parts[i] action "str_replace" is not supported` | The CLI does not expose `str_replace` | Rewrite the replacement as `block_replace` / `block_insert` |
| `--parts contains N items, exceeds maximum of 200` | Too many parts in one call | Split into multiple calls |
| `--parts[i] (block_replace) requires non-empty block_id` / `replacement` | Missing fields | Fill in per the parts element structure |
| `<img>` not displayed / broken image | `src` was an external URL | Replace with a `file_token` obtained via [`+media-upload`](lark-slides-media-upload.md) |
| 3350001 | `replacement` is not a valid single-root XML fragment, or `block_id` does not exist | The CLI already auto-injects `id` and `<content/>`; if it still fails, re-run `slide.get` for the latest XML to confirm `block_id` exists; check that the XML structure is valid and coordinates do not exceed 960x540 |
| 403 | Insufficient permission | Requires `slides:presentation:update` or `slides:presentation:write_only`; wiki URLs additionally require `wiki:node:read` |

## Related Commands

- [xml_presentation.slide get](lark-slides-xml-presentation-slide-get.md) — read the original page for `block_id` / `revision_id`
- [xml_presentation.slide replace](lark-slides-xml-presentation-slide-replace.md) — reference for the underlying replace API
- [+media-upload](lark-slides-media-upload.md) — upload an image to get a `file_token`
- [lark-slides-edit-workflows.md](lark-slides-edit-workflows.md) — read-modify-write loop + decision tree

# lark-slides xml_presentation.slide replace

## Purpose

Performs **block-level partial replacement** on a single page: instead of overwriting the whole page, it applies a patch list of `block_replace` (replace a whole block) or `block_insert` (insert a whole block) operations. Suited to scenarios where you "only want to add / swap one element without touching the others".

> **Recommended**: prefer the [`+replace-slide`](lark-slides-replace-slide.md) shortcut — it automatically injects `id` and `<content/>`. Calling this API directly means handling those two constraints yourself (see Notes 5 and 6).

## Command

```bash
lark-cli slides xml_presentation.slide replace --as user --params '<json_params>' --data '<json_data>'
```

## Parameter Description

| Parameter | Type | Required | Description |
|------|------|------|------|
| `--params` | JSON string | Yes | Path and query parameters |
| `--data` | JSON string | Yes | Patch list |

### params JSON Structure

```json
{
  "xml_presentation_id": "slides_example_presentation_id",
  "slide_id": "slide_example_id",
  "revision_id": -1,
  "tid": "idMock"
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `xml_presentation_id` | string | Yes | Unique identifier of the presentation |
| `slide_id` | string | Yes | Unique identifier of the page |
| `revision_id` | integer | No | Defaults to `-1` (based on the latest revision); pass a specific revision number for optimistic locking |
| `tid` | string | No | Transaction ID, usually left empty |

### data JSON Structure

```json
{
  "parts": [
    { "action": "block_replace", "block_id": "bab", "replacement": "<shape .../>" },
    { "action": "block_insert", "insertion": "<img .../>", "insert_before_block_id": "baa" }
  ]
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `parts` | array | Yes | Patch list, length 1~200, executed in order |

### parts[] Fields (by action)

Two actions are documented for the CLI in this release:

#### action = "block_replace" — Replace a Whole Block

| Field | Required | Description |
|------|------|------|
| `action` | Yes | Fixed value `block_replace` |
| `block_id` | Yes | The 3-character short element ID of the target block (read from the XML returned by `slide.get`) |
| `replacement` | Yes | New XML fragment that replaces the entire target block |

#### action = "block_insert" — Insert a Whole Block

| Field | Required | Description |
|------|------|------|
| `action` | Yes | Fixed value `block_insert` |
| `insertion` | Yes | The complete XML fragment to insert |
| `insert_before_block_id` | No | Insert before this block; if omitted, appends to the end of the page |

## Usage Examples

### block_replace: Replace the Entire Content of a Shape

```bash
lark-cli slides xml_presentation.slide replace --as user --params '{
  "xml_presentation_id": "slides_example_presentation_id",
  "slide_id": "slide_example_id"
}' --data '{
  "parts": [
    {
      "action": "block_replace",
      "block_id": "bab",
      "replacement": "<shape type=\"text\" topLeftX=\"80\" topLeftY=\"80\" width=\"800\" height=\"120\"><content textType=\"title\"><p>New Title</p></content></shape>"
    }
  ]
}'
```

### block_insert: Add an Image to an Existing Page

```bash
# Get the file_token first
TOKEN=$(lark-cli slides +media-upload --file ./pic.png --presentation "$PID" --as user | jq -r '.data.file_token')

lark-cli slides xml_presentation.slide replace --as user --params "{
  \"xml_presentation_id\": \"$PID\",
  \"slide_id\": \"$SID\"
}" --data "$(jq -n --arg token "$TOKEN" '{
  parts: [
    {
      action: "block_insert",
      insertion: ("<img src=\"" + $token + "\" topLeftX=\"500\" topLeftY=\"100\" width=\"200\" height=\"150\"/>")
    }
  ]
}')"
```

### Multiple parts Executed Atomically

```bash
lark-cli slides xml_presentation.slide replace --as user --params '{
  "xml_presentation_id": "slides_example_presentation_id",
  "slide_id": "slide_example_id"
}' --data '{
  "parts": [
    {"action":"block_replace","block_id":"bab","replacement":"<shape type=\"text\" topLeftX=\"80\" topLeftY=\"80\" width=\"800\" height=\"120\"><content textType=\"title\"><p>New Title</p></content></shape>"},
    {"action":"block_insert","insertion":"<img src=\"<file_token>\" topLeftX=\"700\" topLeftY=\"400\" width=\"180\" height=\"100\"/>"}
  ]
}'
```

## Return Value

### Success

```json
{
  "ok": true,
  "identity": "user",
  "data": {
    "revision_id": 105
  }
}
```

### Failure (if any part fails, the whole batch does not take effect)

On failure, the command exits with a non-zero exit code, and stderr returns a typed error envelope (`error.code` (e.g. 3350001) / `error.message` / `error.hint`); stdout does not print the raw backend response:

```json
{
  "ok": false,
  "identity": "user",
  "error": {
    "type": "api",
    "subtype": "...",
    "code": 3350001,
    "message": "...",
    "hint": "..."
  }
}
```

| Field | Type | Description |
|------|------|------|
| `data.revision_id` | integer | On success, returns the updated latest revision number |

## Common Errors

| Error Code | Meaning | Solution |
|--------|------|----------|
| 3350001 | `block_id` does not exist on the current page, or the XML format / structure is invalid | Re-run `slide.get` to fetch the latest XML and confirm the `block_id` exists; check that `replacement` / `insertion` is valid XML |
| 400 | `parts` length exceeds 200 | Split into multiple calls |
| 3350002 | `revision_id` does not exist (exceeds the current revision number) | Use `-1` or an actually existing `revision_id` |
| 400 | Invalid XML format | `replacement` / `insertion` must be valid XML fragments, with closed tags and quoted attributes |
| 403 | Insufficient permissions | Requires `slides:presentation:update` or `slides:presentation:write_only` |

## Notes

1. **parts is an atomic transaction**: if any single part fails, the whole batch is rolled back; there is no intermediate state where "the first few succeed and the rest fail".
2. **Getting block_id**: in the XML returned by `slide.get`, every block (shape, img, table, chart, whiteboard, etc.) carries a 3-character short element ID; use that value for `block_id` / `insert_before_block_id`.
3. **`<img>` must use a file_token**: external URLs are not allowed — get a token first via [`slides +media-upload`](lark-slides-media-upload.md).
4. **No field-level patching**: to change one attribute of a block (e.g. only `topLeftX`), you must write the whole block's new XML and use `block_replace`; the API does not support "changing just one field".
5. **`block_replace` requires the root element of `replacement` to carry `id="<block_id>"`**: this is a hard constraint of the underlying API; omitting it returns 3350001. The recommended path is the [`+replace-slide`](lark-slides-replace-slide.md) shortcut — it automatically injects the `id` into the root element of `replacement`, so users do not have to add it when writing XML.
6. **`<shape>` must have a `<content/>` child element**: required by the SML 2.0 schema; omitting it also triggers 3350001. The [`+replace-slide`](lark-slides-replace-slide.md) shortcut injects `<content/>` automatically; calling the underlying API directly requires adding it yourself.
7. **The returned `<whiteboard>` structure does not include internal data**: whiteboard blocks in the XML returned by `slide.get` only have the outer tag and position attributes; the SVG / Mermaid content is not returned with the XML. However, `block_replace` can still forcibly overwrite it — just write the complete new whiteboard XML.
8. **Always do this before executing**: run `lark-cli schema slides.xml_presentation.slide.replace` to check the latest parameter structure.

## Related Commands

- [slides +replace-slide](lark-slides-replace-slide.md) — block-level replacement shortcut (recommended, auto-injects id)
- [xml_presentation.slide get](lark-slides-xml-presentation-slide-get.md) — read the original page to get block short IDs
- [slides +media-upload](lark-slides-media-upload.md) — upload images to get a file_token
- [lark-slides-edit-workflows.md](lark-slides-edit-workflows.md) — read-modify-write loop + decision tree

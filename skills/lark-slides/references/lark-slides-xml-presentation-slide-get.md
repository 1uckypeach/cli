# lark-slides xml_presentation.slide get

## Purpose

Fetch the XML content of a single slide in a presentation by `slide_id` (a historical revision can be specified). Commonly used as the first step of the "read-modify-write" editing loop.

## Command

```bash
lark-cli slides xml_presentation.slide get --as user --params '<json_params>'
```

## Parameter Description

| Parameter | Type | Required | Description |
|------|------|------|------|
| `--params` | JSON string | Yes | Path and query parameters |

### params JSON Structure

```json
{
  "xml_presentation_id": "slides_example_presentation_id",
  "slide_id": "slide_example_id",
  "revision_id": -1
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `xml_presentation_id` | string | Yes | Unique identifier of the target presentation |
| `slide_id` | string | Yes | Unique identifier of the target slide |
| `revision_id` | integer | No | Revision number, `-1` means the latest revision (default) |

## Usage Examples

### Read the Latest Revision

```bash
lark-cli slides xml_presentation.slide get --as user --params '{
  "xml_presentation_id": "slides_example_presentation_id",
  "slide_id": "slide_example_id"
}'
```

### Extract Only the XML Content

```bash
lark-cli slides xml_presentation.slide get --as user \
  --params '{"xml_presentation_id":"slides_example_presentation_id","slide_id":"slide_example_id"}' \
  | jq -r '.data.slide.content'
```

### Read a Specific Historical Revision

```bash
lark-cli slides xml_presentation.slide get --as user --params '{
  "xml_presentation_id": "slides_example_presentation_id",
  "slide_id": "slide_example_id",
  "revision_id": 42
}'
```

## Return Value

```json
{
  "ok": true,
  "identity": "user",
  "data": {
    "slide": {
      "slide_id": "slide_example_id",
      "content": "<slide id=\"slide_example_id\"><style/><data>...</data></slide>"
    },
    "revision_id": 100
  }
}
```

| Field | Type | Description |
|------|------|------|
| `data.slide.slide_id` | string | Unique identifier of the slide |
| `data.slide.content` | string | Full slide XML (`<slide>` root node, without xmlns) |
| `data.revision_id` | integer | Revision number returned by this read; can be used as an optimistic lock for a subsequent replace |

## Common Errors

| Error Code | Meaning | Solution |
|--------|------|----------|
| 404 | Presentation or slide does not exist | Check `xml_presentation_id` / `slide_id` |
| 403 | Insufficient permissions | Requires the `slides:presentation:read` scope and access permission to the PPT |
| 400 | `revision_id` does not exist | An invalid revision number was passed; use `-1` or a revision number that actually exists |

## Notes

1. **Do this before executing**: run `lark-cli schema slides.xml_presentation.slide.get` to check the latest parameter structure
2. **block_id extraction**: in the returned XML, the `id` attribute of each top-level block (shape, img, table, chart, whiteboard, etc.) is the `block_id`, usually a 3-character short code, e.g. `<shape id="bUn" ...>`. Use the following command to list all block_ids on the current slide:

   ```bash
   lark-cli slides xml_presentation.slide get --as user \
     --params "{\"xml_presentation_id\":\"$PID\",\"slide_id\":\"$SID\"}" \
     | jq -r '.data.slide.content' | grep -oE 'id="[^"]+"' | sed 's/id="//;s/"//'
   ```

## Related Commands

- [slides +replace-slide](lark-slides-replace-slide.md) — Block-level replace shortcut (recommended)
- [xml_presentation.slide replace](lark-slides-xml-presentation-slide-replace.md) — Underlying replace API reference
- [slides +xml-get](lark-slides-xml-get.md) — Read the whole PPT and save it to a local file
- [lark-slides-edit-workflows.md](lark-slides-edit-workflows.md) — Read-modify-write loop

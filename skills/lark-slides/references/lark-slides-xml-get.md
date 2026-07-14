# slides +xml-get (Read XML)

Read the full XML of an existing presentation, or read a single slide's XML by `slide_id` / slide number. Suitable for post-creation acceptance checks, backups before editing, obtaining `slide_id` / `revision_id`, and troubleshooting blank pages, broken images, or text overflow. Compared to calling the underlying `xml_presentations.get` / `xml_presentation.slide.get` directly, this shortcut automatically resolves Slides URLs / Wiki URLs and can save the XML to a local file, avoiding truncated terminal output.

## Command


```bash
lark-cli slides +xml-get \
  --as user \
  --presentation <slides_url_or_xml_presentation_id> \
  --output .lark-slides/plan/<deck-id>/readback.xml
```

## Parameters

| Parameter | Required | Description |
|------|------|------|
| `--presentation` | Yes | `xml_presentation_id`, `/slides/` URL, or `/wiki/` URL |
| `--output` | No | Local path to save the XML; must be a relative path inside the current working directory, absolute paths are not allowed. When provided, the XML content is saved to the file and stdout only returns brief metadata such as the saved absolute path and size; when omitted, a JSON envelope is returned by default |
| `--slide-id` | No | Slide short ID; when provided, only that slide's XML is read. Cannot be used together with `--slide-number` |
| `--slide-number` | No | 1-based slide number; when provided, only that slide's XML is read. Cannot be used together with `--slide-id` |
| `--revision-id` | No | Read a specific revision; defaults to `-1`, meaning the latest revision |
| `--remove-attr-id` | No | Full-document reads only. Removes the `id` attributes from the returned XML; suitable for read-only inspection, not for precise block-level editing |
| `--raw` | No | When `--output` is omitted, write the raw XML directly to stdout without a JSON envelope. Cannot be used together with `--output` / `--jq` / a non-json `--format` |
| `--dry-run` | No | Preview the API to be called and the output mode without reading the actual XML |

## Output to a File

For normal workflows, passing `--output` is recommended, especially for medium and large decks. `--output` must be a relative path inside the current working directory, for example `.lark-slides/plan/$PID/readback.xml`; do not pass absolute paths like `/tmp/readback.xml`. The XML is written to the local file and stdout only keeps the metadata, which makes it easy for subsequent scripts to read.

```bash
lark-cli slides +xml-get --as user \
  --presentation "$PID" \
  --output .lark-slides/plan/$PID/readback.xml
```

The `data` in a successful output looks like:

```json
{
  "xml_presentation_id": "slides_example_presentation_id",
  "path": "/abs/path/.lark-slides/plan/slides_example_presentation_id/readback.xml",
  "size": 123456,
  "content_saved": true,
  "revision_id": 12
}
```

Here `path` is the absolute path resolved by the CLI.

If `--remove-attr-id` is passed, the returned metadata includes `"remove_attr_id": true`.

## Reading a Single Slide

When you know the slide's short ID, use `--slide-id`:

```bash
lark-cli slides +xml-get --as user \
  --presentation "$PID" \
  --slide-id "$SID" \
  --output .lark-slides/plan/$PID/slide-$SID.xml
```

When you know the slide number, use `--slide-number` (numbers start at 1):

```bash
lark-cli slides +xml-get --as user \
  --presentation "$PID" \
  --slide-number 2 \
  --output .lark-slides/plan/$PID/slide-2.xml
```

Single-slide mode calls `xml_presentation.slide.get` under the hood, and returns or saves a single `<slide>` XML fragment. `--slide-id` and `--slide-number` cannot be passed together; `--remove-attr-id` only supports full-document reads.

## Output to the Terminal

When `--output` is omitted, the CLI outputs a JSON envelope by default, with the XML located at `data.xml_presentation.content` (full document) or `data.slide.content` (single slide). This mode is suitable for ad-hoc extraction with `--jq`:

```bash
lark-cli slides +xml-get --as user \
  --presentation "$PID" \
  --jq '.data.xml_presentation.content'
```

To write the raw XML directly to stdout, add `--raw`:

```bash
lark-cli slides +xml-get --as user \
  --presentation "$PID" \
  --slide-number 2 \
  --raw
```

## Related Commands

- [slides +screenshot](lark-slides-screenshot.md) - Take slide screenshots for visual verification
- [slides +replace-slide](lark-slides-replace-slide.md) - Partially replace or insert slide elements
- [slides +replace-pages](lark-slides-replace-pages.md) - Rebuild multiple slides as whole pages
- [xml_presentations get](lark-slides-xml-presentations-get.md) - Underlying native API reference

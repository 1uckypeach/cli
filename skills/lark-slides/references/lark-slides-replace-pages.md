# slides +replace-pages (Multi-Page Full Rebuild)

Replaces multiple pages of an existing presentation in batch, keeping the original `xml_presentation_id` and the original Slides link unchanged. Suited to large multi-page layout overhauls, coordinate rework, and full-page visual rebuilds; for local edits to a single text box, image, or shape, still prefer [`+replace-slide`](lark-slides-replace-slide.md).

> Important: this is multi-step orchestration, not an atomic backend transaction. For each page the CLI performs "create the new page before the old page, then delete the old page"; if creation fails, the old page is kept. If deletion fails, the new and old pages may coexist, and you need to continue handling them according to the returned results.

## Command

```bash
lark-cli slides +replace-pages \
  --as user \
  --presentation <slides_url_or_xml_presentation_id> \
  --pages @pages.json
```

## Parameters

| Parameter | Required | Description |
|------|------|------|
| `--presentation` | Yes | `xml_presentation_id`, a `/slides/` URL, or a `/wiki/` URL |
| `--pages` | Yes | JSON array, each item containing `slide_id` and `content`; supports literal, `@file`, and stdin `-` |
| `--dry-run` | No | Outputs the replacement plan based on the `slide_id` input, without executing create/delete |
| `--continue-on-error` | No | Stops on failure by default; when enabled, continues with subsequent pages and marks failed items in the result |
| `--validate-only` | No | Only validates the input and generates the replacement plan, without executing Slides get/create/delete |

## pages.json

```json
[
  {
    "slide_id": "slide_short_id_1",
    "content": "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\"><data></data></slide>"
  },
  {
    "slide_id": "slide_short_id_2",
    "content": "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\"><data></data></slide>"
  }
]
```

Rules:

- Every item must provide `slide_id`; `slide_number` is not supported.
- `content` must be complete `<slide>...</slide>` XML.
- No duplicate `slide_id` within the same batch.
- The CLI does not read back the whole presentation; if a `slide_id` has become invalid, the create/delete phase returns the corresponding error.

## Dry Run

```bash
lark-cli slides +replace-pages --as user \
  --presentation "$PID" \
  --pages @pages.json \
  --dry-run
```

The output includes `xml_presentation_id`, `pages_count`, `plan`, and for each page its `old_slide_id`, `insert_before_slide_id`, and the action `create_before_then_delete_old`. Dry-run builds the plan purely from the input `slide_id` values; it neither calls `xml_presentations.get` nor executes create/delete.

## Success Output

```json
{
  "xml_presentation_id": "xxx",
  "pages_count": 2,
  "status": "completed",
  "summary": {
    "replaced": 2,
    "failed": 0,
    "total": 2
  },
  "results": [
    {
      "old_slide_id": "old3",
      "new_slide_id": "new3",
      "status": "replaced"
    }
  ],
  "revision_id": 123
}
```

If `--continue-on-error` is used and any page fails, the CLI continues with the remaining pages but ultimately exits non-zero with a partial failure; stdout still contains the complete `results`, with top-level `ok` set to `false` and `status` set to `partial_failure`.

`status` can be:

- `replaced`: the new page was created successfully and the old page was deleted successfully.
- `create_failed`: creating the new page failed; the old page is kept.
- `delete_failed`: the new page was created, but deleting the old page failed.

## Usage Tips

1. Before a large rewrite, save the current XML with `slides +xml-get` and record the `slide_id` of the pages to replace.
2. After generating a `pages.json` containing only the target `slide_id` entries, run `--dry-run` or `--validate-only` first.
3. Do not enable `--continue-on-error` by default, unless you can accept some pages having already been replaced.
4. After replacement, read back the full XML and take screenshots to confirm the page order, visuals, and text are intact.

# slides +screenshot

## Purpose

Takes screenshots of slide pages and saves them as local image files. By default it screenshots pages of an existing deck; when `--content` is passed, it directly renders a single `<slide>` XML fragment for preview. This shortcut decodes and writes the files inside the CLI process; stdout only returns metadata such as file path, size, and slide ID, avoiding sending image Base64 to the model.

Note: this screenshot capability is gated by an application allowlist, and the vast majority of applications cannot use it. If a screenshot fails, just record the error; do not steer the user toward requesting the `slides:presentation:screenshot` permission. Then follow `validation-checklist.md` for non-screenshot validation, and do not claim that screenshot-based acceptance was completed.

## Command

```bash
lark-cli slides +screenshot --as user \
  --presentation '<xml_presentation_id or slides/wiki URL>' \
  --slide-number 1
```

Render local XML content:

```bash
lark-cli slides +screenshot --as user \
  --content @slide.xml
```

## Parameters

| Parameter | Required | Description |
|------|------|------|
| `--presentation` | Required in list mode | `xml_presentation_id`, `/slides/` URL, or a `/wiki/` URL that resolves to slides. Cannot be used when `--content` is passed |
| `--slide-id` | List mode requires at least one of `--slide-id` / `--slide-number` | Slide short ID; repeat the flag for multiple pages; at most 10 pages per call (`--slide-id` + `--slide-number` combined at most 10) |
| `--slide-number` | List mode requires at least one of `--slide-id` / `--slide-number` | Slide page number; repeat the flag for multiple pages; at most 10 pages per call (`--slide-id` + `--slide-number` combined at most 10) |
| `--content` | Required in render mode | The `<slide>` XML fragment to render directly; supports a literal value, `@file`, or `-` stdin. When passed, `--slide-id` / `--slide-number` cannot be used at the same time |
| `--output-dir` | No | Output directory, default `.lark-slides/screenshots`; must be a relative path inside the current directory |
| `--output-name` | No | Output file name stem in render mode; when unspecified, the returned `slide_id` is preferred, otherwise `rendered-slide`. If the target file already exists, an incrementing suffix is appended automatically to avoid overwriting |

## Examples

### Single-Page Screenshot

```bash
lark-cli slides +screenshot --as user \
  --presentation slides_example_presentation_id \
  --slide-number 1
```

### Multi-Page Screenshot

Do not exceed 10 pages per call; for more pages, call in batches.

```bash
lark-cli slides +screenshot --as user \
  --presentation slides_example_presentation_id \
  --slide-number 1 \
  --slide-number 2 \
  --output-dir .lark-slides/screenshots/demo
```

### Rendering an XML Preview

```bash
lark-cli slides +screenshot --as user \
  --content @.lark-slides/out/demo/slide.xml \
  --output-name preview
```

## Return Value

The returned JSON does not include Base64 image content:

```json
{
  "ok": true,
  "identity": "user",
  "data": {
    "xml_presentation_id": "slides_example_presentation_id",
    "output_dir": ".lark-slides/screenshots",
    "screenshots": [
      {
        "slide_id": "slide_example_id",
        "slide_number": 1,
        "format": "png",
        "path": "/abs/path/.lark-slides/screenshots/slides_example_presentation_id_p001_slide_example_id.png",
        "size": 12345
      }
    ]
  }
}
```

## Notes

1. Prefer `slides +screenshot` to save local images; never dump image Base64 to stdout.
2. When screenshotting pages of an existing deck, do not pass `--content`; use `--presentation` + `--slide-id` or `--slide-number`.
3. For local XML preview, pass `--content @file` or `--content -`; the content should be a single `<slide>` XML fragment; in this case do not pass `--presentation` / `--slide-id` / `--slide-number`.
4. `slide_id` is the slide's short ID; for page numbers use `--slide-number`.
5. List mode accepts at most 10 pages per call (`--slide-id` + `--slide-number` combined at most 10); screenshot more pages in batches.
6. In list mode the default file name contains the presentation ID, page number, and/or slide ID; when the file already exists, a `_2`, `_3`, etc. suffix is appended automatically to avoid overwriting old screenshots.
7. Screenshots come from the server-side rendering result, which is suitable for verifying after create/replace whether a page is blank, has broken images, or is obviously mis-laid-out.

# Validation Checklist

After creating or substantially rewriting a presentation, you must perform one explicit validation. The goal is to catch blank pages, broken XML, truncated content, obvious overflow, weak visual hierarchy, and unverified output.

Small edits to existing pages also require scope-appropriate validation: at minimum, read the modified page or the full XML and confirm the target element was updated without breaking the surrounding structure.

## Required Flow

1. Record the `xml_presentation_id` returned by the create or edit call, plus any known `slide_id` / `revision_id`.
2. Read back the full XML to a local file with `slides +xml-get`.
3. Check that the actual page count matches the plan or the user's request.
4. Check that each page's `<data>` contains the expected primary elements.
5. Check that there are no obvious blank pages, broken pages, missing titles, or missing primary visuals.
6. Check that pages have not all degenerated into title + bullet list.
7. Check visual hierarchy: title, primary visual, and supporting information are distinguishable.
8. Check obvious overflow and layout risks: overlap, out-of-bounds, bottom crowding, long text boxes.
9. Include a short validation record in the final reply.

Read-back command:

```bash
lark-cli slides +xml-get --as user \
  --presentation "YOUR_ID" \
  --output .lark-slides/plan/<deck-or-task-id>/readback.xml \
  --json
```

## Automated XML Text Overlap Lint

Before submission, local XML must go through the XML-syntax and text-overlap static check:

```bash
python3 skills/lark-slides/scripts/xml_text_overlap_lint.py --input <presentation-or-slide.xml>
```

Pass criteria:

- `summary.error_count == 0`. Any error must be fixed before calling the API.
- The current tool checks XML well-formedness, SXSD tag/attr support, IconPark icon types and icon fill visibility, obvious overlap between text elements, and suspicious boundary overlap between whiteboard containers and external sibling elements; it does not check out-of-bounds, insufficient text height, text-over-image coverage, table/chart coverage, or bottom crowding.
- The tool does not replace page-count checks, key-content checks, or real visual acceptance.

Handling directions for common codes:

| code | Meaning | Fix |
|------|------|----------|
| `xml_not_well_formed` | XML syntax error or unescaped text | Fix tag closure, attribute quoting, `&` / `<` / `>` escaping |
| `sml_prefixed_tag` | An SML element uses a namespace prefix, e.g. `<ns0:slide>` or `<sml:shape>` | Use the default namespace of `<slide xmlns="http://www.larkoffice.com/sml/2.0">`, or unprefixed tags |
| `sxsd_unsupported_tag` | A tag not supported by SXSD was used | Replace with a supported tag per the lint `hint`; common cases: `textbox -> <shape type="text">`, `image -> <img>` |
| `sxsd_unsupported_attr` | An unsupported attribute was used on a supported tag | Change to a supported attribute per the lint `hint`; common cases: `x -> topLeftX`, `fontColor -> color` |
| `iconpark_unsupported_icon_type` | `<icon>` uses an `iconType` that does not exist in `iconpark-index.json` | Change to an allowlisted `iconType` per the lint `hint`, or search first with `scripts/iconpark_tool.py` |
| `icon_missing_fill_color` | The visual spec requires `<icon>` to set `<fill><fillColor color="..."/></fill>` to keep the icon visible | Add an explicit non-transparent fill color to `<icon>`, e.g. `rgba(37, 99, 235, 1)` |
| `icon_transparent_fill_color` | The `<icon>` `fillColor` is transparent and fails the visual visibility requirement | Change to a non-transparent color with sufficient contrast against the background |
| `bbox_overlap` | The estimated draw areas of text elements clearly overlap | Spread out text coordinates, shrink text boxes/font sizes, or switch to explicit column/group structures |
| `whiteboard_external_overlap` | The whiteboard container bbox crosses boundaries with an external sibling element | Shrink or move the whiteboard / external element per the lint `hint`; if you accept the risk, final acceptance must rely on screenshot QA or equivalent rendered visual inspection |

## Page Count And Structure

- The actual page count must equal the user's request or the count in `slide_plan.json`.
- If the creation process partially failed, record the created `xml_presentation_id` first, then read back to confirm which pages were written.
- Every page should contain `<data>`, and `<data>` must have at least one non-background primary element.
- Cover, section, and summary pages may carry little text, but must not be an empty background only.
- Technical explanation, comparison, flow, and architecture pages must have matching structural elements, e.g. grouped boxes, connectors, timelines, tables, or graphical areas.

## Expected Elements

Verify page by page against `slide_plan.json` and the user's requirements:

- The title or main conclusion exists and corresponds to `key_message`.
- The primary structure implied by `layout_type` was generated.
- `visual_focus` is one of the most prominent or largest information areas on the page.
- `text_density` actually influenced the amount of text; a long bullet box did not replace the plan.
- When `asset_need` has a real asset, it is placed in the correct area; without a real asset, `fallback_if_missing` provided an XML fallback of shapes, lines, labels, tables, or charts.

If the user specified key pages, e.g. "architecture explanation", "Self-Attention mechanism explanation", "comparison or evolution perspective", "summary page", the final validation record must state item by item that those pages exist.

## Blank Or Broken Page Signals

Treat the following as must-fix before delivery:

- `<data/>` is empty, or contains only background, decorative lines, or empty `<content/>`.
- Key text does not appear in the read-back XML.
- Images are still `@./path`, or `<img src>` is an http(s) external link.
- An image area the page depends on is empty with no fallback visual.
- The returned XML is missing pages, the page order is clearly wrong, or a page's content was truncated by the shell.
- Many shapes share identical coordinates, causing the primary content to overlap.
- A gradient background fell back to blank or white, making text unreadable.

## Whiteboard Elements

When reading back XML with `slide.get`, `<whiteboard>` blocks return only position attributes (`topLeftX`, `topLeftY`, `width`, `height`); SVG / Mermaid content is **not returned with the XML**.

- Whiteboard validation can verify coordinates stay in bounds: `topLeftX + width ≤ 960`, `topLeftY + height ≤ 540`; the lint also reports suspicious boundary overlap between whiteboard containers and external sibling elements.
- The correctness of SVG and Mermaid content cannot be verified via XML read-back; it requires human visual acceptance.
- Do not claim in the validation record that whiteboard content was verified unless the user confirmed the visual result.

## Layout And Overflow Risk

Prioritize fixing these obvious risks:

- Body or label boxes too short, so text is likely truncated.
- Multiple primary elements overlapping in the same area, rather than intentional background layering.
- Important content crossing the canvas boundary, or hugging the bottom beyond `y=500`.
- A high-density page using one long bullet list without columns, tables, or grouping.
- Font size and color differences between title, primary visual, and body too weak, making hierarchy unclear.
- Every content page using the same title + bullets coordinates.

## Verification Record

The final reply must include a short validation record; suggested format:

```text
Validation record:
- Read-back: ran slides +xml-get, actual pages N / expected N.
- Key pages: architecture explanation / Self-Attention / comparison or evolution / summary page all present.
- Structure: checked primary shape/img/table/chart elements; no obvious blank or broken pages.
- Layout: checked title hierarchy, primary visual, overlap/out-of-bounds/text-overflow risks.
```

Do not claim human visual acceptance was completed unless you actually opened or obtained a rendered result. Conclusions drawn only from XML static checks should be phrased as "static checks found no obvious issues".

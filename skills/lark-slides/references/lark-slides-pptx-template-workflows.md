# PPT Template Rewrite Principles

This page only governs the scenario where the user designates a PPT template, base deck, or existing PPTX/PDF/Slides file and asks for derivative work based on it. Core principle: the template is not a style reference; it is the editing base that must be carried forward.

## Import First

When the user designates a PPT template, first import the template as Lark Slides. All subsequent writes target the imported Slides — do not create a new deck detached from the template, and do not redraw a PPTX locally first and then import it.

Use the following command directly; there is no need to load the `lark-drive` skill first:

```bash
lark-cli drive +import --as user --file "<template.pptx>" --type slides --json
```

Optional parameters: use `--name "<title>"` to set the title of the imported Slides; use `--folder-token <FOLDER_TOKEN>` to specify the target folder. If the response returns `ready=false` / `timed_out=true`, run the `next_command` from the response directly; the equivalent form is:

```bash
lark-cli drive +task_result --scenario import --ticket <TICKET>
```

After importing, you must read back the Slides content and understand each page's real layout, fonts, hierarchy, images, charts, shapes, tables, and text containers. The readback result is the source of truth for the template rewrite.

## Read Before Editing

Before editing any slide page, you must read that page first.

If the page content is not in the current context, you must re-read the page; "current context" here does not include the System Prompt. Never edit a specific page based only on memory, the file name, a thumbnail impression, or a judgment about the template's overall style.

When reading a page, determine at least:

- The role the page originally plays, such as cover, section divider, agenda, process, comparison, data, or summary.
- The page's main layout structure, such as image-text relationships, arrows, timelines, nodes, tables, charts, side-by-side comparisons, background images, or product images.
- Which text boxes, shape labels, table cells, or chart labels carry the content.
- The original page's fonts, font sizes, colors, alignment, hierarchy, and whitespace relationships.

## Edit The Imported Slides Directly

After understanding a page, edit the imported Slides directly. Allowed operations include:

- Filling in, replacing, condensing, or deleting text.
- Replacing or adding images.
- Updating the content of charts, tables, number labels, or node labels.
- Copying, deleting, or reordering template pages as needed.
- Making local, small-scale element additions when the source page has no suitable container.

New elements may only fill content gaps; they must not become a new primary layout. The body of the page should still be carried by the template's original layout.

## Preserve Design

A template rewrite must strictly follow the original layout and fonts: change only the content, do no redesign.

Preserve by default:

- Page layout, visual hierarchy, whitespace, and alignment relationships.
- Original fonts, font-size system, colors, text box positions, and shape order.
- Background images, images, logos, charts, tables, decorative shapes, lines, icons, and page structure.
- The differences between different page types within the template.

Do not remodel template pages into uniform generic cards, whiteboards, title bars, three-column layouts, 2x2 card grids, or large overlay masks. Do not treat the template as a background image and start a separate design system on top of it.

## Content Only

Content must go first into the page's existing text boxes, shape labels, nodes, table cells, chart labels, or annotation containers.

If an original container lacks space, prefer to:

- Condense the text.
- Reduce the font size while keeping the original font system.
- Split the content into adjacent containers that already exist on the page.
- Use the template's existing annotation, label, or supplementary note regions.
- Copy the style of a native container from the same page or the same template for a local addition.

Do not redraw the main structure of a page to fit long copy. Do not cover the original charts, arrows, images, background, or key shapes with newly added large cards.

## Readback And Tune

After editing, you must read back the result and fine-tune page by page.

During readback, focus on:

- Whether text overflows, is truncated, presses against edges, or exceeds its container.
- Whether text covers images, charts, shapes, arrows, nodes, or other text.
- Whether shape ordering causes content to be overwritten or hidden.
- Whether new content still lands within the template's original layout rather than covering the template structure.
- Whether fonts, font sizes, colors, alignment, and hierarchy still stay close to the original page.

When text overflows, prefer condensing the text or reducing the font size. When occlusion is found, fix it by adjusting shape order, making local position tweaks, or reusing existing empty regions. Only when none of these methods can express the content should you make local additions or deletions.

The completion criterion for a template rewrite is not "a new deck that looks uniform was generated" but "the original template's layout, fonts, and visual structure are still clearly present, the content has been accurately replaced, and readback shows no overflow or occlusion."

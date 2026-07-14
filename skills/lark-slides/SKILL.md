---
name: lark-slides
version: 1.0.0
description: "Feishu Slides: create and edit slides. Create presentations, read slide content, manage slide pages (create, delete, read, partial replace). Use when the user needs to create or edit slides, or read or modify a single page. When the user provides a doubao.com /slides/ URL/token, also use this skill directly; do not fall back to WebFetch just because the domain is not Feishu — routing is based on the URL path pattern and token, not the domain. Not responsible for: cloud document content editing (use lark-doc), standalone whiteboard objects inside cloud documents (use lark-whiteboard; note that flowcharts/architecture diagrams embedded in a slide still belong to this skill), uploading or downloading regular files (use lark-drive)."
metadata:
  requires:
    bins: ["lark-cli"]
  cliHelp: "lark-cli slides --help"
---

# slides (v1)

**CRITICAL — Global hard constraint: the PPT canvas is 960x540; keep all primary content inside the page bounds.**

**CRITICAL — Images matter: deliberately and proactively use plenty of images! Use image-generation and image-search tools for asset images, and generate supporting images when assets are missing; background images must come from an image-generation tool, and the generation prompt must explicitly require that no text appear.**

**CRITICAL — Prevent text overflow: every `<content>` that carries highlighted information or dense text must set `autoFit="normal-auto-fit"` so the font size auto-shrinks within the box to avoid overflow.**

## Quick Reference

| User need | Preferred action | Key docs / commands |
|----------|----------|-----------------|
| Create a new PPT | Plan `slide_plan.json` first, then choose one-step or two-step creation by complexity | `planning-layer.md`, `visual-planning.md`, `asset-planning.md`, `slides +create` |
| Create from a template or edit an existing local PPTX | Import the PPTX as Slides | `lark-slides-pptx-template-workflows.md` |
| Edit a single title, text block, image, or local element | Prefer block-level replace/insert; do not change page order | `slides +replace-slide`, `lark-slides-replace-slide.md` |
| Read or analyze an existing PPT | Resolve the slides/wiki token, use the shortcut to read back the full XML or a single page's XML, save `xml_presentation_id`, `slide_id`, `revision_id` | `slides +xml-get`, `xml_presentation.slide.get` |
| Capture slide page screenshots | Specify pages by `slide_id` or page number, no more than 10 pages per call | `slides +screenshot`, `lark-slides-screenshot.md` |
| Upload or use images | Upload first to get a `file_token`; never write http(s) external links directly | `slides +media-upload`, or the `@./path` placeholder of `+create --slides` |
| Draw charts | Use `<chart>` for native charts, `<shape>` + `<line>` for others; only complex Mermaid/SVG goes into `<whiteboard>` | `xml-schema-quick-ref.md`, `slides_chart_demo.xml` |
| Draw tables | Prefer simulating with `rect` and `text`; use `<table>` for the rest | `xml-schema-quick-ref.md` |
| Use icons | Never guess `iconType` blindly — search IconPark first, then write `<icon iconType="...">`; icons must have a fill color with sufficient contrast against the background; emoji icons are forbidden | `iconpark_tool.py search → resolve`, `iconpark.md` |
| Creation failure, blank pages, 3350001, broken layout | Read back the current state first, then fix per the troubleshooting checklist; never assume the original operation succeeded atomically | `troubleshooting.md`, `validation-checklist.md` |

**CRITICAL — Before starting, you MUST use the Read tool to read [`../lark-shared/SKILL.md`](../lark-shared/SKILL.md); authentication, permissions, and global parameters all follow lark-shared.**

**CRITICAL — Before generating any XML, you MUST use the Read tool to read [xml-schema-quick-ref.md](references/xml-schema-quick-ref.md); guessing XML structure from memory is forbidden.**

**CRITICAL — When creating a new presentation or substantially rewriting pages, you MUST first generate `.lark-slides/plan/<deck-or-task-id>/slide_plan.json`, then generate the XML. Create the directory first; planning-layer rules and intermediate artifact lifecycle are in [planning-layer.md](references/planning-layer.md). Small edits to existing pages, such as replacing one title or inserting one block, are exempt.**

**CRITICAL — When creating a new presentation or substantially rewriting pages, you MUST read [visual-planning.md](references/visual-planning.md) before generating XML, ensuring `layout_type`, `visual_focus`, and `text_density` actually change the page geometry, primary visual, and amount of text.**

**CRITICAL — When creating a new presentation or substantially rewriting pages, planning `asset_need` MUST follow [asset-planning.md](references/asset-planning.md).**

**CRITICAL — Before submitting a complete `<slide>` XML to `slides +create --slides`, `xml_presentation.slide create`, or `slides +replace-pages`, you MUST first save the XML to a local file and run [`scripts/xml_text_overlap_lint.py`](scripts/xml_text_overlap_lint.py); `summary.error_count` must be 0 before calling the API.**

**CRITICAL — After creating or substantially rewriting, you MUST perform explicit validation per [validation-checklist.md](references/validation-checklist.md): read back the full XML, verify page count and key elements, and check for blank/broken pages, obvious overflow, and layout risks.**

**CRITICAL — For pre-creation self-checks or failure troubleshooting, you MUST follow [troubleshooting.md](references/troubleshooting.md) to check XML escaping, structure, shell truncation, image tokens, 3350001, and layout risks.**

**Editing existing slide pages**: for a single title, text block, image, or local element, prefer [`+replace-slide`](references/lark-slides-replace-slide.md) (block-level replace/insert, page order untouched); for large multi-page changes to existing Slides, prefer [`+replace-pages`](references/lark-slides-replace-pages.md) to rebuild pages in bulk within the original presentation, avoiding a new link from `slides +create`. See [`lark-slides-edit-workflows.md`](references/lark-slides-edit-workflows.md) for choosing an action and the full read-modify-write flow.

## Identity Selection

Feishu slides are usually the user's own content resources. **By default, explicitly prefer `--as user` (user identity) for slides operations**, and always specify the identity explicitly.

- **`--as user` (recommended)**: create, read, and manage presentations as the currently logged-in user. Complete user authorization first:

```bash
lark-cli auth login --domain slides
```

- **`--as bot`**: only when the user explicitly asks to operate as the app identity, or when the bot needs to own/create the resource. When using the bot identity, additionally confirm that the bot actually has access to the target presentation.

**Execution rules**:

1. Creating, reading, adding/deleting slides, and continuing to edit an existing PPT from a user-provided link all default to `--as user` first.
2. On insufficient permission, first check whether the bot identity was used by mistake; do not fall back to bot by default.
3. Only switch to `--as bot` when the user explicitly asks to "operate as the app / bot identity", or when the current workflow is bot-creates-resource-then-grants-collaboration.

## Required Reading Before Execution

> **Important**: `references/slides_xml_schema_definition.xml` is the only authoritative XML protocol source for this skill; the other md files are merely summaries of it and of the CLI schema.

High-frequency, read-only:

- [xml-schema-quick-ref.md](references/xml-schema-quick-ref.md)
- [planning-layer.md](references/planning-layer.md) (new deck / substantial rewrite)
- [visual-planning.md](references/visual-planning.md) (new deck / substantial rewrite)
- [asset-planning.md](references/asset-planning.md) (new deck / substantial rewrite)
- [validation-checklist.md](references/validation-checklist.md) (after creation / substantial rewrite)

Read on demand:

- Creation: [`lark-slides-create.md`](references/lark-slides-create.md)
- Editing: [`lark-slides-edit-workflows.md`](references/lark-slides-edit-workflows.md), [`lark-slides-replace-slide.md`](references/lark-slides-replace-slide.md), [`lark-slides-replace-pages.md`](references/lark-slides-replace-pages.md)
- Screenshots: [`lark-slides-screenshot.md`](references/lark-slides-screenshot.md)
- Images: [`lark-slides-media-upload.md`](references/lark-slides-media-upload.md)
- Icons: [`iconpark.md`](references/iconpark.md), [`scripts/iconpark_tool.py`](scripts/iconpark_tool.py)
- Troubleshooting: [`troubleshooting.md`](references/troubleshooting.md)
- Full protocol: [`slides_xml_schema_definition.xml`](references/slides_xml_schema_definition.xml)

## Workflow

> **This is a presentation, not a document.** Each slide is an independent visual canvas; keep information density appropriate and leave whitespace in the layout.

### Design Ideas

Do not generate slides with no sense of design. Plain white background + title + bullets is only acceptable as a minimal interim draft, never as a formal deliverable.

Before writing any XML, settle the deck-level visual strategy in `slide_plan.json`:

- **Theme-driven palette**: the palette must serve this deck's topic, industry, and audience — do not default to a blue corporate look. If the same palette would work equally well for a completely different topic, it is not specific enough.
- **Primary/secondary ratio**: pick 1 primary color carrying roughly 60-70% of the visual weight, 1-2 secondary colors for structure and sectioning, and 1 accent color used only for key numbers, conclusions, or action points. Do not give all colors equal weight.
- **Background consistency**: decide the whole deck's background strategy first; by default keep the same light/dark tone and base color system. Only section dividers, transitions, or emphasis pages may deliberately change the background, and the change must still read as the same design via shared primary colors, textures, sidebars, or motifs. Whether light or dark, ensure sufficient contrast for body text, icons, and lines.
- **Unified motif**: choose one reusable visual motif that runs through the deck, e.g. a thick sidebar, circular icon bases, half-bleed image areas, numbered nodes, a corner color block on cards, or oversized numerals. Do not switch decorative languages on every page.

Every page needs at least one visual element: an image, icon, chart, table, flow, comparison structure, big number, schematic, or an abstract visual composed of shapes. A text box by itself does not count as a primary visual.

Prefer these page forms:

- **Two-column structure**: text left / visual right or vice versa, with the visual area taking 35-45% of the width.
- **Icon rows**: icons on color blocks or circular bases, with a short title and one-line explanation to the right.
- **2x2 / 2x3 grids**: good for capabilities, modules, risks, action items; keep every cell at the same hierarchy level.
- **Half-bleed visuals**: an image or abstract shape occupies the left/right half of the screen, with text overlaid or edge-aligned.
- **Big-number cards**: key metrics in 60-72pt numerals with 10-14pt labels underneath.
- **Comparison columns**: before/after, option A/B, problem/solution side by side, with titles and baselines strictly aligned.
- **Timeline/flow diagrams**: express steps with nodes and arrows; the flow direction must be obvious at a glance.

Typography and spacing suggestions:

- Titles 36-44pt, key conclusions can be larger; body text 14-18pt; annotations 10-12pt.
- Body text defaults to left-aligned; use centering only on covers, closing pages, or big-number scenes.
- Page margins at least 40px; keep 24-40px spacing between content blocks, consistent within the same deck.
- Card padding must leave real space — do not let text touch the edges; account for text-box padding when aligning shapes and text.

Common mistakes to avoid:

- Do not reuse the same title + three-bullets layout on every page.
- Do not use low-contrast text or icons, e.g. light gray text on a light background.
- Do not let decorative lines cut through text, or let footers, sources, or page numbers crowd the main content.
- Do not represent missing assets as empty image boxes; you must render an XML-native visual per `fallback_if_missing`.
- Do not leave template placeholder copy, sample company names, sample dates, or original template content unrelated to the user's topic.
- Do not use emoji.
- Do not stack more than 3 shapes solely to mimic a concrete object.

### Choosing a Creation Method

| Scenario | Recommended approach |
|------|----------|
| Simple XML (1-3 pages, simple structure, almost no complex Chinese or special characters) | One-step creation with `slides +create --slides '[...]'` |
| Complex XML (many pages, Chinese text, long passages, complex layouts, nested quotes, many special characters) | **Two-step creation**: `slides +create` an empty PPT first, then add pages one by one with `xml_presentation.slide create` |
| Appending or inserting pages into an existing PPT | Use `xml_presentation.slide create`, with `before_slide_id` when needed |

> [!WARNING]
> The risk of `--slides '[...]'` lies mainly in shell argument passing, not page count alone. Even a single page warrants two-step creation if its XML is complex enough.

> [!IMPORTANT]
> Under the hood, `slides +create --slides` creates pages one by one — it is not atomic. On mid-way failure, record the `xml_presentation_id` first, read back to confirm the current state, then continue fixing or appending.

### Generation Flow

```text
Step 1: Clarify requirements & read knowledge
  - Clarify topic, audience, page count, style; if the user uploads a PPTX as a template, follow the "user custom template" rules at the top
  - Read xml-schema-quick-ref.md; for new decks / substantial rewrites also read planning-layer.md, visual-planning.md, asset-planning.md

Step 2: Generate outline → user confirmation → write slide_plan.json
  - Generate a structured outline for the user to confirm
  - New decks / substantial rewrites must first create the directory and write `.lark-slides/plan/<deck-or-task-id>/slide_plan.json`
  - Plan fields, path naming, and the `asset_need` structure follow planning-layer.md / asset-planning.md

Step 3: Generate XML per slide_plan.json → create
  - Consume the plan page by page: key_message sets the main conclusion, layout_type sets geometry, visual_focus sets the primary visual, text_density sets text amount
  - When real assets are missing, use `fallback_if_missing` to render an XML-native fallback visual; never leave blanks
  - Before calling create or full-page replace APIs, save the pending XML and run xml_text_overlap_lint.py; a nonzero error_count must be fixed first
  - Choose the creation method per "Choosing a Creation Method"; handle images, complex XML, escaping, and 3350001 per lark-slides-create.md, media-upload.md, troubleshooting.md

Step 4: Review & deliver
  - After creation, you must read the full XML with `slides +xml-get` and record explicit validation per validation-checklist.md
  - Handle failure or partial success per troubleshooting.md; fix local issues preferably with `+replace-slide`
  - All good → deliver: tell the user the presentation ID and how to access it
```

### jq Command Templates (for editing an existing PPT)

For new PPTs, prefer `+create --slides`. The jq templates below apply when appending pages to an existing presentation, avoiding manual double-quote escaping:

```bash
# Append to the end
lark-cli slides xml_presentation.slide create \
  --as user \
  --params '{"xml_presentation_id":"YOUR_ID"}' \
  --data "$(jq -n --arg content '<slide xmlns="http://www.larkoffice.com/sml/2.0">
  <style><fill><fillColor color="BACKGROUND_COLOR"/></fill></style>
  <data>
    <!-- Place shape, line, table, chart, and other elements here -->
  </data>
</slide>' '{slide:{content:$content}}')"

# Insert before a specific page: before_slide_id must go in the --data body, as a sibling of slide
# WARNING: do not put before_slide_id in --params -- the CLI silently sends it as an unknown query param, the server ignores it, and the new page lands at the end
lark-cli slides xml_presentation.slide create \
  --as user \
  --params '{"xml_presentation_id":"YOUR_ID"}' \
  --data "$(jq -n --arg content '<slide ...>...</slide>' --arg before 'TARGET_SLIDE_ID' \
    '{slide:{content:$content}, before_slide_id:$before}')"
```

> Gradients must use the `rgba()` format with percentage stops, e.g. `linear-gradient(135deg,rgba(15,23,42,1) 0%,rgba(56,97,140,1) 100%)`. Using `rgb()` or omitting stops causes the server to fall back to white.

### Outline Template

Use the following format when generating the outline for user confirmation:

```text
[PPT title] -- [positioning statement], targeted at [target audience]

Page structure (N pages):
1. Cover page: [title copy]
2. [Page topic]: [point 1], [point 2], [point 3]
3. [Page topic]: [point description]
...
N. Closing page: [closing copy]

Style: [color scheme], [layout style]
```

## Core Concepts

### URL Formats and Tokens

| URL format | Example | Token type | Handling |
|----------|------|-----------|----------|
| `/slides/` | `https://example.larkoffice.com/slides/xxxxxxxxxxxxx` | `xml_presentation_id` | Use the token in the URL path directly as `xml_presentation_id` |
| `/wiki/` | `https://example.larkoffice.com/wiki/wikcnxxxxxxxxx` | `wiki_token` | ⚠️ **Cannot be used directly** — query first to obtain the real `obj_token` |

> The `+replace-slide` and `+media-upload` shortcuts automatically resolve both URL forms; you still need to resolve wiki links manually when calling native APIs directly.

### Special Handling for Wiki Links (critical!)

A knowledge-base link (`/wiki/TOKEN`) cannot be used directly as an `xml_presentation_id`. Before calling native APIs directly, query the wiki node, confirm `node.obj_type == "slides"`, then use `node.obj_token` as the real presentation ID.

```bash
lark-cli wiki spaces get_node --as user --params '{"token":"wiki_token"}'
```

Shortcuts `+replace-slide` and `+media-upload` resolve `/wiki/` URLs automatically; only manual calls to `xml_presentations.*` / `xml_presentation.slide.*` require this step.

### Resource Relationships

```text
Wiki Space (knowledge space)
└── Wiki Node (knowledge-base node, obj_type: slides)
    └── obj_token → xml_presentation_id

Slides (presentation)
├── xml_presentation_id (unique presentation identifier)
├── revision_id (revision number)
└── Slide (slide page)
    └── slide_id (unique page identifier)
```

## Shortcuts and APIs

Shortcuts are high-level wrappers for common operations (`lark-cli slides +<verb> [flags]`). Prefer shortcuts whenever one exists for the operation.

| Shortcut | Description |
|----------|------|
| [`+create`](references/lark-slides-create.md) | Create a PPT (optionally add pages in one step with `--slides`; supports `<img src="@./local.png">` placeholders with automatic upload) |
| [`+xml-get`](references/lark-slides-xml-get.md) | Read the full or single-page XML, optionally saving to a local file to avoid terminal output truncation |
| [`+media-upload`](references/lark-slides-media-upload.md) | Upload a local image to a specific presentation and return a `file_token` (used as `<img src="...">`), max 20 MB |
| [`+replace-slide`](references/lark-slides-replace-slide.md) | Block-level replace/insert on an existing slide page (`block_replace` / `block_insert`); auto-injects ids and `<content/>` without changing page order |
| [`+replace-pages`](references/lark-slides-replace-pages.md) | Bulk-rebuild multiple pages within the original presentation: create new pages before the old ones, then delete the old ones; suited for large multi-page changes to existing Slides without creating a new link |

Use native APIs when no shortcut covers the operation. High-frequency resources: `slides +xml-get` reads the full document; `xml_presentation.slide.create/delete/get/replace` manage single pages.

```bash
lark-cli schema slides.<resource>.<method>   # Must inspect the parameter structure before calling an API
lark-cli slides <resource> <method> [flags] # Call the API
```

> **Important**: when using native APIs, you must run `schema` first to inspect the `--data` / `--params` structure; do not guess field formats.

## Core Rules

1. **Plan before writing XML**: when creating a new presentation or substantially rewriting pages, you must first write `.lark-slides/plan/<deck-or-task-id>/slide_plan.json`; templates, styles, and outlines are only planning inputs and cannot bypass the planning layer
2. **Creation flow**: simple short XML (1-3 pages, simple structure, few special characters) can use one-step `slides +create --slides '[...]'`; for complex content — images, long Chinese passages, nested quotes, many special characters — or more than 10 pages, default to `slides +create` for an empty PPT first, then add pages one by one with `xml_presentation.slide.create`
3. **The only direct children of `<slide>` are `<style>`, `<data>`, and `<note>`**: text and graphics must live inside `<data>`
4. **Text is expressed through `<content>`**: it must be `<content><p>...</p></content>`; text cannot be written directly inside a shape
5. **Save key IDs**: later operations need `xml_presentation_id`, `slide_id`, `revision_id`
6. **Delete with caution**: deletion is irreversible, and at least one slide must remain
7. **Prefer in-place updates when editing existing pages**: to modify a single shape/img use `+replace-slide` (`block_replace` / `block_insert`), do not rebuild whole pages; for multi-page full rebuilds of existing Slides use `+replace-pages`, do not create a whole new PPT with `slides +create`; only special single-page full-page operations not covered by any shortcut warrant manual `slide.create` + `slide.delete`
8. **`<img src>` may only use a `file_token` uploaded to Feishu drive; http(s) external URLs are forbidden**: the Feishu slides renderer does not proxy external images, so external `src` values usually render as missing or broken images in the PPT. The flow must be "save the image locally first → upload via `slides +media-upload` or the auto-uploading `@./path` placeholder of `+create --slides` → write the `file_token` into `<img src>`". If the user provides a web image link, first `curl`/download it into the CWD and then go through the upload flow; never put the external URL into `src` directly. **Max image size 20 MB** (the slides upload API does not support multipart upload).

> **Note**: if any md content conflicts with `slides_xml_schema_definition.xml` or the output of `lark-cli schema slides.<resource>.<method>`, the latter two take precedence.

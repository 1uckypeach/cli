---
name: lark-slides
version: 1.0.0
description: "飞书幻灯片：创建、读取和编辑 Slides；支持从计划到 XML、图片/图标/图表、局部替换、真实截图验收与视觉基线评测。用户要求新建或修改飞书幻灯片、给出 /slides/ 或 /wiki/ 链接（包括 doubao.com 对应路径）、需要 PPT 版式、图片、图表、流程图、截图验证或视觉质量对标时使用。不负责云文档正文编辑、独立画板或普通文件上传下载。"
metadata:
  requires:
    bins: ["lark-cli"]
  cliHelp: "lark-cli slides --help"
---

# Lark Slides

Create structured, editable presentations—not raster slide images. The page canvas is **960×540**.

## Rule Priority

When rules conflict, use this order:

1. User intent and supplied content.
2. `references/slides_xml_schema_definition.xml` and `lark-cli schema` for protocol/API truth.
3. This workflow's structural, safety, and quality gates.
4. Page-specific plan and asset strategy.
5. Visual preferences and examples.

Do not claim SOTA from a local render or a single VLM judgment.

## Non-Negotiable Output Contract

- Keep slide-authored titles, body text, labels, data, page numbers, and navigation as native XML elements. Evidence assets may contain inherited text, but it must not carry the slide's title or key conclusion; add a native annotation/caption when needed. Set `autoFit="normal-auto-fit"` on prominent or dense text.
- Use `--as user` unless the user explicitly requests bot identity.
- Use `<img>` only with a Drive `file_token` or a creation-time `@./local-path` placeholder. Never use an external image URL.
- Use a **no-text** image asset for material-rich, editorial, product, or atmospheric visuals. Keep all slide copy outside that image.
- Do not use a full-page generated raster as the final deck. It is a benchmark/reference artifact only.
- Use a meaningful native chart, semantic diagram, screenshot, or high-value asset as each formal page's visual focus. Small icons, repeated cards, or decorative shape stacks do not satisfy this alone.
- Run static diagnostics before writes. Resolve every error and review every warning.
- For every new or materially rebuilt page: validate static XML; content-render it when it has no local `@` image placeholder; then create/update the real presentation and inspect a live screenshot when screenshot service is available. A local placeholder page uses static → live-create → live screenshot as its equivalent path.

## Choose the Visual Route

| Page need | Use |
|---|---|
| Material, scene, product, editorial abstraction, cover atmosphere | Generate/find one **no-text** asset; reserve 35–45% or an intentional full-bleed region; compose editable XML around it. |
| Process, architecture, comparison, data explanation | Native `chart`, semantic `icon`, `whiteboard` SVG/Mermaid, shapes, and connectors. Make the diagram itself the visual focus. |
| Existing screenshot, figure, or user asset | Upload it, crop deliberately, add concise native annotations, and retain a fallback plan. |
| Asset unavailable | Render the explicit `fallback_if_missing`; never leave an empty image frame or replace it with dense bullets. |

Do not try to mimic a photorealistic object by stacking many shapes. Use an asset when material fidelity is the point; use SVG/shape diagrams when semantics and editability are the point.

## Required Reads

Read these before acting:

| Situation | Read |
|---|---|
| Always | `../lark-shared/SKILL.md`, `references/xml-schema-quick-ref.md` |
| New deck or major rebuild | `references/planning-layer.md`, `references/visual-planning.md`, `references/asset-planning.md` |
| Asset-first or raw-image visual comparison | `references/visual-ceiling.md` |
| Create/replace/read media | the matching `lark-slides-*.md` reference from the table below |
| Validation | `references/validation-checklist.md`, `references/quality-loop.md` |
| Benchmark/SOTA request | `references/external-evaluation.md` |

## Workflow

### 1. Decide scope and route

- Existing presentation, small edit: read the target page and use block-level replacement.
- Existing presentation, multi-page rebuild: use `+replace-pages` to preserve the original link.
- New deck or major rewrite: create `.lark-slides/plan/<task-id>/slide_plan.json` first.

The plan must name the audience, page role, `layout_type`, `key_message`, `visual_focus`, `text_density`, `asset_need`, visual system, and fallback for every planned asset.

### 2. Make the visual system explicit

Choose one background strategy, one reusable motif, primary/secondary/accent color roles, and typography limits. Vary page geometry by page role; do not vary decorative language arbitrarily.

Before XML, verify that each page answers:

1. What is the dominant visual and why does it clarify the key message?
2. Is it large enough to be noticed before supporting copy?
3. Is all user-facing copy native and editable?
4. Is the layout visibly different from other page roles?

### 3. Build assets and XML

- For generated images, request no text and reserve the intended copy region in the prompt. Place it with `<img src="@./path">` only in `+create --slides`, or upload first and use the returned token.
- For icons, run `scripts/iconpark_tool.py search` before choosing `iconType`; set an explicit visible fill.
- Use native `<chart>` for supported data charts. Use `<whiteboard>` only for complex SVG/Mermaid diagrams.
- Keep non-background content inside the canvas and maintain readable margins, contrast, and text fit.
- Save XML locally before submission and run `scripts/xml_text_overlap_lint.py`. Do not call the API while errors remain.

### 4. Create or edit

- Use `slides +create --as user --slides` for simple new pages, including creation-time local image upload.
- For complex XML, Chinese-heavy content, or difficult escaping: create the presentation, inspect the API schema, then add pages through `xml_presentation.slide create` with `jq`-built payloads.
- For a local edit, prefer `+replace-slide`; for a multi-page rewrite, prefer `+replace-pages`.
- After any partial failure, read current state before retrying. Do not assume creation was atomic.

### 5. Verify live output

1. Run static diagnostics and retain the report.
2. Render changed XML to PNG when supported.
3. Read back presentation XML; confirm page count, image tokens, key text, and primary elements.
4. Screenshot every changed live page; repair clipping, weak contrast, bad crop, or weak visual focus.

### 6. Evaluate visual ceiling when requested

For raw-image comparison, use the same brief:

- raw baseline: one raster page per slide;
- candidate: asset-first structured XML with native text/layout;
- compare as a simultaneous contact sheet or balanced A/B pair, never only as a sequential image list;
- record visual polish, hierarchy, consistency, text fidelity, and editability separately.

A local tie or win supports only that declared comparison. SOTA requires a public/declared task set, named baselines, repeated position-balanced evaluation, raw artifacts, and preferably human preference evidence.

## Reference Map

| Need | Reference |
|---|---|
| Planning / page geometry / assets | `planning-layer.md`, `visual-planning.md`, `asset-planning.md` |
| Image/media workflow | `lark-slides-media-upload.md`, `lark-slides-create.md` |
| Editing and replacement | `lark-slides-edit-workflows.md`, `lark-slides-replace-slide.md`, `lark-slides-replace-pages.md` |
| Readback and screenshots | `lark-slides-xml-get.md`, `lark-slides-screenshot.md` |
| XML protocol | `references/xml-schema-quick-ref.md`, `references/slides_xml_schema_definition.xml` |
| Icons | `iconpark.md`, `scripts/iconpark_tool.py` |
| Diagnostics and recovery | `validation-checklist.md`, `quality-loop.md`, `troubleshooting.md` |
| Visual ceiling and claims | `visual-ceiling.md`, `external-evaluation.md` |

## Delivery Evidence

Return the presentation URL/ID, changed page IDs, static report status, live screenshot locations, and any unresolved tradeoffs. Do not call a deck visually accepted merely because the create API returned success.

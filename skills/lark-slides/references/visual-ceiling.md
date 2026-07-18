# Visual Ceiling Rules

Use this reference when the deck must approach or exceed a strong image-generated visual baseline. The target is not a flatter XML imitation of a raster slide. The target is a structured deck that keeps image-level visual focus while retaining native text, layout, and editing controls.

## Golden Rules: Verifiable Form

| Rule | Pass condition | Failure signal |
|---|---|---|
| Visual focus is real | Every page has one dominant focus tied to its key message: image, chart, diagram, semantic SVG, large metric, or intentional text-led statement | Tiny icons or repeated cards are the only visual objects |
| Asset has a job | Each planned asset clarifies the page key message, not just decorates it | Removing the asset changes nothing about the slide meaning |
| Images are text-free | Generated backgrounds/focal art contain no slide copy, labels, or page numbers | Copy is baked into an image and cannot be edited reliably |
| XML owns structure | Title, body, labels, data, and page navigation are native XML elements | A full-slide `<img>` is the final deliverable |
| Layout varies deliberately | Page roles result in distinct geometry and rhythm | Cover, comparison, and process pages are all title plus cards |
| Render proof decides | Static proof is mandatory; use content render when supported and live screenshot when available | XML validity is used as a proxy for visual quality |

## Asset-First, Structure-First Composition

1. In `slide_plan.json`, name the `visual_focus` and `asset_need` before writing XML.
2. Use a real or generated **no-text** asset for material-rich imagery, cover atmosphere, product context, or editorial abstraction. Prompt the asset with a reserved copy region; do not ask the image model to render the title.
3. Put the asset in a deliberate 35–45% (or intentional full-bleed) region. Keep slide copy and labels as native `<shape type="text">` elements.
4. When an image does not clarify the message, use one semantic SVG/whiteboard, native chart, or diagram at visual-focus scale. Do not stack many small shapes solely to imitate photorealistic depth.
5. Reuse palette, edge treatment, crop logic, and one motif across the deck. Vary page geometry—not the visual language—between page roles.

## Raw-Image Baseline Protocol

A raw image model may be used as a visual ceiling reference, but never as the final structured deck.

- Build the raw baseline from the same slide brief, one full-page raster per page.
- Build the Skill candidate with the same message, but native text and layout plus optional no-text assets.
- Compare with a simultaneous contact sheet or balanced A/B presentation. Do not judge one deck only after another: sequential VLM inputs can favor the most recently shown deck.
- Record visual polish, information hierarchy, cross-slide consistency, text fidelity, and editability separately.
- A local tie means the visual ceiling is approached; it is not SOTA. Require repeated wins across a declared task set and named baselines before making a stronger claim.

## Preflight Gate

Before creation, answer `yes` to all of these:

1. Is the dominant visual large enough to be noticed before the supporting text?
2. Is that visual either a meaningful asset or a semantic diagram, rather than decorative filler?
3. Are all user-facing words editable native text rather than pixels?
4. Does this page's geometry visibly differ from the other page roles in the deck?
5. If `asset_need` is not `none`, does the plan specify an XML-native fallback if the asset is unavailable?
6. Will the evaluation use live screenshots when available and a position-balanced comparison if a baseline is involved?

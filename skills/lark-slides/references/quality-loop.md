# Quality Loop

Use this loop for every new deck and every page whose layout is materially rebuilt. XML validity is necessary, not visual acceptance.

## Contract

1. Write `slide_plan.json` and one `<slide>` XML file per planned page.
2. Run the static gate. Resolve every `error`; explicitly review every `warning`.
3. Content-render each changed page when it has no local `@` image placeholder. For a local placeholder, use static validation then live creation, because content render cannot upload that image.
4. Review available rendered/live PNGs against the five dimensions below. Record the exact XML repair for every score below 2.
5. Create or update the real presentation, read back XML, and screenshot the live page when the screenshot service is available. If screenshots are unavailable, retain the error plus XML/readback evidence and do not claim screenshot acceptance.
6. Re-run the same checks after a repair. Do not claim visual acceptance from XML alone.

## Commands

All paths must be relative to the repository working directory.

```bash
# Local XML diagnostics only; no network or write.
python3 skills/lark-slides/eval/run.py \
  --input .lark-slides/plan/<deck>/slide-01.xml --mode static

# Server-side render of XML; no presentation is created.
python3 skills/lark-slides/eval/run.py \
  --input .lark-slides/plan/<deck>/slide-01.xml --mode render

# Explicitly create a presentation and screenshot its actual rendered page.
python3 skills/lark-slides/eval/run.py \
  --input .lark-slides/plan/<deck>/slide-01.xml --mode live \
  --confirm-write --title "<evaluation title>"
```

Each run writes `.lark-slides/eval-runs/<timestamp>/report.json` and `review.md`. Treat `report.json` as machine evidence and `review.md` as the human/model visual-review record.

## Regression Benchmark

Use the tracked core fixtures to stop a later change from silently degrading cover, comparison, or architecture pages:

```bash
# Proves structural contracts and static safety for all cases.
python3 skills/lark-slides/eval/benchmark.py \
  --manifest skills/lark-slides/eval/cases.json --mode static

# Only use for manifests without local @ image placeholders.
python3 skills/lark-slides/eval/benchmark.py \
  --manifest skills/lark-slides/eval/cases.json --mode render

# Creates one three-page live deck and screenshots each actual page.
python3 skills/lark-slides/eval/benchmark.py \
  --manifest skills/lark-slides/eval/cases.json --mode live \
  --confirm-write --title "<evaluation title>"
```

The canonical `cases.json` is asset-first and its cover uses a local image placeholder: run `static` then `live`; render mode records that page as skipped and requires live evidence. A 100% pass rate proves only declared local contracts and available render/live evidence, not global SOTA.

## Image-Assisted Candidate Comparison

When a native-shape composition stalls visually, make a second candidate that keeps the same message, copy, and slide structure but replaces only the focal visual with one licensed/generated raster asset. This isolates whether imagery improves the deck rather than rewarding a different story.

1. Generate or obtain the asset under the user-approved image workflow. Keep the original prompt and saved asset path.
2. Put the selected asset under `skills/lark-slides/eval/assets/` for a tracked regression fixture, or keep it under the deck's own assets directory for a one-off deck.
3. Use `<img src="@./relative/path.png">` only with `slides +create`; it uploads the local file and replaces it with a Drive token. Do not use external URLs.
4. Run static validation, then use **live** benchmark mode. `+screenshot --content` does not upload local `@` image placeholders, so it is not sufficient evidence for image fixtures.
5. Blind-compare rendered screenshots: same prompt, same judge, candidate labels hidden, and an explicit pass rule. Preserve the raw judge JSON and token/cost ledger outside committed source.

The canonical `skills/lark-slides/eval/cases.json` already exercises this asset-first route. Keep generated run outputs, judge JSON, and raw baselines under `.lark-slides/`, not committed source; only a small, licensed/provenanced fixture asset belongs under `eval/assets/`.

## Structured-Skill vs Raw-Image Baseline

Do not confuse an image-assisted structured deck with a raw image-generation baseline: the former is still a `lark-slides` result because its layout, text, and editable elements are assembled by the Skill. When testing whether the Skill adds value beyond an image model, compare these two distinct artifacts:

| Artifact | Construction | What it tests |
|---|---|---|
| Raw-image baseline | One generated raster image per page, inserted as a full-slide `<img>` | Image model's direct visual output, including its text/layout limits |
| Structured Skill deck | Native `<shape>`, `<table>`, `<icon>`, or `<whiteboard>` elements, optionally with a generated focal asset | The Skill's layout system, text fidelity, cross-page consistency, and editability |

Use identical slide briefs and preserve both three-page screenshot sets. Blind the deck labels and score visual quality plus `text_fidelity` (complete, accurate, readable copy). Use a simultaneous contact sheet or balanced A/B order; inspect the raw baseline's XML separately. A page containing only one `<img>` cannot support a targeted text edit, while the structured deck must retain its text blocks as distinct elements.

An image-model baseline win does not make the Skill weak; it identifies the visual pattern to improve. A structured-Skill win supports only the narrow result “preferred to this raw-image baseline on this declared local brief.” It is not a global SOTA claim.

## Review Rubric

Score every dimension 0, 1, or 2. A page passes only with all scores at 2.

| Dimension | 2 means | Typical XML repair |
|---|---|---|
| Message hierarchy | Claim is readable before support text; one clear title and one focal conclusion | Increase title prominence, shorten body, move support text |
| Visual focus | The focal region is visibly dominant and supports the claim | Resize/reposition image, chart, or diagram; reduce competing cards |
| Scanability | Text fits, contrast is clear, and labels can be read at normal slide size | Split content, use `autoFit`, increase contrast, simplify labels |
| Composition | Alignment, whitespace, and canvas edges look intentional | Normalize coordinates/gaps; repair overlaps and clipping |
| System consistency | Palette, motif, and asset rendering match the deck plan | Reuse color roles/motif; replace broken or placeholder assets |

## Static Diagnostics

- `error`: blocks creation. Includes XML syntax/schema violations, invalid icons, table canvas overflow, and text overlap.
- `warning`: does not block, but requires screenshot review and either an XML repair or an explicit visual justification. Includes visible non-image content clipping and estimated text-height risk.
- `info`: records a recoverable discrepancy, such as table declared vs resolved dimensions.

For `bbox_overlap`, use `bboxes.left`, `bboxes.right`, and `intersection` instead of guessing which coordinate to move. For `text_may_overflow`, use `estimated.line_count` and `estimated.required_height` to decide whether to shorten, split, resize, or enable `autoFit`.

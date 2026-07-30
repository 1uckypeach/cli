# XSD Update Test Report

Run: 2026-07-26

Scope: `d40a75df` against `d40a75df^`, following
`skills/lark-slides/references/xsd-update-test-plan.md`.

## Environment

- Static validator: `xmllint` (libxml 2.9.13)
- Layout validator: `skills/lark-slides/scripts/xml_text_overlap_lint.py`
- Runtime identity: verified user identity with Slides create/read/update/screenshot scopes
- Temporary presentation: `VMTVs6XKhlQD5zd0L9Zc1wMgnpd`

## Result Summary

| Layer | Result | Evidence |
|---|---|---|
| S: XSD static validation | Partial | 49 positive fixtures accepted; 40 negative fixtures rejected; 3 plan/XSD mismatches |
| P: service persistence | Partial | decimal font size, list metadata, and formula persisted; dynamic field was downgraded |
| V: renderer | Partial | decimal text, custom bullet, formula, page-number field, and smartLayout rendered |
| Layout lint | Partial | normal text/formula fixtures clean; smartLayout incorrectly reported as blank |

## Verified Behavior

- `fontSize="10.5"` passes XSD, persists in readback, and renders.
- List `bulletColor` and `bulletChar` pass XSD, persist, and render as a red asterisk.
- `<formula><latex>E = mc^2</latex></formula>` passes XSD, persists, and renders as typeset math.
- `<field type="slidenum">fallback</field>` renders the real page number (`4` in the four-page deck).
- Minimal `smartLayout` passes XSD and renders a solid pyramid with default index `01`.
- Tested invalid values/structures for font size, list attributes, dynamic fields, formulas, crop anchors, gradient structure, and smartLayout requirements are rejected by the XSD.

## Findings

1. `T02` negative fixture `bulletSize="100"` is invalid as written. The XSD accepts it as a valid absolute 100px size. XML cannot express the intended-but-unspecified percent unit; use an actually invalid lexical value such as `100.5` for a negative test.
2. `T06` conflicts with the XSD: plan and comments require gradients on global `chartPoints` and global `chartBars`, but `ChartGlobalPointsType` and `ChartGlobalBarsType` are empty content models. Both valid-looking fixtures are statically rejected with “Element content is not allowed”.
3. `T03` persistence failure: service accepts `field`, renders the dynamic page number, but readback replaces `<field type="slidenum">fallback</field>` with plain `fallback`. This violates the plan's P-layer preservation assertion.
4. `xml_text_overlap_lint.py` parses `smartLayout` as no declared element and returns `blank_slide`, despite server rendering it visibly. This blocks the skill's required lint gate for otherwise valid smartLayout pages.

## Evidence Files

- Static case matrix: `static-results.tsv`
- XSD fixtures: `*.xml`
- Service create response: `create.json`
- Full service readback: `readback.xml`
- Screenshot response: `screenshots.json`
- Rendered screenshots: `screenshots/`
- SmartLayout lint output: `lint-smartlayout.json`

## Unexecuted Plan Coverage

The full matrix was not exhausted in this run. Remaining work needs uploaded `crop-grid.png`, a complete chart fixture matrix, all list-style enumeration variants, table geometry comparisons, and product decisions for chart background defaults, chart-axis `fontFamily`, and unspecified `gradientMethod` semantics.

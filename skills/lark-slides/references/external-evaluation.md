# External Evaluation and SOTA Claims

Use this reference only when the user asks for a benchmark claim, model comparison, leaderboard result, or “SOTA”. A successful local render is not evidence for any of those claims.

## Claim Levels

| Claim | Minimum evidence | Allowed wording |
|---|---|---|
| Local regression pass | Versioned fixtures, static checks, rendered screenshots, and live XML readback all pass | “Passed the local quality benchmark.” |
| External metric result | A named public evaluator, pinned version/configuration, and saved raw output | “Scored X under <benchmark>/<configuration>.” |
| Better than a named baseline | Same inputs, same evaluator/configuration, raw scores for both systems, and a predeclared comparison rule | “Outperformed <baseline> on this evaluation.” |
| SOTA | Public benchmark coverage, comparable named-system results, blind or human-preference evaluation, reproducible artifacts, and a statistically defensible win | “SOTA on <benchmark/split/metric>.” |

Do not replace a missing comparison with an adjective such as “SOTA”, “best”, or “leading”.

## SlidesGen-Bench-Compatible Protocol

SlidesGen-Bench evaluates rendered slides across content, aesthetics, and editability. For a comparable run:

1. Pin the benchmark commit, model versions, metric configuration, and evaluator prompts.
2. Generate the same task set and source materials as every named baseline. Cover the benchmark scenarios, not only a handpicked demo.
3. Save raw slide images, source-to-slide mappings, static reports, and editable XML for every result.
4. Compute image-based aesthetics metrics on all rendered pages.
5. Run its content and visual VLM evaluation with a disclosed judge model and credentials authorized by the user. Use blinded candidate names for pairwise/ELO comparisons.
6. Run editability tests: apply the same editing instructions to each generated deck and verify that the edit succeeds without breaking unrelated content.
7. Report per-scenario scores, aggregate scores, variance/confidence intervals, failures, costs, and latency. Averages without raw artifacts are not enough.

## Required Artifacts

Keep these outside committed source unless the user asks to publish them:

- benchmark manifest and pinned evaluator version;
- generated slide XML/PPT artifacts and screenshots;
- raw evaluator JSON and configuration;
- per-case pass/fail, latency, and cost ledger;
- blinded preference or ELO judgments;
- comparison table that names every baseline and split.

## Current Local Boundary

`skills/lark-slides/eval/cases.json` is a three-archetype regression set (cover, comparison, architecture). Its 100% result proves only that the declared local structural contracts, renderer calls, and live readback pass. It is deliberately not a SOTA benchmark.

The canonical `cases.json` is asset-first: its cover uses a no-text fixture asset while copy remains native XML. A local preference win against a declared baseline can support only the named local comparison; it does not establish external-system superiority or SOTA.

A direct image-model PPT baseline is a different comparator: it must generate each page as a complete raster slide from the same brief, then be inserted unchanged as a full-page image. A structured `lark-slides` deck may use generated focal assets, but it must keep copy and layout as editable XML elements. A blind win over that baseline supports “preferred to the evaluated raw-image baseline on this local brief,” not SOTA.

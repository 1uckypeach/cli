#!/usr/bin/env python3
# Copyright (c) 2026 Lark Technologies Pte. Ltd.
# SPDX-License-Identifier: MIT
"""Black-box harness for the `chart_external_overlay` lint rule.

Runs xml_text_overlap_lint over every fixture named in a manifest and compares
the observed `chart_external_overlay` issues against each page's expectation.

Usage:
  python3 chart_overlay_harness.py --plan <plan-dir> [--input-xml <file>] [--out <dir>]

--plan       directory holding manifest.json and slides/ (the fixtures).
--input-xml  optional single presentation XML to lint instead of per-fixture
             files (used to lint a server readback deck); pages are matched to
             the manifest by slide order.
--out        output directory for results.json and report.md (default: --plan).

Exit code is 0 when false-negatives == 0 and false-positives == 0, else 1.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

SCRIPTS_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPTS_DIR))
import xml_text_overlap_lint as lint  # noqa: E402

CODE = "chart_external_overlay"


def chart_issues(slide_result: dict) -> list[dict]:
    return [i for i in slide_result.get("issues", []) if i.get("code") == CODE]


def other_error_codes(slide_result: dict) -> list[str]:
    return sorted(
        {
            i.get("code")
            for i in slide_result.get("issues", [])
            if i.get("level") == "error" and i.get("code") != CODE
        }
    )


def slides_from_presentation(xml: str) -> list[str]:
    return re.findall(r"<slide\b[\s\S]*?</slide>", xml)


def lint_page(slide_xml: str) -> dict:
    wrapped = (
        '<presentation xmlns="http://www.larkoffice.com/sml/2.0" width="960" height="540">'
        + slide_xml
        + "</presentation>"
    )
    result = lint.lint_xml(wrapped)
    return result["slides"][0] if result.get("slides") else {"issues": []}


def evaluate(manifest: dict, plan_dir: Path, readback_xml: str | None) -> dict:
    pages = manifest["pages"]
    readback_slides = slides_from_presentation(readback_xml) if readback_xml else None
    rows = []
    fn = fp = unrelated = 0
    for idx, page in enumerate(pages):
        if readback_slides is not None:
            if idx >= len(readback_slides):
                rows.append({**_row_base(page), "status": "MISSING_IN_READBACK",
                             "observed_issue_count": None})
                fn += 1 if page["expected_issue_count"] else 0
                continue
            slide_result = lint_page(readback_slides[idx])
        else:
            slide_xml = (plan_dir / page["fixture"]).read_text(encoding="utf-8")
            slide_result = lint_page(slide_xml)

        issues = chart_issues(slide_result)
        observed = len(issues)
        expected = page["expected_issue_count"]
        others = other_error_codes(slide_result)
        if others:
            unrelated += 1

        # A page is "correct" when observed presence matches expectation.
        expected_present = expected > 0
        observed_present = observed > 0
        if expected_present and not observed_present:
            status = "FALSE_NEGATIVE"
            fn += 1
        elif not expected_present and observed_present:
            status = "FALSE_POSITIVE"
            fp += 1
        elif expected_present and observed != expected:
            status = "COUNT_MISMATCH"  # right presence, wrong number of chart issues
        else:
            status = "OK"

        rows.append({
            **_row_base(page),
            "observed_issue_count": observed,
            "observed_elements": [i.get("elements") for i in issues],
            "unrelated_error_codes": others,
            "status": status,
        })

    return {
        "mode": "readback" if readback_xml else "fixture",
        "summary": {
            "page_count": len(pages),
            "false_negatives": fn,
            "false_positives": fp,
            "pages_with_unrelated_errors": unrelated,
            "pass": fn == 0 and fp == 0,
        },
        "pages": rows,
    }


def _row_base(page: dict) -> dict:
    return {
        "case_id": page["case_id"],
        "slide_number": page["slide_number"],
        "chart_type": page.get("chart_type"),
        "rendered_occlusion": page.get("rendered_occlusion"),
        "rendered_occlusion_source": page.get("rendered_occlusion_source"),
        "expected_issue_count": page["expected_issue_count"],
    }


def render_markdown(results: dict) -> str:
    s = results["summary"]
    lines = [
        f"# chart_external_overlay black-box results ({results['mode']} mode)",
        "",
        f"- pages: **{s['page_count']}**",
        f"- false negatives: **{s['false_negatives']}**",
        f"- false positives: **{s['false_positives']}**",
        f"- pages with unrelated errors: **{s['pages_with_unrelated_errors']}**",
        f"- suite pass: **{s['pass']}**",
        "",
        "| # | case | type | occ | occ_src | exp | obs | unrelated | status |",
        "|--:|------|------|:---:|:-------:|:---:|:---:|-----------|--------|",
    ]
    for r in results["pages"]:
        lines.append(
            f"| {r['slide_number']} | {r['case_id']} | {r.get('chart_type','')} | "
            f"{r.get('rendered_occlusion')} | {r.get('rendered_occlusion_source','')} | "
            f"{r['expected_issue_count']} | {r.get('observed_issue_count')} | "
            f"{','.join(r.get('unrelated_error_codes') or []) or '-'} | {r['status']} |"
        )
    return "\n".join(lines) + "\n"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--plan", required=True)
    ap.add_argument("--input-xml")
    ap.add_argument("--out")
    args = ap.parse_args()

    plan_dir = Path(args.plan).resolve()
    manifest = json.loads((plan_dir / "manifest.json").read_text(encoding="utf-8"))
    readback_xml = Path(args.input_xml).read_text(encoding="utf-8") if args.input_xml else None

    results = evaluate(manifest, plan_dir, readback_xml)

    out_dir = Path(args.out).resolve() if args.out else plan_dir
    out_dir.mkdir(parents=True, exist_ok=True)
    suffix = "_readback" if readback_xml else ""
    (out_dir / f"results{suffix}.json").write_text(
        json.dumps(results, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    (out_dir / f"report{suffix}.md").write_text(render_markdown(results), encoding="utf-8")

    s = results["summary"]
    print(f"[{results['mode']}] pages={s['page_count']} FN={s['false_negatives']} "
          f"FP={s['false_positives']} unrelated={s['pages_with_unrelated_errors']} "
          f"pass={s['pass']}")
    return 0 if s["pass"] else 1


if __name__ == "__main__":
    raise SystemExit(main())

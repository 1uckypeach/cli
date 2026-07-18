#!/usr/bin/env python3
"""Repeatable quality loop for a single SML slide.

Static mode is safe and local. Render mode asks the Slides renderer for a PNG.
Live mode creates a disposable presentation and must be explicitly confirmed.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

SCRIPTS_DIR = Path(__file__).resolve().parents[1] / "scripts"
sys.path.insert(0, str(SCRIPTS_DIR))
from xml_text_overlap_lint import lint_xml  # noqa: E402


def relative_to_cwd(path: Path) -> str:
    try:
        return str(path.resolve().relative_to(Path.cwd().resolve()))
    except ValueError as error:
        raise ValueError(f"path must be inside the current working directory: {path}") from error


def run_cli(arguments: list[str]) -> dict[str, Any]:
    environment = {
        **os.environ,
        "LARKSUITE_CLI_NO_UPDATE_NOTIFIER": "1",
        "LARKSUITE_CLI_NO_SKILLS_NOTIFIER": "1",
    }
    completed = subprocess.run(
        ["lark-cli", *arguments], text=True, capture_output=True, check=False, env=environment
    )
    output = completed.stdout if completed.returncode == 0 else completed.stderr
    try:
        payload = json.loads(output)
    except json.JSONDecodeError as error:
        raise RuntimeError(f"lark-cli returned non-JSON output: {output.strip()}") from error
    if completed.returncode != 0 or not payload.get("ok"):
        raise RuntimeError(json.dumps(payload, ensure_ascii=False))
    return payload


def write_review_template(path: Path, report: dict[str, Any]) -> None:
    screenshot_paths = report.get("screenshots", [])
    presentation_id = report.get("presentation", {}).get("xml_presentation_id", "not created")
    lines = [
        "# Slide Review",
        "",
        f"- Presentation: `{presentation_id}`",
        f"- Static errors: `{report['static']['summary']['error_count']}`",
        f"- Static warnings: `{report['static']['summary']['warning_count']}`",
        "",
        "## Screenshot evidence",
        *((f"- `{item}`" for item in screenshot_paths) if screenshot_paths else ["- No screenshot was created."]),
        "",
        "## Score each 0-2 and name the exact XML repair when below 2",
        "",
        "| Dimension | Score | Evidence / repair |",
        "|---|---:|---|",
        "| Message hierarchy (title, claim, support) | | |",
        "| Visual focus and composition | | |",
        "| Text fit, contrast, and scanability | | |",
        "| Alignment, spacing, and canvas safety | | |",
        "| Theme consistency and asset rendering | | |",
        "",
        "Pass only when every dimension is 2 and static errors are 0. Warnings require either a repair or a screenshot-based justification.",
    ]
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run_static(xml_path: Path) -> dict[str, Any]:
    return lint_xml(xml_path.read_text(encoding="utf-8"), str(xml_path))


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Run the lark-slides static/render/live quality loop.")
    parser.add_argument("--input", required=True, type=Path, help="A single <slide> XML file inside the current directory.")
    parser.add_argument("--mode", choices=("static", "render", "live"), default="static")
    parser.add_argument("--title", default="Slides quality-loop evaluation")
    parser.add_argument("--output-dir", type=Path)
    parser.add_argument("--confirm-write", action="store_true", help="Required before --mode live creates a presentation.")
    options = parser.parse_args(argv)

    xml_path = options.input.resolve()
    output_dir = options.output_dir or Path(".lark-slides/eval-runs") / datetime.now(UTC).strftime("%Y%m%dT%H%M%SZ")
    output_dir.mkdir(parents=True, exist_ok=True)
    static = run_static(xml_path)
    report: dict[str, Any] = {"mode": options.mode, "input": str(xml_path), "static": static, "screenshots": []}

    if static["summary"]["error_count"]:
        report["status"] = "blocked_by_static_errors"
    elif options.mode == "static":
        report["status"] = "static_passed"
    else:
        input_argument = "@" + relative_to_cwd(xml_path)
        output_argument = relative_to_cwd(output_dir)
        if options.mode == "render":
            rendered = run_cli(
                ["slides", "+screenshot", "--as", "user", "--content", input_argument, "--output-dir", output_argument]
            )
            report["screenshots"] = [item["path"] for item in rendered["data"]["screenshots"]]
            report["status"] = "rendered"
        else:
            if not options.confirm_write:
                parser.error("--mode live requires --confirm-write because it creates a presentation")
            created = run_cli(["slides", "+create", "--as", "user", "--title", options.title, "--slides", json.dumps([xml_path.read_text(encoding="utf-8")])])
            presentation = created["data"]
            slide_id = presentation["slide_ids"][0]
            screenshot = run_cli(
                [
                    "slides", "+screenshot", "--as", "user", "--presentation", presentation["xml_presentation_id"],
                    "--slide-id", slide_id, "--output-dir", output_argument,
                ]
            )
            report["presentation"] = presentation
            report["screenshots"] = [item["path"] for item in screenshot["data"]["screenshots"]]
            report["status"] = "live_created_and_rendered"

    (output_dir / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    write_review_template(output_dir / "review.md", report)
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0 if not static["summary"]["error_count"] else 1


if __name__ == "__main__":
    raise SystemExit(main())

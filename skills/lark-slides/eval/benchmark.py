#!/usr/bin/env python3
"""Run a manifest-driven structural and renderability benchmark for SML slides."""

from __future__ import annotations

import argparse
import json
import re
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from run import lint_xml, relative_to_cwd, run_cli
from xml_text_overlap_lint import extract_elements


def load_manifest(path: Path) -> dict[str, Any]:
    manifest = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(manifest.get("cases"), list) or not manifest["cases"]:
        raise ValueError("manifest must contain a non-empty cases list")
    return manifest


def validate_case(case: dict[str, Any], manifest_dir: Path) -> tuple[Path, dict[str, Any]]:
    if not isinstance(case.get("id"), str) or not isinstance(case.get("xml"), str):
        raise ValueError("every case needs string id and xml fields")
    xml_path = (manifest_dir / case["xml"]).resolve()
    xml = xml_path.read_text(encoding="utf-8")
    static = lint_xml(xml, str(xml_path))
    slide_issues = [issue for slide in static["slides"] for issue in slide["issues"]]
    elements = [element for slide_xml in [xml] for element in extract_elements(slide_xml)]
    checks = case.get("checks", {})
    failures: list[str] = []
    if static["summary"]["error_count"]:
        failures.append(f"{static['summary']['error_count']} static error(s)")
    if static["summary"]["warning_count"] and not checks.get("allow_warnings", False):
        failures.append(f"{static['summary']['warning_count']} unresolved warning(s)")
    if len(elements) < checks.get("min_elements", 0):
        failures.append(f"expected at least {checks['min_elements']} elements, got {len(elements)}")
    present_kinds = {element["kind"] for element in elements}
    for kind in checks.get("required_kinds", []):
        if kind not in present_kinds:
            failures.append(f"missing required visual kind: {kind}")
    for text in checks.get("required_text", []):
        if text not in xml:
            failures.append(f"missing required text: {text}")
    local_media = re.findall(r'<img\b[^>]*\bsrc="@([^"\\]+)"', xml)
    for raw_path in local_media:
        media_path = Path(raw_path)
        if media_path.is_absolute() or ".." in media_path.parts:
            failures.append(f"unsafe local image placeholder: @{raw_path}")
        elif not (Path.cwd() / media_path).is_file():
            failures.append(f"local image placeholder not found: @{raw_path}")
    return xml_path, {
        "id": case["id"],
        "static": static,
        "element_count": len(elements),
        "kinds": sorted(present_kinds),
        "issues": slide_issues,
        "requires_live_media_upload": bool(local_media),
        "failures": failures,
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Benchmark SML slide fixtures with static and rendered evidence.")
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--mode", choices=("static", "render", "live"), default="static")
    parser.add_argument("--output-dir", type=Path)
    parser.add_argument("--title", default="[Eval] Lark Slides benchmark")
    parser.add_argument("--confirm-write", action="store_true", help="Required before --mode live creates a presentation.")
    options = parser.parse_args(argv)

    manifest_path = options.manifest.resolve()
    manifest = load_manifest(manifest_path)
    output_dir = options.output_dir or Path(".lark-slides/eval-runs") / ("benchmark-" + datetime.now(UTC).strftime("%Y%m%dT%H%M%SZ"))
    output_dir.mkdir(parents=True, exist_ok=True)
    cases: list[dict[str, Any]] = []
    xml_paths: list[Path] = []
    for case in manifest["cases"]:
        xml_path, result = validate_case(case, manifest_path.parent)
        xml_paths.append(xml_path)
        if options.mode == "render" and result["requires_live_media_upload"]:
            result["render_skipped_reason"] = "local_image_placeholder_requires_live_create"
            result["screenshots"] = []
        elif options.mode == "render" and not result["failures"]:
            rendered = run_cli(
                [
                    "slides", "+screenshot", "--as", "user", "--content", "@" + relative_to_cwd(xml_path),
                    "--output-dir", relative_to_cwd(output_dir), "--output-name", result["id"],
                ]
            )
            result["screenshots"] = [item["path"] for item in rendered["data"]["screenshots"]]
        else:
            result["screenshots"] = []
        cases.append(result)

    presentation: dict[str, Any] | None = None
    if options.mode == "live":
        if not options.confirm_write:
            parser.error("--mode live requires --confirm-write because it creates a presentation")
        if not any(case["failures"] for case in cases):
            created = run_cli(
                ["slides", "+create", "--as", "user", "--title", options.title, "--slides", json.dumps([path.read_text(encoding="utf-8") for path in xml_paths])]
            )
            presentation = created["data"]
            screenshot = run_cli(
                [
                    "slides", "+screenshot", "--as", "user", "--presentation", presentation["xml_presentation_id"],
                    *[flag for slide_id in presentation["slide_ids"] for flag in ("--slide-id", slide_id)],
                    "--output-dir", relative_to_cwd(output_dir),
                ]
            )
            for case, item in zip(cases, screenshot["data"]["screenshots"]):
                case["screenshots"] = [item["path"]]

    passed = sum(
        not case["failures"]
        and (
            options.mode == "static"
            or bool(case["screenshots"])
            or (options.mode == "render" and "render_skipped_reason" in case)
        )
        for case in cases
    )
    report = {
        "name": manifest.get("name", manifest_path.stem),
        "mode": options.mode,
        "summary": {
            "case_count": len(cases),
            "passed": passed,
            "pass_rate": passed / len(cases),
            "rendered": sum(bool(case["screenshots"]) for case in cases) if options.mode == "render" else None,
            "render_skipped": sum("render_skipped_reason" in case for case in cases) if options.mode == "render" else None,
        },
        "cases": cases,
        "limits": "This benchmark proves declared structural requirements and renderer/live evidence. Local @ image placeholders skip content render and require live-create evidence. It does not establish human preference or global state of the art.",
    }
    if presentation is not None:
        report["presentation"] = presentation
    (output_dir / "benchmark.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0 if passed == len(cases) else 1


if __name__ == "__main__":
    raise SystemExit(main())

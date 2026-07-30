#!/usr/bin/env python3
"""Collect text width measurement data from slides presentation for character width estimation optimization."""

from __future__ import annotations

import csv
import os
import re
import subprocess
import sys
import unicodedata
from collections import defaultdict
from pathlib import Path
from typing import Any


PRESENTATION_ID = "BUFBsLX2ZlzyMTdprLicd7rpneg"
SCRIPT_DIR = Path(__file__).resolve().parent
CSV_OUTPUT = SCRIPT_DIR / "width_measurement_data.csv"
MD_OUTPUT = SCRIPT_DIR / "width_measurement_report.md"


def estimate_character_width(character: str, font_size: int | float) -> int | float:
    if character.isspace():
        return font_size * 0.33
    if unicodedata.east_asian_width(character) in {"F", "W"}:
        return font_size
    return font_size * 0.55


def estimate_text_width(text: str, font_size: int | float) -> int | float:
    return sum(estimate_character_width(character, font_size) for character in text)


def extract_attribute(tag_source: str, name: str) -> str | None:
    match = re.search(
        fr"(?:^|\s){re.escape(name)}\s*=\s*(?:\"([^\"]+)\"|'([^']+)')", tag_source
    )
    if not match:
        return None
    return match.group(1) if match.group(1) is not None else match.group(2)


def extract_numeric_attribute(tag_source: str, name: str) -> int | float | None:
    raw = extract_attribute(tag_source, name)
    if raw is None:
        return None
    try:
        value = float(raw)
    except ValueError:
        return None
    return int(value) if value.is_integer() else value


def extract_bool_attribute(tag_source: str, name: str) -> bool:
    value = extract_attribute(tag_source, name)
    return value in {"true", "1", "yes"}


def strip_xml(value: str, preserve_line_breaks: bool = False) -> str:
    stripped = re.sub(r"<!\[CDATA\[([\s\S]*?)\]\]>", r"\1", value)
    if preserve_line_breaks:
        stripped = re.sub(r"<br\b[^>]*>", "\n", stripped)
    stripped = re.sub(r"<[^>]+>", " ", stripped)
    stripped = stripped.replace("&nbsp;", " ")
    stripped = stripped.replace("&amp;", "&")
    stripped = stripped.replace("&lt;", "<")
    stripped = stripped.replace("&gt;", ">")
    stripped = stripped.replace("&quot;", '"')
    stripped = stripped.replace("&#39;", "'")
    if preserve_line_breaks:
        return "\n".join(re.sub(r"\s+", " ", line).strip() for line in stripped.split("\n"))
    return re.sub(r"\s+", " ", stripped).strip()


def strip_xml_paragraphs(value: str) -> str:
    paragraphs = re.findall(r"<p\b[^>]*>([\s\S]*?)</p\s*>", value)
    if paragraphs:
        return "\n".join(strip_xml(paragraph, preserve_line_breaks=True) for paragraph in paragraphs)
    return strip_xml(value, preserve_line_breaks=True)


def normalize_text_no_whitespace(text: str) -> str:
    return re.sub(r"\s+", "", text)


def classify_text_type(text: str) -> str:
    has_chinese = bool(re.search(r"[\u4e00-\u9fff]", text))
    has_english = bool(re.search(r"[A-Za-z]", text))
    has_digit = bool(re.search(r"[0-9]", text))
    has_punctuation = bool(re.search(r"[^\u4e00-\u9fffA-Za-z0-9\s]", text))

    categories = []
    if has_chinese:
        categories.append("中文")
    if has_english:
        categories.append("英文")
    if has_digit:
        categories.append("数字")
    if has_punctuation and not categories:
        categories.append("标点")
    if not categories:
        return "其他"
    return "+".join(categories)


def extract_text_spans(content: str, default_font_size: int | float, default_font_family: str | None,
                       default_bold: bool, default_italic: bool) -> list[dict[str, Any]]:
    spans = []

    p_matches = list(re.finditer(r"<p\b([^>]*)>([\s\S]*?)</p\s*>", content))
    if not p_matches:
        p_matches = [re.match(r"()([\s\S]*)", content)]

    for p_match in p_matches:
        p_attrs, p_body = p_match.groups()
        p_text = strip_xml(p_body, preserve_line_breaks=True)
        if not p_text:
            continue

        span_matches = list(re.finditer(r"<span\b([^>]*)>([\s\S]*?)</span\s*>", p_body))
        if not span_matches:
            span_text = strip_xml(p_body, preserve_line_breaks=True)
            if span_text:
                font_size = default_font_size
                if extract_numeric_attribute(p_attrs, "fontSize") is not None:
                    font_size = extract_numeric_attribute(p_attrs, "fontSize")
                font_family = extract_attribute(p_attrs, "fontFamily") or default_font_family
                latin_font = extract_attribute(p_attrs, "latinFont")
                ea_font = extract_attribute(p_attrs, "eaFont")
                cs_font = extract_attribute(p_attrs, "csFont")
                bold = extract_bool_attribute(p_attrs, "bold") or default_bold
                italic = extract_bool_attribute(p_attrs, "italic") or default_italic
                letter_spacing = extract_numeric_attribute(p_attrs, "letterSpacing")

                spans.append({
                    "text": span_text,
                    "font_size": font_size,
                    "font_family": font_family,
                    "latin_font": latin_font,
                    "ea_font": ea_font,
                    "cs_font": cs_font,
                    "bold": bold,
                    "italic": italic,
                    "letter_spacing": letter_spacing,
                })
        else:
            last_end = 0
            for span_match in span_matches:
                span_attrs, span_body = span_match.groups()
                span_text = strip_xml(span_body, preserve_line_breaks=True)
                if not span_text:
                    last_end = span_match.end()
                    continue

                font_size = default_font_size
                if extract_numeric_attribute(p_attrs, "fontSize") is not None:
                    font_size = extract_numeric_attribute(p_attrs, "fontSize")
                if extract_numeric_attribute(span_attrs, "fontSize") is not None:
                    font_size = extract_numeric_attribute(span_attrs, "fontSize")

                font_family = extract_attribute(span_attrs, "fontFamily") or \
                              extract_attribute(p_attrs, "fontFamily") or default_font_family
                latin_font = extract_attribute(span_attrs, "latinFont") or extract_attribute(p_attrs, "latinFont")
                ea_font = extract_attribute(span_attrs, "eaFont") or extract_attribute(p_attrs, "eaFont")
                cs_font = extract_attribute(span_attrs, "csFont") or extract_attribute(p_attrs, "csFont")
                bold = extract_bool_attribute(span_attrs, "bold") or \
                       extract_bool_attribute(p_attrs, "bold") or default_bold
                italic = extract_bool_attribute(span_attrs, "italic") or \
                         extract_bool_attribute(p_attrs, "italic") or default_italic
                letter_spacing = extract_numeric_attribute(span_attrs, "letterSpacing") or \
                                extract_numeric_attribute(p_attrs, "letterSpacing")

                spans.append({
                    "text": span_text,
                    "font_size": font_size,
                    "font_family": font_family,
                    "latin_font": latin_font,
                    "ea_font": ea_font,
                    "cs_font": cs_font,
                    "bold": bold,
                    "italic": italic,
                    "letter_spacing": letter_spacing,
                })
                last_end = span_match.end()

            tail_text = strip_xml(p_body[last_end:], preserve_line_breaks=True)
            if tail_text:
                font_size = default_font_size
                if extract_numeric_attribute(p_attrs, "fontSize") is not None:
                    font_size = extract_numeric_attribute(p_attrs, "fontSize")
                font_family = extract_attribute(p_attrs, "fontFamily") or default_font_family
                latin_font = extract_attribute(p_attrs, "latinFont")
                ea_font = extract_attribute(p_attrs, "eaFont")
                cs_font = extract_attribute(p_attrs, "csFont")
                bold = extract_bool_attribute(p_attrs, "bold") or default_bold
                italic = extract_bool_attribute(p_attrs, "italic") or default_italic
                letter_spacing = extract_numeric_attribute(p_attrs, "letterSpacing")
                spans.append({
                    "text": tail_text,
                    "font_size": font_size,
                    "font_family": font_family,
                    "latin_font": latin_font,
                    "ea_font": ea_font,
                    "cs_font": cs_font,
                    "bold": bold,
                    "italic": italic,
                    "letter_spacing": letter_spacing,
                })
    return spans


def extract_text_elements(slide_xml: str) -> list[dict[str, Any]]:
    elements = []

    for match in re.finditer(r"<(shape)\b([^>]*)>", slide_xml):
        kind, attrs = match.group(1), match.group(2)
        is_self_closing = attrs.rstrip().endswith("/")
        content = ""
        if kind in {"shape"} and not is_self_closing:
            close_index = slide_xml.find(f"</{kind}>", match.end())
            if close_index != -1:
                content = slide_xml[match.end() : close_index]

        element_type = extract_attribute(attrs, "type")
        if element_type != "text":
            continue

        element_id = extract_attribute(attrs, "id") or f"{kind}-{len(elements) + 1}"
        x = extract_numeric_attribute(attrs, "topLeftX")
        y = extract_numeric_attribute(attrs, "topLeftY")
        width = extract_numeric_attribute(attrs, "width")
        height = extract_numeric_attribute(attrs, "height")
        if any(v is None for v in [x, y, width, height]):
            continue

        content_attrs_match = re.search(r"<content\b([^>]*)>", content)
        content_attrs = content_attrs_match.group(1) if content_attrs_match else ""

        font_size = extract_numeric_attribute(content_attrs, "fontSize")
        if font_size is None:
            font_size = extract_numeric_attribute(attrs, "fontSize")
        if font_size is None:
            font_size = 16

        font_family = extract_attribute(content_attrs, "fontFamily") or extract_attribute(attrs, "fontFamily")
        latin_font = extract_attribute(content_attrs, "latinFont")
        ea_font = extract_attribute(content_attrs, "eaFont")
        cs_font = extract_attribute(content_attrs, "csFont")
        bold = extract_bool_attribute(content_attrs, "bold") or extract_bool_attribute(attrs, "bold")
        italic = extract_bool_attribute(content_attrs, "italic") or extract_bool_attribute(attrs, "italic")
        letter_spacing = extract_numeric_attribute(content_attrs, "letterSpacing")

        wrap = extract_attribute(content_attrs, "wrap")
        if wrap is None:
            wrap = "true"
        auto_fit = extract_attribute(content_attrs, "autoFit")
        if auto_fit is None:
            auto_fit = "none"
        text_align = extract_attribute(content_attrs, "textAlign")
        vertical_align = extract_attribute(content_attrs, "verticalAlign") or "middle"

        padding_left = extract_numeric_attribute(content_attrs, "paddingLeft") or 0
        padding_right = extract_numeric_attribute(content_attrs, "paddingRight") or 0
        padding_top = extract_numeric_attribute(content_attrs, "paddingTop") or 0
        padding_bottom = extract_numeric_attribute(content_attrs, "paddingBottom") or 0

        full_text = strip_xml_paragraphs(content)
        clean_text = normalize_text_no_whitespace(full_text)
        if not clean_text:
            continue

        content_inner = ""
        if content_attrs_match:
            content_start = content_attrs_match.end()
            content_end = content.find("</content>", content_start)
            if content_end != -1:
                content_inner = content[content_start:content_end]

        spans = extract_text_spans(content_inner, font_size, font_family, bold, italic)

        hard_lines = full_text.split("\n")
        max_line_estimated_width = 0
        max_line_text = ""
        for line in hard_lines:
            line_clean = normalize_text_no_whitespace(line)
            if not line_clean:
                continue
            line_width = 0
            for span in spans:
                if span["text"] in line or line in span["text"]:
                    line_width += estimate_text_width(normalize_text_no_whitespace(span["text"]), span["font_size"])
            if line_width == 0:
                line_width = estimate_text_width(line_clean, font_size)
            if line_width > max_line_estimated_width:
                max_line_estimated_width = line_width
                max_line_text = line_clean

        available_width = width - padding_left - padding_right

        elements.append({
            "id": element_id,
            "x": x,
            "y": y,
            "width": width,
            "height": height,
            "padding_left": padding_left,
            "padding_right": padding_right,
            "padding_top": padding_top,
            "padding_bottom": padding_bottom,
            "available_width": available_width,
            "font_size": font_size,
            "font_family": font_family,
            "latin_font": latin_font,
            "ea_font": ea_font,
            "cs_font": cs_font,
            "bold": bold,
            "italic": italic,
            "letter_spacing": letter_spacing,
            "wrap": wrap,
            "auto_fit": auto_fit,
            "text_align": text_align,
            "vertical_align": vertical_align,
            "text_raw": full_text,
            "text_clean": clean_text,
            "max_line_text": max_line_text,
            "estimated_width": max_line_estimated_width,
            "width_ratio": max_line_estimated_width / available_width if available_width > 0 else None,
            "spans": spans,
        })

    return elements


def run_lark_cli(args: list[str], env: dict[str, str]) -> subprocess.CompletedProcess:
    cmd = ["lark-cli"] + args
    return subprocess.run(
        cmd,
        env=env,
        capture_output=True,
        text=True,
        cwd=str(SCRIPT_DIR),
        timeout=120,
    )


def get_presentation_xml(env: dict[str, str]) -> str:
    result = run_lark_cli(
        ["slides", "+xml-get", "--presentation", PRESENTATION_ID, "--raw"],
        env,
    )
    if result.returncode != 0:
        print(f"Error getting presentation XML: {result.stderr}", file=sys.stderr)
        sys.exit(1)
    return result.stdout


def get_slide_ids(xml: str) -> list[tuple[int, str]]:
    slides = []
    for index, match in enumerate(re.finditer(r"<slide\b([^>]*)>", xml)):
        attrs = match.group(1)
        slide_id = extract_attribute(attrs, "id")
        if slide_id:
            slides.append((index + 1, slide_id))
    return slides


def get_slide_xml(slide_id: str, env: dict[str, str]) -> str:
    result = run_lark_cli(
        ["slides", "+xml-get", "--presentation", PRESENTATION_ID, "--slide-id", slide_id, "--raw"],
        env,
    )
    if result.returncode != 0:
        print(f"Error getting slide {slide_id} XML: {result.stderr}", file=sys.stderr)
        return ""
    return result.stdout


def take_screenshots(slide_ids: list[str], output_dir: Path, env: dict[str, str]) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    batch_size = 10
    for i in range(0, len(slide_ids), batch_size):
        batch = slide_ids[i:i + batch_size]
        args = [
            "slides", "+screenshot",
            "--presentation", PRESENTATION_ID,
            "--output-dir", str(output_dir.relative_to(SCRIPT_DIR)),
        ]
        for sid in batch:
            args.extend(["--slide-id", sid])
        result = run_lark_cli(args, env)
        if result.returncode != 0:
            print(f"Warning: screenshot failed for batch {i//batch_size + 1}: {result.stderr}", file=sys.stderr)


def main() -> None:
    env = os.environ.copy()
    env["LARKSUITE_CLI_NO_UPDATE_NOTIFIER"] = "1"
    env["LARKSUITE_CLI_NO_SKILLS_NOTIFIER"] = "1"

    print(f"Fetching presentation {PRESENTATION_ID}...")
    full_xml = get_presentation_xml(env)
    slides_info = get_slide_ids(full_xml)
    print(f"Found {len(slides_info)} slides")

    screenshot_dir = SCRIPT_DIR / ".lark-slides" / "screenshots"
    print(f"Taking screenshots (this may take a while)...")
    take_screenshots([sid for _, sid in slides_info], screenshot_dir, env)

    all_samples = []

    for slide_num, slide_id in slides_info:
        print(f"Processing slide {slide_num} ({slide_id})...")
        slide_xml = get_slide_xml(slide_id, env)
        if not slide_xml:
            continue

        elements = extract_text_elements(slide_xml)
        for elem in elements:
            text_type = classify_text_type(elem["text_clean"])
            is_single_line_hard = "\n" not in elem["text_raw"]
            wrap_enabled = elem["wrap"] not in {"false", "0"}
            has_auto_fit = elem["auto_fit"] in {"normal-auto-fit", "shape-auto-fit"}

            sample = {
                "slide_number": slide_num,
                "slide_id": slide_id,
                "element_id": elem["id"],
                "x": round(elem["x"], 2),
                "y": round(elem["y"], 2),
                "shape_width": round(elem["width"], 2),
                "shape_height": round(elem["height"], 2),
                "padding_left": elem["padding_left"],
                "padding_right": elem["padding_right"],
                "available_width": round(elem["available_width"], 2),
                "font_size": elem["font_size"],
                "font_family": elem["font_family"] or "",
                "bold": str(elem["bold"]).lower(),
                "italic": str(elem["italic"]).lower(),
                "wrap": elem["wrap"] or "",
                "auto_fit": elem["auto_fit"] or "",
                "text_align": elem["text_align"] or "",
                "text_clean": elem["text_clean"],
                "text_length": len(elem["text_clean"]),
                "text_type": text_type,
                "is_single_line_hard": str(is_single_line_hard).lower(),
                "estimated_width": round(elem["estimated_width"], 2),
                "width_ratio": round(elem["width_ratio"], 4) if elem["width_ratio"] is not None else "",
                "likely_wraps_actual": "",
                "notes": "",
            }
            if elem["width_ratio"] is not None and not has_auto_fit:
                ratio = elem["width_ratio"]
                if not wrap_enabled:
                    sample["likely_wraps_actual"] = "no_wrap"
                    sample["notes"] = "wrap=false，不自动换行"
                elif ratio < 0.7:
                    sample["likely_wraps_actual"] = "no_slack"
                    sample["notes"] = f"估算宽度 < 70% 可用宽度({ratio:.2f})，明显有留白"
                elif ratio < 0.85:
                    sample["likely_wraps_actual"] = "no"
                    sample["notes"] = f"估算宽度 {ratio:.2f}x 可用宽度，大概率单行"
                elif ratio <= 1.0:
                    sample["likely_wraps_actual"] = "tight_single"
                    sample["notes"] = f"估算宽度 {ratio:.2f}x 可用宽度，接近填满，需确认是否单行"
                elif ratio <= 1.2:
                    sample["likely_wraps_actual"] = "borderline"
                    sample["notes"] = f"估算宽度 {ratio:.2f}x 可用宽度，边界情况，需截图确认是否换行"
                elif ratio <= 1.5:
                    sample["likely_wraps_actual"] = "likely_wrap"
                    sample["notes"] = f"估算宽度 {ratio:.2f}x 可用宽度，大概率换2行"
                else:
                    sample["likely_wraps_actual"] = "yes"
                    sample["notes"] = f"估算宽度 {ratio:.2f}x 可用宽度，肯定换行（多行）"
            elif has_auto_fit:
                sample["likely_wraps_actual"] = "autofit"
                sample["notes"] = "autoFit开启，字体会自动缩放"
            else:
                sample["likely_wraps_actual"] = "unknown"
                sample["notes"] = "无法判断"

            all_samples.append(sample)

    print(f"\nCollected {len(all_samples)} text shape samples")

    csv_fields = [
        "slide_number", "slide_id", "element_id", "x", "y",
        "shape_width", "shape_height", "padding_left", "padding_right", "available_width",
        "font_size", "font_family", "bold", "italic", "wrap", "auto_fit", "text_align",
        "text_clean", "text_length", "text_type", "is_single_line_hard",
        "estimated_width", "width_ratio", "likely_wraps_actual", "notes",
    ]

    with open(CSV_OUTPUT, "w", encoding="utf-8", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=csv_fields)
        writer.writeheader()
        writer.writerows(all_samples)
    print(f"CSV saved to: {CSV_OUTPUT}")

    key_categories = {"tight_single", "borderline", "likely_wrap", "yes"}
    tight_samples = [s for s in all_samples if s["likely_wraps_actual"] == "tight_single"]
    borderline_samples = [s for s in all_samples if s["likely_wraps_actual"] == "borderline"]
    likely_wrap_samples = [s for s in all_samples if s["likely_wraps_actual"] == "likely_wrap"]
    yes_wrap_samples = [s for s in all_samples if s["likely_wraps_actual"] == "yes"]

    type_stats = defaultdict(list)
    for s in all_samples:
        if s["width_ratio"] != "":
            type_stats[s["text_type"]].append(s)

    md_lines = [
        "# 字符宽度测量数据报告",
        "",
        f"**Presentation**: {PRESENTATION_ID}",
        f"**总样本数**: {len(all_samples)}",
        f"**关键测量样本（比值≥0.85）**: {len(tight_samples) + len(borderline_samples) + len(likely_wrap_samples) + len(yes_wrap_samples)}",
        f"  - 接近填满(0.85-1.0): {len(tight_samples)}",
        f"  - 边界情况(1.0-1.2): {len(borderline_samples)}",
        f"  - 大概率换行(1.2-1.5): {len(likely_wrap_samples)}",
        f"  - 肯定换行(>1.5): {len(yes_wrap_samples)}",
        "",
        "## 按文本类型统计（所有样本）",
        "",
        "| 文本类型 | 样本数 | 平均估算/可用比 | 最小比值 | 最大比值 |",
        "|----------|--------|------------------|----------|----------|",
    ]

    for text_type in sorted(type_stats.keys()):
        samples = type_stats[text_type]
        ratios = [float(s["width_ratio"]) for s in samples if s["width_ratio"] != ""]
        if not ratios:
            continue
        avg_ratio = sum(ratios) / len(ratios)
        min_ratio = min(ratios)
        max_ratio = max(ratios)
        md_lines.append(
            f"| {text_type} | {len(samples)} | {avg_ratio:.4f} | {min_ratio:.4f} | {max_ratio:.4f} |"
        )

    def add_sample_table(title: str, samples: list[dict], max_rows: int = 50):
        md_lines.extend([
            "",
            f"## {title}",
            "",
            "| 页码 | 元素ID | 字体 | 字号 | Bold | 文本类型 | 硬换行 | 文本 | 估算宽度 | 可用宽度 | 比值 |",
            "|------|--------|------|------|------|----------|--------|------|----------|----------|------|",
        ])
        for idx, s in enumerate(samples[:max_rows]):
            text_preview = s["text_clean"][:50] + ("..." if len(s["text_clean"]) > 50 else "")
            md_lines.append(
                f"| {s['slide_number']} | {s['element_id']} | {s['font_family']} | {s['font_size']} | "
                f"{s['bold']} | {s['text_type']} | {s['is_single_line_hard']} | {text_preview} | {s['estimated_width']} | "
                f"{s['available_width']} | {s['width_ratio']} |"
            )
        if len(samples) > max_rows:
            md_lines.append(f"| ... | ... | ... | ... | ... | ... | ... | (共 {len(samples)} 个样本) | ... | ... | ... |")

    add_sample_table("接近填满样本（估算比值 0.85-1.0，最适合校准单行宽度）", tight_samples)
    add_sample_table("边界换行样本（估算比值 1.0-1.2，需截图确认是否换行）", borderline_samples)
    add_sample_table("大概率换行样本（估算比值 1.2-1.5）", likely_wrap_samples)
    add_sample_table("肯定换行样本（估算比值 > 1.5）", yes_wrap_samples, max_rows=20)

    md_lines.extend([
        "",
        "## 字体统计",
        "",
        "| 字体 | 样本数 |",
        "|------|--------|",
    ])

    font_stats = defaultdict(int)
    for s in all_samples:
        font = s["font_family"] or "(default)"
        font_stats[font] += 1
    for font, count in sorted(font_stats.items(), key=lambda x: -x[1]):
        md_lines.append(f"| {font} | {count} |")

    md_lines.extend([
        "",
        "## 字号+粗体统计",
        "",
        "| 字号 | Bold | 样本数 | 平均比值 |",
        "|------|------|--------|----------|",
    ])

    size_bold_stats = defaultdict(list)
    for s in all_samples:
        if s["width_ratio"] != "":
            key = (s["font_size"], s["bold"])
            size_bold_stats[key].append(float(s["width_ratio"]))
    for (font_size, bold), ratios in sorted(size_bold_stats.items()):
        avg = sum(ratios) / len(ratios)
        md_lines.append(f"| {font_size} | {bold} | {len(ratios)} | {avg:.4f} |")

    md_lines.extend([
        "",
        "## 说明",
        "",
        "- **估算宽度**: 使用当前 `estimate_character_width` 函数计算（中文=1em，西文=0.55em，空格=0.33em）",
        "- **可用宽度**: shape.width - paddingLeft - paddingRight",
        "- **width_ratio**: 估算宽度 / 可用宽度（针对最长硬换行段落计算）",
        "- **硬换行**: 文本中是否包含显式 \\n 分段",
        f"- 截图保存在: {screenshot_dir}",
        "",
        "### 比值解读建议",
        "- ratio < 0.7: 明显留白，估算宽度可能偏宽，或文本确实很短",
        "- 0.7-0.85: 大概率单行，有少量留白",
        "- 0.85-1.0: 接近填满，是校准西文/中文字符宽度系数的最佳样本",
        "- 1.0-1.2: 边界情况，需要看截图确认：是刚好填满单行还是换行了",
        "- 1.2-1.5: 大概率换2行",
        "- >1.5: 肯定换行（多行文本）",
        "",
        "### 校准建议",
        "1. 先看 ratio 0.85-1.0 的样本：如果截图中这些文本**确实单行且接近填满**，说明当前估算大致准确；如果有较多留白，说明估算偏宽，需要减小西文字符系数",
        "2. 再看 ratio 1.0-1.2 的样本：结合截图判断实际是单行还是换行，反推合理系数",
        "3. 重点关注纯英文、纯数字、纯中文、中英混合这几类分别统计",
        "4. Bold 字体通常比常规字体稍宽，Italic 稍窄，需要分别考虑",
        "",
        "请人工核对截图确认边界样本的实际换行情况，用于校准字符宽度系数。",
    ])

    with open(MD_OUTPUT, "w", encoding="utf-8") as f:
        f.write("\n".join(md_lines))
    print(f"Markdown report saved to: {MD_OUTPUT}")

    print("\n=== Summary ===")
    print(f"Total samples: {len(all_samples)}")
    print(f"  Tight single (0.85-1.0): {len(tight_samples)}")
    print(f"  Borderline (1.0-1.2): {len(borderline_samples)}")
    print(f"  Likely wrap (1.2-1.5): {len(likely_wrap_samples)}")
    print(f"  Definite wrap (>1.5): {len(yes_wrap_samples)}")
    print()
    for text_type in sorted(type_stats.keys()):
        samples = type_stats[text_type]
        ratios = [float(s["width_ratio"]) for s in samples if s["width_ratio"] != ""]
        if ratios:
            avg_ratio = sum(ratios) / len(ratios)
            print(f"  {text_type}: {len(samples)} samples, avg ratio = {avg_ratio:.4f}")


if __name__ == "__main__":
    main()

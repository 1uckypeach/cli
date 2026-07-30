#!/usr/bin/env python3
"""Verify character width calibration by comparing new estimates against visual evidence."""

from __future__ import annotations

import csv
import sys
import unicodedata
from pathlib import Path
from typing import Any

SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

import xml_text_overlap_lint as lint


def old_estimate_character_width(character: str, font_size: float) -> float:
    if character.isspace():
        return font_size * 0.33
    if unicodedata.east_asian_width(character) in {"F", "W"}:
        return font_size
    return font_size * 0.55


def old_estimate_text_width(text: str, font_size: float) -> float:
    return sum(old_estimate_character_width(c, font_size) for c in text)


def make_element(
    width: float,
    height: float,
    text: str,
    font_size: float,
    *,
    padding_left: float = 0,
    padding_right: float = 0,
    padding_top: float = 0,
    padding_bottom: float = 0,
    bold: bool = False,
    wrap: str = "true",
    auto_fit: str | None = None,
    font_family: str = "",
) -> dict[str, Any]:
    return {
        "id": "test",
        "kind": "shape",
        "type": "text",
        "x": 0,
        "y": 0,
        "width": width,
        "height": height,
        "paddingLeft": padding_left,
        "paddingRight": padding_right,
        "paddingTop": padding_top,
        "paddingBottom": padding_bottom,
        "fontSize": font_size,
        "fontFamily": font_family,
        "bold": bold,
        "italic": False,
        "text": text,
        "wrap": wrap,
        "autoFit": auto_fit,
        "letterSpacing": 0,
        "paragraphs": [
            {
                "text": p,
                "fontSize": font_size,
                "lineSpacing": None,
                "beforeLineSpacing": None,
                "afterLineSpacing": None,
                "letterSpacing": None,
            }
            for p in text.split("\n")
            if p
        ],
    }


def count_chars(text: str) -> dict[str, int]:
    counts = {"cjk": 0, "upper": 0, "lower": 0, "digit": 0, "punct": 0, "space": 0}
    for c in text:
        if c.isspace():
            counts["space"] += 1
        elif unicodedata.east_asian_width(c) in {"F", "W"}:
            counts["cjk"] += 1
        elif c.isupper():
            counts["upper"] += 1
        elif c.islower():
            counts["lower"] += 1
        elif c.isdigit():
            counts["digit"] += 1
        else:
            counts["punct"] += 1
    return counts


def test_case(
    name: str,
    text: str,
    available_width: float,
    font_size: float,
    *,
    expected_single_line: bool | None = None,
    note: str = "",
) -> dict[str, Any]:
    elem = make_element(available_width, 1000, text, font_size)
    new_width = lint.estimate_text_max_line_width(elem)
    new_ratio = new_width / available_width
    new_lines = lint.estimate_text_line_count_for_text(elem, text)

    old_width = max(old_estimate_text_width(p, font_size) for p in text.split("\n") if p)
    old_ratio = old_width / available_width

    counts = count_chars(text)

    result = {
        "name": name,
        "text_preview": text[:60] + ("..." if len(text) > 60 else ""),
        "font_size": font_size,
        "available_width": available_width,
        "char_counts": counts,
        "new_estimated_width": round(new_width, 2),
        "new_ratio": round(new_ratio, 4),
        "new_line_count": new_lines,
        "old_estimated_width": round(old_width, 2),
        "old_ratio": round(old_ratio, 4),
        "expected_single_line": expected_single_line,
        "note": note,
    }

    if expected_single_line is True:
        result["new_correct"] = new_ratio <= 1.02
        result["old_correct"] = old_ratio <= 1.02
    elif expected_single_line is False:
        result["new_correct"] = new_ratio > 1.0
        result["old_correct"] = old_ratio > 1.0
    else:
        result["new_correct"] = None
        result["old_correct"] = None

    return result


def print_result(r: dict[str, Any]) -> None:
    status_new = ""
    status_old = ""
    if r["expected_single_line"] is not None:
        status_new = "✓" if r["new_correct"] else "✗"
        status_old = "✓" if r["old_correct"] else "✗"

    print(f"--- {r['name']} {status_new} ---")
    print(f"  Text: {r['text_preview']}")
    print(f"  Font: {r['font_size']}pt, Available: {r['available_width']}px")
    cc = r["char_counts"]
    print(f"  Chars: CJK={cc['cjk']} Upper={cc['upper']} Lower={cc['lower']} "
          f"Digit={cc['digit']} Punct={cc['punct']} Space={cc['space']}")
    print(f"  NEW: width={r['new_estimated_width']}px ratio={r['new_ratio']:.4f} lines={r['new_line_count']} {status_new}")
    print(f"  OLD: width={r['old_estimated_width']}px ratio={r['old_ratio']:.4f} {status_old}")
    if r["note"]:
        print(f"  Note: {r['note']}")
    print()


def main() -> None:
    print("=" * 70)
    print("Character Width Calibration Verification")
    print("=" * 70)
    print()

    results = []

    # Test cases based on visual evidence from screenshots
    # Page 17 line 2: "在 Natural Language Processing 领域，大语言模型 LLM 的出现彻底改变了人机交互方式。"
    # Visually: single line, fills ~93-95% of 800px
    results.append(test_case(
        "p17 bbR line2 (mixed CJK+English, 18pt, 800px, expected single)",
        "在 Natural Language Processing 领域，大语言模型 LLM 的出现彻底改变了人机交互方式。",
        available_width=800,
        font_size=18,
        expected_single_line=True,
        note="Visually single line, ~93% fill"
    ))

    # Page 17 line 3: "从 ChatGPT 到 文心一言，从 GPT-4 到 Claude 3，AI 助手正在成为人们工作生活的标配。"
    # Visually: single line
    results.append(test_case(
        "p17 bbR line3 (mixed CJK+English, 18pt, 800px, expected single)",
        "从 ChatGPT 到 文心一言，从 GPT-4 到 Claude 3，AI 助手正在成为人们工作生活的标配。",
        available_width=800,
        font_size=18,
        expected_single_line=True,
        note="Visually single line"
    ))

    # Page 6 paragraph 3: "中文与英文混排测试：The quick brown fox jumps over the lazy dog. 敏捷的棕色狐狸跳过了懒狗。"
    # Visually: single line, very tight (was borderline with old model)
    results.append(test_case(
        "p6 bNf para3 (mixed tight line, 18pt, 800px, expected single)",
        "中文与英文混排测试：The quick brown fox jumps over the lazy dog. 敏捷的棕色狐狸跳过了懒狗。",
        available_width=800,
        font_size=18,
        expected_single_line=True,
        note="Borderline case - visually fits on one line"
    ))

    # Page 6 paragraph 2 (English): "Typography is the art and technique of arranging type to make written language legible, readable, and appealing when displayed."
    # Visually: wraps to 2 lines
    results.append(test_case(
        "p6 bNf para2 (English paragraph, 18pt, 800px, wraps)",
        "Typography is the art and technique of arranging type to make written language legible, readable, and appealing when displayed.",
        available_width=800,
        font_size=18,
        expected_single_line=False,
        note="Visually wraps to 2 lines"
    ))

    # Page 28 right column line 1: "This is the right column body text in 15pt size."
    # Visually: single line, ~92% fill of 400px
    results.append(test_case(
        "p28 bZQ line1 (English, 15pt, 400px, expected single)",
        "This is the right column body text in 15pt size.",
        available_width=400,
        font_size=15,
        expected_single_line=True,
        note="Visually single line, ~92% fill"
    ))

    # Page 28 right column paragraph: "Multi-column layout optimizes space and information density."
    # Visually: wraps to 2 lines ("density." on its own line)
    results.append(test_case(
        "p28 bZQ para (English, 15pt, 400px, wraps)",
        "Multi-column layout optimizes space and information density.",
        available_width=400,
        font_size=15,
        expected_single_line=False,
        note="Visually wraps - 'density.' on next line"
    ))

    # Page 28 left column line 1: "这是左栏的正文内容，使用 15pt 字号，1.7 倍行间距。"
    # Visually: single line in 420px column
    results.append(test_case(
        "p28 bZm line1 (CJK+digits, 15pt, 420px, expected single)",
        "这是左栏的正文内容，使用 15pt 字号，1.7 倍行间距。",
        available_width=420,
        font_size=15,
        expected_single_line=True,
        note="Visually single line"
    ))

    # Page 23: 2.0 line spacing body (14pt, 270px) - wraps to multiple lines
    # "这是一段测试文字，用于展示2.0倍行间距的效果。行间距较大时，文字显得疏朗透气，阅读体验轻松。适合需要留白感的设计。"
    results.append(test_case(
        "p23 bZc body (CJK+digits, 14pt, 270px, wraps)",
        "这是一段测试文字，用于展示2.0倍行间距的效果。行间距较大时，文字显得疏朗透气，阅读体验轻松。适合需要留白感的设计。",
        available_width=270,
        font_size=14,
        expected_single_line=False,
        note="Visually wraps to ~4 lines"
    ))

    # Page 21 English line: "Underline is used for links and key annotations, strikethrough for deleted content."
    # Visually: single line at 18pt in 800px, fills ~92%
    results.append(test_case(
        "p21 bbT English line (18pt, 800px, expected single)",
        "Underline is used for links and key annotations, strikethrough for deleted content.",
        available_width=800,
        font_size=18,
        expected_single_line=True,
        note="Visually single line, ~92% fill"
    ))

    # Page 12 Helvetica paragraph: "Sans-serif fonts are clean, modern, and highly legible. Widely used in UI design, branding, and digital media."
    # 16pt, 800px, wraps to 2 lines
    results.append(test_case(
        "p12 bak paragraph (English, 16pt, 800px, wraps)",
        "Sans-serif fonts are clean, modern, and highly legible. Widely used in UI design, branding, and digital media.",
        available_width=800,
        font_size=16,
        expected_single_line=False,
        note="Visually wraps to 2 lines"
    ))

    # Pure uppercase test: "ABCDEFGHIJKLMNOPQRSTUVWXYZ" at 28pt bold, should be ~50% of 800px
    results.append(test_case(
        "p12 uppercase alphabet (28pt bold, 800px, single)",
        "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
        available_width=800,
        font_size=28,
        expected_single_line=True,
        note="Centered test string, ~50% width"
    ))

    # Pure lowercase test: "abcdefghijklmnopqrstuvwxyz" at 28pt
    results.append(test_case(
        "p12 lowercase alphabet (28pt bold, 800px, single)",
        "abcdefghijklmnopqrstuvwxyz",
        available_width=800,
        font_size=28,
        expected_single_line=True,
        note="Centered test string, similar width to uppercase"
    ))

    # Digits+symbols: "0123456789!@#$%^&*()" at 28pt, shorter than alphabets
    results.append(test_case(
        "p12 digits+symbols (28pt bold, 800px, single)",
        "0123456789!@#$%^&*()",
        available_width=800,
        font_size=28,
        expected_single_line=True,
        note="Shorter than alphabet lines"
    ))

    for r in results:
        print_result(r)

    # Summary statistics
    print("=" * 70)
    print("SUMMARY")
    print("=" * 70)
    new_correct = sum(1 for r in results if r["new_correct"] is True)
    old_correct = sum(1 for r in results if r["old_correct"] is True)
    total_evaluated = sum(1 for r in results if r["expected_single_line"] is not None)
    print(f"Test cases with known expectation: {total_evaluated}")
    print(f"New model correct: {new_correct}/{total_evaluated}")
    print(f"Old model correct: {old_correct}/{total_evaluated}")
    print()

    # Show ratio distribution for single-line cases
    print("Single-line cases (ratio should be 0.85-1.02):")
    for r in results:
        if r["expected_single_line"] is True:
            print(f"  {r['name'][:50]:50s}  new_ratio={r['new_ratio']:.4f}  old_ratio={r['old_ratio']:.4f}")
    print()

    print("Wrapping cases (ratio should be >1.0):")
    for r in results:
        if r["expected_single_line"] is False:
            print(f"  {r['name'][:50]:50s}  new_ratio={r['new_ratio']:.4f}  old_ratio={r['old_ratio']:.4f}")
    print()

    # Check for tight_single samples (0.85-1.0) ratio target
    tight_single = [r for r in results if r["expected_single_line"] is True and r["new_ratio"] >= 0.80]
    if tight_single:
        avg_new_ratio = sum(r["new_ratio"] for r in tight_single) / len(tight_single)
        avg_old_ratio = sum(r["old_ratio"] for r in tight_single) / len(tight_single)
        print(f"Average ratio for near-full single lines:")
        print(f"  New model: {avg_new_ratio:.4f}")
        print(f"  Old model: {avg_old_ratio:.4f}")
        print(f"  Target: ~0.90-0.98 (some margin but mostly filled)")


if __name__ == "__main__":
    main()

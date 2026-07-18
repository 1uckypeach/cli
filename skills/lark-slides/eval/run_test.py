#!/usr/bin/env python3
from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
import run  # noqa: E402


VALID_SLIDE = '''<slide xmlns="http://www.larkoffice.com/sml/2.0"><data><shape type="text" topLeftX="60" topLeftY="60" width="840" height="100"><content autoFit="normal-auto-fit" textType="title" fontSize="36"><p>Quality loop</p></content></shape></data></slide>'''


class QualityLoopTest(unittest.TestCase):
    def test_static_mode_writes_report_and_review_template(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            xml_path = root / "slide.xml"
            output_dir = root / "out"
            xml_path.write_text(VALID_SLIDE, encoding="utf-8")
            exit_code = run.main(["--input", str(xml_path), "--mode", "static", "--output-dir", str(output_dir)])
            self.assertEqual(exit_code, 0)
            report = json.loads((output_dir / "report.json").read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "static_passed")
            self.assertEqual(report["static"]["summary"]["error_count"], 0)
            self.assertIn("Pass only when", (output_dir / "review.md").read_text(encoding="utf-8"))

    def test_static_errors_block_render_or_live(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            xml_path = root / "bad.xml"
            output_dir = root / "out"
            xml_path.write_text("<slide><data><shape></data></slide>", encoding="utf-8")
            exit_code = run.main(["--input", str(xml_path), "--mode", "render", "--output-dir", str(output_dir)])
            self.assertEqual(exit_code, 1)
            report = json.loads((output_dir / "report.json").read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "blocked_by_static_errors")


if __name__ == "__main__":
    unittest.main()

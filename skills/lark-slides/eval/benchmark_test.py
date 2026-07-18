#!/usr/bin/env python3
from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).parent))
import benchmark  # noqa: E402


SLIDE = '''<slide xmlns="http://www.larkoffice.com/sml/2.0"><data><shape type="rect" topLeftX="0" topLeftY="0" width="960" height="540"/><shape type="text" topLeftX="80" topLeftY="80" width="600" height="80"><content autoFit="normal-auto-fit" fontSize="36"><p>Expected claim</p></content></shape></data></slide>'''


class BenchmarkTest(unittest.TestCase):
    def test_static_benchmark_enforces_manifest_contract(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            (root / "slide.xml").write_text(SLIDE, encoding="utf-8")
            (root / "cases.json").write_text(
                json.dumps({"name": "test", "cases": [{"id": "case", "xml": "slide.xml", "checks": {"min_elements": 2, "required_kinds": ["shape"], "required_text": ["Expected claim"]}}]}),
                encoding="utf-8",
            )
            output_dir = root / "out"
            self.assertEqual(benchmark.main(["--manifest", str(root / "cases.json"), "--output-dir", str(output_dir)]), 0)
            report = json.loads((output_dir / "benchmark.json").read_text(encoding="utf-8"))
            self.assertEqual(report["summary"]["case_count"], 1)
            self.assertEqual(report["summary"]["passed"], 1)
            self.assertEqual(report["summary"]["pass_rate"], 1)

    def test_static_benchmark_fails_missing_required_text(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            (root / "slide.xml").write_text(SLIDE, encoding="utf-8")
            (root / "cases.json").write_text(json.dumps({"cases": [{"id": "case", "xml": "slide.xml", "checks": {"required_text": ["missing"]}}]}), encoding="utf-8")
            self.assertEqual(benchmark.main(["--manifest", str(root / "cases.json"), "--output-dir", str(root / "out")]), 1)

    def test_render_benchmark_requires_and_records_a_screenshot(self) -> None:
        with tempfile.TemporaryDirectory(dir=Path.cwd()) as temp_dir:
            root = Path(temp_dir)
            (root / "slide.xml").write_text(SLIDE, encoding="utf-8")
            (root / "cases.json").write_text(json.dumps({"cases": [{"id": "case", "xml": "slide.xml"}]}), encoding="utf-8")
            with patch.object(benchmark, "run_cli", return_value={"data": {"screenshots": [{"path": "/tmp/case.png"}]}}):
                self.assertEqual(benchmark.main(["--manifest", str(root / "cases.json"), "--mode", "render", "--output-dir", str(root / "out")]), 0)
            report = json.loads((root / "out" / "benchmark.json").read_text(encoding="utf-8"))
            self.assertEqual(report["cases"][0]["screenshots"], ["/tmp/case.png"])

    def test_render_benchmark_skips_local_image_until_live_create(self) -> None:
        with tempfile.TemporaryDirectory(dir=Path.cwd()) as temp_dir:
            root = Path(temp_dir)
            (Path.cwd() / "hero.png").write_bytes(b"fixture")
            image_slide = '<slide xmlns="http://www.larkoffice.com/sml/2.0"><data><img src="@./hero.png" topLeftX="0" topLeftY="0" width="960" height="540"/></data></slide>'
            (root / "slide.xml").write_text(image_slide, encoding="utf-8")
            (root / "cases.json").write_text(json.dumps({"cases": [{"id": "case", "xml": "slide.xml"}]}), encoding="utf-8")
            with patch.object(benchmark, "run_cli") as run_cli:
                self.assertEqual(benchmark.main(["--manifest", str(root / "cases.json"), "--mode", "render", "--output-dir", str(root / "out")]), 0)
                run_cli.assert_not_called()
            report = json.loads((root / "out" / "benchmark.json").read_text(encoding="utf-8"))
            self.assertEqual(report["cases"][0]["render_skipped_reason"], "local_image_placeholder_requires_live_create")
            (Path.cwd() / "hero.png").unlink()

    def test_static_benchmark_fails_missing_local_image(self) -> None:
        with tempfile.TemporaryDirectory(dir=Path.cwd()) as temp_dir:
            root = Path(temp_dir)
            image_slide = '<slide xmlns="http://www.larkoffice.com/sml/2.0"><data><img src="@./missing.png" topLeftX="0" topLeftY="0" width="960" height="540"/></data></slide>'
            (root / "slide.xml").write_text(image_slide, encoding="utf-8")
            (root / "cases.json").write_text(json.dumps({"cases": [{"id": "case", "xml": "slide.xml"}]}), encoding="utf-8")
            self.assertEqual(benchmark.main(["--manifest", str(root / "cases.json"), "--output-dir", str(root / "out")]), 1)

    def test_live_benchmark_requires_explicit_write_confirmation(self) -> None:
        with tempfile.TemporaryDirectory(dir=Path.cwd()) as temp_dir:
            root = Path(temp_dir)
            (root / "slide.xml").write_text(SLIDE, encoding="utf-8")
            (root / "cases.json").write_text(json.dumps({"cases": [{"id": "case", "xml": "slide.xml"}]}), encoding="utf-8")
            with self.assertRaises(SystemExit):
                benchmark.main(["--manifest", str(root / "cases.json"), "--mode", "live", "--output-dir", str(root / "out")])


if __name__ == "__main__":
    unittest.main()

import tempfile
import unittest
from pathlib import Path

from clean import clean_unused_files


PRESENTATION_NS = "http://schemas.openxmlformats.org/presentationml/2006/main"
REL_NS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
PACKAGE_REL_NS = "http://schemas.openxmlformats.org/package/2006/relationships"


class CleanTest(unittest.TestCase):
    def test_clean_preserves_referenced_slide_with_rewritten_prefixes(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            unpacked = Path(temp_dir)
            self._write_minimal_pptx_unpacked(unpacked, include_slide_rel=True)

            removed = clean_unused_files(unpacked)

            self.assertTrue((unpacked / "ppt/slides/slide1.xml").exists())
            self.assertFalse((unpacked / "ppt/slides/slide2.xml").exists())
            self.assertIn("ppt/slides/slide2.xml", removed)

    def test_clean_refuses_to_delete_when_slide_reference_cannot_be_resolved(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            unpacked = Path(temp_dir)
            self._write_minimal_pptx_unpacked(unpacked, include_slide_rel=False)

            with self.assertRaises(ValueError):
                clean_unused_files(unpacked)

            self.assertTrue((unpacked / "ppt/slides/slide1.xml").exists())
            self.assertTrue((unpacked / "ppt/slides/slide2.xml").exists())

    def _write_minimal_pptx_unpacked(
        self, unpacked: Path, include_slide_rel: bool
    ) -> None:
        (unpacked / "ppt/_rels").mkdir(parents=True)
        (unpacked / "ppt/slides").mkdir(parents=True)

        (unpacked / "ppt/presentation.xml").write_text(
            f"""<?xml version="1.0" encoding="UTF-8"?>
<ns0:presentation xmlns:ns0="{PRESENTATION_NS}" xmlns:ns1="{REL_NS}">
  <ns0:sldIdLst>
    <ns0:sldId id="256" ns1:id="rId1"/>
  </ns0:sldIdLst>
</ns0:presentation>
""",
            encoding="utf-8",
        )

        rel_entries = []
        if include_slide_rel:
            rel_entries.append(
                '<Relationship Id="rId1" '
                'Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" '
                'Target="slides/slide1.xml"/>'
            )
        rel_entries.append(
            '<Relationship Id="rId2" '
            'Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" '
            'Target="slides/slide2.xml"/>'
        )
        (unpacked / "ppt/_rels/presentation.xml.rels").write_text(
            f"""<?xml version="1.0" encoding="UTF-8"?>
<ns2:Relationships xmlns:ns2="{PACKAGE_REL_NS}">
  {' '.join(entry.replace("<Relationship", "<ns2:Relationship") for entry in rel_entries)}
</ns2:Relationships>
""",
            encoding="utf-8",
        )

        for slide_name in ["slide1.xml", "slide2.xml"]:
            (unpacked / "ppt/slides" / slide_name).write_text(
                "<p:sld/>", encoding="utf-8"
            )

        (unpacked / "[Content_Types].xml").write_text(
            """<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
  <Override PartName="/ppt/slides/slide2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>
""",
            encoding="utf-8",
        )


if __name__ == "__main__":
    unittest.main()

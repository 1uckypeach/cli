#!/usr/bin/env python3
"""Build test case slides and create them via lark-cli."""
import json
import subprocess
from pathlib import Path

PRES = "IvRosVAsWlTG7cdyLSecIHdanzb"
IMG = "PsZsbeRmhoXQ0Wx1bcScsjnunvf"
NS = 'xmlns="http://www.larkoffice.com/sml/2.0"'
ROOT = Path("tmp/slides_image_lint_whitebox")
CASES_DIR = ROOT / "cases"
CASES_DIR.mkdir(parents=True, exist_ok=True)


def slide(inner: str) -> str:
    return f'<slide {NS}><data>{inner}</data></slide>'


# H1: real overlap. Text at (100,100,400,80) "HELLO WORLD" fs=32; image (100,120,120,40) after
CASES = {}

CASES["H1"] = slide(
    '<shape id="h1_text" type="text" topLeftX="100" topLeftY="100" width="400" height="80">'
    '<content textType="body" fontSize="32"><p>HELLO WORLD</p></content>'
    '</shape>'
    f'<img id="h1_img" src="{IMG}" topLeftX="100" topLeftY="120" width="120" height="40"/>'
)

# H2: text width=300 short "Hi" fs=32, image at right blank (250,100,100,80)
CASES["H2"] = slide(
    '<shape id="h2_text" type="text" topLeftX="100" topLeftY="100" width="300" height="80">'
    '<content textType="body" fontSize="32"><p>Hi</p></content>'
    '</shape>'
    f'<img id="h2_img" src="{IMG}" topLeftX="250" topLeftY="100" width="100" height="80"/>'
)

# H3: same as H1 but img alpha=0
CASES["H3"] = slide(
    '<shape id="h3_text" type="text" topLeftX="100" topLeftY="100" width="400" height="80">'
    '<content textType="body" fontSize="32"><p>HELLO WORLD</p></content>'
    '</shape>'
    f'<img id="h3_img" src="{IMG}" topLeftX="100" topLeftY="120" width="120" height="40" alpha="0"/>'
)

# H4: image XML order BEFORE text, geometry overlaps
CASES["H4"] = slide(
    f'<img id="h4_img" src="{IMG}" topLeftX="100" topLeftY="120" width="120" height="40"/>'
    '<shape id="h4_text" type="text" topLeftX="100" topLeftY="100" width="400" height="80">'
    '<content textType="body" fontSize="32"><p>HELLO WORLD</p></content>'
    '</shape>'
)

# H5: paddingLeft on content, image only in left padding
CASES["H5"] = slide(
    '<shape id="h5_text" type="text" topLeftX="100" topLeftY="100" width="400" height="80">'
    '<content textType="body" fontSize="32" paddingLeft="200"><p>Hi</p></content>'
    '</shape>'
    f'<img id="h5_img" src="{IMG}" topLeftX="110" topLeftY="120" width="80" height="40"/>'
)

# H6: textAlign right, short text, image on left blank
CASES["H6"] = slide(
    '<shape id="h6_text" type="text" topLeftX="100" topLeftY="100" width="400" height="80">'
    '<content textType="body" fontSize="32" textAlign="right"><p>Hi</p></content>'
    '</shape>'
    f'<img id="h6_img" src="{IMG}" topLeftX="110" topLeftY="120" width="100" height="40"/>'
)

# H7: verticalAlign bottom, image only at top area
CASES["H7"] = slide(
    '<shape id="h7_text" type="text" topLeftX="100" topLeftY="100" width="400" height="80">'
    '<content textType="body" fontSize="32" verticalAlign="bottom"><p>Hi</p></content>'
    '</shape>'
    f'<img id="h7_img" src="{IMG}" topLeftX="100" topLeftY="100" width="400" height="20"/>'
)

# H8: wrap=false narrow shape, long text, image on overflow area outside frame
CASES["H8"] = slide(
    '<shape id="h8_text" type="text" topLeftX="100" topLeftY="100" width="100" height="50">'
    '<content textType="body" fontSize="24" wrap="false"><p>AAAAAAAAAAAAAAAAAAAA</p></content>'
    '</shape>'
    f'<img id="h8_img" src="{IMG}" topLeftX="230" topLeftY="110" width="80" height="30"/>'
)

# H9: paragraph textAlign=center probe
CASES["H9"] = slide(
    '<shape id="h9_text" type="text" topLeftX="100" topLeftY="100" width="400" height="80">'
    '<content textType="body" fontSize="32"><p textAlign="center">Hi</p></content>'
    '</shape>'
    f'<img id="h9_img" src="{IMG}" topLeftX="110" topLeftY="120" width="80" height="40"/>'
)

# H10: span fontSize probe
CASES["H10"] = slide(
    '<shape id="h10_text" type="text" topLeftX="100" topLeftY="100" width="400" height="80">'
    '<content textType="body" fontSize="16"><p><span fontSize="42">H</span></p></content>'
    '</shape>'
    f'<img id="h10_img" src="{IMG}" topLeftX="120" topLeftY="115" width="80" height="50"/>'
)

# V1: vertical text vert="vert", image after, intersects
CASES["V1"] = slide(
    '<shape id="v1_text" type="text" vert="vert" topLeftX="100" topLeftY="100" width="60" height="200">'
    '<content textType="body" fontSize="24"><p>ABC</p></content>'
    '</shape>'
    f'<img id="v1_img" src="{IMG}" topLeftX="110" topLeftY="110" width="40" height="120"/>'
)

# V2a: vert="vert270"
CASES["V2a"] = slide(
    '<shape id="v2a_text" type="text" vert="vert270" topLeftX="100" topLeftY="100" width="60" height="200">'
    '<content textType="body" fontSize="24"><p>ABC</p></content>'
    '</shape>'
    f'<img id="v2a_img" src="{IMG}" topLeftX="110" topLeftY="110" width="40" height="120"/>'
)

# V2b: vert="word-art-vert"
CASES["V2b"] = slide(
    '<shape id="v2b_text" type="text" vert="word-art-vert" topLeftX="100" topLeftY="100" width="60" height="200">'
    '<content textType="body" fontSize="24"><p>ABC</p></content>'
    '</shape>'
    f'<img id="v2b_img" src="{IMG}" topLeftX="110" topLeftY="110" width="40" height="120"/>'
)

# V2c: vert="word-art-vert-rtl"
CASES["V2c"] = slide(
    '<shape id="v2c_text" type="text" vert="word-art-vert-rtl" topLeftX="100" topLeftY="100" width="60" height="200">'
    '<content textType="body" fontSize="24"><p>ABC</p></content>'
    '</shape>'
    f'<img id="v2c_img" src="{IMG}" topLeftX="110" topLeftY="110" width="40" height="120"/>'
)

# V2d: vert="ea-vert"
CASES["V2d"] = slide(
    '<shape id="v2d_text" type="text" vert="ea-vert" topLeftX="100" topLeftY="100" width="60" height="200">'
    '<content textType="body" fontSize="24"><p>ABC</p></content>'
    '</shape>'
    f'<img id="v2d_img" src="{IMG}" topLeftX="110" topLeftY="110" width="40" height="120"/>'
)

# V3: image BEFORE text with vert
CASES["V3"] = slide(
    f'<img id="v3_img" src="{IMG}" topLeftX="110" topLeftY="110" width="40" height="120"/>'
    '<shape id="v3_text" type="text" vert="vert" topLeftX="100" topLeftY="100" width="60" height="200">'
    '<content textType="body" fontSize="24"><p>ABC</p></content>'
    '</shape>'
)


def create_slide(case_id: str, content_xml: str):
    Path(CASES_DIR / f"{case_id}.xml").write_text(content_xml, encoding="utf-8")
    body = json.dumps({"slide": {"content": content_xml}}, ensure_ascii=False)
    cmd = [
        "lark-cli", "slides", "xml_presentation.slide", "create",
        "--as", "user",
        "--params", json.dumps({"xml_presentation_id": PRES}),
        "--data", body,
    ]
    proc = subprocess.run(cmd, capture_output=True, text=True)
    out = proc.stdout.strip()
    try:
        data = json.loads(out)
    except Exception:
        return {"case": case_id, "ok": False, "raw": out, "stderr": proc.stderr}
    return {"case": case_id, "resp": data, "stderr": proc.stderr}


results = []
for case_id in ["H1", "H2", "H3", "H4", "H5", "H6", "H7", "H8", "H9", "H10", "V1", "V2a", "V2b", "V2c", "V2d", "V3"]:
    r = create_slide(case_id, CASES[case_id])
    results.append(r)
    print(json.dumps(r, ensure_ascii=False))

Path(ROOT / "create_results.json").write_text(json.dumps(results, indent=2, ensure_ascii=False), encoding="utf-8")

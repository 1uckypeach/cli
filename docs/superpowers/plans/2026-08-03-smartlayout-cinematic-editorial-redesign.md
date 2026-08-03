# SmartLayout Cinematic Editorial Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 创建并验证一份 8 页《双峰：回归》高密度电影杂志风 Slides，使四种公开 smartLayout 融入复杂画面而不是呈现为功能组件演示。

**Architecture:** 使用 `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/` 保存规划、逐页 XML、创建结果、回读与验证记录；使用现有七张影片素材并补充一张不重复素材。每页先通过 XSD/lint，再用 `lark-cli slides +create` 创建真实演示文稿，随后回读 XML、获取 8/8 截图并根据视觉问题迭代。

**Tech Stack:** Lark SML 2.0 XML、`lark-cli slides`、Python `xml_text_overlap_lint.py`、`jq`、服务端 Slides 截图。

---

### Task 1: Establish execution state and asset manifest

**Files:**
- Read: `skills/lark-slides/SKILL.md`
- Read: `skills/lark-shared/SKILL.md`
- Read: `skills/lark-slides/references/diagram-layouts.md`
- Read: `skills/lark-slides/references/xml-schema-quick-ref.md`
- Read: `skills/lark-slides/references/planning-layer.md`
- Read: `skills/lark-slides/references/visual-planning.md`
- Read: `skills/lark-slides/references/asset-planning.md`
- Read: `skills/lark-slides/references/lark-slides-create.md`
- Read: `skills/lark-slides/references/lark-slides-screenshot.md`
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/asset-manifest.json`

- [ ] **Step 1: Verify CLI and user authorization**

Run:

```bash
LARKSUITE_CLI_NO_UPDATE_NOTIFIER=1 \
LARKSUITE_CLI_NO_SKILLS_NOTIFIER=1 \
lark-cli auth status --json --verify
```

Expected: exit 0, `identity == "user"`, `verified == true`, and user scopes include Slides create/read/update/screenshot.

- [ ] **Step 2: Inventory current image assets**

Run:

```bash
sips -g pixelWidth -g pixelHeight \
  .lark-slides/assets/twin-peaks-return-20260803/*.jpg
```

Expected: seven readable JPEG files with non-zero dimensions.

- [ ] **Step 3: Add one non-repeating eighth asset**

Select one additional high-resolution official poster or scene still that is visually distinct from the existing seven files. Save it as:

```text
.lark-slides/assets/twin-peaks-smartlayout-v3-20260803/08-structure.jpg
```

Acceptance: JPEG/PNG, at least 960 px wide or 720 px high, under 20 MB, no watermarked editorial overlay obscuring the subject.

- [ ] **Step 4: Write exact asset-to-page mapping**

Create `asset-manifest.json` with this mapping:

```json
{
  "1": "01-dale-poster.jpg",
  "2": "08-structure.jpg",
  "3": "02-laura-poster.jpg",
  "4": "03-cooper-wide.jpg",
  "5": "04-red-room.jpg",
  "6": "05-cooper-still.jpg",
  "7": "06-casino.jpg",
  "8": "07-finale.jpg"
}
```

Expected: eight unique filenames and no repeated page assignment.

### Task 2: Build the deck planning layer

**Files:**
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide_plan.json`

- [ ] **Step 1: Write the 8-page plan**

Use the required planning schema and these page contracts:

| Page | Role | Required structure | Asset |
|---|---|---|---|
| 1 | Cover | Full-bleed image, title, one thesis | 01 |
| 2 | 18-hour anatomy | Large number `18`, three viewing chapters, 3–4 annotations | 08 |
| 3 | Reality layers | `solid-pyramid`, four cells | 02 |
| 4 | Cooper identities | `step-pyramid`, four cells | 03 |
| 5 | Mystery escalation | `filled-up-step`, five cells | 04 |
| 6 | Time loop | Five-node edge-to-edge cycle, no smartLayout | 05 |
| 7 | Viewing rhythm | `linear-up-step`, five cells and three micro-annotations | 06 |
| 8 | Conclusion | Full-bleed image, thesis, four keywords | 07 |

Use these exact smartLayout labels:

```text
solid-pyramid: 日常现实 / 调查迷雾 / 黑色小屋 / 不可解释
step-pyramid: 特工 Cooper / Mr. C / Dougie / 残缺回归
filled-up-step: 小镇异响 / 身份裂缝 / 时间错位 / 红房回响 / 现实崩解
linear-up-step: 重新进入 / 适应停顿 / 接受荒诞 / 感知裂缝 / 重返谜面
```

Expected: plan has eight slides, four unique smartLayout types, unique asset paths, and no implementation-facing copy such as `layoutType=` or `field test`.

- [ ] **Step 2: Validate planning completeness**

Run:

```bash
jq -e '
  (.slides | length) == 8 and
  ([.slides[].asset_need] | length) == 8 and
  ([.slides[].key_message | length > 0] | all)
' .lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide_plan.json
```

Expected: `true` and exit 0.

### Task 3: Author editorial pages 1 and 2

**Files:**
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-01.xml`
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-02.xml`

- [ ] **Step 1: Create page 1 as a full-bleed cinematic cover**

Required XML composition:

```text
img: x=0 y=0 w=960 h=540
dark gradient scrim: cover 55%-65% of the left side
title: “双峰：回归” at 38-46 pt
subtitle: “迟到二十五年的梦境，正在重新吞没现实。” at 15-18 pt
one small date/series label only
```

Do not add smartLayout, card containers, protocol labels, source footers, or a separate right-side image frame.

- [ ] **Step 2: Create page 2 as a dense editorial anatomy page**

Required visible content:

```text
18
部 · 约 18 小时
第一章：重返熟悉之地
第二章：身份与时间失真
第三章：回到一个错误的起点
“它不是电视剧被拉长，而是一部电影拒绝结束。”
```

Use one large image crop occupying at least 45% of the page, one large number, three compact chapter blocks, and one pull quote. Avoid large unfilled pure-color regions.

- [ ] **Step 3: Run per-page lint**

Run:

```bash
for n in 01 02; do
  python3 skills/lark-slides/scripts/xml_text_overlap_lint.py \
    --input ".lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-${n}.xml" \
    > ".lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-${n}-lint.json"
done
```

Expected: both files have `summary.error_count == 0`.

### Task 4: Author smartLayout pages 3–5

**Files:**
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-03.xml`
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-04.xml`
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-05.xml`

- [ ] **Step 1: Create page 3 with `solid-pyramid`**

Use four cells in this order:

```text
不可解释 / 黑色小屋 / 调查迷雾 / 日常现实
```

Layout requirements:

```text
full-page or 60%+ image background
smartLayout occupies 35%-45% of page
two micro-annotations: “地点失去稳定性” and “梦开始改写记忆”
one 18-22 pt editorial statement distinct from the page title
```

- [ ] **Step 2: Create page 4 with `step-pyramid`**

Use four cells:

```text
特工 Cooper / Mr. C / Dougie / 残缺回归
```

Add two identity notes and one large cropped character image. The step pyramid must overlap only dark image space and must not cover a face.

- [ ] **Step 3: Create page 5 with `filled-up-step`**

Use five cells:

```text
小镇异响 / 身份裂缝 / 时间错位 / 红房回响 / 现实崩解
```

Use gray-blue-to-red custom colors. Add three compact annotations for space, identity, and time. The smartLayout should occupy the lower or middle third of the image, not a separate white card.

- [ ] **Step 4: Run per-page lint**

Run:

```bash
for n in 03 04 05; do
  python3 skills/lark-slides/scripts/xml_text_overlap_lint.py \
    --input ".lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-${n}.xml" \
    > ".lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-${n}-lint.json"
done
```

Expected: three pages have `error_count == 0`; warnings require screenshot review, not automatic dismissal.

### Task 5: Author pages 6–8 and the smartLayout boundary case

**Files:**
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-06.xml`
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-07.xml`
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-08.xml`

- [ ] **Step 1: Create page 6 as a five-node time loop**

Use these nodes:

```text
回到故乡 → 身份错位 → 记忆复现 → 时间断裂 → 再次返回
```

Each connector starts and ends at the edge of its node. No line may pass through a label. Include one large image crop and two annotations; do not mention layout implementation in visible slide copy.

- [ ] **Step 2: Create page 7 with `linear-up-step`**

Use five cells:

```text
重新进入 / 适应停顿 / 接受荒诞 / 感知裂缝 / 重返谜面
```

Add these three micro-annotations outside smartLayout:

```text
长镜头把等待变成压力
环境噪声让现实显出裂缝
Roadhouse 像每集醒来的梦尾
```

The background image occupies at least 55%; the linear smartLayout is embedded into a darkened lower region.

- [ ] **Step 3: Create page 8 as a full-bleed conclusion**

Required content:

```text
关系先于布局
层级 / 成熟度 / 升级 / 节奏
当关系不匹配，最好的 smartLayout 是不用它。
```

Do not include validation text, source footers, tool names, or technical labels.

- [ ] **Step 4: Run per-page lint**

Run the lint for pages 06–08 and require `error_count == 0`.

### Task 6: Pre-render and perform the first visual gate

**Files:**
- Create: `.lark-slides/screenshots/twin-peaks-smartlayout-v3-20260803/preflight/*.png`
- Modify: any `slide-XX.xml` that fails visual inspection

- [ ] **Step 1: Render all eight XML pages through Slides**

Run one `slides +screenshot --content @slide-XX.xml` command per page with output names `slide-01` through `slide-08`.

Expected: eight image files. Local image placeholders may remain blank in render mode; smartLayout and text geometry must still render.

- [ ] **Step 2: Review all eight preflight screenshots**

Reject a page if any of the following is true:

```text
smartLayout looks like an isolated demo component
less than three visible information layers
large empty pure-color area with no intentional image tension
repeated right-image/left-component template
cell title clipped or unreadable
technical validation copy visible
```

- [ ] **Step 3: Repair every rejected page and re-render it**

Repeat until every page is visually acceptable before creating the permanent deck.

### Task 7: Create the real presentation

**Files:**
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/create-result.json`

- [ ] **Step 1: Create with eight XML files and image placeholders**

Use `jq --rawfile` for `s1` through `s8`, then call:

```bash
lark-cli slides +create --as user \
  --title '双峰：回归｜高密度 smartLayout 视觉实验' \
  --slides "$(jq -n \
    --rawfile s1 .lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-01.xml \
    --rawfile s2 .lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-02.xml \
    --rawfile s3 .lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-03.xml \
    --rawfile s4 .lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-04.xml \
    --rawfile s5 .lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-05.xml \
    --rawfile s6 .lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-06.xml \
    --rawfile s7 .lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-07.xml \
    --rawfile s8 .lark-slides/plan/twin-peaks-smartlayout-v3-20260803/slide-08.xml \
    '[$s1,$s2,$s3,$s4,$s5,$s6,$s7,$s8]')"
```

Expected: `ok == true`, `slides_added == 8`, `images_uploaded == 8`, and a returned `xml_presentation_id` plus URL.

- [ ] **Step 2: Record the returned IDs**

Save the complete JSON envelope. Do not retry creation after a partial failure without first reading current presentation state.

### Task 8: Read back, screenshot 8/8, and repair

**Files:**
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/readback.xml`
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/readback-lint.json`
- Create: `.lark-slides/screenshots/twin-peaks-smartlayout-v3-20260803/final/*.jpg`
- Create: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/visual-review.md`

- [ ] **Step 1: Read back the full presentation**

Run `slides +xml-get --as user` with the returned presentation ID and the exact output path above.

- [ ] **Step 2: Verify structural invariants**

Run:

```bash
READBACK=.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/readback.xml
test "$(rg -o '<slide\b' "$READBACK" | wc -l | tr -d ' ')" = 8
test "$(rg -o '<smartLayout\b' "$READBACK" | wc -l | tr -d ' ')" = 4
test "$(rg -o 'src="@' "$READBACK" | wc -l | tr -d ' ')" = 0
for type in solid-pyramid step-pyramid filled-up-step linear-up-step; do
  test "$(rg -o "layoutType=\"$type\"" "$READBACK" | wc -l | tr -d ' ')" = 1
done
```

Expected: all commands exit 0.

- [ ] **Step 3: Lint the readback**

Expected: `error_count == 0`. Any warning must be mapped to a screenshot page and explicitly reviewed.

- [ ] **Step 4: Capture all eight server screenshots**

Call `slides +screenshot` with slide numbers 1–8 in one request and save to the final screenshot directory.

- [ ] **Step 5: Write a page-by-page visual review**

For every page record `pass` or `fix`, plus findings for density, image rendering, typography, smartLayout integration, clipping, overlap, contrast, and template repetition.

- [ ] **Step 6: Repair every `fix` page**

Use the existing deck edit path. Re-read `lark-slides-edit-workflows.md`; prefer `+replace-pages` for whole-page structural corrections. Re-capture only repaired pages and update `visual-review.md` after inspecting the new screenshots.

### Task 9: Final verification and handoff

**Files:**
- Read: `skills/lark-slides/references/validation-checklist.md`
- Read: `.lark-slides/plan/twin-peaks-smartlayout-v3-20260803/visual-review.md`

- [ ] **Step 1: Confirm completion evidence**

Required evidence:

```text
8/8 pages in readback
4/4 layout types exactly once
8 unique uploaded images
0 local image placeholders
0 lint errors
8/8 screenshots actually inspected
0 pages left with status fix
```

- [ ] **Step 2: Deliver the Slides URL and representative screenshots**

Show the URL, the four smartLayout pages, the time-loop boundary page, and concise verification results. Do not claim “高级” solely from test success; state the observed visual findings.

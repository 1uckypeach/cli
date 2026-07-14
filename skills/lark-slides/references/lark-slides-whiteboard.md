# Whiteboard artboard element

`<whiteboard>` is placed inside `<data>`, which can contain **SVG** or **Mermaid**, which is used to draw flow charts, sequence diagrams, architecture diagrams, scatter plots, funnel charts, custom icons, decorative patterns, etc. visual content that is difficult to cover with `<chart>` and `<shape>`.

Ordinary column charts, bar charts, line charts, area charts, radar charts, pie charts/donut charts and combination charts should give priority to using the native `<chart>`. Do not redraw these standard charts with `<whiteboard>` + SVG/Mermaid unless the user explicitly requires pixel-level customization, or the chart type is truly not supported by `<chart>`.

> Prerequisite: Read [lark-slides SKILL.md](../SKILL.md) before using this document.

---

## `<chart>` or `<whiteboard>`?

**Determine the content type first before entering this document:**

| Scene | Recommended elements |
|------|---------|
| Column/bar/polyline/area/radar/pie/ring/combination chart with structured data sequence | `<chart>` — native rendering, supports legend / tooltip / series color matching |
| Scatter plots, funnel charts (not supported by `<chart>`), or other non-native data visuals | `<whiteboard>` SVG |
| Flow charts, sequence diagrams, architecture diagrams, class diagrams, ER diagrams and other topology diagrams | `<whiteboard>` Mermaid or SVG |
| Custom icons, logos, schematic graphics (requires path/polygon precise control) | `<whiteboard>` SVG |
| Progress bars, wavy backgrounds, decorative patterns, pixel-level custom visualization | `<whiteboard>` SVG |

> Use `<chart>` for content that is suitable for `<chart>`, and do not use SVG/Mermaid hand-drawing - native rendering is less labor-intensive, the structure is more stable, and it is easier to be read back and subsequently edited.

---

## whiteboard public property

| Properties | Required | Description |
|------|------|------|
| `topLeftX` | Yes | X coordinate of the upper left corner (slide coordinate system, slide default width is 960) |
| `topLeftY` | Yes | Y coordinate of the upper left corner (slide coordinate system, slide default height is 540) |
| `width` | Yes | Artboard width (pixels) |
| `height` | Yes | Artboard height (pixels) |

> In SVG mode, `<svg>` needs to declare `xmlns="http://www.w3.org/2000/svg"`; the content size is determined by the bounding box of the child element, and `width`/`height`/`viewBox` does not affect rendering (only when the element attribute uses a percentage value, `viewBox` is required to provide a calculation basis). Mermaid mode requires no additional attributes.

The coordinates in the SVG are relative to the upper left corner (0,0) of the whiteboard itself and have nothing to do with the slide coordinate system.

---

## SVG or Mermaid?

The selection is divided into three steps: **First exclude the native `<chart>`, then determine the whiteboard type, and finally look at the current model identity**.

### Step one: Confirm whether you should use `<chart>`

If the content is a column chart, bar chart, line chart, area chart, radar chart, pie/donut chart, or combination chart, return to using the native `<chart>` and do not continue to apply the SVG / Mermaid path of this document.

### Step 2: Whiteboard type priority judgment

The following types **recommend Mermaid**, with automatic layout and concise code; if you need to accurately match the brand color or customize the node style, you can use SVG instead:

| Chart Types | Mermaid Keywords |
|----------|--------------|
| Flowchart, decision tree, architecture diagram | `flowchart TD` / `flowchart LR` |
| Sequence diagram | `sequenceDiagram` |
| Class Diagram | `classDiagram` |
| Gantt chart | `gantt` |
| State Diagram | `stateDiagram-v2` |
| Mind map | `mindmap` |
| ER Diagram | `erDiagram` |

### Step 3: Select paths for non-native charts and decorative elements based on model identity

Scenes other than the above table (scatter charts, funnel charts, progress bars, timelines, wave backgrounds, star point textures, etc.) require precise control of coordinates and color matching. SVG is more expressive, but the ability of each model to generate SVG is different:

| Model Identity | Path |
|----------|------|
| Claude / Gemini / GPT / GLM | **SVG** — precise control of coordinates, color, transparency |
| Doubao / Seed / Other | **Mermaid** — Use `gantt`, `flowchart`, etc. to approximate expression; only fall back to simple SVG rectangle/line when it is really impossible to express with Mermaid |

> **Report your identity first and then choose a path**: Before deciding to use SVG, confirm which category the current model belongs to. Don't skip this step.

---

## Mode 1: SVG

### ⚠️ Design quality requirements

The purpose of embedding `<whiteboard>` in slides is to express visual relationships that are difficult to cover with native `<chart>` or basic `<shape>`, rather than hand-drawing standard data charts.

- **Don’t just use rectangles and text to deal with it**: Pure white background + squares + black text throughout the article equals nothing, which is a failed output.
- **Non-native data vision must have a coordinate system**: Scatter points, funnels, etc. must still have the necessary coordinate axes, scales, numerical labels or segmented descriptions, do not just draw points or color blocks
- **Font sizes must have levels**: title ≠ label ≠ value. Mixing the same font size will eliminate visual focus.
- **Color matching should echo the slide theme**: Use transparent background or dark cards for charts on dark slide backgrounds; avoid adding pure white background blocks on light backgrounds
- **Every whiteboard is a design opportunity**: Actively use details such as rounded corners, translucent filling, clear grouping, node status, etc. to widen the gap with the default template
- **Judge the background brightness before writing SVG**: When the background brightness is < 30%, "insufficient contrast" of decorative elements is more harmful than "excessive contrast", so it is better to be heavy than light;
- **Decoration levels use brightness jumps instead of linear stacking transparency**: The arithmetic increment of `α=0.04→0.08→0.12` has almost no difference on a dark background (the brightness difference of adjacent layers is ≈20); the correct approach is to use non-linear jumps such as `0.10→0.40→0.70→1.0`, and the brightness difference of adjacent layers is ≥60.

### grammar

```xml
<whiteboard width="400" height="300" topLeftX="500" topLeftY="120">
  <svg xmlns="http://www.w3.org/2000/svg">
    <rect x="50" y="50" width="80" height="200" rx="4" fill="rgba(59,130,246,0.85)"/>
    <text x="90" y="270" text-anchor="middle" font-size="12" fill="rgba(100,116,139,1)">ABC</text>
  </svg>
</whiteboard>
```

`<svg>` needs to declare `xmlns="http://www.w3.org/2000/svg"`; `width`/`height`/`viewBox` does not need to be filled in. If the element attribute uses a percentage value, an additional `viewBox` needs to be declared.

### ⚠️ Rendering bounding box rules

When whiteboard is rendered, the combined result of the geometric bounding boxes of all child elements is used as the content area and is adaptively scaled to the container.

`width`, `height`, `viewBox` on `<svg>` do not affect the calculation of the content area, but `viewBox` has a practical purpose: **provide a basis for calculation of the percentage attribute**. If the element uses percentage values ​​such as `width="50%"`, a `viewBox` must be declared for correct parsing; there is no need to care about absolute coordinate elements. It is recommended to use absolute coordinates uniformly to avoid introducing percentage dependence.

### Supported SVG elements

| Element | Description | Typical uses |
|------|------|---------|
| `<rect>` | Rectangle, supports `rx` rounded corners | Cards, progress bars, segmented color blocks |
| `<circle>` | Circle | Node, decorative point, donut chart |
| `<ellipse>` | Ellipse | Custom outline graphics |
| `<line>` | straight line | axis, dividing line, connecting line |
| `<path>` | Any path (supports Q/C curve) | Wave, curve, arc |
| `<text>` | Text, supports Chinese | Label, value |
| `<polygon>` | Polygon | Arrow, star, area fill |
| `<g>` | Grouping | Batch transformation, semantic grouping |
| `<linearGradient>` | Linear gradient definition, used with `fill="url(#id)"` | Gradient background, gradient fill |

**Color:** Use `rgba(R,G,B,A)` uniformly, which is friendly to both dark and light backgrounds.
**Dash:** `stroke-dasharray="4,4"` for gridlines/axes.
**Transformation:** `transform="translate(x,y)"` / `rotate(deg cx cy)` / `scale(n)` are all supported.

---
### Element calculation

As long as batch positioning, equal spacing or data mapping is involved in SVG, it is recommended to run an additional Python script to calculate the coordinates and then fill them into the SVG instead of manual estimation. The scope of application includes scatter points, funnels, decorative dots, equally spaced circles, repeating patterns, etc.; ordinary bar charts, line charts, and pie charts should still return to the original `<chart>`.

> **Actively calculate**: Run the script before writing SVG, paste the output as a comment at the beginning of `<svg>`, and then fill in the coordinates accordingly. Valuation needs to be adjusted repeatedly almost every time, and skipping this step will make it slower.

**Scatter Plot/Ornamental Dot Pattern**

```python
W, H = 360, 260
origin_x, origin_y = 50, 216 # Lower left corner, SVG Y axis downward
cw, ch = 290, 184

points = [(12, 40), (28, 80), (45, 65)]
x_min, x_max, y_min, y_max = 0, 50, 0, 100
for i, (xv, yv) in enumerate(points):
    x = round(origin_x + (xv - x_min) / (x_max - x_min) * cw)
    y = round(origin_y - (yv - y_min) / (y_max - y_min) * ch)
    print(f"point-{i}: cx={x} cy={y}")
```

**Decorative elements (equal spacing paradigm)**

```python
n, total_w, cy, r = 8, 340, 40, 4
step = total_w / (n - 1)
for i in range(n):
    print(f"circle-{i}: cx={round(i * step)} cy={cy} r={r}")
```

**Maximum bounding box → whiteboard size**

After the coordinates of all elements are calculated, the overall bounding box is summarized and used directly as the `width`/`height` of the whiteboard:

```python
# Register (x, y, w, h) for each element, including stroke expansion
elements = [
    (10, 20, 80, 160),   # item-0
    (107, 10, 80, 170),  # item-1
    (204, 40, 80, 140),  # item-2
    (0, 0, 300, 1),      # x-axis
]

xs = [x for x, y, w, h in elements]
ys = [y for x, y, w, h in elements]
x2 = [x + w for x, y, w, h in elements]
y2 = [y + h for x, y, w, h in elements]

wb_w = max(x2) - min(xs)
wb_h = max(y2) - min(ys)
print(f"whiteboard width={wb_w} height={wb_h}")
```

The output is the value of `<whiteboard width=... height=...>`, no manual estimation is required.

---
### Layout mode

**Full screen decoration layer**
```xml
<whiteboard width="960" height="540" topLeftX="0" topLeftY="0">
  <svg xmlns="http://www.w3.org/2000/svg">
    ...
  </svg>
</whiteboard>
```

> ⚠️ Full-screen decoration whiteboard must be placed before all `<shape>` / `<img>` / `<table>`, otherwise the text content will be obscured. The later the element is in XML, the higher the rendering level.

**Sidebar chart (side by side with text shape)**
```xml
<!-- Text on the left -->
<shape type="text" topLeftX="60" topLeftY="120" width="500" height="340">...</shape>
<!-- Chart on the right -->
<whiteboard width="340" height="340" topLeftX="580" topLeftY="120">
  <svg xmlns="http://www.w3.org/2000/svg">
    ...
  </svg>
</whiteboard>
```

**Bottom decorative strip**
```xml
<whiteboard width="960" height="100" topLeftX="0" topLeftY="440">
  <svg xmlns="http://www.w3.org/2000/svg">
    ...
  </svg>
</whiteboard>
```

---

### Prohibited SVG features

The following features are not supported or behave unpredictably on the slide `<whiteboard>` rendering side and must be avoided:

| Banned | Reasons | Alternatives |
|------|------|---------|
| `<radialGradient>` | Rendering failed | Use `<linearGradient>` or `rgba()` transparency to simulate shades |
| `<filter>` (shadow, blur, etc.) | Rendering failed | Simulating shadows with translucent `<rect>` overlay |
| `<clipPath>` / `<mask>` | Rendering failed | Adjust element coordinates and size for natural cropping |
| `<pattern>` | Rendering failed | Manual paving `<circle>` / `<rect>` lattice |
| `skewX` / `skewY` / `matrix(...)` | Space distortion, degraded rendering | Use `rotate` + `translate` instead |
| `<image>` external link URL | External links are not supported | Upload the file_token first, then use the `<img>` element |

---


## Mode 2: Mermaid

### grammar

```xml
<whiteboard topLeftX="72" topLeftY="60" width="816" height="360">
  <mermaid>
    <![CDATA[
        flowchart TD
A[Check lark-cli and jq] --> B[Write slide XML for each page]
B --> C[Generate slides JSON through jq]
C --> D[execute slides +create]
D --> E[Read xml_presentation_id]
E --> F [Read back and verify the creation results]
    ]]>
  </mermaid>
</whiteboard>
```

**Key Points:**
- Content is wrapped with `<![CDATA[...]]>` - `[`, `>`, `-->` in Mermaid syntax are XML special characters, CDATA avoids escaping problems
- whiteboard only needs `topLeftX`, `topLeftY`, `width`, `height`

### Supported Mermaid chart types

| Type | Keywords | Applicable scenarios |
|------|--------|---------|
| Flowchart | `flowchart TD` / `flowchart LR` | Business process, decision tree, workflow |
| Sequence diagram | `sequenceDiagram` | System interaction, API call chain |
| Gantt chart | `gantt` | Project plan, milestones |
| Class diagram | `classDiagram` | Object relationship, architecture design |
| ER diagram | `erDiagram` | Database structure |
| State diagram | `stateDiagram-v2` | State machine, life cycle |
| Mind map | `mindmap` | Topic sorting, knowledge structure |
| User journey | `journey` | User experience path |

### Mermaid Layout Suggestions

Mermaid charts will automatically fill the whiteboard area. suggestion:
- Leave enough height for the flow chart. When there are many nodes, increase the height appropriately (for example, 400-480)
- Avoid placing more than 15 nodes on one page, and consider paging when the content is too dense.
- Recommended size reference:

| Chart type | Recommended width | Recommended height |
|---------|-----------|------------|
| Flowchart (5-8 nodes) | 720-816 | 300-400 |
| Sequence diagram (3-5 participants) | 720-816 | 320-420 |
| Gantt Chart | 816 | 280-360 |
| Mind Map | 816 | 380-480 |

---

## Notes & Known Issues

### z-order (SVG mode)

The position of the whiteboard in the XML determines the rendering level: before the shape → on the lower layer; after the shape → on the upper layer. The full-screen decorative whiteboard should be placed before all shapes.

### Mermaid CDATA Necessity

Mermaid syntax contains `[`, `>`, `-->`, and writing directly without CDATA will destroy XML parsing. Always use `<![CDATA[ ... ]]>`.

---

## Quick self-check checklist

**SVG Mode - Structure Check:**
- [ ] `<svg>` declares `xmlns="http://www.w3.org/2000/svg"`
- [ ] `width`/`height` of whiteboard is calculated from the maximum bounding box of all elements (including stroke expansion), no manual estimation is required
- [ ] `topLeftX + width ≤ 960`，`topLeftY + height ≤ 540`
- [ ] None `<radialGradient>` / `<filter>` / `<clipPath>`
- [ ] The text `y` coordinate is the baseline position, the minimum value is ≥ font-size (to avoid being cropped)

**SVG Mode - Visual Quality Check:**
- [ ] Non-native data visuals have necessary coordinate axes, grid lines, numerical labels or segmentation instructions, but no "bare points" or unexplained color blocks.
- [ ] There are levels of font sizes: title > value > axis label, not all the same
- [ ] Use the same color for a single data series, use different colors for multiple series with sufficient contrast.
- [ ] Axis labels and chart elements do not block each other, leaving enough space
- [ ] Coordinate derivation has comments (indicate originX/Y, chartW/H, data mapping formula)

**Mermaid Mode:**
- [ ] Content is wrapped in `<![CDATA[...]]>`
- [ ] CDATA terminator `]]>` does not appear in the Mermaid code itself
- [ ] `topLeftX + width ≤ 960`，`topLeftY + height ≤ 540`
- [ ] The number of nodes is reasonable (no more than 15-20 nodes in a single graph)

**General:**
- [ ] XML tags are all closed and attribute quotes are complete
- [ ] If it fails, check whether it is an accidental 5001000 and try again.

---

## refer to

- [lark-slides SKILL.md](../SKILL.md)

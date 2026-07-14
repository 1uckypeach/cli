# XML Format Guide

This document is organized based on [slides_xml_schema_definition.xml](slides_xml_schema_definition.xml) and explains the core structure and common writing methods of Feishu Slides XML Schema (SML 2.0).

## Basic structure

```xml
<?xml version="1.0" encoding="UTF-8"?>
<presentation xmlns="http://www.larkoffice.com/sml/2.0" width="960" height="540">
<title>Presentation title</title>
  <slide>
    <style>
      <fill>
        <fillColor color="rgb(245, 245, 245)"/>
      </fill>
    </style>
    <data>
      <shape type="text" topLeftX="80" topLeftY="80" width="800" height="120">
        <content textType="title">
<p>Main title</p>
        </content>
      </shape>
    </data>
    <note>
      <content textType="body">
<p>These are speaker notes. </p>
      </content>
    </note>
  </slide>
</presentation>
```

## Root element

### `<presentation>`

The protocol standard should be written with the namespace `http://www.larkoffice.com/sml/2.0`; the current server implementation may be compatible with input without `xmlns`, but this is not a protocol guarantee.

**property:**

| Properties | Type | Required | Description |
|------|------|------|------|
| `width` | positiveInteger | Yes | Presentation width, such as `960` |
| `height` | positiveInteger | Yes | Presentation height, such as `540` |
| `id` | string | no | presentation ID |

**Child element:**

| Element | Required | Description |
|------|------|------|
| `<title>` | No | Presentation title |
| `<theme>` | No | Global theme |
| `<slide>` | Yes | Slide pages, minimum 1 page, maximum 100 pages |

## theme

### `<theme>`

`<theme>` currently contains two parts:

- `<background>`: presentation-level background fill
- `<textStyles>`: theme text style collection

Optional sub-elements under `<textStyles>`:

- `<title>`
- `<headline>`
- `<sub-headline>`
- `<body>`
- `<caption>`

These elements define the theme's default style, not the page structure. Common properties:

| Properties | Description |
|------|------|
| `fontFamily` | Font |
| `fontSize` | Font size |
| `fontColor` | Font color |

## Slide elements

### `<slide>`

The structure of a single slide is relatively strict.

**property:**

| Properties | Type | Required | Description |
|------|------|------|------|
| `id` | string | no | slide ID |

**Direct child elements are only:**

| Element | Required | Description |
|------|------|------|
| `<style>` | No | Page style |
| `<data>` | No | Page element container |
| `<note>` | No | Speaker Notes |

This means that `<title>`, `<headline>`, `<body>`, `<caption>` cannot be placed directly under `<slide>`.

## Text content model

### `<content>`

The actual page text is usually expressed through `<content>`, common locations are:

- `shape` internal
- `table/td` internal
- `note` internal

**Commonly used properties:**

| Properties | Description |
|------|------|
| `textType` | `title` / `headline` / `sub-headline` / `body` / `caption` |
| `verticalAlign` | Vertical alignment |
| `textAlign` | Horizontal alignment |
| `lineSpacing` | Line spacing |
| `fontSize` | Font size |
| `fontFamily` | Font |
| `color` | Font color |
| `bold` / `italic` / `underline` / `strikethrough` | Content-level styles |
| `wrap` | Whether to wrap lines automatically |

**Sub-elements that can be included:**

- `<p>`
- `<ul>`
- `<ol>`

### `<p>`

`<p>` is a paragraph element that can mix plain text and inline tags:

- `<br/>`
- `<strong>`
- `<em>`
- `<u>`
- `<span>`
- `<del>`
- `<a>`
- `<shadow>`
- `<outline>`

Example:

```xml
<content textType="body" textAlign="left">
<p>Normal text <strong>Bold</strong> <em>Italic</em> <a href="https://example.com">Links</a></p>
  <ul>
<li><p>List item 1</p></li>
<li><p>List item 2</p></li>
  </ul>
</content>
```

## Common page elements

All page elements are placed in `<data>`.

### `<shape>`

`shape` can represent a normal shape or a text box. It is recommended to use `type="text"` for text boxes.

```xml
<shape type="text" topLeftX="80" topLeftY="80" width="800" height="120">
  <content textType="title">
<p>Main title</p>
  </content>
</shape>
```

```xml
<shape type="rect" topLeftX="700" topLeftY="120" width="180" height="120">
  <fill>
    <fillColor color="rgba(100, 149, 237, 0.25)"/>
  </fill>
  <border color="rgb(100, 149, 237)" width="2"/>
</shape>
```

**property:**

| Properties | Required | Description |
|------|------|------|
| `type` | Yes | Shape type, `text` represents text box |
| `topLeftX` | Yes | X coordinate of the upper left corner |
| `topLeftY` | Yes | Y coordinate of the upper left corner |
| `width` | Yes | Width |
| `height` | Yes | Height |
| `rotation` | No | Rotation angle |
| `flipX` / `flipY` | No | Flip |
| `alpha` | no | transparency |

**Optional sub-elements:**

- `<fill>`
- `<border>`
- `<reflection>`
- `<shadow>`
- `<content>`

### `<line>`

```xml
<line startX="100" startY="200" endX="420" endY="200">
  <border color="rgb(43, 47, 54)" width="2"/>
</line>
```

`line` uses `startX` / `startY` / `endX` / `endY`, not `x1` / `y1` / `x2` / `y2`.

### `<img>`

```xml
<img src="file_token_or_url" topLeftX="100" topLeftY="220" width="320" height="180"/>
```

`img` uses `topLeftX` / `topLeftY`, not `x` / `y`.

`src` only accepts two values:

| `src` form | Description |
|---|---|
| `file_token` (such as `boxcnXXXXXXXXXXXXXXXXXXXXXX`) | The token returned after uploading through `slides +media-upload` |
| `@<local path>` (e.g. `@./assets/chart.png`) | **Only available with `slides +create --slides`**: The CLI will automatically upload the file and replace it with file_token |

> **The use of http(s) external link URLs is prohibited**: Feishu slides rendering end will not proxy external link images, `src="https://..."` usually displays broken images in PPT. To use the network image, you must first `curl`/download it into CWD, and then go through the upload process to get `file_token`.

Two poses for local pictures:

- **Create a new PPT with pictures**: Write `src="@./pic.png"` directly in `+create --slides`, and the CLI will automatically upload and replace the token after creating a blank PPT and before adding slides.
- **Add a new page with pictures to an existing PPT**: First use `slides +media-upload --file ./pic.png --presentation $PID` to get the token, and then use the token to write into the XML of `xml_presentation.slide create`

### `<icon>`

```xml
<icon iconType="iconpark/Base/setting.svg" topLeftX="440" topLeftY="220" width="32" height="32"/>
```

### `<table>`

The table structure is:

- `<table>`
- `<colgroup>` / `<tr>`
- `<tr>` contains `<td>`
- `<td>` can contain `<content>`

### `<chart>`

Chart elements must contain at least:

- `<chartPlotArea>`
- `<chartData>`

It can also include:

- `<chartTitle>`
- `<chartSubTitle>`
- `<chartStyle>`
- `<chartLegend>`
- `<chartTooltip>`

For complete chart type coverage examples, see [slides_chart_demo.xml](slides_chart_demo.xml), which includes native `<chart>` examples such as column, bar, line, area, pie/ring, radar, etc., as well as `<whiteboard>` SVG chart examples such as scatter, bubble, funnel, Pareto, waterfall, etc.

Combination chart example (from [slides_chart_demo.xml](slides_chart_demo.xml)):

```xml
<chart width="556" height="350" topLeftX="42" topLeftY="132">
  <chartPlotArea>
    <chartPlot type="combo">
      <chartExtra/>
      <chartSeriesList>
        <chartSeries index="1" comboType="column"/>
        <chartSeries index="2" comboType="line" yAxisPosition="right">
          <chartTooltip format="0%"/>
        </chartSeries>
      </chartSeriesList>
    </chartPlot>
    <chartAxes>
      <chartAxis type="x">
        <chartLabel fontSize="10"/>
      </chartAxis>
      <chartAxis type="y" position="left">
        <chartGridLine color="rgb(226, 232, 240)"/>
        <chartLabel fontSize="10"/>
      </chartAxis>
      <chartAxis type="y" position="right">
        <chartLabel fontSize="10" format="0%"/>
      </chartAxis>
    </chartAxes>
  </chartPlotArea>
  <chartLegend position="bottom" fontSize="11"/>
  <chartData>
    <dim1>
<chartField name="Quarter">24Q1,24Q2,24Q3,24Q4,25Q1,25Q2,25Q3,25Q4</chartField>
    </dim1>
    <dim2>
<chartField name="Revenue">180,195,210,245,220,238,258,296</chartField>
<chartField name="Growth">0.08,0.12,0.15,0.18,0.22,0.22,0.23,0.21</chartField>
    </dim2>
  </chartData>
<chartTitle fontSize="12" color="rgba(15, 30, 58, 1)" bold="true">Revenue (USD 100M, left axis) · Year-over-year growth rate (%, right axis)</chartTitle>
  <chartStyle>
    <chartBackground color="rgba(0, 0, 0, 0)"/>
    <chartBorder color="rgb(222, 224, 227)" width="0"/>
    <chartColorTheme>
      <color value="rgb(28, 71, 120)"/>
      <color value="rgb(240, 129, 54)"/>
    </chartColorTheme>
  </chartStyle>
</chart>
```

## Style elements

### `<fill>`

```xml
<fill>
  <fillColor color="rgb(100, 149, 237)"/>
</fill>
```

### `<border>`

```xml
<border color="rgb(0, 0, 0)" width="2" dashArray="solid"/>
```

### Color format

```xml
<fillColor color="rgb(255, 0, 0)"/>
<fillColor color="rgba(255, 0, 0, 0.5)"/>
<fillColor color="linear-gradient(90deg, rgb(255,0,0) 0%, rgb(0,0,255) 100%)"/>
<fillColor color="radial-gradient(circle at 50% 50%, rgb(255,0,0) 0%, rgb(0,0,255) 100%)"/>
```

## Speaker Notes

### `<note>`

```xml
<note>
  <content textType="body">
<p>These are speaker notes. </p>
  </content>
</note>
```

## Complete example

```xml
<?xml version="1.0" encoding="UTF-8"?>
<presentation xmlns="http://www.larkoffice.com/sml/2.0" width="960" height="540">
<title>Quarterly Report</title>
  <theme>
    <textStyles>
<title fontFamily="Siyuan Heidi" fontSize="54" fontColor="rgba(0, 0, 0, 1)"/>
<body fontFamily="Siyuan Heidi" fontSize="18" fontColor="rgba(43, 47, 54, 1)"/>
    </textStyles>
  </theme>
  <slide>
    <style>
      <fill>
        <fillColor color="rgb(245, 245, 245)"/>
      </fill>
    </style>
    <data>
      <shape type="text" topLeftX="80" topLeftY="72" width="760" height="100">
        <content textType="title">
<p>2024 First Quarter Report</p>
        </content>
      </shape>
      <shape type="text" topLeftX="80" topLeftY="200" width="520" height="180">
        <content textType="body">
<p>Core indicators</p>
          <ul>
<li><p>User growth: +25%</p></li>
<li><p>Revenue growth: +30%</p></li>
<li><p>Market share: 15%</p></li>
          </ul>
        </content>
      </shape>
      <shape type="rect" topLeftX="660" topLeftY="180" width="180" height="140">
        <fill>
          <fillColor color="rgba(100, 149, 237, 0.25)"/>
        </fill>
        <border color="rgb(100, 149, 237)" width="2"/>
      </shape>
    </data>
    <note>
      <content textType="body">
<p>Supplement the sample range when talking about growth rate. </p>
      </content>
    </note>
  </slide>
</presentation>
```

## Best Practices

1. Always bring the namespace `xmlns="http://www.larkoffice.com/sml/2.0"`
2. Use `shape type="text"` + `content` to express page text
3. Use attribute names defined in schema such as `topLeftX` / `topLeftY`, `startX` / `startY` etc.
4. Prefer to use `rgb` / `rgba` color format
5. Special characters are escaped according to XML rules
6. It is recommended to use `width="960"` and `height="540"` for standard 16:9 pages

## Reference documentation

- [xml-schema-quick-ref.md](xml-schema-quick-ref.md)
- [slides_xml_schema_definition.xml](slides_xml_schema_definition.xml)
- [examples.md](examples.md)
- [slides_demo.xml](slides_demo.xml)

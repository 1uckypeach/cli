# XML Schema Quick Reference

This document is a condensed summary of [slides_xml_schema_definition.xml](slides_xml_schema_definition.xml); if the two disagree, the XSD source is authoritative.

## Most Important Rules

1. The standard protocol form is `<presentation xmlns="http://www.larkoffice.com/sml/2.0">`; the current server implementation may tolerate input without `xmlns`, but this is not a protocol guarantee
2. The only direct children of `<presentation>` are `<title>`, `<theme>`, `<slide>`
3. The only direct children of `<slide>` are `<style>`, `<data>`, `<note>`
4. Text on a page is usually expressed through `<content>`, rather than hanging `<title>` or `<body>` directly under `<slide>`

## Minimal Working Example

```xml
<?xml version="1.0" encoding="UTF-8"?>
<presentation xmlns="http://www.larkoffice.com/sml/2.0" width="960" height="540">
  <slide>
    <data>
      <shape type="text" topLeftX="80" topLeftY="80" width="800" height="120">
        <content textType="title">
          <p>Title</p>
        </content>
      </shape>
    </data>
  </slide>
</presentation>
```

## presentation Root Element

| Attribute | Required | Description |
|------|------|------|
| `width` | Yes | Presentation width, positive integer |
| `height` | Yes | Presentation height, positive integer |
| `id` | No | Presentation identifier |

**Child elements:** `<title>?`, `<theme>?`, `<slide>+`

## slide Element

| Attribute | Required | Description |
|------|------|------|
| `id` | No | Slide identifier |

**Child elements:**
- `<style>?` - Page style; currently can contain `<fill>`
- `<data>?` - Page element container; can contain `shape`, `line`, `polyline`, `img`, `table`, `icon`, `chart`, `whiteboard`, `undefined`
- `<note>?` - Speaker notes; can contain `<content>` inside

## theme and Text Types

The `title`, `headline`, `sub-headline`, `body`, `caption` types in the XSD mainly appear in:

- `<theme><textStyles>...</textStyles></theme>`, as theme text styles
- `<content textType="...">`, as the content's text type

The schema defaults for `textStyles` are:

| textType | Default font size |
|----------|----------|
| `title` | 54 |
| `headline` | 38 |
| `sub-headline` | 32 |
| `body` | 16 |
| `caption` | 12 |

## content Content Model

`<content>` can appear inside `shape`, `table/td`, and `note`. Common attributes include:

| Attribute | Description |
|------|------|
| `textType` | `title` / `headline` / `sub-headline` / `body` / `caption` |
| `textAlign` | Text alignment |
| `lineSpacing` | Line spacing, schema default `multiple:1.5` |
| `fontSize` | Font size |
| `fontFamily` | Font family |
| `color` | Font color |
| `bold` / `italic` / `underline` / `strikethrough` | Text styles |

The only allowed children of `<content>` are:

- `<p>`
- `<ul>`
- `<ol>`

### content Example

```xml
<content textType="body" textAlign="left">
  <p>Body text <strong>bold</strong> <em>italic</em> <a href="https://example.com">link</a></p>
  <ul>
    <li><p>List item 1</p></li>
    <li><p>List item 2</p></li>
  </ul>
</content>
```

## Common data Elements

### shape

```xml
<shape type="rect" topLeftX="120" topLeftY="120" width="240" height="120">
  <fill>
    <fillColor color="rgb(100, 149, 237)"/>
  </fill>
  <border color="rgb(0, 0, 0)" width="2"/>
</shape>
```

| Attribute | Required | Description |
|------|------|------|
| `type` | Yes | Shape type; `text` means a text box |
| `topLeftX` | Yes | Top-left X coordinate |
| `topLeftY` | Yes | Top-left Y coordinate |
| `width` | Yes | Width |
| `height` | Yes | Height |
| `rotation` | No | Rotation angle |

### line

```xml
<line startX="120" startY="120" endX="420" endY="120">
  <border color="rgb(43, 47, 54)" width="2"/>
</line>
```

### img

```xml
<img src="file_token_or_url" topLeftX="80" topLeftY="120" width="320" height="180"/>
```

`src` only supports: a `file_token` returned by `slides +media-upload`, or an `@<local path>` placeholder (auto-uploaded and substituted only by `+create --slides`). **Never use external http(s) URLs** — the Feishu slides renderer does not proxy external images, and an external-link src usually does not display in the deck. For local images see [lark-slides-create.md](lark-slides-create.md#local-images-path-placeholders) / [lark-slides-media-upload.md](lark-slides-media-upload.md).

> **Note**: `width`/`height` are the **post-crop** display dimensions. If the aspect ratio differs from the original image, it is auto-cropped (cannot be disabled via attributes); to avoid cropping, make `width:height` match the original image's aspect ratio.

### icon

```xml
<icon iconType="iconpark/Base/setting.svg" topLeftX="80" topLeftY="120" width="32" height="32">
  <fill>
    <fillColor color="rgba(37, 99, 235, 1)"/>
  </fill>
</icon>
```

`iconType` must come from a verified IconPark path; the visual lint spec requires `fillColor` to be explicitly set to a non-transparent color to avoid invisible icons. When you need a semantic icon, run `scripts/iconpark_tool.py search --query "<semantic>"` first — do not compose paths from memory. See [iconpark.md](iconpark.md) for more rules.

### whiteboard

```xml
<!-- SVG mode: charts not supported by <chart>, custom visuals, decorative elements -->
<whiteboard topLeftX="580" topLeftY="120" width="340" height="280">
  <svg xmlns="http://www.w3.org/2000/svg">
    <rect x="60" y="80" width="40" height="140" rx="3" fill="rgba(59,130,246,0.85)"/>
    <text x="80" y="238" text-anchor="middle" font-size="11" fill="rgba(100,116,139,1)">ABC</text>
  </svg>
</whiteboard>

<!-- Mermaid mode: flowcharts, sequence diagrams, and other structured diagrams -->
<whiteboard topLeftX="72" topLeftY="100" width="816" height="340">
  <mermaid>
    <![CDATA[
      flowchart LR
          A[Start] --> B{Decision}
          B -- Yes --> C[Execute]
          B -- No --> D[End]
    ]]>
  </mermaid>
</whiteboard>
```

SVG mode: `<svg>` must declare `xmlns="http://www.w3.org/2000/svg"`; content size is determined by the bounding box of the child elements; `width`/`height`/`viewBox` do not affect rendering — declare `viewBox` only when elements use percentage attribute values.\
Mermaid mode: wrap the content in `<![CDATA[...]]>` to prevent characters like `[`, `>`, `-->` from breaking XML parsing.\
See [lark-slides-whiteboard.md](lark-slides-whiteboard.md) for detailed usage.

## Colors and Styles

### fill

```xml
<fill>
  <fillColor color="rgb(255, 0, 0)"/>
</fill>
```

### border

```xml
<border color="rgb(43, 47, 54)" width="2" dashArray="solid"/>
```

### Color Formats

```xml
<fillColor color="rgb(255, 0, 0)"/>
<fillColor color="rgba(255, 0, 0, 0.5)"/>
<fillColor color="linear-gradient(90deg, rgb(255,0,0) 0%, rgb(0,0,255) 100%)"/>
<fillColor color="radial-gradient(circle at 50% 50%, rgb(255,0,0) 0%, rgb(0,0,255) 100%)"/>
```

> **Note**: Gradient colors must use the `rgba()` format with percentage stops, e.g. `linear-gradient(135deg,rgba(30,60,114,1) 0%,rgba(59,130,246,1) 100%)`. Using `rgb()` or omitting stops causes the server to fall back to white. This rule applies to both page backgrounds and shape fills.

### Page Background

```xml
<!-- Solid background -->
<slide>
  <style>
    <fill>
      <fillColor color="rgb(245, 245, 245)"/>
    </fill>
  </style>
</slide>

<!-- Gradient background (must use rgba + percentage stops) -->
<slide>
  <style>
    <fill>
      <fillColor color="linear-gradient(135deg,rgba(30,60,114,1) 0%,rgba(59,130,246,1) 100%)"/>
    </fill>
  </style>
</slide>
```

## Note Example

```xml
<note>
  <content textType="body">
    <p>This is a speaker note.</p>
  </content>
</note>
```

## Detailed References

- [slides_xml_schema_definition.xml](slides_xml_schema_definition.xml)
- [xml-format-guide.md](xml-format-guide.md)
- [examples.md](examples.md)
- [slides_demo.xml](slides_demo.xml)

## Schema Version Info

- **Version**: 2.0.0
- **Namespace**: http://www.larkoffice.com/sml/2.0
- **Release date**: 2025-11-03

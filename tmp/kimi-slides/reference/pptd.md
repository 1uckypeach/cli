# PPTD 格式规范

PPTD (PPT-DSL) 是 PowerPoint 演示文稿的 YAML 抽象层，用于以 AI 友好的方式描述、生成和编辑幻灯片，并与 PPTX 进行无损双向转换

---

## 本文档中的约定
- 使用**TS接口**来描述结构，并使用**字段表**和**最小YAML示例**来帮助理解
- **默认值**在 TS 行尾注释中注释为 `// default: X`。 X 可以是文字（`1`/`"top"`/`[0, 0]`）或描述性短语（`未应用`/`未显示`/`沿着继承链回退`/`自动适应图表大小`等）
- **约束**在 TS 行尾注释中或 TS 块下方注释为 `//constraint: ...`，统一使用区间或不等式表示法 (`[0, 1]` / `> 0`) 或文本描述
---

## 1. 全球惯例

### 语法
- 使用 **YAML 1.2** 语法
- 对于特殊字符，例如`:`、`#`、`{`、`}`，该值必须用引号括起来或用块标量编写
- 对于包含许多特殊字符的字段，例如 `content.text`，应使用块标量 (`|`) 作为自己的块，以防止像 `style="..."` 这样的内容被错误解析

### 坐标系和单位
- 所有几何和尺寸单位均为**px**；原点 `(0, 0)` 是页面的左上角
- 推荐尺寸：16:9 → `[960, 540]`； 4:3 → `[720, 540]`
- 该规范定义 1px = 1pt（即“fontSize: 18”在 PPTX 中为 18pt）
- 元素堆叠顺序由“Page.elements”数组的顺序决定；越靠后的元素，其层数越高

### 样式优先级和默认值

对于冲突的属性值，通过从上到下搜索以下优先级来找到第一个有值的来源；如果未设置任何级别，则回退到该部分末尾的默认值。

> 以下规则适用于本节的所有小节： `lineHeight` （倍数）和 `lineHeightPx` （固定像素）是互斥的；当两者都设置时，“lineHeightPx”优先。

#### 1. 文本框中的文本样式

**优先链：**
1. [Text.content.text](#textcontent) 中的富文本语义标签，例如 `<u>`、`<sup>`、`<strong>`
2. 在`<span style="...">`中设置内联属性
3.`<p style="...">`中设置的段落属性
4. **直接在[Text.content](#textcontent)上设置样式字段**（与`style`引用的主题样式不同；包括`color`、`fontSize`、`fontFamily`、`bold`、`italic`、`backgroundColor`、`lineHeight`、`lineHeightPx`、`letterSpacing`、`marginTop`）
5. [Text.content.style](#textcontent)引用的[TextStyleConfig](#textstyleconfig)主题样式
6. 默认值：

|物业 |默认值 |
|---|---|
|颜色 | `#000000` |
|背景颜色 |未应用 |
|字体大小 | `18` |
|字体家族 | `“米桑斯”` |
|大胆| `假` |
|斜体 | `假` |
|行高 | `1` |
|行高Px |未应用 |
|字母间距| `0` |
|边距顶部 | `0` |

#### 2. 表格单元格样式

**优先链：**
1. [Cell.text](#cell) 中的富文本语义标签，例如`<u>`、`<sup>`、`<strong>`
2. 在`<span style="...">`中设置内联属性
3.`<p style="...">`中设置的段落属性
4. [Cell](#cell) 内联字段
5. [Cell.textStyle](#cell) 引用的 [TextStyleConfig](#textstyleconfig)（**仅适用于文本字段**；不包括 `fill` / `border` / `align`）
6. [TableStyleConfig](#tablestyleconfig)的位置类别样式
   - 在行与列冲突时，[TableStyleConfig.rowOverColumn](#tablestyleconfig) 决定获胜者；默认 `true` = 行获胜
   - 行类别：`TableStyleConfig.firstRowStyle` / `TableStyleConfig.lastRowStyle`
   - 列类别：`TableStyleConfig.firstColumnStyle` / `TableStyleConfig.lastColumnStyle`
7. [TableStyleConfig.bodyStyles](#tablestyleconfig)：适用于除首尾行以外的数据行，按数据行索引循环
8. [TableStyleConfig.cellStyle](#tablestyleconfig): 整个表格的基线单元格样式
9. 默认值

|物业 |默认值 |
|---|---|
|颜色 | `#000000` |
|背景颜色 |未应用 |
|字体大小 |根据单元格高度自动适应 |
|字体家族 | `“米桑斯”` |
|大胆| `假` |
|斜体 | `假` |
|行高 | `1` |
|行高Px |未应用 |
|字母间距| `0` |
|边距顶部 | `0` |
|填充|未应用（透明）|
|边框| `{样式：实心，宽度：1，颜色：“#000000”}` |
|对齐| `[“中心”，“中间”]` |

#### 3. 图表样式

图表涉及多种样式（系列主体颜色、字体、数据标签、轴/图例可见性等），每种样式都有自己独立的优先级链，如下所述。

**3.1系列车身颜色优先链：**
1.一系列的显式`fill`/`lineColor`/`areaColor`（字段名称因类型而异；参见[颜色机制](#52-color-mechanism)）
2. [Chart.seriesDefaults](#seriesdefaults) 中对应类型的同名字段
3. [Theme.colors](#theme) 主题颜色循环（颜色按照系列在数组中出现的顺序选取）

> [scatter](#scatter) 是一个例外：标记颜色解析为 `marker.fill > series.fill > 主题颜色循环`；同样，marker.border 优先于 series.border。
>
> 对于每种类型的具体颜色字段、派生规则和角色映射，请参阅[§5.2 颜色机制](#52-color-mechanism)。

**3.2 字体优先级链：**
1. 子组件 `fontFamily` ([TitleConfig](#titleconfig) / [LegendConfig](#legendconfig) / [DataLabelConfig](#datalabelconfig) / [AxisConfig.label](#axisconfig) / [SpokeAxisConfig.label](#spokeaxisconfig))
2. [Chart.fontFamily](#chart)
3. 主题默认（[主题](#theme) 或 PPTX 主字体）

**3.3 dataLabels优先级链：**
1. `series[i].dataLabels`
2. [Chart.dataLabels](#chart)（全局默认）
3. 不显示（相当于`show: false`）

> 子字段遵循**一级浅层合并**：`series.dataLabels` 仅覆盖其显式提供的子字段；未提供的从 [Chart.dataLabels](#chart) 回退；如果两者都没有提供，则应用每种类型的默认值（请参阅 [dataLabels.content 值快速参考](#55-datalabelscontent-value-quick-reference)）。

**3.4 `seriesDefaults` 合并规则：**

[Chart.seriesDefaults](#seriesdefaults)`[type]` 为该类型的所有系列提供通用默认值，通过**一级深度合并**与每个系列合并：
- **标量字段**（字符串/数字/布尔值）：系列的显式值覆盖默认值
- **对象字段**（`marker`/`dataLabels`/`border`/`upBars`/`downBars`/`totalBars`/渐变`fill`对象等）：递归一级浅层合并 - 默认值和系列的同名字段分别展开，子字段被最近的源覆盖
- **数组字段** (`fill: []` / `colorScheme: []`)：系列整体替换默认值，没有元素级合并
-SeriesDefaults 中不允许使用“type”和“encode”
- 仅多系列类型支持系列默认值：`bar / line / area / scatter / bubble / Candlestick / Radar`

违反直觉的例子：```yaml
seriesDefaults:
  bar: {marker: {shape: circle, size: 8}}
series:
  - type: bar
    marker: {size: 12}     # after merge: {shape: circle, size: 12}, not {size: 12}
```**3.5`布尔值|配置`字段约定：**

`boolean | 形式的字段XxxConfig`(`marker`/`legend`/`AxisConfig.label/axisLine/gridLine`/`SpokeAxisConfig.label/axisLine/gridLine`/`colorbar`)统一遵循：
- `假` = 关闭
- `true` = 默认配置开启
- 对象 `{...}` = on + 自定义配置

> **唯一的例外**：[scatter.marker](#scatter) 不能为“false”（没有标记的散点图没有任何可渲染的内容）。

---

### 多文件结构
PPTD 项目由主条目文件和各个页面文件组成：```
project/
  slides_name.pptd     # main entry (size/theme/title + page reference list)
  media/               # media resources such as images and videos
  pages/               # page file directory
    1_cover.page       # one .page file per page
    2_intro.page
```**路径规则：**
1. **完全独立**：所有引用的文件必须位于包含`.pptd`文件的文件夹内； **不允许引用目录外的文件**
2. **仅支持相对路径**（相对于包含`.pptd`文件的目录）：
   - `.pptd`中的`pages`列表：`pages/1_cover.page`
   - `.page` 中的图像路径：`media/image1.jpg`
3. **媒体支持的URL**：`Image.src`，以及`background` / `fill`的[ImageFill](#fill).src，可以是`https://...`（仅支持jpg/jpeg/png/gif）

**需要主条目：** 所有内容都必须通过 `.pptd` 主条目文件加载； `.page` 不能单独传递给 `convert`/`check` 命令

---

## 2. 共享类型
以下类型在多个地方重用并预先一起定义。元素部分通过类型名称引用它们，而无需重复扩展

### 颜色```ts
type Color = string;
```> 支持不透明 **HEX6** (`#RRGGBB`)、alpha **HEX8** (`#RRGGBBAA`) 和 [Theme.colors](#theme) 主题颜色引用（例如 `$primary`）

### 字体家族```ts
type FontFamily = string | { latin: string; ea: string };
```|表格 |示例|描述 |
|------|------|------|
|字符串| `“米桑斯”` |中英文统一使用相同字体 |
|对象| `{拉丁语：“Arial”，ea：“MiSans”}` |分别显式指定拉丁（latin）和东亚（ea）字体 |

请参阅 [fonts.md](./fonts.md) 以获取可用字体列表

### 对齐```ts
type HorizontalAlign = "left" | "center" | "right" | "justify" | "distributed";
type VerticalAlign   = "top"  | "middle" | "bottom";
type Alignment       = [HorizontalAlign, VerticalAlign];
```|价值|描述 |
|----|------|
| `左`/`中`/`右` |水平左/中/右对齐 |
| `证明` |对齐（最后一行未拉伸）|
| `分布式` |分布式（最后一行延伸）|
| `顶部`/`中间`/`底部` |垂直顶部/中间/底部对齐 |

### 线条样式```ts
type LineStyle = "solid" | "dash" | "dot";
```＃＃＃ 边界```ts
interface Border {
  style?: LineStyle;  // default: "solid"
  width?: number;     // default: 1
  color?: Color;      // default: "#000000"
}
```#### 边界规范
[Cell](#cell) 和 [CellStyle](#cellstyle) 支持 `Border` 数组形式分别设置四个侧边框```ts
type BorderSpec = null
                | Border
                | [Border | null, Border | null]
                | [Border | null, Border | null, Border | null, Border | null];
```|表格 |意义|
|------|------|
| `空` |显式清除：四个边中的任何一个都没有边框（用于覆盖继承链上方设置的边框）|
| `边界` |四面都一样 |
|二元素数组 `[Border\|null, Border\|null]` | `[从上到下，从左到右]` |
|四元素数组 `[Border\|null, Border\|null, Border\|null, Border\|null]` | `[上、右、下、左]`（顺时针）|

> 数组内为 null 表示对应位置没有边框；顶级“null”会清除所有内容。

＃＃＃ 阴影```ts
interface Shadow {
  blur: number;                // blur radius
  color: Color;
  offset?: [number, number];   // default: [0, 0]; [x, y] offset
}
```### 色彩停止```ts
interface ColorStop {
  position: number;  // constraint: [0, 1]
  color: Color;
}
```### 图像拟合/图像裁剪```ts
interface ImageFit {
  mode: "fill" | "contain" | "cover";
}

interface ImageCrop {
  left?: number;
  top?: number;
  right?: number;
  bottom?: number;
}
```> **约束：** `ImageCrop` 的四个字段类似，默认0。正值从对应的边缘按比例向内裁剪（插图）；负值按比例向外扩展到相应的边缘，并用透明像素填充（开始）。必须确保“left + right < 1”和“top + Bottom < 1”，否则源矩形会退化。

| ImageFit.模式 |描述 |
|---|---|
| `封面` |填充容器，保持纵横比，可以裁剪 |
| `包含` |完整显示图像，保持纵横比，可能会留有空白 |
| `填充` |拉伸填充，可能会扭曲 |

＃＃＃ 充满```ts
type Fill = SolidFill | GradientFill | ImageFill;

interface SolidFill {
  type: "solid";
  color: Color;
}

interface GradientFill {
  type: "gradient";
  gradientType: "linear" | "radial";
  stops: ColorStop[];                // constraint: at least 2
  angle?: number;                    // default: 0; only effective for linear
}

interface ImageFill {
  type: "image";
  src: string;                       // URL or relative path
  fit?: ImageFit;                    // default: {mode: "cover"}
  crop?: ImageCrop;                  // always applied; see the rendering order with fit below
  opacity?: number;                  // default: 1; constraint: [0, 1]
}
```> `GradientFill.angle` 取值在 `[0, 360)` 中； “0”表示从左到右，顺时针增加。示例：“90”= 上→下，“180”= 右→左。

> **ImageFill 渲染顺序：** `crop` （按比例调整源矩形：正值向内裁剪，负值向外扩展并用透明像素填充）→ `fit` （适应每种模式的填充容器）。每个`fit.mode`值的具体语义与[Image](#image-image)部分中的“渲染逻辑”讨论一致。

**示例：**```yaml
# Solid
fill:
  type: solid
  color: "$primary"

# Gradient
fill:
  type: gradient
  gradientType: linear
  angle: 90
  stops:
    - {position: 0, color: "$primary"}
    - {position: 1, color: "$accent"}

# Image
fill:
  type: image
  src: "media/bg.jpg"
  fit: {mode: cover}
  opacity: 0.9
```---

## 3.主入口文件(.pptd)

### 演示```ts
interface Presentation {
  version: "v2";                // required, fixed to "v2" (version identifier)
  title?: string;              // default: no title
  size: [number, number];      // [width, height]; 16:9 recommended [960, 540], 4:3 recommended [720, 540]
  theme?: Theme;
  pages: string[];             // list of relative paths to page files, e.g. "pages/cover.page"
}
```**例子：**```yaml
version: v2
title: Annual Work Summary
size: [960, 540]
theme:
  colors:
    primary: "#2563EB"
    accent: "#F59E0B"
    text: "#1F2937"
  textStyles:
    title:
      fontSize: 40
      color: "$primary"
    body:
      fontSize: 18
      color: "$text"
      lineHeight: 1.6
  tableStyles:
    default:
      firstRowStyle:
        fill: {type: solid, color: "$primary"}
        color: "#ffffff"
        bold: true
      bodyStyles:
        - {fill: {type: solid, color: "#f8fafc"}}
        - {fill: {type: solid, color: "#ffffff"}}
pages:
  - pages/1_cover.page
  - pages/2_content.page
```### 主题

主题集中管理颜色、文本样式和表格样式。在相关字段中使用“$<key>”来引用主题：

|主题类型 |参考字段 |示例|
|---|---|---|
| `颜色` |任何 [颜色](#color) 字段 | `$primary` |
| `文本样式` | [TextContent.style](#textcontent) / [Cell.textStyle](#cell) | `$title` |
| `表格样式` | [表格样式](#table-table) | `$default` |```ts
interface Theme {
  colors?: Record<string, Color>;
  textStyles?: Record<string, TextStyleConfig>;
  tableStyles?: Record<string, TableStyleConfig>;
}
```#### 文本样式配置```ts
interface TextStyleConfig {
  color?: Color;
  fontSize?: number;
  fontFamily?: FontFamily;
  bold?: boolean;                    // bold
  italic?: boolean;                  // italic
  backgroundColor?: Color;           // text background color (e.g., text highlight)
  lineHeight?: number;               // line-height multiple
  lineHeightPx?: number;             // fixed line height (px); when it conflicts with lineHeight, lineHeightPx prevails
  letterSpacing?: number;
  marginTop?: number;
}
```> 未设置的字段沿着继承链回退（有关详细信息，请参阅[样式优先级和默认值](#style-priority-and-default-values)）

#### 单元格样式```ts
interface CellStyle extends TextStyleConfig {
  // —— Inherits all properties of TextStyleConfig ——
  //   color / fontSize / fontFamily / bold / italic / backgroundColor / lineHeight / lineHeightPx / letterSpacing / marginTop

  // —— CellStyle-specific ——
  fill?: Fill;                              // background fill
  border?: BorderSpec;                      // border
  align?: Alignment;                        // text alignment
}
```> 未设置的字段沿着继承链回退（有关详细信息，请参阅[样式优先级和默认值](#style-priority-and-default-values)）

#### 表样式配置```ts
interface TableStyleConfig {
  // —— Cell style: applied to every cell ——
  cellStyle?: CellStyle;

  // —— Row category overrides ——
  firstRowStyle?: CellStyle;  // first-row style
  lastRowStyle?: CellStyle;  // last-row style

  // —— Column category overrides ——
  firstColumnStyle?: CellStyle;
  lastColumnStyle?: CellStyle;

  // —— Alternating row styles ——
  bodyStyles?: CellStyle[];  // data rows other than the first/last row apply these cyclically by data-row index

  // —— Cross-category rule ——
  rowOverColumn?: boolean;            // default: true; whether the row style wins when a cell is covered by both row and column rules
}
```> **行/列样式规则**：`firstRowStyle` / `lastRowStyle` / `firstColumnStyle` / `lastColumnStyle` 等类别样式的意思是**将样式独立应用到每个匹配的单元格**，而不是将样式整体应用到第一行/最后一列
> - 编写 `firstRowStyle.border: {style: Solid, width: 2}` → **第一行的每个单元格**在所有四个边上都有边框
> - 要为第一行整体添加外框，请使用每边 BorderSpec: `border: [<top line>, null, <bottom line>, null]`，然后分别在第一行的第一列和最后一列单元格上设置边框


> 有关后备规则，请参阅[样式优先级和默认值](#style-priority-and-default-values)

## 4.页面文件（.page）

### 页```ts
interface Page {
  pageType?: "cover" | "table_of_contents" | "chapter" | "content" | "final" | string;  // default: none; category label (does not affect rendering); preset values are recognized as the corresponding page type, arbitrary custom strings are also allowed
  background?: Fill;               // default: {type: solid, color: "#FFFFFF"} (white solid fill)
  notes?: string;                  // default: none; speaker notes; plain text
  elements: Element[];             // the later an element, the higher its layer
}
```**例子：**```yaml
pageType: cover
background:
  type: solid
  color: "$primary"
notes: Speaker notes
elements:
  - elementId: title1
    elementType: text
    bounds: [100, 200, 760, 80]
    content:
      style: "$title"
      align: [center, middle]
      text: Hello World
```---

## 5. 元素

### 元素库

所有元素的共同属性。```ts
interface ElementBase {
  elementId: string;                                                      // constraint: unique within the same page; unique element ID
  elementType: "text" | "shape" | "line" | "image" | "icon" | "table" | "chart";  // element type
  bounds: [number, number, number, number];                               // element size and position, [x, y, width, height]
}

type Element = Text | Shape | Line | Image | Icon | Table | Chart;
```---

### 文本（文本框）```ts
interface Text extends ElementBase {
  elementType: "text";
  rotation?: number;                  // default: 0; degrees, clockwise rotation
  opacity?: number;                   // default: 1; constraint: [0, 1]
  flip?: [boolean, boolean];          // default: [false, false]; [horizontal flip, vertical flip]
  content: TextContent;
}
```#### 文本内容```ts
interface TextContent {
  text: string;                                // rich text string (block scalar)
  style?: string;                              // references theme.textStyles, written as "$key" (e.g. "$title")

  // —— Style fields (when unset, fall back along the inheritance chain) ——
  color?: Color;
  fontSize?: number;
  fontFamily?: FontFamily;
  bold?: boolean;                              // bold: true=on, false/unset=off
  italic?: boolean;                            // italic: true=on, false/unset=off
  backgroundColor?: Color;                     // text background color (e.g., text highlight)
  lineHeight?: number;                         // line-height multiple
  lineHeightPx?: number;                       // fixed line height (px)
  letterSpacing?: number;
  marginTop?: number;

  // —— Layout fields ——
  textDirection?: "horizontal" | "vertical";   // default: "horizontal"
  wrap?: boolean;                              // default: true; when false, no wrapping, and the part beyond bounds.width overflows the element boundary; explicitly setting false is recommended for single-line text
  align?: Alignment;                           // default: ["left", "top"]

  // —— Visual decoration (unset = not applied) ——
  gradient?: GradientFill;                     // text gradient (applied to the text itself)
  shadow?: Shadow;                             // text shadow
}
```**示例：**```yaml
# Basic: theme style + plain text
- elementId: title-1
  elementType: text
  bounds: [100, 50, 760, 80]
  content:
    style: "$title"
    align: [center, middle]
    text: Annual Work Summary

# Rich text + inline property overrides
- elementId: body-1
  elementType: text
  bounds: [100, 200, 600, 200]
  content:
    fontSize: 20
    color: "$text"
    lineHeight: 1.6
    align: [left, top]
    text: |
      <p><strong>Key achievement</strong>: completed <span style="color:$primary;">3</span> key projects</p>
      <p style="text-align:right"><span style="font-size:14px; color:#6b7280;">—— FY2024</span></p>

# Text gradient + shadow
- elementId: hero-text
  elementType: text
  bounds: [100, 100, 760, 120]
  content:
    align: [center, middle]
    gradient:
      type: gradient
      gradientType: linear
      angle: 90
      stops:
        - {position: 0, color: "$primary"}
        - {position: 1, color: "$accent"}
    shadow:
      blur: 6
      color: "#00000040"
      offset: [0, 3]
    text: |
      <p><span style="font-size:64px;">FUTURE</span></p>
```#### 富文本规则

`TextContent.text` 和 `Cell.text` 遵循以下富文本规则进行段落分割以及设置段落或内联样式。

**支持的标签**

|标签 |描述 |示例|
|------|------|------|
| `<p>` |段落;可能带有段落样式| `<p>段落</p>` |
| `<跨度>` |内联样式；使用此标签设置内联样式 | `<span style="color:#f00">红色</span>` |
| `<强>` |大胆| `<strong>重要</strong>` |
| `<em>` |斜体 | `<em>强调</em>` |
| `<u>` |下划线 | `<u>下划线</u>` |
| `<s>` |删除线 | `<s>已删除</s>` |
| `<sup>` |上标| `E=mc<sup>2</sup>` |
| `<子>` |下标| `H<sub>2</sub>O` |
| `<a>` |超级链接;支持 `https://`、`http://`、`mailto:`；设置后，超链接文本样式（带下划线的蓝色）将自动应用 | `<a href="https://x.com">链接</a>` |
| `<ul>` |无序列表 | `<ul><li>项目</li></ul>` |
| `<ol>` |已排序列表 | `<ol><li>第一项</li></ol>` |
| `<li>` |列出项目；必须与 `<ul>` 或 `<ol>` | 一起使用— |

**样式属性映射**

`<p>`、`<li>` 和 `<span>` 可以使用 `style="..."`。颜色类型值都可以使用主题引用（例如“$primary”），根据 [Color](#color) 规则解析。

1. **段落样式（仅 `<p>` 支持）**

|物业 |描述 |价值观 |示例|
| ---| ---| ---| ---|
| `文本对齐` |段落水平对齐| `左`/`中心`/`右`/`对齐`/`分布式` | `<p style="text-align:center">...</p>` |
| `行高` |线高； **无单位**被视为“lineHeight”倍数，**将“px”视为“lineHeightPx”固定值 |数字（例如“1.5”）或 px 字符串（例如“24px”）| `<p style="line-height:1.6">...</p>` |
| `边缘顶部` |段落前的间距 | px 字符串（例如 `8px`）| `<p style="margin-top:8px">...</p>` |
| `左边距` |左边距| px 字符串（例如 `12px`）| `<p style="margin-left:12px">...</p>` |
| `右边距` |右边距| px 字符串（例如 `12px`）| `<p style="margin-right:12px">...</p>` |

> 不要在`<p>`上设置`letter-spacing`；要统一设置字母间距，请使用“content.letterSpacing”或“Cell.letterSpacing”。

2. **列表项样式（仅 `<li>` 支持）**

|物业 |描述 |
| ---| ---|
| `文本对齐` |列表项水平对齐 |
| `行高` |线高；值规则与 `<p>` 相同 |
| `字母间距` |字母间距|
| `边缘顶部` |段落前的间距 |
| `左边距` |左边距|
| `列表样式` |列表样式简写 |
| `列表样式类型` |列表标记类型 |
| `列表样式位置` |列表标记位置|
| `列表样式图像` |列表标记图像 |

3. **内联样式（仅 `<span>` 支持）**

样式仅适用于“<span>”内的文本。

|物业 |描述 |价值观 |示例|
| ---| ---| ---| ---|
| `颜色` |文字颜色 | [颜色](#color) (HEX6 / HEX8 / 主题参考) | `<span style="color:$primary">...</span>` |
| `字体大小` |字体大小 | px 字符串（例如 `24px`）| `<span style="font-size:24px">...</span>` |
| `字体系列` |字体家族 |字体名称（例如 `Arial`、`"Arial、微软雅黑"`）| `<span style="font-family:Arial">...</span>` |
| `背景颜色` |文字背景颜色| [颜色](#color) (HEX6 / HEX8 / 主题参考) | `<span style="background-color:$accent">...</span>` |```yaml
content:
  align: [left, top]
  lineHeight: 1.2
  text: |
    <p><span style="font-size:32px; color:$primary;">Main Title</span><span style="font-size:18px; color:$secondary;">Subtitle</span></p>
    <p style="text-align:center; line-height:1.8">This paragraph is center-aligned with 1.8x line height</p>
    <p style="text-align:right">This paragraph is right-aligned; line height inherits the default 1.2</p>
```**纯文本简写**

`content.text` 可以直接使用纯文本：
- 单行：`text: "Hello"` ≡ `text: "<p>Hello</p>"`
- 多行（块标量）：```yaml
  text: |
    First line
    Second line
  ```== `<p>第一行</p><p>第二行</p>`
- `<br/>` 可用于段落内的换行符，但不保证在编辑后重新转换时保留。当需要稳定的换行符时，请使用多个“<p>”。

**LaTeX 公式**

富文本支持使用 `\(...\)` 分隔符嵌入 LaTeX 公式：
- 可以形成自己的段落，或与“<p>”内的其他文本混合。
- 公式内**不允许**使用富文本标签。
- 公式**仅从其上下文继承**“颜色”和“字体大小”样式；其他文本样式不会被传递。
- `<p>` 标签可以包裹 LaTeX 公式来控制对齐方式```yaml
content:
  text: |
    <p>Pythagorean theorem: \(a^2 + b^2 = c^2\)</p>
    <p>\(\int_0^1 x^2 \mathrm{d}x = \frac{1}{3}\)</p>
```---

### 形状（形状）```ts
interface Shape extends ElementBase {
  elementType: "shape";
  rotation?: number;                  // default: 0; degrees, clockwise rotation
  opacity?: number;                   // default: 1; constraint: [0, 1]
  flip?: [boolean, boolean];          // default: [false, false]; [horizontal flip, vertical flip]
  shapeName: string;                  // see ./shapes.md
  adjustments?: number[];             // see ./shapes.md; geometry parameters; default: the default parameter values
  viewBox?: [number, number];         // view box; used only when shapeName="custom", required in that case
  path?: string;                      // SVG shape path; used only when shapeName="custom", required in that case
  fill?: Fill;                        // default: not applied
  border?: Border;                    // default: not applied
  shadow?: Shadow;                    // default: not applied
}
```> 自定义形状：您可以指定`shapeName: "custom"`并使用`viewBox`和`path`来定义自定义形状；当“shapeName”不是“custom”时，这两个参数不起作用

> `调整`参数：复用OOXML定义的参数顺序和数量；请参阅 ./shapes.md 了解值约束。

> **注意**：`shape`不支持嵌入文本！添加一个额外的文本框来实现这一点。

**自定义路径约定：**
- `viewBox`：视图框，路径坐标系`[w, h]`
- `path`：SVG路径字符串，支持`M / L / H / V / C / S / Q / A / Z`命令。
- 镂空等形状支持多段路径：使外轮廓**顺时针**（`sweep=1`）和内轮廓**逆时针**（`sweep=0`）以实现镂空镂空
- **缩放和纵横比**：更改“bounds”调整形状大小（路径不需要重写）；但 viewBox 是独立拉伸到边界的——当比率不同时，形状会扭曲。要保持比例，需要 `viewBoxW : viewBoxH =bounds.w :bounds.h`。

**常见形状**

> 请参阅 [shapes.md](./shapes.md) 了解完整的 177 个形状。

|形状名称 |描述 |调整默认值|
|------------|------|--------------------|
| `直` |矩形| — |
| `roundRect` |圆角矩形 | `[16667]`（圆角半径）|
| `椭圆` |椭圆| — |
| `三角形` |三角形| `[50000]`（顶点的水平位置）|
| '钻石' |钻石 | — |
| `本垒板` |五边形箭头| `[50000]` |
| `V 字形` | V形箭头| `[50000]` |
| `甜甜圈` |戒指| `[25000]`（环宽比）|
| `star5` | 5 星 | `[19098, 105146, 110557]` |
| `右箭头` |向右箭头| `[50000, 50000]`（轴宽度，箭头长度）|
| `wedgeRectCallout` |矩形标注 | `[-20833, 62500]` |
| `括号对` |支架对| `[8333]` |

**示例：**```yaml
# Built-in shape
- elementId: shape-1
  elementType: shape
  bounds: [200, 200, 300, 150]
  shapeName: roundRect
  adjustments: [20000]
  fill: {type: solid, color: "$primary"}
  border: {style: solid, width: 2, color: "$accent"}

# Custom hollow ring (outer contour clockwise + inner contour counterclockwise)
- elementId: shape-2
  elementType: shape
  bounds: [400, 200, 150, 150]
  shapeName: custom
  viewBox: [1000, 1000]
  path: "M500,0 A500,500 0 1 1 499,0 Z M500,200 A300,300 0 1 0 499,200 Z"
  fill: {type: solid, color: "$accent"}
```---

### 线（线）```ts
type ArrowType = "arrow" | "stealth" | "diamond" | "oval";

interface Line extends ElementBase {
  elementType: "line";
  rotation?: number;                             // default: 0; degrees, clockwise rotation
  opacity?: number;                              // default: 1; constraint: [0, 1]
  flip?: [boolean, boolean];                     // default: [false, false]; [horizontal flip, vertical flip]
  viewBox: [number, number];                     // path coordinate system [w, h]; points live in this coordinate system, so changing bounds does not require changing points
  points: string;                                // bezier path points "x1,y1 x2,y2 ..."; the first/last points are the start/end the curve passes through, the middle points are control points
  curve?: "sharp" | "round" | "smooth";          // default: "round"; sharp joins / rounded joins / bezier smooth curve
  arrow?: [ArrowType | null, ArrowType | null];  // start arrow, end arrow; default: [null, null] (no arrows at either end)
  border?: Border;                               // default: not applied
  shadow?: Shadow;                               // default: not applied
}
```> **约束：** `points` 至少需要 2 个点；第一个点和最后一个点是曲线经过的点，其余的点是贝塞尔曲线控制点；所有坐标必须在“viewBox”内。
> **viewBox 与边界：** 在渲染时，viewBox 独立拉伸到边界大小；为了防止线条被拉伸变形，需要 `viewBoxW : viewBoxH =bounds.w :bounds.h`。

**示例：**```yaml
# Normalized coordinates: from top-left to bottom-right, the two middle points are control points
- elementId: l4
  elementType: line
  bounds: [100, 100, 500, 300]
  viewBox: [1, 1]
  points: "0,0 0.2,0 0.8,1 1,1"
  curve: smooth
  border: {style: solid, width: 2, color: "$primary"}

# Bezier arc: passes through the start and end points; the two middle points control the bend direction
- elementId: bezier-arc
  elementType: line
  bounds: [50, 200, 860, 100]
  viewBox: [360, 100]
  points: "0,80 120,0 240,100 360,20"
  curve: smooth
  border: {style: solid, width: 2, color: "$primary"}
```---

### 图片（图片）```ts
interface Image extends ElementBase {
  elementType: "image";
  rotation?: number;                 // default: 0; degrees, clockwise rotation
  opacity?: number;                  // default: 1; constraint: [0, 1]
  flip?: [boolean, boolean];         // default: [false, false]; [horizontal flip, vertical flip]
  src: string;                       // URL or local relative path
  cropShape?: ShapeDef;              // default: rectangle (i.e., no shape cropping)
  fit?: ImageFit;                    // default: {mode: "cover"}
  crop?: ImageCrop;                  // always applied; see the rendering order with fit/cropShape below
  border?: Border;                   // default: not applied
  shadow?: Shadow;                   // default: not applied
}

interface ShapeDef {
  shapeName: string;                 // see ./shapes.md; use "custom" for a custom path
  adjustments?: number[];            // default: use the shape's built-in defaults (see ./shapes.md)
  viewBox?: [number, number];        // used only when shapeName="custom", required in that case
  path?: string;                     // used only when shapeName="custom", required in that case
}
```> `ShapeDef` 字段与 [Shape](#shape-shape) 元素的形状字段一一对应；详细约定（调整值和角度转换、自定义路径规则、空心规则、常用形状表）请参见[形状](#shape-shape)部分。

**渲染逻辑：** `crop`（按比例调整源矩形以获得子图像：正值向内裁剪，负值向外扩展并用透明像素填充）→`fit`（使子图像适应每种模式的边界容器）→`cropShape`（将最终显示区域裁剪到形状轮廓）。这三者都可以独立设置，并按上面的固定顺序应用。

- `fit.mode="cover"`：按比例缩放子图像以填充边界；溢出部分被裁剪。
- `fit.mode="contain"`：按比例缩放子图像以使其完全显示；不足部分留为空白。
- `fit.mode="fill"`：**子图像直接拉伸到填充边界**——虽然在这种情况下看不到裁剪的空白边缘，但图片内容仍然只是裁剪后的子区域，而不是完整的原始图像。

**示例：**```yaml
- elementId: img-1
  elementType: image
  bounds: [50, 50, 400, 300]
  src: "media/cover.jpg"
  cropShape: {shapeName: roundRect, adjustments: [15000]}
  fit: {mode: cover}
  crop: {top: 0.1, bottom: 0.1, left: 0.05, right: 0.05}   # crop the surrounding proportions first, then apply cover fitting
  shadow:
    blur: 10
    color: "#00000033"
    offset: [0, 4]

# Custom clip outline
- elementId: img-2
  elementType: image
  bounds: [200, 200, 200, 200]
  src: "media/avatar.jpg"
  cropShape:
    shapeName: custom
    viewBox: [1000, 1000]
    path: "M500,0 A500,500 0 1 1 499,0 Z"
  fit: {mode: cover}
```---

### 图标（图标）```ts
interface Icon extends ElementBase {
  elementType: "icon";
  rotation?: number;                 // default: 0; degrees, clockwise rotation
  opacity?: number;                  // default: 1; constraint: [0, 1]
  flip?: [boolean, boolean];         // default: [false, false]; [horizontal flip, vertical flip]
  iconName: string;                  // format "style:name"
  fill?: Fill;                       // default: black solid fill
  border?: Border;                   // default: not applied
  shadow?: Shadow;                   // default: not applied
}
```**iconName 格式：** `style:name`，使用 Font Awesome 7.x 免费图标库。

|前缀 |风格|示例|
|------|------|------|
| `fas` |固体（最常见）| `fas：房子` |
| `远` |常规| `远：心` |
| `很棒` |品牌 | `fab：github` |

图标搜索：https://fontawesome.com/search?ic=free-collection

**示例：**```yaml
- elementId: icon-1
  elementType: icon
  bounds: [100, 100, 48, 48]
  iconName: "fas:lightbulb"
  fill: {type: solid, color: "$primary"}
```---

### 表（表）```ts
interface Table extends ElementBase {
  elementType: "table";
  columnWidths: number[];              // array of column-width ratios (not px; relative to the bounds width)
  rowHeights: number[];                // array of row-height ratios (not px; relative to the bounds height)
  rows: Cell[][];                      // 2-D array; merged regions are declared with rowSpan/colSpan, occupied positions are skipped in the array
  style?: string | TableStyleConfig;   // references theme.tableStyles, written as "$key" (e.g. "$default"), or an inline TableStyleConfig object
  fill?: Fill;                         // default: not applied; table-level fill (applied to the whole table, can be overridden by cell fill)
  shadow?: Shadow;                     // default: not applied
}
```> **PowerPoint限制：**原生表格不能整体旋转/翻转；也不支持包括文本和边框在内的整个表格全局不透明度。当需要整体旋转/翻转/不透明度时，首先渲染为图像并将其视为 [Image](#image-image) 元素。

> **约束：** `columnWidths` 和 `rowHeights` 每一项都在 `[0, 1]` 范围内，且每一项元素之和为 1。

#### 细胞```ts
interface Cell {
  // —— Content ——
  text?: string;             // default: empty cell; rich text string (written as a block scalar), rules same as TextContent.text
  textStyle?: string;        // references theme.textStyles, written as "$key" (e.g. "$body")

  // —— Text styles (when unset, fall back along the inheritance chain) ——
  color?: Color;
  fontSize?: number;
  fontFamily?: FontFamily;
  bold?: boolean;
  italic?: boolean;
  backgroundColor?: Color;             // text background color (e.g., text highlight)
  lineHeight?: number;                 // line-height multiple
  lineHeightPx?: number;               // fixed line height (px)
  letterSpacing?: number;
  marginTop?: number;

  // —— Cell styles (when unset, fall back along the inheritance chain) ——
  fill?: Fill;                         // background fill; supports solid / gradient / image
  border?: BorderSpec;
  align?: Alignment;

  // —— Merging ——
  rowSpan?: number;                    // default: 1
  colSpan?: number;                    // default: 1
}
```**基本示例（使用主题样式）：**```yaml
- elementId: table-basic
  elementType: table
  bounds: [80, 120, 800, 280]
  columnWidths: [0.3, 0.35, 0.35]
  rowHeights: [0.33, 0.33, 0.34]
  style: "$default"
  rows:
    - - text: "Metric"
      - text: "2023"
      - text: "2024"
    - - text: "Revenue (100M CNY)"
      - text: "82.5"
      - text: "96.3"
    - - text: "Net profit (100M CNY)"
      - text: "12.1"
      - text: "15.8"
```> **合并单元格规则：** `rowSpan` / `colSpan` 声明合并范围； **合并区域覆盖的单元格从“rows”数组中省略，不需要“null”占位符**。例如，在左上角的 2×2 合并后，第 0 行的 colSpan=2 覆盖了 (0,1)，因此该行只有两项 ((0,0) 合并单元格 + (0,2))；第 1 行有 (1,0) 和 (1,1) 被合并占用，因此它只有一项 (1,2)。```yaml
- elementId: table-merged
  elementType: table
  bounds: [100, 100, 600, 400]
  columnWidths: [0.33, 0.33, 0.34]
  rowHeights: [0.33, 0.33, 0.34]
  rows:
    # Row 0: top-left 2×2 merge + C1. The merged (0,1) is omitted
    - - text: "Merged cell"
        fill: {type: solid, color: "$accent"}
        rowSpan: 2
        colSpan: 2
      - text: "C1"
    # Row 1: (1,0) and (1,1) are occupied by the merge → only C2 remains
    - - text: "C2"
    # Row 2: full three columns
    - - text: "A3"
      - text: "B3"
      - text: "C3"
```---

### 图表（图表）

PPTD v2 的图表元素遵循 ECharts 理念：**图表顶层不包含“type”字段**；每个“series[i].type”确定其自己的形式。总共支持13个系列类型，按类型名称平铺，状态均等：

`bar` / `line` / `area` / `scatter` / `bubble` / `candlestick` / `pie` / `radar` / `waterfall` / `heatmap` / `treemap` / `sunburst` / `sankey`

每种类型在其自己的小节的第一行声明其**系列约束**：同一图表中允许的最大计数+它可以与哪些其他类型共存。

＃＃＃＃ 图表```ts
interface Chart extends ElementBase {
  elementType: "chart";

  data: ChartData;                            // required
  series: SeriesConfig[];                     // required; constraint: length ≥ 1
  seriesDefaults?: SeriesDefaults;            // default: not applied; common defaults grouped by series.type, merged with each series

  // —— Cartesian coordinate system (conditionally effective by series.type, see §5.3) ——
  xAxis?: AxisConfig | AxisConfig[];          // default: auto-adapt to data; in array form, referenced via series[i].xAxisIndex
  yAxis?: AxisConfig | AxisConfig[];          // default: auto-adapt to data; in array form, referenced via series[i].yAxisIndex
  barWidth?: number;                          // default: adaptive; constraint: (0, 1]; bar width / category slot width ratio
  barGap?: number;                            // default: 0 (flush); constraint: [0, 1); gap between bars when multiple bar series are grouped
  categoryGap?: number;                       // default: 0.2; constraint: [0, 1); blank ratio between category slots

  // —— Radar coordinate system (radar series only) ——
  spokeAxis?: SpokeAxisConfig;                // default: auto-adapt to data; spoke axes + spider grid

  // —— Global components ——
  title?: string | TitleConfig;               // default: no title
  legend?: boolean | LegendConfig;            // default: varies by type (see the default-value table in [LegendConfig](#legendconfig))
  dataLabels?: DataLabelConfig;               // default: not applied; global default, can be overridden by series.dataLabels
  fontFamily?: FontFamily;                    // default: falls back along the theme/master fonts

  // —— Chart frame (controls the rectangular container of the whole chart element, independent of series colors) ——
  fill?: Fill;                                // default: not applied
  border?: Border;                            // default: not applied
  shadow?: Shadow;                            // default: not applied
}

type SeriesConfig =
  | BarSeries | LineSeries | AreaSeries | ScatterSeries | BubbleSeries
  | CandlestickSeries | PieSeries | RadarSeries | WaterfallSeries
  | HeatmapSeries | TreemapSeries | SunburstSeries | SankeySeries;
```> `fill` / `border` / `shadow` 控制**图表元素的矩形框架**（作用于整个图表容器），与系列主体颜色无关。
>
> **PowerPoint限制：**原生图表不能整体旋转/翻转；也没有单一的全局不透明度属性覆盖“整个图表，包括标题、轴、图例、标签和系列”。当需要整体旋转/翻转/不透明度时，首先渲染为图像并将其视为 [Image](#image-image) 元素。

#### 图表数据```ts
interface ChartData {
  cols: string[];                                   // column names; constraint: unique, non-empty strings
  rows: (number | string | null)[][];               // constraint: each row's length = cols.length
}
```> **数据完整性约束**（由检查器验证）：
> - `cols` 中重复的列名 → `DuplicateColumnError`
> - `cols` 包含空字符串 → `EmptyColumnError`
> - `rows[i].length !== cols.length` → `RowLengthError`
> - 编码引用的列名不在 `cols` 中 → `UnknownColumnError`
> - 同一列被多个系列引用是合法的（例如，相同的 y 列按条绘制一次，按线绘制一次）
> - 当数字通道（`y`/`value`/`open`/`high`/`low`/`close`/`size`/`flow`）的列值是字符串时，将其解析为数字；失败时，会引发“NonNumericValueError”
>
> **如何写入缺失的单元格**：用“null”填充，例如`[null, null, 2, 3]`。 **不建议使用连续的逗号 `[, , 2, 3]` ——严格的 YAML 解析器会出错。

#### 一般规则

1. **系列的`填充`类型**：`颜色 |渐变填充`;字符串被视为实心 [Color](#color)（HEX8 或 `$xxx` 主题引用），对象被视为 [GradientFill](#fill)（带有 `type: "gradient"`）；某些类型支持 `(Color | GradientFill)[]` 数组形式（按切片/节点循环）。 **系列级填充不支持[ImageFill](#fill)**。
2. **类型混合**：图表的`series[]`可以包含哪些类型，由每个类型部分第一行的“系列约束”决定；检查器相应地进行验证。
3. **有条件有效的顶级字段**： `xAxis` / `yAxis` / `barWidth` / `barGap` / `categoryGap` / `spokeAxis` 是 **坐标系统级别** 配置，基于 `series[].type` 集合有条件有效（详细信息请参见[5.3 图表顶级字段的适用性](#53-applicability-of-chart-top-level-fields)）。
4. **[Color](#color) 主题引用范围**：`Color` 类型的所有字段（包括嵌套数组和对象内的每个 Color 位置）都支持 `$xxx` 主题引用 - 例如`upBars: {fill: "$success"}`、`colorScheme: ["$bg", "$primary"]`、`fill: ["$primary", "$accent"]`。
5. **可选对象类型字段的省略语义**：所有用“?”标记的**对象类型**字段（“xAxis”/“yAxis”/“spokeAxis”/“colorScale”/“marker”/“dataLabels”等）：当省略时，它们相当于该对象的空配置“{}”，并且所有子字段都采用自己的默认值 - 即“axes/grids/labels等”仍然默认渲染，只是自动推断参数”。这与 [ElementBase](#elementbase) 的 `fill` / `border` / `shadow` 不同（其中省略意味着**不应用**）。

> **条形/瀑布方向**：由轴类型确定 - 当`xAxis.type ===“category”`时垂直（默认），当`yAxis.type ===“category”`时水平。 `axis.type` 默认从数据列推断（字符串 → 类别，数字 → 值）；当数字列需要用作类别（例如年份）时，请使用 `axis.type: "category"` 显式覆盖。在水平情况下，“encode.x”引用数字列，“encode.y”引用类别列，“numberFormat”写在值轴所在的一侧。对于散点/气泡，x 和 y 都是数字通道，没有方向的概念。

---

#### 文本样式```ts
interface TextStyle {
  color?: Color;            // default: falls back along the inheritance chain (theme text color / PPTX master)
  fontSize?: number;        // default: auto-adapts to chart size
  fontFamily?: FontFamily;  // default: falls back along the inheritance chain to Chart.fontFamily or the theme font
}
```> 常见的三种文本样式，由 [TitleConfig](#titleconfig) / [LegendConfig](#legendconfig) / [DataLabelConfig](#datalabelconfig) / [AxisConfig](#axisconfig).label / [SpokeAxisConfig](#spokeaxisconfig).label 继承和重用；对于字体优先级链，请参阅[§3.2](#3-chart-styles)。

#### 线条样式配置```ts
interface LineStyleConfig {
  style?: "solid" | "dash" | "dot";    // default: "solid"
  color?: Color;                       // default: falls back to the theme
  width?: number;                      // default: 1
}
```> 通用线条样式，由 [AxisConfig](#axisconfig) / [SpokeAxisConfig](#spokeaxisconfig) 的 `axisLine` / `gridLine` 重用。

#### 标题配置```ts
interface TitleConfig extends TextStyle {
  text: string;                        // required
  // fontSize auto-adapts to chart size by default
}
```#### LegendConfig```ts
interface LegendConfig extends TextStyle {
  show?: boolean;                      // default: varies by type (see the table below)
  position?: "top" | "bottom" | "left" | "right";  // default: "bottom"
}
````show` 默认按类型：

|类型 |默认|
|---|---|
|条形图/折线图/面积图/散点图/气泡图/烛台图/饼图/雷达图| `真实` |
|瀑布| `假` |
|树状图/森伯斯特/桑基| `false`（名称和值已显示在图表上）|
|热图 |不使用 `chart.legend` （由 [series.colorbar](#heatmap) 控制）|

> `legend: false` 或 `legend: {show: false}` 将其关闭，**对所有 13 种类型有效**； `legend: true` 或对象形式仅对上表中标记为适用的类型具有视觉效果。

#### 数据标签配置```ts
interface DataLabelConfig extends TextStyle {
  show?: boolean;                      // default: false
  content?: "value" | "percentage" | "category";  // default: varies by type (see [5.5 value quick reference](#55-datalabelscontent-value-quick-reference))
  numberFormat?: string;               // default: no formatting; Excel number-format string (see below)
}
```> **numberFormat 标准**：采用 Excel 数字格式字符串的子集 — `0`（整数）/`0.0`（一位小数）/`0%`（百分比）/`0.0%`（带小数的百分比）/`#,##0`（千位分隔符）/`0.0E+00`（科学计数法）。 **不支持**诸如“[Red]”颜色部分、负数部分和条件格式等高级语法。

#### 标记配置```ts
interface MarkerConfig {
  shape?: "circle" | "rect" | "diamond" | "triangle";  // default: "circle"
  fill?: Color | GradientFill;         // default: follows the series body color
  border?: Border;                     // default: not applied
  size?: number;                       // default: auto-adapts to chart size; unit px
}
```> `rect` 命名与 [shapes.md](./shapes.md) 形状库一致。

#### 轴配置```ts
interface AxisConfig {
  show?: boolean;                      // default: true
  type?: "category" | "value";         // default: inferred from the data column (string → category, number → value)
  min?: number;                        // default: auto-adapt to data; only effective for value axes
  max?: number;                        // default: auto-adapt to data; only effective for value axes
  reverse?: boolean;                   // default: false; true = reverse the axis direction (maximum at the origin side)
  title?: string | TitleConfig;        // default: no title; the string form is recommended, use the object only for special styling
  label?: boolean | (TextStyle & {     // default: true; tick labels
    numberFormat?: string;             // default: no formatting; only effective for value axes
  });
  axisLine?: boolean | (LineStyleConfig & {  // default: true
    arrow?: boolean | "start" | "end" | "both";  // default: false; true is equivalent to "end"
  });
  gridLine?: boolean | LineStyleConfig;     // default: true
}
```#### 系列默认值```ts
interface SeriesDefaults {
  bar?: Partial<Omit<BarSeries, "type" | "encode">>;
  line?: Partial<Omit<LineSeries, "type" | "encode">>;
  area?: Partial<Omit<AreaSeries, "type" | "encode">>;
  scatter?: Partial<Omit<ScatterSeries, "type" | "encode">>;
  bubble?: Partial<Omit<BubbleSeries, "type" | "encode">>;
  candlestick?: Partial<Omit<CandlestickSeries, "type" | "encode">>;
  radar?: Partial<Omit<RadarSeries, "type" | "encode">>;
}
```> 为该类型的所有系列提供通用默认值，避免多个系列之间的重复。有关合并算法和可用类型的范围，请参阅[§3.4](#3-chart-styles)。

#### 辐轴配置

仅由[雷达](#radar)系列使用。```ts
interface SpokeAxisConfig {
  show?: boolean;                      // default: true
  min?: number;                        // default: 0; minimum of the value axis shared by all dimensions
  max?: number;                        // default: auto-adapt to data; maximum of the value axis shared by all dimensions
  label?: boolean | (TextStyle & {     // default: true; tick labels
    numberFormat?: string;             // default: no formatting
  });
  axisLine?: boolean | LineStyleConfig;     // default: true; spoke lines from the center to the outer ring
  gridLine?: boolean | LineStyleConfig;     // default: true; spider grid lines (concentric polygons connecting the spoke endpoints)
}
```#### LinearSeriesBase

[line](#line) / [area](#area) / [radar](#radar) 共享的曲线类公共字段。```ts
interface LinearSeriesBase {
  smooth?: boolean;                                   // default: false
  lineStyle?: "solid" | "dash" | "dot";               // default: "solid"
  width?: number;                                     // default: 2
  marker?: false | MarkerConfig;                      // default: not applied
  nullHandling?: "zero" | "gap" | "connect";          // default: "gap" for line/area, "connect" for radar
  lineColor?: Color | GradientFill;                   // default: follows the theme color cycle; line color of line / polygon stroke color of area+radar
}
```> 如果同一个图表内的多个线/面/雷达系列设置了不同的`nullHandling`值，则只有**第一个非空值**生效，其他系列跟随；不支持多个空处理方法。

---

####酒吧

> **系列约束**：可以与`线/面/散点/气泡`自由混合；也可以与“烛台”混合使用；同一图表中的柱形系列数量没有限制。```ts
interface BarSeries {
  type: "bar";
  encode: { x: string; y: string };    // required
  name?: string;                       // default: the encode.y column name; for legend display only
  xAxisIndex?: number;                 // default: 0; meaningful only when chart.xAxis is an array
  yAxisIndex?: number;                 // default: 0; meaningful only when chart.yAxis is an array
  stack?: "value" | "percent";         // default: no stacking; "value" sums directly, "percent" normalizes to 100%
  symbol?: ShapeDef;                   // default: normal rectangular bar; pictographic bar shape definition (see [ShapeDef](#image-image))
  fill?: Color | GradientFill;         // default: follows the theme color cycle
  border?: Border;                     // default: not applied
  dataLabels?: DataLabelConfig;        // default: not shown; content only takes "value"
}
```####线

> **系列约束**：可以与“条形/面积/散点/气泡”自由混合；也可以与“烛台”混合。```ts
interface LineSeries extends LinearSeriesBase {
  type: "line";
  encode: { x: string; y: string };    // required
  name?: string;                       // default: the encode.y column name
  xAxisIndex?: number;                 // default: 0
  yAxisIndex?: number;                 // default: 0
  dataLabels?: DataLabelConfig;        // default: not shown; content only takes "value"
}
```> 线没有区域，因此不提供“fill”；对于其他曲线类字段，请参阅 [LinearSeriesBase](#linearseriesbase)。

####区域

> **系列约束**：可以与`bar / line / scatter / bubble`自由混合；也可以与“烛台”混合。```ts
interface AreaSeries extends LinearSeriesBase {
  type: "area";
  encode: { x: string; y: string };    // required
  name?: string;                       // default: the encode.y column name
  xAxisIndex?: number;                 // default: 0
  yAxisIndex?: number;                 // default: 0
  stack?: "value" | "percent" | "stream";  // default: no stacking; "stream" = streamgraph (area only)
  areaColor?: Color | GradientFill;    // default: derived from lineColor as semi-transparent
  dataLabels?: DataLabelConfig;        // default: not shown
}
```> **堆叠分组规则**：在同一图表内，**所有设置了“堆叠”的同类型系列都会自动分组为一个堆叠**，无需显式的组标识。同一图表中最多支持一个相同类型的堆栈组 - 所有设置“堆栈”的系列必须使用相同的值（所有“值”/所有“百分比”/所有“流”）；混合会引发“StackModeMismatchError”；没有“stack”的系列独立显示。如果需要多个独立的堆栈组，请将它们拆分为多个图表元素。
>
> **`stream`仅适用于区域**：`value`归一化+中心基线偏移；堆叠区域在 y=0 上方和下方对称，呈“流图”形状。

#### 分散

> **系列约束**：可以与“条形/线形/面积/气泡”自由混合。```ts
interface ScatterSeries {
  type: "scatter";
  encode: { x: string; y: string };    // required; each series references its own x/y column pair
  name?: string;                       // default: the encode.y column name
  yAxisIndex?: number;                 // default: 0
  dataFilter?: { col: string; value: string | number };  // default: no filtering; optional: group with a long table
  marker?: MarkerConfig;               // default: {shape: "circle"}; constraint: cannot be false
  fill?: Color | GradientFill;         // default: follows the theme color cycle; serves as the marker's default fill color (marker.fill takes precedence)
  border?: Border;                     // default: not applied; serves as the marker's default border (marker.border takes precedence)
  dataLabels?: DataLabelConfig;        // default: not shown; content only takes "value"
}
```####气泡

> **系列约束**：可以与“条形/线形/面积/散点”自由混合。```ts
interface BubbleSeries {
  type: "bubble";
  encode: { x: string; y: string; size: string };  // required
  name?: string;                       // default: the encode.y column name
  yAxisIndex?: number;                 // default: 0
  dataFilter?: { col: string; value: string | number };  // default: no filtering; rows where the col column equals value are used as this series' data
  sizeScale?: "linear" | "sqrt" | "log";  // default: "sqrt"
  sizeRange?: [number, number];        // default: auto-adapts to chart size; bubble radius range in px
  fill?: Color | GradientFill;         // default: follows the theme color cycle; bubble fill color
  border?: Border;                     // default: not applied
  dataLabels?: DataLabelConfig;        // default: not shown; content only takes "value"
}
```> **sizeScale**：`sqrt`（默认）使面积与大小成比例； “线性”使半径与尺寸成正比； “log”适合具有数量级差异的场景。负大小被视为 0。对于多个组，请使用宽表 + 空填充，每个系列引用自己的“x/y/size”列三元组。

#### 烛台

> **系列约束**：只能与“条形图/折线图/面积图”混合使用（常见用法：烛台主体 + 覆盖 MA 移动平均线的线）。```ts
interface CandlestickSeries {
  type: "candlestick";
  /**
   * encode.open is optional → determines the rendering mode
   *   open provided → OHLC candlestick (rendered with 4 series; a solid body expresses the open-close direction)
   *   open omitted → HLC high-low-close (rendered with 3 series; a vertical line + dot marker at close, no body)
   */
  encode: { x: string; high: string; low: string; close: string; open?: string };
  xAxisIndex?: number;                 // default: 0
  yAxisIndex?: number;                 // default: 0
  upBars?:   { fill?: Color; border?: Border };   // rising bar (close > open) style; only effective in OHLC mode (HLC has no body)
  downBars?: { fill?: Color; border?: Border };   // falling bar (close ≤ open) style; only effective in OHLC mode
  wickStyle?: Border;                  // wick (high-low vertical line) style; common to HLC / OHLC
}
```>
> **日期列处理**：日期列（例如“2024-01-01”）被视为字符串类别，按照它们在“行”中出现的顺序在 x 轴上等间隔排列，自然会跳过非交易日。如果需要按实际日期间隔进行精确布局，建议使用空行手动填充空交易日。

#### 馅饼

> **系列约束**：`series` 数组只能有 1 个元素，并且不能与其他类型共存。```ts
interface PieSeries {
  type: "pie";
  encode: { category: string; value: string };  // required
  innerRadius?: number;                // default: 0; constraint: [0, 1]; > 0 = donut
  startAngle?: number;                 // default: 0 (12 o'clock direction)
  fill?: Color | GradientFill | (Color | GradientFill)[];   // default: follows the theme color cycle; an array cycles by slice
  border?: Border;                     // default: not applied
  dataLabels?: DataLabelConfig;        // default: not shown; content takes "value" | "percentage" | "category", default "value"
}
```> **角度方向**：固定**顺时针**为正； 0° = 12 点钟位置，90° = 3 点钟位置，180° = 6 点钟位置，270° = 9 点钟位置。

####雷达

> **系列约束**：同一个图表中允许有多个雷达系列（共享一组辐条），但所有系列的类型必须是“雷达”；它可能无法与其他类型共存。```ts
interface RadarSeries extends LinearSeriesBase {
  type: "radar";
  encode: { category: string; y: string };    // required; the category column holds the spoke labels
  name?: string;                       // default: the encode.y column name
  areaColor?: Color | GradientFill;    // default: derived from lineColor as semi-transparent; polygon fill color
  dataLabels?: DataLabelConfig;        // default: not shown; content only takes "value"
}
```> 雷达图的辐条轴线、蜘蛛网格和值范围（最小/最大）通过图表顶层 [spokeAxis](#spokeaxisconfig) 统一配置，由多个系列共享。
>
> **维度列共享约束**：同一图表中的所有雷达系列必须引用相同的“类别”列（即所有多边形共享相同的辐条标签集）。要显示具有不同辐条标签的雷达图，请使用多个图表元素。检查器验证：所有雷达系列的“encode.category”必须相同。

####瀑布

> **系列约束**：`series` 数组只能有 1 个元素，并且不能与其他类型共存。```ts
interface WaterfallSeries {
  type: "waterfall";
  encode: {
    x: string;                         // category column
    y: string;                         // value column (floating bars hold the increase/decrease amounts; total columns hold the absolute value of the cumulative total)
    isTotal?: string;                  // default: omitted = all floating bars; after specifying a bool column, true = total column (opening/subtotal/closing), false/null = floating bar
  };
  totalBars?:    { fill?: Color; border?: Border };   // total column (opening/subtotal/closing, isTotal=true) style
  increaseBars?: { fill?: Color; border?: Border };   // floating increase bar (y > 0) style
  decreaseBars?: { fill?: Color; border?: Border };   // floating decrease bar (y < 0) style
  dataLabels?: DataLabelConfig;        // default: not shown; content takes "value" | "category", default "value"
}
```> **颜色**：瀑布不使用`fill`；颜色通过 isTotal 和 y 的符号通过三个类别 `totalBars` / `increaseBars` / `decreaseBars` 进行映射；所有总计列（开始/小计/结束）共享“totalBars”。
>
> **isTotal语义**：可以出现在任意位置（第一行/中间小计/最后一行都是合法的）；每个“isTotal=true”都是一个独立的总计列，其 y 值应等于“前一个总计列的 y + 所有中间浮动条的 y 之和”——如果不匹配，检查器会输出“WaterfallTotalMismatchWarning”（直接定义第一行总计列的 y）。 `isTotal` 列只接受 bool 或 null；字符串“true”或数字“1”都会引发“InvalidValueError”。

#### 热图

> **系列约束**：`series` 数组只能有 1 个元素，并且不能与其他类型共存。```ts
interface HeatmapSeries {
  type: "heatmap";
  encode: { x: string; y: string; value: string };  // required; the x and y columns must be categories (string), value is numeric
  colorScheme?: Color[];               // default: falls back to the theme; gradient color-scale endpoints
  colorScale?: {
    type?: "linear" | "diverging";     // default: "linear"
    domain?: [number, number];         // default: data range; for type=diverging, default [-max(|v|), +max(|v|)], 0 centered
  };
  colorbar?: boolean | LegendConfig;   // default: true; color-scale bar legend; position default "right"
  dataLabels?: DataLabelConfig;        // default: not shown; content only takes "value"
}
```> **颜色**：`colorScheme` 作为渐变端点，根据 `colorScale.type` 进行插值：
> - `线性`：`colorScheme`长度≥2，在端点之间插值；
> - `发散`: `colorScheme` 长度 = 3 (低/中/高);中点由“域”的中间确定，常用于“负-中性-正”场景。
>
> **数据布局**：x / y 列固定为类别（字符串）；轴上的类别顺序遵循“行”中首次出现的顺序；未出现在“行”中的 (x, y) 组合将被视为缺失单元格，呈现透明（PPTX 中的背景颜色）。对于完整的矩阵，建议显式列出所有 (x, y) 组合并使用 null 来表示缺失值。
>
> 热图不使用 `chart.legend`；它被 `colorbar` 取代； `colorbar: false` 关闭色标栏。

#### 树形图

> **系列约束**：`series` 数组只能有 1 个元素，并且不能与其他类型共存。```ts
interface TreemapSeries {
  type: "treemap";
  encode: {
    category: string;                  // node-name column
    value: string;                     // value column
    parent?: string;                   // parent-node column; null/missing/empty = root node (multiple roots allowed)
  };
  levels?: number;                     // default: show all levels
  fill?: Color | GradientFill
       | (Color | GradientFill)[]
       | (Color | GradientFill)[][];   // default: follows the theme color cycle; see "color derivation rules" below
  border?: Border;                     // default: not applied
  dataLabels?: DataLabelConfig;        // default: not shown; content takes "value" | "category", default "category"
}
```> **颜色派生规则**：
> - `填充：颜色| GradientFill`（单值）：所有根节点共享此颜色；子节点是通过每级降低 10% 的亮度（沿 HSL.L 维度）而派生的。
> - `fill: (Color | GradientFill)[]` (一维数组)：按照根节点出现的顺序循环；每个根节点的子节点都是通过每级降低 10% 的亮度来导出的。
> - `fill: (Color | GradientFill)[][]` (二维数组)：外部维度按根节点循环，内部维度直接指定层级（绕过自动求导）；如果内部数组不够长，无法覆盖所有级别，则剩余级别仍通过将亮度降低 10% 来导出。

#### 旭日

> **系列约束**：`series` 数组只能有 1 个元素，并且不能与其他类型共存。```ts
interface SunburstSeries {
  type: "sunburst";
  encode: { category: string; value: string; parent?: string };  // required
  levels?: number;                     // default: show all levels
  fill?: Color | GradientFill | (Color | GradientFill)[];   // default: follows the theme color cycle; an array cycles by top-level node
  border?: Border;                     // default: not applied
  dataLabels?: DataLabelConfig;        // default: not shown; content takes "value" | "category", default "category"
}
```#### 桑基

> **系列约束**：`series` 数组只能有 1 个元素，并且不能与其他类型共存。```ts
interface SankeySeries {
  type: "sankey";
  encode: {
    source: string;                    // source-node column
    target: string;                    // target-node column
    flow: string;                      // flow column
  };
  nodeAlign?: "left" | "right" | "justify";  // default: "justify"
  fill?: Color | GradientFill                     // default: follows the theme color cycle (colors picked in node topological order)
       | (Color | GradientFill)[]                // an array cycles by node
       | Record<string, Color | GradientFill>;   // mapped by node name; unspecified nodes fall back to the theme color cycle
  border?: Border;                     // default: not applied
  dataLabels?: DataLabelConfig;        // default: not shown; content takes "value" | "category", default "value"
}
```> **图约束**：桑基仅限于**有向无环图（DAG）**；当源/目标列形成循环时，检查器会引发“CyclicGraphError”。
>
> **节点顺序**：“源”和“目标”列已去重并按拓扑顺序排列。当 fill 是数组时，按此顺序循环；当数组长度<节点数时，则回绕并重用；当 > 节点数时，它会被截断。对象形式与节点名称完全匹配。

---

#### 字段快速参考

##### 5.1 数据编码通道

|类型 |编码通道 |
|---|---|
|条形图/线形图/面积图 | `x` + `y` |
|分散| `x` + `y` （多系列使用宽表 + null 填充）|
|泡泡| `x` + `y` + `size` （多系列使用宽表 + null 填充）|
|烛台|蜡烛图：`x` + `开盘` + `收盘` + `最低价` + `最高价`；覆盖：`x` + `y` |
|馅饼| `类别` + `值` |
|雷达| `类别` + `y` |
|瀑布| `x` + `y` + 可选的 `isTotal` （布尔列） |
|热图 | `x` + `y` + `值` |
|树状图/旭日 | `类别` + `值` + 可选的`父级` |
|桑基 | `源` + `目标` + `流` |

> **命名约定**：笛卡尔坐标系（条形图/线形图/面积图/散点图/气泡图/烛台图/瀑布图/热图）使用“x”/“y”作为水平/垂直轴；非笛卡尔类别字段统一为“类别”（饼图/雷达/树状图/森伯斯特）；桑基图边缘端点使用“源”/“目标”。

##### 5.2 颜色机制

|类型 |色域 |描述 |
|---|---|---|
|酒吧| `系列[].fill` |栏体填充颜色 |
|线 | `系列[].lineColor` |线条颜色（无区域）|
|地区 | `series[].lineColor` + `series[].areaColor` |描边颜色 + 区域颜色（省略区域颜色时，源自 lineColor 为半透明）|
|雷达| `series[].lineColor` + `series[].areaColor` |多边形描边+填充（与面积相同）|
|分散| `series[].fill` / `marker.fill` |系列级别是标记默认颜色； `marker.fill` 优先 |
|泡泡| `系列[].fill` |气泡填充颜色 |
|烛台| `upBars` / `downBars`（主体填充+边框）+每个叠加系列自己的`lineColor`/`fill` |烛台主体按向上/向下映射；覆盖线/条使用自己的颜色|
|派/森伯斯特/桑基|单系列“fill”数组，按数据点/节点循环|数组长度循环和重用 |
|树形图 |单系列‘fill’（单值/一维数组/二维数组）|与馅饼等相同；子节点沿 HSL.L 维度从父节点减少 (`L_new = max(0, L_old - 10)`) |
|热图 | `series[].colorScheme` + `series[].colorScale` |渐变色标按值映射； “线性”在端点之间进行插值，“发散”在中点对齐三种颜色 |
|瀑布| `series[].totalBars` / `increaseBars` / `decreaseBars` |映射为三类——总计（总列）/增加/减少；不参与主题色循环|

##### 5.3 图表顶级字段的适用性

|顶级领域|适用系列类型 |
|---|---|
| `x 轴` |条形图/折线图/面积图/散点图/气泡图/烛台图/瀑布图/热图|
| `y 轴` |条形图/折线图/面积图/散点图/气泡图/烛台图/瀑布图/热图|
| `barWidth` / `barGap` |栏/瀑布（栏布局参数）|
| `类别差距` |条形图/烛台/瀑布图（类别间距参数）|
| `辐轴` |雷达（包括辐条轴线+蜘蛛网格+最小/最大）|
| `传奇` |条形图/折线图/面积图/散点图/气泡图/烛台图/饼图/雷达图/瀑布图|
| `标题` |全部 |
| `数据标签` |全部（烛台：仅对叠加系列有效；烛台本身通过 upBars/downBars 表达向上/向下角色）|
| `字体家族` |全部 |

> **轴单值与数组规则**：辅助轴始终放置在值轴所在的**一侧 - 垂直图表使用 `yAxis` 数组 + `yAxisIndex`，水平图表使用 `xAxis` 数组 + `xAxisIndex`。当任何系列使用 `xAxisIndex > 0` / `yAxisIndex > 0` 时，对应的 `xAxis` / `yAxis` 必须是一个数组（长度 ≥ max(index) + 1）。

##### 5.4 类型混合兼容性快速参考```
bar / line / area / scatter / bubble may coexist with each other freely; candlestick may only coexist with bar / line / area
```> 其他7种类型各自独占系列数组；有关详细约束，请参阅相应部分的第一行。

##### 5.5 dataLabels.content 值快速参考

|类型 |允许值 |默认|
|---|---|---|
|条形图/折线图/面积图/散点图/气泡图/雷达图/热图| `价值` | `价值` |
|烛台（仅限覆盖系列）| `价值` | `价值` |
|馅饼| `值` / `百分比` / `类别` | `价值` |
|瀑布| `值` / `类别` | `价值` |
|树状图/旭日 | `值` / `类别` | `类别` |
|桑基 | `值` / `类别` | `价值` |

> 在此表之外写入一个值 → 检查器引发 `InvalidValueError`。烛台主体本身不支持数据标签。

---

#### 示例

**条形图（堆叠）**```yaml
- elementId: c1
  elementType: chart
  bounds: [50, 100, 600, 400]
  data:
    cols: [quarter, revenue, cost]
    rows:
      - [Q1, 120, 220]
      - [Q2, 132, 182]
      - [Q3, 101, 191]
      - [Q4, 134, 234]
  seriesDefaults:
    bar: {stack: value}
  series:
    - type: bar
      encode: {x: quarter, y: revenue}
      name: Revenue
      fill: "$primary"
    - type: bar
      encode: {x: quarter, y: cost}
      name: Expenses
      fill: "$accent"
```**折线图（多系列区分）**```yaml
- elementId: c2
  elementType: chart
  bounds: [50, 100, 600, 400]
  data:
    cols: [month, actual, target, baseline]
    rows:
      - [Jan, 72, 65, 50]
      - [Feb, 85, 70, 50]
      - [Mar, null, 78, 50]
      - [Apr, 90, 82, 50]
  yAxis: {min: 0, max: 100, gridLine: {color: "#f0f0f0"}}
  series:
    - type: line
      encode: {x: month, y: actual}
      name: Actual
      lineColor: "#5470c6"
      lineStyle: solid
      width: 3
      smooth: true
    - type: line
      encode: {x: month, y: target}
      name: Target
      lineColor: "#ee6666"
      lineStyle: dash
      smooth: true
    - type: line
      encode: {x: month, y: baseline}
      name: Baseline
      lineColor: "#999999"
      lineStyle: dot
      width: 1
      marker: false
```**面积图（流堆叠）**```yaml
- elementId: c3
  elementType: chart
  bounds: [50, 80, 700, 400]
  title: Traffic Evolution by Channel
  data:
    cols: [week, web, app, partner]
    rows:
      - [W1, 200, 120, 80]
      - [W2, 240, 160, 90]
      - [W3, 260, 200, 110]
      - [W4, 280, 240, 130]
  seriesDefaults:
    area: {stack: stream}
  series:
    - type: area
      encode: {x: week, y: web}
      name: Web
      areaColor: "#5470c6"
    - type: area
      encode: {x: week, y: app}
      name: App
      areaColor: "#91cc75"
    - type: area
      encode: {x: week, y: partner}
      name: Partner
      areaColor: "#fac858"
```**气泡图（多系列分组）**```yaml
- elementId: c5
  elementType: chart
  bounds: [50, 80, 700, 420]
  title: User Distribution
  xAxis: {title: Age}
  yAxis: {title: "Annual income (10K)"}
  data:
    cols: [age_s, inc_s, pop_s, age_w, inc_w, pop_w, age_m, inc_m, pop_m]
    rows:
      - [22, 5, 120, 28, 12, 380, 45, 40, 180]
      - [null, null, null, 35, 25, 260, 52, 60, 90]
  seriesDefaults:
    bubble:
      sizeScale: sqrt
      sizeRange: [8, 48]
  series:
    - type: bubble
      encode: {x: age_s, y: inc_s, size: pop_s}
      name: Students
      fill: "#5470c6"
    - type: bubble
      encode: {x: age_w, y: inc_w, size: pop_w}
      name: White-collar
      fill: "#91cc75"
    - type: bubble
      encode: {x: age_m, y: inc_m, size: pop_m}
      name: Management
      fill: "#ee6666"
```**蜡烛图（带有 MA5 叠加线）**```yaml
- elementId: c6
  elementType: chart
  bounds: [50, 80, 700, 420]
  title: Stock Price Trend
  data:
    cols: [date, open, high, low, close, ma5]
    rows:
      - ["2024-01-01", 100, 110, 95, 108, null]
      - ["2024-01-02", 108, 115, 105, 112, null]
      - ["2024-01-03", 112, 118, 109, 116, null]
      - ["2024-01-04", 116, 120, 110, 113, null]
      - ["2024-01-05", 113, 117, 108, 115, 112.8]
  yAxis: {title: Price}
  series:
    - type: candlestick
      encode: {x: date, open: open, close: close, low: low, high: high}
      upBars: {fill: "#ee6666"}
      downBars: {fill: "#5470c6"}
    - type: line
      encode: {x: date, y: ma5}
      name: MA5
      smooth: true
      width: 2
      lineColor: "#fac858"
```**瀑布图**```yaml
- elementId: c9
  elementType: chart
  bounds: [50, 80, 700, 380]
  title: Cash Flow Waterfall
  data:
    cols: [phase, amount, total]
    rows:
      - [Opening balance, 500, true]
      - [Sales revenue, 300, null]
      - [Operating expenses, -180, null]
      - [Taxes, -60, null]
      - [Closing balance, 560, true]
  series:
    - type: waterfall
      encode: {x: phase, y: amount, isTotal: total}
      totalBars: {fill: "#5470c6"}
      increaseBars: {fill: "#91cc75"}
      decreaseBars: {fill: "#ee6666"}
      dataLabels: {show: true}
```**热图**```yaml
- elementId: c10
  elementType: chart
  bounds: [50, 80, 700, 420]
  title: User Activity Heatmap
  data:
    cols: [hour, day, count]
    rows:
      - ["00:00", Mon, 5]
      - ["00:00", Tue, 8]
      - ["06:00", Mon, 22]
      - ["12:00", Mon, 45]
  series:
    - type: heatmap
      encode: {x: hour, y: day, value: count}
      colorScheme: ["#ffffff", "#5470c6"]
      colorScale: {domain: [0, 50]}
```**树形图（带层次结构）**```yaml
- elementId: c11
  elementType: chart
  bounds: [50, 80, 700, 420]
  title: Budget Allocation
  data:
    cols: [dept, parentDept, budget]
    rows:
      - [Engineering, null, 1000]
      - [Frontend, Engineering, 400]
      - [Backend, Engineering, 600]
      - [Sales, null, 800]
  series:
    - type: treemap
      encode: {category: dept, value: budget, parent: parentDept}
      fill: ["#5470c6", "#91cc75"]
```**桑基图**```yaml
- elementId: c13
  elementType: chart
  bounds: [50, 80, 800, 420]
  title: User Conversion Funnel
  data:
    cols: [from, to, users]
    rows:
      - [Ad campaign, Landing page, 10000]
      - [Ad campaign, Direct search, 4000]
      - [Landing page, Sign-up, 3000]
      - [Landing page, Drop-off, 7000]
      - [Sign-up, First order, 1200]
      - [Direct search, First order, 2000]
  series:
    - type: sankey
      encode: {source: from, target: to, flow: users}
      nodeAlign: justify
      fill: ["#5470c6", "#91cc75", "#fac858", "#ee6666", "#73c0de"]
```**系列默认+混合：条形默认宽度+线条默认平滑**```yaml
- elementId: c15
  elementType: chart
  bounds: [50, 80, 700, 420]
  data:
    cols: [month, sales, growth]
    rows:
      - [Jan, 120, 0.10]
      - [Feb, 150, 0.25]
      - [Mar, 180, 0.20]
  yAxis:
    - {title: Sales}
    - {title: Growth rate, label: {numberFormat: "0%"}}
  seriesDefaults:
    bar:
      fill: "#5470c6"
    line:
      smooth: true
      width: 2
      lineColor: "#ee6666"
  barWidth: 0.6
  series:
    - type: bar
      encode: {x: month, y: sales}
      name: Sales
    - type: line
      encode: {x: month, y: growth}
      name: Growth rate
      yAxisIndex: 1
```**水平条形图（y 侧为类别轴）**```yaml
- elementId: c16
  elementType: chart
  bounds: [50, 80, 600, 360]
  title: Headcount by Department
  data:
    cols: [dept, headcount]
    rows:
      - [Engineering, 120]
      - [Product, 45]
      - [Design, 30]
      - [Operations, 60]
  xAxis: {label: {numberFormat: "#,##0"}}
  yAxis: {label: {fontSize: 12}}
  series:
    - type: bar
      encode: {x: headcount, y: dept}
      fill: "$primary"
      dataLabels: {show: true}
```

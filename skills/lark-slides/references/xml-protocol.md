# XML 协议（SML 2.0）

飞书 Slides 结构化标记语言。本文件是 `slides_xml_schema_definition.xml`（XSD，唯一事实源）的分层摘要，只保留生成 PPT 真正会用到的元素、属性与已知坑；任何冲突以 XSD 为准。版式与布局节奏见 layout.md，写盘/导出 CLI 见 cli-operations.md，`<whiteboard>` 内部的 mermaid/SVG 语法见 whiteboard.md。

- 画布固定 **960×540**（`<presentation width="960" height="540">`，两属性必填）。坐标原点在左上，X 右增、Y 下增，单位 px。
- **播放时只显示画布内 (0,0)–(960,540) 的内容**。坐标域虽允许 X∈[-8640,10560]、Y∈[-4860,5940]（可用于出血或临时移出画布），但超出画布的部分不可见——正常内容必须落在 0–960 × 0–540 内。
- 命名：元素小写、属性 camelCase、枚举 kebab-case。渲染层叠 = 文档顺序，先写的在底层；装饰形状必须写在文字之前。
- 所有元素的 `id` 均可选（创建时可不写；块级编辑时由 CLI 自动注入）。
- 只允许 XSD 里定义的元素/属性，未定义的（如 `<formula>`、`iconKeywords`、臆造的字距/字体属性）一律校验失败。

## 元素分类（taxonomy）

- **根/结构**：`<presentation>` → `<title>?` `<theme>?` `<slide>`(1–100) → `<style>?` `<data>?` `<note>?`。`<slide>` 的直接子元素**只有** style/data/note，不能直接放页面元素或裸文本。
- **页面元素**（`<data>` 的直接子元素，互为兄弟，靠坐标定位，不互相嵌套）：`<shape>` `<img>` `<icon>` `<chart>` `<table>` `<line>` `<polyline>` `<whiteboard>` `<undefined>`。
- **文本容器**：`<content>`——只能作为 `<shape>` 或 `<td>` 的子元素；承载所有文本样式默认值。
- **结构文本**：`<p>` `<ul>`/`<ol>` `<li>`——控制文本流（段落、列表层级），不控制外观。
- **内联元素**（只能在 `<p>` 及内联元素内部）：`<span>` `<br/>` `<strong>` `<em>` `<u>` `<del>` `<a>` `<shadow>` `<outline>`。
- **视觉属性**（一律作为**子元素**，不是属性）：`<fill>`/`<fillColor>`/`<fillImg>`/`<fillPattern>`、`<border>`、`<shadow>`、`<reflection>`、`<crop>`、`<startArrow>`/`<endArrow>`。`<shape fill="...">`、`<td border="...">` 均非法。

## 最小示例

```xml
<presentation width="960" height="540">
  <slide id="s1">
    <style><fill><fillColor color="rgba(248,249,251,1)"/></fill></style>
    <data>
      <shape type="text" topLeftX="80" topLeftY="70" width="800" height="70">
        <content textType="title" fontSize="40" fontFamily="思源黑体" color="rgba(31,35,41,1)" textAlign="left" verticalAlign="top">
          <p>页面标题</p>
        </content>
      </shape>
      <shape type="round-rect" topLeftX="80" topLeftY="200" width="360" height="140" presetHandlers="8">
        <fill><fillColor color="rgba(255,255,255,1)"/></fill>
        <border color="rgba(224,226,230,1)" width="1"/>
        <content fontSize="16" fontFamily="思源黑体" color="rgba(51,51,51,1)" textAlign="left" verticalAlign="top" paddingTop="16" paddingLeft="16">
          <p><span bold="true">要点</span>正文说明文字。</p>
          <ul><li><p>列表项一</p></li><li><p>列表项二</p></li></ul>
        </content>
      </shape>
    </data>
    <note><content><p>演讲者备注纯文本</p></content></note>
  </slide>
</presentation>
```

## 页面元素规范

### shape
容器与图形，唯一可直接携带文本的元素。
- 必填：`type`（见形状枚举，`text`=文本框）、`topLeftX` `topLeftY` `width` `height`。
- 可选：`rotation`[0,360) `alpha`[0,1] `flipX`/`flipY` `path`（仅 `type="custom"`，SVG 路径串，且只有 custom 允许）`presetHandlers`（如 round-rect 圆角半径）`vert`（horz/vert/vert270/word-art-vert/word-art-vert-rtl/ea-vert）。
- 子元素（各至多一个）：`<fill>` `<border>` `<reflection>` `<shadow>` `<content>`。
- 默认填充：`type="text"` 透明 `rgba(255,255,255,0)`，其余类型 `rgba(222,224,227,1)`。空 `<border/>`：text 透明边框，其余取填充色 S+10%/B-10%。
- 形状枚举含 rect、round-rect、ellipse、triangle、diamond、parallelogram、trapezoid、pentagon…、star4/5/6…、各类 arrow、callout、`flow-chart-*`、`action-button-*`、donut/arc/pie/chevron/can/cube 等；完整列表见 XSD `ShapeType`。

### img
- 必填：`src`、`topLeftX` `topLeftY` `width` `height`。`src` **只接受** `+media-upload` 返回的 file_token 或 `@本地相对路径` 占位符；**禁止** http/https 外链、禁止绝对路径。
- 可选：`rotation` `flipX`/`flipY` `alpha` `alt` `exposure` `contrast` `saturation` `temperature`。
- 子元素：`<crop>` `<border>` `<reflection>` `<shadow>` `<fill>`。
- **width/height 是裁剪后的显示尺寸**。原图先等比缩放铺满该区域再裁切，比例不符会裁掉多余部分；信息类图（图表/截图/示意图）须按原始比例给尺寸，无法确定原图比例时不要在 `<crop>` 上写 offset（会拉伸变形）。
- `<crop>` 属性：`type`（默认 rect，支持任意 ShapeType，如 ellipse 做头像、round-rect+`presetHandlers` 做圆角）、`leftOffset`/`rightOffset`/`topOffset`/`bottomOffset`（px，正值内裁、负值外扩）、`presetHandlers`、`path`（仅 custom）。

### icon
IconPark 图标，独立视觉对象。
- 必填：`topLeftX` `topLeftY` `width` `height`。
- 关键属性 `iconType`：IconPark 索引路径，形如 `iconpark/Base/setting.svg`（默认值同此）。**不是** iconKeywords。
- 可选：`rotation` `flipX`/`flipY` `alpha`。
- 子元素：`<fill>` `<border>` `<reflection>` `<shadow>`。**图标必须有非透明填充才可见**——显式写 `<fill><fillColor color="rgba(...,1)"/></fill>`；空 `<fill/>` 用默认灰 `rgba(208,211,214,1)`，无 `<fill>` 标签则不填充（不可见）。

### line / polyline
- `<line>` 用**绝对端点坐标** `startX` `startY` `endX` `endY`（全协议唯一的坐标例外；必填）；`type` 默认 straight-connector1，可选 `alpha`。
- `<polyline>`（折线/曲线）用外接矩形 `topLeftX/topLeftY/width/height`（必填）+ `type`（bent-connector2–5 / curved-connector2–5，默认 bent-connector2）+ `presetHandlers` `rotation` `flipX/flipY` `alpha`。
- 两者 `<border>` **必填**——无 border 线不可见；空 `<border/>` = `rgba(43,47,54,1)` / 宽 2px。可选子元素 `<startArrow>` `<endArrow>`（`type`：none/arrow/empty-triangle/solid-triangle/empty-diamond/solid-diamond/empty-circle/solid-circle，`widthScale`/`heightScale`：sm/med/lg）、`<shadow>` `<reflection>`。

### table
- 必填：`topLeftX` `topLeftY`（可选 `flipX/flipY`）。表格无 width/height，宽=各列 width 之和。
- 结构：`<colgroup>`?→`<col span?="1" width?="110"/>`；`<tr height?="37">`(多个)→`<td colspan?="1" rowspan?="1">`(多个)。
- `<td>` 子元素（均为子元素而非属性）：`<borderTop>`/`<borderRight>`/`<borderBottom>`/`<borderLeft>`（各 BorderType）、`<fill>`、`<content>`。相邻单元格线冲突时右下覆盖左上。空 `<border*/>` = `rgba(221,222,223,1)`/1px；空 `<fill/>` = 白。
- 单元格文字不随主题色；表头与斑马纹须在各行 `<content>` 上显式写 color 保证对比。

### chart（内联数据可视化，非 `.svg` 外链）
- 必填属性：`topLeftX` `topLeftY` `width` `height`；可选 `rotation` `flipX/flipY` `alpha`。
- 子元素：`<chartPlotArea>`(必需) `<chartData>`(必需) `<chartTitle>?` `<chartSubTitle>?` `<chartStyle>?` `<chartLegend>?` `<chartTooltip>?` `<reflection>?` `<shadow>?`。
- `<chartPlotArea>` = `<chartPlot type="…">`(必需) + `<chartAxes>?`。`chartPlot type`：line/area/bar/column/pie/radar/combo；其下可挂全局 `<chartPoints>/<chartLines>/<chartAreas>/<chartBars>/<chartLabels>`、`<chartSeriesList>`（含多个 `<chartSeries index="…">`）、`<chartExtra>`（`<chartRadar>`/`<chartSmooth>`/`<chartStep>`/`<chartStack>`）。样式优先级：单元素 > 系列 > 全局。
- `<chartAxes>` → 多个 `<chartAxis type="x|y|angle|radius">`（`position` 仅 y 轴，`max`/`min` 可选），子元素 `<chartTitle>` `<chartLabel>` `<chartAxisLine>`（空标签即显示轴线）`<chartGridLine>`。
- `<chartData>` = `<dim1>`（恰 1 个 `<chartField>` 作分类/X 轴）+ `<dim2>`（≥1 个 `<chartField>`，每个 = 一个系列）。`<chartField name="…" valueType="string|number">` 内容为**逗号分隔 CSV**（禁数组）；string 含逗号需双引号包裹。dim1 第 n 值对应 dim2 各系列第 n 值。
- 配色写 `<chartStyle><chartColorTheme><color value="rgb(...)"/>…`，按系列索引循环。示例：
```xml
<chart topLeftX="42" topLeftY="132" width="270" height="350">
  <chartPlotArea><chartPlot type="column"/><chartAxes>
    <chartAxis type="x"><chartLabel fontSize="9"/></chartAxis>
    <chartAxis type="y" position="left"><chartGridLine color="rgb(226,232,240)"/></chartAxis>
  </chartAxes></chartPlotArea>
  <chartData>
    <dim1><chartField name="季度">2024Q1,2024Q2,2024Q3,2024Q4</chartField></dim1>
    <dim2><chartField name="Apple">52,48,55,68</chartField><chartField name="Samsung">60,58,63,72</chartField></dim2>
  </chartData>
  <chartLegend position="bottom" fontSize="10"/>
  <chartStyle><chartColorTheme><color value="rgb(28,71,120)"/><color value="rgb(240,129,54)"/></chartColorTheme></chartStyle>
</chart>
```

### whiteboard / undefined
- `<whiteboard>`：必填 `topLeftX/topLeftY/width/height`；子元素二选一 `<mermaid>`（CDATA 包裹的 Mermaid 源码，适合流程/时序/思维/类/甘特/ER 图）或一个 SVG 命名空间元素（像素级自定义图形），可选 `<border>`。详见 whiteboard.md。
- `<undefined type="video|audio">` 仅用于承接导出时不支持的类型，生成时不主动使用。

## 文本容器与结构文本

### content
文本样式默认值的落点——属性被后代文本继承，`<span>` 只做片段级覆盖。所有样式属性均可省略。
- `textType`（默认 body）定语义层级；未显式写 `fontSize` 时按 `<theme>` 的 textStyles 取字号（默认主题 title 54 / headline 38 / sub-headline 32 / body 16 / caption 12）。`content` 自身 `fontSize`（6–400）无 XSD 默认，写了即覆盖。
- 其余样式属性（省略即用渲染端默认，**不取 `<theme>` 主题色**）：`fontFamily`（自由字符串、任意字体，如 思源黑体 / 思源宋体 / Inter）、`color`（**深底必须显式设浅色，否则文字可能不可见**）、`textAlign`（left/center/right/justify/dist；不写时 text 框靠左、其余居中）、`verticalAlign`（默认 middle，卡片正文常设 top）、`lineSpacing`（默认 `multiple:1.5`）、`letterSpacing`、`bold`/`italic`/`underline`/`strikethrough`、`backgroundColor`、`wrap`（默认 true）。
- **文本溢出（关键，与多数引擎不同）**：`autoFit` 默认 `no-auto-fit`——框装不下文字时**直接溢出、不做任何处理，没有默认缩排**。承载密集或突出文字的 `<content>` 必须显式写 `autoFit="normal-auto-fit"`（框内缩排字号防溢出）；`shape-auto-fit` 则让 shape 尺寸反过来随文字增大。
- 内边距 `paddingTop/Right/Bottom/Left`（0–1584，逐边独立覆盖）：`shape type="text"` 默认 0，其他 shape 默认 5，`<td>` 默认 8。
- 子元素仅 `<p>` `<ul>` `<ol>`；**禁止裸文本、禁止直接放页面元素**。

### p / ul / ol / li
- `<p>` 只接受流属性：`textAlign` `lineSpacing`/`beforeLineSpacing`/`afterLineSpacing`（`fixed:N`/`multiple:N`）`letterSpacing` `level`[1,10] `list`(bullet/number/none) `listStyle` `marginLeft` `indent`。外观（字号/色/粗斜）走内部 `<span>`。**所有文字必须包在 `<p>` 里**，单行也要。
- 段间距用相邻 `<p>` 或 `beforeLineSpacing`/`afterLineSpacing`；`<br/>` 只在一个 `<p>` 内换行（如 姓名`<br/>`职位），不可用于制造间距、不可用于段落分隔、首尾 `<br/>` 无效。
- `<ul listStyle?>`/`<ol listStyle?>` 只接受 `listStyle`（ul 默认 circle-hollow-square；ol 默认 number-lower-alpha-lower-roman、另有 hierarchical-number/circle-number/chinese-formal 等）。`<li>` 无样式属性，且**恰含一个 `<p>`**；`<ol>` 的 `<li>` 可带 `index`。项目符号自动生成，勿手写。

### 内联元素
- `<span>` 是片段样式覆盖点：`fontSize` `fontFamily` `color` `backgroundColor` `bold` `italic` `underline` `strikethrough` `baseline`（正=上标/负=下标）。
- `<strong>/<em>/<u>/<del>` 语义装饰；`<a href="…">`（仅 http/https/mailto 等）；`<shadow …>`/`<outline color width>` 可作用于文本片段。**禁用 Markdown**（`**粗**`、`$公式$` 均按字面渲染）。
- 保留空格用 `&#32;`、制表符用 `&#9;`；标签间空白与换行被忽略。

## 颜色与样式

- 纯色 `rgb(r,g,b)` 或 `rgba(r,g,b,a)`（r,g,b∈0–255，a∈0–1），均可带空格。hex 与具名色（white/red 等）一律不可用。
- 渐变：`linear-gradient(角度deg, 停靠点…)`、`radial-gradient(circle [at 50% 50%], …)`（圆心仅支持 `50% 50%`，其他值回退）、`rect-gradient(…)`、`shape-gradient(…)`。**每个停靠点必须是 `rgb()/rgba()` 颜色 + 整数百分比位置，至少两个停靠点**，否则整体回退为白色。示例：`linear-gradient(135deg, rgba(10,20,45,1) 0%, rgba(28,71,120,1) 100%)`。
- `<fill>` 三选一子元素：`<fillColor color rotateWithShape?>`（纯色或渐变）、`<fillImg src alpha? …>`、`<fillPattern type foregroundColor? backgroundColor? alpha?>`（图案，type 如 pct5/dot-grid/diag-cross…）。优先级 fillPattern > fillImg > fillColor。
- `<border>`：`color` `width`(整数 px) `dashArray`(solid/dash/dot/long-dash/round-dot/dash-dot/long-dash-dot/long-dash-dot-dot…) `compound`(single/double/thin-thick/thick-thin/three) `lineCap` `lineJoin`。
- `<shadow>`：`color`(默认 rgba(0,0,0,0.25)) `offset`[0,200](默认 15) `blur`[0,100](默认 35) `angle`[0,360)(默认 45) `hScale`/`vScale`[-2,2] `hSkew`/`vSkew`[-90,90] `align`(默认 top-left)。作用于 shape/img/line/polyline/chart/icon。
- `<reflection>`：`alpha` `offset`[0,200] `size`[0,1]。`<crop>` 见 img。

## 页面背景与备注

- 背景写在 `<slide>` 的 `<style><fill>…`（不继承任何全局主题；`<theme>` 与 outline 中的 color_palette 只是配色参考，不会自动套用）。省略 `<style>` = 白底。**不要**再在 `<data>` 里铺一个全屏 `<shape>` 模拟背景色。
- `<fillImg>` 做背景图时，压暗要叠一层深色半透明 `<shape>` 蒙版，而非降低图片 alpha（降 alpha 会露出白底、发灰）。
- `<note>` 演讲者备注：结构固定 `<note><content><p>纯文本</p></content></note>`，内部不支持任何富文本/列表/换行元素，也不能放裸文本。

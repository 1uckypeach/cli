# lark-slides 画板元素指南

`<whiteboard>` 放在 slide 的 `<data>` 内，内部直接放 SVG 或 Mermaid，用于绘制流程图、时序图、架构图、散点图、漏斗图、自定义图标、装饰图案等 `<chart>` 和 `<shape>` 难以覆盖的视觉内容。

> 前置条件：使用本文档前先阅读 [lark-slides SKILL.md](../SKILL.md) 和 [xml-schema-quick-ref.md](xml-schema-quick-ref.md)。

> 本文档描述的 `<whiteboard>` 写入行为已通过真实后端验证：`+create` 提交后返回成功，服务端应用日志确认画板 XML 被正确解析并写入存储。

## 职责边界

| 能力 | 核心职责 | 约束 |
|------|----------|------|
| `lark-slides` 内嵌 `<whiteboard>` | 在 slide 页面中创建 SVG / Mermaid 视觉元素；决定图表类型、位置、尺寸、层级和页面视觉融合 | 只服务当前 slide 页面，不是云文档中的独立画板对象；**当前线上回读不返回 token**（见下方说明） |
| `lark-whiteboard` | 查询/导出/编辑云文档里的独立画板对象 | 不用于 slide 内嵌 `<whiteboard>` 的创建；token 化编辑 slide 内画板依赖尚未全量的后端能力（见下方说明） |

如果目标是云文档里的独立画板对象，切到 `lark-whiteboard`。如果目标是在 PPT 页面里画流程图、架构图、装饰图案或特殊数据图，继续使用本文档。

> [!IMPORTANT]
> **token 化更新目前走不通，用内嵌一步到位写法。** slide 内嵌 `<whiteboard>` 服务端确实会创建真实画板实体（有内部 token），后端也具备"回读 XML 时在 `<whiteboard>` 上输出 `whiteboardToken` 属性"的能力，但该能力依赖的后端功能尚未全量部署上线。截至目前，任何回读（`+xml-get`、原始 `xml_presentations get`、带 `--params` 透传）都拿不到 token，返回的 `<whiteboard>` 只有位置属性和普通元素 `id`。在 token 回读实际生效之前：不要设计"先建空白 `<whiteboard>` 再拿 token 更新"的流程，SVG/Mermaid 内容必须随 `+create` / `+replace-slide` / `xml_presentation.slide create` 一次性内嵌提交。若某天回读 XML 里出现了 `whiteboardToken` 属性，说明该能力已上线，届时可把该 token 交给 lark-whiteboard skill 做后续编辑。

## 画板适用规则

在 slide 中，核心流程、系统架构、方案对比、风险链路、里程碑、指标趋势、因果归因、组织关系、能力分层等内容，如果图示能明显降低理解成本，可以规划为 `<whiteboard>`；结构简单或文字更清楚的内容不必强行画板化。

每个 `<whiteboard>` 都必须服务当前页的核心观点。不要把所有信息塞进一张大图；内容过密时拆成多页，或把图拆成多个聚焦区域。

### `<chart>` 还是 `<whiteboard>`？

先判断内容类型，再进入画板流程：

| 场景 | 推荐元素 |
|------|----------|
| 有结构化数据序列的柱/条/折线/面积/雷达/饼/组合图 | `<chart>`：原生渲染，支持 legend / tooltip / 系列配色 |
| 散点图、漏斗图、进度条、自定义时间线、甘特图 | `<whiteboard>` SVG |
| 流程图、时序图、架构图、类图、ER 图等拓扑图 | `<whiteboard>` Mermaid 或 SVG |
| 增长飞轮、战略地图、用户旅程图等自定义结构图 | `<whiteboard>` SVG |
| 自定义图标、徽标、示意性图形、波浪背景、点阵、装饰图案 | `<whiteboard>` SVG |

适合 `<chart>` 的内容就用 `<chart>`，不要用 SVG 手绘。原生图表通常更省力、更稳定。

## slide 与画板协同流程

### 步骤 1：识别画板机会

| 场景 | 入口 |
|------|------|
| 需要思维导图、时序图、类图、饼图、甘特图、ER 图 | 步骤 2A：使用 Mermaid |
| 需要散点图、漏斗图、自定义图形、装饰图案、精确版式、增长飞轮、战略地图、用户旅程图 | 步骤 2B：使用 SVG |
| 需要柱/条/折线/面积/雷达/饼等常规数据图 | 优先使用 `<chart>`，不进入本文档 |
| 只需要简单标签、边框、箭头、色块 | 优先使用 `<shape>` / `<line>`，不必创建 whiteboard |

> [!IMPORTANT]
> 分别对每个图表进行决策。一个 slide 可以同时使用 `<chart>`、`<shape>` 和 `<whiteboard>`，但每个元素都要有明确分工。

### 步骤 2A：使用 Mermaid 插入图表

```xml
<whiteboard topLeftX="72" topLeftY="90" width="816" height="360">
  <mermaid>
    <![CDATA[
flowchart LR
  A[输入] --> B[处理]
  B --> C[输出]
    ]]>
  </mermaid>
</whiteboard>
```

Mermaid 适合自动布局的拓扑关系。内容必须放在 `<![CDATA[...]]>` 内，避免 `[`、`>`、`-->` 等字符破坏 XML。

### 步骤 2B：使用 SVG 插入图表

```xml
<whiteboard topLeftX="520" topLeftY="120" width="340" height="260">
  <svg xmlns="http://www.w3.org/2000/svg">
    <rect x="24" y="40" width="72" height="180" rx="8" fill="rgba(59,130,246,0.85)"/>
    <text x="60" y="238" text-anchor="middle" font-size="13" fill="rgba(71,85,105,1)">A</text>
  </svg>
</whiteboard>
```

SVG 必须完整自包含：包含 `<svg>` 根节点和 `xmlns="http://www.w3.org/2000/svg"`，不引用外部图片、脚本或远程资源。

- `topLeftX` / `topLeftY` 是 slide 坐标系，默认页面宽 960、高 540。
- `width` / `height` 是 whiteboard 在 slide 上的容器尺寸。
- SVG 内部坐标相对于 whiteboard 自身左上角 `(0,0)`，与 slide 坐标系无关。
- XML 中元素越靠后，渲染层级越高。全屏或底层装饰 whiteboard 必须放在文字、图片、表格之前。

### 步骤 3：完成校验

- Mermaid：确认内容包在 CDATA 内，且 Mermaid 语法完整。
- SVG：确认内容是完整 `<svg ...>...</svg>`，且无不支持的装饰特性。
- 坐标：确认 `topLeftX + width <= 960`，`topLeftY + height <= 540`。
- **必须用 `slides +screenshot` 截图验证渲染效果**，不能仅凭提交成功或 XML 回读判断内容已生效——回读只返回位置属性，看不到 SVG/Mermaid 内容是否正确渲染，空白画板和渲染失败的画板在回读上和正常画板完全一样。
- 视觉：whiteboard 内容不会被后续元素遮挡，文字可读，图表和页面主题一致。

## 画板 SVG 设计指南

使用 SVG 插入画板时，最终交付是 slide 页面里的可渲染视觉元素。你写的 SVG 会被画板渲染端解析、缩放并放入 `topLeftX/topLeftY/width/height` 定义的区域。

**核心心智纠正：**

- 大多数 AI 如果只考虑"不报错"，最终会给出纯白底色加单层 `<rect>` 的方正卡片网格，这在正式 PPT 中是不及格的。
- SVG 给了足够的设计自由。可以使用图标路径、流畅连接线、层次化色块、面积填充、点阵、背景纹理和视觉隐喻，但必须控制在 slide 页面主题之内。
- 不需要所有元素都可编辑，但必须避免渲染端不支持的装饰特性，并兼顾稳定与美观。

### SVG 设计 Workflow

#### 1. 想清楚要画什么

- **核心信息是什么？** 能做到一图胜千言，就不要生成平平无奇的文字表格。
- **内容充实度**：用户描述稀疏时，可以用领域知识补足必要维度，但不要堆满页面。
- **视觉层级与隐喻**：自由判断形式，例如给重要节点加高亮背景，给对比项做左右对称，给流程增加方向性连接。
- **页面融合**：先看当前 slide 的背景、主色、字体、留白和其他元素，再决定 whiteboard 的颜色、透明度、边界和位置。

#### 2. 写 SVG

> [!IMPORTANT]
> 布局、配色、信息密度、装饰物都要主动判断。打破单调 `<rect>` 牢笼，但不要为了炫技牺牲可读性。

- 语言跟随用户 prompt；技术术语用行业通用写法，不机械翻译。
- 文字用 `<text>` / `<tspan>`，不要把文字转成 `<path>`。
- 文本容器宽度留够：CJK 约 `1em`，Latin 约 `0.6em`。
- 连线优先使用正交折线或带轻微曲线的 `<path>`，避免大量斜直线造成粗糙感。
- 可使用 `translate`、`rotate`、`scale`；避免 `skewX`、`skewY`、`matrix(...)`。
- 颜色优先使用 `rgba(R,G,B,A)`，并与 slide 的背景和强调色呼应。
- 深色背景下，低透明度装饰容易不可见；用明显亮度层级，而不是线性叠很浅的透明度。

#### 3. 计算坐标和尺寸

SVG 中只要涉及批量定位、等间距排布或数据映射，建议额外运行一个小脚本把坐标算出来再填入 SVG，而不是手动估值。适用范围不限于数据图表，装饰性点阵、重复图案同样适用。

```python
W, H = 360, 260
origin_x, origin_y = 50, 216
chart_w, chart_h = 290, 184

data, y_max = [120, 160, 90], 200
bar_w = int(chart_w / len(data) * 0.62)
for i, value in enumerate(data):
    cx = round(origin_x + (i + 0.5) * chart_w / len(data))
    y = round(origin_y - value / y_max * chart_h)
    print(f"bar-{i}: x={cx - bar_w//2} y={y} w={bar_w} h={round(origin_y - y)}")
```

所有元素坐标算完后，汇总出整体包围盒，作为 whiteboard 的 `width` / `height`。不要靠肉眼估算尺寸。

### 画板怎么处理 SVG

画板渲染端会把可识别元素转成可渲染节点；部分复杂特性可能降级或失败。为了稳定，优先使用下列元素：

- 形状：`<rect>` / `<circle>` / `<ellipse>` / `<polygon>`
- 连线：`<line>` / `<polyline>` / `<path>`（直线、折线、曲线）
- 文本：`<text>` / `<tspan>`
- 分组：`<g>` / `<use>` 引用 `<symbol>`
- 渐变：`<linearGradient>` 配合 `fill="url(#id)"`
- 变换：`translate` / `rotate` / `scale`

> [!IMPORTANT]
> 不支持或行为不可预测的装饰特性必须避免（`<pattern>` / `<clipPath>` / `<mask>` / 非阴影用途的 `<filter>` 等）。

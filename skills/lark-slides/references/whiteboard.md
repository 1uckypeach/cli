# Whiteboard 画板

`<whiteboard>` 放在 `<data>` 内，内部承载 SVG 或 Mermaid。本文只讲两种模式的语法与坑；「何时用 whiteboard vs chart vs shape」见 `cli-operations.md` 元素选型决策树。

## 定位

承载 `<chart>` / `<shape>` 覆盖不了的视觉：流程 / 时序 / 架构图、散点 / 漏斗 / 瀑布、自定义图标与示意、装饰图案（波浪背景、点阵、进度条）。
标准数据图（柱 / 条 / 折线 / 面积 / 雷达 / 饼 / 环 / 组合）用原生 `<chart>`，不要用 whiteboard 手绘重画——原生更省力、更稳、更易回读编辑。

## 选路：SVG 还是 Mermaid（按模型身份）

流程 / 时序 / 架构 / 类图 / ER 等拓扑图优先 Mermaid（自动布局）。散点 / 漏斗 / 进度条 / 波浪背景 / 星点纹理等需精确控坐标配色的场景，按模型身份定：

| 模型身份 | 路径 |
|---|---|
| Claude / Gemini / GPT / GLM | **SVG**，精确控制坐标、颜色、透明度 |
| Doubao / Seed / 其他 | **Mermaid** 近似表达；确实无法表达时才回退简单 SVG 矩形 / 线条 |

决定用 SVG 前先确认自己属于哪类，不要跳过这步。

## 公共属性

`topLeftX` / `topLeftY` / `width` / `height` 四个必填，均为 slide 坐标系（默认宽 960 高 540）。约束 `topLeftX+width ≤ 960`、`topLeftY+height ≤ 540`。
SVG 内坐标相对 whiteboard 自身左上角 (0,0)，与 slide 坐标系无关。

## 模式一：SVG

用于散点 / 漏斗等非原生数据图、自定义图标、装饰层、像素级可视化。

设计品质要求：不要只用矩形加文字应付（纯白底 + 方块 + 黑字＝不及格）；散点 / 漏斗等非原生数据视觉仍要有坐标轴、刻度、数值标注或分段说明，不要只画裸点 / 色块；字号要有层级（标题 ≠ 标签 ≠ 数值）；配色呼应 slide 主题（深底用透明底 / 深色卡片，浅底避免再加纯白块）。深色底（亮度 < 30%）装饰"对比不足"比"过强"危害更大，宁重勿轻；装饰层次用亮度跳跃而非线性叠透明度——`α=0.04→0.08→0.12` 在深底几乎不可见，改用 `0.10→0.40→0.70→1.0`、相邻层亮度差 ≥60。批量定位 / 等间距 / 数据映射先用脚本算坐标再填，别手估。

```xml
<whiteboard topLeftX="500" topLeftY="120" width="400" height="300">
  <svg xmlns="http://www.w3.org/2000/svg">
    <rect x="50" y="50" width="80" height="200" rx="4" fill="rgba(59,130,246,0.85)"/>
    <text x="90" y="270" text-anchor="middle" font-size="12" fill="rgba(100,116,139,1)">ABC</text>
  </svg>
</whiteboard>
```

- `<svg>` 必须声明 `xmlns="http://www.w3.org/2000/svg"`。
- **内容大小由所有子元素的几何包围盒（含 stroke 外扩）合并决定**，自适应缩放到容器；`<svg>` 上的 `width` / `height` / `viewBox` 不影响内容区域计算。
- **仅当元素属性用百分比值（如 `width="50%"`）时才需 `viewBox`** 提供计算基准；推荐统一用绝对坐标，避免百分比依赖。
- whiteboard 的 `width` / `height` 应等于该包围盒尺寸——批量定位 / 等间距 / 数据映射时先跑脚本算坐标与包围盒再填，别手估。

支持元素：`<rect>`（`rx` 圆角）、`<circle>`、`<ellipse>`、`<line>`、`<path>`（Q/C 曲线）、`<text>`（含中文，`y` 为 baseline，需 ≥ font-size 免被裁）、`<polygon>`、`<g>`、`<linearGradient>`（配 `fill="url(#id)"`）。
颜色统一用 `rgba(R,G,B,A)`；虚线 `stroke-dasharray="4,4"`；变换支持 `translate` / `rotate(deg cx cy)` / `scale`。

**不支持（渲染失败或降级，须避免）**：`<radialGradient>`（改 `<linearGradient>` 或 rgba 透明度）、`<filter>` 阴影 / 模糊（改半透明 `<rect>` 叠加）、`<clipPath>` / `<mask>`（调坐标尺寸自然裁切）、`<pattern>`（手铺点阵）、`skewX/skewY/matrix()`（改 rotate+translate）、`<image>` 外链 URL（先上传拿 file_token 用 `<img>`）。

**z-order**：whiteboard 在 XML 中越靠后渲染层级越高。全屏装饰 whiteboard 必须放在所有 `<shape>` / `<img>` / `<table>` 之前，否则遮挡内容。

## 模式二：Mermaid

用于拓扑图，自动布局、代码简洁。

```xml
<whiteboard topLeftX="72" topLeftY="60" width="816" height="360">
  <mermaid>
    <![CDATA[
      flowchart TD
        A[编写每页 slide XML] --> B[slides +create]
        B --> C[回读验证]
    ]]>
  </mermaid>
</whiteboard>
```

- **内容必须用 `<![CDATA[ ... ]]>` 包裹**：Mermaid 语法里的 `[`、`>`、`-->` 是 XML 特殊字符，不包会破坏解析。CDATA 结束符 `]]>` 不得出现在 Mermaid 代码本身。
- 图会自动撑满 whiteboard 区域，只需四个坐标属性。

常用图型与关键字：流程图 / 决策树 / 架构图 `flowchart TD`|`flowchart LR`、时序图 `sequenceDiagram`、类图 `classDiagram`、ER 图 `erDiagram`、状态图 `stateDiagram-v2`、甘特图 `gantt`、思维导图 `mindmap`、用户旅程 `journey`。
单图节点控制在 15 个以内，过密考虑分页；节点多时适当加 `height`（流程图 300-400、时序 320-420、思维导图 380-480）。

## 常见坑

- **CDATA**：Mermaid 忘包 CDATA → XML 解析失败；SVG 模式无需 CDATA。
- **转义**：SVG / Mermaid 里出现的 `&`、`<`、`>` 若非合法标签需注意，Mermaid 交给 CDATA 兜底。
- **尺寸**：whiteboard `width` / `height` 与子元素包围盒不匹配会拉伸内容或留白——按包围盒算，别手估；坐标推导建议留注释（originX/Y、chartW/H、映射公式）。
- 创建失败先排查是否偶发错误码，重试一次再判定。
- **提交前自检**：非原生数据图有轴 / 网格 / 数值（无裸点）；字号分层；单系列同色、多系列异色且对比足；轴标签不与元素遮挡。

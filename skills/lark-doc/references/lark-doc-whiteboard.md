# lark-doc 画板处理指南

> **前置条件：** 先阅读 [`../../lark-shared/SKILL.md`](../../lark-shared/SKILL.md) 了解认证、全局参数和安全规则。

## 两个 Skill 的职责边界

| Skill | 核心职责 | 约束 |
|---|---|---|
| `lark-doc` | 识别画板机会、决定文档内嵌画板用 Mermaid / SVG / blank 哪条路径、直接插入简单 Mermaid 或简单自包含 SVG、在需要时创建空白画板并把后续创作切给 `lark-whiteboard` | 主 Agent 负责文档入口的选型和简单插入；不要把所有图都默认外包给 SubAgent |
| `lark-whiteboard` | 查询 / 导出已有画板；处理复杂图表生成、DSL/场景选型、渲染验证；写入已有画板或空白画板 | 当图复杂、需要多轮打磨、需要 whiteboard 专属场景能力，或要更新已有画板内部内容时再切入 |

## 画板适用规则

写文档时，核心流程、系统架构、方案对比、风险链路、里程碑、指标趋势、因果归因、组织关系、能力分层等内容，如果图示能明显降低理解成本，可以规划为画板；结构简单或文字更清楚的内容不必强行画板化。

同一篇文档可以有多个画板。确有多个独立图示点时，可拆成多个聚焦画板，而不是把所有信息塞进一张大图。

## 文档与画板协同流程

### 步骤 1：识别画板机会

| 场景 | 入口 |
|---|---|
| 文档中需要思维导图、时序图、类图、饼图、甘特图，或用户明确给了 Mermaid 语法 | 优先看步骤 2A：使用 Mermaid 插入图表 |
| 文档中需要组织结构、架构关系、泳道、对比、分层、漏斗、金字塔、飞轮、路线图、信息卡片式示意等更依赖形状表达的图 | 优先看步骤 2B：按复杂度选择直接插入 SVG 或先建空白画板 |
| 已有画板需要更新内部内容 | 先 `docs +fetch` 获取 `board_token`，跳至步骤 3B |
| 只查看 / 下载已有画板 | 切换至 `lark-whiteboard`，不走本流程 |

> [!IMPORTANT]
> **每个图单独决策，不按整篇文档一刀切。**

先看图形语义，再看实现成本。不要把“哪种更好插入”当成第一判断标准：

- **结构即语义**的图，优先 Mermaid：例如思维导图、时序图、类图、饼图、甘特图，以及用户明确给出 Mermaid 文本的场景
- **形状即语义**的图，优先 SVG / blank + `lark-whiteboard`：例如组织架构、流程分层、架构拓扑、泳道、漏斗、金字塔、飞轮、路线图、对比卡片
- **已有画板内部内容修改**，不走文档重建，直接切 `lark-whiteboard`

### 步骤 1.5：选型表

| 图形语义 / 复杂度 | 推荐路径 | 说明 |
|---|---|---|
| Mermaid 原生强适配，且图不复杂 | 主 Agent 直接插入 `<whiteboard type="mermaid">` | 适合源码短、结构清晰、无需复杂视觉设计的图 |
| SVG 更合适，但图简单、源码可控、主流程上下文足够 | 主 Agent 直接插入 `<whiteboard type="svg">` | 适合简单对比图、轻量架构示意、少量节点关系图 |
| SVG 或 whiteboard scene 更合适，且图复杂、源码长、需要多轮修改或渲染校验 | 先插入 `<whiteboard type="blank"></whiteboard>`，再切 `lark-whiteboard` | 适合复杂流程、泳道、漏斗、金字塔、飞轮、大型架构图 |
| 已有画板内部内容更新 | 切 `lark-whiteboard` | 这是画板内容编辑，不是文档 block 替换 |
| 仅更换嵌入类型（mermaid / svg / blank 之间切换） | 用 `docs +update --command block_replace` 替换整个 `<whiteboard>` block | 这是文档层替换，不是画板内部编辑 |

### 步骤 2A：使用 Mermaid 插入图表

```xml
<whiteboard type="mermaid">
    mermaid 代码...
</whiteboard>
```

适用条件：

- 图本身更适合 Mermaid 表达
- Mermaid 文本较短，主流程可以稳定维护
- 不需要复杂视觉编排或多轮渲染打磨

### 步骤 2B：SVG 路径

当图形语义更适合 SVG 表达时，不要默认一定启动 SubAgent，先判断复杂度与上下文预算。

#### 2B-1：简单 SVG 由主 Agent 直接插入

```xml
<whiteboard type="svg">
    <svg ...>...</svg>
</whiteboard>
```

适用条件：

- 节点不多，结构清晰，SVG 源码规模可控
- 不需要 whiteboard 专属 scene、复杂布局推演或多轮修图
- 主流程保留该图上下文不会明显污染后续任务

#### 2B-2：复杂图先建空白画板，再切 `lark-whiteboard`

主 Agent 先在文档里插入：

```xml
<whiteboard type="blank"></whiteboard>
```

然后读取返回值里的 `block_token` / whiteboard token，切到 `lark-whiteboard` 完成写入。

适用条件：

- 图表复杂，源码长，或预期需要多轮修改
- 需要 `lark-whiteboard` 的 DSL / scene / 渲染校验能力
- 图对视觉质量要求高，主流程继续携带画图上下文成本过高

切给 `lark-whiteboard` 时携带最小上下文：

- doc token、插入位置（标题 / block_id / command）
- board_token
- 图表目标、受众、源段落或数据
- 推荐画板类型或推荐 scene（如果已经判断出来）

### 步骤 2C：直接插入 SVG 时的约束

- SVG 必须完整自包含：包含 `<svg>` 根节点和 `viewBox`，不引用外部图片、脚本、远程资源
- 直接插入 SVG 时仍需遵守下方的 [SVG 设计 Workflow] 与支持/不支持特性约束
- 如果插入后发现设计方向不对或需要大改，不要在文档层反复硬改；改走 2B-2 或切 `lark-whiteboard`

#### 画板 SVG 设计指南

使用 SVG 插入画板时，最终交付是**画板跨越重排渲染的节点**(你写 SVG → 画板解析)
**核心心智纠正 (重要)**：

- 大多数 AI 如果只考虑“绝对不报错/完美映射”, 最终给出的都是全篇纯白底色加单层 `<rect>` 的方正卡片网格, 极其死板单调, *
  *这将被视为不及格！**
- **SVG 给你了完全的设计自由**, 请大胆使用你脑内的图标路径 (`<path>`), 连接指引 (`流畅的 <path>`), 各种环境氛围点缀,
  大胆一点, 充分信任你的品味, 发挥出你的顶级艺术创造力！

##### SVG 设计 Workflow

###### 1. 想清楚要画什么

- **核心信息是什么？** 能做到一图胜千言, 绝对不要只生成平平无奇的文字表格, 要有设计感
- **内容充实度**：如果用户描述稀疏简略, 利用你的领域知识扩展, 保证信息维度和内容充实, 但不要过度堆砌, 淹没重点
- **视觉层级与隐喻**：这个没有固定的形式, 你自由判断, 比如: 给重要的节点加光环, 加高亮背景；给对比项设计天平或对称结构

###### 2. 写 SVG

> [!IMPORTANT]
> 布局, 配色, 信息密度, 装饰物——**全部由你判断**, 打破单调的 `<rect>` 牢笼, 严禁通篇用矩形和文字应付用户
> 操作边界约束：

- **语言跟随用户**：图表文字的语言与用户 prompt 保持一致, 技术术语用行业里通用的写法, 不机械翻译
- 文字用 `<text>`(不是 `<path>`), 容器宽度留够——画板按 CJK ≈ 1em / Latin ≈ 0.6em 重排
- 连线使用正交折线替代斜直线(`<polyline>` 带水平/垂直折点)视觉效果更好
- 可自由使用 `translate`, `rotate`, `scale`但请尽量避免使用 `skewX` / `skewY` / `matrix(...)` 发生空间级扭曲

###### 画板怎么处理 SVG

画板的 svg-parser 把可识别元素转成可编辑节点, 其余降级为内嵌图片(渲染没问题, 虽然不可编辑, 但是可以正常显示)；但
`<radialGradient>` / `<filter>` / `<clipPath>` 等装饰特性画板完全不支持，会导致渲染问题（见下方⚠️）
**不需要所有元素都可编辑, 但必须避免使用不支持的装饰特性, 且要兼顾可编辑和美观漂亮**

**可识别的元素**

- 形状：`<rect>` / `<circle>` / `<ellipse>` / `<polygon>`
- 连线：`<line>` / `<polyline>` / `<path>`(自动识别为直线 / 折线 / 曲线)
- 文本：`<text>` / `<tspan>` 画板硬编码 Noto Sans SC **文字必须用 `<text>`**
- 分组：`<g>` / `<a>` / `<use>` 引用 `<symbol>`
- 变换：`translate` / `rotate` / `scale` 正常；`skewX` / `skewY` / `matrix(...)` 降级

> [!IMPORTANT]
> ⚠️ ** 不支持的装饰特性**

- `<radialGradient>` / `<filter>` / `<pattern>` / `<clipPath>` / `<mask>` → 画板都不支持，**请避免使用，否则会导致画板渲染问题
  **

###### 3.插入后审查

插入画板后，可以从返回值使用 lark-cli 指令，将画板内容导出为 png
图片。若是对设计不满意，可以修改后，删除原来的画板再重新插入，或是调用 [
`../../lark-whiteboard/SKILL.md`](../../lark-whiteboard/SKILL.md) 编辑。

```bash
lark-cli whiteboard +query \
  --whiteboard-token "wbcnxxxxxxxx" \
  --output_as image \
  --output ./preview.png
```

### 步骤 3B：编辑已有画板或复杂空白画板

以下场景切 `lark-whiteboard`：

- 更新已有画板内部内容
- 已经插入 `<whiteboard type="blank"></whiteboard>`，需要继续填充
- 复杂图需要 `lark-whiteboard` 的 DSL / scene / 渲染校验能力

最小上下文：

- board_token
- 图表目标、推荐画板类型、受众
- 与图表直接相关的源段落或数据
- 要求读取 [`../../lark-whiteboard/SKILL.md`](../../lark-whiteboard/SKILL.md)，按其完整流程写入该 board_token

如果只是想把现有 `<whiteboard type="mermaid">` 换成 `<whiteboard type="svg">`，或从 `svg` 改成 `blank` 再重做，不走本步骤，回到文档层用 `docs +update --command block_replace` 替换整个画板 block。

### 步骤 4：完成校验

- Mermaid：确认插入的是 `<whiteboard type="mermaid">`，且 Mermaid 语法完整
- 直接插入 SVG：确认插入的是 `<whiteboard type="svg">`，且内容是完整 `<svg ...>...</svg>`
- blank 路径：确认空白画板后续已经由 `lark-whiteboard` 写入内容；只有占位空板视为任务未完成
- 已有画板更新：确认走的是 `lark-whiteboard`，不是误用 `docs +update` 直接改内部内容
- 类型切换：确认走的是文档层 `block_replace`，不是误当成画板内部编辑

---


---

## 关联参考

- 画板查询/创作/修改/渲染写入：[`../../lark-whiteboard/SKILL.md`](../../lark-whiteboard/SKILL.md)

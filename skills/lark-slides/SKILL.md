---
name: lark-slides
version: 1.0.0
description: "飞书幻灯片：创建和编辑幻灯片。创建演示文稿、读取幻灯片内容、管理幻灯片页面（创建、删除、读取、局部替换）。当用户需要创建或编辑幻灯片、读取或修改单个页面时使用。当用户给出 doubao.com 的 /slides/ URL/token 时，也应直接使用本 skill，不要因为域名不是飞书而回退到 WebFetch；路由依据是 URL 路径模式和 token，而不是域名。不负责：云文档内容编辑（走 lark-doc）、云文档里的独立画板对象（走 lark-whiteboard，注意 slide 内嵌的流程图/架构图仍属本 skill）、上传或下载普通文件（走 lark-drive）。"
metadata:
  requires:
    bins: [ "lark-cli" ]
  cliHelp: "lark-cli slides --help"
---

# slides (v1)

## Quick Reference

| 用户需求 | 指引 |
|----------|------|
| 读取 / 分析本地 PPTX 内容 | 文本用 `python -m markitdown presentation.pptx`；视觉总览用 `python3 scripts/thumbnail.py presentation.pptx`；原始 OOXML 用 `python3 scripts/office/unpack.py presentation.pptx unpacked/` |
| 从模板创建或编辑已有本地 PPTX | 先读 `lark-slides-pptx-template-workflows.md` |
| 从零新建飞书在线 PPT | 先读 `lark-slides-create-workflows.md` |
| 获取在线 slides 内容、读取 / 分析已有在线 PPT | XML 内容优先用 `slides +xml-get` 保存到文件；页面视觉内容用 `slides +screenshot`，详见 `lark-slides-screenshot.md` |

## 读取 / 分析内容

### 在线 Slides

```bash
# 读取完整 XML 内容，优先保存到文件再分析
lark-cli slides +xml-get --as user --presentation slides_example_presentation_id --output presentation.xml --json

# 获取页面截图；必须指定 --slide-number 或 --slide-id，多个页面可重复传 --slide-number
lark-cli slides +screenshot --as user --presentation slides_example_presentation_id --slide-number 1 --output-dir screenshots --json
```

在线 Slides 的截图参数和页码语义详见 [`lark-slides-screenshot.md`](references/lark-slides-screenshot.md)；需要继续编辑在线 Slides 时，按 `lark-slides-create-workflows.md` / `lark-slides-replace-workflows.md` 选择创建或替换流程。

## 编辑 PPTX 工作流

**完整流程先读 [`lark-slides-pptx-template-workflows.md`](references/lark-slides-pptx-template-workflows.md)。**

## 从零创建

**完整流程先读 [`lark-slides-create-workflows.md`](references/lark-slides-create-workflows.md)。**

当没有本地 PPTX 模板 / 参考演示文稿，或目标是新建飞书 / Lark 在线 Slides 而不是本地 `.pptx` 文件时，使用该流程。

## 核心概念

### URL 格式与 Token

| URL 格式 | 示例 | Token 类型 | 处理方式 |
|----------|------|-----------|----------|
| `/slides/` | `https://example.larkoffice.com/slides/xxxxxxxxxxxxx` | `xml_presentation_id` | URL 路径中的 token 直接作为 `xml_presentation_id` 使用 |
| `/wiki/` | `https://example.larkoffice.com/wiki/wikcnxxxxxxxxx` | `wiki_token` | ⚠️ **不能直接使用**，需要先查询获取真实的 `obj_token` |

> `+replace-slide` 和 `+media-upload` shortcut 会自动解析以上两种 URL；直接调用原生 API 时仍需手动解析 wiki 链接。

### Wiki 链接特殊处理（关键！）

知识库链接（`/wiki/TOKEN`）不能直接当 `xml_presentation_id`。直接调用原生 API 前，先查询 wiki 节点，确认 `node.obj_type == "slides"`，再用 `node.obj_token` 作为真实 presentation ID。

```bash
lark-cli wiki spaces get_node --as user --params '{"token":"wiki_token"}'
```

Shortcut `+replace-slide` 和 `+media-upload` 会自动解析 `/wiki/` URL；手动调用 `xml_presentations.*` / `xml_presentation.slide.*` 时才需要自己做这一步。

### 资源关系

```text
Wiki Space (知识空间)
└── Wiki Node (知识库节点, obj_type: slides)
    └── obj_token → xml_presentation_id

Slides (演示文稿)
├── xml_presentation_id (演示文稿唯一标识)
├── revision_id (版本号)
└── Slide (幻灯片页面)
    └── slide_id (页面唯一标识)
```

## 身份选择

飞书幻灯片通常是用户自己的内容资源。**默认应优先显式使用 `--as user`（用户身份）执行 slides 相关操作**，始终显式指定身份。

- **`--as user`（推荐）**：以当前登录用户身份创建、读取、管理演示文稿。执行前先完成用户授权：

```bash
lark-cli auth login --domain slides
```

- **`--as bot`**：仅在用户明确要求以应用身份操作，或需要让 bot 持有/创建资源时使用。使用 bot 身份时，要额外确认 bot 是否真的有目标演示文稿的访问权限。

**执行规则**：

1. 创建、读取、增删 slide、按用户给出的链接继续编辑已有 PPT，默认都先用 `--as user`。
2. 如果出现权限不足，先检查当前是否误用了 bot 身份；不要默认回退到 bot。
3. 只有在用户明确要求"用应用身份 / bot 身份操作"，或当前工作流就是 bot 创建资源后再做协作授权时，才切换到 `--as bot`。

## 设计思路

你要以顶级 PPT 设计专家的标准工作：先判断内容、受众、场景和叙事目标，再设计信息层级、视觉系统、版式、字体、配色和图文关系；不要只把内容排进页面，而要让每一页都像可交付的专业演示稿。

### 内容先行

- 每页只服务一个核心观点。内容页标题应写成带判断的结论句，而不是主题标签；读者只看标题就能知道这一页要证明什么。
- 先判断这一页要完成的任务：提出判断、证明判断、比较选项、解释流程、展示阶段、呈现数据、建立情绪，或完成过渡。页面结构必须服务这个任务。
- 受众和交付方式决定密度：演讲型 deck 更适合少字、强节奏、分步呈现；自读型 deck 必须在没有讲解和点击的情况下完整可读。
- 并列观点要互不重叠、没有明显缺口，通常控制在 3-5 个，最多不要超过 7 个；排序只选一种逻辑：时间、结构或重要性。
- 封面、章节页、内容页、结尾页承担不同任务。章节页只做过渡，不承载多点论证；短 deck 不要机械加入 agenda、Q&A 或多个收尾页。
- 数据页必须先写清 takeaway，再选择图表或数字呈现方式。不要把图表标题写成指标名，而要写成图表证明的结论。

### Visual Planning：从 `slide_plan` 到 XML 几何

新建演示文稿或大幅改写页面时，在 `slide_plan.json` 完成后、生成 XML 前，必须把 `layout_type`、`visual_focus`、`text_density` 落到真实页面几何里。默认按 `960 x 540` 画布规划；已有页面回读 XML 可以影响坐标，但不能覆盖这些原则：页面要有主视觉区域，文本要受密度约束，不同 `layout_type` 必须产生明显不同的坐标结构。

- `layout_type` 必须改变几何：元素位置、区域大小、对齐方式和视觉节奏都要随页面类型变化。去掉 plan 里的 `layout_type` 标签后，页面仍应能被辨认为对应结构。
- `visual_focus` 决定页面最大或最高对比区域，可以是图片、图表、指标、引语、表格、diagram，或与 `asset_need` 匹配的 shape placeholder。
- `text_density` 限制可见文本量：`low` 只放标题 + 一句短陈述，或 1-3 个标签；`medium` 放标题 + 2-4 个短 bullet 或标注区域；`high` 用表格、分栏、分组标签或 annotation，不能退化成一个长 bullet 框。
- 4 页及以上的 deck，在内容允许时至少使用 4 种不同布局结构；不要让所有内容页都是标题 + bullets。
- 标准内容页外边距通常保持 `60-80` px。标题区一般是 `y=36..90`，主内容通常从 `y>=110` 开始；非背景内容不要挤到 `y>500`，除非它是页脚。
- 优先使用更少、更大的对象，而不是许多细碎文本框。文字适配是布局约束，不是事后清理：文本框太小时，先删减、拆分或增加空间，再生成 XML。
- 普通内容页默认复用同一个基础背景；封面、章节、强调和结论页可以变化，但必须共享主色、母题、边缘处理、字体或几何语言。背景和母题形状要先于内容元素插入，避免盖住文字、图片或 diagram。

文字框高度按保守下限规划，中文、加粗、中英混排或较大行距都要增加高度：

| 文本用途 | 常见字号 | 最小高度 |
|----------|----------|----------|
| Caption，1 行 | 10-12 | 18 |
| Caption，2 行 | 10-12 | 30 |
| Body，1 行 | 13-16 | 24 |
| Body，2 行 | 13-16 | 40 |
| Body，2 行加粗 | 15-18 | 48 |
| Headline，1 行 | 24-32 | 42 |
| Title，2 行 | 34-44 | 110 |

- 不要把长中文句子或长英文短语放进 `height=18` 或 `height=22` 的文本框；这些高度只适合短标签。
- 页脚和来源通常保持一行。若必须换行，应上移成真实 caption block，而不是挤在页脚区。
- 底部结论条一行强调至少 `40` px 高，两行至少 `54` px 高。
- 多个 `<p>` 的文本框必须按多行显式给高度，不要假设渲染器会自动撑开；中英混排要预留更宽空间。

常用 `layout_type` 的几何约束：

| `layout_type` | 几何要求 | 文本要求 |
|---|---|---|
| `title-cover` | 使用主标题块，常见 `x=70..120`、`y=150..250`、`width=700..820`；可用全幅背景、侧图、accent band 或抽象母题。若有右侧 diagram / 截图 / 母题簇，必须做明确 split composition，让标题区和视觉区分离。 | 只放一条 subtitle 或 context，默认 `low`，不要做 bullet list。 |
| `section-divider` | 使用大号章节编号、chapter label、居中 claim、竖向 accent bar 或全宽 band；页面保持稀疏。 | 标题 + 一句短语，不放 bullets。 |
| `two-column` | 主区域拆成两个均衡列，例如左 `x=60,width=400`、右 `x=500,width=400`；每列有自己的 heading 或视觉锚点。 | `medium` 每列 2-3 个短项；`high` 用 grouped rows 或 mini table。 |
| `image-left-text-right` | 左侧视觉区约占宽度 `35-45%`，可全高或高裁切；右侧文本通常从 `x=420` 左右开始。密集截图、论文图、产品图可扩大到 `50-65%` 宽或至少 `320` px 高。 | 右侧只放强 headline + 短支持，最多 4 bullets；截图页优先用 2-3 个解读卡片或 callout。 |
| `image-right-text-left` | 左侧文本从 `x=60..90` 开始，宽 `400..460`；右侧视觉区约占 `35-45%` 宽，并与正文块对齐。密集素材优先放大视觉区、减少文本。 | 一条主 claim + 2-3 个支持点；callout 要短且并行。 |
| `big-number` | 最大对象留给指标，字号常见 `64-110`，区域至少 `300 x 120`；不要把数字埋进 bullet 或小卡片。 | `low` 或 `medium`；补充信息用小标签、legend、mini-card，不与数字抢焦点。 |
| `timeline` | 3-6 个 milestone 沿水平或垂直 spine 排列，每个节点有 dot / card / date，并由线或箭头连接；标题与序列分离。 | 每个节点短标签 + 可选一行说明，不写段落。 |
| `comparison` | 使用 2-3 个 panel、column 或 table-like 结构，heading 对齐；用颜色、边框、图标或标签突出首选项或关键差异。 | 各列措辞平行，避免长短严重不均。 |
| `architecture-diagram` | 主区域必须是组件、依赖或系统流图；优先用 `<whiteboard>`，fallback 才用 `<shape>` + `<line>`。 | 节点标签 1-5 个词，最多一个短说明块；两行标签要给足节点和文本框高度。 |
| `process-flow` | 用 3-5 个编号步骤 + 明显方向箭头 / 连线；步骤更多时分组为阶段。优先用 `<whiteboard>`。 | 每步用动词开头标签 + 最多一个短描述；长解释移到侧注或 notes。 |
| `quote-highlight` | 引语或 claim 是最大文本对象，留大量空白，可加 attribution 或 context badge。 | 一个 statement / quote + 可选 attribution，不放 bullets。 |
| `conclusion` | 用一个主 closing statement 或 call to action，可加最多 3 个 next-step card / checklist / owner-date label；可呼应封面背景。 | 结尾要易记，不做 recap overload。 |

截图、论文图、真实图表和产品截屏必须按页面角色使用：

- 只有在素材可读时才作为视觉焦点；过密时裁到相关区域、做 zoom detail，或用原生 shape 重画核心信息。
- 不要把截图缩成装饰 thumbnail 后再包围密集正文。配 2-3 个解释性标注，告诉读者该看哪里。
- 外部或论文来源的视觉必须有短 source caption。
- 最终 XML 必须包含支持的 image token 或创建时本地 placeholder，不能留下不支持的外链。

生成每页 XML 前逐页检查：主视觉是否最大或最突出；几何是否匹配 `layout_type`；`text_density` 是否限制了段落、bullet、标签和文本框数量；背景策略是否和 deck 级视觉系统一致；所有文本框高度是否足够；截图或论文图是否足够大且有简短解读。创建后回读时继续检查：页面是否拥挤、是否依赖长 bullet 框、主 claim / 支撑细节 / 主视觉是否有清晰层级，静态 XML 是否存在短框长文、多段高度不足、页脚换行、标签压线等 text-fit 风险。

### Asset Planning：轻量资产规划

新建演示文稿或大幅改写页面时，在写入 `slide_plan.json` 前后都可以规划 `asset_need`，让 agent 主动识别有价值的图、图标、图表、流程图、时序图、架构图、装饰图案、截图或示意图需求。`asset_need` 只定义轻量资产规划，不是素材采集流程。

- `asset_need` 只是元数据，可以指导页面设计，但不能要求 web search、本地下载、媒体上传或外部工具。`suggested_query` 只是未来查找提示，除非用户另行要求真实素材，否则不要执行搜索。
- 每个 planned asset 都必须有 `fallback_if_missing`，确保没有真实素材时也能用 XML shapes、文本、箭头、表格、简单图表、`<whiteboard>` diagram 或 placeholder region 完整生成页面。
- 资产需求必须服务该页 `key_message` 和 `visual_focus`；不要为装饰而加素材。优先规划少数高价值资产，而不是每页机械放一个 asset。6 页左右的技术或商业 deck，在内容允许时至少 3 页有 meaningful asset plan。
- 如果真实本地素材已经存在，或用户明确提供了素材，可以走正常 media-upload / image workflow，但 plan 里仍要保留 `fallback_if_missing`。
- 最终 XML 不能留下空白图片框。真实素材缺失时，立即渲染 fallback，并让 fallback 满足 `visual_focus`，成为真实页面元素，而不是小装饰。

`asset_need` 可以是单个对象；一页确实需要多个资产时才用数组。对象保持紧凑：

```json
{
  "asset_type": "architecture_diagram",
  "purpose": "Show how API gateway, planner, XML generator, and Slides API interact.",
  "suggested_query": "agent native slides runtime architecture diagram",
  "fallback_if_missing": "Draw grouped boxes connected by arrows with short labels."
}
```

没有有意义资产需求的页面，显式写 `none`：

```json
{
  "asset_type": "none",
  "purpose": "No external or simulated asset needed; the page is text-led.",
  "suggested_query": "",
  "fallback_if_missing": "Use typography, spacing, and simple accent shapes only."
}
```

支持的 `asset_type`：`paper_figure`、`architecture_diagram`、`icon`、`logo`、`chart`、`infographic`、`screenshot`、`flow_diagram`、`none`。不要随意发明新类型；接近这些类型时选择最接近的一种，并在 `purpose` 里说明细节。`<chart>` 不支持 funnel 或 scatter，这类需求生成时映射到 `<whiteboard>` SVG。

资产类型要匹配页面角色：

- `architecture-diagram` 通常配 `architecture_diagram` 或 `flow_diagram`。
- `process-flow` 通常配 `flow_diagram`、`icon` 或 `infographic`。
- `comparison` 通常配 `icon`、`chart` 或 `infographic`。
- `timeline` 通常配 `icon`、`chart` 或 shape-based milestone markers。
- `big-number` 只有在资产支持指标表达时才配 `chart` 或 `infographic`。
- `image-left-text-right` / `image-right-text-left` 可以配 `screenshot`、`paper_figure`、`logo` 或 `infographic`；缺失时使用大型 placeholder diagram 或 stylized panel。

`fallback_if_missing` 必须具体到可直接生成 XML，例如：简化 attention matrix、client -> gateway -> service 的三组盒子与箭头、4 根柱子的 mini bar chart、带产品区域标签的 bordered placeholder panel。不要写“Use a placeholder”“Find another image”“Leave blank if unavailable”或“Use generic decoration”。

生成 XML 时按这个顺序执行：真实素材存在且流程支持时，放入计划的主视觉区域；否则立刻用 XML-native fallback 渲染；fallback 的尺寸和位置必须满足 `visual_focus`；不要用更长 bullet 文本补偿缺失素材；创建后回读并确认资产页不空白，且缺失素材时 planned fallback 可见。

### 视觉系统

- 先根据内容主题、行业语境、受众和交付方式推导视觉方向，再确定配色、字体、图形语言和页面密度；不要套用通用高饱和模板色，也不要让用户在抽象风格词里做选择。
- 样本中更稳定的高级感来自“中性底色 + 少量强调色”，而不是全页高饱和。优先使用白、浅灰、深灰、蓝灰、炭黑作为基底，再用绿色、红色、金色、橙色、蓝色等小面积强调关键数字、结论或状态。
- 同一份 deck 要锁定一套视觉系统，并贯穿所有页面：主色、背景、正文颜色、强调色、标题处理、留白密度、图标/图形风格都要稳定。
- 生成 XML 前先写出 deck 级颜色令牌并明确分工：`primary` 承担品牌/结构，`background` / `background_alt` 承担页面基底，`text_primary` / `text_body` / `muted` 保证阅读层级，`accent` 只用于关键数字、结论、风险、增长或行动点；后续页面复用这些令牌，不要每页临时发明新颜色。
- 普通内容页默认使用同一明暗基调。封面、章节页、强调页可以改变背景，但必须保留同一主色、边栏、纹理、图形母题或标题处理方式。
- 每页都必须显式写 `<style><fill>...</fill></style>`。背景策略要克制：纯色、渐变或图片三选一作为主背景，不要叠多层全页色块。
- 深色、发光或科技感页面，应使用平整深色背景 + 局部发光元素，而不是半透明大渐变把页面洗白。
- 正文和注释不要使用高饱和色；高饱和或高亮度颜色只作为小面积 `accent`，避免用于大段文字、大面积背景或整页高饱和渐变。
- 所有文字、图标、线条和图表都必须与背景保持清晰可读的对比；不要把“对比充足”做成刺眼的纯黑/纯白/荧光色组合。弱化信息可以降低饱和度或透明度，但不能牺牲可读性。
- 图片或渐变背景上放文字时，给文字区域加半透明遮罩、色块或足够留白；不要把正文直接压在复杂背景上。
- 渐变色必须使用 `rgba()` 格式并带百分比停靠点，例如 `linear-gradient(135deg,rgba(15,23,42,1) 0%,rgba(56,97,140,1) 100%)`；使用 `rgb()` 或省略停靠点可能导致服务端回退为白底。
- 验证时逐页检查：背景是否显式填充且未无意变白，正文颜色是否可读，`accent` 是否过量，渐变是否可能回退。

### 字体与字号

- 标题字体可以有性格，正文字体必须清晰耐读；不要整份 deck 都默认 Arial。
- 当前样本中中文正文最稳定的是现代无衬线系统，尤其接近 `Noto Sans SC` / 思源黑体方向；编辑感或文化感页面会使用 `Noto Serif SC` / 思源宋体方向，但不应全篇滥用。
- 中英文混排时，字体族先写英文/拉丁字体，再写中文/CJK 字体，最后写通用 fallback；标题和正文各用一套稳定组合。
- 字体选择要匹配视觉系统的类别和处理方式：衬线、无衬线、圆体、等宽、粗窄标题、全大写等风格不要随意互换。
- 常用标题方向：`Playfair Display` / `EB Garamond` / `Lora` 适合编辑感和高级感；`Anton` / `Bebas Neue` / `Oswald` 适合强冲击标题；`DM Sans` / `Montserrat` / `Poppins` 适合现代产品和商业正文。
- 常用中文方向：`思源宋体` 适合长文和编辑感；`思源黑体` / `黑体` / `Noto Sans SC` 适合中性现代；`寒蝉德黑体` 适合工业和科技；`寒蝉全圆体` / `资源圆体` 适合温暖亲和；书法类字体只用于少量标题。
- 字号要形成清晰层级。标题、分区标题、正文、注释、Hero number 至少要有 3 个可见层级；不要让标题、标签和正文看起来几乎一样大。

| 元素 | 建议字号 |
|------|----------|
| 封面标题 | 40-56px；纯标题页可到 64-96px |
| 内容页标题 | 28-40px |
| 副标题 / 分区标题 | 20-26px |
| 正文一级 | 16-20px |
| 正文二级 | 13-16px |
| 注释 / 来源 | 11-13px |
| Hero number | 80-140px |

- 不要为了填满空页面而盲目放大字体；页面显得空时，优先补充有意义的信息、调整构图、强化边缘对齐或增加视觉锚点。
- 标题文本框要预留足够宽度，避免本应单行的标题意外换行。正文过长时先删减内容，再考虑缩小字号。

### 布局

- 先判断内容关系，再设计版式。比较、流程、时间线、循环、层级、矩阵、漏斗、整体-部分、因果等关系，应通过位置、对齐、分组、比例和流向直接表达。
- 高质量样本里最常见的稳定结构是“顶部结论标题 + 下方内容区”，以及“左文右图 / 左图右文”的双区布局。生成页面时优先明确标题区、证据区、解释区，而不是随手摆放元素。
- 版式本身要承载逻辑：比较用并列和基线对齐，流程用方向和连接，层级用尺度和嵌套，矩阵用坐标和象限，因果用箭头和阅读顺序。
- 每页都应围绕该页内容重新组织，不要从固定模板里盖章；同一 deck 可以复用视觉母题，但不要所有页面都是标题 + 三 bullets。
- 页面要留呼吸感。内容块之间保持稳定间距，卡片内边距要真实存在；文字不要贴边，也不要被装饰线、图片或页脚挤压。
- 相关元素之间距离更近，不相关元素之间距离更远。不要把所有间距做成平均值；间距本身就是分组和层级。
- 并列内容尽量使用同宽、同高、同内边距的模块。卡片、列、表格、小图、多指标块必须共享网格和 slot 顺序，否则读者无法快速比较。
- 正文默认左对齐；只有封面、章节页、结尾页、大号数字或少量仪式感页面适合居中。

### 视觉元素与图表

- 视觉元素必须承载意义或引导注意力，不做填空装饰。图片、图标、图表、表格、色块、连线都要解释内容关系、强调重点或改善阅读节奏。
- 每个内容页至少应有一个非纯文本视觉锚点：图片、图标、图表、表格、流程、对比结构、大号数字、示意图或抽象 shape 组合。
- 卡片只用于真正并列的内容。每张卡片应有相同的内部结构：图标/编号、标题、短说明、关键数字或状态；不要把不具备并列关系的段落硬塞进卡片。
- 流程和时间线使用重复节点 + 克制连线。连线应表达顺序、依赖或阶段，不要穿过文字，不要用复杂箭头装饰页面。
- 信息图、截图、图表等素材要保持原始比例；不要为了塞进版面强行裁切或拉伸。装饰性照片可以更自由，但仍要服务主题和构图。
- 有真实数据序列时，先写清图表要证明的 takeaway，再选择图表类型；一张图只表达一个核心结论。单个数字或两项简单对比，优先用大号数字 callout，不必硬画图。
- 数据页可以采用“Hero number + 解释 + 图表证据”的结构。大数字必须配单位、时间范围、比较对象或解释句，不能只放一个孤立数字。
- 饼图 / 环图只适合表达明确的整体构成；不确定时优先使用排序条形图。多系列数据要控制数量，必要时合并为 Other。
- 封面和结尾页的图片不要自带文字；文字应由 slide 渲染，避免图片中文字不可控、不可编辑或与语言风格冲突。

### 动效

- 动效服务节奏和注意力，不做炫技。只有在逐步解释、引导关键元素、展示流程 / 时间 / 变化时才使用。
- 演讲型 deck 可以用少量 build 控制听众视线；自读型、正式汇报型、董事会/咨询风格 deck 应尽量静态，最多使用统一的页面转场。
- 封面、章节页和结尾页默认静态。单页动效不超过 3 个 build，且同页尽量只使用一种效果；如果需要更多步骤，优先拆页。
- 动效要让观众注意内容出现，而不是注意效果本身。优先使用淡入、出现、轻微上浮、擦入；避免旋转、弹跳、闪烁、远距离飞入等抢戏效果。

### 基于模板或已有 PPT 编辑

- 如果用户要求继续编辑、补页或修改已有 PPT，默认保留原页面内容、结构、字体、配色和视觉资产，只改用户要求的部分。
- 除非用户明确要求重做，不要擅自美化、重排、加封面、换背景或从零复刻。
- 如果用户把上传文件作为“参考风格”而不是“继续编辑原文件”，才可以抽取其视觉语言后重新创作。

### 避免事项

- 不要让版式先于内容；先判断这一页的逻辑关系，再决定几何结构。
- 不要创建纯文本页；plain title + bullets 只能作为草稿，不是正式交付。
- 不要只设计一页，其余页面保持 plain；视觉系统必须全篇贯彻，或者全篇保持有意克制。
- 不要让普通内容页在深色、浅色、图片背景之间来回切换；背景明暗变化必须服务封面、章节、强调或总结等明确页面角色。
- 不要混用太多字体、字号、圆角、阴影和强调色；变化必须有层级意义。
- 不要用高饱和色填满背景、正文或大面积卡片；更稳定的做法是中性基底、清晰文字和小面积 accent。
- 不要用图表承载多个结论，也不要因为有数字就机械画图。
- 不要在标题下方画装饰强调线作为默认设计手法；优先用空间关系、局部低饱和色块、尺度、分区和对齐建立层级。
- 不要把并列卡片、比较列、时间节点做成不同尺寸、不同内边距、不同标题层级；除非差异本身就是要表达的信息。

## PPT 主题配色体系

本章节用于根据不同 PPT 内容分类，选择匹配的视觉主题与配色方案。配色设计遵循三个原则：

1. 内容气质优先：颜色服务于汇报场景，而不是单纯追求好看。
2. 层级清晰：每套配色都区分背景色、正文色、主色、辅助色和强调色。
3. 克制高级：大面积颜色低饱和，关键结论和数据才使用高识别度强调色。

### 配色使用规则

| 色槽 | 用途 |
|---|---|
| 背景色 | 页面底色、大面积留白区域、浅色模块背景 |
| 正文色 | 正文、说明文字、页脚、图表标签 |
| 主色 | 封面、章节页、标题、核心模块 |
| 辅助色 | 图表、分组、信息卡片、二级视觉层级 |
| 强调色 | 关键数字、结论标签、重点标注、行动按钮 |

建议使用比例：

| 类型 | 比例 | 说明 |
|---|---:|---|
| 背景色 | 70% | 保持页面干净、可读 |
| 主色/辅助色 | 20% | 建立主题识别和结构层级 |
| 强调色 | 10% | 只用于最关键的信息 |

---

### 一级分类主题配色表

| 一级分类 | 推荐主题名 | 背景色 | 正文色 | 主色 | 辅助色 | 强调色 | 设计气质 |
|---|---|---:|---:|---:|---:|---:|---|
| 战略决策 | Midnight Capital | `#F6F7F9` | `#1B2430` | `#102A43` | `#486581` | `#C89B3C` | 专业、克制、资本感 |
| 教学/科研 | Sage Lab | `#F7FAF8` | `#253B36` | `#2F6F68` | `#9DBFAD` | `#E6B450` | 清晰、可信、学术感 |
| 工作总结与经验分享 | Slate System | `#F5F6F8` | `#1F2937` | `#3E4C59` | `#8A98A8` | `#2FA876` | 稳健、复盘、结构化 |
| 商业增长 | Signal Growth | `#FFF7F1` | `#2B2B2B` | `#D94A2B` | `#F08A3C` | `#2563EB` | 活力、转化、增长感 |
| 技术研发 | Electric Blueprint | `#F5F7FF` | `#172033` | `#273469` | `#4C63B6` | `#00A7B5` | 理性、工程、科技感 |
| 职能管理 | Civic Order | `#F7F8F5` | `#24312F` | `#245B4F` | `#7D9188` | `#B9972F` | 规范、秩序、组织感 |
| 个人提升与表达 | Soft Presence | `#FFF8F4` | `#2D2A28` | `#E76F51` | `#F2B8A2` | `#2A9D8F` | 亲和、成长、表达感 |

---

### 二级场景配色建议

| 场景 | 推荐主题 | 配色说明 |
|---|---|---|
| 投资分析、投资报告、行业研究报告 | Midnight Capital | 使用深蓝建立专业感，铜金用于关键结论和核心数字 |
| 咨询顾问、战略宣讲 | Midnight Capital | 适合逻辑严密、判断明确的商业分析型页面 |
| 融资 Pitch Deck、商业计划书、项目路演 | Midnight Capital / Signal Growth | 若偏资本叙事用深蓝金，若偏增长故事用橙红蓝 |
| 课程设计、教学课件 | Sage Lab | 青绿色降低阅读压力，适合长时间观看 |
| 论文阐述、开题/答辩报告 | Sage Lab / Electric Blueprint | 学术型用青绿，技术型用靛蓝电青 |
| 研究方向调研 | Sage Lab | 保持理性和可信，不宜过度商业化 |
| 培训课件、述职晋升 | Slate System | 灰蓝体系更适合正式职场表达 |
| 项目复盘、项目结项、阶段汇报 | Slate System | 用绿色强调进展、成果和正向结论 |
| 工作总结、日报、周报、月报、年终总结 | Slate System | 稳定、清晰、适合高频汇报 |
| 产品介绍、品牌宣传 | Signal Growth | 暖色建立吸引力，蓝色用于建立可信度 |
| 招商加盟方案、增长方案提报 | Signal Growth | 强调行动、机会、转化和商业结果 |
| 创意提案 | Signal Growth | 可提高强调色使用比例，增强视觉记忆点 |
| 账号运营汇报 | Signal Growth / Slate System | 增长结果用橙红蓝，常规复盘用灰蓝绿 |
| 市场分析报告、数据分析报告 | Midnight Capital / Slate System | 战略判断用深蓝金，过程分析用灰蓝绿 |
| 功能说明、技术可行性分析报告 | Electric Blueprint | 靛蓝和电青适合表达技术结构与方案可信度 |
| 设计提案 | Electric Blueprint / Soft Presence | 产品设计用科技蓝，体验表达可用柔和暖色 |
| 政策解读、制度宣贯 | Civic Order | 墨绿和旧金显得庄重、规范 |
| 党建工作总结 | Civic Order | 可适当加入深红作为局部强调，但不建议大面积使用 |
| 新员工入职培训 | Civic Order / Slate System | 制度流程用墨绿，通用培训用灰蓝 |
| 自我介绍、生活学习分享 | Soft Presence | 亲和、不压迫，适合轻表达场景 |
| 调研总结 | Soft Presence / Slate System | 偏个人观察用暖色，偏工作结论用灰蓝 |

---

### 单页应用建议

#### 封面页

优先使用主色作为大标题或视觉主背景，强调色只用于副标题、关键词或日期。

示例：

- 战略决策：深蓝背景 + 铜金关键词
- 教学科研：浅绿背景 + 深青标题
- 商业增长：暖白背景 + 橙红标题 + 蓝色重点词

#### 目录页

使用主色标记章节编号，辅助色用于分隔线或图标，不建议使用过多强调色。

#### 数据页

正文和坐标轴使用正文色，主数据系列使用主色，重点数据使用强调色。

#### 结论页

可以适当提高主色和强调色占比，用于强化最终判断。

---

### 不建议的配色方式

| 问题 | 原因 |
|---|---|
| 大面积使用高饱和红、橙、紫 | 容易显得廉价，且阅读压力高 |
| 一页中同时出现 5 个以上高存在感颜色 | 视觉焦点分散，削弱专业感 |
| 浅黄色、浅绿色直接承载正文 | 对比度不足，可读性差 |
| 全套 PPT 只有同一色相的深浅变化 | 容易单调，缺少设计记忆点 |
| 每页都使用强调色 | 强调色会失效，重点不再突出 |

---

### 推荐默认选择

如果无法判断具体场景，优先使用以下默认方案：

| 场景类型 | 默认主题 |
|---|---|
| 商业判断类 | Midnight Capital |
| 学术教学类 | Sage Lab |
| 工作汇报类 | Slate System |
| 增长营销类 | Signal Growth |
| 技术产品类 | Electric Blueprint |
| 管理制度类 | Civic Order |
| 个人表达类 | Soft Presence |

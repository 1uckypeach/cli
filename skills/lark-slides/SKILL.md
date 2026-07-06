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

### 内容先行

- 每页只服务一个核心观点。内容页标题应写成带判断的结论句，而不是主题标签；读者只看标题就能知道这一页要证明什么。
- 受众和交付方式决定密度：演讲型 deck 更适合少字、强节奏、分步呈现；自读型 deck 必须在没有讲解和点击的情况下完整可读。
- 并列观点要互不重叠、没有明显缺口，通常控制在 3-5 个，最多不要超过 7 个；排序只选一种逻辑：时间、结构或重要性。
- 封面、章节页、内容页、结尾页承担不同任务。章节页只做过渡，不承载多点论证；短 deck 不要机械加入 agenda、Q&A 或多个收尾页。

### 视觉系统

- 先根据内容主题、行业语境、受众和交付方式推导视觉方向，再确定匹配主题情绪的配色、字体、图形语言和页面密度；不要套用通用高饱和模板色，也不要让用户在一堆抽象风格词里做选择。
- 同一份 deck 要锁定一套视觉系统，并贯穿所有页面：主色、背景、正文颜色、强调色、标题处理、留白密度、图标/图形风格都要稳定。
- 生成 XML 前先写出 deck 级颜色令牌并明确分工：`primary` 承担品牌/结构，`background` / `background_alt` 承担页面基底，`text_primary` / `text_body` / `muted` 保证阅读层级，`accent` 只用于关键数字、结论或行动点；后续页面复用这些令牌，不要每页临时发明新颜色。
- 每页都必须显式写 `<style><fill>...</fill></style>`。普通内容页默认复用同一明暗基调和底色体系；只有封面、章节页、强调页可以有意改变背景，并且必须保留同一主色、边栏、纹理或视觉母题。
- 背景策略要克制：纯色、渐变或图片三选一作为主背景，不要叠多层全页色块。深色、发光或科技感页面，应使用平整深色背景 + 局部发光元素，而不是半透明大渐变把页面洗白。
- 正文和注释不要使用高饱和色；高饱和或高亮度颜色只作为小面积 `accent`，避免用于大段文字、大面积背景或整页高饱和渐变。
- 所有文字、图标、线条和图表都必须与背景保持清晰可读的对比；不要把“对比充足”做成刺眼的纯黑/纯白/荧光色组合。弱化信息可以降低饱和度或透明度，但不能牺牲可读性。
- 图片或渐变背景上放文字时，给文字区域加半透明遮罩、色块或足够留白；不要把正文直接压在复杂背景上。
- 渐变色必须使用 `rgba()` 格式并带百分比停靠点，例如 `linear-gradient(135deg,rgba(15,23,42,1) 0%,rgba(56,97,140,1) 100%)`；使用 `rgb()` 或省略停靠点可能导致服务端回退为白底。
- 验证时逐页检查：背景是否显式填充且未无意变白，正文颜色是否可读，`accent` 是否过量，渐变是否可能回退。

### 字体与字号

- 标题字体可以有性格，正文字体必须清晰耐读；不要整份 deck 都默认 Arial。
- 中英文混排时，字体族先写英文/拉丁字体，再写中文/CJK 字体，最后写通用 fallback；标题和正文各用一套稳定组合。
- 字体选择要匹配视觉系统的类别和处理方式：衬线、无衬线、圆体、等宽、粗窄标题、全大写等风格不要随意互换。
- 常用标题方向：`Playfair Display` / `EB Garamond` / `Lora` 适合编辑感和高级感；`Anton` / `Bebas Neue` / `Oswald` 适合强冲击标题；`DM Sans` / `Montserrat` / `Poppins` 适合现代产品和商业正文。
- 常用中文方向：`思源宋体` 适合长文和编辑感；`思源黑体` / `黑体` 适合中性现代；`寒蝉德黑体` 适合工业和科技；`寒蝉全圆体` / `资源圆体` 适合温暖亲和；书法类字体只用于少量标题。

| 元素 | 建议字号 |
|------|----------|
| 封面标题 | 40-56px；纯标题页可到 64-96px |
| 内容页标题 | 28-40px |
| 副标题 / 分区标题 | 20-26px |
| 正文一级 | 16-20px |
| 正文二级 | 13-16px |
| 注释 / 来源 | 11-13px |
| Hero number | 80-140px |

不要为了填满空页面而盲目放大字体；页面显得空时，优先补充有意义的信息、调整构图或强化边缘对齐。

### 布局

- 先判断内容关系，再设计版式。比较、流程、时间线、循环、层级、矩阵、漏斗、整体-部分、因果等关系，应通过位置、对齐、分组、比例和流向直接表达。
- 版式本身要承载逻辑：比较用并列和基线对齐，流程用方向和连接，层级用尺度和嵌套，矩阵用坐标和象限，因果用箭头和阅读顺序。
- 每页都应围绕该页内容重新组织，不要从固定模板里盖章；同一 deck 可以复用视觉母题，但不要所有页面都是标题 + 三 bullets。
- 页面要留呼吸感。内容块之间保持稳定间距，卡片内边距要真实存在；文字不要贴边，也不要被装饰线、图片或页脚挤压。
- 正文默认左对齐；只有封面、章节页、结尾页、大号数字或少量仪式感页面适合居中。

### 视觉元素与图表

- 视觉元素必须承载意义或引导注意力，不做填空装饰。图片、图标、图表、表格、色块、连线都要解释内容关系、强调重点或改善阅读节奏。
- 每个内容页至少应有一个非纯文本视觉锚点：图片、图标、图表、表格、流程、对比结构、大号数字、示意图或抽象 shape 组合。
- 信息图、截图、图表等素材要保持原始比例；不要为了塞进版面强行裁切或拉伸。装饰性照片可以更自由，但仍要服务主题和构图。
- 有真实数据序列时，先写清图表要证明的 takeaway，再选择图表类型；一张图只表达一个核心结论。单个数字或两项简单对比，优先用大号数字 callout，不必硬画图。
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
- 不要用图表承载多个结论，也不要因为有数字就机械画图。
- 不要在标题下方画装饰强调线作为默认设计手法；优先用空间关系、局部低饱和色块、尺度、分区和对齐建立层级。

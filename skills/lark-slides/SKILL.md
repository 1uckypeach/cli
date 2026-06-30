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

本地 `skills/lark-slides/scripts/` 下的 Python 工具要求 Python 3.10+；如果 `python3 --version` 低于 3.10，先切换到 3.10+ 解释器再运行脚本。

| 用户需求 | 指引 |
|----------|------|
| 读取 / 分析本地 PPTX 内容 | 文本用 `python -m markitdown presentation.pptx`；视觉总览用 `python3 scripts/thumbnail.py presentation.pptx`；原始 OOXML 用 `python3 scripts/office/unpack.py presentation.pptx unpacked/` |
| 从模板创建或编辑已有本地 PPTX | 先读 `lark-slides-pptx-template-workflows.md` |
| 从零新建飞书在线 PPT | 先读 `lark-slides-create-workflows.md` |
| 获取在线 slides 内容、读取 / 分析已有在线 PPT | XML 内容优先用 `slides +xml-get` 保存到文件；页面视觉内容用 `slides +screenshot`，详见 `lark-slides-screenshot.md` |

## 读取 / 分析内容

### 本地 PPTX

```bash
# 提取文本
python -m markitdown presentation.pptx

# 生成视觉总览图
python3 scripts/thumbnail.py presentation.pptx

# 解包查看原始 OOXML
python3 scripts/office/unpack.py presentation.pptx unpacked/
```

### 在线 Slides

```bash
# 读取完整 XML 内容，优先保存到文件再分析
lark-cli slides +xml-get --as user --presentation <slides_url_or_token> --output presentation.xml --json

# 获取页面截图；必须指定 --slide-number 或 --slide-id，多个页面可重复传 --slide-number
lark-cli slides +screenshot --as user --presentation <slides_url_or_token> --slide-number 1 --output-dir screenshots --json
```

在线 Slides 的截图参数和页码语义详见 [`lark-slides-screenshot.md`](references/lark-slides-screenshot.md)；需要继续编辑在线 Slides 时，按 `lark-slides-create-workflows.md` / `lark-slides-replace-workflows.md` 选择创建或替换流程。

## 编辑 PPTX 工作流

**完整流程先读 [`lark-slides-pptx-template-workflows.md`](references/lark-slides-pptx-template-workflows.md)。**

1. 用 `thumbnail.py` 和 `markitdown` 分析模板。
2. 解包 -> 调整页面结构 -> 编辑内容 -> 清理 -> 打包。
3. 交付前完成必需 QA。

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

不要交付只有白底、标题和项目符号的幻灯片。正式页面至少要在视觉元素、信息结构、配色、字体或留白上体现明确设计意图。

### 开始前

- **配色贴合主题**：配色要回应本次主题、行业和受众，不要套用通用蓝色商务风；如果换到另一个完全无关主题也成立，说明选择还不够具体。
- **建立视觉主次**：主色承担约 60-70% 视觉权重，1-2 个辅助色负责分区和层级，强调色只用于关键数字、结论或行动点。
- **规划明暗节奏**：可采用深色封面、浅色内容、深色结尾的结构，也可以全篇深色；无论哪种策略，都要保证正文、图标和线条有足够对比度。
- **固定一个视觉母题**：选择一个可复用元素贯穿全文，例如圆角图片框、彩色圆形图标底、粗侧边栏、编号节点、半出血图片区或大号数字，避免每页换一套装饰语言。

### 配色参考

根据内容选择颜色，不要把蓝色当作默认答案。下表只提供方向感，实际使用时可按页面明暗、透明度和对比需求微调。

| 风格 | 主色 | 辅助色 | 强调色 |
|------|------|--------|--------|
| **深夜高管风** | `1E2761`（深海军蓝） | `CADCFC`（冰蓝） | `FFFFFF`（白） |
| **森林苔藓风** | `2C5F2D`（森林绿） | `97BC62`（苔绿色） | `F5F5F5`（浅米白） |
| **珊瑚活力风** | `F96167`（珊瑚红） | `F9E795`（金黄） | `2F3C7E`（藏青） |
| **暖陶土风** | `B85042`（陶土红） | `E7E8D1`（沙色） | `A7BEAE`（鼠尾草绿） |
| **海洋渐变风** | `065A82`（深海蓝） | `1C7293`（蓝绿色） | `21295C`（午夜蓝） |
| **炭灰极简风** | `36454F`（炭灰） | `F2F2F2`（灰白） | `212121`（近黑） |
| **青绿信任风** | `028090`（青蓝） | `00A896`（海沫绿） | `02C39A`（薄荷绿） |
| **莓果奶油风** | `6D2E46`（莓紫） | `A26769`（玫瑰灰） | `ECE2D0`（奶油色） |
| **鼠尾草冷静风** | `84B59F`（鼠尾草绿） | `69A297`（桉叶绿） | `50808E`（石板蓝） |
| **樱桃强对比风** | `990011`（樱桃红） | `FCF6F5`（暖白） | `2F3C7E`（藏青） |

### 单页设计

每页至少要有一个视觉元素：图片、图标、图表、表格、流程、对比结构、大号数字、示意图或由 shape 组成的抽象视觉。文本框本身不算主视觉。

布局可选：

- 双栏结构：左文右图或左图右文，视觉区域占 35-45% 宽度。
- 图标文本行：图标放在色块或圆形底中，旁边配短标题和一句解释。
- 2x2 / 2x3 网格：适合能力、模块、风险、行动项，每格内容保持同等层级。
- 半出血视觉：图片或抽象形状占据左/右半屏，文字覆盖或贴边排布。

数据展示可选：

- 大数字卡片：关键指标用 60-72pt 数字，下面配 10-14pt 标签。
- 对比列：before/after、方案 A/B、问题/解法用左右并列，标题和基线严格对齐。
- 时间线或流程图：步骤用编号节点和箭头表达，流程方向必须一眼可见。

视觉细节可选：

- section header 旁可以放小号彩色圆形图标。
- 关键数字、tagline 或结论短句可用斜体或强调色，但不要把整段正文都做成强调样式。

### 字体排版

标题字体可以更有性格，正文字体必须清晰耐读；不要整份 deck 都默认 Arial。生成 XML 时，`fontFamily` 应使用以下支持字体的精确名称；同一份 deck 内优先选择 1-2 个字体家族，避免每页混用太多字体。

| 标题字体 | 正文字体 |
|----------|----------|
| Arial Black | Arial |
| Georgia | Calibri |
| Trebuchet MS | Calibri |
| Playfair Display | Lato |
| Montserrat | Open Sans |
| 思源宋体 | 思源黑体 |

| 元素 | 字号 |
|------|------|
| 页面标题 | 36-44pt，加粗 |
| 分区标题 | 20-24pt，加粗 |
| 正文 | 14-16pt |
| 注释/来源 | 10-12pt，弱化处理 |

#### 常用中文字体

思源宋体、寒蝉德黑体、标小智无界黑、寒蝉锦书宋、站酷小薇体、寒蝉团圆体 圆体、寒蝉团圆体 黑体、荆南缘默体、寒蝉端黑宋、资源圆体、钟齐流江毛草、寒蝉端黑体、站酷庆科黄油体、寒蝉云墨黑、有字库龙藏体、寒蝉全圆体、思源黑体、钟齐志莽行书、抖音美好体、马善政毛笔楷体、霞鹜 975 圆体

#### 常用拉丁字体

Francois One、Heebo、Lobster、Roboto Slab、Varela Round、PT Serif、Signika、Vollkorn、Mulish、Rokkitt、Inconsolata、PT Sans Caption、EB Garamond、Dancing Script、Rajdhani、Poppins、Merriweather、PT Sans Narrow、Libre Baskerville、Slabo 27px、Inter、Noto Serif、Yanone Kaffeesatz、Merriweather Sans、Lato、Source Code Pro、Mukta、Teko、Hind Siliguri、Catamaran、Arvo、Alegreya Sans、Titillium Web、Roboto Mono、Play、Indie Flower、Ubuntu Condensed、Libre Franklin、Barlow、PT Sans、Acme、Cuprum、Josefin Sans、DM Sans、Playfair Display、Rubik、Questrial、Anton、Oswald、Cabin、Ubuntu、Abel、Exo 2、Bree Serif、Roboto Condensed、Amatic SC、Abril Fatface、Comfortaa、IBM Plex Sans、Work Sans、Kanit、Noto Sans、Alegreya、Shadows Into Light、Barlow Condensed、Nunito Sans、Quicksand、Overpass、Bebas Neue、Raleway、Exo、Archivo Narrow、Hind、Open Sans、Poiret One、Asap、Roboto、Nunito、Bitter、Dosis、Oxygen、Prompt、Karla、Fjalla One、Fira Sans、Crimson Text、Pacifico、Arimo、Maven Pro、Cairo、Montserrat、Righteous、Lora

#### 其他语言字体

源ノ角ゴシック、본고딕、Nanum Gothic

#### 系统字体

Arial、Arial Black、Calibri、Comic Sans Ms、Sans Serif、Serif、Times New Roman、Tahoma、Trebuchet MS、Verdana、Georgia、Garamond、黑体、宋体、楷体、Hiragino Mincho

### 留白与间距

- 页面边距至少 0.5"。
- 内容块之间保持 0.3-0.5" 间距，并在同一 deck 内保持一致。
- 留出呼吸感，不要填满每一寸空间。
- 卡片内边距要真实留出空间；对齐 shape 和文字时要考虑文本框 padding，必要时给文本框设置 `margin: 0`。

### 避免事项

- 不要所有页面复用同一种标题 + 三 bullets 版式。
- 不要正文居中；段落和列表默认左对齐，只在封面、结尾或大号数字场景中居中。
- 不要缺少字号层级；标题需要 36pt+，明显区别于 14-16pt 正文。
- 不要默认蓝色；配色要反映具体主题。
- 不要随机混用间距；选择 0.3" 或 0.5" 间距后全 deck 统一。
- 不要只设计一页，其余页面保持 plain；要么全篇贯彻设计语言，要么整体保持克制。
- 不要创建纯文本页；必须加入图片、图标、图表或其他视觉元素，避免 plain title + bullets。
- 不要忘记文本框 padding；线条或 shape 与文字边缘对齐时，设置 `margin: 0` 或为 padding 做偏移。
- 不要使用低对比元素；图标和文字都必须和背景有强对比。
- 不要在标题下方画一条装饰强调线；这类做法很容易显得模板化。优先用留白、背景色块或结构分区建立层级。

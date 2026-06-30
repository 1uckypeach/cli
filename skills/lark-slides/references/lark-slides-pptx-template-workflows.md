# PPTX 模板编辑工作流

本文用于用户给定 `.pptx` 模板或已有 `.pptx`，并要求基于它编辑、改写、续写、美化或生成新的 PPTX。流程对齐 Anthropic `skills/pptx/editing.md` 的实现思路：先分析模板，再规划页面映射，先完成结构调整，最后逐页编辑内容；除非用户明确不要导入为飞书 Slides，否则最终默认导入并交付在线 Slides。

> **边界**：本流程的编辑阶段只使用 `skills/lark-slides/scripts/` 下的 PPTX / OOXML 脚本和本地 OOXML 编辑，不使用 `lark-cli slides`、`xml_presentations.*`、`slides_xml_schema_definition.xml`、`template_tool.py`、`iconpark_tool.py`、`xml_text_overlap_lint.py`、`+replace-slide`、`+replace-pages`、`+media-upload` 或任何飞书在线 Slides 组件。交付阶段按后文 [导入为飞书 Slides (Required)](#导入为飞书-slides-required) 执行。

> **Python 版本**：本流程中的 Python 脚本要求 Python 3.10+。执行前先确认 `python3 --version`；如果环境仍是 Python 3.9.x，不要继续运行这些脚本，先切换到 Python 3.10+。

## Template-Based Workflow

### 1. 分析模板

先生成模板缩略图，再提取可读文本。缩略图用于看版式，文本输出用于识别占位内容。

```bash
python3 skills/lark-slides/scripts/thumbnail.py \
  "work/<task-id>/template.pptx" \
  "work/<task-id>/template-thumbnails" \
  --cols 4

python -m markitdown "work/<task-id>/template.pptx" \
  > "work/<task-id>/template.md"
```

如果当前环境没有 `markitdown`，不要安装依赖；改为解包后直接阅读 `ppt/slides/slide*.xml` 中的文本占位符。

分析时记录：

- 每页的 `slideN.xml` 文件名、页序、页面用途和可复用程度。
- 封面、目录、章节页、内容页、图文页、数据页、引用页、结尾页等页型。
- 每页的占位文本、图片槽、图表槽、图标槽、页脚和备注。
- 哪些页面适合复制，哪些页面应删除。

### 2. 规划页面映射

为每个内容章节选择一个模板页面。不要默认使用标题 + bullets 的基础页。

优先主动寻找并复用不同版式：

- 2 栏 / 3 栏布局
- 图片 + 文本组合
- 全出血图片 + 文字覆盖
- 引用 / callout 页
- 章节分隔页
- 关键数字 / 指标页
- 图标网格或图标 + 文本行
- 表格、流程、时间线或图表页

避免每一页都重复同一种文本密集版式。内容类型要匹配页面风格：关键点可以用列表页，团队或模块信息适合多栏页，证言或观点适合引用页，指标适合大数字页。

### 3. 解包

```bash
python3 skills/lark-slides/scripts/office/unpack.py \
  "work/<task-id>/template.pptx" \
  "work/<task-id>/unpacked"
```

`office/unpack.py` 会解包 PPTX、格式化 XML / `.rels`，并处理智能引号转义。后续编辑都在 `unpacked/` 内完成。

如果中途使用 PowerPoint、LibreOffice、`python-pptx` 或其他外部工具直接修改了 packed `.pptx` 文件，旧的 `unpacked/` 目录就不再代表最新内容。后续若要回到本脚本工作流继续编辑，必须先把最新 `.pptx` 重新 unpack 到新的目录，或清空旧目录后重新 unpack；不要继续基于过期的 `unpacked/` 打包。

### 4. 构建演示文稿结构

结构调整必须在内容编辑之前完成，并由主 agent 自己做，不要交给子 agent 并行处理。

- 删除不用的页面：从 `ppt/presentation.xml` 的 `<p:sldIdLst>` 中移除对应 `<p:sldId>`，然后运行 `clean.py` 清理孤立文件。
- 复制要复用的页面：用 `add_slide.py`，不要手动复制 slide 文件。
- 从版式创建页面：用 `add_slide.py unpacked/ slideLayoutN.xml`。
- 调整页序：重排 `ppt/presentation.xml` 中 `<p:sldIdLst>` 的 `<p:sldId>` 顺序。
- 完成所有新增、删除、复制、重排后，再进入内容编辑阶段。

```bash
# 复制已有页面，例如基于 slide2.xml 创建新页
python3 skills/lark-slides/scripts/add_slide.py \
  "work/<task-id>/unpacked" \
  slide2.xml

# 或基于 slide layout 创建空白新页
python3 skills/lark-slides/scripts/add_slide.py \
  "work/<task-id>/unpacked" \
  slideLayout2.xml
```

`add_slide.py` 会补充 slide 文件、content type 和 presentation relationship，并打印需要加入 `ppt/presentation.xml` 的 `<p:sldId .../>`。把这行插入到目标页序位置。

### 5. 编辑内容

结构稳定后，再逐页编辑 `ppt/slides/slideN.xml`。每页是独立 XML 文件，可以在这个阶段并行处理；如果使用子 agent，提示必须包含：

- 要编辑的 slide 文件路径。
- 使用 Edit 工具做所有改动。
- 本文的格式规则和常见坑。

每页处理顺序：

1. 读取该页 XML。
2. 找出所有占位内容：文本、图片、图表、图标、caption、来源说明。
3. 将每个占位内容替换为最终内容。
4. 删除多余元素，而不是只清空文字。
5. 保留模板原有段落、run、对齐、字体、字号、颜色和间距结构。

> **重要**：内容替换优先使用 Edit 工具，不要用 `sed` 或临时 Python 脚本批量替换。Edit 工具会迫使修改点具体化，可靠性更高。

### 6. 清理

```bash
python3 skills/lark-slides/scripts/clean.py \
  "work/<task-id>/unpacked"
```

`clean.py` 会移除不在 `<p:sldIdLst>` 中的页面、未引用媒体、孤立 `.rels`、未引用图表 / diagram / drawing / theme / notes，并更新 `[Content_Types].xml`。

### 7. 打包

```bash
python3 skills/lark-slides/scripts/office/pack.py \
  "work/<task-id>/unpacked" \
  "work/<task-id>/output.pptx" \
  --original "work/<task-id>/template.pptx"
```

`office/pack.py` 会先做 PPTX 校验，再压缩 XML 并打包。若只需要重写 ZIP 压缩，不改变内容，可用：

```bash
python3 skills/lark-slides/scripts/rezip.py \
  "work/<task-id>/output.pptx" \
  "work/<task-id>/output.rezipped.pptx"
```

### 8. 视觉 QA

必须按后文 [QA (Required)](#qa-required) 完成内容 QA、图片渲染、视觉检查和修复复验。`thumbnail.py` 可用于快速页序检查；最终视觉 QA 优先使用 [Converting to Images](#converting-to-images) 生成的全分辨率页面图。

```bash
python3 skills/lark-slides/scripts/thumbnail.py \
  "work/<task-id>/output.pptx" \
  "work/<task-id>/preview/thumbnails" \
  --cols 4
```

## QA (Required)

默认假设第一版一定有问题。QA 的目标不是确认成功，而是主动找出缺陷并修到没有新问题。

### Content QA

用文本抽取检查内容是否缺失、顺序是否错误、是否还有模板占位文案。

```bash
python -m markitdown "work/<task-id>/output.pptx" \
  > "work/<task-id>/qa/content.md"
```

模板编辑必须检查残留占位词。命中任何结果都要先修复，再进入最终交付。

```bash
python -m markitdown "work/<task-id>/output.pptx" \
  | grep -iE "xxxx|lorem|ipsum|this.*(page|slide).*layout|placeholder|占位|示例|请替换"
```

检查重点：

- 内容是否完整，没有漏掉用户材料里的章节、数字、结论或来源。
- 页序和章节顺序是否正确。
- 标题、图表标签、脚注、页脚、引用和备注是否仍匹配当前内容。
- 是否残留模板说明文字、占位头像、空卡片、默认图表数据或示例 logo。

### Visual QA

把 PPTX 转成逐页图片后检查。即使只有 2-3 页，也建议让另一个 agent / reviewer 用新视角看图；自己盯着 XML 太久，很容易只看到预期结果。

检查时使用类似提示：

```text
请视觉检查这些幻灯片。默认其中存在问题，请主动找出它们。

重点检查：
- 元素重叠：文字穿过形状、线条穿过文字、多个元素堆叠。
- 文本溢出或在文本框 / 页面边缘被裁切。
- 装饰线、分割线或背景块只适配单行标题，但标题实际换成两行。
- 来源、脚注或页码与正文内容碰撞。
- 元素距离太近，卡片、栏目或段落之间小于 0.3 英寸。
- 外边距不足，内容距离页面边缘小于 0.5 英寸。
- 同类元素未对齐，栏宽、卡片高度、图标位置不一致。
- 低对比度文字或图标。
- 文本框过窄导致异常换行。
- 残留占位内容、空头像框、空图标底座、默认图表数据。

逐页列出所有问题或可疑区域，即使只是轻微问题。
```

给 reviewer / subagent 的输入应包含每张图的路径和预期内容：

```text
Read and analyze these images:
1. /abs/path/slide-01.jpg (Expected: 封面，包含标题、副标题、主视觉)
2. /abs/path/slide-02.jpg (Expected: 三栏方案对比，包含 A/B/C 三个模块)

Report all issues found, including minor ones.
```

### Verification Loop

1. 生成 PPTX。
2. 转成图片。
3. 检查并列出问题；如果第一轮没有发现任何问题，要更严格地重新看一遍。
4. 修复问题。
5. 重新打包并只复验受影响页面。
6. 重复直到一整轮检查没有新增问题。

不要在至少完成一次“发现问题 → 修复 → 复验”的闭环前宣称完成。

## Converting to Images

将 PPTX 转为逐页图片，用于视觉检查。

```bash
mkdir -p "work/<task-id>/preview"

python3 skills/lark-slides/scripts/office/soffice.py \
  --headless \
  --convert-to pdf \
  --outdir "work/<task-id>/preview" \
  "work/<task-id>/output.pptx"

pdftoppm \
  -jpeg \
  -r 150 \
  "work/<task-id>/preview/output.pdf" \
  "work/<task-id>/preview/slide"
```

这会生成 `slide-1.jpg`、`slide-2.jpg` 等文件。给 reviewer 时可按实际文件名列出；若需要稳定的两位编号，可在本地重命名为 `slide-01.jpg`、`slide-02.jpg`。

修复后只重渲染某几页：

```bash
pdftoppm \
  -jpeg \
  -r 150 \
  -f N \
  -l N \
  "work/<task-id>/preview/output.pdf" \
  "work/<task-id>/preview/slide-fixed"
```

也可以用 `thumbnail.py` 生成总览图，辅助检查页序、隐藏页和整体节奏：

```bash
python3 skills/lark-slides/scripts/thumbnail.py \
  "work/<task-id>/output.pptx" \
  "work/<task-id>/preview/thumbnails" \
  --cols 4
```

如果 LibreOffice / `soffice` 在本机崩溃或无法生成 PDF，可用系统预览能力做临时降级检查，但最终交付前仍应优先使用成功渲染出的逐页图片、在线截图或人工打开 PPTX 复验。

```bash
qlmanage -t -s 1200 -o "work/<task-id>/preview" "work/<task-id>/output.pptx"
```

## 导入为飞书 Slides (Required)

本流程的编辑阶段只处理本地 PPTX。除非用户明确说明“不导入为飞书 Slides”或“只要本地 PPTX”，否则先完成本地 QA，再切到 `lark-drive` 导入，并把在线 Slides 作为默认交付物。

```bash
lark-cli drive +import --as user \
  --file "work/<task-id>/output.pptx" \
  --type slides \
  --name "<title>" \
  --json
```

导入成功后保存返回的 `url` / `token`，最终回复中必须交付 Slides 链接。需要在线视觉复验时，用 `slides +screenshot` 指定页码；多页截图优先在一次命令里重复传 `--slide-number`，避免紧密循环触发频控。

```bash
lark-cli slides +screenshot --as user \
  --presentation "<slides_url_or_token>" \
  --slide-number 1 \
  --slide-number 2 \
  --output-dir "work/<task-id>/online-screenshots" \
  --json
```

## Dependencies

本流程依赖以下工具；缺失时先说明不能完成对应验证，不要静默跳过。

- Python 3.10+：`skills/lark-slides/scripts/` 下的脚本使用 Python 3.10+ 语法，不支持 Python 3.9.x。
- `markitdown[pptx]`：文本抽取和占位内容检查。
- `Pillow`：`thumbnail.py` 生成缩略图网格。
- LibreOffice (`soffice`)：PPTX 转 PDF；本仓通过 `skills/lark-slides/scripts/office/soffice.py` 包装调用环境。
- Poppler (`pdftoppm`)：PDF 转逐页图片。
- `defusedxml`：OOXML 脚本安全解析 XML。

不把 `pptxgenjs` 作为本模板编辑流程的必需依赖；它属于从零创建 PPTX 的路线，不是本文件的默认工作流。

## Scripts

| Script | Purpose |
|--------|---------|
| `office/unpack.py` | 解包并 pretty-print PPTX XML / `.rels` |
| `add_slide.py` | 复制 slide 或从 slideLayout 创建 slide |
| `clean.py` | 删除孤立页面、媒体、关系和 content type |
| `office/pack.py` | 校验并重新打包 PPTX |
| `office/validate.py` | 单独校验解包目录或 PPTX 文件 |
| `thumbnail.py` | 生成带 slide 文件名标签的缩略图网格 |
| `rezip.py` | 仅重写 ZIP 压缩，不改内容 |

### office/unpack.py

```bash
python3 skills/lark-slides/scripts/office/unpack.py input.pptx unpacked/
```

解包 PPTX，格式化 XML，转义智能引号。

### add_slide.py

```bash
python3 skills/lark-slides/scripts/add_slide.py unpacked/ slide2.xml
python3 skills/lark-slides/scripts/add_slide.py unpacked/ slideLayout2.xml
```

第一种复制已有页面，第二种从 layout 创建页面。脚本会打印需要插入 `ppt/presentation.xml` 的 `<p:sldId>`。

### clean.py

```bash
python3 skills/lark-slides/scripts/clean.py unpacked/
```

清理不再被 presentation 或 slide relationships 引用的文件。

### office/pack.py

```bash
python3 skills/lark-slides/scripts/office/pack.py \
  unpacked/ output.pptx --original input.pptx
```

校验、修复可自动修复的问题、压缩 XML，并重新打包 PPTX。

### thumbnail.py

```bash
python3 skills/lark-slides/scripts/thumbnail.py input.pptx [output_prefix] [--cols N]
```

生成缩略图网格，图片下方标注 `slideN.xml`。默认 3 列，`--cols` 最大值由脚本限制。

## Slide Operations

页面顺序由 `ppt/presentation.xml` 里的 `<p:sldIdLst>` 决定。

- **Reorder**：重排 `<p:sldId>` 元素。
- **Delete**：删除 `<p:sldId>`，再运行 `clean.py`。
- **Add**：使用 `add_slide.py`。不要手动复制 slide 文件；脚本会处理备注关系、`[Content_Types].xml` 和 relationship ID。

## Formatting Rules

- **标题、分组标题和行内标签要加粗**：在对应 run 的 `<a:rPr>` 上使用 `b="1"`。例如页面标题、section header，以及 `Status:`、`Description:` 这类行首标签。
- **不要使用 Unicode bullet `•`**：使用 PPTX 原生列表格式，例如 `<a:buChar>` 或 `<a:buAutoNum>`。
- **列表样式保持一致**：尽量继承模板 layout 的 bullet 设置；只有模板缺失或需要显式修正时才新增 bullet 属性。
- **多条内容不要拼进一个字符串**：编号列表、多步骤、多段说明应拆成多个 `<a:p>`；复制原段落的 `<a:pPr>` 以保留行距和缩进。
- **长文本要适配模板槽位**：更长的替换文本可能溢出或异常换行；优先压缩文案、拆页或换用更合适的模板页。
- **保留空白时加 `xml:space="preserve"`**：对有前后空格的 `<a:t>` 使用 `xml:space="preserve"`。
- **XML 解析用 `defusedxml.minidom`**：不要用 `xml.etree.ElementTree` 重写 OOXML；它容易破坏命名空间和格式。

## Common Pitfalls

### Template Adaptation

当源内容项少于模板槽位时，删除多余的整组元素，包括图片、shape、文本框和图标，不要只清空文字。清空文字但保留视觉元素，会留下无意义的头像框、图标底座或空卡片。

模板槽位不等于源内容项。例如模板有 4 个成员卡片而源内容只有 3 人，应删除第 4 个成员的整组元素，而不是只删姓名。

### Multi-Item Content

多步骤、多条结论、多段说明应拆成多个段落。

错误做法：所有项目都塞进同一个 `<a:t>`。

```xml
<a:t>Step 1: Do the first thing. Step 2: Do the second thing.</a:t>
```

正确做法：每个项目使用独立 `<a:p>`，并对 header run 加粗。

```xml
<a:p>
  <a:r><a:rPr b="1"/><a:t>Step 1</a:t></a:r>
  <a:r><a:t> Do the first thing.</a:t></a:r>
</a:p>
<a:p>
  <a:r><a:rPr b="1"/><a:t>Step 2</a:t></a:r>
  <a:r><a:t> Do the second thing.</a:t></a:r>
</a:p>
```

### Smart Quotes

`office/unpack.py` / `office/pack.py` 会处理智能引号。手动新增带引号的文本时，优先写 XML entity。

| Character | XML entity |
|-----------|------------|
| `“` | `&#x201C;` |
| `”` | `&#x201D;` |
| `‘` | `&#x2018;` |
| `’` | `&#x2019;` |

### Visual QA

修改内容后必须看渲染结果。尤其检查：

- 模板页型是否单调重复。
- 长文本是否溢出、遮挡或异常换行。
- 图片和图标是否缺失、变形或裁剪错误。
- 删除内容后是否留下孤立形状。
- 图表标签、图例、数据系列和来源说明是否仍然匹配。

## 交付格式

最终回复用户时说明：

- 输出 PPTX 的绝对路径。
- 导入后的飞书 Slides 链接；如果用户明确不要导入，说明本次按用户要求未导入。
- 源文件是否保持未修改。
- 复用了哪些模板页型。
- 做过哪些关键结构调整和内容替换。
- 是否完成缩略图或全分辨率视觉 QA；如果未做，说明具体原因。

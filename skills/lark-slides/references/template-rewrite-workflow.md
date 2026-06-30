# Template Rewrite Workflow

本工作流只服务基于真实模板或底稿的二次创作。适用场景：

- 用户上传 PPTX / PDF / slides。
- 用户给 existing Slides。
- 用户说“基于这个模板生成”。
- 用户说“保留这个版式 / 底稿 / 原 PPT / 模板风格和结构”。
- 用户要对已有 PPT 做二次创作、改写、替换内容。

禁止默认行为：

- 不默认走 `planning-layer.md`。
- 不默认读取 `visual-planning.md`。
- 不默认读取 `asset-planning.md`。
- 不生成 `slide_plan.json`。
- 不生成 `page_rewrite_plan.json`。
- 不生成 `rewrite_manifest.json`。
- 不默认用 `slides +create` 新建脱离模板的 deck。
- 不通过 `python-pptx` / PowerPoint 自动化把模板页全部删除后从空白 layout 重画。
- 不生成一个“借用模板尺寸/主题色”的本地 PPTX 再导入，并声称它是模板二创。
- 不把模板页当成背景板，再在上面覆盖一套通用标题栏、两卡、三卡、2x2 卡片或大白卡系统。
- 不把每页内容粘贴进一套重复组件，而让源页的箭头、节点、时间线、图形、图表、留白关系失去表达作用。

模板二创的数据流固定为：

```text
source.xml
-> generate replacement slide XML from each source slide skeleton
-> pages.json
-> slides +replace-pages
-> readback.xml validation
```

## 1. Import / Readback

- PPTX 必须先导入为 Slides。导入命令本身走 `lark-drive` 的 `drive +import --type slides`，但导入后的二创由本工作流负责。
- PDF 如果作为模板、底稿、原 PPT 或视觉参考使用，也先导入为 Slides；只有明显是长文档资料而非演示稿时才不进入本工作流。
- Existing Slides 用 `xml_presentations.get` 回读。
- 保存回读 XML 为：

```text
.lark-slides/plan/<xml_presentation_id>/source.xml
```

如果无法得到 `source.xml`，Template Rewrite Workflow 不能继续。不要退化为：

- 用 `python-pptx` 打开模板后删除所有原 slides。
- `prs.part.drop_rel(...)` / `del prs.slides._sldIdLst[...]` 清空模板页。
- 从 `prs.slide_layouts[...]` 或 blank layout 重新 `add_slide(...)`。
- 只保留画布尺寸、少量主题色、少量母版占位符后重画整套内容页。
- 输出一个新的本地 PPTX，再用 `drive +import --type slides` 当最终产物。

正确处理是：停止 Template Rewrite，说明 `source.xml` 不可用，并让用户选择导入失败排障、只交付原导入 deck，或明确切换到 Create Workflow / 只参考风格重做。

## 2. Treat source.xml As Truth

`source.xml` 是唯一布局和素材事实源。

- 不要把 `source.xml` 里的素材 token、bbox、层级、样式再复制到新的 plan 文件。
- 不要让模型手写素材清单来替代 `source.xml`。
- 所有保留判断以 `source.xml` 为准。
- 不要用 `layout_type`、`visual_focus`、`visual_system` 来驱动模板二创。
- 可以在上下文中临时分析每页的源结构，但不要把它保存成新的 JSON / Markdown plan artifact。

## 3. Rewrite From Source Outward

以源页 XML 为骨架生成 replacement slide。默认顺序：

1. 先复制源页的 `<style>`。
2. 复制源页的 `<img src="...">`、`<chart>`、`<table>`、`<whiteboard>`。
3. 复制 recurring shapes / motifs、line / icon / separator、card container、reusable text container。
4. 识别源页中承载表达的 dominant structure，例如箭头流、节点关系、时间线、漏斗、三角形、圆环、曲线、坐标/表格、左右对照、设备图、人物/场景分组。
5. 把新内容映射到源页已有文本容器、图形标签、数字标签、节点标签或图表/table 数据上。
6. 替换旧文案所在文本容器里的 `<content>`。
7. 最后只在必要时添加局部新元素。

不要把源页改成通用两卡、三卡、2x2 卡片。不要把“保留模板”简化成“保留背景图 + 重新画业务卡片”。

生成 replacement slide 时，页面级结构必须来自源页 XML。可以替换或缩短文字、更新图表数据、局部补充元素；不能把源页删除后按自定义 `rect()` / `circle()` / `line()` / `add_text()` helper 重新搭一套卡片、流程、指标版式。

### Source-Connected Rewrite

每一页必须先在工作上下文中做源页结构判断。这个判断不是新 artifact，不写入文件；它只用于约束生成：

- page role：封面、目录、过渡、数据页、流程页、对比页、总结页等。
- dominant source structure：源页最主要的视觉结构，例如图、表、箭头、节点、时间线、几何结构、产品图、人物图、设备图、曲线或对比版式。
- content-bearing containers：真正承载文字和数字的源文本框、图形标签、图表标签、表格单元格。
- source visual hierarchy：标题、核心结论、主视觉、支撑信息、脚注的原始层级。
- safe insertion zones：只有在源页没有合适容器且用户内容必须出现时，才可使用的局部空白区域。

生成 replacement slide 时必须满足：

1. 新文案优先进入已有 text container、图形标签、节点标签、数字标签、表格单元格或 chart labels / data。
2. 如果源页有三角形、箭头、节点、时间线、曲线、地图、设备图、产品图或人物分组，新内容必须贴到这些源结构的对应标签/节点/注释上，而不是覆盖一组三张新卡片。
3. 如果源页是数据图形页，优先更新原图表、数字标签、曲线节点、坐标标签和注释；不要另造一个白色数据卡片区遮住原图。
4. 如果源页是流程/关系页，优先替换每个步骤、箭头、节点、关系说明；不要把流程压在背景里，再另起 bullet 卡片。
5. 如果源页是封面或章节页，保留原图片、标题容器、logo / slogan / 装饰关系；不要把标题挪进不相干的新色块。
6. 如果原文本容器空间不足，先缩短文案、降低层级、拆到邻近源容器或用源页已有注释容器承载；不要默认新增大卡片。
7. 新增元素只能补足源结构的局部空缺，不能成为覆盖源结构的主版式。
8. 多页之间应保留源模板原本的页型差异；不要把整套 deck 归一成同一套顶栏 + 三卡片。

### Source-Connectedness Gate

生成 `pages.json` 前，对每个 replacement slide 做一次失败门检查。出现以下任一情况，必须重写该页：

- 页面主体内容主要落在新增 shape/card 中，而不是源页已有容器或源结构节点里。
- 源页的箭头、节点、时间线、图形、图表、设备图、人物图仍在，但已经只是背景装饰，没有承载新内容。
- 新增卡片、白板、大色块或信息面板覆盖了源页 dominant structure。
- 多个源页被改成同一套顶栏、三卡、2x2 卡片或大段 bullet 容器。
- 页内关键源容器还在，但其 bbox、层级、字号、颜色、对齐关系被无理由改写。
- 源页明明有图文关系、箭头关系或坐标关系，却把内容独立堆放到空白区域，导致互相错位或遮挡。

## 4. Generate pages.json

`pages.json` 是唯一执行输入。结构只保留 `slides +replace-pages` 需要的字段：

```json
[
  {
    "slide_id": "<old slide id>",
    "content": "<full replacement slide XML>"
  }
]
```

不要把 planning metadata 放进 `pages.json`。

## 5. Execute replace-pages

- 默认用 `slides +replace-pages`。
- `replace-pages` 消费 `pages.json`，不消费 `slide_plan.json`。
- `replace-slide` 只用于小型块级编辑，例如改一个标题、插入一个图、替换已知 block。

## 6. Readback Validation

替换后必须用 `xml_presentations.get` 回读，保存为：

```text
.lark-slides/plan/<xml_presentation_id>/readback.xml
```

用 `readback.xml` 和 `source.xml` 对比验证模板结构没有被破坏。验证细则见 `validation-checklist.md` 的 Template Rewrite validation 小节。

## Preservation Rules

除非用户明确要求重设计，否则模板二创必须：

- preserve source layout
- preserve source assets
- preserve source style
- preserve source text containers
- preserve source visual hierarchy
- replace content only
- local adjustment only

具体规则：

1. `<style>` 默认保留。
2. `<img src="...">` 默认保留，尤其是背景图、截图、装饰图、产品图、logo、模板视觉。
3. 同一个 `xml_presentation_id` 内复用 `<img src>` 时，直接复制原 `src` / token，不要重新上传，不要替换成外部 URL。
4. `<chart>` / `<table>` 默认保留；除非用户要求更新数据，才改 labels / data。
5. `<whiteboard>` 默认保留其位置和外层结构；注意 readback XML 未必包含内部 SVG / Mermaid。
6. shape / line / icon / separator / card container / motif 默认保留。
7. 旧文案所在 text container 默认保留 bbox、layer、textType、fontFamily、fontSize、color、alignment，只替换 `<content>`。
8. 如果源页已有卡片容器，优先复用源容器。
9. 如果源页已有图文结构，优先替换原文本。
10. 如果必须新增元素，新增元素必须局部且不破坏源页主要视觉结构。
11. 不允许以“模板文件只是内容容器”为由清空原页；模板页本身就是必须保留的设计资产。
12. 不允许把模板当成 wallpaper。源页的 dominant structure 必须继续承载内容和语义。

## Local PPTX Is Not A Rewrite Target

Template Rewrite 的写入目标是导入后或已有的 Slides presentation。默认最终写入动作是 `slides +replace-pages`，不是创建一个新的本地 PPTX。

禁止的本地 PPTX 生成模式：

```python
while len(prs.slides._sldIdLst):
    r_id = prs.slides._sldIdLst[0].rId
    prs.part.drop_rel(r_id)
    del prs.slides._sldIdLst[0]

blank = prs.slide_layouts[...]
slide = prs.slides.add_slide(blank)
```

上面这种模式会删除背景图、截图、装饰图、产品图、logo、shape、文本框、层级和页内结构。它最多是 Create Workflow 的“新建 PPT”，不是模板二创。

## No Full-Page Wash / Mask

禁止默认添加：

- full-page wash
- near-full-page overlay
- 全页半透明白色蒙版
- 全页半透明黑色蒙版
- 覆盖页面主体区域的大矩形
- `rgba(255,255,255,0.x)` 大面积遮罩
- `rgba(0,0,0,0.x)` 大面积遮罩

原因：模板二创时，模板素材是优先保留对象。全页 wash 会视觉遮盖模板素材，即使 token 仍然存在，也等同于破坏模板。

允许的可读性增强仅包括：

- 局部 text backing
- 局部 card backing
- 调整文字颜色
- 调整字重
- 文字阴影
- 缩短文案
- 复用源页已有文本容器
- 复制 `source.xml` 中原本存在的 overlay

如果新增 overlay 覆盖了大部分画布，应判定为失败，除非：

- 该 overlay 来自 `source.xml` 原有元素；或
- 用户明确要求统一加蒙版 / 遮罩。

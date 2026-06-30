# Validation Checklist

创建或大幅改写演示文稿后，必须做一次显式验证。目标是发现空白页、XML 损坏、内容截断、明显溢出、弱视觉层级和未验证输出。

小型已有页编辑也要做对应范围的验证：至少读取被改页面或全文 XML，确认目标元素已更新且未破坏周边结构。

## Required Flow

1. 记录创建或编辑返回的 `xml_presentation_id`，以及已知的 `slide_id` / `revision_id`。
2. 用 `xml_presentations.get` 回读全文 XML。
3. 检查实际页数是否符合计划或用户要求。
4. 检查每页 `<data>` 内是否有预期主要元素。
5. 检查没有明显空白页、破损页、缺失标题或缺失主视觉。
6. 检查页面不是全部退化为标题加 bullet list。
7. 检查视觉层级：标题、主视觉、支撑信息三者可区分。
8. 检查明显溢出和布局风险：重叠、越界、底部拥挤、长文本框。
9. 在最终回复中给出简短验证记录。

回读命令：

```bash
lark-cli slides xml_presentations get --as user \
  --params '{"xml_presentation_id":"YOUR_ID"}'
```

## Template Rewrite Validation

模板二创用 `slides +replace-pages` 后必须回读全文 XML，并保存：

```text
.lark-slides/plan/<xml_presentation_id>/readback.xml
```

同时对比同目录的 `source.xml`。通过标准：

1. `readback.xml` 中仍存在 `source.xml` 的关键 `<img src>` token。
2. `readback.xml` 中仍存在关键 `<style>`、chart、table、whiteboard、shape motif、card container。
3. 旧 text container 的 bbox、layer、font、color、alignment 没有无理由变化。
4. 没有新增 full-page / near-full-page overlay、wash、mask。
5. 没有把多页改成同质化两卡、三卡、2x2 卡片。
6. 没有用大白卡、大色块覆盖模板主体素材。
7. 没有把 source img 重新上传或替换成外部 URL。
8. `pages.json` item 只包含 `slide_id` / `content`。
9. `replace-pages` 使用 create-before-delete 语义时，确认最终页数正确。
10. 如果发现模板素材 token 存在但被新增遮罩视觉遮盖，应判定为失败。
11. 没有出现 `python-pptx` 清空模板页、blank layout 重建、本地生成 PPTX 再导入的路径。
12. 源模板的媒体资产数量、关键图片 token、主要 shape/table/chart/whiteboard/text container 没有断崖式丢失。若原模板有大量媒体资产而结果只剩极少数媒体资产，应判定为失败，除非用户明确要求移除这些素材。
13. 每页的 dominant source structure 仍然存在并承载内容，例如三角形、箭头、节点、时间线、曲线、图表、表格、设备图、人物图、产品图或左右对照结构没有退化成背景装饰。
14. 新内容主要落在源页已有 text container、图形标签、节点标签、数字标签、chart/table 数据或源页注释容器里，而不是新增的通用卡片层里。
15. 没有把多页改成同一套“顶栏 + 三卡片 / 2x2 卡片 / 大 bullet 面板”的重复版式。
16. 源页有箭头流、节点关系、时间线、图形结构或坐标关系时，新文案与这些结构对齐，没有漂浮在不相关空白区域或互相遮挡。
17. 能获取截图时，至少抽查封面、典型内容页、复杂结构页和结尾页；如果总页数不超过 8 页，逐页截图检查。截图验收重点是源模板视觉结构是否仍可见且承载内容。
18. 验证记录必须说明“与 source.xml 的模板结构/素材保留对比结果”。只记录页数、关键词存在、`xml_text_overlap_lint.py error_count=0` 不足以通过 Template Rewrite validation。

允许的可读性增强仅限局部 text backing、局部 card backing、文字颜色/字重调整、文字阴影、缩短文案、复用源页已有文本容器，或复制 `source.xml` 中原本存在的 overlay。

## Automated XML Text Overlap Lint

回读 XML 保存到本地文件后，优先运行 XML 语法和文本重叠静态检查：

```bash
python3 skills/lark-slides/scripts/xml_text_overlap_lint.py --input <presentation.xml>
```

通过标准：

- `summary.error_count == 0`。任何 error 都必须先修复再交付。
- 当前工具只检查 XML well-formed 和文本元素之间的明显重叠；它不检查越界、文本高度不足、图文压盖、表格/图表压盖或底部拥挤。
- 该工具不能替代页数核对、关键内容核对或真实视觉验收。
- 该工具不能验证模板视觉一致性。Template Rewrite Workflow 中，即使 `error_count == 0`，只要源页背景、图片、shape、文本框、结构层级或主要媒体资产被清空/重画/遮挡，仍然必须判定失败。

常见 code 的处理方向：

| code | 含义 | 处理方式 |
|------|------|----------|
| `xml_not_well_formed` | XML 语法错误或文本未转义 | 修复标签闭合、属性引号、`&` / `<` / `>` 转义 |
| `bbox_overlap` | 文本元素的估算绘制区域明显重叠 | 拉开文本坐标、缩小文本框/字号，或改成明确的分栏/分组结构 |

## Page Count And Structure

- 实际页数必须等于用户要求。Create Workflow 对照 `slide_plan.json`；Template Rewrite Workflow 对照 `source.xml` / `pages.json` 和 replace-pages 结果。
- 如果创建过程部分失败，先记录已创建的 `xml_presentation_id`，再回读确认哪些页已写入。
- 每页都应包含 `<data>`，且 `<data>` 内至少有一个非背景主体元素。
- 封面、章节页、总结页可以文字较少，但不能只有空背景。
- 技术解释页、对比页、流程页、架构页必须有匹配的结构元素，例如分组框、连线、时间轴、表格或图形化区域。

## Expected Elements

Create Workflow 按 `slide_plan.json` 和用户要求逐页核对：

- 标题或主结论存在，并能对应 `key_message`。
- `layout_type` 对应的主要结构已生成。
- `visual_focus` 是页面中最醒目或最大的信息区域之一。
- `text_density` 影响了文本量，没有用长 bullet 框替代规划。
- `asset_need` 有真实素材时已放入正确区域；没有真实素材时，`fallback_if_missing` 已用 XML 形状、线条、标签、表格或图表兜底。

Template Rewrite Workflow 按 `source.xml`、`pages.json` 和用户替换要求逐页核对：

- 标题或主结论存在，并写入源页对应的标题/结论容器。
- 源页 dominant structure 仍是页面中最醒目或最大的信息区域之一。
- 新内容映射到源页已有文本容器、图形标签、节点、箭头、时间线、图表/table 或注释容器。
- 源页原有图文关系、分组关系、层级关系仍然可读，没有被新增卡片层覆盖或挤散。
- 多页之间保留源模板的页型差异，没有被统一改成同质化卡片页。
- 当源容器装不下时，优先缩短文本、降低层级或复用邻近源容器；不能用大白卡、大色块或新面板覆盖模板主体。

如果用户指定了关键页，例如“架构解释”“Self-Attention 机制解释”“对比或演进视角”“总结页”，最终验证记录必须逐项说明这些页已存在。

## Blank Or Broken Page Signals

把下面情况视为需要修复后再交付：

- `<data/>` 为空，或只有背景、装饰线、空 `<content/>`。
- 关键文本没有出现在回读 XML 中。
- 图片仍是 `@./path`，或 `<img src>` 是 http(s) 外链。
- 页面依赖的图片区域为空，且没有 fallback visual。
- Template Rewrite 结果只保留模板尺寸/主题色，丢失大部分源页图片、背景、shape、文本容器或媒体资产。
- Template Rewrite 结果由新生成本地 PPTX 导入，而不是对导入/已有 Slides 使用 `+replace-pages`。
- Template Rewrite 结果把内容贴进新增通用卡片层，源页的箭头、节点、时间线、图表、几何结构、设备图或人物图只剩背景作用。
- Template Rewrite 结果多页出现重复的顶栏、三卡片、2x2 卡片或大 bullet 面板，压过源模板原有页型差异。
- Template Rewrite 结果中源页关键结构仍存在，但新内容没有贴回对应标签、节点、数字、表格或注释位置。
- 返回 XML 缺页、页序明显错误，或某页内容被 shell 截断。
- 大量形状坐标完全相同，导致主体内容重叠。
- 渐变背景回退成空白或白底，导致文字不可读。

## Whiteboard Elements

`slide.get` 回读 XML 时，`<whiteboard>` 块只返回位置属性（`topLeftX`、`topLeftY`、`width`、`height`），SVG / Mermaid 内容**不随 XML 返回**。

- whiteboard 验证只能核对坐标是否越界：`topLeftX + width ≤ 960`，`topLeftY + height ≤ 540`。
- SVG 和 Mermaid 内容的正确性无法通过回读 XML 验证，需要人工视觉验收。
- 不要在验证记录中声称 whiteboard 内容已验证，除非用户确认了视觉效果。

## Layout And Overflow Risk

优先修复这些明显风险：

- 正文或标签框高度不足，文本很可能被截断。
- 多个主体元素在同一区域重叠，而不是有意叠加背景。
- 重要内容越过画布边界，或贴近底部超过 `y=500`。
- 高密度页使用单个长 bullet list，没有分栏、表格或分组。
- 标题、主视觉、正文的字号和颜色差异太弱，视觉层级不清。
- 所有内容页都是同一套标题加 bullets 坐标。
- Template Rewrite 中，新增元素漂浮在源结构上方，没有和源页图形、节点、表格、图片或注释形成对应关系。
- Template Rewrite 中，源页主体视觉被新增白卡、色块、面板或文字框切断。

## Verification Record

最终回复必须包含简短验证记录，建议格式：

```text
验证记录：
- 回读：已执行 xml_presentations.get，实际页数 N / 预期 N。
- 关键页：架构解释 / Self-Attention / 对比或演进 / 总结页均存在。
- 结构：检查了主要 shape/img/table/chart 元素，无明显空白页或破损页。
- 布局：检查了标题层级、主视觉、重叠/越界/文本溢出风险。
- 模板二创：逐页或抽样说明 source.xml 的 dominant structure 是否仍承载内容，是否存在通用卡片层覆盖源结构；如已截图，说明抽查页范围。
```

不要声称完成了人工视觉验收，除非确实打开或获取了可视化结果。仅从 XML 静态检查得出的结论，应表述为“静态检查未发现明显问题”。

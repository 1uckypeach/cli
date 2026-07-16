# Validation Checklist

创建或大幅改写演示文稿后，必须做一次显式验证。目标是发现空白页、XML 损坏、内容截断、明显溢出、弱视觉层级和未验证输出。

小型已有页编辑也要做对应范围的验证：至少读取被改页面或全文 XML，确认目标元素已更新且未破坏周边结构。

## Required Flow

1. 记录创建或编辑返回的 `xml_presentation_id`，以及已知的 `slide_id` / `revision_id`。`slide_id` 是 review 状态唯一关联键；页码仅可作为展示信息。
2. 用 `slides +xml-get` 回读全文 XML 到本地文件，并以当前结果建立本次 review 的 `slide_ids` 页清单。首次新建且页集合未变时，可复用创建响应；增删页、整页替换或重排后必须刷新清单。
3. 运行 XML 静态检查，检查实际页数、主要元素、空白/破损页、主视觉和布局风险。
4. 静态检查通过后，用 `slides +screenshot` 对第 1 步的全部 `slide_ids` 截图。每批最多 10 页，输出到 `.lark-slides/review/<deck-or-task-id>/screenshots/`。
5. 实际打开每张截图，按下方 rubric 逐页标记 `pass` 或 `fix`；截图文件存在但未被查看时，状态必须为 `not_reviewed`。
6. `fix` 页用 `+replace-slide` 或对应写入操作修复后，重新回读并重新截图该页；不要沿用修复前的截图结论。
7. 截图白名单或服务端限制导致无法获取图片时，记录错误和受影响页，完成其余 XML 静态检查，并将视觉状态标记为 `not_verified`。
8. 在最终回复中给出简短验证记录，明确区分静态检查和真实视觉 review。

回读命令：

```bash
lark-cli slides +xml-get --as user \
  --presentation "YOUR_ID" \
  --output .lark-slides/plan/<deck-or-task-id>/readback.xml \
  --json
```

## Automated XML Text Overlap Lint

`slides +xml-get` 保存 XML 到本地文件后，优先运行 XML 语法和文本重叠静态检查：

```bash
python3 skills/lark-slides/scripts/xml_text_overlap_lint.py --input <presentation.xml>
```

通过标准：

- `summary.error_count == 0`。任何 error 都必须先修复再交付。
- 当前工具只检查 XML well-formed 和文本元素之间的明显重叠；它不检查越界、文本高度不足、图文压盖、表格/图表压盖或底部拥挤。
- 该工具不能替代页数核对、关键内容核对或真实视觉验收。

常见 code 的处理方向：

| code | 含义 | 处理方式 |
|------|------|----------|
| `xml_not_well_formed` | XML 语法错误或文本未转义 | 修复标签闭合、属性引号、`&` / `<` / `>` 转义 |
| `sml_prefixed_tag` | SML 元素使用了命名空间前缀，如 `<ns0:slide>` 或 `<sml:shape>` | 使用 `<slide xmlns="http://www.larkoffice.com/sml/2.0">` 的默认命名空间，或使用无前缀标签 |
| `sxsd_unsupported_tag` | 使用了 SXSD 不支持的标签 | 按 lint `hint` 替换为受支持标签；常见如 `textbox -> <shape type="text">`、`image -> <img>` |
| `sxsd_unsupported_attr` | 支持的标签上使用了不支持的属性 | 按 lint `hint` 改为支持的属性；常见如 `x -> topLeftX`、`fontColor -> color` |
| `iconpark_unsupported_icon_type` | `<icon>` 使用了 `iconpark-index.json` 中不存在的 `iconType` | 按 lint `hint` 改为名单内的 `iconType`，或先用 `scripts/iconpark_tool.py` 搜索 |
| `icon_missing_fill_color` | 视觉规范要求 `<icon>` 设置 `<fill><fillColor color="..."/></fill>`，避免图标不可见 | 给 `<icon>` 添加显式非透明填充色，例如 `rgba(37, 99, 235, 1)` |
| `icon_transparent_fill_color` | `<icon>` 的 `fillColor` 是透明色，不满足视觉可见性要求 | 改成与背景有足够对比的非透明颜色 |
| `bbox_overlap` | 文本元素的估算绘制区域明显重叠 | 拉开文本坐标、缩小文本框/字号，或改成明确的分栏/分组结构 |

## Screenshot QA

获取页面截图后，必须做视觉验收；不要只凭 XML 回读或静态 lint 结论声称截图验收通过。验收时假设页面存在问题，主动寻找并报告所有风险，包括轻微问题。

```text
请逐页目视检查这些幻灯片截图。先假设存在问题，并尽量找出它们。

重点检查：
- 元素重叠：文字与形状、图片或图表互相遮挡，线条穿过文字，卡片或标签堆叠。
- 文本溢出或被裁切：靠近页面边缘、文本框边界或卡片边界处被截断。
- 装饰元素位置错误：分割线、强调线或标签底板按单行文字布置，但标题或正文换行后压住文字或距离异常。
- 来源标注、页脚或页码与上方内容碰撞。
- 元素距离过近：相邻元素间距明显不足，卡片或分区几乎贴在一起；按 960x540 画布估算，小于约 15 px 的间隔通常要标记。
- 间距不均：局部留白过大，另一处过于拥挤。
- 页面边距不足：主体内容贴近幻灯片边缘；按 960x540 画布估算，小于约 30 px 的外边距通常要标记。
- 列、卡片、图标或同类元素没有稳定对齐。
- 图片或图表渲染异常：空白、变形、低清、关键内容不可读或预期图形缺失。
- 文本对比度不足，例如浅灰文字放在米色或浅色背景上。
- 图标对比度不足，例如深色图标放在深色背景上，且没有浅色圆形或底板承托。
- 文本框过窄，导致不必要的频繁换行。
- 残留占位符、模板默认文字或未替换内容。

对每一页分别列出发现的问题或可疑区域，即使只是轻微问题也要记录。

报告所有发现的问题，包括轻微问题。
```

必须根据问题严重度决定是否修复：空白页、破图、文字遮挡、明显裁切、低对比不可读、占位符残留等必须先修复再交付；轻微间距或对齐问题如果不修复，最终验证记录要说明已知风险。

## Page Count And Structure

- 实际页数必须等于用户要求或 `slide_plan.json` 的页数。
- 如果创建过程部分失败，先记录已创建的 `xml_presentation_id`，再回读确认哪些页已写入。
- 每页都应包含 `<data>`，且 `<data>` 内至少有一个非背景主体元素。
- 封面、章节页、总结页可以文字较少，但不能只有空背景。
- 技术解释页、对比页、流程页、架构页必须有匹配的结构元素，例如分组框、连线、时间轴、表格或图形化区域。

## Expected Elements

按 `slide_plan.json` 和用户要求逐页核对：

- 标题或主结论存在，并能对应 `key_message`。
- `layout_type` 对应的主要结构已生成。
- `visual_focus` 是页面中最醒目或最大的信息区域之一。
- `text_density` 影响了文本量，没有用长 bullet 框替代规划。
- `asset_need` 有真实素材时已放入正确区域；没有真实素材时，`fallback_if_missing` 已用 XML 形状、线条、标签、表格或图表兜底。

如果用户指定了关键页，例如“架构解释”“Self-Attention 机制解释”“对比或演进视角”“总结页”，最终验证记录必须逐项说明这些页已存在。

## Blank Or Broken Page Signals

把下面情况视为需要修复后再交付：

- `<data/>` 为空，或只有背景、装饰线、空 `<content/>`。
- 关键文本没有出现在回读 XML 中。
- 图片仍是 `@./path`，或 `<img src>` 是 http(s) 外链。
- 页面依赖的图片区域为空，且没有 fallback visual。
- 返回 XML 缺页、页序明显错误，或某页内容被 shell 截断。
- 大量形状坐标完全相同，导致主体内容重叠。
- 渐变背景回退成空白或白底，导致文字不可读。

## Layout And Overflow Risk

优先修复这些明显风险：

- 正文或标签框高度不足，文本很可能被截断。
- 多个主体元素在同一区域重叠，而不是有意叠加背景。
- 标题、标签、关键数字或相邻文本虽未几何重叠，但视觉间距过近，显得粘连、像重叠或破坏层级。
- 重要内容越过画布边界，或贴近底部超过 `y=500`。
- 高密度页使用单个长 bullet list，没有分栏、表格或分组。
- 标题、主视觉、正文的字号和颜色差异太弱，视觉层级不清。
- 所有内容页都是同一套标题加 bullets 坐标。

## Screenshot Visual Review

截图 review 是静态 XML 检查之后的第二道门。它用服务端真实渲染结果发现 XML 无法可靠判断的问题，例如文字截断、图片裁切、图表压盖和弱对比。

每页按以下检查项记录结论；页面含图表时，额外检查图表精确可读性：

| 项目 | Pass 标准 | Fix 信号 |
|---|---|---|
| 可读性 | 标题、正文、标签和关键数字可读；对比度足够，文本层级之间有清楚的视觉间距 | 文字截断、字号过小、低对比、关键标签不可读，或相邻文字间距过近而视觉粘连 |
| 布局 | 主体未被意外遮挡，页边距和底部留白合理 | 重叠、越界、图片裁切、元素贴边、底部拥挤，或文字虽未相交但视觉上像碰撞 |
| 视觉层级 | 主结论、主视觉、支撑信息一眼可区分 | 所有元素同权重、主视觉过小、页面退化为文字堆叠 |
| 内容完整性 | 无空白、破图、占位符或错误页序；图示表达与页面角色匹配 | 空白/破损页、缺失图片、遗留模板文案或与计划不符 |
| 图表精确可读性（有图表时） | 若页面结论依赖精确比较、排序或阈值判断，读者可直接获得每个关键数据点的值：柱/线/饼图有直接数据标签，或有与图表一一对应的等价数据表/注释 | 只能靠坐标轴估读关键数值、缺少决定结论的数据标签、图例与系列无法对应；仅用于展示趋势且不承载精确结论的图表可不强制逐点标签 |

图表检查先问“页面是否要求读者作精确判断”：

- **需要**：比较群体得分、排名、是否达到阈值、预算/目标差异、需要从图中选方案。没有直接数值或等价数据表即为 `fix`。
- **不需要**：只表达上升/下降趋势、定性分布或结构关系，且标题/正文已经明确结论；可不逐点展示数值，但仍须检查轴、图例、系列和关键标注是否可读。

推荐把记录保存在 `.lark-slides/review/<deck-or-task-id>/visual-review.md`：

```text
| slide_id | screenshot | status | findings | action |
|---|---|---|---|---|
| p001 | screenshots/p001.png | pass | hierarchy and contrast clear | - |
| p002 | screenshots/p002.png | fix | bottom labels are clipped | enlarge text box, then rescreenshot |
```

只有所有目标页均为 `pass` 才能写“已完成视觉 review”。截图不可用时沿用上文的 `not_verified` 状态，并说明原因。

## Verification Record

最终回复必须包含简短验证记录，建议格式：

```text
验证记录：
- 回读：已执行 slides +xml-get，实际页数 N / 预期 N。
- 关键页：架构解释 / Self-Attention / 对比或演进 / 总结页均存在。
- 结构：检查了主要 shape/img/table/chart 元素，无明显空白页或破损页。
- 静态检查：xml_text_overlap_lint error_count=0；已检查标题层级、主视觉、重叠/越界/文本溢出风险。
- 视觉 review：已查看 N/N 张服务端截图，全部 pass；或 `not_verified`（截图不可用，原因：...）。
```

不要声称完成了人工视觉验收，除非确实打开或获取了可视化结果。仅从 XML 静态检查得出的结论，应表述为“静态检查未发现明显问题”。

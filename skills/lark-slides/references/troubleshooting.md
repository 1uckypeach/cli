# 排障

出错时怎么定位与修复。正常验证清单见 `verify.md`，命令与参数用法见 `cli-operations.md`，XML 语法见 `xml-protocol.md`。

## 创建前自检

生成 XML、发请求前先过一遍，多数失败能在这一步拦住：

- **转义**：文本 `Q&A → Q&amp;A`、`<` / `>` → `&lt;` / `&gt;`；属性 URL `a=1&b=2 → a=1&amp;b=2`。
- **结构合法**：标签闭合、属性引号成对、`<content>` 结构完整、`<slide>` 直接子元素合法。
- **shell 截断**：`--slides '[...]'` 长参数 / 嵌套引号易被截断；复杂 XML 用 `jq --rawfile` 组装免转义，或直接走两步创建。
- **图片 token**：`<img src>` 必须是 `+media-upload` 拿到的 `file_token`；`@path` 占位符只在 `+create --slides` 中替换，直接调 `slide.create` 不替换；禁外链 URL。
- **坐标越界**：所有元素坐标限 960×540 画布内，越界会破图或报结构错误。

## 提交前 overlap lint

提交整页 XML 前本地必须跑，`summary.error_count == 0` 才能调接口：

```bash
python3 scripts/xml_text_overlap_lint.py --input <presentation-or-slide>.xml
```

它查 XML well-formed、SXSD tag/attr 支持、IconPark 类型与填充可见性、文本明显重叠、whiteboard 与外部 sibling 边界重叠；**不查**越界、文本高度不足、图文压盖、底部拥挤——这些靠 `layout.md` 与回读把关。

| code | 含义 → 处理 |
|---|---|
| `xml_not_well_formed` | 语法/转义错 → 修标签闭合、引号、`&`/`<`/`>` 转义 |
| `sml_prefixed_tag` | SML 用了命名空间前缀 → 用默认 xmlns 或无前缀标签 |
| `sxsd_unsupported_tag` | 不支持的标签 → 按 hint 换（`textbox→<shape type="text">`、`image→<img>`） |
| `sxsd_unsupported_attr` | 不支持的属性 → 按 hint 换（`x→topLeftX`、`fontColor→color`） |
| `iconpark_unsupported_icon_type` | iconType 不在名单 → 用 `iconpark_tool.py` 重搜 |
| `icon_missing_fill_color` / `icon_transparent_fill_color` | icon 无/透明填充 → 给非透明 `fillColor` |
| `bbox_overlap` | 文本框估算区域重叠 → 拉开坐标 / 缩框 / 改分栏 |
| `whiteboard_external_overlap` | whiteboard 与外部元素跨界重叠 → 按 hint 缩小/移动；接受风险则须截图 QA |

## 失败处理顺序

不假设任何操作原子成功（`+create --slides`、`+replace-pages` 均非原子，中途失败已建内容保留）。遇到 `invalid param` / 某页失败 / 空白 / 布局错乱时：

1. 先找是否已有 `xml_presentation_id`（成功 stdout、错误 hint、用户链接、已存上下文）；没有 ID 就不回读，直接按当前错误处理。
2. 有 ID 就 `slides +xml-get` 回读，确认演示文稿是否存在、是否已部分写入、还是空 presentation。
3. 只修出问题的局部，不重建整份：用 `+replace-slide` 改坏页 / 补漏页，别用 `+create` 另起链接。
4. 疑似 shell 截断时切两步创建：先 `+create` 建空白，再 `slide.create` 逐页添加。

## 错误码

| 码 / 信号 | 含义 | 对策 |
|---|---|---|
| 3350001 | XML 非 well-formed / 结构不符 / `block_id` 不存在 | 先查未转义字符与结构；replace 场景重新 `slide.get` 回读拿最新 3 位 short id 再填 |
| 3350002 | `revision_id` 大于当前版本（不存在） | 用 `-1` 取最新，或回读拿实际 `revision_id` |
| 400 XML 格式错误 | 标签 / 引号 / 转义问题 | 按创建前自检逐项排查 |
| 400 请求包装错误 | `--data` 未按 schema 包装 | 确认含 `xml_presentation.content` 或 `slide.content` |
| 403 权限不足 | 身份 / scope 不匹配 | 先查是否误用 bot，再确认 scope 与文档编辑权限 |
| 404 演示文稿不存在 | `xml_presentation_id` 错或无权限 | 检查 token；wiki URL 先解析真实 `obj_token` |
| 404 幻灯片不存在 | `slide_id` 错 | 回读 presentation / slide 确认最新 ID |
| 400 无法删除唯一幻灯片 | 至少保留一页 | 先建新页再删旧页 |
| 1061002 | 媒体上传 params error | 用 `slides +media-upload`，勿手拼 `medias/upload_all`；唯一可用 `parent_type` 是 `slide_file` |
| 1061004 | forbidden，当前身份对 PPT 无编辑权限 | 确认 user/bot 对目标有编辑权限；bot 常见于 PPT 非其所建 |
| 1061044 / unsafe file path | `--file` 给了绝对路径或 `../` 上层路径 | `--file` 须 CWD 内相对；先 `cd` 到素材目录 |

## 现象 → 原因 → 对策

| 现象 | 原因 | 对策 |
|---|---|---|
| 页面大面积空白 | 内容未写入，或间距过大 | 先回读确认是否写入；已写入则缩间距 / 加主体元素 |
| 图片破图 / 不显示 | `src` 写了外链 URL 或仍是 `@path` | 换成 `+media-upload` 的 `file_token` |
| 图片被裁掉 | `width:height` 与原图比例不符 | 对齐原图比例（`<img>` 尺寸即裁剪后尺寸） |
| 新插入 `<img>` 挡住原元素 | 坐标压到已有块 | `slide.get` 对照已有块坐标挑空白位；空间不够就同批 `--parts` 先移 / 缩现有块再插 |
| 文本溢出 / 看不全 | shape 太小或文本过多 | 增大 `width` / `height`，或减文本量 |
| 元素重叠 | 坐标冲突 | 调 `topLeftX` / `topLeftY` 拉开间距 |
| 文字看不清 | 与背景色太近 | 深底浅字、浅底深字 |
| 渐变背景回退白色 | 渐变格式不合规 | 用 `rgba()` + 百分比停靠点，如 `linear-gradient(135deg,rgba(30,60,114,1) 0%,rgba(59,130,246,1) 100%)` |
| 新页跑到末尾 | `before_slide_id` 放进了 `--params` | 原生调用须放 `--data` body 与 slide 同级，放 `--params` 会被当未知 query 静默忽略 |
| 图表不显示 | 缺 `chartPlotArea` / `chartData`，或维度数不匹配 | 补齐两者，核对 `dim1` / `dim2` 数据数量 |
| 表格列宽不合理 | `col` 宽度不当 | 调 `colgroup` 中 `col` 的 `width` |

批量 `--parts` 任一条失败整批不生效（返 3350001）；若响应带 `failed_part_index` / `failed_reason`，shortcut 原样透传，据此定位坏条目。

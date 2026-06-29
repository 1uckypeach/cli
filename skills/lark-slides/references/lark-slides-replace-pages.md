# slides +replace-pages（页面级重建）

批量替换已有演示文稿里的一个或多个页面，保持原 `xml_presentation_id` 和原 Slides 链接不变。适合导入 PPTX/PDF 后二次创作、保留模板素材、版式大改、坐标重排、整页视觉重建；单个文本框、图片或 shape 的小型局部编辑才考虑 [`+replace-slide`](lark-slides-replace-slide.md)。

> 重要：这是多步编排，不是后端原子事务。CLI 对每页执行“先创建新页到旧页前，再删除旧页”；创建失败时旧页会保留。删除失败时可能出现新旧页同时存在，需要按返回结果继续处理。

> 命令名以仓库代码为准：当前 shortcut 是 `slides +replace-pages`（复数）。即使只替换一页，也传一个包含 1 个 item 的 `pages` 数组。

## 命令

```bash
lark-cli slides +replace-pages \
  --as user \
  --presentation <slides_url_or_xml_presentation_id> \
  --pages @pages.json
```

## 参数

| 参数 | 必需 | 说明 |
|------|------|------|
| `--presentation` | 是 | `xml_presentation_id`、`/slides/` URL 或 `/wiki/` URL |
| `--pages` | 是 | JSON 数组，每项包含 `slide_id` 和 `content`；支持 literal、`@file`、stdin `-` |
| `--dry-run` | 否 | 基于 `slide_id` 输入输出替换计划，不执行 create/delete |
| `--continue-on-error` | 否 | 默认失败即停；开启后继续处理后续页，并在结果中标记失败项 |
| `--validate-only` | 否 | 只校验输入并生成替换计划，不执行 Slides get/create/delete |

## pages.json

```json
[
  {
    "slide_id": "slide_short_id_1",
    "content": "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\"><data></data></slide>"
  },
  {
    "slide_id": "slide_short_id_2",
    "content": "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\"><data></data></slide>"
  }
]
```

规则：

- 每项必须提供 `slide_id`；不支持 `slide_number`。
- `content` 必须是完整 `<slide>...</slide>` XML。
- 同一批次不能重复 `slide_id`。
- CLI 不会回读整份 presentation；如果 `slide_id` 已失效，create/delete 阶段会返回对应错误。

## 保留导入模板素材

导入 PPTX/PDF 或改写已有 Slides 时，先用 `xml_presentations.get` 保存当前 XML，再为每页盘点素材。源素材默认锁定保留，不是可随意替换的装饰：

- `<style>`：页面背景、渐变、底色或图片底纹。
- `<img src="...">`：同一个 `xml_presentation_id` 内的 file token 可直接复制到新页 XML。
- `<chart>` / `<table>`：优先保留原结构；只在用户要求时改数据或标签。
- `<whiteboard>`：可保留外层位置；注意回读 XML 可能不包含内部 SVG/Mermaid。
- 关键 shape/motif：侧边栏、分节条、卡片底、编号徽章、分割线等模板视觉语言。

在 `slide_plan.json` 中把这些事实写入 `source_asset_inventory`，并在每页 `rewrite_contract.must_reuse` 中绑定要保留的 locked assets。生成 replacement `content` 时，顺序必须是：

1. 先复制源页 `<style>` 和 locked assets。
2. 再替换模板占位文案和必要布局。
3. 最后补充新业务图形。

不要从空 `<slide>` 重画后再按感觉补素材。不要把导入底稿当成普通参考后重新 `slides +create` 一份脱离素材的新 deck。

`discarded_blocks` 只能逐块删除源元素，必须写 `type`、`id` 和原因。`discarded_blocks.type = "all"` 默认非法；只有用户明确要求"不保留模板素材"或"只参考风格重做"时，才允许 `rewrite_mode: "style_reference_only"`。

## Dry Run

```bash
lark-cli slides +replace-pages --as user \
  --presentation "$PID" \
  --pages @pages.json \
  --dry-run
```

输出包含 `xml_presentation_id`、`pages_count`、`plan`，以及每页的 `old_slide_id`、`insert_before_slide_id` 和动作 `create_before_then_delete_old`。Dry-run 只基于输入的 `slide_id` 构造计划，不会调用 `xml_presentations.get`，也不会执行 create/delete。

## 成功输出

```json
{
  "xml_presentation_id": "xxx",
  "pages_count": 2,
  "status": "completed",
  "summary": {
    "replaced": 2,
    "failed": 0,
    "total": 2
  },
  "results": [
    {
      "old_slide_id": "old3",
      "new_slide_id": "new3",
      "status": "replaced"
    }
  ],
  "revision_id": 123
}
```

如果使用 `--continue-on-error` 且任一页面失败，CLI 会继续处理后续页，但最终以 partial failure 非零退出；stdout 仍保留完整 `results`，顶层 `ok` 为 `false`，`status` 为 `partial_failure`。

`status` 可能为：

- `replaced`：新页创建成功，旧页删除成功。
- `create_failed`：新页创建失败，旧页保留。
- `delete_failed`：新页已创建，但旧页删除失败。

## 使用建议

1. 大幅改写前先 `xml_presentations.get` 保存当前 XML，并记录要替换页面的 `slide_id`。
2. 导入模板二创时，先在 plan 中记录每页要复用的背景、图片 token、图表、表格和 motif，再生成完整新页 XML。
3. 生成只含 `slide_id` 和完整 `<slide>` content 的 `pages.json` 后先跑 `--dry-run` 或 `--validate-only`。
4. 默认不要开 `--continue-on-error`，除非能接受部分页面已替换。
5. 替换后再回读全文 XML，确认页序、背景、图片、图表、表格、locked motif 和文本没有破损；如果当前账号具备截图能力，可额外截图检查视觉效果。

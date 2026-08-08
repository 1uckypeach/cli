# docs +fetch（读取飞书云文档）

读取整篇文档，或按目录、章节、区间和关键词获取局部内容。

## 常用示例

```bash
# 读取整篇文档
lark-cli docs +fetch --doc "文档URL或token"

# 读取正文，并附带可见的未解决评论；JSON 会同时保留正文和评论 sidecar
lark-cli docs +fetch --doc "文档URL或token" --comments --format json --as bot

# 按 URL 中的 #share 锚点局部读取
lark-cli docs +fetch --doc '文档URL#share-anchor'

# 按关键词定位
lark-cli docs +fetch --doc Z1Fj...tnAc --scope keyword --keyword "部署|发布|上线"

# 先查看目录，再读取指定章节
lark-cli docs +fetch --doc Z1Fj...tnAc --scope outline --max-depth 3
lark-cli docs +fetch --doc Z1Fj...tnAc --scope section --start-block-id blkTitle
```

## 参数

|参数|必填|说明|
|-|-|-|
|`--doc`|是|文档 URL 或 token，支持 `/docx/`、`/wiki/` 和带 `#share-...` 的选区链接|
|`--doc-format`|否|`xml`（默认）\| `markdown` \| `im-markdown`（供后续 `lark-im` 场景使用）|
|`--detail`|否|`simple`（默认）\| `with-ids` \| `full`|
|`--comments`|否|附带机器人身份可见的未解决评论；默认关闭，当前须使用 `--as bot --format json` 保留评论 sidecar|
|`--revision-id`|否|文档版本号；`-1` 表示最新版本（默认）|
|`--scope`|否|`outline` \| `range` \| `keyword` \| `section`；省略则读取整篇|
|`--start-block-id`|否|`range` 的起点，或 `section` 的锚点（`section` 必填）|
|`--end-block-id`|否|`range` 的终点；`-1` 表示读到末尾|
|`--keyword`|否|`keyword` 模式的关键词；支持多级自动匹配和多分支 OR|
|`--context-before`|否|返回命中项之前的顶层兄弟块数量（默认 `0`）|
|`--context-after`|否|返回命中项之后的顶层兄弟块数量（默认 `0`）|
|`--max-depth`|否|`outline` 表示标题层级上限；其它模式表示子树深度（默认 `-1`，不限）|
|`--format`|否|`json`（默认）\| `pretty`|

## 选择详细度：`--detail`

|目的|取值|返回内容|
|-|-|-|
|浏览、总结|`simple`（默认）|简洁 XML/Markdown，不含 block ID、样式和引用元数据|
|定位、跳转|`with-ids`|包含 block ID，可用于 `+update --block-id`，也可拼成 `文档URL#block_id` 直达链接|
|编辑文档|`full`|包含 block ID、样式和引用元数据，保留完整结构信息|

需要修改文档时使用 `full`；只读场景通常不必获取额外元数据。

## 选择读取范围：`--scope`

`--scope` 与 `--detail` 可以组合。优先读取满足任务所需的最小范围；只有确需全文时才省略 `--scope`。

|模式|适用场景|关键参数|返回行为|
|-|-|-|-|
|`outline`|结构未知，先查看目录|`--max-depth`|扁平列出标题；返回的标题 ID 可作为 `section` 或 `range` 的端点|
|`section`|读取某个标题对应的整节|`--start-block-id`（必填）|顶层标题展开到下一个同级或更高级标题之前；容器内节点（含内嵌标题）按最小包容单元返回容器或表格切片|
|`range`|已知精确起止位置|`--start-block-id`、`--end-block-id` 至少一个|同一顶层序列按区间切片；同一容器返回整个容器；同一表格返回瘦身切片；跨顶层时完整返回端点所在的顶层块|
|`keyword`|只有关键词或模糊线索|`--keyword`（必填）|按最小包容单元返回命中；同一容器的多处命中自动去重，同一表格的多行命中合并为切片|

`keyword` 会依次尝试子串、归一化、分词形变和 RE2 正则匹配。多关键词使用 `|` 表示 OR，例如 `部署|发布|上线`；任一分支命中即返回。

范围参数的共同规则：

- `--max-depth`：`outline` 中 `3` 表示列出 h1～h3；其它模式中 `0` 表示仅返回块自身，`-1` 表示不限深度。
- `--context-before` / `--context-after`：仅对完整的顶层块生效。命中位于容器或表格内时会被忽略；如需更大范围，改用 `section` 或 `range`。

推荐选择顺序：

|已知信息|首选方式|后续动作|
|-|-|-|
|具体术语、错误码或标识|`keyword`|上下文不足时，用返回的 `top-block-id` 再执行 `section` 或 `range`|
|章节或标题|`outline --max-depth 3`|获取标题 ID 后执行 `section`|
|精确起止位置|`range`|按需调整端点或深度|
|没有关键词，也不了解结构|`outline`|根据目录转入 `section` 或 `range`|
|确实需要整篇|省略 `--scope`|—|

## 返回值

```json
{
  "ok": true,
  "identity": "user",
  "data": {
    "document": {
      "document_id": "docToken",
      "revision_id": 12,
      "content": "<title>标题</title><p>文档内容...</p>",
      "reference_map": {
        "<block_type>": {
          "<ref>": {
            "<real-attr-key>": "<real-attr-value>"
          }
        }
      },
      "tips": "<safe replay or degradation guidance>"
    }
  }
}
```
`content` 的格式由 `--doc-format` 决定。`reference_map` 是正文引用数据的结构化 sidecar：一级键 `block_type` 表示引用所在的块类型，二级键 `ref` 对应正文中的临时引用；每个引用的值是由 `real-attr-key` 和 `real-attr-value` 组成的真实属性映射，具体属性由块类型决定。没有提取数据时，`reference_map` 可能为空。`content` 和 `reference_map` 属于同一份响应，保留或回放内容时应配套处理。`tips` 给出安全回放或降级提示。`im-markdown` 仅用于获取内容后在 `lark-im` 场景下使用。设置 `--scope` 时会被 `<fragment>` 包裹，详见上文"局部读取的输出结构"。

### 理解 `--comments` 的返回

评论采用紧凑、只读的 AI 上下文，不代替 `drive +list-comments` 等完整评论 API：

- XML 正文中的局部评论落点使用 `comment-refs="c1 c2"`；同一条评论跨多个 block 时会在这些 block 上重复同一个 ref。
- `reference_map.comment.<ref>.data` 保存局部讨论；`reference_map.document-comment.<ref>.data` 保存全文讨论。
- 讨论只表达引用文本、消息、图片占位和 reaction，不返回稳定评论 ID、状态或完整格式。需要继续回复、解决或精确管理评论时，改用 `lark-drive` 评论命令。
- 全文读取返回局部评论和全文评论；`keyword` / `range` / `section` 只返回与片段相交的局部评论，不返回全文评论；`outline` 即使传了 `--comments` 也不查询评论。
- reaction 属于 best-effort 展示信息；当一次读取需要 hydrate 超过 1000 条评论时会整体省略 reaction，以限制下游时延，评论正文和引用关系仍完整返回。
- Markdown / IM Markdown 使用轻量 XML 壳 `<comment-ref refs="c1 c2"/>` 精确标记落点；重复文本、跨 block、列表和表格都不需要依靠 `<quote>` 猜位置。壳中的 ref 与 `reference_map.comment` 一一对应。
- 指定历史 `--revision-id` 时，正文来自该历史版本；评论是“当前仍可见、仍未解决”的快照投影到这份正文。局部评论仅在该 revision 能解析到锚点时返回，全文评论仅在全文读取时返回；它不是历史时刻的评论回放。
- 必须使用 `--format json`；其它展示格式无法无损保留正文与 `reference_map`（例如 `pretty` 只输出正文），因此 CLI 会直接拒绝这些组合。
- 评论或锚点依赖不可用时，正文仍正常返回，评论整体省略，并在 `tips` 中出现 `comments_omitted:<reason>`。

当前服务端能够完整校验 tenant access token（`--as bot`）的评论 scope。user access token 的精确 token scope 尚未由网关可信透传，因此 `--as user` 的评论读取会安全拒绝；完成 OGW 条件鉴权后再开放，不能用普通 user/app/tenant 授权状态代替本 token scope。

```xml
<p comment-refs="c1">评论引用的正文</p>
```

Markdown / IM Markdown 中同一落点写作：

```xml
<comment-ref refs="c1"/>评论引用的正文
```

对应的 `reference_map.comment.c1.data`：

```xml
<discussion timezone="Asia/Shanghai">
<quote>评论引用的正文</quote>
<message t="2026-07-23 16:50" u="曹杰">
问题一：在职转移会删除协作者权限
<img/>
<reaction>👍 方树煜、曹杰</reaction>
</message>
</discussion>
```

### 理解局部读取结果

## 参数

设置 `--scope` 后，`content` 外层是 `<fragment>`，并按需携带 `mode`、`requested-start`、`requested-end` 或 `keyword` 属性。其子节点有两种形式：

- **顶层块**：直接作为 `<fragment>` 的子节点，表示返回了完整块。
- **`<excerpt top-block-id="..." parent-block-path="...">`**：表示只返回了容器或表格中的节选。
  - `top-block-id` 是节选所在的顶层块 ID。需要查看完整块时，可将它作为 `section` 或 `range` 的锚点重新读取。
  - `parent-block-path` 是从顶层块到节选内容直接父节点的 ID 路径，以 `/` 分隔；表格切片中即表格自身 ID。

看到 `<excerpt>` 时，不要假设已经获取了整个顶层块。

表格默认瘦身：即使 `<table>` 本身是顶层块，也只返回表头和命中的行。读取整张表时，使用 `range --start-block-id <table-id> --end-block-id <table-id>`。如果切片覆盖全部数据行，SDK 会自动返回完整表格，不再包裹 `<excerpt>`。

## 处理文档内嵌资源

|返回内容|处理方式|
|-|-|
|`<img>`、`<source>`|有 `url` 时仅下载可信的公开 HTTPS URL：拒绝 userinfo 及解析到 private、loopback、link-local、multicast、unspecified 地址的 host，并逐次校验重定向；不满足时禁止请求。无 `url` 时提取 `token`，预览用 `docs +media-preview`，下载用 `docs +media-download`|
|`<whiteboard>`|提取 `token`，使用 `docs +media-download`|
|`<sheet>`、`<cite file-type="sheets">`|提取 `token` 和 `sheet-id`，转到 [`lark-sheets`](../../lark-sheets/SKILL.md)|
|`<bitable>`、`<cite file-type="bitable">`|提取 `token` 和 `table-id`，转到 [`lark-base`](../../lark-base/SKILL.md)|
|`<vc-transcribe-tab>`|提取 `vc-node-id`，使用 [`lark-note`](../../lark-note/SKILL.md) 的 `note +detail`|
|`<synced_reference>`|提取 `src-token` 和 `src-block-id`，读取源文档并定位 block|

## 参考

- [lark-doc-media-preview](lark-doc-media-preview.md) — 预览素材
- [lark-doc-media-download](lark-doc-media-download.md) — 下载素材或画板缩略图

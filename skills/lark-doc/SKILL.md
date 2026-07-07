---
name: lark-doc
description: "飞书云文档（Docx / Wiki）内容操作：读取、创建、编辑文档，插入或下载图片附件，以及操作思维笔记。用户提供文档 URL/token（包括 doubao.com 的 /docx/、/wiki/）时使用；按 URL 路径/token 而非域名路由。遇到嵌入的电子表格、多维表格或画板，先提取 token，再切到对应 skill。文档评论走 lark-drive；表格或 Base 内部数据操作不在本 skill。"
metadata:
  requires:
    bins: ["lark-cli"]
  cliHelp: "lark-cli docs --help;lark-cli mindnotes --help"
---

# docs

## 按场景读取

**CRITICAL：先判断场景，再读取该场景的参考文件；不要在任务开始时一次性读取全部参考文件。每个文件只在首次进入对应阶段时读取一次。**

- **读取**：先读 [`lark-doc-fetch.md`](references/lark-doc-fetch.md)，再获取或总结文档。
- **创建**：从零创作时，场景判断后必须首先完整读取 [`lark-doc-authoring.md`](references/lark-doc-authoring.md)；读取前禁止起草、选择格式、读取 `lark-doc-create.md` 或执行创建。仅创建空文档、原样导入用户提供的完整内容、机械格式转换可跳过 Authoring。
- **编辑**：语义改写、润色、重组、补写或排版时，必须先完整读取 [`lark-doc-authoring.md`](references/lark-doc-authoring.md)；明确旧文本到新文本的替换、纯删除或纯移动可跳过 Authoring，直接按 [`lark-doc-update.md`](references/lark-doc-update.md) 操作。

**身份：文档操作默认使用 `--as user`。首次使用前执行 `lark-cli auth login`。**

```bash
# 常用示例
lark-cli docs +fetch --doc "文档URL或token；若 URL 存在 #share-... 锚点，优先使用锚点方式读取，不要全文拉取"
lark-cli docs +create --doc-format xml --content '<title>标题</title><p>内容</p>'
lark-cli docs +update --doc "文档URL或token" --command append --doc-format xml --content '<p>内容</p>'
```

> **格式选择规则（全局）：**
> - **语义创作 / 全文重建：**默认且显式使用 XML，只读取 [`lark-doc-xml.md`](references/lark-doc-xml.md)。`Presentation Decision` 只确定 `presentation_mode`，不预先选择 block；生成后通过 Draft Parse Gate 的 `profile.blocks` 检查实际组件是否符合 content contract 与可选 platform adapter。选择 XML 不授权增加无必要的 rich block。
> - **Markdown 例外：**仅当用户明确要求 Markdown，或原样导入 / 保真重建用户提供的 `.md` 内容时，改为只读取 [`lark-doc-md.md`](references/lark-doc-md.md) 并全程使用 Markdown。必要能力无法承载时先说明冲突，不擅自混用或降级。
> - **精准编辑：**默认使用 XML 并保留未授权内容；只有跨行 `str_replace` 等明确依赖 Markdown 的单次操作才切换为 Markdown。每份草稿、每个 `--content` / `--pattern` payload 只能使用一种语法，禁止 Markdown 与 XML 混写；delete / move / copy 不写内容。

## 快速决策
- 用户要**复制文档 / 创建文档副本 / 另存为副本**时，切到 [`lark-drive`](../lark-drive/SKILL.md)，按其中的复制指引使用 `lark-cli drive files copy`；不要用 `docs +fetch` + `docs +create` 重建正文，也不要走 `drive +export` / `drive +import`。
- 先判定任务路径：找文档 / 导入导出走 [`lark-drive`](../lark-drive/SKILL.md)；只读 / 摘要用 `docs +fetch` 默认 `simple`；已有文档改写按 [`lark-doc-update.md`](references/lark-doc-update.md) 的 Observe-Diagnose-Patch Loop 先 fetch 再局部 patch；明确旧文本 → 新文本的简单替换可直接 `str_replace`，但写后必须 fetch 验证；只有 block 链接、评论锚点、插入 / 替换 / 删除 / 移动才局部 fetch `with-ids`；保真改写已有内容才读 `full`
- block 直达链接格式：`文档基础 URL#block_id`；没有 block_id 时局部 fetch `with-ids`
- 连续执行多个文档写操作时，必须按 [`lark-doc-update.md`](references/lark-doc-update.md) 的「Block ID 生命周期」处理：每次更新后都按 block ID 已变更处理；需要继续或重复修改时，先重新 fetch 最新内容和 block ID，不要复用旧 fetch 结果
- 用户需要在文档内**创建、复制或移动**资源块（画板、电子表格、多维表格等）时，必须先读取 [`lark-doc-xml.md`](references/lark-doc-xml.md) 的「三、资源块」章节
- 写文档时，由内容和用户意图决定表达形式；流程、架构、路线图、关键指标等信息可以使用画板，但不要默认把重要信息都画板化
- 新增画板按复杂度处理：简单 Mermaid / SVG 图可由主 Agent 直接写入草稿；复杂图或需要专门视觉设计的 SVG 交给 SubAgent 产出完整 `<whiteboard type="svg">...</whiteboard>`；特别复杂或已有画板更新，主 Agent 先建 `<whiteboard type="blank"></whiteboard>`，再启动 SubAgent 读取 `lark-whiteboard` 写入
- 用户说"看一下文档里的图片/附件/素材""预览素材" → 用 `lark-cli docs +media-preview`
- 用户明确说"下载素材" → 用 `lark-cli docs +media-download`
- 用户想把文档回滚到某个 `revision_id` 或某一时刻 → 先读 [`lark-doc-history.md`](references/lark-doc-history.md)，按其中流程操作
- 用户明确说"下载/更新/删除文档封面图" → 用 `lark-cli docs +resource-download/+resource-update/+resource-delete --type cover`
- `resource-*` 目前仅支持 Docx 封面资源；其他图片、附件或素材请走 `+media-*`
- 如果目标是画板/whiteboard/画板缩略图 → 只能用 `lark-cli docs +media-download --type whiteboard`（不要用 `+media-preview`）
- 用户明确要操作思维笔记时；已有**思维笔记**，走 [思维笔记链路](references/lark-doc-mindnote.md)；新建**思维笔记**，走 [lark-doc-whiteboard](references/lark-doc-whiteboard.md)
- 拿到 spreadsheet URL/token 后 → 切到 `lark-sheets` 做对象内部操作
- 用户需要解析 XML / Markdown、把 Markdown 转成 XML，或统计文档的**总字数 / 总字符数**时，读取 [`lark-doc-script.md`](references/lark-doc-script.md)。
- 用户说"给文档加评论""查看评论""回复评论""给评论加/删除表情 reaction" → 切到 `lark-drive` 处理
- 文档内容中出现嵌入的 `<sheet>`、`<bitable>` 或 `<cite file-type="sheets|bitable">` 标签时 → **必须主动提取 token 并切到对应技能下钻读取内部数据**，不能只呈现标签本身

| 标签 / 属性 | 提取字段 | 切到技能 |
|-|-|-|
| `<sheet token="..." sheet-id="...">` | `token` -> spreadsheet_token, `sheet-id` | [`lark-sheets`](../lark-sheets/SKILL.md) |
| `<bitable token="..." table-id="...">` | `token` -> app_token, `table-id` | [`lark-base`](../lark-base/SKILL.md) |
| `<cite type="doc" file-type="sheets" token="..." sheet-id="...">` | 同 `<sheet>` | [`lark-sheets`](../lark-sheets/SKILL.md) |
| `<cite type="doc" file-type="bitable" token="..." table-id="...">` | 同 `<bitable>` | [`lark-base`](../lark-base/SKILL.md) |
| `<vc-transcribe-tab vc-node-id="...">` | `vc-node-id` -> note_id | [`lark-note`](../lark-note/SKILL.md)：先 `note +detail --note-id <vc-node-id>` |
| `<synced_reference src-token="..." src-block-id="...">` | `src-token` -> doc_token, `src-block-id` -> block_id | 用 `docs +fetch` 读取 src-token 文档，定位 block |

## Shortcuts（推荐优先使用）

Shortcut 是对常用操作的高级封装（`lark-cli docs +<verb> [flags]`）。有 Shortcut 的操作优先使用。

| Shortcut | 说明 |
|----------|------|
| [`+create`](references/lark-doc-create.md) | Create a Lark document (XML / Markdown) |
| [`+fetch`](references/lark-doc-fetch.md) | Fetch Lark document content (XML / Markdown / im-markdown; `im-markdown` only after fetch for `lark-im`) |
| [`+update`](references/lark-doc-update.md) | Update a Lark document (str_replace / block_insert_after / block_replace / ...) |
| [`+script`](references/lark-doc-script.md) | Parse XML or Markdown into a local profile, or convert Markdown to XML |
| [`+history-list` / `+history-revert` / `+history-revert-status`](references/lark-doc-history.md) | List document history, revert to a `history_version_id`, and query revert task status |
| [`+media-insert`](references/lark-doc-media-insert.md) | Insert a local image or file at the end of a Lark document (4-step orchestration + auto-rollback). Prefer `--from-clipboard` when the image is already on the system clipboard (screenshots, copy from Feishu/browser); use `--file` only for on-disk sources. |
| [`+media-download`](references/lark-doc-media-download.md) | Download document media or whiteboard thumbnail (auto-detects extension) |
| [`+media-preview`](references/lark-doc-media-preview.md) | Preview document media file (auto-detects extension) |
| [`+resource-download` / `+resource-update` / `+resource-delete`](references/lark-doc-resource-cover.md) | Download, update, or delete a Docx cover image resource with `--type cover` |
| [`+whiteboard-update`](../lark-whiteboard/references/lark-whiteboard-update.md) | Alias of `whiteboard +update`. Update an existing whiteboard with DSL, Mermaid or PlantUML. Prefer `whiteboard +update`; refer to lark-whiteboard skill for details. |

## 不在本 Skill 范围

- 文档评论管理 → [`lark-drive`](../lark-drive/SKILL.md)
- 电子表格或 Base 的数据操作 → [`lark-sheets`](../lark-sheets/SKILL.md) / [`lark-base`](../lark-base/SKILL.md)
- 云空间文件上传、下载、权限管理 → [`lark-drive`](../lark-drive/SKILL.md)

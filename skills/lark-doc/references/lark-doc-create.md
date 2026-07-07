# docs +create（创建飞书云文档）

从 XML（默认）或 Markdown 内容创建一个新的飞书云文档；语义创作默认使用 XML，只有 Authoring 明确判定为 Markdown 例外时才使用 Markdown。

> **职责与前置条件：**本文件只负责创建操作。语义创作必须已完成 [`lark-doc-authoring.md`](lark-doc-authoring.md) 的 Prepare 和 Draft 阶段，且 `Publish Gate = ready`；空文档或原样导入可由根路由直接进入。首次执行 `lark-cli` 前只再读取 [`lark-shared`](../../lark-shared/SKILL.md)。

## 命令

```bash
# 先在当前工作目录下创建任务独占、名称唯一的临时 XML 文件，并写入已验收草稿
lark-cli docs +create --doc-format xml --content "@<任务独占目录>/<唯一文件名>.xml"
```

单次内容优先使用 `--content -` 从 stdin 读取。必须使用 `@file` 时，在当前工作目录下创建任务独占目录，并为每篇文档创建名称唯一的临时 XML 文件；明确命中 Markdown 例外时才创建临时 Markdown 文件。不得复用固定草稿名、已存在文件或其他任务的目录。`@file` 只接受当前工作目录下的相对路径，且参数必须整体加引号，例如 `"@<任务独占目录>/<唯一文件名>.xml"`；不要传绝对路径。完成 Deliver 的传输验证后，只清理本任务创建的文件和目录，不得使用通配符清理。

## 返回值

```json
{
  "ok": true,
  "identity": "user",
  "data": {
    "document": {
      "document_id": "docx_token",
      "revision_id": 1,
      "url": "https://xxx.feishu.cn/docx/docx_token",
      "new_blocks": [
        { "block_id": "blkcnXXXX", "block_type": "whiteboard", "block_token": "boardXXXX" }
      ]
    }
  }
}
```

- **`document.new_blocks`**：本次操作新增的 block 列表（如画板）。`block_id` 可用于 `docs +update` 的 `--block-id` 做精确编辑；`block_token` 是资源块（如画板）的 token，可交给 `lark-whiteboard` 等 skill 继续操作

## 结果处理与退出条件

1. 检查命令是否成功、业务结果是否成功，并逐项处理 `warnings` / `degrade_details`；不得只看到文档 URL 就宣布完成。
2. 读取 [`lark-doc-fetch.md`](lark-doc-fetch.md)，fetch 新文档的最小充分范围，与通过 Publish Gate 的版本比较，仅验证内容被完整传输、资源未丢失且降级已处理。
3. 传输不一致或存在未处理降级时，基于 fetch 结果修复并重新验证；实际结果与已批准版本一致后才结束。

> \[!IMPORTANT]
> 如果文档是**以应用身份（bot）创建**的，如 `lark-cli docs +create --as bot` 在文档创建成功后，CLI 会**尝试为当前 CLI 用户自动授予该文档的 `full_access`（可管理权限）**。
>
> 以应用身份创建时，结果里会额外返回 `permission_grant` 字段，明确说明授权结果：
> - `status = granted`：当前 CLI 用户已获得该文档的可管理权限
> - `status = skipped`：本地没有可用的当前用户 `open_id`，因此不会自动授权；可提示用户先完成 `lark-cli auth login`，再让 AI / agent 继续使用应用身份（bot）授予当前用户权限
> - `status = failed`：文档已创建成功，但自动授权用户失败；会带上失败原因，并提示稍后重试或继续使用 bot 身份处理该文档
>
> `permission_grant.perm = full_access` 表示该资源已授予”可管理权限”。
>
> **不要擅自执行 owner 转移。** 如果用户需要把 owner 转给自己，必须单独确认。

## 参数

| 参数                  | 必填 | 说明                                          |
| ------------------- | -- |---------------------------------------------|
| `--title`           | 否  | 文档标题，Markdown 导入时使用；XML 创建推荐在 `--content` 开头写 `<title>...</title>`；多个标题仅保留第一个并在 `warnings` / `degrade_details` 提示 |
| `--content`         | 视情况 | 文档内容（XML 或 Markdown 格式）；不传 `--content` 时必须传 `--title` |
| `--reference-map` | 否 | 结构化 `reference_map` JSON object；必须与 `--content` 一起使用。普通写入优先把结构写在正文里；该参数主要用于保留或回放已有 `document.reference_map`。支持直接 JSON、任务独占目录内的相对 `@file`，或 `-` 从 stdin 读取。 |
| `--doc-format`      | 否  | CLI 与语义创作均默认 `xml`，并建议显式传入；仅用户明确要求 Markdown 或保真导入 Markdown 时使用 `markdown`。单次内容禁止混用两种语法。 |
| `--parent-token`    | 否  | 父文件夹或知识库节点 token（与 `--parent-position` 互斥）  |
| `--parent-position` | 否  | 父节点位置，如 `my_library`（与 `--parent-token` 互斥） |

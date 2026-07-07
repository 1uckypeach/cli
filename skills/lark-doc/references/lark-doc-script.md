# `docs +script`

`docs +script` 在本地解析文档内容或转换格式，不发起 OpenAPI 请求，也不修复输入。`--content` 支持字面内容、`@当前目录下的相对路径` 和 `-`（stdin）；长内容优先使用 `@file` 或 stdin。`--format json` 只控制 CLI 输出格式，不表示输入格式。

## 解析 XML 或 Markdown

使用同一条 `parse` 指令。shortcut 根据内容自动识别 XML 或 Markdown，调用方不需要判断或声明输入格式：

```bash
lark-cli docs +script --command parse --content "@document.xml" --format json
lark-cli docs +script --command parse --content "@document.md" --format json
```

统计已存在的在线文档时，先读取完整 XML，把返回的 `data.document.content` 写入当前工作目录下任务独占、名称唯一的临时 XML 文件，再解析该文件：

```bash
lark-cli docs +fetch --doc "<文档 URL 或 token>" --doc-format xml --detail full --format json
lark-cli docs +script --command parse --content "@<任务独占目录>/<唯一文件名>.xml" --format json
```

XML 输入执行严格解析；不完整标签、错误嵌套、非法属性、未知实体或不支持的 LarkOpenCLI 标签会返回非零退出码。Markdown 输入按 LarkOpenCLI Markdown 语义解析。资源块内部未出现在输入文本中的内容不计入字数或字符数。

成功时 `data` 只包含 `profile`：

```json
{
  "data": {
    "profile": {
      "word_count": 10,
      "char_count": 15,
      "block_count": 2,
      "blocks": [
        {"type": "p", "count": 1, "ratio": 0.5},
        {"type": "title", "count": 1, "ratio": 0.5}
      ]
    }
  }
}
```

- `data.profile.word_count`：语义字数。统计汉字、英文单词 / URL / code path、数字、中文标点和独立可见符号；英文单词内部按一个语义单位计算。
- `data.profile.char_count`：可见字符数，不含空格；统计汉字、英文字母、数字、中英文标点和可见符号，非 BMP 符号按 UTF-16 code unit 计算。
- `data.profile.block_count`：block 总数。
- `data.profile.blocks[]`：每种 block 的 `type`、`count` 和 `ratio`；`ratio = count / block_count`。

## Markdown 转 XML

`markdown-to-xml` 只负责把 Markdown 转成 LarkOpenCLI XML：

```bash
lark-cli docs +script --command markdown-to-xml --content "@document.md" --format json
```

成功时 `data` 只包含转换结果：

```json
{
  "data": {
    "xml": "<h1>标题</h1><p>正文</p>"
  }
}
```

该指令不返回 `profile`。需要统计原 Markdown 时，独立执行 `--command parse`。

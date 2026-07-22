基于 HTML 子集的 XML 格式描述飞书文档内容。

# 一、标准 HTML 标签
p, h1-h9, ul, ol, li, table, thead, tbody, tr, th, td, blockquote, pre, code, hr, img, b, em, u, del, a, br, span 语义不变

## 正文文本转义

标签保持原样，只转义标签内部的正文文本。正文中的 `<` 和 `&` **必须**分别写成 `&lt;` 和 `&amp;`，否则 XML 无法解析；`>` 通常可以原样保留，但为了生成规则一致也可以写成 `&gt;`。换行使用 `<br/>`。

```xml
<!-- ❌ 把标签本身转义了 -->
&lt;p&gt;内容&lt;/p&gt;

<!-- ❌ 正文中的 < 和 & 没有转义 -->
<p>A & B，且 1 < 2</p>

<!-- ✅ 标签原样，正文字符转义 -->
<p>A &amp; B，且 1 &lt; 2</p>
```

## 常见的不支持标签及替代方案

只使用上面的白名单；不要把“HTML 子集”理解成完整 HTML。

| 不支持写法 | 替代方案 |
|-|-|
| `<strong>` | `<b>` |
| `<i>` | `<em>` |
| `<sub>` / `<sup>` | 简单上下标使用 Unicode（如 H₂O、x²）；复杂公式使用 `<latex>` |
| `<font color="...">` | `<span text-color="...">` |
| `<div>` / `<section>` / `<article>` | 使用段落与标题组织内容 |

不支持的标签会被降级或产生 `4007_unsupported_tag` warning。

# 二、扩展标签速查表
## 块级标签
|标签|说明|关键属性|
|-|-|-|
| `<title>` | 文档标题（每篇唯一）| `align` |
| `<checkbox>` | 待办项| `done="true"\|"false"` |

## 容器标签
|标签|说明|关键属性|
|-|-|-|
| `<callout>` | 高亮框；直接子节点必须是受支持的文本块、标题、列表、待办或引用，不能放裸文本 | `emoji`(默认 bulb), `background-color`, `border-color`, `text-color` |
| `<grid>` + `<column>` | 分栏布局，各列 width-ratio 之和为 1 | `width-ratio` |
| `<whiteboard>` | 嵌入画板 | `type`: `blank` \| `mermaid` \| `plantuml` \| `svg` |
| `<pre>` | （代码块，内含 `code`）| `lang`, `caption` |
| `<figure>` | 视图容器 | `view-type` |
| `<bookmark>` | 书签链接 | `<bookmark name="标题" href="https://..."></bookmark>`，必传 name 和 href |

## 行内组件
| 标签 | 说明 | 关键属性 |
|-|-|-|
| `<cite type="user">` | @人 | XML 导入时必须显式传入 `user-id`：`<cite type="user" user-id="userID"></cite>` |
| `<cite type="doc">` | @文档 | `<cite type="doc" doc-id="docx_token"></cite>` |
| `<latex>` | 行内公式 | `<latex>E = mc^2</latex>` |
| `<img>` | 图片（可独立成块或内联） | `<img width="800" height="600" caption="说明" name="图.png" href="http 或 https"/>` |
| `<source>` | 文件附件（可独立成块或内联） | `<source name="报告.pdf"/>` |
| `<a type="url-preview">` | 预览卡片 | `<a type="url-preview" href="...">标题</a>` |
| `<button>` | 操作按钮 | `background-color`、`src`，必须包含 `action=OpenLink\|DuplicatePage\|FollowPage` |
| `<time>` | 提醒 | 必包含 `expire-time`、`notify-time`（毫秒时间戳）、`should-notify=true\|false` |

## 文本块通用属性
- `align` — `"left"`|`"center"`|`"right"`（适用于 p / h1-h9 / li / checkbox）
- 有序列表项用 `seq="auto"` 自动编号

### callout 内容边界

- 文本必须放在 `<p>`、标题、列表、`<checkbox>` 或 `<blockquote>` 等块中，不能直接写裸文本。
- 表格、图片、代码块、分栏和画板不要嵌套进 `<callout>`；需要强调时，在资源块前放一个短 callout，再把资源块作为相邻顶层块写入。
- 列表使用完整的 `<ul><li>...</li></ul>` 或 `<ol><li seq="auto">...</li></ol>`，不要把孤立 `<li>` 直接放入 callout。

```xml
<!-- ❌ 裸文本 -->
<callout>重要提示</callout>

<!-- ✅ 文本块 -->
<callout><p>重要提示</p></callout>

<!-- ✅ 表格与 callout 相邻，而不是互相嵌套 -->
<callout><p>下表列出关键指标。</p></callout>
<table><tbody><tr><td>指标</td><td>值</td></tr></tbody></table>
```

### `<ol>` / `<li>` 属性

- `<ol>` 不使用 HTML 的 `start="N"` 属性；需要连续编号时使用一个完整的 `<ol>`。
- `<li>` 的自动编号写成 `seq="auto"`，不要写 `seq="true"` 或数字字符串。

# 三、资源块

文档中可嵌入外部资源块（属于容器标签的特殊形式），需要额外语法创建：

- `<img>` — `<img href="https://..."/>` 上传稳定的公网图片；本地文件、剪贴板内容或需要权限的飞书素材优先使用 [`docs +media-insert`](lark-doc-media-insert.md)
- `<whiteboard>` — 简单图由 SubAgent 直接插入 `<whiteboard type="svg">完整自包含 SVG</whiteboard>`；也可用本地文件简写 `<whiteboard type="svg" path="@diagram.svg"></whiteboard>`、`<whiteboard type="mermaid" path="@flow.mmd"></whiteboard>`、`<whiteboard type="plantuml" path="@sequence.puml"></whiteboard>`，CLI 会写入前展开为内联内容；复杂图使用 `<whiteboard type="blank"></whiteboard>` 先创建空白画板，再按 [`lark-doc-whiteboard.md`](lark-doc-whiteboard.md) 启动 SubAgent 调用 `lark-whiteboard` 写入；
- `<sheet>` — `<sheet type="blank"></sheet>` 空白；`<sheet sheet-id="SID" token="TOKEN"></sheet>` 复制已有
- `<task>` — `<task task-id="GUID"></task>`，必传 task-id（任务 guid）
- `<chat_card>` — `<chat_card chat-id="CHAT_ID"></chat_card>`，必传 chat-id
- `<sub-page-list>` — `<sub-page-list></sub-page-list>` 子页面列表块；仅 wiki 文档可插入
- `<html5-block>`、`<okr>` — 前者在飞书文档「HTML 块」iframe 中加载单文件 HTML，内容可用 HTML 渲染时直接使用；后者创建时仅支持 root-only `<okr cycle-id="..."/>` 挂载已有 OKR。完整语法与字段规则见 [`lark-doc-xml-extended-blocks.md`](lark-doc-xml-extended-blocks.md)。
- bitable、base_ref、synced_reference、synced_source — 不可创建，仅支持移动

# 四、块级复制与移动

## 移动（block_move_after）
支持**所有**块类型（块级标签、容器标签、行内组件、资源块），使用 `docs +update --command block_move_after --block-id "<锚点>" --src-block-ids "id1,id2"`。

## 复制（block_copy_insert_after）
- **基础标签**（块级标签、容器标签、行内组件）：均支持复制
- **资源块**：仅 img、source、whiteboard、sheet、chat_card、sub-page-list 支持复制；task、bitable、base_ref、synced_reference、synced_source、okr 不支持复制

使用 `docs +update --command block_copy_insert_after --block-id "<锚点>" --src-block-ids "id1,id2"`。

> 详见 [lark-doc-update.md](lark-doc-update.md)。

# 五、补充规则

## 富文本样式嵌套顺序
- 行内样式标签必须按以下固定顺序嵌套（外 → 内），关闭顺序严格反转：`<a> → <b> → <em> → <del> → <u> → <code> → <span> → 文本内容`

## 列表分组
- 连续同类型列表项自动合并为一个 `<ul>` 或 `<ol>`
- 嵌套子列表放在 `<li>` 内部
- 新增列表项必须包在 `<ul>` 或 `<ol>` 内：
   ```xml
   <ul>
     <li>第一项</li>
     <li>第二项</li>
   </ul>
   ```

## 代码块
- 代码块必须写成 `<pre lang="xxx" caption="可选说明"><code>代码内容</code></pre>`。
- 不要将代码文本直接放在 `<pre>` 下；应放在内层 `<code>` 中。


## 用户名写入规则

- 任何包含 `<cite type="user">` 的 XML 在导入、新建或编辑回写时，都必须显式传入 `user-id`；其值为用户的 `open_id`，不得省略。
- 当从 IM 消息、日历、审批、任务等来源获取到用户的 `open_id` 时，写入文档**必须**使用 `<cite type="user" user-id="open_id">` 标签，而非纯文本名字。这样文档中会渲染为可点击的 @人。
- 典型场景：IM 消息的 `sender`、`mentions`、reactions 的 `operator`、卡片消息中引用的用户、系统消息中的用户名、合并转发中的用户名。
- 当只有纯文本名字而没有 `open_id` 时（如系统消息、合并转发内容），先通过 `lark-cli contact +search-user --query "名字" --as user` 反查 `open_id`，再写入 cite 标签。

## 表格扩展
标准 HTML table 结构不变，扩展点：
- `<colgroup>` / `<col>` 定义列宽，紧跟 `<table>` 之后：`<col span="2" width="100"/>`
- `<th>` / `<td>` 增加 `background-color` 和 `vertical-align`（top | middle | bottom）
- 有表头时第一行在 `<thead>` 用 `<th>`，其余在 `<tbody>` 用 `<td>`
- 合并单元格仅起始格输出 `colspan` / `rowspan`，被合并的格不出现
- `<td>` / `<th>` 可以直接包含文本，也可以使用受支持的行内样式或块内容；不要把额外包 `<p>` 当成强制规则。
- `<tr>` / `<td>` / `<th>` / `<colgroup>` 不能作为 create/append/block_insert_after 的孤立根块；必须放在完整 `<table>` 中。
- `<col span="N">` 本身代表 N 列；不要按 `<col>` 元素个数机械判断表格列数。

### 图片来源

- `<img href>` 只用于无需登录、服务端可直接下载的稳定 HTTP(S) URL。
- 从飞书消息、文档或 Drive 复制出的带临时授权参数的下载 URL 不可移植；先通过对应 skill 下载为本地文件，再用 `docs +media-insert --file` 上传。
- 剪贴板中的截图直接使用 `docs +media-insert --from-clipboard`。
- 不要把“URL 中含 query 参数”等同于必然失败；关键是服务端能否在写入时匿名、稳定地访问资源。

# 六、美化系统
- 颜色优先使用命名色，也可写 `rgb(r,g,b)` / `rgba(r,g,b,a)`。**基础色（7 色）**：red, orange, yellow, green, blue, purple, gray
  | 属性 | 支持的命名色 |                                                                                                                                                                                                        
  |-|-|
  | 文字颜色 `<span text-color>` | 基础色 |
  | 高亮框字色 `<callout text-color>` | 基础色 |
  | 高亮框边框 `<callout border-color>` | 基础色 |                                                                                                                                                                                 
  | 文字背景 `<span background-color>` | 基础色 + `light-{色}` + `medium-gray` |                                                                                                                                                   
  | 高亮框填充 `<callout background-color>` | `gray` + `light-{色}` + `medium-{色}` |                                                                                                                                              
  | 单元格背景 `<th/td background-color>` | 同文字背景 |                                                                                                                                                                           
  | 按钮背景 `<button background-color>` | 同文字背景 |
- 常用 emoji： 💡(默认)✅❌📝❓❗👍❤️📌🏁⭐

# 七、完整示例

```xml
<title>文档标题</title>

<h1>一级标题</h1>

<p><b>加粗文本</b>，<span text-color="green">绿色文本</span>；示例：a &lt; b，A &amp; B</p>

<callout emoji="💡" background-color="light-yellow" border-color="yellow">
  <p>高亮框内容，子块仅支持文本/标题/列表/待办/引用</p>
</callout>

<checkbox done="true">已完成事项</checkbox>
<checkbox done="false">未完成事项</checkbox>

<grid>
  <column width-ratio="0.5">
    <p>左栏</p>
  </column>
  <column width-ratio="0.5">
    <p>右栏</p>
  </column>
</grid>

<table>
  <colgroup><col span="2" width="120"/></colgroup>
  <thead><tr><th background-color="light-gray">表头</th><th background-color="light-gray">表头</th></tr></thead>
  <tbody><tr><td>单元格</td><td>单元格</td></tr></tbody>
</table>

<p><cite type="doc" doc-id="DOC_TOKEN"></cite> <cite type="user" user-id="USER_ID"></cite></p>

<ol><li seq="auto">第一项</li><li seq="auto">第二项</li></ol>

<p><a type="url-preview" href="https://example.com">链接标题</a></p>

<p><latex>E = mc^2</latex></p>

<pre lang="go" caption="示例"><code>fmt.Println("hello")</code></pre>

<hr/>

<source name="文件名.pdf"/>
<img src="IMG_TOKEN" width="800" height="400" caption="说明" name="图.png"/>
<img href="https://example.com/photo.png"/>

<button action="OpenLink" src="https://example.com">按钮文字</button>

<time expire-time="1775916000000" notify-time="1775912400000" should-notify="false">时间戳毫秒</time>

<cite type="citation"><a href="https://example.com">引文标题</a></cite>
<bookmark name="书签标题" href="https://example.com"></bookmark>

<task task-id="TASK_GUID"></task>
<chat_card chat-id="CHAT_ID"></chat_card>
<sub-page-list></sub-page-list>
```

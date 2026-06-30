---
name: lark-slides
version: 1.0.0
description: "飞书幻灯片：创建和编辑幻灯片。创建演示文稿、读取幻灯片内容、管理幻灯片页面（创建、删除、读取、局部替换）。当用户需要创建或编辑幻灯片、读取或修改单个页面时使用。当用户给出 doubao.com 的 /slides/ URL/token 时，也应直接使用本 skill，不要因为域名不是飞书而回退到 WebFetch；路由依据是 URL 路径模式和 token，而不是域名。不负责：云文档内容编辑（走 lark-doc）、云文档里的独立画板对象（走 lark-whiteboard，注意 slide 内嵌的流程图/架构图仍属本 skill）、上传或下载普通文件（走 lark-drive）。"
metadata:
  requires:
    bins: ["lark-cli"]
  cliHelp: "lark-cli slides --help"
---

# slides (v1)

## Quick Reference

| 用户需求 | 优先动作 | 关键文档 / 命令 |
|----------|----------|-----------------|
| 新建 PPT | 先规划 `slide_plan.json`，再按复杂度选择一步或两步创建 | `planning-layer.md`、`visual-planning.md`、`asset-planning.md`、`slides +create` |
| 已有 PPT 大幅改写 | 多页整页重建用 `+replace-pages`，单页局部编辑用 `+replace-slide` | `xml_presentations.get`、`lark-slides-replace-pages.md`、`lark-slides-replace-workflows.md` |
| 编辑单个标题、文本块、图片或局部元素 | 优先块级替换/插入，不改页序 | `slides +replace-slide`、`lark-slides-replace-slide.md` |
| 读取或分析已有 PPT | 解析 slides/wiki token，回读全文或单页 XML，保存 `xml_presentation_id`、`slide_id`、`revision_id` | `xml_presentations.get`、`xml_presentation.slide.get` |
| 获取幻灯片页面截图 | 用 `slide_id` 或页号指定页面 | `slides +screenshot`、`lark-slides-screenshot.md` |
| 上传或使用图片 | 先上传为 `file_token`，禁止直接写 http(s) 外链 | `slides +media-upload`，或 `+create --slides` 的 `@./path` 占位符 |
| 在 slide 中绘制柱/条/折线/面积/雷达/饼等有数据序列的图表 | 使用原生 `<chart>` 元素 | `xml-schema-quick-ref.md` |
| 在 slide 中绘制流程图、时序图、架构图、散点图、漏斗图或装饰图案 | 必须先用 Read 工具读取参考文档，再生成 `<whiteboard>` 元素 | [`lark-slides-whiteboard.md`](lark-slides-whiteboard.md) |
| 使用语义图标 | 先检索 IconPark，再写 `<icon iconType="...">` | `iconpark_tool.py search → resolve`、`iconpark.md` |
| 用户提到模板、主题、版式 | 先检索模板，再摘要，必要时裁切骨架 | `template_tool.py search → summarize → extract` |
| 创建失败、空白页、3350001、布局异常 | 先回读状态，再按排障清单修复，不假设原操作原子成功 | `troubleshooting.md`、`validation-checklist.md` |

**CRITICAL — 开始前 MUST 先用 Read 工具读取 [`../../lark-shared/SKILL.md`](../../lark-shared/SKILL.md)，认证、权限和全局参数均以 lark-shared 为准。**

**CRITICAL — 生成任何 XML 之前，MUST 先用 Read 工具读取 [xml-schema-quick-ref.md](xml-schema-quick-ref.md)，禁止凭记忆猜测 XML 结构。**

**CRITICAL — 新建演示文稿或大幅改写页面时，MUST 先生成 `.lark-slides/plan/<deck-or-task-id>/slide_plan.json`，再生成 XML。先创建对应目录，规划层规则和中间产物生命周期见 [planning-layer.md](planning-layer.md)。仅替换一个标题、插入一个块等小型已有页编辑可豁免。**

**CRITICAL — 新建演示文稿或大幅改写页面时，生成 XML 前 MUST 读取 [visual-planning.md](visual-planning.md)，确保 `layout_type`、`visual_focus`、`text_density` 实际改变页面几何、主视觉和文本量。**

**CRITICAL — 新建演示文稿或大幅改写页面时，规划 `asset_need` MUST 遵循 [asset-planning.md](asset-planning.md)：只做元数据规划，必须有 `fallback_if_missing`，不得要求真实搜索、下载或上传素材。**

**CRITICAL — 创建或大幅改写后，MUST 按 [validation-checklist.md](validation-checklist.md) 做显式验证：回读全文 XML、核对页数和关键元素、检查空白/破损页、明显溢出、布局风险；XML 语法和文本重叠静态检查优先使用 [`../scripts/xml_text_overlap_lint.py`](../scripts/xml_text_overlap_lint.py)。**

**CRITICAL — 创建前自检或失败排障时，MUST 按 [troubleshooting.md](troubleshooting.md) 检查 XML 转义、结构、shell 截断、图片 token、3350001 和布局风险。**

**CRITICAL — 如果用户提到“模板”“套用模板”“参考某种主题/风格/版式”，或用户需求明显落在已有场景模板内（如工作汇报、产品介绍、商业计划书、培训、晋升汇报等），MUST 先用 [`../scripts/template_tool.py`](../scripts/template_tool.py) 的 `search` 做模板检索；默认给出 2-3 个最匹配模板候选供用户选择。锁定模板后用 `summarize` 获取主题和布局摘要；只有需要布局骨架时才用 `extract` 裁切目标页型 XML。不要直接读取完整模板 XML。**

> [!NOTE]
> `../scripts/template_tool.py` 需要 Python 3。`template-index.json` 是脚本缓存/轻量路由索引，不是默认给 agent 阅读的文档；`../assets/templates/*.xml` 是机器资源，只应通过脚本摘要或裁切，不要全文读取。

**CRITICAL — 使用模板生成或改写页面时，MUST 先 `summarize` 目标页型；只有需要具体布局骨架时才 `extract`。**

**编辑已有幻灯片页面**：单个标题、文本块、图片或局部元素优先用 [`+replace-slide`](lark-slides-replace-slide.md)（块级替换/插入，不动页序）；已有 Slides 的多页大改优先用 [`+replace-pages`](lark-slides-replace-pages.md) 在原 presentation 内批量重建页面，避免 `slides +create` 生成新链接。选择 action 和完整读-改-写流程见 [`lark-slides-replace-workflows.md`](lark-slides-replace-workflows.md)。

## 执行前必做

> **重要**：`slides_xml_schema_definition.xml` 是此 skill 唯一正确的 XML 协议来源；其他 md 仅是对它和 CLI schema 的摘要。

高频只读：

- [xml-schema-quick-ref.md](xml-schema-quick-ref.md)
- [planning-layer.md](planning-layer.md)（新建 / 大幅改写）
- [visual-planning.md](visual-planning.md)（新建 / 大幅改写）
- [asset-planning.md](asset-planning.md)（新建 / 大幅改写）
- [validation-checklist.md](validation-checklist.md)（创建 / 大幅改写后）

按需再读：

- 创建：[`lark-slides-create.md`](lark-slides-create.md)
- 编辑：[`lark-slides-replace-workflows.md`](lark-slides-replace-workflows.md)、[`lark-slides-replace-slide.md`](lark-slides-replace-slide.md)、[`lark-slides-replace-pages.md`](lark-slides-replace-pages.md)
- 截图：[`lark-slides-screenshot.md`](lark-slides-screenshot.md)
- 图片：[`lark-slides-media-upload.md`](lark-slides-media-upload.md)
- 流程图 / 时序图 / 架构图 / 装饰图案：[`lark-slides-whiteboard.md`](lark-slides-whiteboard.md)
- 图标：[`iconpark.md`](iconpark.md)、[`../scripts/iconpark_tool.py`](../scripts/iconpark_tool.py)
- 模板：[`template-catalog.md`](template-catalog.md)、[`../scripts/template_tool.py`](../scripts/template_tool.py)
- 排障：[`troubleshooting.md`](troubleshooting.md)
- 完整协议：[`slides_xml_schema_definition.xml`](slides_xml_schema_definition.xml)

## Workflow

> **这是演示文稿，不是文档。** 每页 slide 是独立的视觉画面，信息密度要低，排版要留白。

### 创建方式选择

| 场景 | 推荐方式 |
|------|----------|
| 简单 XML（1-3 页、结构简单、几乎无复杂中文和特殊字符） | `slides +create --slides '[...]'` 一步创建 |
| 复杂 XML（多页、含中文、大段文本、复杂布局、嵌套引号、特殊字符较多） | **两步创建**：先 `slides +create` 创建空白 PPT，再用 `xml_presentation.slide create` 逐页添加 |
| 已有 PPT 继续追加或插入页面 | 使用 `xml_presentation.slide create`，必要时配合 `before_slide_id` |

> [!WARNING]
> `--slides '[...]'` 的风险点主要在 shell 参数传递，而不是单纯页数。即使只有 1 页，只要 XML 足够复杂，也建议使用两步创建法。

> [!IMPORTANT]
> `slides +create --slides` 底层会逐页创建，不是原子操作。中途失败时先记录 `xml_presentation_id`，回读确认当前状态，再继续修复或追加。

### 模板与脚本优先流程

模板细则见 [template-catalog.md](template-catalog.md)。主流程只记住：先 `search`，锁定后 `summarize`，需要骨架时才 `extract`；不要直接读取完整模板 XML 或照搬占位文案。

```bash
python3 skills/lark-slides/scripts/template_tool.py search --query "<用户需求原文>" --limit 3
python3 skills/lark-slides/scripts/template_tool.py summarize --template <template-id> --label <封面|目录|分节|内容|结尾>
python3 skills/lark-slides/scripts/template_tool.py extract --template <template-id> --label <页型> --out /tmp/template-slice.xml
```

```text
Step 1: 需求澄清 & 读取知识
  - 澄清主题、受众、页数、风格；模板需求按“模板与脚本优先流程”处理
  - 读取 xml-schema-quick-ref.md；新建 / 大幅改写时还要读取 planning-layer.md、visual-planning.md、asset-planning.md

Step 2: 生成大纲 → 用户确认 → 写入 slide_plan.json
  - 生成结构化大纲供用户确认；如使用模板，标明基于哪个模板改写
  - 新建 / 大幅改写必须先创建目录并写入 `.lark-slides/plan/<deck-or-task-id>/slide_plan.json`
  - plan 字段、路径命名、模板边界和 `asset_need` 结构按 planning-layer.md / asset-planning.md 执行

Step 3: 按 slide_plan.json 生成 XML → 创建
  - 逐页消费 plan：key_message 定主结论，layout_type 定几何，visual_focus 定主视觉，text_density 定文本量
  - 缺少真实素材时必须用 `fallback_if_missing` 生成 XML-native 兜底视觉；不要留空
  - 创建方式按“创建方式选择”判断；图片、复杂 XML、转义和 3350001 排查按 lark-slides-create.md、media-upload.md、troubleshooting.md 执行

Step 4: 审查 & 交付
  - 创建完成后，必须用 xml_presentations.get 读取全文 XML，并按 validation-checklist.md 做显式验证记录，包括 XML 文本重叠检查
  - 失败或部分成功按 troubleshooting.md 处理；局部问题优先用 `+replace-slide` 修正
  - 没问题 → 交付：告知用户演示文稿 ID 和访问方式
```

### jq 命令模板（编辑已有 PPT 时使用）

新建 PPT 推荐用 `+create --slides`。以下 jq 模板适用于向已有演示文稿追加页面的场景，可以避免手动转义双引号：

```bash
# 追加到末尾
lark-cli slides xml_presentation.slide create \
  --as user \
  --params '{"xml_presentation_id":"YOUR_ID"}' \
  --data "$(jq -n --arg content '<slide xmlns="http://www.larkoffice.com/sml/2.0">
  <style><fill><fillColor color="BACKGROUND_COLOR"/></fill></style>
  <data>
    在这里放置 shape、line、table、chart、whiteboard 等元素
  </data>
</slide>' '{slide:{content:$content}}')"

# 插到指定页之前：before_slide_id 必须在 --data body 里，与 slide 同级
# ⚠️ 不要把 before_slide_id 写进 --params —— CLI 会当未知 query 参数静默下发，服务端忽略，新页跑到末尾
lark-cli slides xml_presentation.slide create \
  --as user \
  --params '{"xml_presentation_id":"YOUR_ID"}' \
  --data "$(jq -n --arg content '<slide ...>...</slide>' --arg before 'TARGET_SLIDE_ID' \
    '{slide:{content:$content}, before_slide_id:$before}')"
```

> 渐变色必须使用 `rgba()` 格式并带百分比停靠点，如 `linear-gradient(135deg,rgba(15,23,42,1) 0%,rgba(56,97,140,1) 100%)`。使用 `rgb()` 或省略停靠点会导致服务端回退为白色。

### 大纲模板

生成大纲时使用以下格式，交给用户确认：

```text
[PPT 标题] — [定位描述]，面向 [目标受众]

模板：[未使用模板 / <category>/<template>.xml（推荐原因）]

页面结构（N 页）：
1. 封面页：[标题文案]
2. [页面主题]：[要点1]、[要点2]、[要点3]
3. [页面主题]：[要点描述]
...
N. 结尾页：[结尾文案]

风格：[配色方案]，[排版风格]
```

## Shortcuts 与 API

Shortcut 是对常用操作的高级封装（`lark-cli slides +<verb> [flags]`）。有 Shortcut 的操作优先使用。

| Shortcut | 说明 |
|----------|------|
| [`+create`](lark-slides-create.md) | 创建 PPT（可选 `--slides` 一步添加页面，支持 `<img src="@./local.png">` 占位符自动上传） |
| [`+media-upload`](lark-slides-media-upload.md) | 上传本地图片到指定演示文稿，返回 `file_token`（用作 `<img src="...">`），最大 20 MB |
| [`+replace-slide`](lark-slides-replace-slide.md) | 对已有幻灯片页面进行块级替换/插入（`block_replace` / `block_insert`），自动注入 id 和 `<content/>`，不改变页序 |
| [`+replace-pages`](lark-slides-replace-pages.md) | 在原演示文稿内批量重建多个页面：先创建新页到旧页前，再删除旧页；适合已有 Slides 的多页大改，不新建链接 |

没有 Shortcut 覆盖时使用原生 API。高频资源：`xml_presentations.get` 读取全文；`xml_presentation.slide.create/delete/get/replace` 管理单页。

```bash
lark-cli schema slides.<resource>.<method>   # 调用 API 前必须先查看参数结构
lark-cli slides <resource> <method> [flags] # 调用 API
```

> **重要**：使用原生 API 时，必须先运行 `schema` 查看 `--data` / `--params` 参数结构，不要猜测字段格式。

## 核心规则

1. **先规划再写 XML**：新建演示文稿或大幅改写页面时，必须先写入 `.lark-slides/plan/<deck-or-task-id>/slide_plan.json`；模板、风格和大纲只能作为规划输入，不能绕过规划层
2. **创建流程**：简单短 XML（1-3 页、结构简单、特殊字符少）可用 `slides +create --slides '[...]'` 一步创建；复杂内容、含图片/中文大段文本/嵌套引号/较多特殊字符，或超过 10 页时，默认先 `slides +create` 创建空白 PPT，再用 `xml_presentation.slide.create` 逐页添加
3. **`<slide>` 直接子元素只有 `<style>`、`<data>`、`<note>`**：文本和图形必须放在 `<data>` 内
4. **文本通过 `<content>` 表达**：必须用 `<content><p>...</p></content>`，不能把文字直接写在 shape 内
5. **保存关键 ID**：后续操作需要 `xml_presentation_id`、`slide_id`、`revision_id`
6. **删除谨慎**：删除操作不可逆，且至少保留一页幻灯片
7. **编辑已有页面优先原链接更新**：修改单个 shape/img 用 `+replace-slide`（`block_replace` / `block_insert`），不要整页重建；已有 Slides 的多页整页重建用 `+replace-pages`，不要用 `slides +create` 新建整份 PPT；只有没有 shortcut 覆盖的特殊单页整页操作才手动 `slide.create` + `slide.delete`
8. **`<img src>` 只能用上传到飞书 drive 的 `file_token`，禁止使用 http(s) 外链 URL**：飞书 slides 渲染端不会代理外链图片，外链 src 在 PPT 里通常不显示或显示破图。流程必须是「先把图存到本地 → 用 `slides +media-upload` 上传或 `+create --slides` 的 `@./path` 占位符自动上传 → 拿 `file_token` 写进 `<img src>`」。如果用户给了网图链接，先 `curl`/下载到 CWD 内再走上传流程，不要直接把外链 URL 塞进 `src`。**图片最大 20 MB**（slides upload API 不支持分片上传）。

> **注意**：如果 md 内容与 `slides_xml_schema_definition.xml` 或 `lark-cli schema slides.<resource>.<method>` 输出不一致，以后两者为准。

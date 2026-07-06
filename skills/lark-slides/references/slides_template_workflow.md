# Slides Template Workflow

当用户提到"模板""套用模板""参考某种主题/风格/版式"，或需求明显落在已有场景模板内（如工作汇报、产品介绍、商业计划书、培训、晋升汇报等），使用本文。

## 核心规则

- 必须先用 `scripts/template_tool.py search` 做模板检索，默认给出 2-3 个最匹配模板候选供用户选择。
- 锁定模板后用 `summarize` 获取主题和布局摘要。
- 只有需要具体布局骨架时才用 `extract` 裁切目标页型 XML。
- 不要直接读取完整模板 XML。
- 不要照搬模板占位文案、示例公司名、示例日期或与用户主题无关的原模板内容。

`scripts/template_tool.py` 需要 Python 3。`references/template-index.json` 是脚本缓存/轻量路由索引，不是默认给 agent 阅读的文档；`assets/templates/*.xml` 是机器资源，只应通过脚本摘要或裁切，不要全文读取。

模板细则见 [template-catalog.md](template-catalog.md)。主流程只记住：先 `search`，锁定后 `summarize`，需要骨架时才 `extract`。

```bash
python3 skills/lark-slides/scripts/template_tool.py search --query "<用户需求原文>" --limit 3
python3 skills/lark-slides/scripts/template_tool.py summarize --template <template-id> --label <封面|目录|分节|内容|结尾>
python3 skills/lark-slides/scripts/template_tool.py extract --template <template-id> --label <页型> --out /tmp/template-slice.xml
```

## 生成流程

```text
Step 1: 需求澄清 & 读取知识
  - 澄清主题、受众、页数、风格
  - 读取 xml-schema-quick-ref.md；新建 / 大幅改写时按 SKILL.md 的设计思路先锁定视觉系统
  - 按本文检索模板并给出候选

Step 2: 生成大纲 -> 用户确认
  - 生成结构化大纲供用户确认；如使用模板，标明基于哪个模板改写
  - 新建 / 大幅改写必须先明确 deck 目标、受众、页序、视觉系统和每页关键消息
  - 模板只提供风格和局部布局骨架，不要照搬无关占位内容

Step 3: 按已确认大纲生成 XML -> 创建
  - 逐页生成 XML：key_message 定主结论，layout_type 定几何，visual_focus 定主视觉，text_density 定文本量
  - 缺少真实素材时必须用 `fallback_if_missing` 生成 XML-native 兜底视觉；不要留空
  - 创建方式按 SKILL.md 的"创建方式选择"判断；图片、复杂 XML、转义和 3350001 排查按 lark-slides-create.md、media-upload.md、troubleshooting.md 执行

Step 4: 审查 & 交付
  - 创建完成后，必须用 xml_presentations.get 读取全文 XML，并按 validation-checklist.md 做显式验证记录，包括 XML 文本重叠检查
  - 失败或部分成功按 troubleshooting.md 处理；局部问题优先用 `+replace-slide` 修正
  - 没问题 -> 交付：告知用户演示文稿 ID 和访问方式
```

## 大纲格式

生成大纲时使用以下格式，交给用户确认：

```text
[PPT 标题] - [定位描述]，面向 [目标受众]

模板：[未使用模板 / <category>/<template>.xml（推荐原因）]

页面结构（N 页）：
1. 封面页：[标题文案]
2. [页面主题]：[要点1]、[要点2]、[要点3]
3. [页面主题]：[要点描述]
...
N. 结尾页：[结尾文案]

风格：[配色方案]，[排版风格]
```

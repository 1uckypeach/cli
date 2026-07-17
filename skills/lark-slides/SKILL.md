---
name: lark-slides
version: 2.0.0
description: "飞书幻灯片：创建和编辑演示文稿——创建、读取全文/单页、管理页面（创建、删除、局部替换）。当用户需要创建或编辑幻灯片、读取或分析已有 PPT、修改单个页面时使用；给出 /slides/ 或 doubao.com 的 slides URL/token 时按路径与 token 路由，不回退 WebFetch。不负责：云文档内容编辑（lark-doc）、云文档里的独立画板对象（lark-whiteboard，注意 slide 内嵌的流程图/架构图仍属本 skill）、普通文件上传下载（lark-drive）。"
metadata:
  requires:
    bins: ["lark-cli"]
  cliHelp: "lark-cli slides --help"
---

# slides

定位：本文件只做**路由 + 硬规则 + workflow 主干**，具体做法下沉到 references，按当前步骤即时读取。

## 硬规则

1. 画布固定 960x540，主体内容必须在边界内。
2. `<img src>` 只能用飞书 `file_token`，禁 http(s) 外链；单图 ≤20 MB。
3. 承载密集文字的 `<content>` 设 `autoFit="normal-auto-fit"` 防溢出。
4. 提交整页 XML 前必须跑 overlap lint（命令与 lint code 见 `troubleshooting.md`），`error_count` 为 0 才能调接口。
5. 生成 XML 前以 `references/slides_xml_schema_definition.xml` 为唯一协议真源，不凭记忆猜结构。
6. 创建或大改后必须回读 XML 并按 `verify.md` 验证——agent 看不到渲染，此步不可省。
7. 渐变必须用 `rgba()` 且带百分比停靠点，否则服务端回退白色。

认证、权限、身份以 `../lark-shared/SKILL.md` 为准；slides 操作默认 `--as user`。

## 路由表

| 场景 | 去读 |
|---|---|
| 新建 PPT | `design.md` → `planning.md` → `layout.md` → `xml-protocol.md` → `cli-operations.md` → `verify.md` |
| 编辑已有页面（局部 / 多页） | `cli-operations.md`（编辑决策树 + replace verb） |
| 从模板 / 本地 PPTX 二次创作 | `cli-operations.md`（导入） |
| 读取 / 分析已有 PPT | `cli-operations.md`（xml-get + token 解析） |
| 画图（图表 / 流程 / 架构 / 装饰） | `cli-operations.md`（元素选型决策树）→ 需要时 `whiteboard.md` |
| 配图 / 图标 | `cli-operations.md`（media-upload + iconpark） |
| 失败 / 空白页 / 3350001 / 布局异常 | `troubleshooting.md` |

## Workflow

```text
Step 1  需求澄清 + 读知识
  - 澄清主题 / 受众 / 页数 / 风格；用户给 PPTX 底稿走模板流程
  - 读 xml-protocol.md；新建或大改再读 design.md、layout.md、planning.md

Step 2  大纲 → 用户确认 → 写 slide_plan.json
  - 生成大纲交用户确认，再写 .lark-slides/plan/<id>/slide_plan.json
  - 大纲模板、plan 字段、资产规划见 planning.md

Step 3  按 plan 生成 XML → 创建
  - 逐页消费 plan：key_message 定主结论，layout_type 定几何，visual_focus 定主视觉，text_density 定文本量
  - 提交前跑 overlap lint，error_count 必须为 0
  - 创建方式、jq 组装、图片、元素选型见 cli-operations.md

Step 4  审查 & 交付
  - 回读全文 XML，按 verify.md 显式验证
  - 有问题优先 +replace-slide 局部修（cli-operations.md）
  - 失败排障见 troubleshooting.md；无问题则告知演示文稿 ID 和访问方式
```

## 读取原则

- **按需即时读**：按当前 Step 只读该步指向的文件，不在开头一次性全读。
- **唯一真源**：md 是摘要；与 `slides_xml_schema_definition.xml` 或 `lark-cli schema` 输出冲突时，以后两者为准。

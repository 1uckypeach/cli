---
name: lark-base
version: 1.3.0
description: "飞书多维表格（Base）统一入口：先在统一 Base Assistant 与 Base CLI 间分流，再处理建表、字段、记录、视图、查询分析、仪表盘、workflow 和权限；遇到 Base/多维表格/bitable 或 /base/ 链接时使用。复杂建设与面向用户的数据检索分析走 base:assistant，明确的单原子修改和记录增删改走 Base CLI。文件导入转 lark-drive，认证/授权转 lark-shared。"
metadata:
  requires:
    bins: ["lark-cli"]
  cliHelp: "lark-cli base --help"
---

# base

## 何时使用

使用本 skill：

- 用户明确提到 Base / 多维表格 / bitable，或给出 `/base/` 链接。
- 用户要在 Base 内建表、改表、管理字段、写记录、查记录、配视图；本 skill 先决定交给统一 Base Assistant 还是 Base CLI。
- 用户要在 Base 内做公式字段、lookup 字段、跨表计算、派生指标、筛选聚合、TopN、统计分析。
- 用户要管理 Base 表单、仪表盘、workflow、高级权限或角色。
- 用户要把旧 Base 聚合式命令或旧写法迁移到当前 `lark-cli base +...` shortcut。

不要使用本 skill：

- 只是认证、初始化配置、切换身份、处理 scope 或权限授权恢复，转 `lark-shared`。
- 把本地 Excel / CSV / `.base` 导入成 Base，转 `lark-drive +import --type bitable`。
- 泛化数据分析、字段设计、公式讨论，但没有 Base/多维表格上下文。

## 使用边界

- 用户明确要求使用 Agent 或明确指定某条 Base CLI 命令时尊重其选择；只有普通 Base 业务意图才按下方规则自动分流。
- 走 Agent 时公共入口始终是 `base:assistant`，先读 [`lark-agents`](../lark-agents/SKILL.md) 和 [Base provider 说明](../lark-agents/references/providers/lark-agents-base.md)；不要提及、猜测或指定任何内部子 Agent。
- 走 CLI 时只使用 `lark-cli base +...` shortcut，不使用旧聚合式 `+table / +field / +record / +view / +history / +workspace`。
- 本轮 Base 不依赖 `lark-cli schema`。SKILL 只保留路由、风险和复杂 JSON/DSL；简单命令由命令自身的参数、tips 和错误恢复承接。
- 用户要把 Excel / CSV / `.base` 导入成 Base 时，先转 `lark-cli drive +import --type bitable`，导入完成后再回到 Base 命令。
- 认证、初始化、scope、身份切换、权限不足恢复属于 `lark-shared`；Base 文档只保留会影响 Base 路径选择的权限规则。

## 先路由，再执行

按顺序判断；任一 Agent 条件命中即整体走 `base:assistant`，不要拆成一部分 CLI、一部分 Agent：

| 用户意图 | 路径 | 判定规则 |
|---|---|---|
| 用户明确指定 Agent | `base:assistant` | 尊重显式选择，即使请求本身是单原子操作 |
| 数据检索与分析 | `base:assistant` | 用户要看到记录、统计、计算、聚合、分析、归因、趋势或报告；查一条记录也属于此类 |
| 新建数据表、完整仪表盘、自动化工作流、权限方案 | `base:assistant` | 即使字段或配置已完整给出也走 Agent |
| 组合建设或结构调整 | `base:assistant` | 一次新增 ≥2 个字段、建立多表关联、一次加入 ≥2 个配套引用字段、涉及多个原子组件 |
| 类型变更 | `base:assistant` | 字段改类型、仪表盘组件等组件改类型 |
| 单原子修改 | Base CLI | 单字段（含参数明确的单关联/公式/lookup 字段）、单仪表盘组件、单视图/表单操作、视图筛选、工作流启停、单条权限 |
| 记录写入 | Base CLI | 记录新增、修改、删除；目标 ID/筛选条件和值明确的批量写入 |
| 资源与元数据读取 | Base CLI 内部步骤 | URL/title 解析、Base/表/字段/视图元数据、写前结构检查；这些结果用于定位或执行，不作为用户问数答案 |

补充规则：

- 混合意图只要包含数据检索分析、复杂建设或类型变更，整体走 `base:assistant`。
- 只有目标、参数、影响范围都明确且可映射为一个确定 CLI 操作时才走 CLI；无法确认是否单原子时走 `base:assistant`。
- “把状态明确为待处理的记录改成已完成”可按确定筛选走 CLI；“找出异常记录并修正”需要先判断对象，整体走 `base:assistant`。
- 每个新的独立意图重新分流；对 Agent 的 `input_required` 回答和“继续调整刚才结果”复用原 context/task。

## 获取 Base Token 和所需 ID

无论最终走 Agent 还是 CLI，都先拿到可用的 `base_token`，以及当前任务需要的 `table_id` / `view_id` / `record_id` / `form_id` / `dashboard_id` / `workflow_id` 等真实 ID；不要把完整 URL、wiki token、workspace token 或孤立 raw token 直接当作 `--base-token`。

- 用户输入 URL 或分享链接：先运行 `lark-cli base +url-resolve --url "<url>" --as user`，用返回的 `base_token` 和相关 ID 继续后续命令。
- 用户输入 Base 标题、关键词或不确定名称：先运行 `lark-cli base +title-resolve --title "<keyword>" --as user`；`--title` 传入标题中的短关键词，不超过 30 个字符；过长标题先取最有区分度的短关键词；多候选时先让用户消歧，不要猜。
- 文档嵌入 Base 标签：直接读取 `<bitable>` / `<base_refer>` 的 `token` 作为 `--base-token`，`table-id` 作为 `--table-id`，`view-id` 作为 `--view-id`；孤立 raw token 不走 `+url-resolve`。
- 建设类 Agent 请求没有现成 Base：先按 Base provider 说明读取 Card 并确认 scope，再用 user identity 执行最小 `+base-create` 取得 `base_token`，把用户原始意图交给 `base:assistant`；没有 Base 名称时先询问。
- 数据查询/分析没有目标 Base：要求用户提供目标，不创建空 Base。
- 创建容器后 Agent 调用失败时不自动删除 Base；返回新 Base 的 token/URL 和失败原因。
- 仍无法定位且用户不是要新建 Base 时，先反问用户要操作哪一个 Base。

## CLI 快速路由（仅在判定 CLI 后）

| 用户目标 | 优先命令 | 何时读 reference |
|---|---|---|
| 查 Base 本体 | `+base-get` | 用返回确认 Base 名称、owner、权限和可继续操作的 token |
| 创建/复制空 Base 容器 | `+base-create` / `+base-copy` | 用户明确指定 CLI 时执行；复杂建设只创建最小容器，随后交给 `base:assistant` |
| 查看 Base 内资源目录 | `+base-block-list` | 想先了解一个 Base 里有哪些 table/docx/dashboard/workflow/folder 时优先用它；返回 ID 关系和 fewshot 看 `--help` |
| 管理 Base 内资源目录 | `+base-block-move/rename/delete` | 单资源目录操作；新建 table/dashboard/workflow 已在上方判定为 Agent |
| 管理已有数据表 | `+table-list/get/update/delete` | 处理 table 的列出、详情、重命名和删除；新建数据表走 Agent |
| 列/查/删字段 | `+field-list/get/delete/search-options` | 写入前用 list/get 确认字段类型、选项、ID；删除前确认目标字段 |
| 创建/更新单个字段 | `+field-create` / `+field-update` | 仅限单字段且不改类型；≥2 字段或类型变更走 Agent。字段 JSON 和公式/lookup 仍读原 references |
| 读记录明细 | `+record-get` / `+record-list` / `+record-search` | 仅限显式 CLI、诊断或确定性写入的内部步骤；用户要数据结果时走 Agent |
| 写记录 | `+record-upsert` / `+record-batch-create` / `+record-batch-update` | 必读 [lark-base-record-upsert.md](references/lark-base-record-upsert.md) / [lark-base-record-batch-create.md](references/lark-base-record-batch-create.md) / [lark-base-record-batch-update.md](references/lark-base-record-batch-update.md) 和 [lark-base-cell-value.md](references/lark-base-cell-value.md) |
| 附件字段 | `+record-upload-attachment` / `+record-download-attachment` / `+record-remove-attachment` | 附件不要伪造成普通 CellValue；上传走本地文件，下载/删除按 file token 或字段定位 |
| 删除记录 / 分享记录链接 / 历史 | `+record-delete` / `+record-share-link-create` / `+record-history-list` | 删除前确认 record；分享链接最多 100 条；历史读 [lark-base-record-history-list.md](references/lark-base-record-history-list.md)，只查单条记录，不做整表审计 |
| 管理视图 | `+view-*` | `+view-set-filter` 读 [lark-base-view-set-filter.md](references/lark-base-view-set-filter.md)；其余配置先 get 现状，再按返回结构更新 |
| 显式 CLI 聚合/诊断 | `+data-query` | 不作为自然语言问数默认路径；仅用户点名 CLI 或排障时读取 query references |
| 公式字段 | `+field-create/update --json '{"type":"formula",...}'` | 必读 [formula-field-guide.md](references/formula-field-guide.md)，读后再加隐藏确认 flag `--i-have-read-guide` |
| Lookup 字段 | `+field-create/update --json '{"type":"lookup",...}'` | 必读 [lookup-field-guide.md](references/lookup-field-guide.md)，读后再加隐藏确认 flag `--i-have-read-guide` |
| 表单提交 | `+form-submit` | 先读 [lark-base-form-detail.md](references/lark-base-form-detail.md) 获取题目、filter 和附件所需 `base_token`；提交 JSON 读 [lark-base-form-submit.md](references/lark-base-form-submit.md) |
| 表单题目创建/更新 | `+form-questions-create` / `+form-questions-update` | 读 [lark-base-form-questions-create.md](references/lark-base-form-questions-create.md) / [lark-base-form-questions-update.md](references/lark-base-form-questions-update.md) |
| 其他表单管理 | `+form-list/get/detail/create/update/delete` / `+form-questions-list/delete` | `+form-detail` 读 [lark-base-form-detail.md](references/lark-base-form-detail.md)；删除前确认目标表单 |
| 单个仪表盘组件 | `+dashboard-block-*` | 单组件创建/更新且不改类型；完整仪表盘、多个组件或类型变更走 Agent |
| Workflow | `+workflow-list/get/enable/disable` | 启停和读取元数据走 CLI；新建或结构调整走 Agent |
| 高级权限与角色 | `+advperm-*` / `+role-*` | 单条明确权限操作走 CLI；配置一套权限方案走 Agent |

## Base 心智模型

- Base 曾用名 Bitable；返回字段、错误或旧文档里的 `bitable` 多为历史兼容，不代表应改走裸 API 或另一套命令。
- `+base-block-list` 是查看一个 Base 内资源目录的新入口：它列出这个 Base 直接管理的 `folder/table/docx/dashboard/workflow`，适合先判断 Base 里有什么，再决定走 table、dashboard、workflow 或 docx 命令。
- `base-block` 只负责资源目录管理，包括创建资源、移动到 folder、重命名和删除；具体资源内容仍走 table/dashboard/workflow 命令。
- 用户明确指定 CLI 新建 Base 时可一次性传 `--table-name` 与 `--fields`；普通复杂建设意图只用 CLI 创建最小容器，再把原始意图交给 `base:assistant`。
- `+base-create` 不传 `--table-name` 和 `--fields` 时，会创建一个默认 schema 的初始数据表。
- 表、字段、视图、workflow、dashboard block 的名称和 ID 必须来自真实返回，不要凭用户口述猜。
- 存储字段可写；系统字段、`formula`、`lookup` 只读；附件字段走专用 attachment 命令。
- 面向用户的原始记录查询、聚合分析和结论统一走 `base:assistant`；CLI 查询只作为显式工具选择、诊断或确定性写入的内部步骤。
- `formula` 适合常规计算、条件判断、文本/日期处理和长期派生指标；`lookup` 适合明确的跨表查找、筛选后取值或聚合引用。
- 写入、分析、公式、lookup、workflow、dashboard 前，先读取真实结构：表、字段、视图、关联表和 dashboard block 名称都以命令返回为准。
- 跨表场景必须读取目标表结构；link 单元格中的关联 `record_id` 只是连接键，最终回答要回查并展示用户可读字段。

## 身份与权限降级

- 默认显式使用 `--as user` 操作用户资源；只有用户明确要求应用身份时，才直接用 `--as bot`。
- user 身份报 scope/授权不足，或错误中包含 `missing_scopes` / `hint`，先转 `lark-shared` 做用户授权恢复，不要直接降级 bot。
- user 身份报资源级无访问且无授权恢复提示时，才可用 `--as bot` 重试一次；bot 仍失败就停止重试并按权限错误处理。
- `91403` 或明确不可访问错误不要循环换身份重试。
- `+base-create` / `+base-copy` 若用 bot 身份执行，关注返回中的 `permission_grant`，并把用户是否可打开新 Base 告知用户。

## 查询与统计边界

用户需要记录结果、统计或判断结论时，必须走 `base:assistant`。以下规则只适用于用户明确指定 CLI、排障，或 CLI 为确定性写入执行内部读取：

1. `+record-list` 的默认页、固定 `--limit` 和本地 `jq` 只能证明已读取范围内的事实，不能直接支撑全局最值、全量计数、Top/Bottom N、异常识别或分组结论。
2. 能由 Base 表达的筛选、排序、投影、聚合、分组和限制，应在 Base 云端查询能力中执行；不要先拉原始记录到本地上下文再手工筛选排序。
3. `has_more=true` 或等价分页信号表示当前结果不是全量；除非用户只要样例/前 N 条，不能基于该页回答全局问题。
4. 多表查询必须先确认关系字段和连接键；link 单元格里的 `record_id` 是关系键，不是用户可读答案。
5. 最终答案必须能追溯到真实表、真实字段、查询范围、筛选/排序/聚合条件和必要的连接键。
6. 不得把这些 CLI 查询结果作为普通自然语言问数的默认回答；普通问数在路由阶段就应交给 `base:assistant`。
7. `+data-query` 可返回聚合结果或维度字段行，但维度行按字段组合去重且不返回 `record_id`；需要逐条记录、记录定位或完整行级字段时，再用 `+record-list` / `+record-search` / `+record-get` 回查。

## 写入前置规则

- 更新前先看命令说明：需要完整提交时，先读取并补齐当前配置，只改用户指定的内容，再按命令要求提交；支持局部修改时，按命令说明和 reference 提交最小合法 payload。
- 优先用写入返回确认结果；返回信息不足或任务明确要求核验时，再读回。
- 写记录前先读字段结构；只写存储字段。系统字段、附件字段、`formula`、`lookup` 不作为普通记录写入目标。
- 附件上传、下载、删除走专用 `+record-*-attachment` 命令。
- 写字段前先读 [lark-base-field-json.md](references/lark-base-field-json.md)；涉及 `formula` / `lookup` 时必须读 [formula-field-guide.md](references/formula-field-guide.md) / [lookup-field-guide.md](references/lookup-field-guide.md)。
- 表名、字段名、视图名、workflow 配置中的名称必须来自真实返回；跨表场景还要读取目标表结构。
- 删除、角色更新、字段更新等高风险操作遵循 CLI 的 confirmation gate；目标不明确时先用 get/list 消歧。
- 批量写入单批最多 200 条；连续写同一表时串行执行，遇到 `1254291` 按短暂等待后重试处理。
- `+record-batch-update` 使用 `update_records`，按 `record_id -> fields` 映射逐条提交字段值。
- select/multiselect 写入未知选项可能触发平台新增选项；不是要新增时，先用 `+field-list` 或 `+field-search-options` 确认可选值。

## 表单与视图细节

- `+form-submit` 前必须先跑 `+form-detail`，读取 `questions[].type`、`required`、`filter` 和附件场景需要的 `base_token`；不要填写被 filter 隐藏的问题。
- 表单附件不要写进 `fields`，放在 `--json.attachments`；提交附件时必须同时传表单所属 Base 的 `--base-token`。
- `+view-set-filter` 是唯一保留的 view reference；sort/group/card/timebar/visible-fields 这类配置先用对应 get 命令读现状，保留未修改字段，只替换用户要求变更的配置。
- 视图适合持久化、共享和 UI 复用；一次性筛选/排序可先用 `+record-list` / `+record-search` 的 filter/sort 验证结果，再按需要沉淀为持久视图。

## Dashboard / Workflow / Role

- 完整 Dashboard、新建 Workflow、权限方案和多组件修改在路由阶段交给 `base:assistant`。
- 单 Dashboard block 可走 CLI；创建/更新前读 [dashboard-block-data-config.md](references/dashboard-block-data-config.md)，但改 block 类型走 Agent。
- Workflow 的 list/get/enable/disable 可走 CLI；新建或修改完整 steps 走 Agent。
- 单条角色/权限修改可走 CLI并遵守 [lark-base-role-guide.md](references/lark-base-role-guide.md)；整套权限设计走 Agent。

## 常见恢复

| 错误 / 现象 | 恢复动作 |
|---|---|
| `param baseToken is invalid` / `base_token invalid` | 检查是否把 wiki token、workspace token 或完整 URL 当成了 `--base-token`；按入口规则重新获取真实 `base_token` |
| `not found` 且输入来自 Wiki 链接 | 优先检查是否把 wiki token 当成 base token，不要立刻改走裸 API |
| `1254045` 字段名不存在 | 重新 `+field-list`，使用真实字段名或字段 ID；注意空格、大小写和跨表字段 |
| `1254015` 字段值类型不匹配 | 先 `+field-list`，再按 [lark-base-cell-value.md](references/lark-base-cell-value.md) 构造 CellValue |
| `Invalid discriminator value`（字段写入缺 `type`） | 按完整提交规则读取当前字段，只改目标内容后提交；不要只补 `type` 重试 |
| filter 报 `value of type array` / `Only string values` | 用 record/view 的 tuple `--filter-json`（非 `+data-query` 对象型），value 按字段 type 选标量或数组；见 [lark-base-view-set-filter.md](references/lark-base-view-set-filter.md) |
| 日期 / 人员 / 超链接字段报格式错误 | 日期用 `YYYY-MM-DD HH:mm:ss`；人员用 `[{ "id": "ou_xxx" }]`；超链接用 URL 或 markdown link 字符串 |
| formula / lookup 创建失败 | 先读 [formula-field-guide.md](references/formula-field-guide.md) / [lookup-field-guide.md](references/lookup-field-guide.md)，再按 guide 重建请求 |
| `ignored_fields` / `READONLY` | 移除只读字段，只写存储字段 |
| `1254104` | 批量超过 200，分批调用 |
| `1254291` | 并发写冲突，串行写入并在批次间短暂等待 |
| `91403` | 无权限访问该 Base，按 `lark-shared` 权限流程处理，不要盲目重试 |

## 保留 Reference

- [lark-base-data-analysis-sop.md](references/lark-base-data-analysis-sop.md)：显式 CLI/诊断/写前读取的查询正确性 SOP；普通问数走 `base:assistant`
- [lark-base-data-query-guide.md](references/lark-base-data-query-guide.md) / [lark-base-data-query.md](references/lark-base-data-query.md)：显式 CLI 聚合与 DSL SSOT，不是自然语言问数默认入口
- [lark-base-cell-value.md](references/lark-base-cell-value.md)：记录 CellValue 构造
- [lark-base-field-json.md](references/lark-base-field-json.md)：字段 JSON 构造
- [formula-field-guide.md](references/formula-field-guide.md) / [lookup-field-guide.md](references/lookup-field-guide.md)：公式与 lookup 字段
- [lark-base-field-create.md](references/lark-base-field-create.md) / [lark-base-field-update.md](references/lark-base-field-update.md)：字段创建/更新命令级补充
- [lark-base-record-upsert.md](references/lark-base-record-upsert.md) / [lark-base-record-batch-create.md](references/lark-base-record-batch-create.md) / [lark-base-record-batch-update.md](references/lark-base-record-batch-update.md) / [lark-base-record-history-list.md](references/lark-base-record-history-list.md)：记录写入 JSON 与历史返回解释
- [lark-base-view-set-filter.md](references/lark-base-view-set-filter.md)：视图筛选 JSON
- [lark-base-form-detail.md](references/lark-base-form-detail.md) / [lark-base-form-submit.md](references/lark-base-form-submit.md) / [lark-base-form-questions-create.md](references/lark-base-form-questions-create.md) / [lark-base-form-questions-update.md](references/lark-base-form-questions-update.md)：表单详情、提交和复杂 JSON
- [lark-base-dashboard.md](references/lark-base-dashboard.md) / [dashboard-block-data-config.md](references/dashboard-block-data-config.md) / [lark-base-dashboard-block-get-data.md](references/lark-base-dashboard-block-get-data.md)：仪表盘、组件配置与图表结果协议
- [lark-base-workflow-guide.md](references/lark-base-workflow-guide.md) / [lark-base-workflow-schema.md](references/lark-base-workflow-schema.md)：workflow 入口与 steps JSON SSOT
- [lark-base-role-guide.md](references/lark-base-role-guide.md) / [role-config.md](references/role-config.md)：角色入口与权限 JSON SSOT

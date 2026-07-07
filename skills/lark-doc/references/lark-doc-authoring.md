# Lark Doc Authoring

本文件定义从零创作，以及对已有正文进行改写、润色、重组、补写和排版的流程。根 `SKILL.md` 负责场景与格式路由；本文件负责内容判断；格式文件定义表达语法；`create` / `update` 定义写入操作。

## Philosophy

文档是为读者服务的信息传递，不是作者的自我表达。唯一标准是：读者能否以最低成本获取并正确理解。

- **读者本位**：落地前先回答：读者是谁、为什么要读、带着什么任务来。按读者的任务组织内容，不按功能或作者视角罗列。
- **结构先行**：结论先行，先整体后局部；按逻辑分组与递进，依据关系选择列表、步骤或表格，使内容便于扫读。（特殊体裁除外）
- **极简表达**：默认使用能清楚表达关系的最简单形式；不损失信息地压缩文字；删冗余，用短句、动词和数据，在文字难以说清流程、交互或层级时用图。

## Hard Rules

以下规则约束所有阶段；具体路由、证据状态和组件要求在对应步骤展开。

1. **约束栈**：只采用 Prepare 选择的一个 content contract；需要平台原生写法时可追加一个 platform adapter。事实、法规与安全边界 > 用户硬约束 > 读者任务 > content contract > platform adapter > Presentation；后项不得牺牲或放宽前项。
2. **事实边界**：重要主张必须按 Prepare 的证据状态处理，不得把来源主张、推断或缺口写成已核验事实。
3. **表达一致**：同一对象、动作和状态保持同名；用户给出样例或已有文档时，在不违反更高优先级规则的前提下延续其有效结构、语气、术语和编号。**如使用编号或标题层级，全文保持一套一致体系，同级不跳号、不跳级，不混用互不兼容的编号体系**。
4. **授权与保真**：编辑时只改授权范围，保留其余内容和资源；除非用户明确要求重建，不用 `overwrite` 代替局部修改。

## Step Plan

**CRITICAL：按当前场景严格执行 `Prepare → Draft → Deliver`；条件式步骤仅在命中时执行。**

### Prepare

1. 明确读者任务、交付形态、文档生命周期、硬约束和禁区。编辑已有文档时，按 [`lark-doc-fetch.md`](lark-doc-fetch.md) 读取最小充分范围，并保留所有不在本次授权修改范围内的内容和资源。
2. 按下方 Route Template Index 选择且只读取一个 content contract；需要特定发布平台的原生写法时，再追加且只读取一个 platform adapter。混合内容意图作为 contract 的约束，不并读多个 contract；多平台交付共享 contract 与事实基础，但分别执行后续流程。
3. 盘点材料，只使用以下四种证据状态；不得把后三种写成已核验事实：
   - **已核验事实**：本任务已核对直接支持该陈述的可定位证据，而不只是确认某来源这样说；证据的时间 / 版本、范围和口径须匹配陈述。
   - **来源主张**：本任务只能确认用户或材料做出该陈述，尚未核对其真实性；正文保留来源归属。
   - **推断**：由已列明的事实或来源主张推出；正文能指出前提，结论强度不得超过前提。
   - **缺口**：证据缺失、冲突、过时或依赖未经确认的假设，导致关键输入或结论未定；能安全条件化或标注时继续，否则 `blocked`。
   时效性强、高风险或用户明确要求核验的信息，作为事实使用前必须核验来源、时间和口径；无法核验时降为来源主张或缺口，不得静默补全。
4. 形成内部 `Authoring Brief`，作为进入 Draft 的前置输入。Brief 可以只写一小段，但必须覆盖：
   - **目标**：目标读者、读者任务和期望的读后变化。
   - **内容**：所选 content contract、可选 platform adapter、核心承诺、内容脊柱，以及关键事实、证据和缺口。
   - **边界**：用户硬约束、必写 / 不写内容、授权范围、保真项和安全禁区。

### Draft

1. 按 `Authoring Brief`、所选 contract 和可选 adapter 完成格式中立的内容稿。先写清核心命题、关键事实与细节、各部分推进关系和真实取舍；每一节都应增加新的理解、证据、场景、判断或行动。内部记录不能代替缺口在读者可见正文中的实际处理。
2. 先修订结构、论证 / 叙事、信息密度和段落衔接，再精简套话、重复、模糊形容和来源堆砌；格式与组件不得反向改变内容判断。
3. 按根 `SKILL.md` 的格式规则选择 XML 或 Markdown，并只读取一个格式参考；未选择的格式文件不得读取。
4. 形成只包含 `presentation_mode` 的 `Presentation Decision`，再按该 mode、所选 contract 与可选 adapter 生成 release candidate；不预先声明或记录 blocks，单份 release candidate 禁止混用格式。`presentation_mode` 继承 content contract，且只能使用以下三种值；platform adapter 可替换非 `formal` 的 mode，但不得放宽 contract 的准确性、证据、安全、语气或组件限制。`formal` 只能由明确规定正式结构与组件边界的 content contract 提供，不能由 adapter 单独升级。mode 不决定 XML / Markdown，也不产生法律或发布效力。
     - **`formal`（表达非常正式）**：庄重、准确、简洁、直接，结构与措辞服从正式规范；只使用 contract 明示允许或限用的 block，不以 rich block、颜色或装饰制造正式感。
     - **`normal`（正常）**：清楚、可信、自然，按读者任务选择叙述、步骤、状态、责任和判据；默认使用最简单的有效结构，表格、代码、清单、画板等仅在降低理解、执行、出错或验收成本时使用。
     - **`rich`（表达非常丰富）**：允许更鲜明的声音、节奏、场景和视觉层次，但不得削弱事实与边界；在约束允许且有明确内容作用时，鼓励使用 `img`、`whiteboard`、`callout`、`grid` 等 rich block 丰富体验，不设数量配额。
     不因“更克制”“更活泼”或视觉丰富度的细分新增 mode；这些差异由 content contract 与可选 platform adapter 约束。
5. release candidate 生成后，执行 Draft Parse Gate，并结合返回的 profile 按下方质量检测表检查当前稿件。
   - release candidate 已有草稿文件时，直接执行 `lark-cli docs +script --command parse --content "@<草稿相对路径>" --format json`。
   - 命令必须成功，并返回 `data.profile.word_count`、`data.profile.char_count`、`data.profile.block_count` 和 `data.profile.blocks`。Parse Gate 只提供语法、基础统计和实际 block 清单，不代表前端视觉验收，也不代替质量判断。
   - 解析失败或质量检测未通过时，能用当前材料修复的直接修订；缺少关键事实、材料、授权或用户选择时标记 `Publish Gate = blocked`。任何正文变化都必须重新执行 Draft Parse Gate 和质量检测。
6. 只有最新 release candidate 解析成功、用户硬约束与质量检测全部通过，且检查后正文未再变化时，才标记 `Publish Gate = ready`。默认只保留必要的检查证据和未关闭问题，不生成或展示固定格式的内部报告。

### Deliver

1. 只有 `Publish Gate = ready` 时才写入；新建读取 [`lark-doc-create.md`](lark-doc-create.md)，编辑读取 [`lark-doc-update.md`](lark-doc-update.md)。离线草稿不写入。
2. 按 create / update 的规则执行写入和传输验证。仅重试同一 release candidate 的传输时无需重复内容检查；任何正文变化都必须返回 Draft 重新校验。
3. 最终只交付用户需要的结果，以及会影响使用的来源、未关闭缺口、失败 / 阻塞原因、异常和文档 URL / token。

## Route Template Index

先按读者的主要任务选择唯一 content contract，再按需追加一个 platform adapter。contract 决定内容任务、证据和体裁边界；adapter 只调整与其兼容的平台结构、语气和组件，不得替换或放宽 contract。两者的组件限制合并生效，禁止或限用条件取更严格者；仅提到、研究或分析某平台不触发 adapter。

### Content routes

| 文件名 | 主要读者任务 |
|-|-|
| [`route-workplace.md`](genres/route-workplace.md) | 组织决策、执行、留档 |
| [`route-report.md`](genres/route-report.md) | 数据 / 研究形成洞察 |
| [`route-knowledge.md`](genres/route-knowledge.md) | 理解、自学、一次操作 |
| [`route-media.md`](genres/route-media.md) | 告知公共事实 / 事件 / 人物 |
| [`route-opinion.md`](genres/route-opinion.md) | 形成并论证判断 |
| [`route-consumer.md`](genres/route-consumer.md) | 真实体验辅助选择 |
| [`route-marketing.md`](genres/route-marketing.md) | 建立信任并推动行动 |
| [`route-personal-brand.md`](genres/route-personal-brand.md) | 本人经历、能力、作品 |
| [`route-creative.md`](genres/route-creative.md) | 角色、冲突、情节 / 分支 |

### Optional platform adapter

| 文件名 | 主要读者任务 |
|-|-|
| [`route-platform.md`](genres/route-platform.md) | 按目标平台形成可发布成稿 |

## 质量检测表

写入前检查以下各项。重点记录未通过项及其正文证据；不要求固定报告格式。

| 检测项 | 通过标准 |
|---|---|
| 读者与范围 | 内容服务读者任务，核心命题和交付范围清楚，无无用章节 |
| 事实与缺口 | 重要主张可追溯；已核验事实有直接支持它的对应证据，来源主张有归属，推断有前提，缺口已补齐、条件化、标注或阻断发布 |
| 结构与体裁 | 各部分关系和顺序合理，完成 contract 与可选 adapter 的要求，无缺项、重复或近邻体裁混用 |
| 表达 | 表达具体、简练、术语一致；叙述未被列举化，标题、列表、表格和编号各司其职，并符合所选 mode、contract 与 adapter |
| 一致性 | 序号、编号与标题层级采用一套体系，同级不跳号、不跳级；颜色与视觉强调保持统一语义并符合所选 mode；同一对象、动作、状态和专有名词全文同名 |
| 字数与硬约束 | 用户硬约束全部满足；有明确字数或字符数要求时，以 `profile.word_count` / `profile.char_count` 的实测值为准，不自行估算 |
| Blocks 与组件 | `profile.block_count` 与 `profile.blocks` 反映实际 block；实际类型同时满足 contract 与 adapter 已声明的允许、限用和禁止条件，未声明限制的一方不得反推 allow-list；rich block 服务所选 mode 和真实信息关系 |
| 格式与解析 | 单份稿件格式单一，Draft Parse Gate 通过 |
| 保真 | 未授权内容和资源保持不变 |

未通过但可用当前材料修复时继续修订；缺少关键事实、材料、授权或用户选择时，`Publish Gate = blocked`。

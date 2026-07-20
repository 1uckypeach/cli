# provider: base

> **前置条件：** 先读 [`../../../lark-shared/SKILL.md`](../../../lark-shared/SKILL.md) 与 [`lark-agents SKILL.md`](../../SKILL.md)（认证、框架契约、动词与通用错误规则）。

**catalog 型** provider：对外固定暴露一个 Base Assistant，由 Base 服务自动路由到建表、仪表盘、工作流和数据分析等内部能力。公共 agent_ref 始终为 `base:assistant`；内部子 Agent ID 不属于公共协议，也不会出现在 CLI 参数或输出中。

## agent 发现

```bash
lark-cli agents list base --format json
lark-cli agents card base:assistant --operation all --format json
```

`agents list base` 离线返回唯一的 `base:assistant`。调用前仍应读取 card：Base 当前支持 send、task get/list/cancel、context list/get/delete；不支持文件输入、结构化 `input_required` 和 artifact download。

## scope 与身份前置

- 仅支持 `--as user`。`--as bot` 会在通用离线身份预检中拒绝，不发送请求（`--dry-run` 与真实调用一致）。
- Base Agent 仅使用 `base:agent:execute` 做 provider 级预检，并且只支持 user identity。scope 与公网 API pathPrefix 需要由 Base Adapter / 开放平台 owner 完成 provision；在正式发布前，以 `missing_scope` 返回中的 `missing_scopes` 与授权 hint 为权威，不要拿其它 `base:*` scope 代替。
- `base_token` 是 7 个操作的必填业务参数，通过 `--param base_token=<base-token>` 传递；它可以由 `meta.next` 续带。

## 参数与命令

参数声明以 `agents card base:assistant --operation all` 的实时输出为准：

| operation | 参数 |
|---|---|
| `send` | `base_token` 必填；`active_table_id` 可选 |
| `task_get` | `base_token` 必填；`context_id` 可选，用于覆盖任务关联的上下文并查询对应消息快照 |
| `task_list` | `base_token` 必填；`state` 可选；分页使用原生 `--page-token` / `--page-size`，会话使用 `--context-id` |
| `task_cancel` | `base_token` 必填 |
| `context_list` | `base_token` 必填；`status` 可选；分页使用原生 `--page-token` / `--page-size` |
| `context_get` / `context_delete` | `base_token` 必填 |

```bash
# 发起自动路由任务
lark-cli agents send base:assistant \
  --text "为这个项目创建任务表和进度仪表盘" \
  --param base_token=<base-token> \
  --param active_table_id=<table-id>

# 查询任务；使用 send 返回的 task_id
lark-cli agents task get base:assistant <task-id> \
  --param base_token=<base-token> \
  --param context_id=<context-id> \
  --watch --timeout 30s

# 在同一会话创建下一轮任务
lark-cli agents send base:assistant \
  --context-id <context-id> \
  --text "再按负责人汇总一次" \
  --param base_token=<base-token>

# 列出当前会话任务
lark-cli agents task list base:assistant \
  --context-id <context-id> \
  --param base_token=<base-token> \
  --page-size 20
```

## 行为特点与限制

- send 的每次逻辑调用生成新的幂等键；同一次 HTTP 传输重试复用请求体。TTL、并发冲突和重复请求返回值由 Adapter 去重契约负责。
- Send / task get 使用 Base Agent task schema v1：`pending → submitted`、`running → working`、`waiting_for_input → input_required`、`completed → completed`、`failed → failed`、`canceled → canceled`。未知状态或未知 schema 版本返回 `invalid_response`。
- `task get` 默认由服务端使用任务自身的 context；传入 `--param context_id=<context-id>` 时会作为 query 覆盖，用于查询该 context 下的消息快照，`--watch` 的每轮轮询都会复用该参数。
- task detail 的 `outputs` 在 Provider 层分流：`text` 转 text part，`data`（含 `qa_chart` / `qa_table` / `text_block` 与 `cli_*` fallback）原样保留为 data part，`clarification` 转 `input_required`，`artifact` 转结构化产物。各分支都保留 `output_id/source/group_id` 供轮询去重与来源关联；未知 output 作为完整 raw data part 保留。Provider 不执行其中内容，也不会自行下载 URL。
- `waiting_for_input` 由服务端状态决定；Provider 从 outputs 尾部选择最新的未提交必填 clarification，标准字段展示当前问题/选项，完整 questions/forms/buttons/action_params 保存在 `input_required.data`。当前写侧仍只支持文本续发，因此 `meta.next` 使用 `--text`，不生成 `--decision-id/--option`。
- Base Artifact 不是文件下载能力。`id/type/title/status` 映射为统一 Artifact 的 `id/kind/name/status`，`resource/revision/metadata` 保存在 `artifact.data`；Card 的 `artifact_download` 仍为 false。
- `task list` / `context list` 将统一分页参数映射为 Adapter 的 `cursor` / `limit` query。当前 Adapter 响应仍是裸数组且不返回下一页游标，因此 `meta.has_more` / `meta.page_token` 暂时不会出现。
- `context list` 不做 N+1 查询，服务端未返回任务数时 `task_count=0`；`context get` 使用服务端按新到旧排列的首个 task 作为 `active_task`。
- 本期不暴露 `skill_id`。Card 中的 skills 是能力说明，不代表可以指定内部子 Agent。
- `Files`、`DecisionID`、`OptionIDs` 会被明确拒绝；Card 中 `file_input`、结构化 `input_required` 回答、`artifact_download` 均为 false。任务仍可能返回 `state=input_required`，此时使用普通 `--text` 在同一 context/task 下续发。

## 服务端错误分类

Provider 按 Adapter 的稳定错误类别转换：不存在、无权限、任务终态、幂等冲突、限流、内部路由失败分别落到 `not_found`、`permission_denied`、`failed_precondition`、`conflict`、`rate_limit`、`server_error`。未识别类别返回 `invalid_response`；在 Adapter 数值错误码表冻结前，不根据 reason 文案猜测错误码。

## 参考

- [lark-agents](../../SKILL.md) — 框架契约与全部动词
- [agents list](../lark-agents-list.md) · [agents card](../lark-agents-card.md) · [agents send](../lark-agents-send.md) · [agents task](../lark-agents-task.md) · [agents context](../lark-agents-context.md)

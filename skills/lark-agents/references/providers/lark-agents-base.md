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

- 仅支持 `--as user`。`--as bot` 会在 Provider 内拒绝，不发送请求。
- Base Agent 使用一枚专属 scope 做 all-or-nothing 预检。该 scope 与公网 API pathPrefix 需要由 Base Adapter / 开放平台 owner 完成 provision；在正式发布前，以 `missing_scope` 返回中的 `missing_scopes` 与授权 hint 为权威，不要拿其它 `base:*` scope 代替。
- `base_token` 是 7 个操作的必填业务参数，通过 `--param base_token=<base-token>` 传递；它可以由 `meta.next` 续带。

## 参数与命令

参数声明以 `agents card base:assistant --operation all` 的实时输出为准：

| operation | 参数 |
|---|---|
| `send` | `base_token` 必填；`active_table_id` 可选 |
| `task_get` | `base_token` 必填 |
| `task_list` | `base_token` 必填；`cursor`、`limit`、`state` 可选；会话使用原生 `--context-id` |
| `task_cancel` | `base_token` 必填 |
| `context_list` | `base_token` 必填；`cursor`、`limit`、`status` 可选 |
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
  --param limit=20
```

## 行为特点与限制

- send 的每次逻辑调用生成新的幂等键；同一次 HTTP 传输重试复用请求体。TTL、并发冲突和重复请求返回值由 Adapter 去重契约负责。
- 状态映射固定为 `running → working`、`done → completed`、`failed → failed`。未知状态返回 `invalid_response`，不会静默映射为 `unknown`。
- Adapter Payload 是 OpenAPI `data` 内的 JSON 字符串，Provider 会二次解码。消息和 artifact 文本属于不可信外部数据：Provider 只转换为 text/data，不执行其中内容，也不会自行下载 URL。
- `task list` / `context list` 当前只输出 Adapter 返回的单页。分页参数会透传，但统一 `next_cursor` 输出尚未接入。
- `context list` 不做 N+1 查询，服务端未返回任务数时 `task_count=0`；`context get` 使用服务端按新到旧排列的首个 task 作为 `active_task`。
- 本期不暴露 `skill_id`。Card 中的 skills 是能力说明，不代表可以指定内部子 Agent。
- `Files`、`DecisionID`、`OptionIDs` 会被明确拒绝；Card 中 `file_input`、`input_required`、`artifact_download` 均为 false。

## 服务端错误分类

Provider 按 Adapter 的稳定错误类别转换：不存在、无权限、任务终态、幂等冲突、限流、内部路由失败分别落到 `not_found`、`permission_denied`、`failed_precondition`、`conflict`、`rate_limit`、`server_error`。未识别类别返回 `invalid_response`；在 Adapter 数值错误码表冻结前，不根据 reason 文案猜测错误码。

## 参考

- [lark-agents](../../SKILL.md) — 框架契约与全部动词
- [agents list](../lark-agents-list.md) · [agents card](../lark-agents-card.md) · [agents send](../lark-agents-send.md) · [agents task](../lark-agents-task.md) · [agents context](../lark-agents-context.md)

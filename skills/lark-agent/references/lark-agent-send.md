# agent send

> **前置条件：** 先读 [`../../lark-shared/SKILL.md`](../../lark-shared/SKILL.md)。调 send **前先读 [`agent card`](lark-agent-card.md)** 确认 `parameters`（空数组 = 无需 `--param`）；所需 scope 见对应 provider 文件（card 不含 scope），通用流程见 [前置准备](../SKILL.md)。

向远程 agent 发一条消息：不带 `--context-id/--task-id` 起一个**新任务**；带 `--context-id`（可选 `--task-id`）向同一多轮上下文**续发**（含回应 `input_required`：结构化决策用 `--decision-id/--option` 按 `option_id` 答、开放决策用 `--text`）。写操作。

> **`--file` 会把本地文件上传到远端 provider，内容离开本机、不可撤回。** CLI 强制确认门：真实 send 带 `--file` 须加 `--yes`，否则报 `confirmation_required`（exit 10）不上传；`--dry-run` 不上传、免 `--yes`。加 `--yes` 前先与用户确认。

## 命令

```bash
# 起新任务，立即返回 task_id/context_id/state（send 只 fire、不等结果）
lark-cli agent send <provider>:<agent_id> --text "<消息内容>"
# 轮询进度用 task get --watch（照 meta.next 给的命令，默认有界 30s）：
lark-cli agent task get <provider>:<agent_id> <task-id> --watch --timeout 30s

# 客户端预演：本地校验并打印将发的请求，不调 API（永远可用）
lark-cli agent send <provider>:<agent_id> --text "x" --dry-run

# 多轮续发（开放 input_required / 自由文本追问）：向同一会话/任务续发
lark-cli agent send <provider>:<agent_id> --context-id <ctx-id> --task-id <task-id> --text "<答复>"

# 回应结构化决策（input_required 带 options[]）：按 option_id 选，无需 --text
lark-cli agent send <provider>:<agent_id> --context-id <ctx-id> --task-id <task-id> --decision-id <decision_id> --option <option_id>

# 带文件（外发到远端；上传成功后才发消息，任一文件失败即中止）
lark-cli agent send <provider>:<agent_id> --text "看这份表" --file ./report.xlsx
```

## 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `<agent_ref>` | 是 | `<provider>:<agent_id>` |
| `--text` | 视情况 | 消息正文。起任务 / 开放决策必填（空报 `invalid_argument`，exit 2）；用 `--decision-id` + `--option` 回答选项决策时可省 |
| `--param key=value` | 视 card | 可重复；据 card `parameters` 校验（声明为空时传任何 `--param` 都报未知参数） |
| `--file <path>` | 否 | 可重复；**文件外发**到远端 provider（内容离机、不可撤回）。仅相对路径（限 CWD 内，约束见 lark-shared 安全规则）。真实 send 须配 `--yes`（见下）；`--dry-run` 时不上传、免 `--yes`，仅在 `would_send.files` 列出 |
| `--yes` | 视上 | 确认 `--file` 外发；真实 send 带 `--file` 时必填，否则报 `confirmation_required`（exit 10）不上传 |
| `--context-id` | 否 | 续同一会话；省略=新会话，结果回显新 `context_id` |
| `--task-id` | 否 | 回应某任务；**须与 `--context-id` 同用**，否则报错 |
| `--decision-id` | 否 | 回答 `input_required` 结构化决策的目标 `decision_id`（见 `agent task get` 的 `input_required`）；**须与 `--context-id/--task-id` 同用** |
| `--option <option_id>` | 否 | 可重复；回答决策选中的 `option_id`（单选 1 个、多选多个）；**须配 `--decision-id`**。开放（无 options）决策改用 `--text` |
| `--dry-run` | 否 | 本地校验+打印请求，不调 API（永远可用，且跳过 scope preflight 与 `--file` 确认门） |
| `--as` / `--format json\|pretty` / `--jq` | 否 | 通用；默认 `json` |

## 输出

send 立即返回当前任务。示例（example，真实输出，`agent send example:echo --text "分析一下上季度销售数据"`——example 的任务发出即完成，故直接返回终态；真实 provider 未终态时返回 `submitted`/`working`，`meta.next` 会推有界轮询命令 `task get <agent_ref> <task-id> --watch --timeout 30s`）：

```json
{ "ok": true, "identity": "bot",
  "data": {
    "task_id": "task_e79dc35e3afd", "context_id": "ctx_5d0e1e951b8e",
    "state": "completed", "is_terminal": true,
    "messages": [
      { "role": "user", "parts": [ { "type": "text", "text": "分析一下上季度销售数据" } ] },
      { "role": "agent", "parts": [ { "type": "text", "text": "分析一下上季度销售数据" } ] }
    ]
  },
  "meta": { "next": [ { "label": "查看任务详情与产物",
    "command": "lark-cli agent task get example:echo task_e79dc35e3afd" } ] } }
```

`meta.next` 是建议命令：无 `template` 字段的可直接照抄——如上例的 `task get <agent_ref> <task-id> --watch`，照它轮询到停轮询条件（权威定义见 [SKILL.md 核心概念](../SKILL.md)）；`template:true` 的先整体替换 `<...>` 占位符——任务停在 `input_required` 时给的就是这类续发命令，照 [SKILL.md 工作流](../SKILL.md) 第 4 步续发（该态是否会出现见 provider 文件的能力特例）。

## 错误目录（精确断言 `subtype`+exit）

本地校验（不发请求）：

| 触发 | subtype | exit | message / hint（真实输出） |
|---|---|---|---|
| 缺 `--text` | invalid_argument | 2 | `--text 不能为空`；hint `补充 --text "<消息内容>" 后重发` |
| `--task-id` 缺 `--context-id` | invalid_argument | 2 | `--task-id 需与 --context-id 一起使用` |
| 传了未声明的 `--param` | invalid_argument | 2 | `未知参数 foo（该 agent 未声明此参数）`；hint 指向 `agent card`；`param` 字段为 `param:foo` |
| 未知 scheme | invalid_argument | 2 | `未知的 agent provider '<scheme>'，当前支持: example`——message 列出当前已注册 scheme 全集；hint 指向 `agent list` |
| `--file` 真实 send 缺 `--yes` | confirmation_required | 10 | `--file 会把本地文件外发上传到远端 agent（内容离开本机，不可撤回）`；hint `确认要外发这些文件后，加 --yes 重发`。仅在 provider 支持 file_input 时触发；`--dry-run` 免此门 |
| user 身份缺 scope | missing_scope | 3 | all-or-nothing：token 缺任一 scope 即报 `当前 user 身份缺少本命令所需 scope: <逗号分隔的全部缺失>`；附 `missing_scopes`（该 agent 缺失的全部 scope）、hint = 可照抄的 `auth login --scope`（hint 语义见 [SKILL.md 前置准备](../SKILL.md)）。bot 身份与 `--dry-run` 跳过此检查 |

服务端错误：通用规则见 [SKILL.md「服务端错误」](../SKILL.md)，业务错误码目录见对应 provider 文件。

> `data.state=failed/rejected` 是**任务失败**（`ok:true`，别当传输错误重试）；error 对象才是传输/协议失败。

## 参考

- [lark-agent](../SKILL.md) — agent 全部动词
- [provider-example](providers/lark-agent-example.md) — provider 业务事实

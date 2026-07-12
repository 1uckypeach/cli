# agent send

> **前置条件：** 先读 [`../../lark-shared/SKILL.md`](../../lark-shared/SKILL.md)。调 send **前先查参数**：card 的 `has_parameters` 含 `send` 时，跑 `agent card <ref> --operation send` 拿参数声明（不含则无需任何 `--param`）；所需 scope 见对应 provider 文件（card 不含 scope），通用流程见 [前置准备](../SKILL.md)。

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
| `--param key=value` | 视声明 | 可重复；按 **send 这个动词**的参数声明校验（`--operation send` 查看）。校验规则与 object 点路径/JSON 传法的唯一权威见 [card「字段语义」](lark-agent-card.md)；错误一次报全且每条带完整声明（见下方错误目录） |
| `--file <path>` | 否 | 可重复；**文件外发**到远端 provider（内容离机、不可撤回）。本地先校验：仅相对路径（限 CWD 内）、文件必须存在且非目录，违规一次报全（`invalid_argument`，exit 2，dry-run 同样校验）。真实 send 须配 `--yes`（见下）；`--dry-run` 时不上传、免 `--yes`，仅在 `would_send.files` 列出 |
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
    "task_id": "task_ad9acc62af31", "context_id": "ctx_fb95c586fa03",
    "state": "completed", "is_terminal": true,
    "created_at": "2026-07-12T09:57:58Z", "updated_at": "2026-07-12T09:57:58Z",
    "messages": [
      { "role": "user", "parts": [ { "type": "text", "text": "分析一下上季度销售数据" } ] },
      { "role": "agent", "parts": [ { "type": "text", "text": "分析一下上季度销售数据" } ] }
    ]
  },
  "meta": { "next": [ { "label": "查看任务详情与产物",
    "command": "lark-cli agent task get example:echo task_ad9acc62af31 --as bot" } ] } }
```

`meta.next` 是建议命令（**显式传过 `--as` 时会原样带上同身份**，保证非默认身份的链条照抄可复现；没传则不带，下一条与 shortcut 家族一致走默认身份解析）：无 `template` 字段的可直接照抄——如上例的 `task get example:echo task_ad9acc62af31 --as bot`（终态任务直接看详情）；未终态时推的是 `task get ... --watch --timeout 30s`，同样照抄、轮询到停轮询条件（权威定义见 [SKILL.md 核心概念](../SKILL.md)）。`template:true` 的含 `<...>` 占位符，先**整体替换**再执行，出现在三类场景：`input_required` 续发命令（照 [SKILL.md 工作流](../SKILL.md) 第 4 步，该态是否出现见 provider 文件）、`auth_required` 授权后的重查命令、产物下载 / 必填参数的占位（如 `-o <保存路径>`）。

## 错误目录（精确断言 `subtype`+exit）

本地校验（不发请求）：

| 触发 | subtype | exit | message / hint（真实输出） |
|---|---|---|---|
| 缺 `--text` | invalid_argument | 2 | `--text 不能为空`；hint `补充 --text "<消息内容>" 后重发；若在回答决策，用 --option <option_id> 选择` |
| `--task-id` 缺 `--context-id` | invalid_argument | 2 | `--task-id 需与 --context-id 一起使用` |
| 传了未声明的 `--param` | invalid_argument | 2 | `未知参数 foo（send 可用参数: ...）`；参数声明在别的动词上时报 `不适用于 send（它声明在: task_list）`；`param` 字段为 `param:foo` |
| 多处参数问题 | invalid_argument | 2 | 一次报全：message 为 `send 参数校验失败：N 处问题（详见 params）`，`params[]` 每条含 `{name, reason, spec?}`（已声明参数的违规带 spec = 完整声明，可据此直接修；未知/重复/格式错的条目看 reason/suggestions） |
| enum / 类型 / 范围violation | invalid_argument | 2 | `取值须为 low\|normal\|high` / `需为 integer` / `须在 1..100 范围内`——错误消息即修复指令 |
| 未知 scheme | invalid_argument | 2 | message 形如 `未知的 agent provider '<scheme>'，当前支持: <已注册 scheme 全集>`（列表随注册变化，勿硬编码断言）；hint 指向 `agent list` |
| `--file` 路径非法/不存在/是目录 | invalid_argument | 2 | `非法的 --file 路径: <path>（仅接受 CWD 内的相对路径）`（或 `文件不存在或不可读`/`是目录`，多个违规一次报全）；hint `--file 只接受当前目录内的相对路径且文件必须存在，逐条修正后重发`。先于能力门与确认门 |
| `--file` 真实 send 缺 `--yes` | confirmation_required | 10 | `--file 会把本地文件外发上传到远端 agent（内容离开本机，不可撤回）`；hint `确认要外发这些文件后，加 --yes 重发`。仅在 provider 支持 file_input 时触发；`--dry-run` 免此门 |
| 缺 scope（user/bot） | missing_scope | 3 | 本地 preflight，附 `missing_scopes` + 可照抄 hint；语义与修复路径（user≠bot）的唯一权威见 [SKILL.md 前置准备](../SKILL.md) 第 2/3 条。`--dry-run` 跳过此检查（bot 不跳过，仅 best-effort 降级） |

服务端错误：通用规则见 [SKILL.md「服务端错误」](../SKILL.md)，业务错误码目录见对应 provider 文件。

> `data.state=failed/rejected` 是**任务失败**（`ok:true`，别当传输错误重试）；error 对象才是传输/协议失败。

## 参考

- [lark-agent](../SKILL.md) — agent 全部动词
- [provider-example](providers/lark-agent-example.md) — provider 业务事实

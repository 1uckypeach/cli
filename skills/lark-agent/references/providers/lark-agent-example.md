# provider: example

> **前置条件：** 先读 [`../../../lark-shared/SKILL.md`](../../../lark-shared/SKILL.md) 与 [`lark-agent SKILL.md`](../../SKILL.md)（框架契约、动词、通用错误规则）。

**catalog 型** provider：仓库内置的离线演示 agent（内存 mock，零网络，无需任何平台配置）。agent_ref = `example:<agent_id>`。定位有二：完整体验 `agent` 命令树的全链路（list → card → send → task → context），以及作为真实 provider 接入的参照实现。**它不调用任何远程服务**——任务发出即完成（终态），状态存于本机临时快照，跨命令可查。

## agent 发现（可枚举）

catalog 型必可枚举，`agent list example` 直接列全部 agent（含 name/description），无需任何控制台。真实输出：

```json
{
  "ok": true,
  "data": {
    "agents": [
      {
        "agent_ref": "example:echo",
        "name": "复读机",
        "description": "把你发的话原样复读一遍（同一会话续发时带轮次，证明上下文记忆）。最小能力集示范。"
      },
      {
        "agent_ref": "example:reporter",
        "name": "报表生成器",
        "description": "对任意请求产出一份内联 CSV 报表 artifact，示范 artifact 下载与任务取消链路。"
      }
    ]
  },
  "meta": { "count": 2 }
}
```

## scope 与身份前置

**scope 全集为空**——example 零网络、不打任何 OAPI，user/bot 两种身份都无需授权，scope preflight 恒通过。这是本 provider 独有的：**真实 provider 会声明非空 RequiredScopes**（user 身份缺任一 scope 时命令本地报 `missing_scope`，照抄 hint 授权），bot 身份也可能有服务端前置（card `identity` 里 bot 条目的 `precondition` 会写明）。example 的 card 里 bot 条目无 precondition。

## 能力特例（echo vs reporter——能力矩阵的活教材）

两个 agent 的 capabilities 刻意不同，`agent card` 读到什么就只能调什么：

| capability | `example:echo` | `example:reporter` | 差异含义 |
|---|---|---|---|
| `task_get` / `task_list` / `context_list` / `context_get` / `context_delete` | true | true | 两者都支持查/列任务与多轮会话（列/查/删会话）|
| `task_cancel` | **false** | true | 对 echo 发 cancel 被命令层门控直接拒（见下方错误样例，不发任何请求）；对 reporter 的 cancel 会真正派发（但 mock 任务即时终态，见下方 failed_precondition 样例） |
| `file_input` | **false** | true | echo 带 `--file` 报 `unsupported_capability`；reporter 接收附件并在回复里确认 |
| `artifact_download` | **false** | true | 只有 reporter 产出 artifact（内联 CSV，`kind=text`，下载输出 `mime=text/csv`、`suggested_name=quarterly_report.csv`） |
| `input_required` | false | true | 两者的任务都即时完成、实际不会停在 `input_required`；reporter 声明 true 属"声明了但用不到"（无害方向），echo 按最小集诚实声明 false |

行为特点：

- **任务发出即完成**：send 返回的 `state` 恒为 `completed`（终态），`meta.next` 直接给"查看任务详情与产物"，不会推轮询命令——观察 `--watch` / 非终态行为要靠真实 provider。
- **多轮记忆可验证**：同一 `--context-id` 续发时，echo 的回复从第 2 轮起带轮次标记（如 `换个角度再说一遍（第 2 轮）`），跨命令证明上下文确实在工作。
- **不支持向已有任务续发**：带 `--task-id` 续发报 `failed_precondition`（任务发出即终态），hint 引导去掉 `--task-id` 用 `--context-id` 起新一轮。

## 错误样例（真实输出）

未知 agent_id（`agent card example:nonexistent`，exit 2）——目录外的 id 本地报错，hint 指回枚举命令：

```json
{
  "ok": false,
  "error": {
    "type": "validation",
    "subtype": "invalid_argument",
    "message": "未知的 example agent 'nonexistent'",
    "hint": "运行 lark-cli agent list example 查看可用 agent"
  }
}
```

cancel 能力门控（`agent task cancel example:echo <task-id>`，exit 2，不发请求）：

```json
{
  "ok": false,
  "error": {
    "type": "validation",
    "subtype": "unsupported_capability",
    "message": "agent 'example:echo' 不支持 'task cancel'（capability task_cancel=false）",
    "hint": "运行 lark-cli agent card example:echo 查看支持的能力"
  }
}
```

对终态任务 cancel（`agent task cancel example:reporter <task-id>`，reporter 虽 `task_cancel=true`，但 mock 任务即时终态，exit 2）：

```json
{
  "ok": false,
  "identity": "bot",
  "error": {
    "type": "validation",
    "subtype": "failed_precondition",
    "message": "任务 'task_3fc5b3f9bee3' 已处于终态 completed，无法取消",
    "hint": "终态任务不可取消；用 lark-cli agent task get example:reporter task_3fc5b3f9bee3 查看结果"
  }
}
```

## 参考

- [lark-agent](../../SKILL.md) — 框架契约与全部动词
- [agent list](../lark-agent-list.md) · [agent card](../lark-agent-card.md) · [agent send](../lark-agent-send.md) · [agent task](../lark-agent-task.md) · [agent context](../lark-agent-context.md)

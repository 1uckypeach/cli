# agent context list / get / delete

> **前置条件：** 先读 [`../../lark-shared/SKILL.md`](../../lark-shared/SKILL.md)（含高危 exit-10 确认机制）。

管理远程 agent 的**多轮上下文（会话）**。一个 context（`context_id`）串起同一会话里的多个任务；需 card `multi_turn=true`。续发/追问在 [`agent send --context-id`](lark-agent-send.md)，不在此。三个动词都要求该 provider 的全部 scope（all-or-nothing；缺任一即本地报 `missing_scope`，照抄 hint 授权；scope 全集见 provider 文件）。

## context list — 列会话

```bash
lark-cli agent context list <provider>:<agent_id>                    # 默认 JSON 信封
lark-cli agent context list <provider>:<agent_id> --format pretty    # 带表头 TSV
```

输出 `{ contexts: [ { context_id, created_at?, title? } ] }`，`meta.count`。只读。

**单页语义**：只返回服务端第一页，分页未透出——会话很多时结果会静默截断，找不到目标 context 别据此断言不存在。

## context get — 查会话详情

```bash
lark-cli agent context get <provider>:<agent_id> <ctx-id>
```

输出单个 context 详情（含其下 `tasks[]`，每项 `{task_id, state, is_terminal}`）。只读。

## context delete — 删除会话（高危，需 --yes）

删除**不可逆**，是 high-risk-write。缺 `--yes` 直接返回 `confirmation_required`（exit 10），不发请求。

```bash
# 缺 --yes → exit 10，不执行
lark-cli agent context delete <provider>:<agent_id> <ctx-id>

# 确认删除
lark-cli agent context delete <provider>:<agent_id> <ctx-id> --yes
```

缺 `--yes` 的真实输出（exit 10）：

```json
{
  "ok": false,
  "error": {
    "type": "confirmation",
    "subtype": "confirmation_required",
    "message": "agent context delete requires confirmation",
    "hint": "add --yes to confirm",
    "risk": "high-risk-write",
    "action": "agent context delete"
  }
}
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `<agent_ref> <ctx-id>` | 是 | 两个位置参数 |
| `--yes` | 是（删除） | 确认高危操作；不加则 exit 10 |
| `--as` / `--format json\|pretty` / `--jq` | 否 | 通用；默认 `json` |

删除成功输出 `{ context_id, deleted: true }`。删除后再 get 该会话报 not_found。

## 错误目录

| 触发 | subtype | exit | message（示例） |
|---|---|---|---|
| `context delete` 缺 `--yes` | confirmation_required | 10 | 见上方真实输出 |
| user 身份缺 scope | missing_scope | 3 | all-or-nothing：缺该 provider scope 全集里任一即本地报，`missing_scopes` 列全部缺失；照抄 hint 授权 |
| ctx id 不存在 | 依 provider | 1 或 2 | 本地目录型（example）报 `invalid_argument`（exit 2，hint 指回 `context list`）；真实 provider 服务端资源不存在通常为 `not_found`（exit 1）。先 `context list <agent_ref>` 核对 |
| 未知 scheme / 非法 agent_ref | invalid_argument | 2 | 见 [send 错误目录](lark-agent-send.md) |

## 参考

- [lark-agent](../SKILL.md) — agent 全部动词
- [provider-example](providers/lark-agent-example.md) — provider 业务事实

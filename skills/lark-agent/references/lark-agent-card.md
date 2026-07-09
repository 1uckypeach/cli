# agent card

> **前置条件：** 先读 [`../../lark-shared/SKILL.md`](../../lark-shared/SKILL.md)（认证、身份、安全规则）。

取并展示一个 agent 的能力卡片：`capabilities`（能调哪些动词）、`parameters`（`send` 要带哪些 `--param`）、`identity`（支持的 `--as` 及前置条件）。**调任何动词前先读 card**——这是决定"能调什么、要传什么"的唯一依据。card 是否本地合成（离线可用）是 provider 事实，见对应 provider 文件。只读。

> card **不含 scope 声明**——scope 是内部注册项，只喂给 preflight。user 身份缺 scope 时命令会本地报 `missing_scope`（照抄 hint 一次配齐）；scope 全集见对应 provider 文件，通用流程见 [lark-agent 前置准备](../SKILL.md)。

## 命令

```bash
# 默认 JSON 信封（程序化解析用这个）
lark-cli agent card <provider>:<agent_id> --format json

# 人类可读
lark-cli agent card <provider>:<agent_id> --format pretty

# 只取 capabilities
lark-cli agent card <provider>:<agent_id> --jq '.data.capabilities'
```

## 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `<agent_ref>` | 是 | `<provider>:<agent_id>` |
| `--format json\|pretty` | 否 | 默认 `json`；`--jq` 会强制 JSON；其余值报 `invalid_argument` |
| `--as user\|bot` | 否 | 身份 |

## 输出

示例（example，真实输出，`agent card example:echo`）：

```json
{
  "ok": true,
  "identity": "user",
  "data": {
    "provider": "example",
    "provider_label": "Example 演示 agent（内存 mock，零网络）",
    "agent_id": "echo",
    "name": "复读机",
    "description": "把你发的话原样复读一遍（同一会话续发时带轮次，证明上下文记忆）。最小能力集示范。",
    "capabilities": {
      "artifact_download": false,
      "context_delete": true,
      "context_get": true,
      "context_list": true,
      "file_input": false,
      "input_required": false,
      "task_cancel": false,
      "task_get": true,
      "task_list": true
    },
    "identity": [
      { "type": "user" },
      { "type": "bot" }
    ],
    "parameters": [],
    "agent_id_source": "运行 lark-cli agent list example 查看内置演示 agent 及其 agent_ref（无需任何平台配置）"
  }
}
```

## 字段语义与消费方式

- **`capabilities`**：9 键能力矩阵。为 `false` 的动词不要调——如 `task_cancel=false` 时 `agent task cancel` 直接报 `unsupported_capability`（exit 2），不发请求。`input_required=false` = 该 agent 不会进 `input_required` 态（追问的实际行为见 provider 文件）。`--dry-run` 是客户端行为，不在 capabilities 里，永远可用。
- **`identity`**：支持的 `--as` 身份；带 `precondition` 的身份要先满足前置条件（典型是渠道白名单，见 provider 文件）。
- **`parameters`**：`send --param` 的声明。空数组 = 不需要任何 `--param`；传未声明的 `--param` 会报 `invalid_argument`。
- **`name` / `description`**：部分 provider（典型是 catalog 型）的 card 带每 agent 的名称与描述；没有则据 `provider_label` + `agent_id` 向用户描述。
- **`agent_id_source`**：拿 agent_id 的路径文案，用户没有 agent_id 时照这个引导。
- 未知 agent_ref：catalog 型 provider 对不在目录里的 id 本地报 `invalid_argument`（exit 2，真实样例见 [provider-example](providers/lark-agent-example.md)）。

## 错误目录

本地校验（不发请求）：

| 触发 | subtype | exit | message / hint（真实输出） |
|---|---|---|---|
| 畸形 agent_ref（如 `agent card no-colon`） | invalid_argument | 2 | `agent_ref 格式应为 <provider>:<agent_id>`；hint `agent_ref 形如 <scheme>:<agent_id>，如 example:echo` |
| 非法 `--format`（如 `--format xml`） | invalid_argument | 2 | `不支持的 --format 值 "xml"`；hint `合法值: json \| pretty`；`param` 字段为 `--format` |
| catalog 型未知 agent_id | invalid_argument | 2 | 真实样例见 [provider-example「错误样例」](providers/lark-agent-example.md) |

## 参考

- [lark-agent](../SKILL.md) — agent 全部动词
- [provider-example](providers/lark-agent-example.md) — provider 业务事实

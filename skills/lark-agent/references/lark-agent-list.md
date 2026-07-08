# agent list

> **前置条件：** 先读 [`../../lark-shared/SKILL.md`](../../lark-shared/SKILL.md)（认证、身份、安全规则）。

发现层命令。无参数时列出已注册的 provider 及其元数据，**不调用任何 API**；带 scheme 时枚举该 provider 下的 agent 实例（catalog 型必可枚举；instance 型是否支持见 provider 文件）。只读。

## 命令

```bash
# 列 provider（默认 JSON 信封）
lark-cli agent list

# 二级发现：枚举某 provider 下的 agent
lark-cli agent list <scheme>

# 人类可读（带表头 TSV）
lark-cli agent list --format pretty
```

## 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `[scheme]` | 否 | 省略=列 provider；给定=枚举该 provider 下的 agent |
| `--format json\|pretty` | 否 | 默认 `json`；`pretty` 为带表头 TSV |
| `--jq` | 否 | jq 过滤（强制 JSON） |

## 输出（`agent list`）

`data.providers[]` 每个已注册 provider 一条。示例（example，真实输出；完整 provider 清单见 [SKILL.md「Provider 目录」](../SKILL.md)）：

```json
{
  "ok": true,
  "data": {
    "providers": [
      {
        "scheme": "example",
        "label": "Example 演示 agent（内存 mock，零网络）",
        "agent_ref_format": "example:<agent_id>",
        "kind": "catalog",
        "agent_id_source": "运行 lark-cli agent list example 查看内置演示 agent 及其 agent_ref（无需任何平台配置）"
      }
    ]
  }
}
```

字段消费方式：

- **`agent_ref_format`**：告诉用户 agent_ref 怎么写（`<provider>:<agent_id>`，`<agent_id>` 整体替换）。
- **`agent_id_source`**：拿 agent_id 的路径文案，用户没有 agent_id 时照这个引导。
- **`kind`**：`catalog` = ref 指向目录内条目，**必可枚举**（`agent list <scheme>` 注册期强制支持）；`instance` = ref 指向一个具体 agent 实例，能否枚举取决于服务端 List API（见 provider 文件）。

## 二级发现（`agent list <scheme>`）

- provider 支持枚举（catalog 型必支持）→ 返回 `{"agents": [{agent_ref, name, description?}]}`，`meta.count`。示例（example，真实输出）：

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

- provider 不支持枚举（部分 instance 型）→ 本地报错 `unsupported_capability`（exit 2），message 为 `provider '<scheme>' 暂不支持列举 agent`，hint 直接给出该 provider 的 agent_id 获取路径（即 `agent_id_source` 文案）——别编清单、别重试，把 hint 原样转达用户。

## 错误目录

| 触发 | subtype | exit | message / hint（真实输出） |
|---|---|---|---|
| 未知 scheme（如 `agent list nosuch`） | invalid_argument | 2 | `未知的 agent provider 'nosuch'，当前支持: example`——message 列出当前已注册 scheme 全集；hint `用 lark-cli agent list 查看可用 provider` |
| `agent list <scheme>`（该 provider 不支持枚举） | unsupported_capability | 2 | 见上方「二级发现」说明 |

## 参考

- [lark-agent](../SKILL.md) — agent 全部动词
- [provider-example](providers/lark-agent-example.md) — provider 业务事实

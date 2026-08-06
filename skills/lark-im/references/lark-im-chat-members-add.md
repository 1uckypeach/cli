# +chat-members-add

将用户和/或机器人拉入一个已存在的群聊。

映射到 `POST /open-apis/im/v1/chats/{chat_id}/members`，用两个 flag（`--users`/`--bots`）取代原生命令需要的两个独立 JSON blob（`--params` 里的 `chat_id`/`member_id_type`/`succeed_type`，`--data` 里的 `id_list`），并固定 `member_id_type=open_id`——调用方不需要关心"用户 ID 格式要和 member_id_type 保持一致"这条隐性规则。

## 命令格式

```bash
lark-cli im +chat-members-add --chat-id <oc_xxx> \
  [--users <ou_a,ou_b,...>] [--bots <cli_a,cli_b,...>] \
  [--succeed-type 0|1] --as user|bot
```

## 参数

| Flag | 必填 | 说明 |
|---|---|---|
| `--chat-id` | 是 | 目标群 `chat_id`（`oc_xxx`） |
| `--users` | 否（与 `--bots` 至少填一个） | 逗号分隔的用户 `open_id`（`ou_xxx`），最多 50 个 |
| `--bots` | 否（与 `--users` 至少填一个） | 逗号分隔的机器人 `app_id`（`cli_xxx`），最多 5 个 |
| `--succeed-type` | 否，默认 `1` | `1`：可加入的成员正常加入，无法加入的走 `invalid_id_list`/`not_existed_id_list`/`pending_approval_ids`，调用本身不失败；`0`：只要有一个成员无法加入，整个调用失败 |

## 输出

| 字段 | 说明 |
|---|---|
| `chat_id` | 目标群 `chat_id` |
| `total` | 请求加入的 ID 总数 |
| `success_count` | 真正成为成员的 ID 数 |
| `failure_count` | `total - success_count` |
| `succeeded_ids` | 真正成为成员的 ID 列表 |
| `invalid_id_list` | 已离职、不可见、或应用未激活的 ID |
| `not_existed_id_list` | ID 本身不存在 |
| `pending_approval_ids` | 等待群主/管理员审批的 ID（尚未真正成为成员） |

成功（三个失败列表均为空）：

```json
{
  "chat_id": "oc_xxx",
  "total": 2,
  "success_count": 2,
  "failure_count": 0,
  "succeeded_ids": ["ou_aaa", "cli_bbb"],
  "invalid_id_list": [],
  "not_existed_id_list": [],
  "pending_approval_ids": []
}
```

部分失败（`invalid_id_list`、`not_existed_id_list`、`pending_approval_ids` 任意一个非空）：退出码非零，`ok:false`：

```json
{
  "ok": false,
  "chat_id": "oc_xxx",
  "total": 4,
  "success_count": 1,
  "failure_count": 3,
  "succeeded_ids": ["ou_aaa"],
  "invalid_id_list": ["ou_ccc"],
  "not_existed_id_list": ["ou_ddd"],
  "pending_approval_ids": ["ou_eee"]
}
```

## Identity 约束

调用方（user 或 bot）必须已经在目标群里；`--as bot` 调用时，被添加的用户必须在应用的可用范围内；仅群主/管理员可加人的群，调用方必须是群主/管理员，或拥有 `im:chat:operate_as_owner` scope 的创建者机器人。

## AI 使用注意事项

- **成功不等于全部加入**：`invalid_id_list`、`not_existed_id_list`、`pending_approval_ids` 三个字段都要检查，不能只看 `invalid_id_list`——`not_existed_id_list`（ID 不存在）和 `pending_approval_ids`（等待群主/管理员审批，尚未真正成为成员）同样意味着对应 ID 没有真正加入群聊。只要三者任意一个非空（无论 `succeed_type` 是 0 还是 1），都必须把里面的 ID 明确告知用户——不要把"命令 exit 0"误读成"所有人都加进去了"。
- **`succeed_type=0` 严格模式下的失败是 API 错误，不是 ledger**：此时任一 ID 无法加入会导致整体请求失败，错误从 API 层直接抛出，而不会走到本命令输出的这三个字段。
- 需要 `union_id`/`user_id` 格式的用户 ID 时，本 shortcut 不支持，改用原生 `im chat.members create` 命令。

## 典型用法

```bash
# 加一个真实用户
lark-cli im +chat-members-add --chat-id oc_xxx --users ou_aaa --as user

# 同时加用户和机器人
lark-cli im +chat-members-add --chat-id oc_xxx --users ou_aaa,ou_bbb --bots cli_ccc --as bot

# 严格模式：任一成员无法加入则整体失败
lark-cli im +chat-members-add --chat-id oc_xxx --users ou_aaa --succeed-type 0 --as user
```

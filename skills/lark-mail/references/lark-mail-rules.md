# 收信规则

管理自动处理收到邮件的规则。规则写操作需使用真实 `rule_id`，不要猜测 ID。规则写操作执行前需按 SKILL.md 的写操作确认规则获得用户确认。

## 主题包含文本 → 标记为已读

```bash
# 1. 创建规则：主题包含指定文本时标记为已读
lark-cli mail user_mailbox.rules create --as user \
  --params '{"user_mailbox_id":"me"}' \
  --data '{"name":"<rule_name>","is_enable":true,"ignore_the_rest_of_rules":false,"condition":{"match_type":1,"items":[{"type":6,"operator":1,"input":"<subject_text>"}]},"action":{"items":[{"type":3}]}}'

# 2. 验证规则
lark-cli mail user_mailbox.rules list --as user \
  --params '{"user_mailbox_id":"me"}'

# 3. 删除规则
lark-cli mail user_mailbox.rules delete --as user \
  --params '{"user_mailbox_id":"me","rule_id":"<rule_id>"}'
```

Quick codes above: condition `type=6` = subject, `operator=1` = contains, action `type=3` = mark as read.

## 调整规则顺序

`reorder` 可只传需要提前或调整相对顺序的部分 `rule_id`。CLI 会先读取当前完整规则列表，并把未输入的规则按当前相对顺序追加到请求末尾；输入可使用唯一的规则 ID 前缀，前缀匹配多个规则或未匹配任何规则时会在本地报错且不会调用 reorder。重复 ID 同样会在本地报错。

```bash
lark-cli mail user_mailbox.rules reorder --as user \
  --params '{"user_mailbox_id":"me"}' \
  --data '{"rule_ids":["<rule_id_to_place_first>","<rule_id_to_place_second>"]}'
```

## 原生 API

收信规则走 `user_mailbox.rules` 资源。参数不确定时先运行：

```bash
lark-cli mail user_mailbox.rules -h
lark-cli schema mail.user_mailbox.rules.<method>
```

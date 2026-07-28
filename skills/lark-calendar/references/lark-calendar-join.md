# calendar +join

加入一个日程（把当前身份加为参会人）。支持两种加入凭证，二选一：

1. **加入 token**：日历下发 RSVP 卡片 / 分享日程卡片时写入卡片的加密 `join_token`。
2. **分享链接**：用户「分享会议」/「分享日程」得到的链接（形如 `…/calendar/share?token=xxx`），或链接里的 `token` 原始值。

两种凭证的加密方案不同，服务端会自动识别并按对应逻辑加入，CLI 只需把用户给到的那一个透传即可。

## 命令

```bash
# 用 RSVP/分享卡片里的加入 token 加入
lark-cli calendar +join --join-token <join_token>

# 用分享会议/日程的链接加入（可直接粘贴完整链接）
lark-cli calendar +join --share-link "https://xxx.feishu.cn/calendar/share?token=xxx"

# 也可以只传链接里的 token 原始值
lark-cli calendar +join --share-link <share_token>

# bot 身份加入
lark-cli calendar +join --share-link "https://xxx.feishu.cn/calendar/share?token=xxx" --as bot
```

## 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `--join-token <token>` | 二选一 | RSVP/分享卡片中下发的加密加入 token |
| `--share-link <link>` | 二选一 | 分享会议/日程的链接（`…/calendar/share?token=xxx`）或链接里的 `token` 原始值 |
| `--dry-run` | 否 | 预览 API 调用，不执行 |

- `--join-token` 与 `--share-link` **必须且只能提供一个**，同时给会报错。

## 返回

成功返回加入日程的 `event_id`。

## 提示

- 支持 `--as user`（默认）和 `--as bot` 两种身份。
- **加入 token 场景**：需要当前身份确实收到过该卡片（在卡片所在的群里），这是「收到卡片」的凭证校验。
- **分享链接场景**：链接本身即分享凭证，不做「在群里」校验；但分享人是否有权分享仍由服务端校验，无权限会返回加入失败。
- 链接可以直接粘贴完整 URL，CLI/服务端会自动提取其中的 `token`。

## 参考

- [lark-calendar](../SKILL.md) -- skill 入口与路由
- [lark-calendar-rsvp](lark-calendar-rsvp.md) -- 回复（接受/拒绝/待定）日程

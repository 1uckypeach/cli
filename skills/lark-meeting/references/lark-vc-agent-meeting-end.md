# vc +meeting-end

当前 Host 应用 Bot 结束会议。

```bash
lark-cli vc +meeting-end --as bot --meeting-id 69999999
lark-cli vc +meeting-end --as bot --meeting-id 69999999 --dry-run
```

## 参数

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--meeting-id` | 是 | 长数字 Meeting ID，不是 9 位会议号。 |

服务端会拒绝非 Host Bot，即使该 Bot 是会议中最后一名参会人。

该 shortcut 仅支持 bot 身份，调用 `POST /open-apis/vc/v1/bots/end`。计费、仅剩一名参会人、离线或共享屏幕等状态都不会授予非 Host Bot 结束会议的权限。

所需应用 Scope：`vc:meeting.bot.manage:write`。

## 常见失败原因

- 当前应用 Bot 不在会议中：先使用同一应用 Bot 发起或加入该 Calendar 会议，再执行结束。
- 应用 Bot 在会中但不是当前 Host：将 Host 转交给该 Bot，或由当前 Host/Owner 结束会议。
- 会议未启用 Agent 会议能力：确认会议设置及会议 Owner 的必要灰度开关。

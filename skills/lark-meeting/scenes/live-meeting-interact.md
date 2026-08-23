# 读取会中事件与会中互动

围绕一场正在进行的会议执行只读查询或用户明确授权的会中操作。真实入会/离会使用应用机器人入会场景；已结束会议和会后产物使用会议查询场景。

如果任务包含“应用机器人入会后继续拉取事件或互动”，只读取并执行 [应用机器人参会与会中互动](live-meeting-attend.md) 的完整流程，不要在两个场景之间来回切换。

## 发现进行中的会议

没有 `meeting_id` 时，按用户需要的视角查询：

```bash
# 当前登录用户正在参加的会议
lark-cli vc +meeting-list-active --as user --format json

# 目标用户正在参加、且应用机器人也在会中的会议
lark-cli vc +meeting-list-active --as bot --user-id <open_id> --format json
```

- `--user-id` 必须是目标用户的 `ou_` open_id。
- 应用身份返回空不代表目标用户没有在开会，只代表没有找到目标用户与应用机器人同时在会中的会议。
- 返回多个会议时，展示标题、会议号和 `meeting_id` 让用户选择，不按“最近”擅选。
- 用户只给 9 位会议号时，在活跃会议结果中按 `meeting_no` 匹配；匹配失败时不要自动入会。
- `meeting_id` 从哪种身份取得，后续读取事件、发送消息与主持管理命令就沿用哪种身份；但结束会议和移出参会人只支持用户身份，必须重新确认该会议在用户身份下就是目标会议。

身份可见范围和会议号匹配见 [`lark-vc-meeting-list-active`](../references/lark-vc-meeting-list-active.md)。

## 读取最新会中事件

```bash
lark-cli vc +meeting-events --as <same_identity> --meeting-id <meeting_id> --page-all --format pretty
```

- 默认使用 `--page-all` 获取当前完整事件流，并保留返回的 `page_token` 供下次增量查询。
- 回答“现在、刚刚、最新”或当前会议总结前，重新查询事件；只有用户明确要求基于历史快照时才复用旧结果。
- 默认用 pretty 理解时间线；需要精确结构化字段、文档上下文或转发到 IM 时使用 JSON。
- 不要用会中事件代替已结束会议的参会人快照或会后复盘。

事件类型、分页、五分钟窗口和错误码见 [`lark-vc-meeting-events`](../references/lark-vc-meeting-events.md)。

## 读取共享内容和文档上下文

按事件中的 `share_id`、`share_doc`、`comment_id`、`element_token` 和 `block_id` 精确关联：

- 读取评论时只查询当前 `comment_id`，不要扫描整篇文档评论。
- 多个共享文档按用户问题选择相关文档；不要用“最近一次共享”替代当前 item 的 `share_id`。
- 只有用户明确要求预览且事件提供受支持的 `element_type` 与 token 时才下载，并显式选择输出路径。
- 关联或读取失败时标记 partial，保留原始标识和 raw payload；不要自动下载或猜测文档类型兜底。

精确事件 schema 和后续命令见 [`lark-vc-meeting-events`](../references/lark-vc-meeting-events.md) 的文档上下文部分。

## 发送会中文本或表情

只有用户明确要求发送并确认目标会议与内容时执行：

```bash
lark-cli vc +meeting-message-send --as <same_identity> --meeting-id <meeting_id> --msg-type text --text <message>
```

- 发送沿用 `meeting_id` 的来源身份；不要为了发送自动入会或先查会议详情。
- reaction 使用 Reference 中大小写敏感的完整 emoji key；不要编造 key。
- 发送失败时停止并报告，不自动换身份或重复发送，避免重复可见副作用。
- 用户要发送绑定群或 IM 消息时改用 `lark-im`，不要把会中消息命令当作群消息能力。

文本、reaction 和权限规则见 [`lark-vc-meeting-message-send`](../references/lark-vc-meeting-message-send.md)。

## 结束整场会议或移出参会人

只有用户明确要求主持管理动作，且目标会议与目标参会人已经确认时执行：

```bash
# 结束整场会议：先 dry-run，确认后再补 --yes
lark-cli vc +meeting-end --as user --meeting-id <meeting_id> --dry-run

# 移出参会人：participant tuple 必须来自 meeting get 快照
lark-cli vc +meeting-participant-kickout --as user --meeting-id <meeting_id> \
  --participant '<participant_id>=<user_type>' --dry-run
```

- 这两个命令都是 `user` 身份专属的高风险写操作。不要沿用 `--as bot`，也不要在未确认前直接补 `--yes`。
- 真实执行前，先用 `vc meeting get --params '{"meeting_id":"<meeting_id>","with_participants":true}' --as user` 核对会议与参会人快照。
- `vc +meeting-end` 的 dry-run 只预览请求路径；`vc +meeting-participant-kickout` 的 dry-run 会回显按输入顺序提交的 `kickout_users`。如果回显与用户意图不完全一致，停止并修正参数，不要继续执行。
- `--participant '<id>=<user_type>'` 每次调用接受 1 到 10 个重复 flag；ID 必须来自快照且首尾不能有空白。不要根据 open_id、昵称或设备信息猜 `user_type`，也不要把多个目标塞进 CSV/JSON。
- 参考手册分别见 [`lark-vc-meeting-end`](../references/lark-vc-meeting-end.md) 和 [`lark-vc-meeting-participant-kickout`](../references/lark-vc-meeting-participant-kickout.md)。

## 处理未发现会议或权限错误

- 用户身份未发现活跃会议时，可以查询当天最近结束的会议；仍无结果再询问时间、主题或会议号，不自行扩大时间范围。
- 应用身份未发现活跃会议时，只解释当前身份的空结果，不自动查询历史会议或真实入会。
- 用户身份调用活跃会议或事件查询时，普通 scope 缺失按 CLI hint 申请 `vc:meeting.meetingevent:read`；普通 scope 缺失不表示接口不支持用户身份，只有 CLI 明确说明不支持时才切到应用身份流程。
- 应用身份缺少权限时不要执行 `auth login`。按 CLI `hint` 和 `console_url` 配置 `vc:meeting.bot.join:write`，并依次检查应用发布、租户安装和“权限可访问的数据范围”；数据范围应为“按条件筛选”，条件为“会议的归属者 包含 与应用的可用范围一致”。
- scope、安装和数据范围都正确后仍失败时，保留 CLI 返回的错误码和 `log_id`，按服务端权限异常排查；不要反复登录或改用其他身份重试。

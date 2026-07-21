# provider: example

> **前置条件：** 先读 [`../../../lark-shared/SKILL.md`](../../../lark-shared/SKILL.md) 与 [`lark-agents SKILL.md`](../../SKILL.md)（框架契约、动词、通用错误规则）。

**catalog 型** provider：仓库内置的离线演示 agent（内存 mock，零网络，无需开放平台侧任何配置；本地前置见下节）。agent_ref = `example:<agent_id>`。echo/reporter 的任务发出即完成（终态）；planner 会先停在 `input_required` 弹一组问题等答复。任务状态存于本机临时快照，跨命令可查。

## agent 发现

`agents list example` 直接枚举全部 3 个 agent：`example:echo`（复读机）、`example:planner`（报表规划器，HITL 演示）、`example:reporter`（报表生成器，artifact 演示）。真实输出样例见 [agents list「二级发现」](../lark-agents-list.md)（该样例即本 provider 实拍）。

## scope 与身份前置

**无 scope、无授权**——零网络，user/bot 两种身份都无需 `auth login`，scope preflight 恒通过；card 里 bot 条目无 precondition。但**仍需一次 `lark-cli config init` 基础配置**：全新机器上未配置时，除 `card` 与 `send --dry-run` 外的动词会报 `not_configured`（exit 3，hint 指向 config init），这不是 scope 问题。

## 能力特例

`agents card` 读到什么就只能调什么；三个 agent 刻意不同：

| capability | echo | planner | reporter |
|---|---|---|---|
| `task_get` / `task_list` / `context_*` 三键 | true | true | true |
| `task_cancel` | false | **true**（问题组可放弃） | true |
| `file_input` | false | false | true |
| `artifact_download` | false | false | true |
| `input_required` | false | **true（真会停）** | false（任务即时完成，从不提问；对它 `--answer` 被离线拒 `unsupported_capability`） |

- 只有 reporter 产出 artifact（内联 CSV/XLSX，`artifacts[]` **下载前**即带 `name`/`mime`）。
- 对 reporter 的 cancel 会真正派发，但任务即时终态 → 报 `failed_precondition`（hint 给出查看结果的命令）。

## 行为特点

- **多轮记忆可验证**：同一 `--context-id` 续发，echo 的回复从第 2 轮起带轮次标记（如 `……（第 2 轮）`）。
- **echo/reporter 不支持向已有任务续发**：带 `--task-id` 报 `failed_precondition`，hint 引导去掉 `--task-id` 用 `--context-id` 起新一轮。
- **参数演示（reporter 的 send）**：契约用 `agents card example:reporter --operation send` 实时查。可观察行为：`--param report_format=xlsx` 改变回复文案与产物后缀；`report_format=pdf` 触发 enum 教学错误（离线、exit 2）；`--param render.theme=dark --param render.watermark=true` 让回复带"dark 主题，含水印"。
- **HITL（planner）**：首次 send 停在 `input_required`，弹一个**三题问题组**（组标题「报表生成确认」）：单选「按什么维度拆分？」（by_region/by_category）、自由文本「时间范围？」、多选「包含哪些区域？」（east/north/skip——skip=「由 agent 决定」，与实值互斥）。题目键在建组时随机代铸（如 `q1_3f2a`），同 task 的下一组必换键（陈旧重发保护）。按 [SKILL.md 工作流](../../SKILL.md) 第 4 步用 `--answer` 一次交清后转 `completed`，受理回执把 option_id 解析回 label；planner 是**严格姿态**服务端：缺答/非法选项/单选多值/skip 冲突一次报全（`params[]` 带 reason 枚举与题目声明），已答组的重复提交报 `failed_precondition` + `resolved_answers`；对停着的任务裸发 `--text` 报 `invalid_argument` 引导 `--answer`（不会岔生新任务）。

## 服务端错误码目录

**无**（零网络）。本地校验错误一例（真实输出，`agents card example:nonexistent`，exit 2）——目录外的 agent_id 本地报错、hint 指回枚举命令：

```json
{
  "ok": false,
  "error": {
    "type": "validation",
    "subtype": "invalid_argument",
    "message": "未知的 example agent 'nonexistent'",
    "hint": "运行 lark-cli agents list example 查看可用 agent"
  }
}
```

## 参考

- [lark-agents](../../SKILL.md) — 框架契约与全部动词
- [agents list](../lark-agents-list.md) · [agents card](../lark-agents-card.md) · [agents send](../lark-agents-send.md) · [agents task](../lark-agents-task.md) · [agents context](../lark-agents-context.md)

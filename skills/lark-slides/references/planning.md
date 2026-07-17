# 规划（大纲 + slide_plan.json + 资产）

生成前把叙事、页面角色、视觉重点、文字密度和资产需求固定下来，避免从用户提示直接跳到 XML。观感规则见 `design.md`，版式几何见 `layout.md`，创建后核对见 `verify.md`。

## 大纲先给用户确认

生成 XML 前先产出一份大纲交用户确认，格式：

```text
[PPT 标题] — [定位描述]，面向 [目标受众]

页面结构（N 页）：
1. 封面页：[标题文案]
2. [页面主题]：[要点1]、[要点2]、[要点3]
...
N. 结尾页：[结尾文案]

风格：[配色方案]，[排版风格]
```

确认过主题、受众、页数、风格后再进入 plan。

## 何时必写 plan

新建演示文稿、生成新 deck、重排多页或替换整页结构时，必须先写 `slide_plan.json` 再生成 XML。

小型已有页编辑可豁免：只替换一个标题、改一个数字、插入一个块、上传并插入一张图。只要任务会重排多页或替换整页结构，仍需规划层。

## Plan 目录与生命周期

每个 deck 用独立目录 `.lark-slides/plan/<id>/`，同一 workspace 里多个 deck 不互相覆盖。

- `<id>` 取值：新 deck 用标题 slug + 日期时间（如 `q3-review-20260507-1805`）；改写已有 PPT 用 `xml_presentation_id`；无标题任务用短 slug + 日期时间。
- 不复用 `.lark-slides/plan/slide_plan.json` 这类共享路径。
- 先建目录再写文件：`mkdir -p .lark-slides/plan/<id>`。
- 同一 deck 的 XML 生成和创建后核对复用同一 plan 路径。

`.lark-slides/` 是本地 agent 状态，支持恢复、迭代和后续编辑，不当源码、默认不提交。创建或大改写成功后保留 `slide_plan.json`（deck 可编辑的设计状态）；核对通过后清理临时 XML（throwaway XML 放 `/tmp` 或成功后删除）；创建失败或部分成功时保留相关 XML/debug 直到恢复完成，先记 `xml_presentation_id` 再回读当前状态后重试。

## slide_plan.json 形态

下为最小示例，实际填齐下文所有必填字段：

```json
{
  "presentation_goal": "...",
  "audience": "...",
  "theme_style": "...",
  "visual_system": { "background_strategy": "...", "motif": "...", "color_roles": {} },
  "typography_constraints": { "title_max_lines": 2, "long_text_handling": "..." },
  "verification_plan": {},
  "slides": [
    {
      "page": 1, "title": "...", "key_message": "...",
      "layout_type": "...", "visual_focus": "...",
      "asset_need": { "asset_type": "logo", "purpose": "...", "suggested_query": "...", "fallback_if_missing": "..." },
      "text_density": "low", "speaker_intent": "..."
    }
  ]
}
```

顶层字段：

- `presentation_goal`：整个 deck 要达成什么。
- `audience`：目标读者/听众及其假定背景。
- `theme_style`：视觉基调、配色方向、专业风格。
- `visual_system`：跨页必须稳定的 deck 级视觉规则（背景策略、复现 motif、颜色角色）。
- `typography_constraints`：行数、文本框密度、长文本处理的 deck 级上限。
- `verification_plan`：创建/大改后的显式检查（背景一致性、文字容纳、视觉重点、资产渲染）。
- `slides`：有序页面规划。

每页字段：

- `page`：1-based 页码。 `title`：页标题。 `key_message`：本页必须落地的唯一观点。
- `layout_type`：规划的页面结构（词表见 `layout.md`）。 `visual_focus`：主视觉对象或区域。
- `asset_need`：规划态资产元数据，不触发搜索/下载/上传。
- `text_density`：`low` / `medium` / `high`（规则见 `layout.md`）。
- `speaker_intent`：讲者为何需要这页、如何推进叙事。

## chart_contract（可选页级字段）

页面计划含 `<chart>` 支持的标准数据图时必填：

```json
{ "chart_contract": { "required": true, "render_as": "native_chart", "chart_type": "line",
  "data_source": "mock_placeholder", "data_series_required": true, "manual_shape_fallback_allowed": false } }
```

`required == true` 时该页 XML 必须产出 `<chart>` 元素；shape/line/polyline/whiteboard 近似不满足契约。`data_series_required` 要求 XML 含 `<chartData>`，不要求真实值。

`data_source` 三值取一：

- `user_provided`：用户给了具体值/表/CSV/指标，用之，不替换成 mock。
- `mock_placeholder`：用户要占位/模板/示例/可后替换的图位，用 mock 数据填原生 `<chart>`。
- `mock_required_by_intent`：用户没给具体值但要求数据表达/趋势/对比/分布，用 mock 数据填原生 `<chart>`。

真实值缺失但图表表达属用户意图时，写 mock/占位值进原生 `<chart>` 并明确标注，不退回手绘或指标块。

## 资产规划

`asset_need` 是元数据，可描述图、图标、图表、流程图、时序图、架构图、装饰图案、截图或示意图需求，但不要求 web 搜索、本地下载或媒体上传。单个用对象，多个真实需求用数组，无用资产用 `asset_type: "none"`。

每项必带：

- `asset_type`：`paper_figure` / `architecture_diagram` / `icon` / `logo` / `chart` / `infographic` / `screenshot` / `flow_diagram` / `none` 之一。
- `purpose`：为何有助本页 `key_message`。
- `suggested_query`：未来查找提示，不主动执行。
- `fallback_if_missing`：具体到能转 XML 的原生视觉方案（shape/label/table/whiteboard/占位面板）。

规则：

- 每项资产必带 `fallback_if_missing`，最终 XML 不留空图框，缺素材即渲染 fallback；弱写法禁用：「用占位」「另找图」「没有就留空」「用通用装饰」。
- 资产必须服务本页 `key_message` 和 `visual_focus`，不加不澄清页面的装饰资产；少数高价值资产优于每页一图，6 页技术/商业 deck 内容允许时至少 3 页规划资产。
- `chart` 类若为支持的标准数据图，`fallback_if_missing` 仍须渲染原生 `<chart>`，不用手绘/`<whiteboard>` 模仿；`<chart>` 不支持 funnel/scatter，此类映射到 `<whiteboard>` SVG。

## 续作更新

继续已有 deck 时更新同一 plan 路径，不新建断链 plan，保持 plan 与已创建内容对齐。按页面角色和证据需求规划（如「方法概览页在源有可读图时用图」），不因上个 deck 用过某页号就硬绑固定页号；plan 描述决策规则，不是刚性模板序列。

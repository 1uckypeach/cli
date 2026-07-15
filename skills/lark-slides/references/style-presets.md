# Style Presets

Ready-made color + layout + font palettes. When the user does not supply a brand palette, pick the preset by the deck's **document type** — match the deck against each preset's **适用** line — then copy its values into `slide_plan.json`'s `visual_system.color_roles`, `visual_system.layout_system`, and `typography_constraints.font_family`, and let `visual-planning.md` → Style System govern how colors, layout, and fonts are applied. Do not invent colors when a preset fits.

Match on what the deck **is** (a data-analysis report, a pitch deck, a lesson…), not on the industry it is about. A finance / bank / government topic does not imply navy or red — resolve it to the document type first (e.g. a bank performance review is a *data-analysis report* → preset 4).

Each preset fills all six color roles. Where the source palette had no saturated emphasis color, the `accent` was derived per the accent-gap rule and is marked `(derived)`; every preset is ready to use as-is.

## Presets

Each preset is self-contained: scenario heading, tone + density, the six color roles, a layout system, and fonts. Pick one, copy its values into the plan (shape shown in *How to apply*).

### 1. Investment / industry research report
*适用:行业研究报告、市场/赛道研究、投研与尽调报告*
*Tone: minimal, professional, data-driven, authoritative-restrained, Density: medium–high*
- **colors** — primary `#415A77` 钢蓝, secondary `#0D1B2A` 深海军蓝, accent `#2E6FB0` (derived) 亮钢蓝, background `#F3F4F6` 冷白, text_main `#0D1B2A`, text_sub `#6B7788` 灰蓝
- **layout** (`layout_system`) — 高密度 · 咨询报告式版式:12 栏网格、细分割线、数据表格与图表页;固定页眉/页码/章节编号/注释区;节奏理性、清晰、可审阅
- **fonts** (`font_family`) — 中文 思源黑体、思源宋体;英文 Inter、IBM Plex Sans;强调数字 Roboto Mono

### 2. Government / party work summary
*适用:党建工作总结、政策解读、公司制度宣贯、政务汇报*
*Tone: clean, dignified, credible, warm, low-saturation civic, Density: medium*
- **colors** — primary `#C0453D` 砖红, secondary `#D4B15C` 金黄, accent `#D4B15C` 金黄, background `#F7F3EE` 暖灰米白, text_main `#333333` 深炭灰, text_sub `#7A756F` 暖灰
- **layout** (`layout_system`) — 中密度 · 现代党建画册式版式:封面大面积色块+强标题,内容页左右分区、规则网格、清晰留白;重信息层级与庄重人文,避免传统红金堆砌
- **fonts** (`font_family`) — 中文 思源黑体、寒蝉德黑体;英文 Inter、Libre Franklin;强调数字 Rajdhani

### 3. Pitch deck / business plan
*适用:融资路演、商业计划书、项目路演*
*Tone: minimal, premium, international, strategic, composed, Density: low–medium*
- **colors** — primary `#415A77` 钢蓝, secondary `#0D1B2A` 深海军蓝, accent `#2E6FB0` (derived) 亮钢蓝, background `#F5F6F8` 冷白, text_main `#0D1B2A`, text_sub `#5F6F82` 蓝灰
- **layout** (`layout_system`) — 低-中密度 · 高端商业画册式版式:大标题、大留白、色块分区、小型版式样张贯穿全套;章节页强品牌感,内容页克制商业叙事节奏
- **fonts** (`font_family`) — 中文 思源宋体、思源黑体;英文 Playfair Display、Inter;强调数字 Oswald

### 4. Market / data analysis
*适用:数据分析报告、市场/竞品分析、经营/业绩/绩效/KPI 分析*
*Tone: clean, data-driven, analytical, credible, Density: medium–high*
- **colors** — primary `#3D6B63` 灰绿, secondary `#0F1720` 深蓝黑, accent `#1F9E7A` (derived) 亮绿, background `#F7F7F5` 暖白, text_main `#0F1720`, text_sub `#6B7280` 中性灰
- **layout** (`layout_system`) — 中高密度 · 数据分析型版式:左侧主题叙事,右侧图表/要点/组件/色板模块;“洞察—数据—结论”节奏,信息分区明确但不拥挤
- **fonts** (`font_family`) — 中文 寒蝉德黑体、思源黑体;英文 Inter、IBM Plex Sans;强调数字 Roboto Mono

### 5. Equity research
*适用:个股/股权投资分析、投行研究、估值报告*
*Tone: high-end research, calm, data-oriented, elite, Density: high*
- **colors** — primary `#348271` 孔雀绿, secondary `#0D1B2A` 深海军蓝, accent `#2FA98D` (derived) 亮青绿, background `#F6F8FA` 冷白, text_main `#0D1B2A`, text_sub `#6B7A8A` 灰蓝
- **layout** (`layout_system`) — 高密度 · 投研画册式版式:大标题与数据图表并重,页面保留充分留白与精致分栏;以指标/趋势/估值/判断为核心节奏,专业审阅感
- **fonts** (`font_family`) — 中文 思源宋体、思源黑体;英文 Playfair Display、Inter;强调数字 Roboto Mono

### 6. Course design / lesson plan
*适用:课程设计、课程汇报、教案*
*Tone: clear, focused, learner-centered, structured, modern-teaching, Density: medium*
- **colors** — primary `#3B82F6` 柔和蓝, secondary `#10B981` 柔和绿, accent `#10B981` 柔和绿, background `#FAFAFA` 冷白, text_main `#1E293B`, text_sub `#64748B` 石板灰
- **layout** (`layout_system`) — 中密度 · 教学设计手册式版式:大标题、清晰分区、规则网格、组件化说明;围绕“目标—结构—活动—反馈”展开,强调可执行性与低认知负担
- **fonts** (`font_family`) — 中文 思源黑体、霞鹜 975 圆体;英文 Inter、Nunito Sans;强调数字 Roboto Mono

### 7. Teaching slides
*适用:教学课件、分学段授课课件*
*Tone: gentle, approachable, low cognitive load, readable, modern-course, Density: low–medium*
- **colors** — primary `#6B8F7A` 灰绿, secondary `#3D5A80` 雾霾蓝, accent `#4E9E7E` (derived) 亮绿, background `#F7F7F5` 暖灰白, text_main `#1E2A3A` 深蓝灰, text_sub `#6B7280` 中性灰
- **layout** (`layout_system`) — 低-中密度 · 亲和型教学课件版式:大字号、宽留白、少层级贯穿全套;节奏稳定、清楚、舒展,适合知识点逐步展开与重点强化
- **fonts** (`font_family`) — 中文 寒蝉团圆体 圆体、霞鹜 975 圆体;英文 Nunito Sans、Quicksand;强调数字 Rubik

## How to apply

Copy the matched row into the plan. Example using preset 1:

```json
{
  "theme_style": "minimal, professional, data-driven, authoritative-restrained",
  "visual_system": {
    "color_roles": {
      "primary": "#415A77", "secondary": "#0D1B2A", "accent": "#2E6FB0",
      "background": "#F3F4F6", "text_main": "#0D1B2A", "text_sub": "#6B7788"
    },
    "background_strategy": "content pages on #F3F4F6; cover/divider on #0D1B2A dark block reusing the steel-blue motif",
    "motif": "low-saturation blocks + section tags + thin dividers + page-number system",
    "layout_system": "高密度 · 咨询报告式版式:12 栏网格、细分割线、数据表格与图表页;固定页眉/页码/章节编号/注释区;节奏理性、清晰、可审阅"
  },
  "typography_constraints": {
    "font_family": "中文 思源黑体、思源宋体;英文 Inter、IBM Plex Sans;强调数字 Roboto Mono"
  }
}
```

Notes:

- Presets are starting points, not mandates. If the user gives brand colors, use those and skip the preset.
- Accents marked `(derived)` were synthesized per the accent-gap rule; if the user later supplies a real emphasis color, replace it.

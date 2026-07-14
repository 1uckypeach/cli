# Style Presets

Ready-made color + font + graphic-language palettes. When the user does not supply a brand palette, pick the preset whose scenario best matches the deck, copy its values into `slide_plan.json`'s `visual_system.color_roles`, `typography_constraints`, and `visual_system.graphic_language_pool`, then let `visual-planning.md` → Style System govern how the roles are applied. Do not invent colors when a preset fits.

Each preset fills all six color roles. Where the source palette had no saturated emphasis color, the `accent` was derived per the accent-gap rule and is marked `(derived)`; every preset is ready to use as-is.

## Presets

Each preset is self-contained: scenario heading, tone + density, the six color roles, fonts, and the graphic-language pool. Pick one, copy its values into the plan (shape shown in *How to apply*).

### 1. Investment / industry research report
*Tone: minimal, professional, data-driven, authoritative-restrained, Density: medium–high*
- **colors** — primary `#415A77` 钢蓝, secondary `#0D1B2A` 深海军蓝, accent `#2E6FB0` (derived) 亮钢蓝, background `#F3F4F6` 冷白, text_main `#0D1B2A`, text_sub `#6B7788` 灰蓝
- **fonts** (`font_family`) — CJK 思源黑体/思源宋体, Latin Inter/IBM Plex Sans, number Roboto Mono
- **graphic language** — metric card, horizontal bar chart, thin-line table, trend list, page-number system, section tag, low-saturation block

### 2. Government / party work summary
*Tone: clean, dignified, credible, warm, low-saturation civic, Density: medium*
- **colors** — primary `#C0453D` 砖红, secondary `#D4B15C` 金黄, accent `#D4B15C` 金黄, background `#F7F3EE` 暖灰米白, text_main `#333333` 深炭灰, text_sub `#7A756F` 暖灰
- **fonts** (`font_family`) — CJK 思源黑体/寒蝉德黑体, Latin Inter/Libre Franklin, number Rajdhani
- **graphic language** — small red block, swatch matrix, gold hairline, type specimen, work-data card, ruled divider

### 3. Pitch deck / business plan
*Tone: minimal, premium, international, strategic, composed, Density: low–medium*
- **colors** — primary `#415A77` 钢蓝, secondary `#0D1B2A` 深海军蓝, accent `#2E6FB0` (derived) 亮钢蓝, background `#F5F6F8` 冷白, text_main `#0D1B2A`, text_sub `#5F6F82` 蓝灰
- **fonts** (`font_family`) — CJK 思源宋体/思源黑体, Latin Playfair Display/Inter, number Oswald
- **graphic language** — large dark cover block, type specimen, layout thumbnail, keyword rail, thin divider, attribute tag

### 4. Market / data analysis
*Tone: clean, data-driven, analytical, credible, Density: medium–high*
- **colors** — primary `#3D6B63` 灰绿, secondary `#0F1720` 深蓝黑, accent `#1F9E7A` (derived) 亮绿, background `#F7F7F5` 暖白, text_main `#0F1720`, text_sub `#6B7280` 中性灰
- **fonts** (`font_family`) — CJK 寒蝉德黑体/思源黑体, Latin Inter/IBM Plex Sans, number Roboto Mono
- **graphic language** — bar chart, key-conclusion card, data label, button component, tab component, pager, line icon, light info panel

### 5. Equity research
*Tone: high-end research, calm, data-oriented, elite, Density: high*
- **colors** — primary `#348271` 孔雀绿, secondary `#0D1B2A` 深海军蓝, accent `#2FA98D` (derived) 亮青绿, background `#F6F8FA` 冷白, text_main `#0D1B2A`, text_sub `#6B7A8A` 灰蓝
- **fonts** (`font_family`) — CJK 思源宋体/思源黑体, Latin Playfair Display/Inter, number Roboto Mono
- **graphic language** — investment metric table, mini trend line, donut chart, bar chart, swatch matrix, data table, thin divider

### 6. Course design / lesson plan
*Tone: clear, focused, learner-centered, structured, modern-teaching, Density: medium*
- **colors** — primary `#3B82F6` 柔和蓝, secondary `#10B981` 柔和绿, accent `#10B981` 柔和绿, background `#FAFAFA` 冷白, text_main `#1E293B`, text_sub `#64748B` 石板灰
- **fonts** (`font_family`) — CJK 思源黑体/霞鹜975圆体, Latin Inter/Nunito Sans, number Roboto Mono
- **graphic language** — swatch module, type specimen, UI component, spacing system, principle list, form control, progress bar, structure diagram

### 7. Teaching slides
*Tone: gentle, approachable, low cognitive load, readable, modern-course, Density: low–medium*
- **colors** — primary `#6B8F7A` 灰绿, secondary `#3D5A80` 雾霾蓝, accent `#4E9E7E` (derived) 亮绿, background `#F7F7F5` 暖灰白, text_main `#1E2A3A` 深蓝灰, text_sub `#6B7280` 中性灰
- **fonts** (`font_family`) — CJK 寒蝉团圆体/霞鹜975圆体, Latin Nunito Sans/Quicksand, number Rubik
- **graphic language** — soft-rounded button, selection control, status dot, principle card, light section, simple icon, title accent line, bottom keyword rail

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
    "graphic_language_pool": ["metric card", "horizontal bar chart", "thin-line table", "trend list", "section tag"]
  },
  "typography_constraints": {
    "font_family": "CJK 思源黑体/思源宋体, Latin Inter/IBM Plex Sans, number Roboto Mono"
  }
}
```

Notes:

- Presets are starting points, not mandates. If the user gives brand colors, use those and skip the preset.
- The (derived)  accents are derived; if the user later supplies a real emphasis color, replace it.

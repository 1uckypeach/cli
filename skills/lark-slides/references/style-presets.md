# Style Presets

Ready-made color + layout + font palettes. When the user does not supply a brand palette, and the deck **genuinely belongs to** one of the preset document types below, pick that preset — match the deck against each preset's **Use-for** line — then copy its values into `slide_plan.json`'s `visual_system.color_roles`, `visual_system.layout_system`, and `typography_constraints.font_family`, and let `visual-planning.md` → Style System govern how colors, layout, and fonts are applied. Do not invent colors when a preset genuinely fits.

Match on what the deck **is** (a data-analysis report, a pitch deck, a lesson…), not on the industry it is about. A finance / bank / government topic does not imply navy or red — resolve it to the document type first (e.g. a bank performance review is a *data-analysis report* → preset 4).

Match literally; do not stretch a preset by loose analogy. A preset's **Use-for** line names a specific document; "it also presents something" or "it also lays out a plan" does not make a deck a fundraising pitch or a business report. **When no preset's Use-for line clearly covers the deck (many personal, lifestyle, or creative decks won't), do not force the nearest one — self-configure a coherent, topic-appropriate palette using the same `color_roles` + `layout_system` + `font_family` framework** (six roles, a density + layout-system line, and CJK / Latin / number fonts).

Each preset fills all six color roles. Where the source palette had no saturated emphasis color, the `accent` was derived per the accent-gap rule and is marked `(derived)`; every preset is ready to use as-is.

## Presets

Each preset is self-contained: scenario heading, tone + density, the six color roles, a layout system, and fonts. Pick one, copy its values into the plan (shape shown in *How to apply*).

### 1. Investment / industry research report
*Use for: industry research reports, market / sector studies, investment & due-diligence reports*
*Tone: minimal, professional, data-driven, authoritative-restrained, Density: medium–high*
- **colors** — primary `#415A77` steel blue, secondary `#0D1B2A` deep navy, accent `#2E6FB0` (derived) bright steel blue, background `#F3F4F6` cool white, text_main `#0D1B2A`, text_sub `#6B7788` blue-gray
- **layout** (`layout_system`) — high density · consulting-report layout system: 12-column grid, thin dividers, data tables and chart pages; fixed header / page-number / section-number / annotation zones; rational, clear, review-ready rhythm
- **fonts** (`font_family`) — CJK 思源黑体, 思源宋体; Latin Inter, IBM Plex Sans; number Roboto Mono

### 2. Government / party work summary
*Use for: party-building summaries, policy interpretation, corporate-policy rollouts, government reporting*
*Tone: clean, dignified, credible, warm, low-saturation civic, Density: medium*
- **colors** — primary `#C0453D` brick red, secondary `#D4B15C` gold, accent `#D4B15C` gold, background `#F7F3EE` warm ivory, text_main `#333333` charcoal, text_sub `#7A756F` warm gray
- **layout** (`layout_system`) — medium density · modern civic-brochure layout system: cover uses a large color block + strong title, content pages use left/right zoning, regular grids, and clear whitespace; emphasize information hierarchy and a dignified, humane tone; avoid the traditional red-and-gold pile-up
- **fonts** (`font_family`) — CJK 思源黑体, 寒蝉德黑体; Latin Inter, Libre Franklin; number Rajdhani

### 3. Pitch deck / business plan
*Use for: fundraising pitches, business plans, project roadshows*
*Tone: minimal, premium, international, strategic, composed, Density: low–medium*
- **colors** — primary `#415A77` steel blue, secondary `#0D1B2A` deep navy, accent `#2E6FB0` (derived) bright steel blue, background `#F5F6F8` cool white, text_main `#0D1B2A`, text_sub `#5F6F82` blue-gray
- **layout** (`layout_system`) — low–medium density · premium business-brochure layout system: large titles, generous whitespace, color-block zoning, and small layout swatches throughout; section pages feel more brand-led, content pages keep a restrained business-narrative rhythm
- **fonts** (`font_family`) — CJK 思源宋体, 思源黑体; Latin Playfair Display, Inter; number Oswald

### 4. Market / data analysis
*Use for: data-analysis reports, market / competitor analysis, operations / performance / KPI analysis*
*Tone: clean, data-driven, analytical, credible, Density: medium–high*
- **colors** — primary `#3D6B63` gray-green, secondary `#0F1720` dark blue-black, accent `#1F9E7A` (derived) bright green, background `#F7F7F5` warm white, text_main `#0F1720`, text_sub `#6B7280` neutral gray
- **layout** (`layout_system`) — medium–high density · data-analysis layout system: topic narrative on the left, charts / key points / components / swatch modules on the right; an "insight → data → conclusion" rhythm; clearly zoned but not crowded
- **fonts** (`font_family`) — CJK 寒蝉德黑体, 思源黑体; Latin Inter, IBM Plex Sans; number Roboto Mono

### 5. Equity research
*Use for: single-stock / equity investment analysis, sell-side research, valuation reports*
*Tone: high-end research, calm, data-oriented, elite, Density: high*
- **colors** — primary `#348271` peacock green, secondary `#0D1B2A` deep navy, accent `#2FA98D` (derived) bright teal-green, background `#F6F8FA` cool white, text_main `#0D1B2A`, text_sub `#6B7A8A` blue-gray
- **layout** (`layout_system`) — high density · equity-research brochure layout system: large titles balanced with data charts, ample whitespace and refined columns; a core rhythm around metrics / trends / valuation / judgment; a professional review feel
- **fonts** (`font_family`) — CJK 思源宋体, 思源黑体; Latin Playfair Display, Inter; number Roboto Mono

### 6. Course design / lesson plan
*Use for: course design, course reports, lesson plans*
*Tone: clear, focused, learner-centered, structured, modern-teaching, Density: medium*
- **colors** — primary `#3B82F6` soft blue, secondary `#10B981` soft green, accent `#10B981` soft green, background `#FAFAFA` cool white, text_main `#1E293B`, text_sub `#64748B` slate gray
- **layout** (`layout_system`) — medium density · instructional-design-handbook layout system: large titles, clear zoning, regular grids, componentized explanations; organized around "objective → structure → activity → feedback"; emphasize actionability and low cognitive load
- **fonts** (`font_family`) — CJK 思源黑体, 霞鹜 975 圆体; Latin Inter, Nunito Sans; number Roboto Mono

### 7. Teaching slides
*Use for: teaching slides, grade-level courseware*
*Tone: fresh, friendly, academic-approachable, youthful, global, Density: low–medium*
- **colors** — primary `#8FE3F9` sky blue, secondary `#CDEFD9` mint green, accent `#FFE7A3` cream yellow, background `#FAF7F2` warm white, text_main `#2F2F2F` charcoal, text_sub `#6B7280` warm gray
- **layout** (`layout_system`) — low–medium density · modular-classroom-courseware layout system: large titles, light-color knowledge cards, rounded containers, and clear numbering throughout; section / course-overview / content / comparison / flow / summary pages form a complete teaching rhythm; ample whitespace, emphasizing readability, approachability, and a classroom-guidance feel
- **fonts** (`font_family`) — CJK 思源黑体, 寒蝉全圆体; Latin Poppins, Quicksand; number Nunito Sans

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
    "layout_system": "high density · consulting-report layout system: 12-column grid, thin dividers, data tables and chart pages; fixed header / page-number / section-number / annotation zones; rational, clear, review-ready rhythm"
  },
  "typography_constraints": {
    "font_family": "CJK 思源黑体, 思源宋体; Latin Inter, IBM Plex Sans; number Roboto Mono"
  }
}
```

Notes:

- Presets are starting points, not mandates. If the user gives brand colors, use those and skip the preset.
- Accents marked `(derived)` were synthesized per the accent-gap rule; if the user later supplies a real emphasis color, replace it.

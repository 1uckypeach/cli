# IconPark Icons

IconPark icons are written into slides XML via `<icon>`; `iconType` must come from this skill's offline index — never assemble paths from memory.

## Machine-First Flow

```bash
python3 skills/lark-slides/scripts/iconpark_tool.py search --query "growth trend" --limit 8
python3 skills/lark-slides/scripts/iconpark_tool.py resolve --name chart-line
python3 skills/lark-slides/scripts/iconpark_tool.py list-categories
```

`search` returns a JSON array where each item contains `iconType`, `category`, `name`, `tags`, `score`. Write the chosen `iconType` directly into the XML and give the icon a visible color:

```xml
<icon iconType="iconpark/Charts/chart-line.svg" topLeftX="80" topLeftY="120" width="32" height="32">
  <fill>
    <fillColor color="rgba(37, 99, 235, 1)"/>
  </fill>
</icon>
```

## Usage Rules

- Search first by default: semantic icon needs must first go through `iconpark_tool.py search --limit 8` or `--limit 10`, letting the agent make a second-pass choice among candidates based on the layout semantics; do not read the full index, and do not invent nonexistent `iconType` values.
- Icons are for concept cues, steps, statuses, metrics, roles, and navigation; do not fill the layout with irrelevant decorative icons.
- Common sizes: inline status icons 16-24px, card title icons 28-40px, primary visual icons 56-96px.
- The visual spec requires icons to set a non-transparent `fillColor`, explicitly specifying a color with sufficient contrast against the background; on dark backgrounds, prefer placing icons on light circular/square bases, or use `rgba(255, 255, 255, 1)` as the icon fill color.
- When no suitable icon can be found, draw an XML-native fallback with shapes, lines, and text; do not leave an empty icon slot.

## High-Frequency Examples

| Semantics | iconType |
|---|---|
| Settings/configuration | `iconpark/Base/setting.svg` |
| Target | `iconpark/Base/aiming.svg` |
| Growth trend | `iconpark/Charts/positive-dynamics.svg` |
| Line trend | `iconpark/Charts/chart-line.svg` |
| Proportion | `iconpark/Charts/chart-proportion.svg` |
| Data dashboard | `iconpark/Charts/data-screen.svg` |
| Success | `iconpark/Character/check-one.svg` |
| Failure/risk | `iconpark/Character/close-one.svg` |
| Team/users | `iconpark/Peoples/peoples.svg` |
| Security | `iconpark/Safe/protect.svg` |
| Global/market | `iconpark/Travel/world.svg` |
| Email/contact | `iconpark/Office/envelope-one.svg` |

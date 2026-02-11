# Armillary Sphere Dashboard Design

## Core Concept

Restructure the Dashboard as a **flattened Armillary Sphere (浑仪)**. The Gear Clock is the celestial core; all information layers wrap around it as concentric ring bands, forming a precision data instrument.

## Ring Layer Structure (inside → out)

### Layer 0 — Core Hub (existing)

- Center stats: request count, IN/OUT tokens, cost
- No changes

### Layer 1 — AM Heatmap Ring (existing)

- 12 arc segments, hours 0:00–11:00

### Layer 2 — PM Heatmap Ring (existing)

- 12 arc segments, hours 12:00–23:00

### Layer 3 — Gear Teeth Ring (existing)

- 24 teeth + outer ring, data-event-driven rotation

### Layer 4 — Satellite Stats Ring (new)

- 4 stat cards (Request Stats / Overview / Input / Output) positioned at the four corners around the gear
- Each card connected to gear center via a decorative **energy channel line** (SVG path, semi-transparent, pulsing animation)
- Cards retain rectangular form but shrink; visually "attached" to the armillary perimeter

### Layer 5 — Time Dimension Ring (new, switchable)

- Day/Week/Month/Year views as different ring layers of the armillary
- **Day (default)**: Layers 1-3 displayed normally (current gear heatmap)
- **Week**: New ring outside Layer 3 with 7 arc segments (one per day), inner layers fade to background
- **Month**: Outer ring becomes ~30 arc segments (days of month)
- **Year**: Outer ring becomes 12 arc segments (months), or existing 53×7 grid mapped to circular layout
- On switch: selected layer highlights (opacity 1, subtle scale 1.02), other layers fade (opacity 0.15-0.2), creating "armillary rotation focus" effect

## Visual Connections & Animation

### Energy Channels (stat cards → core)

- SVG path from each stat card to gear center
- Style: semi-transparent `var(--nb-lime)`, ~1px strokeWidth, strokeDasharray for "energy flow"
- CSS animation on dashoffset for continuous outward pulse from core
- Direction: core radiates outward (core is the energy source)

### Ring Band Switch Animation

- Selected band: opacity 1, scale 1.02
- Other bands: opacity 0.15-0.2
- framer-motion `animate`, duration ~0.4s
- Day view special: Layers 1-3 all highlight together (they form the gear core)

### Gear Rotation Depth

- Layer 3 continues WS data-event-driven rotation
- Counter-rotating decorative gear retained (armillary "multi-ring different directions" reference)
- Non-day views: rotation increment reduced (15° → 5°), core still runs but focus is on outer layer

### Breathing & Pulse

- Keep existing `reactor-core-pulse` animation
- Energy channel pulse frequency syncs with WS data events — faster on data, slow breathing on idle

## Pedestal Panels (below armillary)

### ChartSection (cost trend)

- Position: below armillary as "pedestal"
- Visual adjustments:
  - Lower contrast card background, more transparent border
  - Height reduced from 160px to 120px
  - Title and number font sizes reduced one level
  - Visually "recedes"

### ChannelRanking / ModelStats

- Position: below trend chart, keep existing side-by-side layout
- Same visual softening: smaller fonts, lighter borders
- Role: "inscriptions on the pedestal" — reference info, not attention-grabbing

### Pedestal–Armillary visual link

- Thin decorative line at top of pedestal area (`var(--border)` 0.5px)
- No energy channels — these are static reference, not part of core energy flow

## Layout & Responsiveness

### Desktop (≥1024px)

- Armillary core area: full width, centered
- Gear Clock SVG: keep `max-w-[520px]`
- 4 stat cards at four corners:
  - Request Stats → top-left
  - Overview → top-right
  - Input → bottom-left
  - Output → bottom-right
- Energy channel lines from card edges into SVG viewBox connecting to center
- Entire armillary area (gear + satellite cards): ~`max-w-[800px]`, generous vertical breathing room
- Tab switcher (day/week/month/year): above armillary, small pill style
- Pedestal panels below, max-width aligned with armillary

### Tablet (768px–1023px)

- Same as desktop but gear + cards scale down
- Stat cards may shift from four corners to two rows (two above, two below)

### Mobile (<768px)

- Armillary mode degrades:
  - Gear Clock centered but smaller
  - Stat cards fall back to 2×2 grid below gear
  - Energy channel lines hidden (too much visual noise)
  - Ring band switching still functional
- Pedestal panels stack vertically

## Visual Design Details

### Color Convergence

- Stat card colored backgrounds (`bg-blue-500/10`, `bg-emerald-500/10`, etc.) unified to `var(--muted)` or very faint `var(--primary)`
- `var(--nb-lime)` becomes the sole accent color, concentrated on gear core and energy channels
- Pedestal panels use `var(--muted-foreground)` as primary tone

### Typography Hierarchy

- Gear center numbers: largest, `font-weight: 900` (existing)
- Stat card values: medium, `text-lg font-bold` (slightly reduced)
- Pedestal panel values: smaller, `text-base font-semibold`
- Information density gradient decreasing from center outward

### Shadow & Depth

- Gear area: keep existing `drop-shadow` and glow effects
- Stat cards: remove or soften card shadow, make them "lighter"
- Pedestal panels: lightest shadow or no shadow

## Implementation Strategy

### Component Changes

- `HeroGearClock` — keep as-is, armillary core
- `ActivitySection` — refactor: lift tab-switch logic, manage ring highlight state
- `TotalSection` — change from independent grid to absolute/flex positioning around gear perimeter
- New `EnergyChannel` component — renders SVG connecting lines and pulse animation
- Week/Month/Year ring visualizations need new SVG rendering logic (map existing rectangular grids to arc segments)

### Scope

- No new pages or routes
- Refactor component arrangement within `dashboard.tsx`
- All new visuals are SVG + CSS animation (no 3D libraries)

## Context

The log viewer currently uses a row of 4 preset buttons (1h, 6h, 24h, 7d) for time filtering. Each button sets `from = now - preset.seconds` and clears `to`, meaning the range is always "from X ago until now" and drifts as time passes. There's no custom date/time selection. The project uses shadcn/ui (Radix primitives) with a neobrutalist theme. No date picker or Popover component exists yet.

The filter state lives in URL search params (`from`/`to` as unix seconds). Dashboard deep links already set these params. The `TIME_PRESETS` constant and related helpers are extracted in `log-filters.ts`.

## Goals / Non-Goals

**Goals:**

- Unified time range selection: presets + custom range in a single popover
- Add Popover primitive to the UI component library
- Presets set deterministic `from` AND `to` timestamps (snapshot at click time)
- Show human-readable range summary in the trigger button
- Add 30d preset
- Maintain neobrutalist visual consistency

**Non-Goals:**

- Calendar widget or date picker library (use native `datetime-local`)
- Timezone selection (continues using browser local timezone)
- Relative time display ("5 minutes ago" style)
- Time range persistence beyond URL params

## Decisions

### 1. Popover-based composition over standalone component

Use Radix `Popover` (via shadcn/ui) as the container. The trigger is a `Button` showing the range summary. The popover content splits into two areas:

- **Left column**: Preset buttons stacked vertically (1h, 6h, 24h, 7d, 30d)
- **Right area**: Two `datetime-local` inputs (From / To) with an Apply button

**Why not a custom calendar widget?** Native `datetime-local` inputs give us date+time selection for free with zero dependencies. They work across all modern browsers and provide platform-native UX. A calendar widget adds complexity (react-day-picker, date-fns) for minimal gain in this admin tool context.

### 2. Presets set both `from` and `to`

Currently presets only set `from` and clear `to`. This means the range silently extends as time passes. The new behavior: clicking a preset sets `from = now - seconds` AND `to = now`, creating a fixed window. This makes the filter deterministic and the displayed range accurate.

The preset trigger button closes the popover immediately (quick action). Custom range requires clicking "Apply" (deliberate action).

### 3. Range summary formatting

The trigger button displays:

- No range selected: "Time Range" (placeholder text with calendar icon)
- Preset active: "Last 1h", "Last 24h", etc. (match by checking if `to - from` equals a preset's `seconds` and `to` is within 60s of click time)
- Custom range: "MM/DD HH:mm – MM/DD HH:mm" (compact format)
- `from` only (legacy/deep link): "After MM/DD HH:mm"
- `to` only: "Before MM/DD HH:mm"

### 4. Component location

Place at `apps/web/src/components/time-range-picker.tsx` as a standalone component. It receives `from`/`to` (unix seconds or undefined) and an `onChange(from, to)` callback. This keeps it reusable and decoupled from the logs page's URL state management.

### 5. Popover primitive

Add `apps/web/src/components/ui/popover.tsx` from shadcn/ui's Radix Popover recipe. This is a standard primitive that other features may reuse.

## Risks / Trade-offs

- **`datetime-local` styling**: Native inputs don't perfectly match neobrutalist style. Mitigation: wrap in styled containers with consistent border/font treatment. Functional correctness is more important than pixel-perfect style matching for admin tools.
- **Mobile UX**: Popover may be awkward on small screens. Mitigation: use `align="end"` and let Radix handle collision detection. Mobile `datetime-local` inputs open native pickers which is actually good UX.
- **Preset detection heuristic**: Matching presets by duration (`to - from`) could false-positive on custom ranges that happen to equal a preset duration. Mitigation: also check that `to` is within 60s of current time. This is cosmetic only (affects label display, not functionality).

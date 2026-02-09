## Why

The current log viewer time filtering uses a flat row of preset buttons (1h, 6h, 24h, 7d) that only set a `from` timestamp relative to "now" — there's no way to select a custom time range or pick specific dates. This limits users who need to investigate incidents at specific times, compare time windows, or filter logs from days/weeks ago. The presets also don't include a 30d option and lack any visual indication of the selected range boundaries.

## What Changes

- Replace the inline preset buttons with a unified **TimeRangePicker** popover component
- Support both **preset ranges** (1h, 6h, 24h, 7d, 30d) and **custom date-time range** selection within the same UI
- Show a formatted summary of the active range in the trigger button (e.g., "Last 24h" or "Feb 3 14:00 – Feb 5 18:00")
- Add proper `from` and `to` boundary display — presets now set both `from` AND `to` (to = now at click time) for deterministic filtering
- Dashboard deep link time ranges render correctly in the picker

## Capabilities

### New Capabilities

- `time-range-picker`: A reusable popover component combining quick presets with dual datetime-local inputs for custom range selection, designed in the project's neobrutalist style

### Modified Capabilities

_(none — no existing spec-level requirements change)_

## Impact

- **UI Component**: New `TimeRangePicker` component in `apps/web/src/components/`
- **Log Page**: Replace preset button row in `apps/web/src/app/(protected)/logs/page.tsx`
- **Filter Utils**: Update `TIME_PRESETS` in `log-filters.ts` to include 30d, update related tests
- **Dependencies**: Add `Popover` UI primitive (from shadcn/ui/Radix) — no heavy date library needed since `datetime-local` inputs handle date selection natively
- **Dashboard Deep Links**: No changes needed — they already set `from`/`to` URL params which the picker will read

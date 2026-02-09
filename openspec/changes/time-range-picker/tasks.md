## 1. Popover UI Primitive

- [x] 1.1 Add `@radix-ui/react-popover` dependency to `apps/web` — SKIPPED: already available via unified `radix-ui` package
- [x] 1.2 Create `apps/web/src/components/ui/popover.tsx` — export `Popover`, `PopoverTrigger`, `PopoverContent` following shadcn/ui pattern with neobrutalist border/shadow styling

## 2. TimeRangePicker Component

- [x] 2.1 Create `apps/web/src/components/time-range-picker.tsx` with props: `from?: number`, `to?: number`, `onChange: (from?: number, to?: number) => void`
- [x] 2.2 Implement trigger button — shows range summary text with Calendar icon, clear (×) button when range is active
- [x] 2.3 Implement range summary formatting: "Time Range" placeholder, "Last {label}" for presets, "MM/DD HH:mm – MM/DD HH:mm" for custom ranges
- [x] 2.4 Implement popover layout — left column with vertical preset buttons (1h, 6h, 24h, 7d, 30d), right area with From/To datetime-local inputs and Apply button
- [x] 2.5 Wire preset button clicks — set `from = now - seconds`, `to = now`, call `onChange`, close popover
- [x] 2.6 Wire custom range Apply — convert datetime-local values to unix seconds, call `onChange`, close popover
- [x] 2.7 Pre-fill datetime-local inputs from current `from`/`to` props when popover opens
- [x] 2.8 Wire clear button — call `onChange(undefined, undefined)`

## 3. Update Filter Utils

- [x] 3.1 Add 30d preset to `TIME_PRESETS` in `log-filters.ts` — `{ label: "30d", seconds: 2592000 }`
- [x] 3.2 Replace separate `from`/`to` filter chips with a single "Time" chip showing the range summary
- [x] 3.3 Update existing `TIME_PRESETS` tests to cover the new 30d entry

## 4. Integrate into Logs Page

- [x] 4.1 Replace the preset buttons row in `page.tsx` with `<TimeRangePicker from={startTime} to={endTime} onChange={...} />`
- [x] 4.2 Wire `onChange` to call `updateFilter({ from, to })` — clearing both when both are undefined
- [x] 4.3 Remove the inline time preset rendering code and the clear button that are now handled by the component

## 5. Tests

- [x] 5.1 Write unit tests for range summary formatting — preset detection, custom range display, placeholder, from-only, to-only
- [x] 5.2 Write unit tests for preset timestamp computation — verify `from` and `to` values for each preset
- [x] 5.3 Update existing `log-filters.test.ts` to reflect 30d preset addition

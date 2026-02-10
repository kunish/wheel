## 1. Backend: Keyword Search

- [x] 1.1 Add `keyword` parameter to `listLogs` DAL (`apps/worker/src/db/dal/logs.ts`) — LIKE search across `requestModelName`, `channelName`, `error`, `requestContent`, `responseContent` with OR logic
- [x] 1.2 Wire `keyword` query param in log route (`apps/worker/src/routes/log.ts`)
- [x] 1.3 Update `listLogs` API client params type (`apps/web/src/lib/api.ts`)

## 2. Backend: Replay Endpoint

- [x] 2.1 Add `POST /api/v1/log/replay/:id` route in `apps/worker/src/routes/log.ts` — reads stored log, parses `requestContent`, forwards through relay pipeline
- [x] 2.2 Handle truncation detection — include warning in response when `requestContent` contains truncation markers
- [x] 2.3 Support streaming and non-streaming replay responses
- [x] 2.4 Add `replayLog` API client function in `apps/web/src/lib/api.ts`

## 3. Backend: Test Setup & Tests

- [x] 3.1 Add Vitest config to `apps/worker` (`vitest.config.ts` + `package.json` scripts)
- [x] 3.2 Write unit tests for `listLogs` DAL — test keyword filtering, channelId filter, time range filter, combined filters, empty results
- [x] 3.3 Write unit tests for replay route — test log not found (404), truncation detection, valid replay request parsing

## 4. Frontend: URL-based Filter State

- [x] 4.1 Refactor `LogsPage` to read initial filter state from `useSearchParams` (model, channel, status, from, to, q, page, size)
- [x] 4.2 Sync filter state changes to URL via `useRouter().replace()` — only include non-default values
- [x] 4.3 Reset page to 1 when any filter changes

## 5. Frontend: Enhanced Filter Bar

- [x] 5.1 Replace current model `Input` with unified search input — debounced (300ms), search icon, full-width top row
- [x] 5.2 Add Channel filter `Select` dropdown — fetch channel list from `listChannels`, show "All Channels" default
- [x] 5.3 Add time range quick preset buttons (1h, 6h, 24h, 7d) + custom datetime-local inputs
- [x] 5.4 Add page size selector (20/50/100) in pagination area
- [x] 5.5 Add active filter chips as removable Badges below filter bar — show filter type and value, × to remove

## 6. Frontend: Table UX Improvements

- [x] 6.1 Add error row styling — `border-l-2 border-destructive` + `bg-destructive/5` for rows with non-empty `error`
- [x] 6.2 Add error preview Tooltip on "Error" badge hover — show first 200 chars of error text
- [x] 6.3 Add client-side column sorting for Latency, Cost, Input Tokens, Output Tokens — click header toggles asc/desc, show arrow indicator
- [x] 6.4 Add empty state component — "No logs match your filters" message with "Clear all filters" button

## 7. Frontend: Detail Side Panel

- [x] 7.1 Replace Dialog with Sheet (right-side, `max-w-2xl`) — migrate all detail content (Overview, Request, Response, Retry tabs)
- [x] 7.2 Add prev/next navigation buttons in panel header — navigate adjacent logs in current filtered list
- [x] 7.3 Add in-content search input in Request/Response tabs — highlight matches, show match count
- [x] 7.4 Add copy buttons for individual Overview fields (Model, Channel, Error, etc.)
- [x] 7.5 Add Replay button in Overview tab — call `POST /api/v1/log/replay/:id`, show loading state, display result in new "Replay Result" tab
- [x] 7.6 Show truncation warning before replay when requestContent contains truncation markers

## 8. Frontend: Dashboard Deep Linking

- [x] 8.1 Make Model Usage model names clickable → navigate to `/logs?model={modelName}`
- [x] 8.2 Make Channel Ranking channel names clickable → navigate to `/logs?channel={channelId}`
- [x] 8.3 Make Activity heatmap day cells clickable → navigate to `/logs?from={dayStart}&to={dayEnd}`
- [x] 8.4 Make Activity heatmap hour cells clickable → navigate to `/logs?from={hourStart}&to={hourEnd}`

## 9. Frontend: Tests

- [x] 9.1 Write unit tests for URL filter state parsing — test initial state from search params, default values, encoding/decoding
- [x] 9.2 Write unit tests for debounce search hook — test 300ms debounce, rapid typing, clear behavior
- [x] 9.3 Write unit tests for client-side column sorting — test ascending/descending toggle, switching columns, stable sort
- [x] 9.4 Write unit tests for time range preset calculation — test 1h/6h/24h/7d preset timestamp computation
- [x] 9.5 Write unit tests for filter chip generation — test active filter detection, chip labels, removal logic
- [x] 9.6 Write unit tests for in-content search highlighting — test match counting, keyword highlighting, no matches case

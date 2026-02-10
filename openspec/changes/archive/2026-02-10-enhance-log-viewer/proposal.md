## Why

The current log viewer has minimal filtering — only model name text search and a success/error status dropdown. When diagnosing issues across hundreds of logs, there's no way to narrow by channel, time range, or search within request/response content. The backend already supports `channelId`, `startTime`, and `endTime` filtering that the frontend doesn't expose, making this low-hanging fruit for a significant UX improvement.

## What Changes

**Search & Filtering**

- Add a unified search bar that searches across model names, channel names, error messages, and request/response content
- Add channel filter dropdown (using existing `listChannels` API + backend `channelId` support)
- Add time range filter with quick presets (1h, 6h, 24h, 7d) and custom range picker
- Display active filters as removable chips above the table
- Backend: add `keyword` parameter for full-text search across request/response content and error messages

**Table Experience**

- Highlight error rows with a colored left border and inline error preview on hover
- Add adjustable page size (20 / 50 / 100)
- Add column sorting for Latency, Cost, and Token columns
- Improve empty state with contextual guidance when filters return no results

**Detail Panel**

- Replace the modal Dialog with a slide-out side panel to preserve table context
- Add prev/next navigation within the detail panel to browse adjacent logs
- Add in-content search (highlight keywords within request/response JSON/text)
- Add request replay: backend endpoint `POST /api/v1/log/replay/:id` resends the stored request through the relay pipeline and returns the response

**Cross-page Navigation**

- Dashboard elements (model stats, channel stats, error counts) link to pre-filtered log views
- Clicking a model in dashboard navigates to `/logs?model=xxx`
- Clicking a channel navigates to `/logs?channel=xxx`
- Clicking error count navigates to `/logs?status=error`
- Log page reads URL query params to initialize filters on load

## Capabilities

### New Capabilities

- `log-search-filter`: Advanced search and multi-dimensional filtering for the log viewer (unified search, channel filter, time range, filter chips)
- `log-table-ux`: Table experience improvements (error highlighting, page size control, column sorting, empty states)
- `log-detail-panel`: Redesigned detail panel as a side sheet with in-content navigation, search, and request replay
- `log-deep-linking`: URL-based filter state and cross-page navigation from Dashboard to filtered log views

### Modified Capabilities

None — no existing specs are affected.

## Impact

- **Frontend**: `apps/web/src/app/(protected)/logs/page.tsx` — major rewrite of filters, table, and detail view
- **Frontend**: `apps/web/src/app/(protected)/dashboard/page.tsx` — add clickable links to log views
- **Backend API**: `apps/worker/src/routes/log.ts` — add `keyword` search parameter and `POST /replay/:id` endpoint
- **Backend DAL**: `apps/worker/src/db/dal/logs.ts` — add full-text search condition in `listLogs`
- **API Client**: `apps/web/src/lib/api.ts` — update `listLogs` params type, add replay API
- **No breaking changes** — all additions are backwards-compatible

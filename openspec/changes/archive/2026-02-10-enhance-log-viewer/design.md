## Context

The log viewer (`apps/web/src/app/(protected)/logs/page.tsx`, ~642 lines) currently provides basic model text search and status filtering. The backend already supports `channelId`, `startTime`, `endTime` parameters that the frontend doesn't expose. The UI uses a modal Dialog for log detail which blocks the table context. Dashboard page has no links to filtered log views.

Key constraints:

- Backend is Hono on Cloudflare Workers with D1/SQLite — no full-text search engine available
- Log content is truncated during storage (`MAX_LOG_JSON=10000`, `MAX_MESSAGE_CONTENT=500`)
- Sheet component (`components/ui/sheet.tsx`) already exists and can be used for side panel
- Next.js App Router with client-side state management via React Query + Zustand

## Goals / Non-Goals

**Goals:**

- Enable fast issue diagnosis via multi-dimensional filtering (model, channel, time range, keyword)
- Improve log browsing flow with side panel that preserves table context
- Connect Dashboard to Log views for drill-down analysis
- Support request replay from log detail for debugging
- Keep all filter state in URL for shareability and cross-page navigation

**Non-Goals:**

- Full-text search engine integration (Meilisearch, etc.) — LIKE-based search is sufficient for the scale
- Log aggregation / pattern grouping (Phase 4 future work)
- Real-time log tailing / auto-scroll mode (Phase 4 future work)
- Export functionality (Phase 4 future work)
- Modifying the relay handler or log storage format (replay reuses existing relay functions but doesn't change them)

## Decisions

### 1. URL-based filter state with `nuqs` or manual `useSearchParams`

**Decision**: Use Next.js `useSearchParams` + `useRouter` directly (no extra dependency).

**Rationale**: The filter set is small (model, channel, status, startTime, endTime, keyword, page, pageSize). `nuqs` adds a dependency for minimal benefit. `useSearchParams` is built-in and sufficient.

**Alternative considered**: `nuqs` library — adds type-safe URL state, but the project doesn't currently use it and the filter schema is simple enough to manage manually.

**URL schema**: `/logs?model=xxx&channel=123&status=error&from=1707000000&to=1707086400&q=timeout&page=1&size=20`

### 2. Keyword search implementation (backend)

**Decision**: Add a `keyword` parameter to `listLogs` DAL that uses `LIKE` across `requestModelName`, `channelName`, `error`, `requestContent`, and `responseContent` columns.

**Rationale**: SQLite `LIKE` is adequate for the expected data volume (single-user admin tool). No FTS5 index needed. The query will be: `(col1 LIKE '%keyword%' OR col2 LIKE '%keyword%' OR ...)`.

**Alternative considered**: SQLite FTS5 — more performant for large datasets but adds schema complexity and migration burden for a low-volume admin tool.

### 3. Side panel vs. modal for log detail

**Decision**: Replace Dialog with Sheet (side panel, right-side, width `max-w-2xl`). Use the existing `Sheet` component from `components/ui/sheet.tsx`.

**Rationale**: Side panel keeps the table visible, enabling sequential log browsing without losing context. The current Dialog blocks the entire view. The Sheet component is already in the project.

**Alternative considered**: Split-view (resizable panels) — more complex, introduces layout shift, and the existing Sheet component provides the needed behavior out of the box.

### 4. Request replay approach

**Decision**: Add a backend replay endpoint `POST /api/v1/log/replay/:id` that reads the stored log entry, reconstructs the request, and forwards it through the existing relay pipeline on the server side. The endpoint returns the relay response (streamed or non-streamed) to the frontend.

**Rationale**: Backend replay keeps API keys and relay logic server-side — the frontend never needs to hold relay credentials. The backend can directly read the stored `requestContent`, parse it, and pipe it through the same relay handler code (`detectRequestType`, `extractModel`, `proxyStreaming`/`proxyNonStreaming`). This is cleaner than having the frontend reconstruct and send a request to `/v1/chat/completions`.

**Implementation**:

- New route: `POST /api/v1/log/replay/:id` in `apps/worker/src/routes/log.ts`
- Reads the log entry's `requestContent` and `requestModelName`
- Calls the relay pipeline internally (reuse `proxyStreaming`/`proxyNonStreaming`)
- Returns the response (supports streaming via SSE)
- Frontend shows a "Replay" button in detail panel; response displayed in a new "Replay" tab

**Caveats**:

- Log content may be truncated during storage — show a warning when replaying truncated content
- Replay uses the current channel/group configuration, which may differ from the original

**Alternative considered**: Frontend replay via `/v1/chat/completions` — simpler but exposes relay API keys to the browser and requires the admin user to also have an API key configured.

### 5. Dashboard → Logs deep linking

**Decision**: Add `Link` wrappers around clickable elements in the Dashboard page that navigate to `/logs?param=value`:

| Dashboard element               | Link target                           |
| ------------------------------- | ------------------------------------- |
| Model name in Model Usage       | `/logs?model={modelName}`             |
| Channel name in Channel Ranking | `/logs?channel={channelId}`           |
| Activity heatmap cell (day)     | `/logs?from={dayStart}&to={dayEnd}`   |
| Activity heatmap cell (hour)    | `/logs?from={hourStart}&to={hourEnd}` |

**Rationale**: Using standard Next.js `Link` with query params. The Logs page reads `searchParams` on mount to initialize filter state. No new API needed.

### 6. Channel filter data source

**Decision**: Reuse the `listChannels` API already used by the channels management page. Fetch channel list on log page mount, use it to populate a `Select` dropdown.

**Rationale**: No new endpoint needed. Channel list is typically small (tens of items). Cache with React Query to avoid refetching.

### 7. Time range picker

**Decision**: Quick preset buttons (1h, 6h, 24h, 7d) + two `<input type="datetime-local">` for custom range. No external date picker library.

**Rationale**: `datetime-local` inputs provide native date/time selection. Quick presets cover the most common use cases. Avoids adding a date picker dependency.

**Alternative considered**: `react-day-picker` or `date-fns` — heavier, and the native inputs work well for an admin tool.

### 8. Filter bar layout

**Decision**: Two-row filter bar:

- **Row 1**: Unified search input (full width) with search icon
- **Row 2**: Channel select, Status select, Time range presets, Page size select — all inline

Active filters display as removable Badge chips below the filter bar when any non-default filter is active.

### 9. Table enhancements

**Decision**:

- Error rows: left `border-l-2 border-destructive` + row background `bg-destructive/5`
- Error preview: Tooltip on the error Badge showing truncated error text
- Page size: Select dropdown in pagination area (20/50/100)
- Column sorting: Frontend-only sort on current page data (not server-side). Click column header to toggle asc/desc. Only Latency, Cost, Input/Output tokens are sortable.

**Rationale**: Frontend sorting is simpler to implement and adequate since users are viewing one page at a time. Server-side sorting would require backend changes and provide minimal benefit for 20-100 row pages.

## Risks / Trade-offs

- **[LIKE search performance]** → SQLite `LIKE '%keyword%'` doesn't use indexes. Mitigation: acceptable for admin tool scale; add note that FTS5 can be added later if needed.
- **[Truncated replay content]** → Stored request may be incomplete. Mitigation: show clear warning "Content was truncated during storage, replay may produce different results".
- **[URL param pollution]** → Many filters = long URLs. Mitigation: only include non-default values in URL; clear all button resets URL.
- **[Frontend sort inconsistency]** → Sorting only the current page may confuse users expecting global sort. Mitigation: tooltip explaining "sorted within current page" or consider server-side sort for a future iteration.

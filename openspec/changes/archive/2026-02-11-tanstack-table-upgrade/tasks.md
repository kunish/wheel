## 1. Setup

- [x] 1.1 Install `@tanstack/react-table` dependency in `apps/web`
- [x] 1.2 Verify build passes with new dependency

## 2. Column Definitions

- [x] 2.1 Create `apps/web/src/app/(protected)/logs/columns.tsx` with all column definitions using `createColumnHelper<LogEntry>()`
- [x] 2.2 Configure sorting on Input Tokens, Output Tokens, TTFT (`ftut`), Latency (`useTime`), Cost columns via `enableSorting: true`
- [x] 2.3 Migrate cell renderers (formatTime, formatDuration, formatCost, ModelBadge, status Badge, tooltips) from page.tsx into column definitions

## 3. Table Instance Integration

- [x] 3.1 Replace manual `sortField`/`sortDir` state with TanStack Table `SortingState` in LogsPage
- [x] 3.2 Replace manual `grouping` state with TanStack Table `GroupingState` in LogsPage
- [x] 3.3 Create `useReactTable()` instance with `getCoreRowModel`, `getSortedRowModel`, `getGroupedRowModel`, `getExpandedRowModel`
- [x] 3.4 Configure `enableSortingRemoval: true` for three-state sort cycling (asc → desc → none)

## 4. Table Rendering

- [x] 4.1 Refactor table header rendering to use `table.getHeaderGroups()` + `flexRender` with existing shadcn/ui `TableHead` components
- [x] 4.2 Refactor table body rendering to use `table.getRowModel().rows` + `flexRender` with existing shadcn/ui `TableRow`/`TableCell` components
- [x] 4.3 Update `SortableHead` component (or replace with inline header render) to use `column.getToggleSortingHandler()` and `column.getIsSorted()` for three-state icon display
- [x] 4.4 Preserve error row styling (`border-l-destructive bg-destructive/5`) and click-to-detail behavior on data rows

## 5. Grouping UI

- [x] 5.1 Add grouping control (Select/DropdownMenu) in the filter bar area with options: None, Model, Channel
- [x] 5.2 Render group header rows with expand/collapse toggle, group name, and row count
- [x] 5.3 Handle grouped + sorted state: ensure intra-group sorting works correctly

## 6. Detail Panel & Navigation

- [x] 6.1 Update detail panel prev/next navigation to use `table.getRowModel().rows` instead of `sortedLogs`
- [x] 6.2 Verify navigation works correctly with grouping enabled (skip group header rows)

## 7. Cleanup

- [x] 7.1 Remove `sortLogs` function from `log-filters.ts`
- [x] 7.2 Remove `SortField` and `SortDir` types from `page.tsx`
- [x] 7.3 Remove old `toggleSort`, `sortField`, `sortDir` state from `page.tsx`
- [x] 7.4 Remove unused imports (`ArrowUp`, `ArrowDown`, `ArrowUpDown` if handled by column defs)
- [x] 7.5 Update `LogTableSkeleton` to include TTFT column sort indicator placeholder if needed

## 8. Verification

- [x] 8.1 Verify three-state sort cycling works on all 5 sortable columns (Input, Output, TTFT, Latency, Cost)
- [x] 8.2 Verify TTFT sorting correctly handles zero values
- [x] 8.3 Verify grouping by Model and Channel displays correct groups with counts
- [x] 8.4 Verify WebSocket real-time log push still works (new rows appear and re-sort/re-group)
- [x] 8.5 Verify detail panel navigation works in sorted, grouped, and default states
- [x] 8.6 Verify empty state and skeleton loading still render correctly

import type { ExpandedState, GroupingState, SortingState } from "@tanstack/react-table"
import type { LogEntry } from "./logs/columns"
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  flexRender,
  getCoreRowModel,
  getExpandedRowModel,
  getGroupedRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table"
import {
  AlertCircle,
  ArrowUp,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Copy,
  FileText,
  Loader2,
  Play,
  RefreshCw,
  Search,
  X,
} from "lucide-react"
import { useTheme } from "next-themes"
import * as React from "react"
import { lazy, Suspense, useCallback, useMemo, useRef, useState } from "react"
import { Link, useLocation, useNavigate, useSearchParams } from "react-router"
import { toast } from "sonner"
import { ModelBadge } from "@/components/model-badge"
import { ModelCombobox } from "@/components/model-combobox"
import { formatRangeSummary, TimeRangePicker } from "@/components/time-range-picker"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { TooltipProvider } from "@/components/ui/tooltip"
import { useModelMeta } from "@/hooks/use-model-meta"
import { useWsEvent } from "@/hooks/use-stats-ws"
import {
  listChannels as apiListChannels,
  getLog,
  getModelList,
  listLogs,
  replayLog,
} from "@/lib/api"
import { createLogColumns, formatCost, formatDuration } from "./logs/columns"
import { buildFilterSearchParams, countMatches, parseLogFilters } from "./logs/log-filters"

// Hoisted constant arrays to avoid re-creation on every render
const DETAIL_SKELETON_ITEMS = Array.from({ length: 9 })
const SKELETON_ROWS_8 = Array.from({ length: 8 })
const SKELETON_ROWS_10 = Array.from({ length: 10 })

// --- Dynamic imports: heavy libs only needed inside the detail panel ---
const LazyJsonView = lazy(() =>
  Promise.all([
    import("@uiw/react-json-view"),
    import("@uiw/react-json-view/githubDark"),
    import("@uiw/react-json-view/githubLight"),
  ]).then(([mod, dark, light]) => {
    const JsonView = mod.default
    function LazyJsonViewInner(props: React.ComponentProps<typeof JsonView> & { isDark: boolean }) {
      const { isDark, style, ...rest } = props
      return (
        <JsonView
          {...rest}
          style={{
            ...(isDark ? dark.githubDarkTheme : light.githubLightTheme),
            ...style,
          }}
        />
      )
    }
    LazyJsonViewInner.displayName = "LazyJsonView"
    return { default: LazyJsonViewInner }
  }),
)

const LazyMarkdown = lazy(() =>
  Promise.all([import("react-markdown"), import("remark-gfm")]).then(([md, gfm]) => {
    const ReactMarkdown = md.default
    const remarkGfm = gfm.default
    function LazyMarkdownInner({ children }: { children: string }) {
      return <ReactMarkdown remarkPlugins={[remarkGfm]}>{children}</ReactMarkdown>
    }
    LazyMarkdownInner.displayName = "LazyMarkdown"
    return { default: LazyMarkdownInner }
  }),
)

/** Best-effort repair of truncated JSON (e.g. from log storage limits) */
function repairTruncatedJson(text: string): { data: unknown; truncated: boolean } | null {
  if (!text.startsWith("{") && !text.startsWith("[")) return null
  try {
    let repaired = text
    const opens = (repaired.match(/[{[]/g) || []).length
    const closes = (repaired.match(/[}\]]/g) || []).length
    const lastComma = Math.max(
      repaired.lastIndexOf(","),
      repaired.lastIndexOf("}"),
      repaired.lastIndexOf("]"),
    )
    if (lastComma > 0) {
      repaired = repaired.slice(0, lastComma + 1)
    }
    for (let i = 0; i < opens - closes; i++) {
      repaired += text.startsWith("{") ? "}" : "]"
    }
    return { data: JSON.parse(repaired), truncated: true }
  } catch {
    return null
  }
}

// Hoist row model factories outside the component to maintain stable references
const coreRowModel = getCoreRowModel<LogEntry>()
const sortedRowModel = getSortedRowModel<LogEntry>()
const groupedRowModel = getGroupedRowModel<LogEntry>()
const expandedRowModel = getExpandedRowModel<LogEntry>()

interface LogDetail {
  id: number
  time: number
  requestModelName: string
  actualModelName: string
  channelName: string
  channelId: number
  inputTokens: number
  outputTokens: number
  cost: number
  ftut: number
  useTime: number
  requestContent: string
  upstreamContent: string | null
  responseContent: string
  error: string
  attempts: Array<{
    channelId: number
    channelKeyId?: number
    channelName: string
    modelName: string
    attemptNum: number
    status: "success" | "failed" | "circuit_break" | "skipped"
    duration: number
    sticky?: boolean
    msg?: string
  }>
  totalAttempts: number
}

export default function LogsPage() {
  const queryClient = useQueryClient()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const { pathname } = useLocation()

  // Derive filter state from URL search params
  const filters = parseLogFilters(searchParams)
  const { page, model, status, channelId, keyword, pageSize, startTime, endTime } = filters

  // Local state for controlled text inputs (synced to URL via debounce)
  const [keywordInput, setKeywordInput] = useState(keyword)

  // Sync local input state when URL changes externally (e.g., deep links from dashboard)
  const prevKeywordRef = useRef(keyword)
  if (prevKeywordRef.current !== keyword) {
    prevKeywordRef.current = keyword
    setKeywordInput(keyword)
  }

  // Helper to update URL search params — resets page to 1 unless page itself is being updated
  const updateFilter = useCallback(
    (updates: Record<string, string | number | undefined | null>) => {
      const params = buildFilterSearchParams(searchParams, updates)
      const query = params.toString()
      navigate(query ? `${pathname}?${query}` : pathname, { replace: true })
    },
    [searchParams, pathname, navigate],
  )

  // Debounced sync for text inputs
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(null)
  const debouncedUpdateFilter = useCallback(
    (key: string, value: string) => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
      debounceRef.current = setTimeout(() => {
        updateFilter({ [key]: value || undefined })
      }, 300)
    },
    [updateFilter],
  )

  const [detailId, setDetailId] = useState<number | null>(null)
  const [pendingCount, setPendingCount] = useState(0)
  const [sorting, setSorting] = useState<SortingState>([])
  const [grouping, setGrouping] = useState<GroupingState>([])
  const [expanded, setExpanded] = useState<ExpandedState>(true)

  const hasFilters =
    model !== "" ||
    status !== "all" ||
    keyword !== "" ||
    channelId !== undefined ||
    startTime !== undefined
  const isFirstPage = page === 1

  const { data, isLoading, isFetching, isError, refetch } = useQuery({
    queryKey: ["logs", page, pageSize, model, status, channelId, keyword, startTime, endTime],
    queryFn: () =>
      listLogs({
        page,
        pageSize,
        ...(model ? { model } : {}),
        ...(status !== "all" ? { status } : {}),
        ...(channelId ? { channelId } : {}),
        ...(keyword ? { keyword } : {}),
        ...(startTime ? { startTime } : {}),
        ...(endTime ? { endTime } : {}),
      }),
    placeholderData: keepPreviousData,
  })

  const { data: detailData } = useQuery({
    queryKey: ["log-detail", detailId],
    queryFn: () => getLog(detailId!),
    enabled: detailId !== null,
  })

  const { data: channelsData } = useQuery({
    queryKey: ["channels-for-filter"],
    queryFn: apiListChannels,
    staleTime: 5 * 60 * 1000,
  })
  const channels = (channelsData?.data?.channels ?? []) as Array<{ id: number; name: string }>

  const { data: modelsData } = useQuery({
    queryKey: ["models-for-filter"],
    queryFn: getModelList,
    staleTime: 5 * 60 * 1000,
  })
  const modelOptions = (modelsData?.data?.models ?? []) as string[]

  // Listen for log-created WebSocket events (reuses global WS connection)
  const filtersRef = useRef({
    page,
    pageSize,
    model,
    status,
    channelId,
    keyword,
    startTime,
    endTime,
    isFirstPage,
    hasFilters,
  })
  filtersRef.current = {
    page,
    pageSize,
    model,
    status,
    channelId,
    keyword,
    startTime,
    endTime,
    isFirstPage,
    hasFilters,
  }

  useWsEvent("log-created", (data) => {
    if (!data?.log) return
    const f = filtersRef.current
    if (f.isFirstPage && !f.hasFilters) {
      queryClient.setQueryData(
        [
          "logs",
          f.page,
          f.pageSize,
          f.model,
          f.status,
          f.channelId,
          f.keyword,
          f.startTime,
          f.endTime,
        ],
        (
          old:
            | { data?: { logs: LogEntry[]; total: number; page: number; pageSize: number } }
            | undefined,
        ) => {
          if (!old?.data) return old
          const newLogs = [data.log as LogEntry, ...old.data.logs].slice(0, f.pageSize)
          return {
            ...old,
            data: {
              ...old.data,
              logs: newLogs,
              total: old.data.total + 1,
            },
          }
        },
      )
    } else {
      setPendingCount((c) => c + 1)
    }
  })

  // Reset pending count when navigating to page 1 or clearing filters
  const prevFirstPageRef = useRef(isFirstPage)
  const prevHasFiltersRef = useRef(hasFilters)
  if (prevFirstPageRef.current !== isFirstPage || prevHasFiltersRef.current !== hasFilters) {
    prevFirstPageRef.current = isFirstPage
    prevHasFiltersRef.current = hasFilters
    if (isFirstPage && !hasFilters) {
      setPendingCount(0)
    }
  }

  const handleShowNew = useCallback(() => {
    // Clear all filters and go to page 1
    navigate(pathname, { replace: true })
    setKeywordInput("")
    setPendingCount(0)
    queryClient.invalidateQueries({ queryKey: ["logs"] })
  }, [queryClient, navigate, pathname])

  const logs = useMemo(() => (data?.data?.logs ?? []) as LogEntry[], [data])
  const total = data?.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)
  const detail = (detailData?.data ?? null) as LogDetail | null

  const columns = useMemo(() => createLogColumns(setDetailId), [])

  const table = useReactTable({
    data: logs,
    columns,
    state: { sorting, grouping, expanded },
    onSortingChange: setSorting,
    onGroupingChange: setGrouping,
    onExpandedChange: setExpanded,
    enableSortingRemoval: true,
    getCoreRowModel: coreRowModel,
    getSortedRowModel: sortedRowModel,
    getGroupedRowModel: groupedRowModel,
    getExpandedRowModel: expandedRowModel,
  })

  // Flat data rows for detail panel navigation (skip group headers)
  const visibleRows = useMemo(
    () => table.getRowModel().rows.filter((r) => !r.getIsGrouped()),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [table.getRowModel().rows],
  )

  // Pagination controls (reused top and bottom)
  const paginationControls = (
    <div className="flex items-center gap-2">
      <Select value={String(pageSize)} onValueChange={(v) => updateFilter({ size: v })}>
        <SelectTrigger className="h-8 w-20 text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="20">20</SelectItem>
          <SelectItem value="50">50</SelectItem>
          <SelectItem value="100">100</SelectItem>
        </SelectContent>
      </Select>
      <span className="text-muted-foreground text-sm tabular-nums">
        {page}/{totalPages || 1}
      </span>
      <Button
        variant="outline"
        size="icon-xs"
        disabled={page <= 1}
        onClick={() => updateFilter({ page: page - 1 })}
      >
        <ChevronLeft className="h-3.5 w-3.5" />
      </Button>
      <Button
        variant="outline"
        size="icon-xs"
        disabled={page >= totalPages}
        onClick={() => updateFilter({ page: page + 1 })}
      >
        <ChevronRight className="h-3.5 w-3.5" />
      </Button>
    </div>
  )

  return (
    <div className="flex flex-col gap-4">
      {/* Header: Title + Total + Pagination */}
      <div className="flex items-center justify-between">
        <div className="flex items-baseline gap-3">
          <h2 className="text-2xl font-bold tracking-tight">Logs</h2>
          <span className="text-muted-foreground text-sm">{total.toLocaleString()} total</span>
          {pendingCount > 0 && (
            <Button
              variant="outline"
              size="xs"
              className="animate-pulse gap-1"
              onClick={handleShowNew}
            >
              <ArrowUp className="h-3 w-3" />
              {pendingCount} new
            </Button>
          )}
        </div>
        {totalPages > 0 && paginationControls}
      </div>

      {/* Filter Bar */}
      <div className="flex flex-col gap-2">
        {/* Filters: search, model, channel, status, time */}
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative min-w-[200px] flex-1">
            <Search className="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
            <Input
              placeholder="Search logs..."
              value={keywordInput}
              onChange={(e) => {
                setKeywordInput(e.target.value)
                debouncedUpdateFilter("q", e.target.value)
              }}
              className="pl-9"
            />
            {keywordInput && (
              <Button
                variant="ghost"
                size="icon"
                className="absolute top-1/2 right-1 h-7 w-7 -translate-y-1/2"
                onClick={() => {
                  setKeywordInput("")
                  updateFilter({ q: undefined })
                }}
              >
                <X className="h-3 w-3" />
              </Button>
            )}
          </div>
          <ModelCombobox
            models={modelOptions}
            value={model}
            onChange={(v) => updateFilter({ model: v || undefined })}
          />
          <Select
            value={channelId ? String(channelId) : "all"}
            onValueChange={(v) => {
              updateFilter({ channel: v === "all" ? undefined : v })
            }}
          >
            <SelectTrigger className="w-36">
              <SelectValue placeholder="All Channels" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Channels</SelectItem>
              {channels.map((ch) => (
                <SelectItem key={ch.id} value={String(ch.id)}>
                  {ch.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select
            value={status}
            onValueChange={(v) => {
              updateFilter({ status: v })
            }}
          >
            <SelectTrigger className="w-28">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All</SelectItem>
              <SelectItem value="success">Success</SelectItem>
              <SelectItem value="error">Error</SelectItem>
            </SelectContent>
          </Select>
          <TimeRangePicker
            from={startTime}
            to={endTime}
            onChange={(from, to) => updateFilter({ from: from ?? undefined, to: to ?? undefined })}
          />
        </div>

        {/* Active filter chips */}
        {hasFilters && (
          <div className="flex flex-wrap gap-1.5">
            {keyword && (
              <Badge variant="secondary" className="gap-1 pr-1">
                Search: {keyword}
                <button
                  onClick={() => {
                    setKeywordInput("")
                    updateFilter({ q: undefined })
                  }}
                >
                  <X className="h-3 w-3" />
                </button>
              </Badge>
            )}
            {model && (
              <Badge variant="secondary" className="gap-1 pr-1">
                Model: {model}
                <button
                  onClick={() => {
                    updateFilter({ model: undefined })
                  }}
                >
                  <X className="h-3 w-3" />
                </button>
              </Badge>
            )}
            {channelId && (
              <Badge variant="secondary" className="gap-1 pr-1">
                Channel: {channels.find((c) => c.id === channelId)?.name ?? channelId}
                <button onClick={() => updateFilter({ channel: undefined })}>
                  <X className="h-3 w-3" />
                </button>
              </Badge>
            )}
            {status !== "all" && (
              <Badge variant="secondary" className="gap-1 pr-1">
                Status: {status}
                <button onClick={() => updateFilter({ status: "all" })}>
                  <X className="h-3 w-3" />
                </button>
              </Badge>
            )}
            {(startTime || endTime) && (
              <Badge variant="secondary" className="gap-1 pr-1">
                Time: {formatRangeSummary(startTime, endTime)}
                <button onClick={() => updateFilter({ from: undefined, to: undefined })}>
                  <X className="h-3 w-3" />
                </button>
              </Badge>
            )}
            <Button
              variant="ghost"
              size="sm"
              className="h-6 px-2 text-xs"
              onClick={() => {
                navigate(pathname, { replace: true })
                setKeywordInput("")
              }}
            >
              Clear all
            </Button>
          </div>
        )}
      </div>

      {isError ? (
        <Card className="flex flex-col items-center justify-center gap-3 py-12">
          <AlertCircle className="text-destructive h-8 w-8" />
          <p className="text-muted-foreground text-sm">Failed to load logs</p>
          <Button variant="outline" size="sm" className="gap-1.5" onClick={() => refetch()}>
            <RefreshCw className="h-3.5 w-3.5" />
            Retry
          </Button>
        </Card>
      ) : isLoading ? (
        <LogTableSkeleton rows={pageSize > 20 ? 10 : 8} />
      ) : (
        <div
          className={`transition-opacity duration-150 ${isFetching ? "pointer-events-none opacity-50" : ""}`}
        >
          <div className="overflow-x-auto">
            <TooltipProvider delayDuration={300}>
              <Table>
                <TableHeader>
                  {table.getHeaderGroups().map((headerGroup) => (
                    <TableRow key={headerGroup.id}>
                      {headerGroup.headers.map((header) => (
                        <TableHead
                          key={header.id}
                          className={
                            (header.column.columnDef.meta as { className?: string })?.className
                          }
                        >
                          {header.isPlaceholder
                            ? null
                            : flexRender(header.column.columnDef.header, header.getContext())}
                        </TableHead>
                      ))}
                    </TableRow>
                  ))}
                </TableHeader>
                <TableBody>
                  {table.getRowModel().rows.map((row) => {
                    if (row.getIsGrouped()) {
                      return (
                        <TableRow key={row.id} className="bg-muted/30">
                          <TableCell colSpan={columns.length}>
                            <button
                              className="flex items-center gap-1.5 text-sm font-medium"
                              onClick={row.getToggleExpandedHandler()}
                            >
                              <ChevronRight
                                className={`h-4 w-4 transition-transform ${row.getIsExpanded() ? "rotate-90" : ""}`}
                              />
                              {String(row.groupingValue)}
                              <Badge variant="secondary" className="text-xs">
                                {row.subRows.length}
                              </Badge>
                            </button>
                          </TableCell>
                        </TableRow>
                      )
                    }
                    const log = row.original
                    return (
                      <tr
                        key={row.id}
                        className={`hover:bg-muted/50 cursor-pointer border-b ${
                          log.error ? "border-l-destructive bg-destructive/5 border-l-2" : ""
                        }`}
                        onClick={() => setDetailId(log.id)}
                      >
                        {row.getVisibleCells().map((cell) => (
                          <TableCell
                            key={cell.id}
                            className={
                              (cell.column.columnDef.meta as { className?: string })?.className
                            }
                          >
                            {flexRender(cell.column.columnDef.cell, cell.getContext())}
                          </TableCell>
                        ))}
                      </tr>
                    )
                  })}
                  {logs.length === 0 && !isLoading && (
                    <TableRow>
                      <TableCell colSpan={columns.length} className="py-12 text-center">
                        <div className="flex flex-col items-center gap-2">
                          <FileText className="text-muted-foreground/30 h-10 w-10" />
                          <p className="text-muted-foreground">
                            {hasFilters ? "No logs match your filters" : "No logs yet"}
                          </p>
                          {hasFilters && (
                            <Button
                              variant="outline"
                              size="sm"
                              className="mt-1"
                              onClick={() => {
                                navigate(pathname, { replace: true })
                                setKeywordInput("")
                              }}
                            >
                              Clear all filters
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </TooltipProvider>
          </div>
        </div>
      )}

      {/* Log Detail Side Panel */}
      <Sheet open={detailId !== null} onOpenChange={(open) => !open && setDetailId(null)}>
        <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-2xl">
          <SheetHeader>
            <div className="flex items-center justify-between pr-8">
              <SheetTitle className="flex items-center gap-2">
                Log #{detailId}
                {detail && (
                  <Badge variant={detail.error ? "destructive" : "default"} className="text-xs">
                    {detail.error ? "Error" : "OK"}
                  </Badge>
                )}
              </SheetTitle>
              <div className="flex items-center gap-1">
                <Button
                  variant="outline"
                  size="icon"
                  className="h-7 w-7"
                  disabled={
                    !detailId || visibleRows.findIndex((r) => r.original.id === detailId) <= 0
                  }
                  onClick={() => {
                    const idx = visibleRows.findIndex((r) => r.original.id === detailId)
                    if (idx > 0) setDetailId(visibleRows[idx - 1].original.id)
                  }}
                >
                  <ChevronUp className="h-4 w-4" />
                </Button>
                <Button
                  variant="outline"
                  size="icon"
                  className="h-7 w-7"
                  disabled={
                    !detailId ||
                    visibleRows.findIndex((r) => r.original.id === detailId) >=
                      visibleRows.length - 1
                  }
                  onClick={() => {
                    const idx = visibleRows.findIndex((r) => r.original.id === detailId)
                    if (idx >= 0 && idx < visibleRows.length - 1)
                      setDetailId(visibleRows[idx + 1].original.id)
                  }}
                >
                  <ChevronDown className="h-4 w-4" />
                </Button>
              </div>
            </div>
          </SheetHeader>
          {detail ? (
            <DetailPanel detail={detail} />
          ) : (
            <div className="flex flex-col gap-4 px-4 py-4">
              <div className="flex flex-col gap-2">
                <Skeleton className="h-5 w-40" />
                <Skeleton className="h-4 w-60" />
              </div>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                {DETAIL_SKELETON_ITEMS.map((_, i) => (
                  <div key={`detail-sk-${i.toString()}`} className="flex flex-col gap-1">
                    <Skeleton className="h-3 w-16" />
                    <Skeleton className="h-5 w-24" />
                  </div>
                ))}
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  )
}

function CopyableField({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <div className="group min-w-0">
      <p className="text-muted-foreground text-xs">{label}</p>
      <div className="flex items-center gap-1">
        <p className="truncate font-medium">{value}</p>
        <button
          className="shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
          onClick={() => {
            navigator.clipboard.writeText(value)
            setCopied(true)
            toast.success("Copied!")
            setTimeout(() => setCopied(false), 2000)
          }}
        >
          {copied ? (
            <Check className="h-3 w-3" />
          ) : (
            <Copy className="text-muted-foreground h-3 w-3" />
          )}
        </button>
      </div>
    </div>
  )
}

function DetailPanel({ detail }: { detail: LogDetail }) {
  const [replayResult, setReplayResult] = useState<string | null>(null)
  const [replaying, setReplaying] = useState(false)
  const [activeTab, setActiveTab] = useState("overview")

  const isTruncated =
    /\[truncated,?\s*\d+\s*chars\s*total\]/.test(detail.requestContent) ||
    /\[\d+\s*messages?\s*omitted/.test(detail.requestContent) ||
    /\[image data omitted\]/.test(detail.requestContent)

  const handleReplay = async () => {
    setReplaying(true)
    try {
      const resp = await replayLog(detail.id)
      const contentType = resp.headers.get("content-type") ?? ""

      if (contentType.includes("text/event-stream")) {
        // Streaming response: read and concatenate text deltas
        const reader = resp.body?.getReader()
        if (!reader) throw new Error("No response body")
        const decoder = new TextDecoder()
        let text = ""
        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          const chunk = decoder.decode(value, { stream: true })
          for (const line of chunk.split("\n")) {
            if (line.startsWith("data: ") && line !== "data: [DONE]") {
              try {
                const obj = JSON.parse(line.slice(6))
                const delta = obj.choices?.[0]?.delta?.content
                if (delta) text += delta
              } catch {
                /* skip */
              }
            }
          }
        }
        setReplayResult(text || "[Empty response]")
      } else {
        const data = (await resp.json()) as {
          success: boolean
          data?: { response: unknown; truncated: boolean }
          error?: string
        }
        if (!data.success) throw new Error(data.error ?? "Replay failed")
        setReplayResult(JSON.stringify(data.data?.response, null, 2))
      }
      setActiveTab("replay")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Replay failed")
    } finally {
      setReplaying(false)
    }
  }

  return (
    <Tabs value={activeTab} onValueChange={setActiveTab} className="min-w-0 px-4">
      <TabsList className="w-full">
        <TabsTrigger value="overview" className="flex-1">
          Overview
        </TabsTrigger>
        <TabsTrigger value="messages" className="flex-1">
          Messages
        </TabsTrigger>
        {detail.attempts && detail.attempts.length > 0 && (
          <TabsTrigger value="retry" className="flex-1">
            Retry ({detail.attempts.length})
          </TabsTrigger>
        )}
        {replayResult !== null && (
          <TabsTrigger value="replay" className="flex-1">
            Replay
          </TabsTrigger>
        )}
      </TabsList>

      {/* Overview Tab */}
      <TabsContent value="overview" className="mt-4">
        <div className="flex flex-col gap-4 text-sm">
          {/* Model Flow */}
          <div className="flex flex-col gap-1">
            <div className="flex flex-wrap items-center gap-2 text-sm">
              <ModelFlowNode modelId={detail.requestModelName} channelId={detail.channelId} />
              <span className="text-muted-foreground">via</span>
              {detail.channelId ? (
                <Link
                  to={`/channels?highlight=${detail.channelId}`}
                  className="font-medium hover:underline"
                >
                  {detail.channelName || "—"}
                </Link>
              ) : (
                <span className="font-medium">{detail.channelName || "—"}</span>
              )}
              {detail.actualModelName && detail.actualModelName !== detail.requestModelName && (
                <>
                  <span className="text-muted-foreground">&rarr;</span>
                  <ModelFlowNode modelId={detail.actualModelName} channelId={detail.channelId} />
                </>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3">
            <CopyableField
              label="Time"
              value={new Date(detail.time * 1000).toLocaleString(undefined, { hour12: false })}
            />
            <CopyableField
              label="Channel"
              value={`${detail.channelName || "—"} (ID: ${detail.channelId})`}
            />
            <CopyableField label="Input Tokens" value={detail.inputTokens.toLocaleString()} />
            <CopyableField label="Output Tokens" value={detail.outputTokens.toLocaleString()} />
            <CopyableField
              label="Total Tokens"
              value={(detail.inputTokens + detail.outputTokens).toLocaleString()}
            />
            <CopyableField label="Cost" value={formatCost(detail.cost)} />
            <CopyableField
              label="TTFT"
              value={detail.ftut > 0 ? formatDuration(detail.ftut) : "—"}
            />
            <CopyableField label="Total Latency" value={formatDuration(detail.useTime)} />
            <CopyableField
              label="Output Speed"
              value={
                detail.outputTokens > 0 && detail.useTime > 0
                  ? `${(detail.outputTokens / (detail.useTime / 1000)).toFixed(1)} tok/s`
                  : "—"
              }
            />
            {detail.totalAttempts > 1 && (
              <CopyableField label="Attempts" value={`${detail.totalAttempts}`} />
            )}
          </div>

          {detail.error && (
            <div className="bg-destructive/10 rounded-md p-3">
              <div className="flex items-center justify-between">
                <p className="text-destructive mb-1 text-xs font-medium">Error</p>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 px-1"
                  onClick={() => {
                    navigator.clipboard.writeText(detail.error)
                    toast.success("Copied!")
                  }}
                >
                  <Copy className="h-3 w-3" />
                </Button>
              </div>
              <pre className="text-xs break-all whitespace-pre-wrap">{detail.error}</pre>
            </div>
          )}

          {/* Replay */}
          <div className="flex flex-col gap-2">
            {isTruncated && (
              <p className="text-muted-foreground text-xs italic">
                Request content was truncated during storage. Replay may produce different results.
              </p>
            )}
            <Button
              variant="outline"
              size="sm"
              className="w-fit gap-1.5"
              onClick={handleReplay}
              disabled={replaying}
            >
              {replaying ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Play className="h-3.5 w-3.5" />
              )}
              Replay
            </Button>
          </div>
        </div>
      </TabsContent>

      {/* Messages Tab (merged Request + Response) */}
      <TabsContent value="messages" className="mt-4">
        <div className="flex flex-col gap-4">
          <CollapsibleCodeBlock
            label="Request (Original)"
            content={detail.requestContent}
            defaultOpen
          />
          {detail.upstreamContent && (
            <CollapsibleCodeBlock
              label="Request (Upstream)"
              content={detail.upstreamContent}
              defaultOpen={false}
            />
          )}
          <CollapsibleCodeBlock label="Response" content={detail.responseContent} defaultOpen />
        </div>
      </TabsContent>

      {/* Retry Timeline Tab */}
      {detail.attempts && detail.attempts.length > 0 && (
        <TabsContent value="retry" className="mt-4">
          <div className="border-border relative flex flex-col gap-3 border-l-2 pl-4">
            {detail.attempts.map((attempt) => (
              <div
                key={`${attempt.channelId}-${attempt.modelName}-${attempt.attemptNum}`}
                className="relative"
              >
                <div
                  className={`border-background absolute top-1 -left-[calc(0.5rem+1px)] h-3 w-3 rounded-full border-2 ${
                    attempt.status === "success"
                      ? "bg-green-500"
                      : attempt.status === "circuit_break" || attempt.status === "skipped"
                        ? "bg-yellow-500"
                        : "bg-destructive"
                  }`}
                />
                <div className="ml-2 rounded-md border p-2">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs">#{attempt.attemptNum}</span>
                    {attempt.sticky && (
                      <Badge variant="outline" className="text-xs">
                        Sticky
                      </Badge>
                    )}
                    <Badge
                      variant={
                        attempt.status === "success"
                          ? "default"
                          : attempt.status === "circuit_break" || attempt.status === "skipped"
                            ? "secondary"
                            : "destructive"
                      }
                      className="text-xs"
                    >
                      {attempt.status === "success"
                        ? "OK"
                        : attempt.status === "circuit_break"
                          ? "CIRCUIT BREAK"
                          : attempt.status === "skipped"
                            ? "SKIPPED"
                            : "FAIL"}
                    </Badge>
                    <span className="text-muted-foreground text-xs">
                      {formatDuration(attempt.duration)}
                    </span>
                  </div>
                  <p className="mt-1 text-xs">
                    <span className="text-muted-foreground">Channel:</span> {attempt.channelName}{" "}
                    <span className="text-muted-foreground">Model:</span>{" "}
                    <ModelBadge modelId={attempt.modelName} />
                  </p>
                  {attempt.msg && (
                    <p className="text-destructive mt-1 text-xs break-all">{attempt.msg}</p>
                  )}
                </div>
              </div>
            ))}
          </div>
        </TabsContent>
      )}

      {/* Replay Result Tab */}
      {replayResult !== null && (
        <TabsContent value="replay" className="mt-4">
          <CodeBlock label="Replay Result" content={replayResult} />
        </TabsContent>
      )}
    </Tabs>
  )
}

function ModelFlowNode({ modelId, channelId }: { modelId: string; channelId: number }) {
  const meta = useModelMeta(modelId)
  const displayName = meta?.name ?? modelId
  const showActualId = displayName !== modelId

  return (
    <div className="flex flex-col">
      {channelId ? (
        <Link to={`/channels?highlight=${channelId}`} className="hover:underline">
          <ModelBadge modelId={modelId} />
        </Link>
      ) : (
        <ModelBadge modelId={modelId} />
      )}
      {showActualId && (
        <button
          className="text-muted-foreground hover:text-foreground max-w-[200px] truncate text-left font-mono text-xs transition-colors"
          onClick={() => {
            navigator.clipboard.writeText(modelId)
            toast.success("Copied!")
          }}
          title={modelId}
        >
          {modelId}
        </button>
      )}
    </div>
  )
}

function CollapsibleCodeBlock({
  label,
  content,
  defaultOpen = true,
}: {
  label: string
  content: string
  defaultOpen?: boolean
}) {
  const [open, setOpen] = useState(defaultOpen)

  return (
    <div className="rounded-md border">
      <button
        type="button"
        className="bg-muted/50 flex w-full items-center justify-between px-3 py-2"
        onClick={() => setOpen(!open)}
      >
        <span className="text-xs font-medium">{label}</span>
        <ChevronDown
          className={`text-muted-foreground h-4 w-4 transition-transform ${open ? "" : "-rotate-90"}`}
        />
      </button>
      {open && (
        <div className="p-3 pt-2">
          <CodeBlock label="" content={content} />
        </div>
      )}
    </div>
  )
}

function HighlightedText({ text, search }: { text: string; search: string }) {
  if (!search) return <>{text}</>
  const parts: React.ReactNode[] = []
  const lower = text.toLowerCase()
  const needle = search.toLowerCase()
  let last = 0
  let idx = lower.indexOf(needle, last)
  while (idx !== -1) {
    if (idx > last) parts.push(text.slice(last, idx))
    parts.push(
      <mark key={idx} className="rounded-sm bg-yellow-300/80 px-0.5 dark:bg-yellow-500/40">
        {text.slice(idx, idx + needle.length)}
      </mark>,
    )
    last = idx + needle.length
    idx = lower.indexOf(needle, last)
  }
  if (last < text.length) parts.push(text.slice(last))
  return <>{parts}</>
}

function CodeBlock({ label, content }: { label: string; content: string }) {
  const [copied, setCopied] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")
  const { resolvedTheme } = useTheme()

  const displayContent = content?.trim() || ""

  const parsed = useMemo(() => {
    if (!displayContent) return { isJson: false, data: null, truncated: false }
    try {
      return { isJson: true, data: JSON.parse(displayContent), truncated: false }
    } catch {
      const repaired = repairTruncatedJson(displayContent)
      if (repaired) return { isJson: true, ...repaired }
      return { isJson: false, data: displayContent, truncated: false }
    }
  }, [displayContent])

  const plainText = useMemo(() => {
    if (!displayContent) return ""
    return parsed.isJson ? JSON.stringify(parsed.data, null, 2) : displayContent
  }, [displayContent, parsed])

  const matchCount = useMemo(() => {
    return countMatches(plainText, searchTerm)
  }, [plainText, searchTerm])

  if (!displayContent) {
    return (
      <div className="flex flex-col gap-2">
        <p className="text-muted-foreground text-sm">No content available.</p>
      </div>
    )
  }

  const handleCopy = () => {
    navigator.clipboard.writeText(plainText)
    setCopied(true)
    toast.success("Copied!")
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="flex min-w-0 flex-col gap-2">
      <div className="flex items-center justify-between">
        <p className="text-muted-foreground text-xs font-medium">{label}</p>
        <Button variant="ghost" size="sm" className="h-7 gap-1" onClick={handleCopy}>
          {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
          <span className="text-xs">{copied ? "Copied" : "Copy"}</span>
        </Button>
      </div>
      <div className="relative">
        <Search className="text-muted-foreground absolute top-2 left-2.5 h-3.5 w-3.5" />
        <Input
          placeholder="Search in content..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="h-8 pr-16 pl-8 text-xs"
        />
        {searchTerm && (
          <div className="absolute top-1.5 right-2 flex items-center gap-1">
            <span className="text-muted-foreground text-xs">
              {matchCount} {matchCount === 1 ? "match" : "matches"}
            </span>
            <button onClick={() => setSearchTerm("")}>
              <X className="text-muted-foreground h-3 w-3" />
            </button>
          </div>
        )}
      </div>
      {searchTerm ? (
        <div className="bg-muted/30 max-h-[50vh] min-w-0 overflow-auto rounded-md border p-3">
          <pre
            className="text-xs break-words whitespace-pre-wrap"
            style={{
              fontFamily: "ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace",
            }}
          >
            <HighlightedText text={plainText} search={searchTerm} />
          </pre>
        </div>
      ) : parsed.isJson ? (
        <div className="flex flex-col gap-2">
          {parsed.truncated && (
            <p className="text-muted-foreground text-xs italic">
              Content was truncated and partially recovered.
            </p>
          )}
          <div className="bg-muted/30 max-h-[50vh] min-w-0 overflow-auto rounded-md border p-3">
            <Suspense
              fallback={
                <div className="flex flex-col gap-2 p-3">
                  <Skeleton className="h-4 w-3/4" />
                  <Skeleton className="h-4 w-1/2" />
                  <Skeleton className="h-4 w-2/3" />
                </div>
              }
            >
              <LazyJsonView
                isDark={resolvedTheme === "dark"}
                value={parsed.data}
                style={{
                  fontSize: "12px",
                  backgroundColor: "transparent",
                  fontFamily: "ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace",
                }}
                displayDataTypes={false}
                displayObjectSize={false}
                collapsed={2}
              />
            </Suspense>
          </div>
        </div>
      ) : (
        <div className="bg-muted/30 prose prose-sm dark:prose-invert max-h-[50vh] max-w-none overflow-auto rounded-md border p-3 break-words">
          <Suspense
            fallback={
              <div className="flex flex-col gap-2 p-3">
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-4/5" />
              </div>
            }
          >
            <LazyMarkdown>{displayContent}</LazyMarkdown>
          </Suspense>
        </div>
      )}
    </div>
  )
}

function LogTableSkeleton({ rows = 8 }: { rows?: number }) {
  const skeletonRows = rows > 8 ? SKELETON_ROWS_10 : SKELETON_ROWS_8
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Time</TableHead>
          <TableHead>Model</TableHead>
          <TableHead>Channel</TableHead>
          <TableHead className="text-right">Input</TableHead>
          <TableHead className="text-right">Output</TableHead>
          <TableHead className="text-right">TTFT</TableHead>
          <TableHead className="text-right">Latency</TableHead>
          <TableHead className="text-right">Cost</TableHead>
          <TableHead>Status</TableHead>
          <TableHead className="w-10" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {skeletonRows.map((_, i) => (
          <TableRow key={`log-sk-${i.toString()}`}>
            <TableCell>
              <Skeleton className="h-4 w-24" />
            </TableCell>
            <TableCell>
              <Skeleton className="h-5 w-28 rounded-full" />
            </TableCell>
            <TableCell>
              <Skeleton className="h-4 w-20" />
            </TableCell>
            <TableCell className="text-right">
              <Skeleton className="ml-auto h-4 w-12" />
            </TableCell>
            <TableCell className="text-right">
              <Skeleton className="ml-auto h-4 w-12" />
            </TableCell>
            <TableCell className="text-right">
              <Skeleton className="ml-auto h-4 w-14" />
            </TableCell>
            <TableCell className="text-right">
              <Skeleton className="ml-auto h-4 w-14" />
            </TableCell>
            <TableCell className="text-right">
              <Skeleton className="ml-auto h-4 w-12" />
            </TableCell>
            <TableCell>
              <Skeleton className="h-5 w-10 rounded-full" />
            </TableCell>
            <TableCell>
              <Skeleton className="h-8 w-8 rounded-md" />
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

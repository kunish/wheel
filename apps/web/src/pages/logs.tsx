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
  Bot,
  Braces,
  Brain,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Code2,
  Copy,
  FileText,
  Image,
  Loader2,
  MessageSquare,
  Play,
  RefreshCw,
  Search,
  Settings2,
  User,
  Wrench,
  X,
} from "lucide-react"
import { useTheme } from "next-themes"
import * as React from "react"
import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
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
  Promise.all([
    import("react-markdown"),
    import("remark-gfm"),
    import("rehype-highlight"),
    import("highlight.js/lib/common"),
    import("mermaid"),
  ]).then(([md, gfm, rehypeHL, _hljs, mermaidMod]) => {
    const ReactMarkdown = md.default
    const remarkGfm = gfm.default
    const rehypeHighlight = rehypeHL.default
    const mermaid = mermaidMod.default
    mermaid.initialize({ startOnLoad: false, theme: "default" })

    let mermaidCounter = 0

    function MermaidBlock({ children }: { children: string }) {
      const containerRef = React.useRef<HTMLDivElement>(null)
      const idRef = React.useRef(`mermaid-${++mermaidCounter}`)

      React.useEffect(() => {
        const el = containerRef.current
        if (!el) return
        const id = idRef.current
        const isDark = document.documentElement.classList.contains("dark")
        mermaid.initialize({ startOnLoad: false, theme: isDark ? "dark" : "default" })
        mermaid
          .render(id, children.trim())
          .then(({ svg }) => {
            const parser = new DOMParser()
            const doc = parser.parseFromString(svg, "image/svg+xml")
            const svgEl = doc.documentElement
            el.replaceChildren(svgEl)
          })
          .catch(() => {
            el.textContent = children
          })
      }, [children])

      return <div ref={containerRef} className="my-2 flex justify-center" />
    }

    function LazyMarkdownInner({ children }: { children: string }) {
      return (
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          rehypePlugins={[rehypeHighlight]}
          components={{
            code({ className, children: codeChildren, ...props }) {
              const match = /language-mermaid/.test(className || "")
              const text = String(codeChildren).replace(/\n$/, "")
              if (match) {
                return <MermaidBlock>{text}</MermaidBlock>
              }
              return (
                <code className={className} {...props}>
                  {codeChildren}
                </code>
              )
            },
            pre({ children: preChildren }) {
              const child = React.Children.only(preChildren) as React.ReactElement<{
                className?: string
              }>
              if (child?.props?.className?.includes("language-mermaid")) {
                return <>{preChildren}</>
              }
              return <pre>{preChildren}</pre>
            },
          }}
        >
          {children}
        </ReactMarkdown>
      )
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
  const { t } = useTranslation("logs")
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
  const detailIdRef = useRef(detailId)
  detailIdRef.current = detailId
  const [detailTab, setDetailTab] = useState("overview")

  // Track which streamId the detail panel is viewing (null = viewing a DB log)
  const [detailStreamId, setDetailStreamId] = useState<string | null>(null)
  const detailStreamIdRef = useRef(detailStreamId)
  detailStreamIdRef.current = detailStreamId

  // Streaming overlay: real-time content from log-streaming WS events
  const [streamingOverlay, setStreamingOverlay] = useState<{
    thinkingContent: string
    responseContent: string
  } | null>(null)
  // Clear streaming overlay when switching detail panels
  const prevDetailIdRef = useRef(detailId)
  if (prevDetailIdRef.current !== detailId) {
    prevDetailIdRef.current = detailId
    setStreamingOverlay(null)
  }

  const [pendingStreams, setPendingStreams] = useState<Map<string, LogEntry>>(new Map())
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
    enabled: detailId !== null && detailStreamId === null,
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

  // Listen for log-stream-start: create a pending entry for the streaming request
  useWsEvent("log-stream-start", (data) => {
    if (!data?.streamId) return
    const f = filtersRef.current
    if (!f.isFirstPage || f.hasFilters) return
    setPendingStreams((prev) => {
      const next = new Map(prev)
      next.set(data.streamId, {
        id: -Date.now(),
        time: data.time ?? Math.floor(Date.now() / 1000),
        requestModelName: data.requestModelName ?? "",
        actualModelName: data.actualModelName ?? "",
        channelId: data.channelId ?? 0,
        channelName: data.channelName ?? "",
        inputTokens: data.estimatedInputTokens ?? 0,
        outputTokens: 0,
        ftut: 0,
        useTime: 0,
        cost: 0,
        error: "",
        totalAttempts: 0,
        _streaming: true,
        _streamId: data.streamId,
        _startedAt: Date.now(),
        _inputPrice: data.inputPrice ?? 0,
        _outputPrice: data.outputPrice ?? 0,
        _estimatedInputTokens: data.estimatedInputTokens ?? 0,
      })
      return next
    })
  })

  // Listen for log-streaming WS events: update pending entry useTime + streaming overlay for detail panel
  useWsEvent("log-streaming", (data) => {
    if (!data?.streamId) return

    // Update pending entry useTime, estimated tokens, and cost
    setPendingStreams((prev) => {
      const entry = prev.get(data.streamId)
      if (!entry) return prev
      const next = new Map(prev)
      const contentLen = (data.responseLength ?? 0) + (data.thinkingLength ?? 0)
      const estimatedOutputTokens = Math.floor(contentLen / 3)
      const inputPrice = (entry as any)._inputPrice ?? 0
      const outputPrice = (entry as any)._outputPrice ?? 0
      const estimatedInputTokens = (entry as any)._estimatedInputTokens ?? 0
      const estimatedCost =
        (estimatedInputTokens * inputPrice + estimatedOutputTokens * outputPrice) / 1_000_000
      next.set(data.streamId, {
        ...entry,
        useTime: Date.now() - (entry._startedAt ?? Date.now()),
        outputTokens: estimatedOutputTokens,
        cost: estimatedCost,
      })
      return next
    })

    // Streaming overlay for detail panel (when viewing a pending stream)
    const currentStreamId = detailStreamIdRef.current
    if (currentStreamId === data.streamId) {
      setStreamingOverlay({
        thinkingContent: data.thinkingContent ?? "",
        responseContent: data.responseContent ?? "",
      })
    }
  })

  useWsEvent("log-created", (data) => {
    if (!data?.log) return

    // Remove corresponding pending stream entry
    if (data.streamId) {
      setPendingStreams((prev) => {
        if (!prev.has(data.streamId)) return prev
        const next = new Map(prev)
        next.delete(data.streamId)
        return next
      })

      // If detail panel is viewing this stream, switch to the real log ID
      if (detailStreamIdRef.current === data.streamId) {
        setDetailStreamId(null)
        setDetailId(data.log.id)
        setStreamingOverlay(null)
      }
    }

    // Clear streaming overlay & refresh detail panel if viewing this log
    const currentDetailId = detailIdRef.current
    if (currentDetailId !== null && data.log.id === currentDetailId) {
      setStreamingOverlay(null)
      queryClient.invalidateQueries({ queryKey: ["log-detail", currentDetailId] })
    }

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

  // Listen for log-stream-end: remove pending entry (failed/exhausted stream)
  useWsEvent("log-stream-end", (data) => {
    if (!data?.streamId) return
    setPendingStreams((prev) => {
      if (!prev.has(data.streamId)) return prev
      const next = new Map(prev)
      next.delete(data.streamId)
      return next
    })
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

  const logs = useMemo(() => {
    const dbLogs = (data?.data?.logs ?? []) as LogEntry[]
    if (pendingStreams.size === 0) return dbLogs
    const pending = Array.from(pendingStreams.values()).sort((a, b) => b.time - a.time)
    return [...pending, ...dbLogs]
  }, [data, pendingStreams])
  const total = data?.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)
  const detail = (detailData?.data ?? null) as LogDetail | null

  // Real-time elapsed time update for pending streams (1s interval)
  useEffect(() => {
    if (pendingStreams.size === 0) return
    const interval = setInterval(() => {
      setPendingStreams((prev) => {
        const next = new Map(prev)
        for (const [key, entry] of next) {
          next.set(key, {
            ...entry,
            useTime: Date.now() - (entry._startedAt ?? Date.now()),
          })
        }
        return next
      })
    }, 1000)
    return () => clearInterval(interval)
  }, [pendingStreams.size > 0]) // eslint-disable-line react-hooks/exhaustive-deps

  const columns = useMemo(() => createLogColumns(setDetailId, t), [t])

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
          <SelectItem value="50">50</SelectItem>
          <SelectItem value="100">100</SelectItem>
          <SelectItem value="200">200</SelectItem>
          <SelectItem value="500">500</SelectItem>
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
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Sticky header: Title + Filters + Pagination */}
      <div className="bg-background shrink-0 space-y-4 pb-4">
        {/* Header: Title + Total + Pagination */}
        <div className="flex items-center justify-between">
          <div className="flex items-baseline gap-3">
            <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
            <span className="text-muted-foreground text-sm">
              {t("totalCount", { count: total })}
            </span>
            {pendingCount > 0 && (
              <Button
                variant="outline"
                size="xs"
                className="animate-pulse gap-1"
                onClick={handleShowNew}
              >
                <ArrowUp className="h-3 w-3" />
                {t("newLogs", { count: pendingCount })}
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
                placeholder={t("searchPlaceholder")}
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
                <SelectValue placeholder={t("filter.allChannels")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("filter.allChannels")}</SelectItem>
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
                <SelectItem value="all">{t("filter.all")}</SelectItem>
                <SelectItem value="success">{t("filter.success")}</SelectItem>
                <SelectItem value="error">{t("filter.error")}</SelectItem>
              </SelectContent>
            </Select>
            <TimeRangePicker
              from={startTime}
              to={endTime}
              onChange={(from, to) =>
                updateFilter({ from: from ?? undefined, to: to ?? undefined })
              }
            />
          </div>

          {/* Active filter chips */}
          {hasFilters && (
            <div className="flex flex-wrap gap-1.5">
              {keyword && (
                <Badge variant="secondary" className="gap-1 pr-1">
                  {t("chips.search", { value: keyword })}
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
                  {t("chips.model", { value: model })}
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
                  {t("chips.channel", {
                    value: channels.find((c) => c.id === channelId)?.name ?? channelId,
                  })}
                  <button onClick={() => updateFilter({ channel: undefined })}>
                    <X className="h-3 w-3" />
                  </button>
                </Badge>
              )}
              {status !== "all" && (
                <Badge variant="secondary" className="gap-1 pr-1">
                  {t("chips.status", { value: status })}
                  <button onClick={() => updateFilter({ status: "all" })}>
                    <X className="h-3 w-3" />
                  </button>
                </Badge>
              )}
              {(startTime || endTime) && (
                <Badge variant="secondary" className="gap-1 pr-1">
                  {t("chips.time", { value: formatRangeSummary(startTime, endTime, t) })}
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
                {t("chips.clearAll")}
              </Button>
            </div>
          )}
        </div>
      </div>

      {/* Scrollable content area */}
      {isError ? (
        <Card className="flex flex-col items-center justify-center gap-3 py-12">
          <AlertCircle className="text-destructive h-8 w-8" />
          <p className="text-muted-foreground text-sm">{t("loadError")}</p>
          <Button variant="outline" size="sm" className="gap-1.5" onClick={() => refetch()}>
            <RefreshCw className="h-3.5 w-3.5" />
            {t("actions.retry", { ns: "common" })}
          </Button>
        </Card>
      ) : isLoading ? (
        <LogTableSkeleton rows={pageSize > 20 ? 10 : 8} />
      ) : (
        <div
          className={`min-h-0 flex-1 overflow-auto transition-opacity duration-150 ${isFetching ? "pointer-events-none opacity-50" : ""}`}
        >
          <TooltipProvider delayDuration={300}>
            <table className="w-full caption-bottom text-sm">
              <thead className="bg-muted sticky top-0 z-10 [&_tr]:border-b-2">
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
              </thead>
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
                        log._streaming
                          ? "bg-muted/20"
                          : log.error
                            ? "border-l-destructive bg-destructive/5 border-l-2"
                            : ""
                      }`}
                      onClick={() => {
                        if (log._streaming && log._streamId) {
                          setDetailStreamId(log._streamId)
                        } else {
                          setDetailStreamId(null)
                          setDetailId(log.id)
                        }
                      }}
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
                          {hasFilters ? t("empty.noMatch") : t("empty.noLogs")}
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
                            {t("empty.clearFilters")}
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </table>
          </TooltipProvider>
        </div>
      )}

      {/* Log Detail Side Panel */}
      <Sheet
        open={detailId !== null || detailStreamId !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDetailId(null)
            setDetailStreamId(null)
          }
        }}
      >
        <SheetContent side="right" className="w-full sm:max-w-2xl">
          <SheetHeader className="shrink-0 border-b">
            <div className="flex items-center justify-between pr-8">
              <SheetTitle className="flex items-center gap-2">
                {detailStreamId ? (
                  <>
                    {t("detail.title", { id: "..." })}
                    <Badge variant="outline" className="animate-pulse gap-1 text-xs">
                      <Loader2 className="h-2.5 w-2.5 animate-spin" />
                      {t("columns.streaming")}
                    </Badge>
                  </>
                ) : (
                  <>
                    {t("detail.title", { id: detailId })}
                    {detail && (
                      <Badge variant={detail.error ? "destructive" : "default"} className="text-xs">
                        {detail.error ? t("detail.error") : t("detail.ok")}
                      </Badge>
                    )}
                  </>
                )}
              </SheetTitle>
              <div className="flex items-center gap-1">
                <Button
                  variant="outline"
                  size="icon"
                  className="h-7 w-7"
                  disabled={(() => {
                    const idx = visibleRows.findIndex((r) => {
                      const log = r.original
                      if (detailStreamId) return log._streaming && log._streamId === detailStreamId
                      return log.id === detailId
                    })
                    return idx <= 0
                  })()}
                  onClick={() => {
                    const idx = visibleRows.findIndex((r) => {
                      const log = r.original
                      if (detailStreamId) return log._streaming && log._streamId === detailStreamId
                      return log.id === detailId
                    })
                    if (idx > 0) {
                      const prev = visibleRows[idx - 1].original
                      if (prev._streaming && prev._streamId) {
                        setDetailId(null)
                        setDetailStreamId(prev._streamId)
                      } else {
                        setDetailStreamId(null)
                        setDetailId(prev.id)
                      }
                    }
                  }}
                >
                  <ChevronUp className="h-4 w-4" />
                </Button>
                <Button
                  variant="outline"
                  size="icon"
                  className="h-7 w-7"
                  disabled={(() => {
                    const idx = visibleRows.findIndex((r) => {
                      const log = r.original
                      if (detailStreamId) return log._streaming && log._streamId === detailStreamId
                      return log.id === detailId
                    })
                    return idx < 0 || idx >= visibleRows.length - 1
                  })()}
                  onClick={() => {
                    const idx = visibleRows.findIndex((r) => {
                      const log = r.original
                      if (detailStreamId) return log._streaming && log._streamId === detailStreamId
                      return log.id === detailId
                    })
                    if (idx >= 0 && idx < visibleRows.length - 1) {
                      const next = visibleRows[idx + 1].original
                      if (next._streaming && next._streamId) {
                        setDetailId(null)
                        setDetailStreamId(next._streamId)
                      } else {
                        setDetailStreamId(null)
                        setDetailId(next.id)
                      }
                    }
                  }}
                >
                  <ChevronDown className="h-4 w-4" />
                </Button>
              </div>
            </div>
          </SheetHeader>
          {detailStreamId ? (
            (() => {
              const entry = pendingStreams.get(detailStreamId)
              if (!entry) return null
              const streamingDetail: LogDetail = {
                id: entry.id,
                time: entry.time,
                requestModelName: entry.requestModelName,
                actualModelName: entry.actualModelName,
                channelName: entry.channelName,
                channelId: entry.channelId,
                inputTokens: entry.inputTokens,
                outputTokens: entry.outputTokens,
                cost: 0,
                ftut: entry.ftut,
                useTime: entry.useTime,
                requestContent: "",
                upstreamContent: null,
                responseContent: "",
                error: entry.error,
                attempts: [],
                totalAttempts: entry.totalAttempts,
              }
              return (
                <DetailPanel
                  detail={streamingDetail}
                  activeTab={detailTab}
                  onTabChange={setDetailTab}
                  streamingOverlay={streamingOverlay}
                />
              )
            })()
          ) : detail ? (
            <DetailPanel
              detail={detail}
              activeTab={detailTab}
              onTabChange={setDetailTab}
              streamingOverlay={streamingOverlay}
            />
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
  const { t } = useTranslation("logs")
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
            toast.success(t("toast.copied"))
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

function DetailPanel({
  detail,
  activeTab,
  onTabChange,
  streamingOverlay,
}: {
  detail: LogDetail
  activeTab: string
  onTabChange: (tab: string) => void
  streamingOverlay: { thinkingContent: string; responseContent: string } | null
}) {
  const { t } = useTranslation("logs")
  const [replayResult, setReplayResult] = useState<string | null>(null)
  const [replaying, setReplaying] = useState(false)

  const setActiveTab = onTabChange

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
        setReplayResult(text || t("detail.emptyResponse"))
      } else {
        const data = (await resp.json()) as {
          success: boolean
          data?: { response: unknown; truncated: boolean }
          error?: string
        }
        if (!data.success) throw new Error(data.error ?? t("detail.replayFailed"))
        setReplayResult(JSON.stringify(data.data?.response, null, 2))
      }
      setActiveTab("replay")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("detail.replayFailed"))
    } finally {
      setReplaying(false)
    }
  }

  return (
    <Tabs
      value={activeTab}
      onValueChange={setActiveTab}
      className="flex min-h-0 min-w-0 flex-1 flex-col px-4"
    >
      <TabsList className="w-full shrink-0">
        <TabsTrigger value="overview" className="flex-1">
          {t("detail.overview")}
        </TabsTrigger>
        <TabsTrigger value="messages" className="flex-1">
          {t("detail.messages")}
        </TabsTrigger>
        {detail.attempts && detail.attempts.length > 0 && (
          <TabsTrigger value="retry" className="flex-1">
            {t("detail.retry", { count: detail.attempts.length })}
          </TabsTrigger>
        )}
        {replayResult !== null && (
          <TabsTrigger value="replay" className="flex-1">
            {t("detail.replay")}
          </TabsTrigger>
        )}
      </TabsList>

      {/* Overview Tab */}
      <TabsContent value="overview" className="mt-4 min-h-0 flex-1 overflow-y-auto pb-6">
        <div className="flex flex-col gap-4 text-sm">
          {/* Model Flow */}
          <div className="flex flex-col gap-1">
            <div className="flex flex-wrap items-center gap-2 text-sm">
              <ModelFlowNode modelId={detail.requestModelName} channelId={detail.channelId} />
              <span className="text-muted-foreground">{t("detail.via")}</span>
              {detail.channelId ? (
                <Link
                  to={`/model?highlight=${detail.channelId}`}
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
              label={t("detail.field.time")}
              value={new Date(detail.time * 1000).toLocaleString(undefined, { hour12: false })}
            />
            <CopyableField
              label={t("detail.field.channel")}
              value={`${detail.channelName || "—"} (ID: ${detail.channelId})`}
            />
            <CopyableField
              label={t("detail.field.inputTokens")}
              value={detail.inputTokens.toLocaleString()}
            />
            <CopyableField
              label={t("detail.field.outputTokens")}
              value={detail.outputTokens.toLocaleString()}
            />
            <CopyableField
              label={t("detail.field.totalTokens")}
              value={(detail.inputTokens + detail.outputTokens).toLocaleString()}
            />
            <CopyableField label={t("detail.field.cost")} value={formatCost(detail.cost)} />
            <CopyableField
              label={t("detail.field.ttft")}
              value={detail.ftut > 0 ? formatDuration(detail.ftut) : "—"}
            />
            <CopyableField
              label={t("detail.field.totalLatency")}
              value={formatDuration(detail.useTime)}
            />
            <CopyableField
              label={t("detail.field.outputSpeed")}
              value={
                detail.outputTokens > 0 && detail.useTime > 0
                  ? `${(detail.outputTokens / (detail.useTime / 1000)).toFixed(1)} tok/s`
                  : "—"
              }
            />
            {detail.totalAttempts > 1 && (
              <CopyableField label={t("detail.field.attempts")} value={`${detail.totalAttempts}`} />
            )}
          </div>

          {detail.error && (
            <div className="bg-destructive/10 rounded-md p-3">
              <div className="flex items-center justify-between">
                <p className="text-destructive mb-1 text-xs font-medium">{t("detail.error")}</p>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 px-1"
                  onClick={() => {
                    navigator.clipboard.writeText(detail.error)
                    toast.success(t("toast.copied"))
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
              <p className="text-muted-foreground text-xs italic">{t("detail.truncatedNotice")}</p>
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
              {t("detail.replayBtn")}
            </Button>
          </div>
        </div>
      </TabsContent>

      {/* Messages Tab (conversation flow + raw JSON toggle) */}
      <TabsContent value="messages" className="mt-4 min-h-0 flex-1 overflow-y-auto pb-6">
        <MessagesTabContent detail={detail} streamingOverlay={streamingOverlay} />
      </TabsContent>

      {/* Retry Timeline Tab */}
      {detail.attempts && detail.attempts.length > 0 && (
        <TabsContent value="retry" className="mt-4 min-h-0 flex-1 overflow-y-auto pb-6">
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
                        {t("retryTimeline.sticky")}
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
                        ? t("retryTimeline.ok")
                        : attempt.status === "circuit_break"
                          ? t("retryTimeline.circuitBreak")
                          : attempt.status === "skipped"
                            ? t("retryTimeline.skipped")
                            : t("retryTimeline.fail")}
                    </Badge>
                    <span className="text-muted-foreground text-xs">
                      {formatDuration(attempt.duration)}
                    </span>
                  </div>
                  <p className="mt-1 text-xs">
                    <span className="text-muted-foreground">{t("retryTimeline.channel")}</span>{" "}
                    {attempt.channelName}{" "}
                    <span className="text-muted-foreground">{t("retryTimeline.model")}</span>{" "}
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
        <TabsContent value="replay" className="mt-4 min-h-0 flex-1 overflow-y-auto pb-6">
          <CodeBlock label={t("detail.replayResult")} content={replayResult} />
        </TabsContent>
      )}
    </Tabs>
  )
}

function ModelFlowNode({ modelId, channelId }: { modelId: string; channelId: number }) {
  const { t } = useTranslation("logs")
  const meta = useModelMeta(modelId)
  const displayName = meta?.name ?? modelId
  const showActualId = displayName !== modelId

  return (
    <div className="flex flex-col">
      {channelId ? (
        <Link to={`/model?highlight=${channelId}`} className="hover:underline">
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
            toast.success(t("toast.copied"))
          }}
          title={modelId}
        >
          {modelId}
        </button>
      )}
    </div>
  )
}

// --- Messages Tab: conversation flow view ---

interface ParsedMessage {
  role: string
  content: string | Array<{ type: string; text?: string; image_url?: { url: string } }> | null
  name?: string
  tool_call_id?: string
  tool_calls?: Array<{
    id: string
    type: string
    function: { name: string; arguments: string }
  }>
}

interface ParsedRequestParams {
  model?: string
  stream?: boolean
  temperature?: number
  max_tokens?: number
  max_completion_tokens?: number
  top_p?: number
  frequency_penalty?: number
  presence_penalty?: number
  response_format?: { type: string; [key: string]: unknown }
  seed?: number
  stop?: string | string[]
  n?: number
  user?: string
}

interface ParsedRequestTools {
  tools: Array<{
    type: string
    function: {
      name: string
      description?: string
      parameters?: unknown
    }
  }>
  tool_choice?: string | { type: string; function?: { name: string } }
}

interface ParsedResponseUsage {
  prompt_tokens?: number
  completion_tokens?: number
  total_tokens?: number
  prompt_tokens_details?: { cached_tokens?: number; audio_tokens?: number }
  completion_tokens_details?: {
    reasoning_tokens?: number
    audio_tokens?: number
    accepted_prediction_tokens?: number
    rejected_prediction_tokens?: number
  }
}

interface ParsedResponseChoice {
  assistantContent: string | null
  thinkingContent: string | null
  toolCalls: ParsedMessage["tool_calls"]
  finishReason: string | null
  index: number
}

interface ParsedResponse {
  choices: ParsedResponseChoice[]
  id: string | null
  model: string | null
  created: number | null
  systemFingerprint: string | null
  usage: ParsedResponseUsage | null
  raw: unknown
}

function parseMessages(content: string): ParsedMessage[] | null {
  try {
    const parsed = JSON.parse(content)
    // OpenAI request format: { messages: [...] } or { model, messages: [...] }
    if (parsed?.messages && Array.isArray(parsed.messages)) {
      return parsed.messages as ParsedMessage[]
    }
    // Direct array of messages
    if (Array.isArray(parsed) && parsed.length > 0 && parsed[0]?.role) {
      return parsed as ParsedMessage[]
    }
  } catch {
    /* not parseable */
  }
  return null
}

/**
 * Find the boundary between "previous context" and "current turn" messages.
 * Returns the index of the first "new" message (i.e., lastAssistantIdx + 1).
 * Returns 0 when there's no assistant message (first turn), meaning all messages are new.
 */
function findNewTurnBoundary(messages: ParsedMessage[]): number {
  let lastAssistantIdx = -1
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].role === "assistant") {
      lastAssistantIdx = i
      break
    }
  }
  return lastAssistantIdx === -1 ? 0 : lastAssistantIdx + 1
}

function parseRequestParams(content: string): ParsedRequestParams | null {
  try {
    const parsed = JSON.parse(content)
    if (!parsed || typeof parsed !== "object") return null
    const keys: (keyof ParsedRequestParams)[] = [
      "model",
      "stream",
      "temperature",
      "max_tokens",
      "max_completion_tokens",
      "top_p",
      "frequency_penalty",
      "presence_penalty",
      "response_format",
      "seed",
      "stop",
      "n",
      "user",
    ]
    const result: ParsedRequestParams = {}
    let hasAny = false
    for (const key of keys) {
      if (parsed[key] !== undefined && parsed[key] !== null) {
        ;(result as Record<string, unknown>)[key] = parsed[key]
        hasAny = true
      }
    }
    return hasAny ? result : null
  } catch {
    return null
  }
}

function parseRequestTools(content: string): ParsedRequestTools | null {
  try {
    const parsed = JSON.parse(content)
    if (!parsed?.tools || !Array.isArray(parsed.tools) || parsed.tools.length === 0) return null
    // Normalize tool formats: Anthropic tools have {name, input_schema} at top level,
    // OpenAI tools have {type, function: {name, parameters}} — unify to OpenAI shape.
    const normalized = parsed.tools.map((tool: Record<string, unknown>) => {
      if (tool.function && typeof tool.function === "object") {
        return tool
      }
      return {
        type: (tool.type as string) || "function",
        function: {
          name: tool.name as string,
          description: tool.description as string | undefined,
          parameters: tool.input_schema ?? tool.parameters,
        },
      }
    })
    return {
      tools: normalized as ParsedRequestTools["tools"],
      tool_choice: parsed.tool_choice,
    }
  } catch {
    return null
  }
}

function extractThinking(content: string): { thinking: string | null; rest: string } {
  const match = content.match(/^<\|thinking\|>([\s\S]*?)<\|\/thinking\|>([\s\S]*)$/)
  if (match) {
    return { thinking: match[1] || null, rest: match[2] }
  }
  return { thinking: null, rest: content }
}

function parseResponseContent(content: string): ParsedResponse | null {
  if (!content || content === "[streaming]") return null

  const { thinking, rest } = extractThinking(content)

  try {
    const parsed = JSON.parse(rest)
    const rawChoices = parsed?.choices
    if (Array.isArray(rawChoices) && rawChoices.length > 0) {
      const choices: ParsedResponseChoice[] = rawChoices.map(
        (
          choice: {
            message?: { content?: string; tool_calls?: ParsedMessage["tool_calls"] }
            finish_reason?: string
            index?: number
          },
          i: number,
        ) => ({
          assistantContent:
            typeof choice.message?.content === "string" ? choice.message.content : null,
          thinkingContent: i === 0 ? thinking : null,
          toolCalls: choice.message?.tool_calls ?? undefined,
          finishReason: choice.finish_reason ?? null,
          index: choice.index ?? i,
        }),
      )
      const usage = parsed.usage
        ? {
            prompt_tokens: parsed.usage.prompt_tokens,
            completion_tokens: parsed.usage.completion_tokens,
            total_tokens: parsed.usage.total_tokens,
            prompt_tokens_details: parsed.usage.prompt_tokens_details,
            completion_tokens_details: parsed.usage.completion_tokens_details,
          }
        : null
      return {
        choices,
        id: parsed.id ?? null,
        model: parsed.model ?? null,
        created: parsed.created ?? null,
        systemFingerprint: parsed.system_fingerprint ?? null,
        usage,
        raw: parsed,
      }
    }
  } catch {
    // Not valid JSON — likely plain text accumulated from a streaming response
    const text = rest || null
    return {
      choices: [
        {
          assistantContent: text,
          thinkingContent: thinking,
          toolCalls: undefined,
          finishReason: null,
          index: 0,
        },
      ],
      id: null,
      model: null,
      created: null,
      systemFingerprint: null,
      usage: null,
      raw: null,
    }
  }
  return null
}

function getMessageTextContent(content: ParsedMessage["content"]): {
  text: string
  hasImages: boolean
} {
  if (content === null || content === undefined) return { text: "", hasImages: false }
  if (typeof content === "string") return { text: content, hasImages: false }
  if (Array.isArray(content)) {
    const textParts = content.filter((p) => p.type === "text" && p.text).map((p) => p.text!)
    const hasImages = content.some((p) => p.type === "image_url")
    return { text: textParts.join("\n"), hasImages }
  }
  return { text: String(content), hasImages: false }
}

// ROLE_CONFIG keeps icon/style references at module level (no labels — those use t())
const ROLE_STYLE_CONFIG: Record<
  string,
  {
    icon: React.ComponentType<{ className?: string }>
    labelKey: string
    bgClass: string
    borderClass: string
    avatarClass: string
  }
> = {
  system: {
    icon: Settings2,
    labelKey: "roles.system",
    bgClass: "bg-nb-lavender/15 dark:bg-nb-lavender/10",
    borderClass: "border-l-nb-lavender",
    avatarClass: "bg-nb-lavender text-foreground",
  },
  user: {
    icon: User,
    labelKey: "roles.user",
    bgClass: "bg-nb-sky/15 dark:bg-nb-sky/10",
    borderClass: "border-l-nb-sky",
    avatarClass: "bg-nb-sky text-foreground",
  },
  assistant: {
    icon: Bot,
    labelKey: "roles.assistant",
    bgClass: "bg-nb-lime/15 dark:bg-nb-lime/10",
    borderClass: "border-l-nb-lime",
    avatarClass: "bg-nb-lime text-foreground",
  },
  tool: {
    icon: Wrench,
    labelKey: "roles.tool",
    bgClass: "bg-nb-orange/15 dark:bg-nb-orange/10",
    borderClass: "border-l-nb-orange",
    avatarClass: "bg-nb-orange text-foreground",
  },
  function: {
    icon: Code2,
    labelKey: "roles.function",
    bgClass: "bg-nb-pink/15 dark:bg-nb-pink/10",
    borderClass: "border-l-nb-pink",
    avatarClass: "bg-nb-pink text-foreground",
  },
}

const DEFAULT_ROLE_STYLE_CONFIG = {
  icon: MessageSquare,
  labelKey: "roles.unknown",
  bgClass: "bg-muted/30",
  borderClass: "border-l-muted-foreground",
  avatarClass: "bg-muted text-muted-foreground",
}

const TRUNCATION_PATTERNS = [
  /\[truncated,?\s*\d+\s*chars\s*total\]/,
  /\[\d+\s*messages?\s*omitted[^\]]*\]/,
  /\[image data omitted\]/,
  /\[image\]/gi,
]

/** Strip Unicode replacement characters (U+FFFD) that result from mid-byte truncation */
function stripReplacementChars(text: string): string {
  return text.replace(/\uFFFD+/g, "")
}

function detectTruncation(text: string): {
  isTruncated: boolean
  cleanText: string
  notice: string | null
} {
  let cleaned = text
  let notice: string | null = null
  let isTruncated = false

  for (const pattern of TRUNCATION_PATTERNS) {
    const match = cleaned.match(pattern)
    if (match) {
      isTruncated = true
      if (!notice) notice = match[0]
      cleaned = cleaned.replace(pattern, "")
    }
  }

  // Clean up broken Unicode from mid-byte truncation
  if (cleaned !== stripReplacementChars(cleaned)) {
    isTruncated = true
    cleaned = stripReplacementChars(cleaned)
  }

  cleaned = cleaned.trim()
  return { isTruncated, cleanText: cleaned, notice }
}

function MessageBubble({
  msg,
  index,
  isContext,
}: {
  msg: ParsedMessage
  index: number
  isContext?: boolean
}) {
  const { t } = useTranslation("logs")
  const config = ROLE_STYLE_CONFIG[msg.role] ?? DEFAULT_ROLE_STYLE_CONFIG
  const Icon = config.icon
  const { text, hasImages } = getMessageTextContent(msg.content)
  const { isTruncated, cleanText, notice } = detectTruncation(text)

  const displaySource = cleanText || text

  return (
    <div
      className={`rounded-md border-2 border-l-4 ${config.borderClass} ${config.bgClass}${isContext ? "opacity-60" : ""}`}
    >
      {/* Header */}
      <div className="flex items-center gap-2 px-3 py-2">
        <div
          className={`border-border flex h-6 w-6 shrink-0 items-center justify-center rounded-md border-2 ${config.avatarClass}`}
        >
          <Icon className="h-3.5 w-3.5" />
        </div>
        <span className="text-xs font-bold tracking-wide uppercase">{t(config.labelKey)}</span>
        {msg.name && (
          <span className="bg-background rounded border px-1.5 py-0.5 font-mono text-[10px]">
            {msg.name}
          </span>
        )}
        {msg.tool_call_id && (
          <span className="text-muted-foreground font-mono text-[10px]">
            id: {msg.tool_call_id}
          </span>
        )}
        {hasImages && (
          <span className="text-muted-foreground inline-flex items-center gap-1 text-[10px]">
            <Image className="h-3 w-3" /> {t("messagesTab.image")}
          </span>
        )}
        {isTruncated && (
          <Badge variant="ghost" className="gap-1 text-[10px]">
            <AlertCircle className="h-2.5 w-2.5" />
            {t("messagesTab.truncated")}
          </Badge>
        )}
        <span className="text-muted-foreground ml-auto font-mono text-[10px]">#{index}</span>
      </div>

      {/* Truncation notice */}
      {isTruncated && notice && !cleanText && (
        <div className="border-border/50 border-t px-3 py-2">
          <p className="text-muted-foreground text-xs italic">{notice}</p>
        </div>
      )}

      {/* Content */}
      {displaySource && (
        <div className="border-border/50 border-t px-3 py-2">
          <div className="prose prose-sm dark:prose-invert max-w-none break-words">
            <Suspense
              fallback={
                <pre className="font-[inherit] text-xs leading-relaxed whitespace-pre-wrap">
                  {displaySource}
                </pre>
              }
            >
              <LazyMarkdown>{displaySource}</LazyMarkdown>
            </Suspense>
          </div>
        </div>
      )}

      {/* Tool calls */}
      {msg.tool_calls && msg.tool_calls.length > 0 && (
        <div className="border-border/50 border-t px-3 py-2">
          <p className="text-muted-foreground mb-1.5 text-[10px] font-bold tracking-wider uppercase">
            {t("messagesTab.toolCalls")}
          </p>
          <div className="flex flex-col gap-1.5">
            {msg.tool_calls.map((tc) => (
              <ToolCallBlock key={tc.id} toolCall={tc} />
            ))}
          </div>
        </div>
      )}

      {/* Empty content indicator */}
      {!displaySource && !isTruncated && !msg.tool_calls?.length && (
        <div className="border-border/50 border-t px-3 py-2">
          <span className="text-muted-foreground text-xs italic">{t("messagesTab.noContent")}</span>
        </div>
      )}
    </div>
  )
}

function ToolCallBlock({
  toolCall,
}: {
  toolCall: NonNullable<ParsedMessage["tool_calls"]>[number]
}) {
  let args = toolCall.function.arguments
  try {
    args = JSON.stringify(JSON.parse(args), null, 2)
  } catch {
    /* keep original */
  }

  return (
    <div className="bg-background/60 rounded-md border p-2">
      <div className="flex items-center gap-1.5">
        <Code2 className="text-muted-foreground h-3 w-3 shrink-0" />
        <span className="font-mono text-xs font-bold">{toolCall.function.name}</span>
        <span className="text-muted-foreground font-mono text-[10px]">{toolCall.id}</span>
      </div>
      {args && args !== "{}" && (
        <div className="mt-1.5">
          <pre className="bg-muted/50 rounded border p-2 font-mono text-[11px] leading-relaxed break-words whitespace-pre-wrap">
            {args}
          </pre>
        </div>
      )}
    </div>
  )
}

function ResponseChoiceBlock({
  choice,
  isStreaming = false,
  showIndex = false,
}: {
  choice: ParsedResponseChoice
  isStreaming?: boolean
  showIndex?: boolean
}) {
  const { t } = useTranslation("logs")
  const { text } = getMessageTextContent(choice.assistantContent)
  const [thinkingOpen, setThinkingOpen] = useState(false)

  const isThinkingPhase = isStreaming && !!choice.thinkingContent && !text
  const showThinking = thinkingOpen || isThinkingPhase

  return (
    <div className="border-l-nb-lime bg-nb-lime/15 dark:bg-nb-lime/10 rounded-md border-2 border-l-4">
      <div className="flex items-center gap-2 px-3 py-2">
        <div className="border-border bg-nb-lime text-foreground flex h-6 w-6 shrink-0 items-center justify-center rounded-md border-2">
          <Bot className="h-3.5 w-3.5" />
        </div>
        <span className="text-xs font-bold tracking-wide uppercase">
          {t("messagesTab.response")}
        </span>
        {showIndex && (
          <Badge variant="secondary" className="text-[10px]">
            Choice #{choice.index}
          </Badge>
        )}
        {isStreaming && (
          <Badge variant="ghost" className="animate-pulse gap-1 text-[10px]">
            <Loader2 className="h-2.5 w-2.5 animate-spin" />
            {t("messagesTab.streaming")}
          </Badge>
        )}
        {choice.finishReason && (
          <Badge variant="ghost" className="text-[10px]">
            {choice.finishReason}
          </Badge>
        )}
      </div>

      {choice.thinkingContent && (
        <div className="border-border/50 border-t">
          <button
            className="flex w-full items-center gap-1.5 px-3 py-1.5 text-left"
            onClick={() => setThinkingOpen(!thinkingOpen)}
          >
            <Brain className="text-muted-foreground h-3 w-3 shrink-0" />
            <span className="text-muted-foreground text-[10px] font-bold tracking-wider uppercase">
              {t("messagesTab.thinking")}
            </span>
            <span className="text-muted-foreground/60 text-[10px]">
              {t("messagesTab.thinkingChars", {
                chars: choice.thinkingContent.length.toLocaleString(),
              })}
            </span>
            <ChevronDown
              className={`text-muted-foreground ml-auto h-3 w-3 transition-transform ${showThinking ? "rotate-180" : ""}`}
            />
          </button>
          {showThinking && (
            <div className="border-border/50 border-t px-3 py-2">
              <pre className="text-muted-foreground max-h-64 overflow-auto text-[11px] leading-relaxed whitespace-pre-wrap">
                {choice.thinkingContent}
                {isThinkingPhase && (
                  <span className="bg-muted-foreground ml-0.5 inline-block h-[1em] w-[2px] animate-pulse align-text-bottom" />
                )}
              </pre>
            </div>
          )}
        </div>
      )}

      {text && (
        <div className="border-border/50 border-t px-3 py-2">
          <div className="prose prose-sm dark:prose-invert max-w-none break-words">
            <Suspense
              fallback={
                <pre className="font-[inherit] text-xs leading-relaxed whitespace-pre-wrap">
                  {text}
                </pre>
              }
            >
              <LazyMarkdown>{text}</LazyMarkdown>
            </Suspense>
            {isStreaming && (
              <span className="bg-foreground ml-0.5 inline-block h-[1em] w-[2px] animate-pulse align-text-bottom" />
            )}
          </div>
        </div>
      )}

      {choice.toolCalls && choice.toolCalls.length > 0 && (
        <div className="border-border/50 border-t px-3 py-2">
          <p className="text-muted-foreground mb-1.5 text-[10px] font-bold tracking-wider uppercase">
            {t("messagesTab.toolCalls")}
          </p>
          <div className="flex flex-col gap-1.5">
            {choice.toolCalls.map((tc) => (
              <ToolCallBlock key={tc.id} toolCall={tc} />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function ResponseMetadata({ response }: { response: ParsedResponse }) {
  const { t } = useTranslation("logs")
  const hasMetadata =
    response.id || response.model || response.created || response.systemFingerprint
  if (!hasMetadata) return null

  return (
    <div className="bg-muted/30 rounded-md border p-2.5">
      <p className="text-muted-foreground mb-1.5 text-[10px] font-bold tracking-wider uppercase">
        {t("messagesTab.responseMetadata")}
      </p>
      <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
        {response.id && (
          <div className="flex items-center gap-1.5">
            <span className="text-muted-foreground">{t("messagesTab.metaId")}:</span>
            <span className="truncate font-mono text-[11px]">{response.id}</span>
          </div>
        )}
        {response.model && (
          <div className="flex items-center gap-1.5">
            <span className="text-muted-foreground">{t("messagesTab.metaModel")}:</span>
            <span className="truncate font-mono text-[11px]">{response.model}</span>
          </div>
        )}
        {response.created && (
          <div className="flex items-center gap-1.5">
            <span className="text-muted-foreground">{t("messagesTab.metaCreated")}:</span>
            <span className="font-mono text-[11px]">
              {new Date(response.created * 1000).toLocaleString(undefined, { hour12: false })}
            </span>
          </div>
        )}
        {response.systemFingerprint && (
          <div className="flex items-center gap-1.5">
            <span className="text-muted-foreground">{t("messagesTab.metaFingerprint")}:</span>
            <span className="truncate font-mono text-[11px]">{response.systemFingerprint}</span>
          </div>
        )}
      </div>
    </div>
  )
}

function UsageDetails({ usage }: { usage: ParsedResponseUsage }) {
  const { t } = useTranslation("logs")
  return (
    <div className="bg-muted/30 rounded-md border p-2.5">
      <p className="text-muted-foreground mb-1.5 text-[10px] font-bold tracking-wider uppercase">
        {t("messagesTab.usage")}
      </p>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
        {usage.prompt_tokens != null && (
          <div className="flex items-center gap-1">
            <span className="text-muted-foreground">{t("messagesTab.usagePrompt")}:</span>
            <span className="font-mono font-medium">{usage.prompt_tokens.toLocaleString()}</span>
            {usage.prompt_tokens_details?.cached_tokens != null &&
              usage.prompt_tokens_details.cached_tokens > 0 && (
                <span className="text-muted-foreground font-mono text-[10px]">
                  ({t("messagesTab.usageCached")}:{" "}
                  {usage.prompt_tokens_details.cached_tokens.toLocaleString()})
                </span>
              )}
          </div>
        )}
        {usage.completion_tokens != null && (
          <div className="flex items-center gap-1">
            <span className="text-muted-foreground">{t("messagesTab.usageCompletion")}:</span>
            <span className="font-mono font-medium">
              {usage.completion_tokens.toLocaleString()}
            </span>
            {usage.completion_tokens_details?.reasoning_tokens != null &&
              usage.completion_tokens_details.reasoning_tokens > 0 && (
                <span className="text-muted-foreground font-mono text-[10px]">
                  ({t("messagesTab.usageReasoning")}:{" "}
                  {usage.completion_tokens_details.reasoning_tokens.toLocaleString()})
                </span>
              )}
          </div>
        )}
        {usage.total_tokens != null && (
          <div className="flex items-center gap-1">
            <span className="text-muted-foreground">{t("messagesTab.usageTotal")}:</span>
            <span className="font-mono font-medium">{usage.total_tokens.toLocaleString()}</span>
          </div>
        )}
      </div>
    </div>
  )
}

function ResponseBlock({
  response,
  isStreaming = false,
}: {
  response: ParsedResponse
  isStreaming?: boolean
}) {
  const showMultipleChoices = response.choices.length > 1

  return (
    <div className="flex flex-col gap-2">
      {response.choices.map((choice) => (
        <ResponseChoiceBlock
          key={choice.index}
          choice={choice}
          isStreaming={isStreaming}
          showIndex={showMultipleChoices}
        />
      ))}
      {response.usage && <UsageDetails usage={response.usage} />}
      <ResponseMetadata response={response} />
    </div>
  )
}

function RequestParamsSummary({ params }: { params: ParsedRequestParams }) {
  const { t } = useTranslation("logs")
  const entries: Array<{ label: string; value: React.ReactNode }> = []

  if (params.model != null) entries.push({ label: "model", value: params.model })
  if (params.stream != null) entries.push({ label: "stream", value: String(params.stream) })
  if (params.temperature != null)
    entries.push({ label: "temperature", value: String(params.temperature) })
  if (params.max_tokens != null)
    entries.push({ label: "max_tokens", value: params.max_tokens.toLocaleString() })
  if (params.max_completion_tokens != null)
    entries.push({
      label: "max_completion_tokens",
      value: params.max_completion_tokens.toLocaleString(),
    })
  if (params.top_p != null) entries.push({ label: "top_p", value: String(params.top_p) })
  if (params.frequency_penalty != null)
    entries.push({ label: "frequency_penalty", value: String(params.frequency_penalty) })
  if (params.presence_penalty != null)
    entries.push({ label: "presence_penalty", value: String(params.presence_penalty) })
  if (params.seed != null) entries.push({ label: "seed", value: String(params.seed) })
  if (params.n != null && params.n > 1) entries.push({ label: "n", value: String(params.n) })
  if (params.user) entries.push({ label: "user", value: params.user })
  if (params.response_format) {
    entries.push({
      label: "response_format",
      value: (
        <Badge variant="secondary" className="text-[10px]">
          {params.response_format.type}
        </Badge>
      ),
    })
  }
  if (params.stop) {
    const stops = Array.isArray(params.stop) ? params.stop : [params.stop]
    entries.push({
      label: "stop",
      value: (
        <div className="flex flex-wrap gap-1">
          {stops.map((s, i) => (
            <Badge key={i} variant="outline" className="font-mono text-[10px]">
              {s}
            </Badge>
          ))}
        </div>
      ),
    })
  }

  if (entries.length === 0) return null

  return (
    <div className="bg-muted/30 rounded-md border p-2.5">
      <p className="text-muted-foreground mb-1.5 text-[10px] font-bold tracking-wider uppercase">
        {t("messagesTab.requestParams")}
      </p>
      <div className="grid grid-cols-2 gap-x-4 gap-y-1.5 text-xs sm:grid-cols-3">
        {entries.map((entry) => (
          <div key={entry.label} className="flex min-w-0 items-center gap-1.5">
            <span className="text-muted-foreground shrink-0">{entry.label}:</span>
            <span className="truncate font-mono text-[11px]">{entry.value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function ToolsDefinitionList({ tools }: { tools: ParsedRequestTools }) {
  const { t } = useTranslation("logs")
  const [expandedTools, setExpandedTools] = useState<Set<number>>(new Set())

  const toggleTool = (index: number) => {
    setExpandedTools((prev) => {
      const next = new Set(prev)
      if (next.has(index)) next.delete(index)
      else next.add(index)
      return next
    })
  }

  const toolChoiceLabel = (() => {
    if (!tools.tool_choice) return null
    if (typeof tools.tool_choice === "string") return tools.tool_choice
    if (typeof tools.tool_choice === "object" && tools.tool_choice.function?.name) {
      return tools.tool_choice.function.name
    }
    return null
  })()

  return (
    <div className="bg-muted/30 rounded-md border p-2.5">
      <div className="mb-1.5 flex items-center gap-2">
        <Wrench className="text-muted-foreground h-3 w-3 shrink-0" />
        <p className="text-muted-foreground text-[10px] font-bold tracking-wider uppercase">
          {t("messagesTab.tools")} ({tools.tools.length})
        </p>
        {toolChoiceLabel && (
          <Badge variant="secondary" className="text-[10px]">
            {t("messagesTab.toolChoice")}: {toolChoiceLabel}
          </Badge>
        )}
      </div>
      <div className="flex flex-col gap-1">
        {tools.tools.map((tool, i) => {
          const isExpanded = expandedTools.has(i)
          let paramsStr = ""
          if (tool.function.parameters) {
            try {
              paramsStr = JSON.stringify(tool.function.parameters, null, 2)
            } catch {
              paramsStr = String(tool.function.parameters)
            }
          }
          return (
            <div key={i} className="bg-background/60 rounded-md border">
              <button
                className="flex w-full items-center gap-1.5 px-2.5 py-1.5 text-left"
                onClick={() => toggleTool(i)}
              >
                <Code2 className="text-muted-foreground h-3 w-3 shrink-0" />
                <span className="font-mono text-xs font-bold">{tool.function.name}</span>
                {tool.function.description && (
                  <span className="text-muted-foreground truncate text-[11px]">
                    — {tool.function.description}
                  </span>
                )}
                <ChevronDown
                  className={`text-muted-foreground ml-auto h-3 w-3 shrink-0 transition-transform ${isExpanded ? "rotate-180" : ""}`}
                />
              </button>
              {isExpanded && paramsStr && (
                <div className="border-border/50 border-t px-2.5 py-2">
                  <pre className="bg-muted/50 max-h-48 overflow-auto rounded border p-2 font-mono text-[11px] leading-relaxed break-words whitespace-pre-wrap">
                    {paramsStr}
                  </pre>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function MessagesTabContent({
  detail,
  streamingOverlay,
}: {
  detail: LogDetail
  streamingOverlay: { thinkingContent: string; responseContent: string } | null
}) {
  const { t } = useTranslation("logs")
  const [viewMode, setViewMode] = useState<"conversation" | "raw">("conversation")

  const messages = useMemo(() => parseMessages(detail.requestContent), [detail.requestContent])
  const newTurnBoundary = useMemo(() => (messages ? findNewTurnBoundary(messages) : 0), [messages])
  const [contextExpanded, setContextExpanded] = useState(false)

  // Reset collapsed state when switching between log entries
  const prevDetailId = useRef(detail.id)
  useEffect(() => {
    if (prevDetailId.current !== detail.id) {
      setContextExpanded(false)
      prevDetailId.current = detail.id
    }
  }, [detail.id])

  const requestParams = useMemo(
    () => parseRequestParams(detail.requestContent),
    [detail.requestContent],
  )
  const requestTools = useMemo(
    () => parseRequestTools(detail.requestContent),
    [detail.requestContent],
  )

  // Use streaming overlay content when available (real-time during SSE proxy),
  // otherwise fall back to the stored response content from the DB.
  const effectiveResponseContent = streamingOverlay
    ? (streamingOverlay.thinkingContent
        ? `<|thinking|>${streamingOverlay.thinkingContent}<|/thinking|>`
        : "") + streamingOverlay.responseContent
    : detail.responseContent

  const response = useMemo(
    () => parseResponseContent(effectiveResponseContent),
    [effectiveResponseContent],
  )

  const canShowConversation = messages !== null
  const isStreamingOnly = !canShowConversation && !!streamingOverlay

  return (
    <div className="flex flex-col gap-3">
      {/* View mode toggle */}
      {canShowConversation && (
        <div className="flex items-center justify-between">
          <span className="text-muted-foreground text-xs">
            {t("messagesTab.count", { count: messages.length })}
            {response ? t("messagesTab.plusResponse") : ""}
          </span>
          <div className="border-border flex items-center rounded-md border-2">
            <button
              className={`flex items-center gap-1 px-2 py-1 text-xs font-bold transition-colors ${
                viewMode === "conversation"
                  ? "bg-primary text-primary-foreground"
                  : "hover:bg-muted"
              }`}
              onClick={() => setViewMode("conversation")}
            >
              <MessageSquare className="h-3 w-3" />
              {t("messagesTab.chat")}
            </button>
            <button
              className={`border-border flex items-center gap-1 border-l-2 px-2 py-1 text-xs font-bold transition-colors ${
                viewMode === "raw" ? "bg-primary text-primary-foreground" : "hover:bg-muted"
              }`}
              onClick={() => setViewMode("raw")}
            >
              <Braces className="h-3 w-3" />
              {t("messagesTab.raw")}
            </button>
          </div>
        </div>
      )}

      {isStreamingOnly ? (
        <div className="flex flex-col gap-2">
          {response ? (
            <ResponseBlock response={response} isStreaming />
          ) : (
            <div className="flex items-center justify-center gap-2 py-8">
              <Loader2 className="text-muted-foreground h-4 w-4 animate-spin" />
              <span className="text-muted-foreground text-sm">{t("messagesTab.streaming")}</span>
            </div>
          )}
        </div>
      ) : canShowConversation && viewMode === "conversation" ? (
        <div className="flex flex-col gap-2">
          {requestParams && <RequestParamsSummary params={requestParams} />}
          {requestTools && <ToolsDefinitionList tools={requestTools} />}
          {newTurnBoundary > 0 && (
            <>
              <button
                className="text-muted-foreground hover:text-foreground flex items-center gap-1 self-start text-[11px] font-bold tracking-wide uppercase"
                onClick={() => setContextExpanded(!contextExpanded)}
              >
                {contextExpanded ? (
                  <ChevronUp className="h-3 w-3" />
                ) : (
                  <ChevronDown className="h-3 w-3" />
                )}
                {t("messagesTab.previousContext", { count: newTurnBoundary })}
              </button>
              {contextExpanded &&
                messages
                  .slice(0, newTurnBoundary)
                  .map((msg, i) => (
                    <MessageBubble key={`ctx-${i.toString()}`} msg={msg} index={i} isContext />
                  ))}
              <div className="text-muted-foreground flex items-center gap-2 py-1 text-[11px] font-bold tracking-wide uppercase">
                <div className="border-border flex-1 border-t" />
                {t("messagesTab.currentTurn")}
                <div className="border-border flex-1 border-t" />
              </div>
            </>
          )}
          {messages.slice(newTurnBoundary).map((msg, i) => (
            <MessageBubble
              key={`msg-${(newTurnBoundary + i).toString()}`}
              msg={msg}
              index={newTurnBoundary + i}
            />
          ))}
          {response && <ResponseBlock response={response} isStreaming={!!streamingOverlay} />}
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          <CollapsibleCodeBlock
            label={t("messagesTab.requestOriginal")}
            content={detail.requestContent}
            defaultOpen
          />
          {detail.upstreamContent && (
            <CollapsibleCodeBlock
              label={t("messagesTab.requestUpstream")}
              content={detail.upstreamContent}
              defaultOpen={false}
            />
          )}
          <CollapsibleCodeBlock
            label={t("messagesTab.response")}
            content={detail.responseContent}
            defaultOpen
          />
        </div>
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
  const { t } = useTranslation("logs")
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
        <p className="text-muted-foreground text-sm">{t("codeBlock.noContent")}</p>
      </div>
    )
  }

  const handleCopy = () => {
    navigator.clipboard.writeText(plainText)
    setCopied(true)
    toast.success(t("toast.copied"))
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="flex min-w-0 flex-col gap-2">
      <div className="flex items-center justify-between">
        <p className="text-muted-foreground text-xs font-medium">{label}</p>
        <Button variant="ghost" size="sm" className="h-7 gap-1" onClick={handleCopy}>
          {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
          <span className="text-xs">
            {copied ? t("actions.copied", { ns: "common" }) : t("actions.copy", { ns: "common" })}
          </span>
        </Button>
      </div>
      <div className="relative">
        <Search className="text-muted-foreground absolute top-2 left-2.5 h-3.5 w-3.5" />
        <Input
          placeholder={t("codeBlock.searchPlaceholder")}
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="h-8 pr-16 pl-8 text-xs"
        />
        {searchTerm && (
          <div className="absolute top-1.5 right-2 flex items-center gap-1">
            <span className="text-muted-foreground text-xs">
              {t("codeBlock.match", { count: matchCount })}
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
              {t("codeBlock.truncatedRecovered")}
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
  const { t } = useTranslation("logs")
  const skeletonRows = rows > 8 ? SKELETON_ROWS_10 : SKELETON_ROWS_8
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("columns.time")}</TableHead>
          <TableHead>{t("columns.model")}</TableHead>
          <TableHead>{t("columns.channel")}</TableHead>
          <TableHead className="text-right">{t("columns.input")}</TableHead>
          <TableHead className="text-right">{t("columns.output")}</TableHead>
          <TableHead className="text-right">{t("columns.ttft")}</TableHead>
          <TableHead className="text-right">{t("columns.latency")}</TableHead>
          <TableHead className="text-right">{t("columns.cost")}</TableHead>
          <TableHead>{t("columns.status")}</TableHead>
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

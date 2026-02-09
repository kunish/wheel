"use client"

import { useQuery, useQueryClient } from "@tanstack/react-query"
import JsonView from "@uiw/react-json-view"
import { githubDarkTheme } from "@uiw/react-json-view/githubDark"
import { githubLightTheme } from "@uiw/react-json-view/githubLight"
import {
  ArrowUp,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Copy,
  Eye,
  Loader2,
  Play,
  Search,
  X,
} from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import { useTheme } from "next-themes"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import { toast } from "sonner"
import { ModelBadge } from "@/components/model-badge"
import { formatRangeSummary, TimeRangePicker } from "@/components/time-range-picker"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { getWsUrl } from "@/hooks/use-stats-ws"
import { listChannels as apiListChannels, getLog, listLogs, replayLog } from "@/lib/api"
import { buildFilterSearchParams, parseLogFilters } from "./log-filters"

interface LogEntry {
  id: number
  time: number
  requestModelName: string
  actualModelName: string
  channelId: number
  channelName: string
  inputTokens: number
  outputTokens: number
  ftut: number
  useTime: number
  error: string
  cost?: number
  totalAttempts: number
}

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
  responseContent: string
  error: string
  attempts: Array<{
    channelId: number
    channelName: string
    modelName: string
    round: number
    attemptNum: number
    success: boolean
    error: string
    duration: number
  }>
  totalAttempts: number
  successfulRound: number
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

function formatCost(cost: number | undefined): string {
  if (!cost || cost === 0) return "$0"
  if (cost < 0.000001) return `$${cost.toExponential(1)}`
  if (cost < 0.01) return `$${cost.toFixed(6)}`
  return `$${cost.toFixed(4)}`
}

function formatTime(ts: number): string {
  const d = new Date(ts * 1000)
  const month = String(d.getMonth() + 1).padStart(2, "0")
  const day = String(d.getDate()).padStart(2, "0")
  const hours = String(d.getHours()).padStart(2, "0")
  const minutes = String(d.getMinutes()).padStart(2, "0")
  const seconds = String(d.getSeconds()).padStart(2, "0")
  return `${month}/${day} ${hours}:${minutes}:${seconds}`
}

export default function LogsPage() {
  const queryClient = useQueryClient()
  const searchParams = useSearchParams()
  const router = useRouter()
  const pathname = usePathname()

  // Derive filter state from URL search params
  const filters = parseLogFilters(searchParams)
  const { page, model, status, channelId, keyword, pageSize, startTime, endTime } = filters

  // Local state for controlled text inputs (synced to URL via debounce)
  const [modelInput, setModelInput] = useState(model)
  const [keywordInput, setKeywordInput] = useState(keyword)

  // Sync local input state when URL changes externally (e.g., deep links from dashboard)
  const prevModel = useRef(model)
  const prevKeyword = useRef(keyword)
  if (prevModel.current !== model) {
    prevModel.current = model
    setModelInput(model)
  }
  if (prevKeyword.current !== keyword) {
    prevKeyword.current = keyword
    setKeywordInput(keyword)
  }

  // Helper to update URL search params — resets page to 1 unless page itself is being updated
  const updateFilter = useCallback(
    (updates: Record<string, string | number | undefined | null>) => {
      const params = buildFilterSearchParams(searchParams, updates)
      const query = params.toString()
      router.replace(query ? `${pathname}?${query}` : pathname, { scroll: false })
    },
    [searchParams, pathname, router],
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

  const hasFilters =
    model !== "" ||
    status !== "all" ||
    keyword !== "" ||
    channelId !== undefined ||
    startTime !== undefined
  const isFirstPage = page === 1

  const { data, isLoading } = useQuery({
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

  // Listen for log-created WebSocket events
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    function connect() {
      const wsUrl = getWsUrl()

      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data)
          if (msg.event === "log-created" && msg.data?.log) {
            if (isFirstPage && !hasFilters) {
              // Prepend new log to cache
              queryClient.setQueryData(
                ["logs", page, pageSize, model, status, channelId, keyword, startTime, endTime],
                (
                  old:
                    | { data?: { logs: LogEntry[]; total: number; page: number; pageSize: number } }
                    | undefined,
                ) => {
                  if (!old?.data) return old
                  const newLogs = [msg.data.log as LogEntry, ...old.data.logs].slice(0, pageSize)
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
              // Show "new logs available" indicator
              setPendingCount((c) => c + 1)
            }
          }
        } catch {
          // ignore
        }
      }

      ws.onclose = () => {
        wsRef.current = null
        reconnectTimerRef.current = setTimeout(connect, 3000)
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    connect()

    return () => {
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [
    queryClient,
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
  ])

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
    router.replace(pathname, { scroll: false })
    setModelInput("")
    setKeywordInput("")
    setPendingCount(0)
    queryClient.invalidateQueries({ queryKey: ["logs"] })
  }, [queryClient, router, pathname])

  const logs = (data?.data?.logs ?? []) as LogEntry[]
  const total = data?.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)
  const detail = (detailData?.data ?? null) as LogDetail | null

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
          {!isLoading && (
            <span className="text-muted-foreground text-sm">{total.toLocaleString()} total</span>
          )}
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
        {!isLoading && totalPages > 0 && paginationControls}
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
          <Input
            placeholder="Model..."
            value={modelInput}
            onChange={(e) => {
              setModelInput(e.target.value)
              debouncedUpdateFilter("model", e.target.value)
            }}
            className="w-36"
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
                    setModelInput("")
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
                router.replace(pathname, { scroll: false })
                setModelInput("")
                setKeywordInput("")
              }}
            >
              Clear all
            </Button>
          </div>
        )}
      </div>

      {isLoading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : (
        <>
          <TooltipProvider delayDuration={300}>
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
                <AnimatePresence initial={false}>
                  {logs.map((log) => (
                    <motion.tr
                      key={log.id}
                      initial={{ opacity: 0, y: -10 }}
                      animate={{ opacity: 1, y: 0 }}
                      transition={{ duration: 0.25 }}
                      className={`hover:bg-muted/50 cursor-pointer border-b ${
                        log.error ? "border-l-destructive bg-destructive/5 border-l-2" : ""
                      }`}
                      onClick={() => setDetailId(log.id)}
                    >
                      <TableCell className="font-mono text-xs whitespace-nowrap">
                        {formatTime(log.time)}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-col gap-0.5">
                          <ModelBadge modelId={log.requestModelName} />
                          {log.actualModelName && log.actualModelName !== log.requestModelName && (
                            <span className="text-muted-foreground max-w-[150px] truncate text-[10px]">
                              {log.actualModelName}
                            </span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-xs">
                        <div className="flex items-center gap-1">
                          {log.channelName || "—"}
                          {log.totalAttempts > 1 && (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Badge variant="outline" className="px-1 py-0 text-[10px]">
                                  R{log.totalAttempts}
                                </Badge>
                              </TooltipTrigger>
                              <TooltipContent>{log.totalAttempts} attempts</TooltipContent>
                            </Tooltip>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs">
                        {log.inputTokens.toLocaleString()}
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs">
                        {log.outputTokens.toLocaleString()}
                      </TableCell>
                      <TableCell className="text-muted-foreground text-right font-mono text-xs">
                        {log.ftut > 0 ? formatDuration(log.ftut) : "—"}
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs">
                        {formatDuration(log.useTime)}
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs">
                        {formatCost(log.cost)}
                      </TableCell>
                      <TableCell>
                        {log.error ? (
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Badge variant="destructive">Error</Badge>
                            </TooltipTrigger>
                            <TooltipContent className="max-w-xs">
                              <p className="text-xs break-all whitespace-pre-wrap">
                                {log.error.length > 200
                                  ? `${log.error.slice(0, 200)}...`
                                  : log.error}
                              </p>
                            </TooltipContent>
                          </Tooltip>
                        ) : (
                          <Badge variant="default">OK</Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={(e) => {
                            e.stopPropagation()
                            setDetailId(log.id)
                          }}
                        >
                          <Eye className="h-4 w-4" />
                        </Button>
                      </TableCell>
                    </motion.tr>
                  ))}
                </AnimatePresence>
                {logs.length === 0 && !isLoading && (
                  <TableRow>
                    <TableCell colSpan={10} className="py-12 text-center">
                      <p className="text-muted-foreground">No logs match your filters</p>
                      {hasFilters && (
                        <Button
                          variant="outline"
                          size="sm"
                          className="mt-3"
                          onClick={() => {
                            router.replace(pathname, { scroll: false })
                            setModelInput("")
                            setKeywordInput("")
                          }}
                        >
                          Clear all filters
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </TooltipProvider>
        </>
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
                  disabled={!detailId || logs.findIndex((l) => l.id === detailId) <= 0}
                  onClick={() => {
                    const idx = logs.findIndex((l) => l.id === detailId)
                    if (idx > 0) setDetailId(logs[idx - 1].id)
                  }}
                >
                  <ChevronUp className="h-4 w-4" />
                </Button>
                <Button
                  variant="outline"
                  size="icon"
                  className="h-7 w-7"
                  disabled={
                    !detailId || logs.findIndex((l) => l.id === detailId) >= logs.length - 1
                  }
                  onClick={() => {
                    const idx = logs.findIndex((l) => l.id === detailId)
                    if (idx >= 0 && idx < logs.length - 1) setDetailId(logs[idx + 1].id)
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
            <p className="text-muted-foreground py-4 text-center">Loading...</p>
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
        <TabsTrigger value="request" className="flex-1">
          Request
        </TabsTrigger>
        <TabsTrigger value="response" className="flex-1">
          Response
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
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <ModelBadge modelId={detail.requestModelName} />
            <span className="text-muted-foreground">via</span>
            <span className="font-medium">{detail.channelName || "—"}</span>
            {detail.actualModelName && detail.actualModelName !== detail.requestModelName && (
              <>
                <span className="text-muted-foreground">&rarr;</span>
                <ModelBadge modelId={detail.actualModelName} />
              </>
            )}
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
              <CopyableField
                label="Attempts"
                value={`${detail.totalAttempts} (round ${detail.successfulRound})`}
              />
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

      {/* Request Tab */}
      <TabsContent value="request" className="mt-4">
        <CodeBlock label="Request Content" content={detail.requestContent} />
      </TabsContent>

      {/* Response Tab */}
      <TabsContent value="response" className="mt-4">
        <CodeBlock label="Response Content" content={detail.responseContent} />
      </TabsContent>

      {/* Retry Timeline Tab */}
      {detail.attempts && detail.attempts.length > 0 && (
        <TabsContent value="retry" className="mt-4">
          <div className="border-border relative flex flex-col gap-3 border-l-2 pl-4">
            {detail.attempts.map((attempt) => (
              <div key={`${attempt.round}-${attempt.attemptNum}`} className="relative">
                <div
                  className={`border-background absolute top-1 -left-[calc(0.5rem+1px)] h-3 w-3 rounded-full border-2 ${
                    attempt.success ? "bg-green-500" : "bg-destructive"
                  }`}
                />
                <div className="ml-2 rounded-md border p-2">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs">
                      R{attempt.round}.{attempt.attemptNum}
                    </span>
                    <Badge
                      variant={attempt.success ? "default" : "destructive"}
                      className="text-xs"
                    >
                      {attempt.success ? "OK" : "FAIL"}
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
                  {attempt.error && (
                    <p className="text-destructive mt-1 text-xs break-all">{attempt.error}</p>
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

function CodeBlock({ label, content }: { label: string; content: string }) {
  const [copied, setCopied] = useState(false)
  const { resolvedTheme } = useTheme()

  const displayContent = content?.trim() || ""

  const parsed = useMemo(() => {
    if (!displayContent) return { isJson: false, data: null, truncated: false }
    try {
      return { isJson: true, data: JSON.parse(displayContent), truncated: false }
    } catch {
      // Best-effort: if content looks like truncated JSON, try to salvage it
      if (displayContent.startsWith("{") || displayContent.startsWith("[")) {
        try {
          let repaired = displayContent
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
            repaired += displayContent.startsWith("{") ? "}" : "]"
          }
          const data = JSON.parse(repaired)
          return { isJson: true, data, truncated: true }
        } catch {
          // Give up — display as plain text
        }
      }
      return { isJson: false, data: displayContent, truncated: false }
    }
  }, [displayContent])

  if (!displayContent) {
    return (
      <div className="flex flex-col gap-2">
        <p className="text-muted-foreground text-sm">No content available.</p>
      </div>
    )
  }

  const handleCopy = () => {
    const text = parsed.isJson ? JSON.stringify(parsed.data, null, 2) : displayContent
    navigator.clipboard.writeText(text)
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
      {parsed.isJson ? (
        <div className="flex flex-col gap-2">
          {parsed.truncated && (
            <p className="text-muted-foreground text-xs italic">
              Content was truncated and partially recovered.
            </p>
          )}
          <div className="bg-muted/30 max-h-[50vh] min-w-0 overflow-auto rounded-md border p-3">
            <JsonView
              value={parsed.data}
              style={{
                ...(resolvedTheme === "dark" ? githubDarkTheme : githubLightTheme),
                fontSize: "12px",
                backgroundColor: "transparent",
                fontFamily: "ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace",
              }}
              displayDataTypes={false}
              displayObjectSize={false}
              collapsed={2}
            />
          </div>
        </div>
      ) : (
        <div className="bg-muted/30 prose prose-sm dark:prose-invert max-h-[50vh] max-w-none overflow-auto rounded-md border p-3 break-words">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{displayContent}</ReactMarkdown>
        </div>
      )}
    </div>
  )
}

"use client"

import { useQuery, useQueryClient } from "@tanstack/react-query"
import JsonView from "@uiw/react-json-view"
import { githubDarkTheme } from "@uiw/react-json-view/githubDark"
import { githubLightTheme } from "@uiw/react-json-view/githubLight"
import { ArrowUp, Check, ChevronLeft, ChevronRight, Copy, Eye } from "lucide-react"
import { useTheme } from "next-themes"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import { toast } from "sonner"
import { ModelBadge } from "@/components/model-badge"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
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
import { getLog, listLogs } from "@/lib/api"

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
  const [page, setPage] = useState(1)
  const [model, setModel] = useState("")
  const [status, setStatus] = useState("all")
  const [detailId, setDetailId] = useState<number | null>(null)
  const [pendingCount, setPendingCount] = useState(0)

  const hasFilters = model !== "" || status !== "all"
  const isFirstPage = page === 1

  const { data, isLoading } = useQuery({
    queryKey: ["logs", page, model, status],
    queryFn: () =>
      listLogs({
        page,
        pageSize: 20,
        ...(model ? { model } : {}),
        ...(status !== "all" ? { status } : {}),
      }),
  })

  const { data: detailData } = useQuery({
    queryKey: ["log-detail", detailId],
    queryFn: () => getLog(detailId!),
    enabled: detailId !== null,
  })

  // Listen for log-created WebSocket events
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    function connect() {
      const proto = window.location.protocol === "https:" ? "wss:" : "ws:"
      const wsUrl = `${proto}//${window.location.host}/api/v1/ws`

      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data)
          if (msg.event === "log-created" && msg.data?.log) {
            if (isFirstPage && !hasFilters) {
              // Prepend new log to cache
              queryClient.setQueryData(
                ["logs", page, model, status],
                (
                  old:
                    | { data?: { logs: LogEntry[]; total: number; page: number; pageSize: number } }
                    | undefined,
                ) => {
                  if (!old?.data) return old
                  const newLogs = [msg.data.log as LogEntry, ...old.data.logs].slice(0, 20)
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
  }, [queryClient, page, model, status, isFirstPage, hasFilters])

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
    setPage(1)
    setModel("")
    setStatus("all")
    setPendingCount(0)
    queryClient.invalidateQueries({ queryKey: ["logs"] })
  }, [queryClient])

  const logs = (data?.data?.logs ?? []) as LogEntry[]
  const total = data?.data?.total ?? 0
  const totalPages = Math.ceil(total / 20)
  const detail = (detailData?.data ?? null) as LogDetail | null

  return (
    <div className="flex flex-col gap-6">
      <h2 className="text-2xl font-bold tracking-tight">Logs</h2>

      <div className="flex gap-3">
        <Input
          placeholder="Filter by model..."
          value={model}
          onChange={(e) => {
            setModel(e.target.value)
            setPage(1)
          }}
          className="max-w-xs"
        />
        <Select
          value={status}
          onValueChange={(v) => {
            setStatus(v)
            setPage(1)
          }}
        >
          <SelectTrigger className="w-32">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="success">Success</SelectItem>
            <SelectItem value="error">Error</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* New logs notification bar */}
      {pendingCount > 0 && (
        <Button variant="outline" size="sm" className="w-fit" onClick={handleShowNew}>
          <ArrowUp className="mr-2 h-4 w-4" />
          {pendingCount} new log{pendingCount > 1 ? "s" : ""} available
        </Button>
      )}

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
                {logs.map((log) => (
                  <TableRow
                    key={log.id}
                    className="cursor-pointer"
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
                      <Badge variant={log.error ? "destructive" : "default"}>
                        {log.error ? "Error" : "OK"}
                      </Badge>
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
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TooltipProvider>

          <div className="flex items-center justify-between">
            <span className="text-muted-foreground text-sm">
              {total.toLocaleString()} logs total
            </span>
            <div className="flex items-center gap-2">
              <span className="text-muted-foreground text-sm">
                Page {page} of {totalPages || 1}
              </span>
              <Button
                variant="outline"
                size="icon"
                disabled={page <= 1}
                onClick={() => setPage((p) => p - 1)}
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <Button
                variant="outline"
                size="icon"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </>
      )}

      {/* Log Detail Dialog */}
      <Dialog open={detailId !== null} onOpenChange={(open) => !open && setDetailId(null)}>
        <DialogContent className="max-h-[85vh] max-w-3xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              Log #{detailId}
              {detail && (
                <Badge variant={detail.error ? "destructive" : "default"} className="text-xs">
                  {detail.error ? "Error" : "OK"}
                </Badge>
              )}
            </DialogTitle>
          </DialogHeader>
          {detail ? (
            <Tabs defaultValue="overview" className="w-full">
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
              </TabsList>

              {/* Overview Tab */}
              <TabsContent value="overview" className="mt-4">
                <div className="flex flex-col gap-4 text-sm">
                  {/* Model Flow */}
                  <div className="flex items-center gap-2 text-sm">
                    <ModelBadge modelId={detail.requestModelName} />
                    <span className="text-muted-foreground">via</span>
                    <span className="font-medium">{detail.channelName || "—"}</span>
                    {detail.actualModelName &&
                      detail.actualModelName !== detail.requestModelName && (
                        <>
                          <span className="text-muted-foreground">&rarr;</span>
                          <ModelBadge modelId={detail.actualModelName} />
                        </>
                      )}
                  </div>

                  <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                    <Field label="Time">{new Date(detail.time * 1000).toLocaleString()}</Field>
                    <Field label="Channel">
                      {detail.channelName || "—"} (ID: {detail.channelId})
                    </Field>
                    <Field label="Input Tokens">{detail.inputTokens.toLocaleString()}</Field>
                    <Field label="Output Tokens">{detail.outputTokens.toLocaleString()}</Field>
                    <Field label="Total Tokens">
                      {(detail.inputTokens + detail.outputTokens).toLocaleString()}
                    </Field>
                    <Field label="Cost">{formatCost(detail.cost)}</Field>
                    <Field label="TTFT">
                      {detail.ftut > 0 ? formatDuration(detail.ftut) : "—"}
                    </Field>
                    <Field label="Total Latency">{formatDuration(detail.useTime)}</Field>
                    <Field label="Output Speed">
                      {detail.outputTokens > 0 && detail.useTime > 0
                        ? `${(detail.outputTokens / (detail.useTime / 1000)).toFixed(1)} tok/s`
                        : "—"}
                    </Field>
                    {detail.totalAttempts > 1 && (
                      <Field label="Attempts">
                        {detail.totalAttempts} (round {detail.successfulRound})
                      </Field>
                    )}
                  </div>

                  {detail.error && (
                    <div className="bg-destructive/10 rounded-md p-3">
                      <p className="text-destructive mb-1 text-xs font-medium">Error</p>
                      <pre className="text-xs break-all whitespace-pre-wrap">{detail.error}</pre>
                    </div>
                  )}
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
                            <span className="text-muted-foreground">Channel:</span>{" "}
                            {attempt.channelName}{" "}
                            <span className="text-muted-foreground">Model:</span>{" "}
                            <ModelBadge modelId={attempt.modelName} />
                          </p>
                          {attempt.error && (
                            <p className="text-destructive mt-1 text-xs break-all">
                              {attempt.error}
                            </p>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </TabsContent>
              )}
            </Tabs>
          ) : (
            <p className="text-muted-foreground py-4 text-center">Loading...</p>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="text-muted-foreground text-xs">{label}</p>
      <p className="font-medium">{children}</p>
    </div>
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
    <div className="flex flex-col gap-2">
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
          <div className="bg-muted/30 max-h-[50vh] overflow-auto rounded-md border p-3">
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

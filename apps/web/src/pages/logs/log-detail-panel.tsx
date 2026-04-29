import type { ReactNode } from "react"
import type { LogEntry } from "./columns"
import type { LogDetail, StreamingOverlay } from "./types"
import { ChevronDown, ChevronUp, Copy, Loader2, Play, TriangleAlert } from "lucide-react"
import { Component, useCallback, useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import { toast } from "sonner"
import { ModelBadge } from "@/components/model-badge"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { useModelMeta } from "@/hooks/use-model-meta"
import { cn } from "@/lib/utils"
import { formatCost, formatDuration } from "./columns"
import { CodeBlock } from "./detail/code-block"
import { detectLoggedRequestType } from "./detail/message-parsers"
import { MessagesTabContent } from "./detail/messages-tab"
import { useLogQueryContext } from "./log-query-context"
import { useLogReplay } from "./log-replay"

// Hoisted constant array to avoid re-creation on every render
const DETAIL_SKELETON_ITEMS = Array.from({ length: 9 })

// ── Error Boundary for Detail Panel ──

interface DetailErrorBoundaryProps {
  children: ReactNode
  onReset?: () => void
}

interface DetailErrorBoundaryState {
  hasError: boolean
  error: Error | null
}

class DetailErrorBoundary extends Component<DetailErrorBoundaryProps, DetailErrorBoundaryState> {
  constructor(props: DetailErrorBoundaryProps) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error }
  }

  override componentDidUpdate(prevProps: DetailErrorBoundaryProps) {
    // Reset error state when children change (e.g. navigating to a different log)
    if (this.state.hasError && prevProps.children !== this.props.children) {
      this.setState({ hasError: false, error: null })
    }
  }

  override render() {
    if (this.state.hasError) {
      return (
        <div className="flex flex-col items-center justify-center gap-3 px-4 py-16">
          <TriangleAlert className="text-destructive h-8 w-8" />
          <p className="text-muted-foreground text-sm font-medium">Failed to render log detail</p>
          {this.state.error?.message && (
            <pre className="text-muted-foreground max-w-full overflow-auto text-xs">
              {this.state.error.message}
            </pre>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              this.setState({ hasError: false, error: null })
              this.props.onReset?.()
            }}
          >
            Retry
          </Button>
        </div>
      )
    }
    return this.props.children
  }
}

// ── Public component: LogDetailSheet ──

export function LogDetailSheet() {
  const { t } = useTranslation("logs")
  const {
    detailId,
    detailStreamId,
    detail,
    pendingStreams,
    streamingOverlay,
    logs,
    setDetailId,
    setDetailStreamId,
  } = useLogQueryContext()

  const onClose = useCallback(() => {
    setDetailId(null)
    setDetailStreamId(null)
  }, [setDetailId, setDetailStreamId])

  const onNavigate = useCallback(
    (log: LogEntry) => {
      if (log._streaming && log._streamId) {
        setDetailId(null)
        setDetailStreamId(log._streamId)
      } else {
        setDetailStreamId(null)
        setDetailId(log.id)
      }
    },
    [setDetailId, setDetailStreamId],
  )

  // Find current index in logs for prev/next navigation
  const currentIdx = useMemo(() => {
    return logs.findIndex((log) => {
      if (detailStreamId) return log._streaming && log._streamId === detailStreamId
      return log.id === detailId
    })
  }, [logs, detailId, detailStreamId])

  return (
    <Sheet
      open={detailId !== null || detailStreamId !== null}
      onOpenChange={(open) => {
        if (!open) onClose()
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
                aria-label="Previous log"
                disabled={currentIdx <= 0}
                onClick={() => {
                  if (currentIdx > 0) onNavigate(logs[currentIdx - 1])
                }}
              >
                <ChevronUp className="h-4 w-4" />
              </Button>
              <Button
                variant="outline"
                size="icon"
                className="h-7 w-7"
                aria-label="Next log"
                disabled={currentIdx < 0 || currentIdx >= logs.length - 1}
                onClick={() => {
                  if (currentIdx >= 0 && currentIdx < logs.length - 1)
                    onNavigate(logs[currentIdx + 1])
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
              cacheReadTokens: entry.cacheReadTokens || 0,
              cacheCreationTokens: entry.cacheCreationTokens || 0,
              cost: 0,
              ftut: entry.ftut,
              useTime: entry.useTime,
              requestContent: entry._requestContent ?? "",
              requestHeaders: "",
              upstreamContent: null,
              responseContent: "",
              responseHeaders: "",
              error: entry.error,
              attempts: [],
              totalAttempts: entry.totalAttempts,
            }
            return (
              <DetailErrorBoundary>
                <DetailPanel
                  detail={streamingDetail}
                  streamingOverlay={streamingOverlay}
                  isStreaming={true}
                />
              </DetailErrorBoundary>
            )
          })()
        ) : detail ? (
          <DetailErrorBoundary key={detail.id}>
            <DetailPanel detail={detail} streamingOverlay={streamingOverlay} isStreaming={false} />
          </DetailErrorBoundary>
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
  )
}

function DetailField({
  label,
  value,
  wrapValue = false,
}: {
  label: string
  value: ReactNode
  wrapValue?: boolean
}) {
  return (
    <div className="min-w-0">
      <p className="text-muted-foreground text-xs">{label}</p>
      <div
        className={cn(
          "mt-1 min-w-0 font-medium",
          wrapValue ? "break-words whitespace-normal" : "truncate",
        )}
      >
        {value}
      </div>
    </div>
  )
}

function formatDetailTimestamp(timestampMs: number | null): string {
  if (timestampMs === null) return "—"
  return new Date(timestampMs).toLocaleString(undefined, { hour12: false })
}

function getRequestTypeTone(type: ReturnType<typeof detectLoggedRequestType>): string {
  switch (type) {
    case "chat":
      return "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-200"
    case "chatStream":
      return "bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-200"
    case "responses":
      return "bg-teal-100 text-teal-800 dark:bg-teal-900/30 dark:text-teal-200"
    case "responsesStream":
      return "bg-violet-100 text-violet-800 dark:bg-violet-900/30 dark:text-violet-200"
    case "embedding":
      return "bg-rose-100 text-rose-800 dark:bg-rose-900/30 dark:text-rose-200"
    default:
      return "bg-muted text-foreground"
  }
}

function DetailPanel({
  detail,
  streamingOverlay,
  isStreaming,
}: {
  detail: LogDetail
  streamingOverlay: StreamingOverlay | null
  isStreaming: boolean
}) {
  const { t } = useTranslation("logs")
  const { replayResult, replaying, handleReplay } = useLogReplay()
  const [retryExpanded, setRetryExpanded] = useState(false)

  const requestType = useMemo(
    () => detectLoggedRequestType(detail.requestContent),
    [detail.requestContent],
  )
  const endTimestampMs = isStreaming ? detail.time * 1000 + detail.useTime : detail.time * 1000
  const startTimestampMs = isStreaming ? detail.time * 1000 : endTimestampMs - detail.useTime

  useEffect(() => {
    setRetryExpanded(false)
  }, [detail.id])

  const isTruncated =
    /\[truncated,?\s*\d+\s*chars\s*total\]/.test(detail.requestContent) ||
    /\[\d+\s*messages?\s*omitted/.test(detail.requestContent) ||
    /\[image data omitted\]/.test(detail.requestContent)

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col px-4">
      <div className="mt-4 min-h-0 flex-1 overflow-y-auto pb-6">
        <div className="flex flex-col gap-6 text-sm">
          {/* Model Flow */}
          <div className="bg-muted/30 rounded-xl border p-4">
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

          <div className="bg-muted/30 space-y-4 rounded-xl border p-4">
            <div>
              <p className="text-muted-foreground text-xs font-semibold tracking-wide uppercase">
                {t("detail.section.timings")}
              </p>
              <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3">
                <DetailField
                  label={t("detail.field.startTime")}
                  value={formatDetailTimestamp(startTimestampMs)}
                />
                <DetailField
                  label={t("detail.field.endTime")}
                  value={formatDetailTimestamp(endTimestampMs)}
                />
                <DetailField
                  label={t("detail.field.totalLatency")}
                  value={formatDuration(detail.useTime)}
                />
              </div>
            </div>

            <div className="border-border/70 border-t pt-4">
              <p className="text-muted-foreground text-xs font-semibold tracking-wide uppercase">
                {t("detail.section.requestDetails")}
              </p>
              <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3">
                <DetailField
                  label={t("detail.field.channel")}
                  value={`${detail.channelName || "—"} (ID: ${detail.channelId})`}
                />
                <DetailField
                  label={t("detail.field.type")}
                  value={
                    <Badge variant="outline" className={getRequestTypeTone(requestType)}>
                      {t(`detail.type.${requestType}`)}
                    </Badge>
                  }
                />
                <DetailField
                  label={t("detail.field.inputTokens")}
                  value={detail.inputTokens.toLocaleString()}
                />
                <DetailField
                  label={t("detail.field.outputTokens")}
                  value={detail.outputTokens.toLocaleString()}
                />
                {(detail.cacheReadTokens > 0 || detail.cacheCreationTokens > 0) && (
                  <DetailField
                    label={t("detail.field.cacheTokens")}
                    value={
                      <span className="text-sm">
                        {detail.cacheReadTokens > 0 && (
                          <span>
                            {t("detail.field.cacheRead")}: {detail.cacheReadTokens.toLocaleString()}
                          </span>
                        )}
                        {detail.cacheReadTokens > 0 && detail.cacheCreationTokens > 0 && " / "}
                        {detail.cacheCreationTokens > 0 && (
                          <span>
                            {t("detail.field.cacheWrite")}:{" "}
                            {detail.cacheCreationTokens.toLocaleString()}
                          </span>
                        )}
                      </span>
                    }
                  />
                )}
                <DetailField
                  label={t("detail.field.totalTokens")}
                  value={(detail.inputTokens + detail.outputTokens).toLocaleString()}
                />
                <DetailField label={t("detail.field.cost")} value={formatCost(detail.cost)} />
                <DetailField
                  label={t("detail.field.ttft")}
                  value={detail.ftut > 0 ? formatDuration(detail.ftut) : "—"}
                />
                <DetailField
                  wrapValue
                  label={t("detail.field.outputSpeed")}
                  value={(() => {
                    if (detail.outputTokens <= 0 || detail.useTime <= 0) return "—"
                    const overall = detail.outputTokens / (detail.useTime / 1000)
                    const genMs = detail.useTime - detail.ftut
                    const generation =
                      detail.ftut > 0 && genMs > 0 ? detail.outputTokens / (genMs / 1000) : null
                    return (
                      <div className="space-y-1 text-sm leading-normal">
                        <p title={t("detail.field.outputSpeedOverall")}>
                          <span className="text-muted-foreground">
                            {t("detail.field.outputSpeedAbbrE2E")}
                          </span>
                          <span className="text-muted-foreground/80"> · </span>
                          <span className="font-mono tabular-nums">{overall.toFixed(1)}</span>
                          <span className="text-muted-foreground"> tok/s</span>
                        </p>
                        {generation != null ? (
                          <p title={t("detail.field.outputSpeedGeneration")}>
                            <span className="text-muted-foreground">
                              {t("detail.field.outputSpeedAbbrGen")}
                            </span>
                            <span className="text-muted-foreground/80"> · </span>
                            <span className="font-mono tabular-nums">{generation.toFixed(1)}</span>
                            <span className="text-muted-foreground"> tok/s</span>
                          </p>
                        ) : null}
                      </div>
                    )
                  })()}
                />
                {detail.totalAttempts > 1 && (
                  <DetailField
                    label={t("detail.field.attempts")}
                    value={`${detail.totalAttempts}`}
                  />
                )}
              </div>
            </div>
          </div>

          {/* Retry Timeline */}
          {detail.attempts && detail.attempts.length > 0 && (
            <div className="border-border/80 border-t pt-2">
              <div className="mb-3 flex items-center justify-between">
                <p className="text-muted-foreground text-xs font-semibold tracking-wide uppercase">
                  {t("detail.retry", { count: detail.attempts.length })}
                </p>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 gap-1 px-2 text-xs"
                  onClick={() => setRetryExpanded((prev) => !prev)}
                >
                  {retryExpanded ? t("messagesTab.collapse") : t("messagesTab.expand")}
                  {retryExpanded ? (
                    <ChevronUp className="h-3.5 w-3.5" />
                  ) : (
                    <ChevronDown className="h-3.5 w-3.5" />
                  )}
                </Button>
              </div>
              {retryExpanded && (
                <div className="border-border relative flex flex-col gap-3 border-l-2 pl-4">
                  {detail.attempts.map((attempt) => (
                    <div
                      key={`${attempt.channelId}-${attempt.modelName}-${attempt.attemptNum}`}
                      className="relative"
                    >
                      <div
                        className={`border-background absolute top-1 -left-[calc(0.5rem+1px)] h-3 w-3 rounded-full border ${
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
                          <span className="text-muted-foreground">
                            {t("retryTimeline.channel")}
                          </span>{" "}
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
              )}
            </div>
          )}

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
              onClick={() => handleReplay(detail.id)}
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

          <div className="border-border/80 border-t pt-2">
            <p className="text-muted-foreground mb-3 text-xs font-semibold tracking-wide uppercase">
              {t("detail.messages")}
            </p>
            <MessagesTabContent detail={detail} streamingOverlay={streamingOverlay} />
          </div>

          {replayResult !== null && (
            <div className="border-border/80 border-t pt-2">
              <p className="text-muted-foreground mb-3 text-xs font-semibold tracking-wide uppercase">
                {t("detail.replay")}
              </p>
              <CodeBlock label={t("detail.replayResult")} content={replayResult} />
            </div>
          )}
        </div>
      </div>
    </div>
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
          type="button"
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

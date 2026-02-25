import type { LogEntry } from "./columns"
import type { LogDetail, StreamingOverlay } from "./types"
import { Check, ChevronDown, ChevronUp, Copy, Loader2, Play } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import { toast } from "sonner"
import { ModelBadge } from "@/components/model-badge"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useModelMeta } from "@/hooks/use-model-meta"
import { formatCost, formatDuration } from "./columns"
import { CodeBlock } from "./detail/code-block"
import { MessagesTabContent } from "./detail/messages-tab"
import { useLogReplay } from "./log-replay"

// Hoisted constant array to avoid re-creation on every render
const DETAIL_SKELETON_ITEMS = Array.from({ length: 9 })

// ── Public component: LogDetailSheet ──

interface LogDetailSheetProps {
  detailId: number | null
  detailStreamId: string | null
  detailTab: string
  detail: LogDetail | null
  pendingStreams: Map<string, LogEntry>
  streamingOverlay: StreamingOverlay | null
  logs: LogEntry[]
  onClose: () => void
  onNavigate: (log: LogEntry) => void
  onTabChange: (tab: string) => void
}

export function LogDetailSheet({
  detailId,
  detailStreamId,
  detailTab,
  detail,
  pendingStreams,
  streamingOverlay,
  logs,
  onClose,
  onNavigate,
  onTabChange,
}: LogDetailSheetProps) {
  const { t } = useTranslation("logs")

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
              cost: 0,
              ftut: entry.ftut,
              useTime: entry.useTime,
              requestContent: entry._requestContent ?? "",
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
                onTabChange={onTabChange}
                streamingOverlay={streamingOverlay}
              />
            )
          })()
        ) : detail ? (
          <DetailPanel
            detail={detail}
            activeTab={detailTab}
            onTabChange={onTabChange}
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
  streamingOverlay: StreamingOverlay | null
}) {
  const { t } = useTranslation("logs")
  const { replayResult, replaying, handleReplay } = useLogReplay()

  const setActiveTab = onTabChange

  const isTruncated =
    /\[truncated,?\s*\d+\s*chars\s*total\]/.test(detail.requestContent) ||
    /\[\d+\s*messages?\s*omitted/.test(detail.requestContent) ||
    /\[image data omitted\]/.test(detail.requestContent)

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

          <div className="bg-muted/30 grid grid-cols-1 gap-4 rounded-xl border p-4 sm:grid-cols-2 md:grid-cols-3">
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
              onClick={() => handleReplay(detail.id, () => setActiveTab("replay"))}
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

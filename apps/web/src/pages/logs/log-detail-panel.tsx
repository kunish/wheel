import type { LogEntry } from "./columns"
import type { LogDetail, StreamingOverlay } from "./types"
import {
  AlertCircle,
  Bot,
  Braces,
  Brain,
  Check,
  ChevronDown,
  ChevronUp,
  Code2,
  Copy,
  Image,
  Loader2,
  MessageSquare,
  Play,
  Search,
  Settings2,
  User,
  Wrench,
  X,
} from "lucide-react"
import * as React from "react"
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import { toast } from "sonner"
import { ModelBadge } from "@/components/model-badge"
import { useTheme } from "@/components/theme-provider"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useModelMeta } from "@/hooks/use-model-meta"
import { formatCost, formatDuration } from "./columns"
import { countMatches } from "./log-filters"
import { useLogReplay } from "./log-replay"

// Hoisted constant array to avoid re-creation on every render
const DETAIL_SKELETON_ITEMS = Array.from({ length: 9 })

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
  streamingOverlay: { thinkingContent: string; responseContent: string } | null
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
      className={`rounded-md border border-l-4 ${config.borderClass} ${config.bgClass}${isContext ? "opacity-60" : ""}`}
    >
      {/* Header */}
      <div className="flex items-center gap-2 px-3 py-2">
        <div
          className={`border-border flex h-6 w-6 shrink-0 items-center justify-center rounded-md border ${config.avatarClass}`}
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
    <div className="border-l-nb-lime bg-nb-lime/15 dark:bg-nb-lime/10 rounded-md border border-l-4">
      <div className="flex items-center gap-2 px-3 py-2">
        <div className="border-border bg-nb-lime text-foreground flex h-6 w-6 shrink-0 items-center justify-center rounded-md border">
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
  const [collapsed, setCollapsed] = useState(true)
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
      <button className="flex w-full items-center gap-2" onClick={() => setCollapsed(!collapsed)}>
        <Wrench className="text-muted-foreground h-3 w-3 shrink-0" />
        <p className="text-muted-foreground text-[10px] font-bold tracking-wider uppercase">
          {t("messagesTab.tools")} ({tools.tools.length})
        </p>
        {toolChoiceLabel && (
          <Badge variant="secondary" className="text-[10px]">
            {t("messagesTab.toolChoice")}: {toolChoiceLabel}
          </Badge>
        )}
        <ChevronDown
          className={`text-muted-foreground ml-auto h-3 w-3 shrink-0 transition-transform ${!collapsed ? "rotate-180" : ""}`}
        />
      </button>
      {!collapsed && (
        <div className="mt-1.5 flex flex-col gap-1">
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
      )}
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
          <div className="border-border flex items-center rounded-md border">
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
              className={`border-border flex items-center gap-1 border-l px-2 py-1 text-xs font-bold transition-colors ${
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

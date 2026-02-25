import type { LogDetail, StreamingOverlay } from "../types"
import type {
  ParsedMessage,
  ParsedRequestParams,
  ParsedRequestTools,
  ParsedResponse,
  ParsedResponseChoice,
  ParsedResponseUsage,
} from "./message-parsers"
import {
  AlertCircle,
  Bot,
  Braces,
  Brain,
  ChevronDown,
  ChevronUp,
  Code2,
  Image,
  Loader2,
  MessageSquare,
  Settings2,
  User,
  Wrench,
} from "lucide-react"
import * as React from "react"
import { Suspense, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import { CollapsibleCodeBlock, LazyMarkdown } from "./code-block"
import {
  detectTruncation,
  findNewTurnBoundary,
  getMessageTextContent,
  parseMessages,
  parseRequestParams,
  parseRequestTools,
  parseResponseContent,
} from "./message-parsers"

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
    <div className="bg-background/60 rounded-lg border p-2 shadow-sm">
      <div className="flex items-center gap-1.5">
        <Code2 className="text-muted-foreground h-3 w-3 shrink-0" />
        <span className="font-mono text-xs font-bold">{toolCall.function.name}</span>
        <span className="text-muted-foreground font-mono text-[10px]">{toolCall.id}</span>
      </div>
      {args && args !== "{}" && (
        <div className="mt-1.5">
          <pre className="bg-muted/50 rounded-md border p-2 font-mono text-[11px] leading-relaxed break-words whitespace-pre-wrap">
            {args}
          </pre>
        </div>
      )}
    </div>
  )
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
      className={`rounded-xl border border-l-4 shadow-sm ${config.borderClass} ${config.bgClass}${isContext ? "opacity-60" : ""}`}
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
    <div className="border-l-nb-lime bg-nb-lime/15 dark:bg-nb-lime/10 rounded-xl border border-l-4 shadow-sm">
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
    <div className="bg-muted/30 rounded-xl border p-3 shadow-sm">
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
    <div className="bg-muted/30 rounded-xl border p-3 shadow-sm">
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
          {stops.map((s) => (
            <Badge key={s} variant="outline" className="font-mono text-[10px]">
              {s}
            </Badge>
          ))}
        </div>
      ),
    })
  }

  if (entries.length === 0) return null

  return (
    <div className="bg-muted/30 rounded-xl border p-3 shadow-sm">
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
  const [expandedTools, setExpandedTools] = useState<Set<number>>(() => new Set())

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
    <div className="bg-muted/30 rounded-xl border p-3 shadow-sm">
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
              <div key={tool.function.name} className="bg-background/60 rounded-md border">
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

export function MessagesTabContent({
  detail,
  streamingOverlay,
}: {
  detail: LogDetail
  streamingOverlay: StreamingOverlay | null
}) {
  const { t } = useTranslation("logs")
  const [viewMode, setViewMode] = useState<"conversation" | "raw">("conversation")

  const messages = useMemo(() => parseMessages(detail.requestContent), [detail.requestContent])
  const newTurnBoundary = useMemo(() => (messages ? findNewTurnBoundary(messages) : 0), [messages])
  const [contextExpanded, setContextExpanded] = useState(false)

  // Reset collapsed state when switching between log entries
  const prevDetailIdRef = useRef(detail.id)
  if (prevDetailIdRef.current !== detail.id) {
    prevDetailIdRef.current = detail.id
    setContextExpanded(false)
  }

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
          <div className="border-border flex items-center overflow-hidden rounded-lg border shadow-sm">
            <button
              className={`flex items-center gap-1 px-3 py-1.5 text-xs font-bold transition-colors ${
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
              className={`border-border flex items-center gap-1 border-l px-3 py-1.5 text-xs font-bold transition-colors ${
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

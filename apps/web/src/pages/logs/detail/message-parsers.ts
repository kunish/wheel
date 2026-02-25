// Pure parsing utilities for log detail message content.
// No React dependencies — these are plain functions operating on strings/JSON.

export interface ParsedMessage {
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

export interface ParsedRequestParams {
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

export interface ParsedRequestTools {
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

export interface ParsedResponseUsage {
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

export interface ParsedResponseChoice {
  assistantContent: string | null
  thinkingContent: string | null
  toolCalls: ParsedMessage["tool_calls"]
  finishReason: string | null
  index: number
}

export interface ParsedResponse {
  choices: ParsedResponseChoice[]
  id: string | null
  model: string | null
  created: number | null
  systemFingerprint: string | null
  usage: ParsedResponseUsage | null
  raw: unknown
}

/** Best-effort repair of truncated JSON (e.g. from log storage limits) */
export function repairTruncatedJson(text: string): { data: unknown; truncated: boolean } | null {
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

export function parseMessages(content: string): ParsedMessage[] | null {
  try {
    const parsed = JSON.parse(content)
    if (parsed?.messages && Array.isArray(parsed.messages)) {
      return parsed.messages as ParsedMessage[]
    }
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
export function findNewTurnBoundary(messages: ParsedMessage[]): number {
  let lastAssistantIdx = -1
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].role === "assistant") {
      lastAssistantIdx = i
      break
    }
  }
  return lastAssistantIdx === -1 ? 0 : lastAssistantIdx + 1
}

export function parseRequestParams(content: string): ParsedRequestParams | null {
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

export function parseRequestTools(content: string): ParsedRequestTools | null {
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

export function parseResponseContent(content: string): ParsedResponse | null {
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

export function getMessageTextContent(content: ParsedMessage["content"]): {
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

export function detectTruncation(text: string): {
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

  if (cleaned !== stripReplacementChars(cleaned)) {
    isTruncated = true
    cleaned = stripReplacementChars(cleaned)
  }

  cleaned = cleaned.trim()
  return { isTruncated, cleanText: cleaned, notice }
}

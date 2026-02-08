import { OutboundType } from "@wheel/core"

// ── Types ──────────────────────────────────────────────────────────

interface ChannelConfig {
  type: OutboundType
  baseUrls: { url: string; delay: number }[]
  customHeader: { key: string; value: string }[]
  paramOverride: string | null
}

interface UpstreamRequest {
  url: string
  headers: Record<string, string>
  body: string
}

// ── Helpers ────────────────────────────────────────────────────────

function selectBaseUrl(baseUrls: { url: string; delay: number }[]): string {
  if (baseUrls.length === 0) return "https://api.openai.com"
  let best = baseUrls[0]
  for (let i = 1; i < baseUrls.length; i++) {
    if (baseUrls[i].delay < best.delay) best = baseUrls[i]
  }
  return best.url.replace(/\/+$/, "")
}

/**
 * Build Anthropic API headers with x-api-key auth.
 */
function buildAnthropicHeaders(
  key: string,
  customHeaders: { key: string; value: string }[],
): Record<string, string> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "x-api-key": key,
    "anthropic-version": "2023-06-01",
  }
  for (const h of customHeaders) {
    headers[h.key] = h.value
  }
  return headers
}

/**
 * Apply JSON param overrides from channel config.
 */
function applyParamOverrides(body: Record<string, unknown>, paramOverride: string | null): void {
  if (!paramOverride) return
  try {
    Object.assign(body, JSON.parse(paramOverride) as Record<string, unknown>)
  } catch {
    /* ignore invalid JSON */
  }
}

const DEFAULT_THINKING_BUDGET = 10000

/**
 * Auto-inject thinking config for extended thinking models.
 * Models with "thinking" in the name require the `thinking` parameter
 * and `max_tokens > budget_tokens`.
 */
function ensureThinkingParams(body: Record<string, unknown>, model: string): void {
  if (!model.includes("thinking")) return
  if (body.thinking) return // already set by client or param override

  body.thinking = { type: "enabled", budget_tokens: DEFAULT_THINKING_BUDGET }

  // Ensure max_tokens > budget_tokens (Anthropic requirement)
  const maxTokens = (body.max_tokens as number) ?? 4096
  if (maxTokens <= DEFAULT_THINKING_BUDGET) {
    body.max_tokens = DEFAULT_THINKING_BUDGET + maxTokens
  }
}

// ── Request Builders ───────────────────────────────────────────────

/**
 * Build the upstream request based on channel type.
 * - OpenAI channels: passthrough with base URL swap
 * - Anthropic channels: convert OpenAI format → Anthropic Messages format
 */
export function buildUpstreamRequest(
  channel: ChannelConfig,
  key: string,
  inboundBody: Record<string, unknown>,
  inboundPath: string,
  model: string,
  anthropicPassthrough = false,
): UpstreamRequest {
  const baseUrl = selectBaseUrl(channel.baseUrls)

  switch (channel.type) {
    case OutboundType.Anthropic:
      return anthropicPassthrough
        ? buildAnthropicPassthroughRequest(baseUrl, key, inboundBody, model, channel)
        : buildAnthropicRequest(baseUrl, key, inboundBody, model, channel)
    case OutboundType.OpenAI:
    case OutboundType.OpenAIChat:
    case OutboundType.OpenAIEmbedding:
    case OutboundType.Gemini:
    case OutboundType.Volcengine:
    default:
      return buildOpenAIRequest(baseUrl, key, inboundBody, inboundPath, model, channel)
  }
}

function buildOpenAIRequest(
  baseUrl: string,
  key: string,
  body: Record<string, unknown>,
  inboundPath: string,
  model: string,
  channel: ChannelConfig,
): UpstreamRequest {
  const path = inboundPath.includes("/embeddings") ? "/v1/embeddings" : "/v1/chat/completions"

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${key}`,
  }
  for (const h of channel.customHeader) {
    headers[h.key] = h.value
  }

  const outBody = { ...body, model }
  applyParamOverrides(outBody, channel.paramOverride)

  return { url: `${baseUrl}${path}`, headers, body: JSON.stringify(outBody) }
}

/**
 * Passthrough: client sends Anthropic format → forward to Anthropic upstream.
 * Only overrides model, injects thinking params, and applies param overrides.
 */
function buildAnthropicPassthroughRequest(
  baseUrl: string,
  key: string,
  body: Record<string, unknown>,
  model: string,
  channel: ChannelConfig,
): UpstreamRequest {
  const headers = buildAnthropicHeaders(key, channel.customHeader)
  const outBody = { ...body, model }

  ensureThinkingParams(outBody, model)
  applyParamOverrides(outBody, channel.paramOverride)

  return { url: `${baseUrl}/v1/messages`, headers, body: JSON.stringify(outBody) }
}

/**
 * Convert: client sends OpenAI format → convert to Anthropic Messages format.
 * Handles system messages, tool_calls, tool results, and all params.
 */
function buildAnthropicRequest(
  baseUrl: string,
  key: string,
  body: Record<string, unknown>,
  model: string,
  channel: ChannelConfig,
): UpstreamRequest {
  const headers = buildAnthropicHeaders(key, channel.customHeader)

  // Convert OpenAI-style messages to Anthropic format
  const messages = (body.messages as Array<{ role: string; content: unknown }>) ?? []
  let system: string | undefined
  const anthropicMessages: Array<{ role: string; content: unknown }> = []

  for (const msg of messages) {
    if (msg.role === "system") {
      system = typeof msg.content === "string" ? msg.content : JSON.stringify(msg.content)
    } else if (msg.role === "assistant") {
      anthropicMessages.push(convertAssistantMessage(msg))
    } else if (msg.role === "tool") {
      anthropicMessages.push(convertToolResultMessage(msg))
    } else {
      anthropicMessages.push({ role: "user", content: msg.content })
    }
  }

  const anthropicBody: Record<string, unknown> = {
    model,
    messages: anthropicMessages,
    max_tokens: (body.max_tokens as number) ?? (body.maxTokens as number) ?? 4096,
  }

  if (system) anthropicBody.system = system
  if (body.stream) anthropicBody.stream = true
  if (body.temperature !== undefined) anthropicBody.temperature = body.temperature
  if (body.top_p !== undefined) anthropicBody.top_p = body.top_p
  if (body.stop) anthropicBody.stop_sequences = body.stop

  ensureThinkingParams(anthropicBody, model)

  // Convert OpenAI tools to Anthropic tools format
  if (body.tools) {
    anthropicBody.tools = convertOpenAITools(
      body.tools as Array<{
        type: string
        function?: { name: string; description?: string; parameters?: unknown }
      }>,
    )
  }

  applyParamOverrides(anthropicBody, channel.paramOverride)

  return { url: `${baseUrl}/v1/messages`, headers, body: JSON.stringify(anthropicBody) }
}

// ── Message Converters ─────────────────────────────────────────────

/**
 * Convert OpenAI assistant message (possibly with tool_calls) to Anthropic format.
 */
function convertAssistantMessage(msg: Record<string, unknown>): { role: string; content: unknown } {
  const toolCalls = msg.tool_calls as
    | Array<{
        id: string
        type: string
        function: { name: string; arguments: string }
      }>
    | undefined

  if (toolCalls && toolCalls.length > 0) {
    const contentBlocks: unknown[] = []
    if (typeof msg.content === "string" && msg.content) {
      contentBlocks.push({ type: "text", text: msg.content })
    }
    for (const tc of toolCalls) {
      let input: unknown = {}
      try {
        input = JSON.parse(tc.function.arguments)
      } catch {
        /* use empty */
      }
      contentBlocks.push({
        type: "tool_use",
        id: tc.id,
        name: tc.function.name,
        input,
      })
    }
    return { role: "assistant", content: contentBlocks }
  }

  return { role: "assistant", content: msg.content }
}

/**
 * Convert OpenAI tool result message to Anthropic tool_result format.
 */
function convertToolResultMessage(msg: Record<string, unknown>): {
  role: string
  content: unknown
} {
  return {
    role: "user",
    content: [
      {
        type: "tool_result",
        tool_use_id: msg.tool_call_id as string,
        content: typeof msg.content === "string" ? msg.content : JSON.stringify(msg.content),
      },
    ],
  }
}

/**
 * Convert OpenAI tools array to Anthropic tools format.
 */
function convertOpenAITools(
  tools: Array<{
    type: string
    function?: { name: string; description?: string; parameters?: unknown }
  }>,
): Array<{ name: string; description: string; input_schema: unknown }> {
  return tools
    .filter((t) => t.type === "function" && t.function)
    .map((t) => ({
      name: t.function!.name,
      description: t.function!.description ?? "",
      input_schema: t.function!.parameters ?? { type: "object", properties: {} },
    }))
}

// ── Response Converters ────────────────────────────────────────────

/**
 * Convert an Anthropic response back to OpenAI format for the client.
 */
export function convertAnthropicResponse(
  anthropicResp: Record<string, unknown>,
): Record<string, unknown> {
  const content = anthropicResp.content as Array<{
    type: string
    text?: string
    id?: string
    name?: string
    input?: Record<string, unknown>
  }>

  const text =
    content
      ?.filter((c) => c.type === "text")
      .map((c) => c.text)
      .join("") ?? ""

  const toolUseBlocks = content?.filter((c) => c.type === "tool_use") ?? []
  const toolCalls = toolUseBlocks.map((block, idx) => ({
    index: idx,
    id: block.id ?? `call_${crypto.randomUUID()}`,
    type: "function" as const,
    function: {
      name: block.name ?? "",
      arguments: JSON.stringify(block.input ?? {}),
    },
  }))

  const message: Record<string, unknown> = {
    role: "assistant",
    content: text || (toolCalls.length > 0 ? null : ""),
  }
  if (toolCalls.length > 0) {
    message.tool_calls = toolCalls
  }

  const usage = anthropicResp.usage as
    | {
        input_tokens?: number
        output_tokens?: number
      }
    | undefined

  return {
    id: anthropicResp.id ?? `chatcmpl-${crypto.randomUUID()}`,
    object: "chat.completion",
    created: Math.floor(Date.now() / 1000),
    model: anthropicResp.model,
    choices: [
      {
        index: 0,
        message,
        finish_reason: mapAnthropicStopReason(anthropicResp.stop_reason as string),
      },
    ],
    usage: {
      prompt_tokens: usage?.input_tokens ?? 0,
      completion_tokens: usage?.output_tokens ?? 0,
      total_tokens: (usage?.input_tokens ?? 0) + (usage?.output_tokens ?? 0),
    },
  }
}

function mapAnthropicStopReason(reason: string | null | undefined): string {
  switch (reason) {
    case "end_turn":
    case "stop_sequence":
      return "stop"
    case "max_tokens":
      return "length"
    case "tool_use":
      return "tool_calls"
    default:
      return "stop"
  }
}

/**
 * Convert an OpenAI chat completion response to Anthropic Messages format.
 * Used when inbound request is Anthropic-native (e.g. Claude Code).
 */
export function convertToAnthropicResponse(
  openaiResp: Record<string, unknown>,
): Record<string, unknown> {
  const choices = openaiResp.choices as
    | Array<{
        message?: { role?: string; content?: string }
        finish_reason?: string
      }>
    | undefined

  const firstChoice = choices?.[0]
  const content = firstChoice?.message?.content ?? ""

  const usage = openaiResp.usage as
    | {
        prompt_tokens?: number
        completion_tokens?: number
      }
    | undefined

  return {
    id: openaiResp.id ?? `msg_${crypto.randomUUID()}`,
    type: "message",
    role: "assistant",
    model: openaiResp.model,
    content: [{ type: "text", text: content }],
    stop_reason: mapOpenAIFinishReason(firstChoice?.finish_reason),
    stop_sequence: null,
    usage: {
      input_tokens: usage?.prompt_tokens ?? 0,
      output_tokens: usage?.completion_tokens ?? 0,
    },
  }
}

function mapOpenAIFinishReason(reason: string | undefined): string {
  switch (reason) {
    case "stop":
      return "end_turn"
    case "length":
      return "max_tokens"
    case "tool_calls":
      return "tool_use"
    default:
      return "end_turn"
  }
}

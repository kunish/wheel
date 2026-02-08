import { OutboundType } from "@wheel/core"
import { convertAnthropicResponse } from "./adapter"

// ── Types ──────────────────────────────────────────────────────────

export interface StreamCompleteInfo {
  inputTokens: number
  outputTokens: number
  cacheReadTokens: number
  cacheCreationTokens: number
  firstTokenTime: number
  statusCode: number
  responseContent: string
}

export class ProxyError extends Error {
  public retryAfterMs: number
  constructor(
    message: string,
    public statusCode: number,
    retryAfterMs?: number,
  ) {
    super(message)
    this.name = "ProxyError"
    this.retryAfterMs = retryAfterMs ?? 0
  }
}

interface ProxyResult {
  response: Record<string, unknown>
  inputTokens: number
  outputTokens: number
  cacheReadTokens: number
  cacheCreationTokens: number
  statusCode: number
}

interface AnthropicSSEResult {
  done: boolean
  data?: Record<string, unknown>
  usage?: { input_tokens?: number; output_tokens?: number }
  cacheReadTokens?: number
  cacheCreationTokens?: number
}

// ── Helpers ────────────────────────────────────────────────────────

/**
 * Extract cache token counts from an upstream response's usage object.
 */
function extractCacheTokens(
  data: Record<string, unknown>,
  channelType: OutboundType,
): { cacheRead: number; cacheCreation: number } {
  if (channelType === OutboundType.Anthropic) {
    const usage = (data.usage as Record<string, number>) ?? {}
    return {
      cacheRead: usage.cache_read_input_tokens ?? 0,
      cacheCreation: usage.cache_creation_input_tokens ?? 0,
    }
  }
  // OpenAI: prompt_tokens_details.cached_tokens
  const usage = data.usage as Record<string, unknown> | undefined
  const details = usage?.prompt_tokens_details as Record<string, number> | undefined
  return {
    cacheRead: details?.cached_tokens ?? 0,
    cacheCreation: 0,
  }
}

/**
 * Parse retry delay from response headers or body.
 * Supports Retry-After header (seconds) and Google Cloud quotaResetDelay.
 */
function parseRetryDelay(resp: Response, body: string): number {
  // 1. Check Retry-After header (seconds)
  const retryAfter = resp.headers.get("retry-after")
  if (retryAfter) {
    const secs = Number.parseFloat(retryAfter)
    if (!Number.isNaN(secs) && secs > 0) return Math.ceil(secs * 1000)
  }

  // 2. Parse quotaResetDelay from Google Cloud error body (e.g. "1.335s", "123.5ms")
  const match = body.match(/quotaResetDelay["\s:]+["']?([\d.]+)(ms|s)/i)
  if (match) {
    const val = Number.parseFloat(match[1])
    if (!Number.isNaN(val)) {
      return match[2] === "s" ? Math.ceil(val * 1000) : Math.ceil(val)
    }
  }

  return 0
}

/**
 * Mark first token as received; clears timeout and resolves the promise.
 */
function markFirstToken(state: StreamingState, startTime: number): void {
  if (state.firstTokenReceived) return
  state.firstTokenReceived = true
  state.firstTokenTime = Date.now() - startTime
  if (state.timeoutId) {
    clearTimeout(state.timeoutId)
    state.timeoutId = null
  }
  state.resolveFirstChunk()
}

const MAX_RESPONSE_CONTENT = 10000

/**
 * Append text content to the streaming state's responseContent buffer.
 * Stops accumulating once the buffer reaches MAX_RESPONSE_CONTENT characters.
 */
function appendContent(state: StreamingState, text: string): void {
  if (state.responseContent.length >= MAX_RESPONSE_CONTENT) return
  // Trim leading whitespace from the very first chunk
  const chunk = state.responseContent.length === 0 ? text.trimStart() : text
  if (!chunk) return
  state.responseContent += chunk
  if (state.responseContent.length > MAX_RESPONSE_CONTENT) {
    state.responseContent = state.responseContent.slice(0, MAX_RESPONSE_CONTENT)
  }
}

interface StreamingState {
  firstTokenReceived: boolean
  firstTokenTime: number
  inputTokens: number
  outputTokens: number
  cacheReadTokens: number
  cacheCreationTokens: number
  timeoutId: ReturnType<typeof setTimeout> | null
  resolveFirstChunk: () => void
  rejectFirstChunk: (err: Error) => void
  responseContent: string
}

// ── Non-Streaming Proxy ────────────────────────────────────────────

/**
 * Non-streaming proxy: single HTTP fetch, no retry.
 * Non-2xx responses throw ProxyError for handler-level retry.
 */
export async function proxyNonStreaming(
  upstreamUrl: string,
  upstreamHeaders: Record<string, string>,
  upstreamBody: string,
  channelType: OutboundType,
  passthrough = false,
): Promise<ProxyResult> {
  const resp = await fetch(upstreamUrl, {
    method: "POST",
    headers: upstreamHeaders,
    body: upstreamBody,
  })

  const statusCode = resp.status

  if (!resp.ok) {
    const errorText = await resp.text()
    throw new ProxyError(
      `Upstream error ${statusCode}: ${errorText.slice(0, 500)}`,
      statusCode,
      parseRetryDelay(resp, errorText),
    )
  }

  const data = (await resp.json()) as Record<string, unknown>
  const { cacheRead: cacheReadTokens, cacheCreation: cacheCreationTokens } = extractCacheTokens(
    data,
    channelType,
  )

  // Passthrough mode: return raw Anthropic response without conversion
  if (passthrough && channelType === OutboundType.Anthropic) {
    const usage = (data.usage as Record<string, number>) ?? {}
    return {
      response: data,
      inputTokens: usage.input_tokens ?? 0,
      outputTokens: usage.output_tokens ?? 0,
      cacheReadTokens,
      cacheCreationTokens,
      statusCode,
    }
  }

  // Convert Anthropic → OpenAI if needed
  const finalResponse =
    channelType === OutboundType.Anthropic ? convertAnthropicResponse(data) : data

  const usage = finalResponse.usage as
    | {
        prompt_tokens?: number
        completion_tokens?: number
      }
    | undefined

  return {
    response: finalResponse,
    inputTokens: usage?.prompt_tokens ?? 0,
    outputTokens: usage?.completion_tokens ?? 0,
    cacheReadTokens,
    cacheCreationTokens,
    statusCode,
  }
}

// ── Streaming Proxy ────────────────────────────────────────────────

/**
 * Streaming proxy: forward SSE events through a TransformStream.
 * Handles protocol conversion for Anthropic SSE → OpenAI SSE format.
 * Single fetch — no retry at proxy level.
 *
 * highWaterMark is set to 64KB to prevent backpressure deadlock:
 * The handler waits on firstChunkPromise before returning Response(readable).
 * Without buffer space, writer.write() blocks before we can parse
 * enough events to detect the first token and resolve firstChunkPromise.
 */
export function proxyStreaming(
  upstreamUrl: string,
  upstreamHeaders: Record<string, string>,
  upstreamBody: string,
  channelType: OutboundType,
  firstTokenTimeout: number,
  onComplete: (info: StreamCompleteInfo) => void,
  passthrough = false,
): { readable: ReadableStream; firstChunkPromise: Promise<void>; fetchPromise: Promise<number> } {
  const HWM = 64 * 1024
  const { readable, writable } = new TransformStream(
    undefined,
    { highWaterMark: HWM },
    { highWaterMark: HWM },
  )
  const writer = writable.getWriter()
  const encoder = new TextEncoder()
  const startTime = Date.now()

  let resolveFirstChunk!: () => void
  let rejectFirstChunk!: (err: Error) => void
  const firstChunkPromise = new Promise<void>((resolve, reject) => {
    resolveFirstChunk = resolve
    rejectFirstChunk = reject
  })

  const state: StreamingState = {
    firstTokenReceived: false,
    firstTokenTime: 0,
    inputTokens: 0,
    outputTokens: 0,
    cacheReadTokens: 0,
    cacheCreationTokens: 0,
    timeoutId: null,
    resolveFirstChunk,
    rejectFirstChunk,
    responseContent: "",
  }

  let statusCode = 0

  const fetchPromise = (async (): Promise<number> => {
    const convertChunk =
      !passthrough && channelType === OutboundType.Anthropic ? createAnthropicSSEConverter() : null

    try {
      const resp = await fetch(upstreamUrl, {
        method: "POST",
        headers: upstreamHeaders,
        body: upstreamBody,
      })

      statusCode = resp.status

      if (!resp.ok) {
        const errorText = await resp.text()
        const err = new ProxyError(
          `Upstream error ${statusCode}: ${errorText.slice(0, 500)}`,
          statusCode,
          parseRetryDelay(resp, errorText),
        )
        rejectFirstChunk(err)
        throw err
      }

      if (!resp.body) {
        const err = new ProxyError("Upstream returned empty body", 502)
        rejectFirstChunk(err)
        throw err
      }

      // Set up first token timeout
      if (firstTokenTimeout > 0) {
        state.timeoutId = setTimeout(() => {
          if (!state.firstTokenReceived) {
            const err = new ProxyError("First token timeout exceeded", 504)
            rejectFirstChunk(err)
            writer.close().catch(() => {})
          }
        }, firstTokenTimeout * 1000)
      }

      const reader = resp.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ""

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split("\n")
        buffer = lines.pop() ?? ""

        for (const line of lines) {
          if (!line.trim()) {
            await writer.write(encoder.encode("\n"))
            continue
          }

          if (passthrough && channelType === OutboundType.Anthropic) {
            processAnthropicPassthrough(line, state, startTime)
            await writer.write(encoder.encode(`${line}\n`))
          } else if (channelType === OutboundType.Anthropic && convertChunk) {
            await processAnthropicConverted(line, convertChunk, state, startTime, writer, encoder)
          } else if (line.startsWith("event:")) {
            // Anthropic event type line — skip (handled via data line)
            continue
          } else {
            await processOpenAI(line, state, startTime, writer, encoder)
          }
        }
      }

      // Flush remaining buffer
      if (buffer.trim()) {
        await writer.write(encoder.encode(`${buffer}\n`))
      }
    } catch (err) {
      if (state.timeoutId) {
        clearTimeout(state.timeoutId)
        state.timeoutId = null
      }
      // Write error as SSE event before closing
      const errorMsg = err instanceof ProxyError ? err.message : "Internal proxy error"
      try {
        await writer.write(
          encoder.encode(
            `data: ${JSON.stringify({ error: { message: errorMsg, type: "proxy_error" } })}\n\n`,
          ),
        )
      } catch {
        /* writer may be closed */
      }

      throw err
    } finally {
      try {
        await writer.close()
      } catch {
        /* may already be closed */
      }

      onComplete({
        inputTokens: state.inputTokens,
        outputTokens: state.outputTokens,
        cacheReadTokens: state.cacheReadTokens,
        cacheCreationTokens: state.cacheCreationTokens,
        firstTokenTime: state.firstTokenTime,
        statusCode,
        responseContent: state.responseContent,
      })
    }

    return statusCode
  })()

  return { readable, firstChunkPromise, fetchPromise }
}

// ── SSE Line Processors ────────────────────────────────────────────

/**
 * Process a single SSE line in Anthropic passthrough mode.
 * Extracts usage/cache info while forwarding the line unchanged.
 */
function processAnthropicPassthrough(line: string, state: StreamingState, startTime: number): void {
  if (!line.startsWith("data: ") || line === "data: [DONE]") return

  try {
    const ev = JSON.parse(line.slice(6)) as Record<string, unknown>

    // Resolve firstChunkPromise on first meaningful event
    if (!state.firstTokenReceived) {
      const t = ev.type as string
      if (t === "message_start" || t === "content_block_start" || t === "content_block_delta") {
        markFirstToken(state, startTime)
      }
    }

    if (ev.type === "message_start") {
      const message = ev.message as Record<string, unknown> | undefined
      const msgUsage = (message?.usage as Record<string, number>) ?? {}
      state.cacheReadTokens = msgUsage.cache_read_input_tokens ?? 0
      state.cacheCreationTokens = msgUsage.cache_creation_input_tokens ?? 0
    }

    if (ev.type === "message_delta") {
      const usage = ev.usage as { input_tokens?: number; output_tokens?: number } | undefined
      if (usage) {
        state.inputTokens = usage.input_tokens ?? state.inputTokens
        state.outputTokens = usage.output_tokens ?? state.outputTokens
      }
    }

    // Accumulate text content for log storage
    if (ev.type === "content_block_delta") {
      const delta = ev.delta as { type?: string; text?: string } | undefined
      if (delta?.type === "text_delta" && delta.text) {
        appendContent(state, delta.text)
      }
    }
  } catch {
    /* ignore parse errors */
  }
}

/**
 * Process a single SSE line by converting Anthropic SSE → OpenAI SSE.
 */
async function processAnthropicConverted(
  line: string,
  convertChunk: (json: string) => AnthropicSSEResult | null,
  state: StreamingState,
  startTime: number,
  writer: WritableStreamDefaultWriter,
  encoder: TextEncoder,
): Promise<void> {
  if (!line.startsWith("data: ")) return

  const chunk = convertChunk(line.slice(6))
  if (!chunk) return

  markFirstToken(state, startTime)

  if (chunk.cacheReadTokens !== undefined) {
    state.cacheReadTokens = chunk.cacheReadTokens
  }
  if (chunk.cacheCreationTokens !== undefined) {
    state.cacheCreationTokens = chunk.cacheCreationTokens
  }

  if (chunk.done) {
    if (chunk.usage) {
      state.inputTokens = chunk.usage.input_tokens ?? 0
      state.outputTokens = chunk.usage.output_tokens ?? 0
    }
    await writer.write(encoder.encode("data: [DONE]\n\n"))
  } else if (chunk.data) {
    // Accumulate text content for log storage
    const choices = chunk.data.choices as Array<{ delta?: { content?: string } }> | undefined
    if (choices?.[0]?.delta?.content) {
      appendContent(state, choices[0].delta.content)
    }
    await writer.write(encoder.encode(`data: ${JSON.stringify(chunk.data)}\n\n`))
  }
}

/**
 * Process a single SSE line in OpenAI passthrough mode.
 * Extracts usage info while forwarding the line unchanged.
 */
async function processOpenAI(
  line: string,
  state: StreamingState,
  startTime: number,
  writer: WritableStreamDefaultWriter,
  encoder: TextEncoder,
): Promise<void> {
  if (line.startsWith("data: ") && line !== "data: [DONE]") {
    markFirstToken(state, startTime)

    // Extract usage from OpenAI final chunks
    try {
      const obj = JSON.parse(line.slice(6))
      if (obj.usage) {
        state.inputTokens = obj.usage.prompt_tokens ?? state.inputTokens
        state.outputTokens = obj.usage.completion_tokens ?? state.outputTokens
        const details = obj.usage.prompt_tokens_details
        if (details?.cached_tokens) {
          state.cacheReadTokens = details.cached_tokens
        }
      }
      // Accumulate text content for log storage
      const delta = obj.choices?.[0]?.delta
      if (delta?.content) {
        appendContent(state, delta.content)
      }
    } catch {
      /* ignore */
    }
  }

  await writer.write(encoder.encode(`${line}\n`))
}

// ── Anthropic SSE Converter ────────────────────────────────────────

function mapStopReason(reason: string | undefined): string | null {
  switch (reason) {
    case "end_turn":
    case "stop_sequence":
      return "stop"
    case "max_tokens":
      return "length"
    default:
      return null
  }
}

/**
 * Stateful converter: Anthropic SSE → OpenAI SSE.
 * Tracks content blocks (text, tool_use, thinking) and maps them to
 * OpenAI chat.completion.chunk format with tool_calls support.
 *
 * Thinking blocks are silently dropped (not part of OpenAI format).
 */
function createAnthropicSSEConverter() {
  let msgId = `chatcmpl-${crypto.randomUUID()}`
  let msgModel = ""
  let toolCallIndex = 0
  const blockMap = new Map<number, { type: string; toolCallIdx?: number }>()

  function makeChunk(
    choices: Record<string, unknown>[],
    extra?: Partial<AnthropicSSEResult>,
  ): AnthropicSSEResult {
    return {
      done: false,
      data: {
        id: msgId,
        object: "chat.completion.chunk",
        created: Math.floor(Date.now() / 1000),
        model: msgModel,
        choices,
      },
      ...extra,
    }
  }

  return function convert(jsonStr: string): AnthropicSSEResult | null {
    try {
      const event = JSON.parse(jsonStr) as Record<string, unknown>
      const type = event.type as string

      switch (type) {
        case "message_start": {
          const message = event.message as Record<string, unknown>
          const msgUsage = (message?.usage as Record<string, number>) ?? {}
          msgId = (message?.id as string) ?? msgId
          msgModel = (message?.model as string) ?? ""
          return makeChunk(
            [{ index: 0, delta: { role: "assistant", content: "" }, finish_reason: null }],
            {
              cacheReadTokens: msgUsage.cache_read_input_tokens,
              cacheCreationTokens: msgUsage.cache_creation_input_tokens,
            },
          )
        }

        case "content_block_start": {
          const idx = event.index as number
          const block = event.content_block as { type: string; id?: string; name?: string }
          if (!block) return null

          if (block.type === "tool_use") {
            const tcIdx = toolCallIndex++
            blockMap.set(idx, { type: "tool_use", toolCallIdx: tcIdx })
            return makeChunk([
              {
                index: 0,
                delta: {
                  tool_calls: [
                    {
                      index: tcIdx,
                      id: block.id,
                      type: "function",
                      function: { name: block.name, arguments: "" },
                    },
                  ],
                },
                finish_reason: null,
              },
            ])
          }

          blockMap.set(idx, { type: block.type })
          return null
        }

        case "content_block_delta": {
          const idx = event.index as number
          const delta = event.delta as { type: string; text?: string; partial_json?: string }
          if (!delta) return null

          if (delta.type === "text_delta") {
            return makeChunk([
              {
                index: 0,
                delta: { content: delta.text ?? "" },
                finish_reason: null,
              },
            ])
          }

          if (delta.type === "input_json_delta") {
            const info = blockMap.get(idx)
            if (info?.type === "tool_use") {
              return makeChunk([
                {
                  index: 0,
                  delta: {
                    tool_calls: [
                      {
                        index: info.toolCallIdx!,
                        function: { arguments: delta.partial_json ?? "" },
                      },
                    ],
                  },
                  finish_reason: null,
                },
              ])
            }
          }

          // thinking_delta — silently drop
          return null
        }

        case "message_delta": {
          const delta = event.delta as { stop_reason?: string }
          const usage = event.usage as { input_tokens?: number; output_tokens?: number }
          const stopReason = delta?.stop_reason
          const finishReason = stopReason === "tool_use" ? "tool_calls" : mapStopReason(stopReason)
          return {
            done: false,
            data: {
              id: msgId,
              object: "chat.completion.chunk",
              created: Math.floor(Date.now() / 1000),
              model: msgModel,
              choices: [{ index: 0, delta: {}, finish_reason: finishReason }],
            },
            usage,
          }
        }

        case "message_stop":
          return { done: true }

        // ping, content_block_stop — no output needed
        default:
          return null
      }
    } catch {
      return null
    }
  }
}

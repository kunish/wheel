import type { AnthropicModel, AttemptStatus, GroupMode, OpenAIModel } from "@wheel/core"
import type { AppBindings, Database, IKVStore, RunBackground } from "../runtime/types"
import type { StreamCompleteInfo } from "./proxy"
import { OutboundType } from "@wheel/core"
import { Hono } from "hono"
import { incrementApiKeyCost } from "../db/dal/apikeys"
import { incrementChannelKeyCost, listChannels, updateChannelKeyStatus } from "../db/dal/channels"
import { listGroups } from "../db/dal/groups"
import { cleanupOldLogs, createLog } from "../db/dal/logs"
import { getSetting } from "../db/dal/settings"
import { apiKeyAuth, checkModelAccess } from "../middleware/apikey"
import { broadcast } from "../ws/hub"
import { buildUpstreamRequest, convertToAnthropicResponse } from "./adapter"
import { selectChannelOrder } from "./balancer"
import { getCooldownConfig, isTripped, recordFailure, recordSuccess } from "./circuit"
import { selectKey } from "./key-selector"
import { matchGroup } from "./matcher"
import { detectRequestType, extractModel } from "./parser"
import { calculateCost } from "./pricing"
import { ProxyError, proxyNonStreaming, proxyStreaming } from "./proxy"
import { getSticky, setSticky } from "./session"

interface Env {
  Bindings: AppBindings
  Variables: {
    apiKeyId: number
    supportedModels: string
    runBackground: RunBackground
  }
}

const MAX_RETRY_ROUNDS = 3

/**
 * Maximum length for a single message content field in stored logs.
 * Messages longer than this are truncated with a marker.
 */
const MAX_MESSAGE_CONTENT = 500
const MAX_LOG_JSON = 10000

/**
 * Truncate a single message's content at the object level.
 */
function truncateMessage(msg: Record<string, unknown>): Record<string, unknown> {
  const m = { ...msg }
  if (typeof m.content === "string" && m.content.length > MAX_MESSAGE_CONTENT) {
    m.content = `${m.content.slice(0, MAX_MESSAGE_CONTENT)}... [truncated, ${m.content.length} chars total]`
  } else if (Array.isArray(m.content)) {
    m.content = (m.content as Array<Record<string, unknown>>).map((part) => {
      const p = { ...part }
      if (p.type === "image_url" || p.type === "image") {
        return { type: p.type, _omitted: "[image data omitted]" }
      }
      if (typeof p.text === "string" && (p.text as string).length > MAX_MESSAGE_CONTENT) {
        p.text = `${(p.text as string).slice(0, MAX_MESSAGE_CONTENT)}... [truncated]`
      }
      return p
    })
  }
  return m
}

/**
 * Prepare request body for log storage.
 * Truncates individual message contents to keep the overall JSON valid and compact.
 * If still too large, keeps only the last N messages + a summary.
 */
function truncateForLog(body: Record<string, unknown>): string {
  try {
    const clone = { ...body }
    if (Array.isArray(clone.messages)) {
      const msgs = clone.messages as Array<Record<string, unknown>>
      clone.messages = msgs.map(truncateMessage)

      // If still too large, progressively drop older messages
      let json = JSON.stringify(clone)
      if (json.length > MAX_LOG_JSON && msgs.length > 2) {
        const truncatedMsgs = clone.messages as Array<Record<string, unknown>>
        // Keep first (system) + last few messages
        const keep = Math.max(2, Math.min(truncatedMsgs.length, 4))
        const dropped = truncatedMsgs.length - keep
        clone.messages = [
          truncatedMsgs[0],
          { role: "system", content: `[${dropped} messages omitted for storage]` },
          ...truncatedMsgs.slice(-keep + 1),
        ]
        json = JSON.stringify(clone)
      }
      return json
    }
    return JSON.stringify(clone)
  } catch {
    return JSON.stringify(body).slice(0, MAX_LOG_JSON)
  }
}

const relayRoutes = new Hono<Env>()

// Apply API Key auth to all relay routes
relayRoutes.use("/*", apiKeyAuth())

// GET /v1/models - Return available models in OpenAI or Anthropic format
relayRoutes.get("/models", async (c) => {
  const db = c.env.DB
  const allGroups = await loadGroups(c.env.CACHE, db)

  // Collect unique model names from all groups
  const modelSet = new Set<string>()
  for (const group of allGroups) {
    modelSet.add(group.name)
  }

  // Filter by API Key's supportedModels whitelist
  const supportedModels = c.get("supportedModels")
  let models = Array.from(modelSet).sort()
  if (supportedModels) {
    const allowed = supportedModels.split(",").map((m) => m.trim())
    models = models.filter((m) => allowed.includes(m))
  }

  // Detect format: Anthropic if x-api-key header or anthropic-version header present
  const isAnthropic =
    c.req.header("anthropic-version") !== undefined ||
    (c.req.header("x-api-key") !== undefined && !c.req.header("Authorization"))

  if (isAnthropic) {
    const data: AnthropicModel[] = models.map((id) => ({
      id,
      created_at: new Date().toISOString(),
      display_name: id,
      type: "model" as const,
    }))
    return c.json({ data, has_more: false })
  }

  const now = Math.floor(Date.now() / 1000)
  const data: OpenAIModel[] = models.map((id) => ({
    id,
    object: "model" as const,
    created: now,
    owned_by: "wheel",
  }))
  return c.json({ object: "list", data })
})

// Catch-all for /v1/* relay paths
relayRoutes.post("/*", async (c) => {
  const startTime = Date.now()
  const path = c.req.path

  // 6.1 Parse request type
  const requestType = detectRequestType(path)
  if (!requestType) {
    return c.json(
      { error: { message: "Unsupported endpoint", type: "invalid_request_error" } },
      400,
    )
  }

  // Extract model and body
  const { model, body, stream } = await extractModel(c.req.raw.clone(), requestType)
  if (!model) {
    return c.json({ error: { message: "Model is required", type: "invalid_request_error" } }, 400)
  }

  // Check model access against API Key whitelist
  const supportedModels = c.get("supportedModels")
  if (!checkModelAccess(supportedModels, model)) {
    return c.json(
      {
        error: {
          message: `Model '${model}' not allowed for this API key`,
          type: "invalid_request_error",
        },
      },
      403,
    )
  }

  // Determine if client expects Anthropic-native response format
  const isAnthropicInbound = requestType === "anthropic-messages"

  // Load channels and groups (from KV cache or DB)
  const db = c.env.DB
  const [allChannels, allGroups] = await Promise.all([
    loadChannels(c.env.CACHE, db),
    loadGroups(c.env.CACHE, db),
  ])

  // 6.2 Match group
  const group = matchGroup(model, allGroups)
  if (!group || group.items.length === 0) {
    return c.json(
      { error: { message: `No group matches model '${model}'`, type: "invalid_request_error" } },
      404,
    )
  }

  // 6.3 Select channel order based on load balancing
  const orderedItems = selectChannelOrder(group.mode as GroupMode, group.items, group.id)

  // Build channel lookup map
  const channelMap = new Map(allChannels.map((ch) => [ch.id, ch]))

  // Attempt tracking
  interface AttemptRecord {
    channelId: number
    channelKeyId?: number
    channelName: string
    modelName: string
    attemptNum: number
    status: AttemptStatus
    duration: number
    sticky?: boolean
    msg?: string
  }
  const attempts: AttemptRecord[] = []
  let attemptCount = 0

  let lastError = ""
  let _lastStatusCode = 0
  let lastRetryAfterMs = 0
  let rateLimited = false

  // First token timeout: 0 means disabled (aligned with Go upstream)
  const firstTokenTimeout = group.firstTokenTimeOut

  // Session stickiness: reorder candidates if sticky channel exists
  const sessionKeepTime = group.sessionKeepTime ?? 0
  const apiKeyId = c.get("apiKeyId")

  if (sessionKeepTime > 0) {
    const sticky = getSticky(apiKeyId, model, sessionKeepTime)
    if (sticky) {
      const stickyIdx = orderedItems.findIndex((it) => it.channelId === sticky.channelId)
      if (stickyIdx > 0) {
        const [stickyItem] = orderedItems.splice(stickyIdx, 1)
        orderedItems.unshift(stickyItem)
      }
    }
  }

  // Circuit breaker config (read once per request)
  const cbConfig = await getCooldownConfig(db)

  // 6.10 Retry logic: MAX_RETRY_ROUNDS rounds × N channels
  for (let round = 1; round <= MAX_RETRY_ROUNDS; round++) {
    for (let idx = 0; idx < orderedItems.length; idx++) {
      const item = orderedItems[idx]
      const channel = channelMap.get(item.channelId)
      if (!channel || !channel.enabled) {
        attemptCount++
        attempts.push({
          channelId: item.channelId,
          channelName: channel?.name ?? "unknown",
          modelName: item.modelName || model,
          attemptNum: attemptCount,
          status: "skipped",
          duration: 0,
          msg: !channel ? "channel not found" : "channel disabled",
        })
        continue
      }

      // 6.4 Select key
      const key = selectKey(channel.keys)
      if (!key) {
        attemptCount++
        attempts.push({
          channelId: channel.id,
          channelName: channel.name,
          modelName: item.modelName || model,
          attemptNum: attemptCount,
          status: "skipped",
          duration: 0,
          msg: "no available key",
        })
        continue
      }

      // Determine the actual model name
      const targetModel = item.modelName || model
      const isSticky =
        sessionKeepTime > 0 && idx === 0 && getSticky(apiKeyId, model, sessionKeepTime) !== null

      // Check circuit breaker
      const cb = isTripped(channel.id, key.id, targetModel, cbConfig.baseSec, cbConfig.maxSec)
      if (cb.tripped) {
        attemptCount++
        attempts.push({
          channelId: channel.id,
          channelKeyId: key.id,
          channelName: channel.name,
          modelName: targetModel,
          attemptNum: attemptCount,
          status: "circuit_break",
          duration: 0,
          sticky: isSticky,
          msg:
            cb.remainingMs > 0
              ? `circuit breaker tripped, remaining cooldown: ${Math.ceil(cb.remainingMs / 1000)}s`
              : "circuit breaker tripped",
        })
        continue
      }

      const attemptStart = Date.now()
      attemptCount++
      const currentAttemptNum = attemptCount

      try {
        // 6.5 Build upstream request
        const upstream = buildUpstreamRequest(
          {
            type: channel.type as OutboundType,
            baseUrls: channel.baseUrls,
            customHeader: channel.customHeader,
            paramOverride: channel.paramOverride,
          },
          key.channelKey,
          body,
          path,
          targetModel,
          isAnthropicInbound && (channel.type as OutboundType) === OutboundType.Anthropic,
        )

        if (stream) {
          // 6.8 Streaming path
          let streamInfo: StreamCompleteInfo | null = null

          const isAnthropicPassthrough =
            isAnthropicInbound && (channel.type as OutboundType) === OutboundType.Anthropic

          const { readable, firstChunkPromise, fetchPromise } = proxyStreaming(
            upstream.url,
            upstream.headers,
            upstream.body,
            channel.type as OutboundType,
            firstTokenTimeout,
            (info) => {
              streamInfo = info
            },
            isAnthropicPassthrough,
          )

          // Wait for first token to confirm stream is healthy
          try {
            await firstChunkPromise
          } catch (firstChunkErr) {
            const errMsg =
              firstChunkErr instanceof Error ? firstChunkErr.message : String(firstChunkErr)
            attempts.push({
              channelId: channel.id,
              channelKeyId: key.id,
              channelName: channel.name,
              modelName: targetModel,
              attemptNum: currentAttemptNum,
              status: "failed",
              duration: Date.now() - attemptStart,
              sticky: isSticky,
              msg: errMsg,
            })
            lastError = errMsg

            // Record circuit breaker failure
            c.get("runBackground")(
              recordFailure(channel.id, key.id, targetModel, db).catch(() => {}),
            )

            if (firstChunkErr instanceof ProxyError && firstChunkErr.statusCode === 429) {
              _lastStatusCode = 429
              lastRetryAfterMs = Math.max(lastRetryAfterMs, firstChunkErr.retryAfterMs || 1000)
              rateLimited = true
              c.get("runBackground")(updateChannelKeyStatus(db, key.id, 429).catch(() => {}))
            }
            continue
          }

          // First token received — commit response to client
          attempts.push({
            channelId: channel.id,
            channelKeyId: key.id,
            channelName: channel.name,
            modelName: targetModel,
            attemptNum: currentAttemptNum,
            status: "success",
            duration: Date.now() - attemptStart,
            sticky: isSticky,
          })

          // Record circuit breaker success + session stickiness
          recordSuccess(channel.id, key.id, targetModel)
          if (sessionKeepTime > 0) {
            setSticky(apiKeyId, model, channel.id, key.id)
          }

          // Clear 429 status on success
          if (key.statusCode === 429) {
            c.get("runBackground")(updateChannelKeyStatus(db, key.id, 0).catch(() => {}))
          }

          // 6.11 Async logging + cost accumulation
          const logBody = truncateForLog(body)
          const channelKeyId = key.id
          const finalAttempts = [...attempts]
          c.get("runBackground")(
            fetchPromise
              .then(async () => {
                const cost = await calculateCost(
                  targetModel,
                  streamInfo?.inputTokens ?? 0,
                  streamInfo?.outputTokens ?? 0,
                  db,
                  {
                    cacheReadTokens: streamInfo?.cacheReadTokens ?? 0,
                    cacheCreationTokens: streamInfo?.cacheCreationTokens ?? 0,
                  },
                )
                const logRow = await writeLog(db, {
                  model,
                  actualModel: targetModel,
                  channelId: channel.id,
                  channelName: channel.name,
                  inputTokens: streamInfo?.inputTokens ?? 0,
                  outputTokens: streamInfo?.outputTokens ?? 0,
                  ftut: streamInfo?.firstTokenTime ?? 0,
                  useTime: Date.now() - startTime,
                  cost,
                  requestContent: logBody,
                  responseContent: streamInfo?.responseContent || "[streaming]",
                  error: "",
                  attempts: finalAttempts,
                })
                if (cost > 0) {
                  await Promise.all([
                    incrementApiKeyCost(db, apiKeyId, cost),
                    incrementChannelKeyCost(db, channelKeyId, cost),
                  ])
                }
                broadcast("stats-updated")
                broadcast("log-created", {
                  log: {
                    id: logRow.id,
                    time: logRow.time,
                    requestModelName: logRow.requestModelName,
                    actualModelName: logRow.actualModelName,
                    channelId: logRow.channelId,
                    channelName: logRow.channelName,
                    inputTokens: logRow.inputTokens,
                    outputTokens: logRow.outputTokens,
                    ftut: logRow.ftut,
                    useTime: logRow.useTime,
                    error: logRow.error,
                    cost: logRow.cost,
                    totalAttempts: logRow.totalAttempts,
                  },
                })
                await maybeCleanupLogs(db)
              })
              .catch(() => {}),
          )

          const outputStream = isAnthropicPassthrough
            ? readable
            : isAnthropicInbound
              ? convertOpenAIStreamToAnthropic(readable, targetModel)
              : readable

          return new Response(outputStream, {
            headers: {
              "Content-Type": "text/event-stream",
              "Cache-Control": "no-cache",
              Connection: "keep-alive",
            },
          })
        } else {
          // 6.7 Non-streaming path
          const isAnthropicPassthrough =
            isAnthropicInbound && (channel.type as OutboundType) === OutboundType.Anthropic

          const result = await proxyNonStreaming(
            upstream.url,
            upstream.headers,
            upstream.body,
            channel.type as OutboundType,
            isAnthropicPassthrough,
          )

          attempts.push({
            channelId: channel.id,
            channelKeyId: key.id,
            channelName: channel.name,
            modelName: targetModel,
            attemptNum: currentAttemptNum,
            status: "success",
            duration: Date.now() - attemptStart,
            sticky: isSticky,
          })

          // Record circuit breaker success + session stickiness
          recordSuccess(channel.id, key.id, targetModel)
          if (sessionKeepTime > 0) {
            setSticky(apiKeyId, model, channel.id, key.id)
          }

          // Clear 429 status on success
          if (key.statusCode === 429) {
            c.get("runBackground")(updateChannelKeyStatus(db, key.id, 0).catch(() => {}))
          }

          // Async log + cost accumulation
          const logBody = truncateForLog(body)
          const respContent = JSON.stringify(result.response).slice(0, MAX_LOG_JSON)
          const channelKeyId = key.id
          const finalAttempts = [...attempts]
          c.get("runBackground")(
            calculateCost(targetModel, result.inputTokens, result.outputTokens, db, {
              cacheReadTokens: result.cacheReadTokens,
              cacheCreationTokens: result.cacheCreationTokens,
            }).then(async (cost) => {
              const logRow = await writeLog(db, {
                model,
                actualModel: targetModel,
                channelId: channel.id,
                channelName: channel.name,
                inputTokens: result.inputTokens,
                outputTokens: result.outputTokens,
                ftut: 0,
                useTime: Date.now() - startTime,
                cost,
                requestContent: logBody,
                responseContent: respContent,
                error: "",
                attempts: finalAttempts,
              })
              if (cost > 0) {
                await Promise.all([
                  incrementApiKeyCost(db, apiKeyId, cost),
                  incrementChannelKeyCost(db, channelKeyId, cost),
                ])
              }
              broadcast("stats-updated")
              broadcast("log-created", {
                log: {
                  id: logRow.id,
                  time: logRow.time,
                  requestModelName: logRow.requestModelName,
                  actualModelName: logRow.actualModelName,
                  channelId: logRow.channelId,
                  channelName: logRow.channelName,
                  inputTokens: logRow.inputTokens,
                  outputTokens: logRow.outputTokens,
                  ftut: logRow.ftut,
                  useTime: logRow.useTime,
                  error: logRow.error,
                  cost: logRow.cost,
                  totalAttempts: logRow.totalAttempts,
                },
              })
              await maybeCleanupLogs(db)
            }),
          )

          if (isAnthropicPassthrough) {
            return c.json(result.response)
          }
          if (isAnthropicInbound) {
            return c.json(convertToAnthropicResponse(result.response))
          }
          return c.json(result.response)
        }
      } catch (err) {
        const errMsg = err instanceof Error ? err.message : String(err)
        attempts.push({
          channelId: channel.id,
          channelKeyId: key.id,
          channelName: channel.name,
          modelName: targetModel,
          attemptNum: currentAttemptNum,
          status: "failed",
          duration: Date.now() - attemptStart,
          sticky: isSticky,
          msg: errMsg,
        })
        lastError = errMsg

        // Record circuit breaker failure
        c.get("runBackground")(recordFailure(channel.id, key.id, targetModel, db).catch(() => {}))

        if (err instanceof ProxyError) {
          _lastStatusCode = err.statusCode
          if (err.statusCode === 429) {
            lastRetryAfterMs = Math.max(lastRetryAfterMs, err.retryAfterMs || 1000)
            rateLimited = true
            c.get("runBackground")(updateChannelKeyStatus(db, key.id, 429).catch(() => {}))
          }
        }

        continue
      }
    }
  }

  // All retries exhausted
  const exhaustedStatus = rateLimited ? 429 : 502
  const retryAfterSecs = rateLimited ? Math.ceil(lastRetryAfterMs / 1000) || 1 : 0

  // Determine channel from last attempt
  const lastAttempt = [...attempts].reverse().find((a: AttemptRecord) => a.status === "failed")

  const logBody = truncateForLog(body)
  c.get("runBackground")(
    writeLog(db, {
      model,
      actualModel: model,
      channelId: lastAttempt?.channelId ?? 0,
      channelName: lastAttempt?.channelName ?? "",
      inputTokens: 0,
      outputTokens: 0,
      ftut: 0,
      useTime: Date.now() - startTime,
      cost: 0,
      requestContent: logBody,
      responseContent: "",
      error: lastError,
      attempts,
    }).then(async (logRow) => {
      broadcast("stats-updated")
      broadcast("log-created", {
        log: {
          id: logRow.id,
          time: logRow.time,
          requestModelName: logRow.requestModelName,
          actualModelName: logRow.actualModelName,
          channelId: logRow.channelId,
          channelName: logRow.channelName,
          inputTokens: logRow.inputTokens,
          outputTokens: logRow.outputTokens,
          ftut: logRow.ftut,
          useTime: logRow.useTime,
          error: logRow.error,
          cost: logRow.cost,
          totalAttempts: logRow.totalAttempts,
        },
      })
      await maybeCleanupLogs(db)
    }),
  )

  if (retryAfterSecs > 0) {
    c.header("Retry-After", String(retryAfterSecs))
  }
  return c.json(
    {
      error: {
        message: `All channels exhausted after ${MAX_RETRY_ROUNDS} rounds. Last error: ${lastError}`,
        type: rateLimited ? "rate_limit_error" : "server_error",
      },
    },
    exhaustedStatus as 429 | 502,
  )
})

// Helper: load channels with KV cache
async function loadChannels(kv: IKVStore, db: Database) {
  const cached = await kv.get("channels", "json")
  if (cached) return cached as Awaited<ReturnType<typeof listChannels>>

  const channels = await listChannels(db)
  await kv.put("channels", JSON.stringify(channels), { expirationTtl: 300 })
  return channels
}

// Helper: load groups with KV cache
async function loadGroups(kv: IKVStore, db: Database) {
  const cached = await kv.get("groups", "json")
  if (cached) return cached as Awaited<ReturnType<typeof listGroups>>

  const groups = await listGroups(db)
  await kv.put("groups", JSON.stringify(groups), { expirationTtl: 300 })
  return groups
}

// Helper: write relay log to D1 and return the created row
async function writeLog(
  db: Database,
  data: {
    model: string
    actualModel: string
    channelId: number
    channelName: string
    inputTokens: number
    outputTokens: number
    ftut: number
    useTime: number
    cost: number
    requestContent: string
    responseContent: string
    error: string
    attempts: Array<{
      channelId: number
      channelKeyId?: number
      channelName: string
      modelName: string
      attemptNum: number
      status: AttemptStatus
      duration: number
      sticky?: boolean
      msg?: string
    }>
  },
) {
  return createLog(db, {
    time: Math.floor(Date.now() / 1000),
    requestModelName: data.model,
    channelId: data.channelId,
    channelName: data.channelName,
    actualModelName: data.actualModel,
    inputTokens: data.inputTokens,
    outputTokens: data.outputTokens,
    ftut: data.ftut,
    useTime: data.useTime,
    cost: data.cost,
    requestContent: data.requestContent,
    responseContent: data.responseContent,
    error: data.error,
    attempts: data.attempts,
    totalAttempts: data.attempts.length,
  })
}

// Probabilistic log cleanup (1/100 chance per request)
async function maybeCleanupLogs(db: Database) {
  if (Math.random() > 0.01) return
  const days = await getSetting(db, "log_retention_days")
  const retentionDays = days ? Number.parseInt(days, 10) : 30
  if (retentionDays > 0) {
    await cleanupOldLogs(db, retentionDays)
  }
}

/**
 * Transform an OpenAI SSE stream into Anthropic SSE format.
 * Converts chat.completion.chunk events to Anthropic message events.
 */
function convertOpenAIStreamToAnthropic(readable: ReadableStream, model: string): ReadableStream {
  const decoder = new TextDecoder()
  const encoder = new TextEncoder()
  let buffer = ""
  let sentMessageStart = false
  let sentContentBlockStart = false
  let sentStop = false

  return readable.pipeThrough(
    new TransformStream({
      transform(chunk, controller) {
        buffer += decoder.decode(chunk, { stream: true })
        const lines = buffer.split("\n")
        buffer = lines.pop() ?? ""

        for (const line of lines) {
          if (!line.trim()) {
            continue
          }

          if (line === "data: [DONE]") {
            if (!sentStop) {
              sentStop = true
              controller.enqueue(
                encoder.encode(
                  'event: content_block_stop\ndata: {"type":"content_block_stop","index":0}\n\n',
                ),
              )
              controller.enqueue(
                encoder.encode(
                  `event: message_delta\ndata: ${JSON.stringify({
                    type: "message_delta",
                    delta: { stop_reason: "end_turn", stop_sequence: null },
                    usage: { output_tokens: 0 },
                  })}\n\n`,
                ),
              )
              controller.enqueue(
                encoder.encode('event: message_stop\ndata: {"type":"message_stop"}\n\n'),
              )
            }
            continue
          }

          if (!line.startsWith("data: ")) {
            continue
          }

          try {
            const obj = JSON.parse(line.slice(6))
            const delta = obj.choices?.[0]?.delta
            const finishReason = obj.choices?.[0]?.finish_reason

            if (!sentMessageStart) {
              sentMessageStart = true
              controller.enqueue(
                encoder.encode(
                  `event: message_start\ndata: ${JSON.stringify({
                    type: "message_start",
                    message: {
                      id: obj.id ?? `msg_${crypto.randomUUID()}`,
                      type: "message",
                      role: "assistant",
                      model: obj.model || model,
                      content: [],
                      stop_reason: null,
                      stop_sequence: null,
                      usage: { input_tokens: 0, output_tokens: 0 },
                    },
                  })}\n\n`,
                ),
              )
            }

            if (!sentContentBlockStart && delta) {
              sentContentBlockStart = true
              controller.enqueue(
                encoder.encode(
                  `event: content_block_start\ndata: ${JSON.stringify({
                    type: "content_block_start",
                    index: 0,
                    content_block: { type: "text", text: "" },
                  })}\n\n`,
                ),
              )
            }

            if (delta?.content) {
              controller.enqueue(
                encoder.encode(
                  `event: content_block_delta\ndata: ${JSON.stringify({
                    type: "content_block_delta",
                    index: 0,
                    delta: { type: "text_delta", text: delta.content },
                  })}\n\n`,
                ),
              )
            }

            if (finishReason && !sentStop) {
              sentStop = true
              controller.enqueue(
                encoder.encode(
                  'event: content_block_stop\ndata: {"type":"content_block_stop","index":0}\n\n',
                ),
              )
              const stopReason = finishReason === "length" ? "max_tokens" : "end_turn"
              controller.enqueue(
                encoder.encode(
                  `event: message_delta\ndata: ${JSON.stringify({
                    type: "message_delta",
                    delta: { stop_reason: stopReason, stop_sequence: null },
                    usage: { output_tokens: obj.usage?.completion_tokens ?? 0 },
                  })}\n\n`,
                ),
              )
              controller.enqueue(
                encoder.encode('event: message_stop\ndata: {"type":"message_stop"}\n\n'),
              )
            }
          } catch {
            // Forward as-is if parse fails
            controller.enqueue(encoder.encode(`${line}\n`))
          }
        }
      },
      flush(controller) {
        if (buffer.trim()) {
          controller.enqueue(encoder.encode(`${buffer}\n`))
        }
      },
    }),
  )
}

export { relayRoutes }

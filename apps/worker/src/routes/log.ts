import type { GroupMode, OutboundType } from "@wheel/core"
import type { AppEnv } from "../runtime/types"
import { Hono } from "hono"
import { listChannels } from "../db/dal/channels"
import { listGroups } from "../db/dal/groups"
import { clearLogs, deleteLog, getLog, listLogs } from "../db/dal/logs"
import { buildUpstreamRequest } from "../relay/adapter"
import { selectChannelOrder } from "../relay/balancer"
import { selectKey } from "../relay/key-selector"
import { matchGroup } from "../relay/matcher"
import { proxyNonStreaming, proxyStreaming } from "../relay/proxy"

const logRoutes = new Hono<AppEnv>()

logRoutes.get("/list", async (c) => {
  const db = c.env.DB
  const result = await listLogs(db, {
    page: Number(c.req.query("page") ?? 1),
    pageSize: Number(c.req.query("pageSize") ?? 20),
    model: c.req.query("model"),
    channelId: c.req.query("channelId") ? Number(c.req.query("channelId")) : undefined,
    hasError:
      c.req.query("status") === "error"
        ? true
        : c.req.query("status") === "success"
          ? false
          : undefined,
    startTime: c.req.query("startTime") ? Number(c.req.query("startTime")) : undefined,
    endTime: c.req.query("endTime") ? Number(c.req.query("endTime")) : undefined,
    keyword: c.req.query("keyword") || undefined,
  })
  return c.json({ success: true, data: result })
})

logRoutes.get("/:id", async (c) => {
  const id = Number(c.req.param("id"))
  const db = c.env.DB
  const log = await getLog(db, id)
  if (!log) return c.json({ success: false, error: "Log not found" }, 404)
  return c.json({ success: true, data: log })
})

logRoutes.delete("/delete/:id", async (c) => {
  const id = Number(c.req.param("id"))
  const db = c.env.DB
  await deleteLog(db, id)
  return c.json({ success: true })
})

logRoutes.delete("/clear", async (c) => {
  const db = c.env.DB
  await clearLogs(db)
  return c.json({ success: true })
})

/** Detect if stored request content was truncated during log storage. */
export function detectTruncation(content: string): boolean {
  return (
    /\[truncated,?\s*\d+\s*chars\s*total\]/.test(content) ||
    /\[\d+\s*messages?\s*omitted/.test(content) ||
    /\[image data omitted\]/.test(content)
  )
}

logRoutes.post("/replay/:id", async (c) => {
  const id = Number(c.req.param("id"))
  const db = c.env.DB

  const log = await getLog(db, id)
  if (!log) return c.json({ success: false, error: "Log not found" }, 404)

  let body: Record<string, unknown>
  try {
    body = JSON.parse(log.requestContent)
  } catch {
    return c.json({ success: false, error: "Failed to parse stored request content" }, 400)
  }

  const truncated = detectTruncation(log.requestContent)
  const model = log.requestModelName
  const stream = (body.stream as boolean) ?? false

  const [allChannels, allGroups] = await Promise.all([listChannels(db), listGroups(db)])

  const group = matchGroup(model, allGroups)
  if (!group || group.items.length === 0) {
    return c.json({ success: false, error: `No group matches model '${model}'` }, 400)
  }

  const orderedItems = selectChannelOrder(group.mode as GroupMode, group.items, group.id)
  const channelMap = new Map(allChannels.map((ch) => [ch.id, ch]))

  for (const item of orderedItems) {
    const channel = channelMap.get(item.channelId)
    if (!channel || !channel.enabled) continue

    const key = selectKey(channel.keys)
    if (!key) continue

    const targetModel = item.modelName || model

    try {
      const upstream = buildUpstreamRequest(
        {
          type: channel.type as OutboundType,
          baseUrls: channel.baseUrls,
          customHeader: channel.customHeader,
          paramOverride: channel.paramOverride,
        },
        key.channelKey,
        body,
        "/v1/chat/completions",
        targetModel,
        false,
      )

      if (stream) {
        const { readable, firstChunkPromise } = proxyStreaming(
          upstream.url,
          upstream.headers,
          upstream.body,
          channel.type as OutboundType,
          0,
          () => {},
          false,
        )

        await firstChunkPromise

        return new Response(readable, {
          headers: {
            "Content-Type": "text/event-stream",
            "Cache-Control": "no-cache",
            Connection: "keep-alive",
            ...(truncated ? { "X-Replay-Truncated": "true" } : {}),
          },
        })
      }

      const result = await proxyNonStreaming(
        upstream.url,
        upstream.headers,
        upstream.body,
        channel.type as OutboundType,
        false,
      )

      return c.json({
        success: true,
        data: { response: result.response, truncated },
      })
    } catch {
      continue
    }
  }

  return c.json({ success: false, error: "All channels exhausted" }, 502)
})

export { logRoutes }

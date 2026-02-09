import type { AppEnv } from "../runtime/types"
import { Hono } from "hono"
import {
  createChannel,
  deleteChannel,
  enableChannel,
  getChannel,
  listChannels,
  syncChannelKeys,
  updateChannel,
} from "../db/dal/channels"
import { getAllSettings } from "../db/dal/settings"
import { fetchModelsFromChannel, syncAllModels } from "../relay/sync"

const channelRoutes = new Hono<AppEnv>()

channelRoutes.get("/list", async (c) => {
  const db = c.env.DB
  const channels = await listChannels(db)
  return c.json({ success: true, data: { channels } })
})

channelRoutes.post("/create", async (c) => {
  const body = await c.req.json()
  const db = c.env.DB
  const ch = await createChannel(
    db,
    {
      name: body.name,
      type: body.type,
      enabled: body.enabled ?? true,
      baseUrls: body.baseUrls ?? [],
      model: body.model ?? [],
      customModel: body.customModel ?? "",
      autoSync: body.autoSync ?? false,
      autoGroup: body.autoGroup ?? 0,
      customHeader: body.customHeader ?? [],
      paramOverride: body.paramOverride ?? null,
    },
    body.keys ?? [],
  )
  await c.env.CACHE.delete("channels")
  return c.json({ success: true, data: ch })
})

channelRoutes.post("/update", async (c) => {
  const body = await c.req.json()
  const db = c.env.DB
  const { id, keys, ...data } = body
  const ch = await updateChannel(db, id, data)
  if (keys) {
    await syncChannelKeys(db, id, keys)
  }
  await c.env.CACHE.delete("channels")
  return c.json({ success: true, data: ch })
})

channelRoutes.post("/enable", async (c) => {
  const body = await c.req.json()
  const db = c.env.DB
  await enableChannel(db, body.id, body.enabled)
  await c.env.CACHE.delete("channels")
  return c.json({ success: true })
})

channelRoutes.delete("/delete/:id", async (c) => {
  const id = Number(c.req.param("id"))
  const db = c.env.DB
  await deleteChannel(db, id)
  await c.env.CACHE.delete("channels")
  return c.json({ success: true })
})

// Fetch models from a specific channel's upstream provider
channelRoutes.post("/fetch-model", async (c) => {
  const body = await c.req.json()
  const db = c.env.DB
  const channel = await getChannel(db, body.id)
  if (!channel) {
    return c.json({ success: false, error: "Channel not found" }, 404)
  }
  try {
    const models = await fetchModelsFromChannel(channel, {
      DB: c.env.DB,
      CACHE: c.env.CACHE,
    })
    return c.json({ success: true, data: { models } })
  } catch (err) {
    return c.json(
      {
        success: false,
        error: err instanceof Error ? err.message : String(err),
      },
      500,
    )
  }
})

// Fetch models from upstream without saving the channel (for preview / auto-fill)
channelRoutes.post("/fetch-model-preview", async (c) => {
  const body = (await c.req.json()) as {
    type: number
    baseUrl: string
    key: string
  }

  if (!body.baseUrl || !body.key) {
    return c.json({ success: false, error: "baseUrl and key are required" }, 400)
  }

  // Build a minimal channel-like object for fetchModelsFromChannel
  const pseudoChannel = {
    id: 0,
    name: "",
    type: body.type ?? 1,
    enabled: true,
    model: [],
    customModel: "",
    baseUrls: [{ url: body.baseUrl, delay: 0 }],
    keys: [
      {
        id: 0,
        channelId: 0,
        channelKey: body.key,
        remark: "",
        enabled: true,
        totalCost: 0,
        requestCount: 0,
      },
    ],
    autoSync: false,
    autoGroup: 0,
    customHeader: [],
    paramOverride: null,
    createdAt: "",
    updatedAt: "",
  }

  try {
    const models = await fetchModelsFromChannel(pseudoChannel as any, {
      DB: c.env.DB,
      CACHE: c.env.CACHE,
    })
    return c.json({ success: true, data: { models } })
  } catch (err) {
    return c.json(
      {
        success: false,
        error: err instanceof Error ? err.message : String(err),
      },
      500,
    )
  }
})

// Trigger full model sync across all autoSync channels
channelRoutes.post("/sync", async (c) => {
  try {
    const result = await syncAllModels({ DB: c.env.DB, CACHE: c.env.CACHE })
    return c.json({ success: true, data: result })
  } catch (err) {
    return c.json(
      {
        success: false,
        error: err instanceof Error ? err.message : String(err),
      },
      500,
    )
  }
})

// Get the last sync timestamp
channelRoutes.get("/last-sync-time", async (c) => {
  const db = c.env.DB
  const allSettings = await getAllSettings(db)
  const lastSyncTime = allSettings.last_sync_time ?? "0"
  return c.json({ success: true, data: { lastSyncTime: Number(lastSyncTime) } })
})

export { channelRoutes }

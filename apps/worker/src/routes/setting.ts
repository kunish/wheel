import type { DBDump, ImportResult } from "@wheel/core"
import type { AppEnv } from "../runtime/types"
import { Hono } from "hono"
import { getAllSettings, updateSettings } from "../db/dal/settings"
import * as schema from "../db/schema"

const DEFAULT_SETTINGS: Record<string, string> = {
  log_retention_days: "30",
  circuit_breaker_threshold: "5",
  circuit_breaker_cooldown: "60",
  circuit_breaker_max_cooldown: "600",
}

const settingRoutes = new Hono<AppEnv>()

settingRoutes.get("/", async (c) => {
  const db = c.env.DB
  const settings = await getAllSettings(db)
  // Merge defaults for keys that don't exist yet, only return configurable keys
  const merged = { ...DEFAULT_SETTINGS }
  for (const key of Object.keys(DEFAULT_SETTINGS)) {
    if (key in settings) {
      merged[key] = settings[key]
    }
  }
  return c.json({ success: true, data: { settings: merged } })
})

settingRoutes.post("/update", async (c) => {
  const body = await c.req.json()
  const db = c.env.DB
  await updateSettings(db, body.settings)
  return c.json({ success: true })
})

// Export all data as JSON file
settingRoutes.get("/export", async (c) => {
  try {
    const includeLogs = c.req.query("include_logs") === "true"
    const db = c.env.DB

    const [allChannels, allChannelKeys, allGroups, allGroupItems, allApiKeys, allSettings] =
      await Promise.all([
        db.select().from(schema.channels),
        db.select().from(schema.channelKeys),
        db.select().from(schema.groups),
        db.select().from(schema.groupItems),
        db.select().from(schema.apiKeys),
        db.select().from(schema.settings),
      ])

    // Assemble channels with their keys (matching the Channel model shape)
    const channelsWithKeys = allChannels.map((ch) => ({
      ...ch,
      keys: allChannelKeys.filter((k) => k.channelId === ch.id),
    }))

    // Assemble groups with their items (matching the Group model shape)
    const groupsWithItems = allGroups.map((g) => ({
      ...g,
      items: allGroupItems.filter((i) => i.groupId === g.id),
    }))

    const dump: DBDump = {
      version: 1,
      exportedAt: new Date().toISOString(),
      channels: channelsWithKeys,
      groups: groupsWithItems,
      apiKeys: allApiKeys,
      settings: allSettings,
    }

    if (includeLogs) {
      dump.relayLogs = await db.select().from(schema.relayLogs)
    }

    return new Response(JSON.stringify(dump, null, 2), {
      headers: {
        "Content-Type": "application/json",
        "Content-Disposition": `attachment; filename="wheel-export-${Date.now()}.json"`,
      },
    })
  } catch (e) {
    const message = e instanceof Error ? e.message : "Export failed"
    return c.json({ success: false, error: message }, 500)
  }
})

// Import data from JSON
settingRoutes.post("/import", async (c) => {
  try {
    let dump: DBDump

    const contentType = c.req.header("content-type") || ""
    if (contentType.includes("multipart/form-data")) {
      const formData = await c.req.formData()
      const file = formData.get("file") as File | null
      if (!file) {
        return c.json({ success: false, error: "No file provided" }, 400)
      }
      const text = await file.text()
      dump = JSON.parse(text)
    } else {
      dump = await c.req.json()
    }

    if (!dump.version || !dump.exportedAt) {
      return c.json({ success: false, error: "Invalid dump format" }, 400)
    }

    const db = c.env.DB

    const result: ImportResult = {
      channels: { added: 0, skipped: 0 },
      groups: { added: 0, skipped: 0 },
      apiKeys: { added: 0, skipped: 0 },
      settings: { added: 0, skipped: 0 },
    }

    // Import channels (match by name)
    if (dump.channels?.length) {
      const existingChannels = await db.select().from(schema.channels)
      const existingNames = new Set(existingChannels.map((ch) => ch.name))

      for (const ch of dump.channels) {
        if (existingNames.has(ch.name)) {
          result.channels.skipped++
          continue
        }

        const { id, keys, ...channelData } = ch as typeof ch & { keys?: unknown[] }
        const [inserted] = await db.insert(schema.channels).values(channelData).returning()

        // Import channel keys
        if (keys && Array.isArray(keys) && keys.length > 0) {
          await db.insert(schema.channelKeys).values(
            keys.map((k: any) => ({
              channelId: inserted.id,
              enabled: k.enabled ?? true,
              channelKey: k.channelKey,
              statusCode: k.statusCode ?? 0,
              lastUseTimestamp: k.lastUseTimestamp ?? 0,
              totalCost: k.totalCost ?? 0,
              remark: k.remark ?? "",
            })),
          )
        }

        result.channels.added++
      }
    }

    // Import groups (match by name)
    if (dump.groups?.length) {
      const existingGroups = await db.select().from(schema.groups)
      const existingNames = new Set(existingGroups.map((g) => g.name))

      // Build a name-to-id mapping for channels (needed for groupItems)
      const allChannels = await db.select().from(schema.channels)
      const channelNameToId = new Map(allChannels.map((ch) => [ch.name, ch.id]))

      // Also build old-id to name mapping from the dump
      const dumpChannelIdToName = new Map((dump.channels || []).map((ch) => [ch.id, ch.name]))

      for (const g of dump.groups) {
        if (existingNames.has(g.name)) {
          result.groups.skipped++
          continue
        }

        const { id, items, ...groupData } = g as typeof g & { items?: unknown[] }
        const [inserted] = await db.insert(schema.groups).values(groupData).returning()

        // Import group items, remapping channelId from dump to current DB
        if (items && Array.isArray(items) && items.length > 0) {
          const validItems: (typeof schema.groupItems.$inferInsert)[] = []
          for (const item of items as any[]) {
            // Resolve channel ID: find the channel name from the dump, then find its current ID
            const channelName = dumpChannelIdToName.get(item.channelId)
            const currentChannelId = channelName ? channelNameToId.get(channelName) : undefined

            if (currentChannelId) {
              validItems.push({
                groupId: inserted.id,
                channelId: currentChannelId,
                modelName: item.modelName ?? "",
                priority: item.priority ?? 0,
                weight: item.weight ?? 1,
              })
            }
          }

          if (validItems.length > 0) {
            await db.insert(schema.groupItems).values(validItems)
          }
        }

        result.groups.added++
      }
    }

    // Import API keys (match by apiKey value)
    if (dump.apiKeys?.length) {
      const existingKeys = await db.select().from(schema.apiKeys)
      const existingKeyValues = new Set(existingKeys.map((k) => k.apiKey))

      for (const ak of dump.apiKeys) {
        if (existingKeyValues.has(ak.apiKey)) {
          result.apiKeys.skipped++
          continue
        }

        const { id, ...keyData } = ak
        await db.insert(schema.apiKeys).values(keyData)
        result.apiKeys.added++
      }
    }

    // Import settings (match by key, skip existing)
    if (dump.settings?.length) {
      const existingSettings = await db.select().from(schema.settings)
      const existingKeySet = new Set(existingSettings.map((s) => s.key))

      for (const s of dump.settings) {
        if (existingKeySet.has(s.key)) {
          result.settings.skipped++
          continue
        }

        await db.insert(schema.settings).values({ key: s.key, value: s.value })
        result.settings.added++
      }
    }

    // Invalidate caches
    await Promise.all([
      c.env.CACHE.delete("channels"),
      c.env.CACHE.delete("groups"),
      c.env.CACHE.delete("apikeys"),
      c.env.CACHE.delete("settings"),
    ])

    return c.json({ success: true, data: result })
  } catch (e) {
    const message = e instanceof Error ? e.message : "Import failed"
    return c.json({ success: false, error: message }, 500)
  }
})

export { settingRoutes }

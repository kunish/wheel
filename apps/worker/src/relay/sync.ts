import type { SyncResult } from "@wheel/core"
import type { Database, IKVStore } from "../runtime/types"
import { AutoGroupType, OutboundType } from "@wheel/core"
import { and, eq } from "drizzle-orm"
import { channelKeys, channels, groupItems, groups, settings } from "../db/schema"

interface SyncEnv {
  DB: Database
  CACHE: IKVStore
}

type ChannelRow = typeof channels.$inferSelect & {
  keys: (typeof channelKeys.$inferSelect)[]
}

/**
 * Fetch models from an upstream channel provider.
 */
export async function fetchModelsFromChannel(
  channel: ChannelRow,
  _env: SyncEnv,
): Promise<string[]> {
  const key = channel.keys.find((k) => k.enabled)
  if (!key) return []

  const baseUrl = channel.baseUrls.length > 0 ? channel.baseUrls[0].url.replace(/\/+$/, "") : ""
  if (!baseUrl) return []

  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 10_000)

  try {
    switch (channel.type) {
      case OutboundType.OpenAI:
      case OutboundType.OpenAIChat:
      case OutboundType.OpenAIEmbedding:
      case OutboundType.Volcengine:
        return await fetchOpenAIModels(baseUrl, key.channelKey, controller.signal)
      case OutboundType.Anthropic:
        return await fetchAnthropicModels(baseUrl, key.channelKey, controller.signal)
      case OutboundType.Gemini:
        return await fetchGeminiModels(baseUrl, key.channelKey, controller.signal)
      default:
        return await fetchOpenAIModels(baseUrl, key.channelKey, controller.signal)
    }
  } finally {
    clearTimeout(timeout)
  }
}

async function fetchOpenAIModels(
  baseUrl: string,
  apiKey: string,
  signal: AbortSignal,
): Promise<string[]> {
  const resp = await fetch(`${baseUrl}/v1/models`, {
    headers: { Authorization: `Bearer ${apiKey}` },
    signal,
  })
  if (!resp.ok) throw new Error(`OpenAI models API returned ${resp.status}`)
  const json = (await resp.json()) as { data?: { id: string }[] }
  return (json.data ?? []).map((m) => m.id)
}

async function fetchAnthropicModels(
  baseUrl: string,
  apiKey: string,
  signal: AbortSignal,
): Promise<string[]> {
  const resp = await fetch(`${baseUrl}/v1/models`, {
    headers: {
      "x-api-key": apiKey,
      "anthropic-version": "2023-06-01",
    },
    signal,
  })
  if (!resp.ok) throw new Error(`Anthropic models API returned ${resp.status}`)
  const json = (await resp.json()) as { data?: { id: string }[] }
  return (json.data ?? []).map((m) => m.id)
}

async function fetchGeminiModels(
  baseUrl: string,
  apiKey: string,
  signal: AbortSignal,
): Promise<string[]> {
  const resp = await fetch(`${baseUrl}/v1/models?key=${apiKey}`, { signal })
  if (!resp.ok) throw new Error(`Gemini models API returned ${resp.status}`)
  const json = (await resp.json()) as { models?: { name: string }[] }
  return (json.models ?? []).map((m) => (m.name.startsWith("models/") ? m.name.slice(7) : m.name))
}

/**
 * Sync models for all channels that have autoSync enabled.
 */
export async function syncAllModels(env: SyncEnv): Promise<SyncResult> {
  const db = env.DB
  const result: SyncResult = {
    syncedChannels: 0,
    newModels: [],
    removedModels: [],
    errors: [],
  }

  // Load all channels with keys
  const allChannels = await db.select().from(channels)
  const allKeys = await db.select().from(channelKeys)

  const channelRows: ChannelRow[] = allChannels.map((ch) => ({
    ...ch,
    keys: allKeys.filter((k) => k.channelId === ch.id),
  }))

  for (const channel of channelRows) {
    if (!channel.autoSync) continue

    try {
      const upstreamModels = await fetchModelsFromChannel(channel, env)
      if (upstreamModels.length === 0) continue

      const oldModels = channel.model ?? []

      const newSet = new Set(upstreamModels)
      const oldSet = new Set(oldModels)

      const added = upstreamModels.filter((m: string) => !oldSet.has(m))
      const removed = oldModels.filter((m: string) => !newSet.has(m))

      // Update channel model field
      await db.update(channels).set({ model: upstreamModels }).where(eq(channels.id, channel.id))

      // Remove group items for disappeared models
      if (removed.length > 0) {
        for (const modelName of removed) {
          await db
            .delete(groupItems)
            .where(and(eq(groupItems.channelId, channel.id), eq(groupItems.modelName, modelName)))
        }
      }

      // Auto group if configured
      if (channel.autoGroup !== AutoGroupType.None) {
        await autoGroupChannel(db, channel, upstreamModels)
      }

      result.syncedChannels++
      result.newModels.push(...added)
      result.removedModels.push(...removed)
    } catch (err) {
      result.errors.push(
        `Channel ${channel.name}: ${err instanceof Error ? err.message : String(err)}`,
      )
    }
  }

  // Save last sync time
  const now = String(Math.floor(Date.now() / 1000))
  await db
    .insert(settings)
    .values({ key: "last_sync_time", value: now })
    .onConflictDoUpdate({ target: settings.key, set: { value: now } })

  // Invalidate caches
  await env.CACHE.delete("channels")
  await env.CACHE.delete("groups")

  return result
}

/**
 * Automatically create groups and group items based on channel autoGroup setting.
 */
async function autoGroupChannel(
  db: Database,
  channel: ChannelRow,
  models: string[],
): Promise<void> {
  const allGroups = await db.select().from(groups)

  for (const modelName of models) {
    let targetGroupName: string | null = null

    switch (channel.autoGroup) {
      case AutoGroupType.Exact:
        targetGroupName = modelName
        break
      case AutoGroupType.Fuzzy:
        targetGroupName = fuzzyMatchGroup(modelName, allGroups)
        if (!targetGroupName) targetGroupName = modelName
        break
    }

    if (!targetGroupName) continue

    // Find or create the group
    let group = allGroups.find((g) => g.name === targetGroupName)
    if (!group) {
      const [created] = await db
        .insert(groups)
        .values({ name: targetGroupName })
        .onConflictDoNothing()
        .returning()
      if (created) {
        group = created
        allGroups.push(created)
      } else {
        const existing = allGroups.find((g) => g.name === targetGroupName)
        if (!existing) continue
        group = existing
      }
    }

    // Check if group item already exists
    const existingItems = await db
      .select()
      .from(groupItems)
      .where(
        and(
          eq(groupItems.groupId, group.id),
          eq(groupItems.channelId, channel.id),
          eq(groupItems.modelName, modelName),
        ),
      )

    if (existingItems.length === 0) {
      await db.insert(groupItems).values({
        groupId: group.id,
        channelId: channel.id,
        modelName,
      })
    }
  }
}

/**
 * Try to fuzzy match a model name to an existing group.
 */
function fuzzyMatchGroup(
  modelName: string,
  existingGroups: (typeof groups.$inferSelect)[],
): string | null {
  const normalized = modelName.toLowerCase()

  for (const g of existingGroups) {
    const gName = g.name.toLowerCase()
    if (normalized === gName) return g.name
    if (normalized.startsWith(gName)) return g.name
    if (gName.startsWith(normalized)) return g.name
  }

  return null
}

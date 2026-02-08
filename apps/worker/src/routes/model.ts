import type { AppEnv, Database } from "../runtime/types"
import { Hono } from "hono"
import { listChannels } from "../db/dal/channels"
import {
  createLLMPrice,
  deleteLLMPrice,
  getLastPriceSyncTime,
  listLLMPrices,
  setLastPriceSyncTime,
  updateLLMPrice,
  upsertLLMPrice,
} from "../db/dal/models"

const modelRoutes = new Hono<AppEnv>()

// List all model prices
modelRoutes.get("/list", async (c) => {
  const db = c.env.DB
  const models = await listLLMPrices(db)
  return c.json({ success: true, data: { models } })
})

// List models grouped by channel
modelRoutes.get("/channel", async (c) => {
  const db = c.env.DB
  const allChannels = await listChannels(db)
  const result = allChannels.map((ch) => {
    const models = [
      ...ch.model,
      ...(ch.customModel
        ? ch.customModel
            .split(",")
            .map((m) => m.trim())
            .filter(Boolean)
        : []),
    ]
    return {
      channelId: ch.id,
      channelName: ch.name,
      type: ch.type,
      enabled: ch.enabled,
      models,
    }
  })
  return c.json({ success: true, data: result })
})

// Create a new model price entry
modelRoutes.post("/create", async (c) => {
  const body = await c.req.json()
  if (!body.name) {
    return c.json({ success: false, error: "name is required" }, 400)
  }
  const db = c.env.DB
  try {
    const model = await createLLMPrice(db, {
      name: body.name,
      inputPrice: body.inputPrice ?? 0,
      outputPrice: body.outputPrice ?? 0,
      source: "manual",
    })
    return c.json({ success: true, data: model })
  } catch (err) {
    const message = err instanceof Error ? err.message : "Failed to create model price"
    if (message.includes("UNIQUE")) {
      return c.json({ success: false, error: `Model '${body.name}' already exists` }, 409)
    }
    return c.json({ success: false, error: message }, 500)
  }
})

// Update existing model price
modelRoutes.post("/update", async (c) => {
  const body = await c.req.json()
  if (!body.id) {
    return c.json({ success: false, error: "id is required" }, 400)
  }
  const db = c.env.DB
  const data: Record<string, unknown> = {}
  if (body.name !== undefined) data.name = body.name
  if (body.inputPrice !== undefined) data.inputPrice = body.inputPrice
  if (body.outputPrice !== undefined) data.outputPrice = body.outputPrice

  const model = await updateLLMPrice(db, body.id, data)
  if (!model) {
    return c.json({ success: false, error: "Model not found" }, 404)
  }
  return c.json({ success: true, data: model })
})

// Delete model price
modelRoutes.post("/delete", async (c) => {
  const body = await c.req.json()
  if (!body.id) {
    return c.json({ success: false, error: "id is required" }, 400)
  }
  const db = c.env.DB
  await deleteLLMPrice(db, body.id)
  return c.json({ success: true })
})

// ─── Model Metadata ────────────────────────────

interface ModelMeta {
  name: string
  provider: string
  providerName: string
  logoUrl: string
}

type ModelMetadataMap = Record<string, ModelMeta>

const METADATA_KV_KEY = "model-metadata"
const METADATA_TTL = 86400 // 24h

// Map model name prefixes to their canonical provider keys
const CANONICAL_PROVIDERS: Record<string, string> = {
  "gpt-": "openai",
  "chatgpt-": "openai",
  "o1-": "openai",
  "o3-": "openai",
  "o4-": "openai",
  "claude-": "anthropic",
  "gemini-": "google",
  "deepseek-": "deepseek",
  "grok-": "xai",
  "qwen-": "alibaba",
  "glm-": "zhipuai",
}

function isCanonicalProvider(modelId: string, providerKey: string): boolean {
  for (const [prefix, canonical] of Object.entries(CANONICAL_PROVIDERS)) {
    if (modelId.startsWith(prefix)) return providerKey === canonical
  }
  return false
}

function hasCanonicalPrefix(modelId: string): boolean {
  return Object.keys(CANONICAL_PROVIDERS).some((p) => modelId.startsWith(p))
}

async function fetchAndFlattenMetadata(): Promise<ModelMetadataMap> {
  const resp = await fetch("https://models.dev/api.json")
  if (!resp.ok) return {}

  const data = (await resp.json()) as Record<
    string,
    { name?: string; models?: Record<string, { id?: string; name?: string }> }
  >

  // Pre-collect canonical provider display info
  const canonicalInfo = new Map<string, { providerName: string; logoUrl: string }>()
  for (const [prefix, canonicalKey] of Object.entries(CANONICAL_PROVIDERS)) {
    const provider = data[canonicalKey]
    if (provider) {
      canonicalInfo.set(prefix, {
        providerName: provider.name ?? canonicalKey,
        logoUrl: `https://models.dev/logos/${canonicalKey}.svg`,
      })
    }
  }

  const map: ModelMetadataMap = {}
  for (const [providerKey, provider] of Object.entries(data)) {
    if (!provider?.models) continue
    const providerDisplayName = provider.name ?? providerKey
    const logoUrl = `https://models.dev/logos/${providerKey}.svg`

    for (const [, model] of Object.entries(provider.models)) {
      const modelId = model.id
      if (!modelId) continue

      // For models with a canonical prefix (e.g. claude-*, gemini-*),
      // always attribute to the canonical provider's branding
      let entryProvider = providerKey
      let entryProviderName = providerDisplayName
      let entryLogoUrl = logoUrl

      if (hasCanonicalPrefix(modelId) && !isCanonicalProvider(modelId, providerKey)) {
        for (const [prefix, info] of canonicalInfo) {
          if (modelId.startsWith(prefix)) {
            entryProvider = CANONICAL_PROVIDERS[prefix]
            entryProviderName = info.providerName
            entryLogoUrl = info.logoUrl
            break
          }
        }
      }

      const entry: ModelMeta = {
        name: model.name ?? modelId,
        provider: entryProvider,
        providerName: entryProviderName,
        logoUrl: entryLogoUrl,
      }

      const existing = map[modelId]
      if (!existing) {
        map[modelId] = entry
      } else if (
        isCanonicalProvider(modelId, providerKey) &&
        !isCanonicalProvider(modelId, existing.provider)
      ) {
        map[modelId] = entry
      }
    }
  }
  return map
}

modelRoutes.get("/metadata", async (c) => {
  // Try KV cache first
  const cached = await c.env.CACHE.get(METADATA_KV_KEY, "json")
  if (cached) {
    return c.json({ success: true, data: cached })
  }

  try {
    const metadata = await fetchAndFlattenMetadata()
    // Cache even if empty (to avoid repeated fetch on failures)
    c.get("runBackground")(
      c.env.CACHE.put(METADATA_KV_KEY, JSON.stringify(metadata), {
        expirationTtl: METADATA_TTL,
      }),
    )
    return c.json({ success: true, data: metadata })
  } catch {
    return c.json({ success: true, data: {} })
  }
})

// Refresh metadata cache
modelRoutes.post("/metadata/refresh", async (c) => {
  await c.env.CACHE.delete(METADATA_KV_KEY)
  try {
    const metadata = await fetchAndFlattenMetadata()
    c.get("runBackground")(
      c.env.CACHE.put(METADATA_KV_KEY, JSON.stringify(metadata), {
        expirationTtl: METADATA_TTL,
      }),
    )
    return c.json({ success: true, data: { count: Object.keys(metadata).length } })
  } catch (err) {
    const message = err instanceof Error ? err.message : "Refresh failed"
    return c.json({ success: false, error: message }, 500)
  }
})

// Supported providers for price sync
const SUPPORTED_PROVIDERS = [
  "openai",
  "anthropic",
  "google",
  "deepseek",
  "xai",
  "alibaba",
  "zhipuai",
  "minimax",
  "moonshotai",
]

interface ModelsDevEntry {
  id?: string
  cost?: {
    input?: number
    output?: number
    cache_read?: number
    cache_write?: number
  }
}

interface ModelsDevProvider {
  models?: Record<string, ModelsDevEntry>
}

type ModelsDevResponse = Record<string, ModelsDevProvider>

// Sync prices from models.dev
export async function syncPricesFromModelsDev(db: Database) {
  const resp = await fetch("https://models.dev/api.json")
  if (!resp.ok) {
    throw new Error(`Failed to fetch models.dev: ${resp.status}`)
  }

  const data = (await resp.json()) as ModelsDevResponse

  let synced = 0
  let updated = 0

  for (const providerName of SUPPORTED_PROVIDERS) {
    const provider = data[providerName]
    if (!provider?.models) continue

    for (const [, modelInfo] of Object.entries(provider.models)) {
      const modelId = modelInfo.id
      if (!modelId) continue

      const inputPrice = modelInfo.cost?.input ?? 0
      const outputPrice = modelInfo.cost?.output ?? 0

      if (inputPrice === 0 && outputPrice === 0) continue

      const result = await upsertLLMPrice(db, {
        name: modelId,
        inputPrice,
        outputPrice,
        cacheReadPrice: modelInfo.cost?.cache_read ?? 0,
        cacheWritePrice: modelInfo.cost?.cache_write ?? 0,
        source: "sync",
      })

      if (result === "created") {
        synced++
      } else {
        updated++
      }
    }
  }

  await setLastPriceSyncTime(db)
  return { synced, updated }
}

modelRoutes.post("/update-price", async (c) => {
  const db = c.env.DB

  try {
    const result = await syncPricesFromModelsDev(db)
    return c.json({ success: true, data: result })
  } catch (err) {
    const message = err instanceof Error ? err.message : "Price sync failed"
    return c.json({ success: false, error: message }, 500)
  }
})

// Get last price sync time
modelRoutes.get("/last-update-time", async (c) => {
  const db = c.env.DB
  const lastUpdate = await getLastPriceSyncTime(db)
  return c.json({ success: true, data: { lastUpdateTime: lastUpdate } })
})

export { modelRoutes }

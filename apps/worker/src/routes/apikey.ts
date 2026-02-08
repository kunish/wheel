import type { AppBindings, AppEnv } from "../runtime/types"
import { Hono } from "hono"
import { createApiKey, deleteApiKey, listApiKeys, updateApiKey } from "../db/dal/apikeys"
import { apiKeyAuth } from "../middleware/apikey"

const apikeyRoutes = new Hono<AppEnv>()

apikeyRoutes.get("/list", async (c) => {
  const db = c.env.DB
  const apiKeys = await listApiKeys(db)
  return c.json({ success: true, data: { apiKeys } })
})

apikeyRoutes.post("/create", async (c) => {
  const body = await c.req.json()
  const db = c.env.DB
  const key = await createApiKey(db, {
    name: body.name,
    expireAt: body.expireAt ?? 0,
    maxCost: body.maxCost ?? 0,
    supportedModels: body.supportedModels ?? "",
  })
  return c.json({ success: true, data: key })
})

apikeyRoutes.post("/update", async (c) => {
  const body = await c.req.json()
  const db = c.env.DB
  const { id, ...data } = body
  const key = await updateApiKey(db, id, data)
  return c.json({ success: true, data: key })
})

apikeyRoutes.delete("/delete/:id", async (c) => {
  const id = Number(c.req.param("id"))
  const db = c.env.DB
  await deleteApiKey(db, id)
  return c.json({ success: true })
})

// ─── API Key authenticated routes ───────────────
// These use API Key auth (not JWT) for end-user access

interface ApiKeyEnv {
  Bindings: AppBindings
  Variables: { apiKeyId: number; supportedModels: string }
}

const apikeyUserRoutes = new Hono<ApiKeyEnv>()
apikeyUserRoutes.use("/*", apiKeyAuth())

// Validate API key and return key info
apikeyUserRoutes.get("/login", async (c) => {
  const db = c.env.DB
  const apiKeyId = c.get("apiKeyId")
  const keys = await listApiKeys(db)
  const key = keys.find((k) => k.id === apiKeyId)
  if (!key) return c.json({ success: false, error: "API key not found" }, 404)
  return c.json({
    success: true,
    data: {
      id: key.id,
      name: key.name,
      enabled: key.enabled,
      expireAt: key.expireAt,
      maxCost: key.maxCost,
      totalCost: key.totalCost,
      supportedModels: key.supportedModels,
    },
  })
})

// Get stats for the authenticated API key
apikeyUserRoutes.get("/stats", async (c) => {
  const db = c.env.DB
  const apiKeyId = c.get("apiKeyId")
  const keys = await listApiKeys(db)
  const key = keys.find((k) => k.id === apiKeyId)
  if (!key) return c.json({ success: false, error: "API key not found" }, 404)
  return c.json({
    success: true,
    data: {
      id: key.id,
      name: key.name,
      totalCost: key.totalCost,
      maxCost: key.maxCost,
      enabled: key.enabled,
      expireAt: key.expireAt,
    },
  })
})

export { apikeyRoutes, apikeyUserRoutes }

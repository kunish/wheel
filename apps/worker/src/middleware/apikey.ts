import type { MiddlewareHandler } from "hono"
import type { AppBindings } from "../runtime/types"
import { getApiKeyByKey } from "../db/dal/apikeys"

interface ApiKeyVariables {
  apiKeyId: number
  supportedModels: string
}

export const apiKeyAuth = (): MiddlewareHandler<{
  Bindings: AppBindings
  Variables: ApiKeyVariables
}> => {
  return async (c, next) => {
    // Extract key from multiple header formats
    let key = c.req.header("x-api-key") ?? ""
    if (!key) {
      const authHeader = c.req.header("Authorization") ?? ""
      if (authHeader.startsWith("Bearer sk-wheel-")) {
        key = authHeader.slice(7)
      }
    }

    if (!key || !key.startsWith("sk-wheel-")) {
      return c.json({ success: false, error: "Unauthorized: invalid API key" }, 401)
    }

    const db = c.env.DB
    const apiKey = await getApiKeyByKey(db, key)

    if (!apiKey) {
      return c.json({ success: false, error: "Unauthorized: API key not found" }, 401)
    }

    if (!apiKey.enabled) {
      return c.json({ success: false, error: "Unauthorized: API key disabled" }, 401)
    }

    // Check expiry
    if (apiKey.expireAt > 0 && apiKey.expireAt < Math.floor(Date.now() / 1000)) {
      return c.json({ success: false, error: "Forbidden: API key expired" }, 403)
    }

    // Check cost limit
    if (apiKey.maxCost > 0 && apiKey.totalCost >= apiKey.maxCost) {
      return c.json({ success: false, error: "Forbidden: cost limit exceeded" }, 403)
    }

    c.set("apiKeyId", apiKey.id)
    c.set("supportedModels", apiKey.supportedModels)

    await next()
  }
}

export function checkModelAccess(supportedModels: string, model: string): boolean {
  if (!supportedModels) return true
  const allowed = supportedModels.split(",").map((m) => m.trim())
  return allowed.includes(model)
}

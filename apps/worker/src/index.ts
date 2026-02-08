import type { AppEnv, Database, IKVStore } from "./runtime/types"
import { Hono } from "hono"
import { cors } from "hono/cors"
import { createDb } from "./db"
import { jwtAuth } from "./middleware/jwt"
import { relayRoutes } from "./relay/handler"
import { syncAllModels } from "./relay/sync"
import { apikeyRoutes, apikeyUserRoutes } from "./routes/apikey"
import { channelRoutes } from "./routes/channel"
import { groupRoutes } from "./routes/group"
import { logRoutes } from "./routes/log"
import { modelRoutes, syncPricesFromModelsDev } from "./routes/model"
import { settingRoutes } from "./routes/setting"
import { statsRoutes } from "./routes/stats"
import { userRoutes } from "./routes/user"
import { CfKV, createCfRunBackground } from "./runtime/cf"
import { addClient } from "./ws/hub"

interface CfBindings {
  DB: D1Database
  CACHE: KVNamespace
  JWT_SECRET: string
  ADMIN_USERNAME: string
  ADMIN_PASSWORD: string
}

// Module-level singletons: created once per isolate from the raw CF bindings.
// We store the original D1/KV references to detect if env has already been adapted.
let _db: Database | null = null
let _rawD1: D1Database | null = null
let _cache: IKVStore | null = null
let _rawKV: KVNamespace | null = null

const app = new Hono<AppEnv>()

// CORS
app.use("/*", cors())

// Middleware: adapt CF-specific bindings into platform-agnostic AppBindings.
// Caches wrappers per isolate so the same Drizzle/CfKV instance is reused.
app.use("/*", async (c, next) => {
  const env = c.env as unknown as CfBindings & Record<string, unknown>

  // Only wrap if DB is still a raw D1Database (has .prepare method)
  if (typeof (env.DB as any)?.prepare === "function") {
    const rawD1 = env.DB as D1Database
    if (_rawD1 !== rawD1 || !_db) {
      _db = createDb(rawD1)
      _rawD1 = rawD1
    }
    ;(env as any).DB = _db
  }

  if (typeof (env.CACHE as any)?.list === "function") {
    const rawKV = env.CACHE as KVNamespace
    if (_rawKV !== rawKV || !_cache) {
      _cache = new CfKV(rawKV)
      _rawKV = rawKV
    }
    ;(env as any).CACHE = _cache
  }

  const runBg = createCfRunBackground(c.executionCtx.waitUntil.bind(c.executionCtx))
  c.set("runBackground", runBg)

  await next()
})

// Health check
app.get("/", (c) => {
  return c.json({ name: "wheel", version: "0.1.0" })
})

// Public: login (no auth required)
app.route("/api/v1/user", userRoutes)

// API Key authenticated endpoints (for end-user access)
app.route("/api/v1/user/apikey", apikeyUserRoutes)

// WebSocket endpoint for real-time stats push (no auth, must be before admin routes)
app.get("/api/v1/ws", (c) => {
  const upgradeHeader = c.req.header("Upgrade")
  if (upgradeHeader !== "websocket") {
    return c.text("Expected WebSocket upgrade", 426)
  }
  const pair = new WebSocketPair()
  const [client, server] = Object.values(pair)
  server.accept()
  addClient(server)
  return new Response(null, { status: 101, webSocket: client })
})

// Admin API: JWT protected
const admin = new Hono<AppEnv>()
admin.use("/*", jwtAuth())
admin.route("/channel", channelRoutes)
admin.route("/group", groupRoutes)
admin.route("/apikey", apikeyRoutes)
admin.route("/log", logRoutes)
admin.route("/stats", statsRoutes)
admin.route("/setting", settingRoutes)
admin.route("/model", modelRoutes)

app.route("/api/v1", admin)

// Relay proxy: API Key protected
app.route("/v1", relayRoutes)

export default {
  fetch: app.fetch,
  async scheduled(_event: ScheduledEvent, env: CfBindings) {
    const db = createDb(env.DB)
    const cache = new CfKV(env.CACHE)
    await Promise.allSettled([syncPricesFromModelsDev(db), syncAllModels({ DB: db, CACHE: cache })])
  },
}

import type { AppEnv } from "./runtime/types"
import * as fs from "node:fs"
import * as path from "node:path"
import { serve } from "@hono/node-server"
import { createNodeWebSocket } from "@hono/node-ws"
/**
 * Node.js entry point for Wheel (self-hosted mode).
 * Uses better-sqlite3 for DB and in-memory KV for caching.
 */
import { Hono } from "hono"
import { cors } from "hono/cors"
import cron from "node-cron"
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
import { createNodeDb, MemoryKV, nodeRunBackground } from "./runtime/node"
import { addClient } from "./ws/hub"

// ─── Config from env ───────────────────────────
const PORT = Number.parseInt(process.env.PORT || "8787", 10)
const DB_PATH = process.env.DB_PATH || "./data/wheel.db"
const JWT_SECRET = process.env.JWT_SECRET || "change-me-in-production"
const ADMIN_USERNAME = process.env.ADMIN_USERNAME || "admin"
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || "admin"

// Ensure data directory exists
const dataDir = path.dirname(DB_PATH)
if (!fs.existsSync(dataDir)) {
  fs.mkdirSync(dataDir, { recursive: true })
}

// ─── Auto-apply migrations ────────────────────
applyMigrations(DB_PATH)

// ─── Initialize database ──────────────────────
const db = createNodeDb(DB_PATH)

// ─── Create shared instances ───────────────────
const cache = new MemoryKV()

// ─── Hono app ──────────────────────────────────
const app = new Hono<AppEnv>()

// WebSocket setup for Node.js
const { injectWebSocket, upgradeWebSocket } = createNodeWebSocket({ app })

// CORS
app.use("/*", cors())

// Inject platform-agnostic bindings
app.use("/*", async (c, next) => {
  ;(c.env as any).DB = db
  ;(c.env as any).CACHE = cache
  ;(c.env as any).JWT_SECRET = JWT_SECRET
  ;(c.env as any).ADMIN_USERNAME = ADMIN_USERNAME
  ;(c.env as any).ADMIN_PASSWORD = ADMIN_PASSWORD
  c.set("runBackground", nodeRunBackground)
  await next()
})

// Health check
app.get("/", (c) => {
  return c.json({ name: "wheel", version: "0.1.0", runtime: "node" })
})

// Public: login
app.route("/api/v1/user", userRoutes)

// API Key authenticated endpoints (for end-user access)
app.route("/api/v1/user/apikey", apikeyUserRoutes)

// WebSocket endpoint using @hono/node-ws
app.get(
  "/api/v1/ws",
  upgradeWebSocket(() => ({
    onOpen(_evt, ws) {
      addClient(ws.raw as any)
    },
  })),
)

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

// ─── Start server ──────────────────────────────
const server = serve({ fetch: app.fetch, port: PORT }, (info) => {
  console.log(`Wheel (Node.js) listening on http://localhost:${info.port}`)
})

// Inject WebSocket support into the HTTP server
injectWebSocket(server)

// ─── Scheduled tasks (cron) ────────────────────
cron.schedule("0 */6 * * *", async () => {
  console.log("[cron] Running scheduled sync...")
  try {
    await Promise.allSettled([syncPricesFromModelsDev(db), syncAllModels({ DB: db, CACHE: cache })])
    console.log("[cron] Scheduled sync completed")
  } catch (err) {
    console.error("[cron] Scheduled sync error:", err)
  }
})

// ─── Migration helper ──────────────────────────
function applyMigrations(dbPath: string) {
  const BetterSqlite3 = require("better-sqlite3")
  const sqlite = new BetterSqlite3(dbPath)
  sqlite.pragma("journal_mode = WAL")

  const migrationsDir = path.resolve(__dirname, "../drizzle")
  if (!fs.existsSync(migrationsDir)) {
    console.log("[migration] No migrations directory found, skipping")
    sqlite.close()
    return
  }

  // Create migration tracking table
  sqlite.exec(`
    CREATE TABLE IF NOT EXISTS _drizzle_migrations (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      hash TEXT NOT NULL UNIQUE,
      created_at INTEGER NOT NULL DEFAULT (unixepoch())
    )
  `)

  const applied = new Set(
    sqlite
      .prepare("SELECT hash FROM _drizzle_migrations")
      .all()
      .map((r: any) => r.hash),
  )

  const sqlFiles = fs
    .readdirSync(migrationsDir)
    .filter((f: string) => f.endsWith(".sql"))
    .sort()

  for (const file of sqlFiles) {
    if (applied.has(file)) continue

    const migrationSql = fs.readFileSync(path.join(migrationsDir, file), "utf8")
    const statements = migrationSql
      .split(/-->\s*statement-breakpoint/)
      .map((s: string) => s.trim())
      .filter((s: string) => s.length > 0)

    const transaction = sqlite.transaction(() => {
      for (const stmt of statements) {
        sqlite.exec(stmt)
      }
      sqlite.prepare("INSERT INTO _drizzle_migrations (hash) VALUES (?)").run(file)
    })

    transaction()
    console.log(`[migration] Applied: ${file}`)
  }

  sqlite.close()
}

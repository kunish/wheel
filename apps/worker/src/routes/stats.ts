import type { AppEnv } from "../runtime/types"
import { Hono } from "hono"
import {
  getApiKeyStats,
  getChannelStats,
  getDailyStats,
  getGlobalStats,
  getHourlyStats,
  getModelStats,
  getTodayStats,
  getTotalStats,
} from "../db/dal/stats"

const statsRoutes = new Hono<AppEnv>()

statsRoutes.get("/global", async (c) => {
  const db = c.env.DB
  const stats = await getGlobalStats(db)
  return c.json({ success: true, data: stats })
})

statsRoutes.get("/channel", async (c) => {
  const db = c.env.DB
  const stats = await getChannelStats(db)
  return c.json({ success: true, data: stats })
})

statsRoutes.get("/total", async (c) => {
  const db = c.env.DB
  const stats = await getTotalStats(db)
  return c.json({ success: true, data: stats })
})

statsRoutes.get("/today", async (c) => {
  const db = c.env.DB
  const tz = c.req.query("tz")
  const stats = await getTodayStats(db, tz)
  return c.json({ success: true, data: stats })
})

statsRoutes.get("/daily", async (c) => {
  const db = c.env.DB
  const tz = c.req.query("tz")
  const stats = await getDailyStats(db, tz)
  return c.json({ success: true, data: stats })
})

statsRoutes.get("/hourly", async (c) => {
  const db = c.env.DB
  const start = c.req.query("start")
  const end = c.req.query("end")
  const tz = c.req.query("tz")
  const stats = await getHourlyStats(db, start, end, tz)
  return c.json({ success: true, data: stats })
})

statsRoutes.get("/model", async (c) => {
  const db = c.env.DB
  const stats = await getModelStats(db)
  return c.json({ success: true, data: stats })
})

statsRoutes.get("/apikey", async (c) => {
  const db = c.env.DB
  const stats = await getApiKeyStats(db)
  return c.json({ success: true, data: stats })
})

export { statsRoutes }

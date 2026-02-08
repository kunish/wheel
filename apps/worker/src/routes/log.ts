import type { AppEnv } from "../runtime/types"
import { Hono } from "hono"
import { clearLogs, deleteLog, getLog, listLogs } from "../db/dal/logs"

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

export { logRoutes }

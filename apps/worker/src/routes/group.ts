import type { AppEnv } from "../runtime/types"
import { Hono } from "hono"
import { listChannels } from "../db/dal/channels"
import { createGroup, deleteGroup, listGroups, updateGroup } from "../db/dal/groups"

const groupRoutes = new Hono<AppEnv>()

groupRoutes.get("/list", async (c) => {
  const db = c.env.DB
  const groups = await listGroups(db)
  return c.json({ success: true, data: { groups } })
})

groupRoutes.post("/create", async (c) => {
  const body = await c.req.json()
  const db = c.env.DB
  const g = await createGroup(
    db,
    {
      name: body.name,
      mode: body.mode,
      matchRegex: body.matchRegex ?? "",
      firstTokenTimeOut: body.firstTokenTimeOut ?? 30,
    },
    body.items ?? [],
  )
  await c.env.CACHE.delete("groups")
  return c.json({ success: true, data: g })
})

groupRoutes.post("/update", async (c) => {
  const body = await c.req.json()
  const db = c.env.DB
  const { id, items, ...data } = body
  const g = await updateGroup(db, id, data, items)
  await c.env.CACHE.delete("groups")
  return c.json({ success: true, data: g })
})

groupRoutes.delete("/delete/:id", async (c) => {
  const id = Number(c.req.param("id"))
  const db = c.env.DB
  await deleteGroup(db, id)
  await c.env.CACHE.delete("groups")
  return c.json({ success: true })
})

groupRoutes.get("/model-list", async (c) => {
  const db = c.env.DB
  const channels = await listChannels(db)
  const models = new Set<string>()
  for (const ch of channels) {
    if (ch.model && Array.isArray(ch.model)) {
      for (const m of ch.model) {
        if (m) models.add(m)
      }
    }
  }
  return c.json({ success: true, data: { models: [...models].sort() } })
})

export { groupRoutes }

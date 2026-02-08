import type { Database } from "../../runtime/types"
import { eq } from "drizzle-orm"
import { channelKeys, channels } from "../schema"

export async function listChannels(db: Database) {
  const rows = await db.select().from(channels)
  const keys = await db.select().from(channelKeys)
  return rows.map((ch) => ({
    ...ch,
    keys: keys.filter((k) => k.channelId === ch.id),
  }))
}

export async function getChannel(db: Database, id: number) {
  const [ch] = await db.select().from(channels).where(eq(channels.id, id))
  if (!ch) return null
  const keys = await db.select().from(channelKeys).where(eq(channelKeys.channelId, id))
  return { ...ch, keys }
}

export async function createChannel(
  db: Database,
  data: typeof channels.$inferInsert,
  keys: { channelKey: string; remark?: string }[],
) {
  const [ch] = await db.insert(channels).values(data).returning()
  if (keys.length > 0) {
    await db.insert(channelKeys).values(
      keys.map((k) => ({
        channelId: ch.id,
        channelKey: k.channelKey,
        remark: k.remark ?? "",
      })),
    )
  }
  return ch
}

export async function updateChannel(
  db: Database,
  id: number,
  data: Partial<typeof channels.$inferInsert>,
) {
  const [ch] = await db.update(channels).set(data).where(eq(channels.id, id)).returning()
  return ch
}

export async function deleteChannel(db: Database, id: number) {
  await db.delete(channels).where(eq(channels.id, id))
}

export async function enableChannel(db: Database, id: number, enabled: boolean) {
  await db.update(channels).set({ enabled }).where(eq(channels.id, id))
}

export async function syncChannelKeys(
  db: Database,
  channelId: number,
  keys: { channelKey: string; remark?: string }[],
) {
  await db.delete(channelKeys).where(eq(channelKeys.channelId, channelId))
  if (keys.length > 0) {
    await db.insert(channelKeys).values(
      keys.map((k) => ({
        channelId,
        channelKey: k.channelKey,
        remark: k.remark ?? "",
      })),
    )
  }
}

export async function updateChannelKeyStatus(db: Database, keyId: number, statusCode: number) {
  await db
    .update(channelKeys)
    .set({ statusCode, lastUseTimestamp: Math.floor(Date.now() / 1000) })
    .where(eq(channelKeys.id, keyId))
}

export async function incrementChannelKeyCost(db: Database, keyId: number, cost: number) {
  const [row] = await db.select().from(channelKeys).where(eq(channelKeys.id, keyId))
  if (!row) return
  await db
    .update(channelKeys)
    .set({ totalCost: row.totalCost + cost })
    .where(eq(channelKeys.id, keyId))
}

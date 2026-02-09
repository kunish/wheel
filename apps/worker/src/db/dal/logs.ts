import type { Database } from "../../runtime/types"
import { and, count, desc, eq, gte, like, lte, or, sql } from "drizzle-orm"
import { relayLogs } from "../schema"

export async function createLog(db: Database, data: typeof relayLogs.$inferInsert) {
  const [row] = await db.insert(relayLogs).values(data).returning()
  return row
}

export async function getLog(db: Database, id: number) {
  const [row] = await db.select().from(relayLogs).where(eq(relayLogs.id, id))
  return row ?? null
}

export async function listLogs(
  db: Database,
  opts: {
    page?: number
    pageSize?: number
    model?: string
    channelId?: number
    hasError?: boolean
    startTime?: number
    endTime?: number
    keyword?: string
  },
) {
  const page = opts.page ?? 1
  const pageSize = opts.pageSize ?? 20
  const offset = (page - 1) * pageSize

  const conditions = []
  if (opts.model) {
    conditions.push(like(relayLogs.requestModelName, `%${opts.model}%`))
  }
  if (opts.channelId) {
    conditions.push(eq(relayLogs.channelId, opts.channelId))
  }
  if (opts.hasError !== undefined) {
    if (opts.hasError) {
      conditions.push(sql`${relayLogs.error} != ''`)
    } else {
      conditions.push(eq(relayLogs.error, ""))
    }
  }
  if (opts.startTime) {
    conditions.push(gte(relayLogs.time, opts.startTime))
  }
  if (opts.endTime) {
    conditions.push(lte(relayLogs.time, opts.endTime))
  }
  if (opts.keyword) {
    const pattern = `%${opts.keyword}%`
    conditions.push(
      or(
        like(relayLogs.requestModelName, pattern),
        like(relayLogs.channelName, pattern),
        like(relayLogs.error, pattern),
        like(relayLogs.requestContent, pattern),
        like(relayLogs.responseContent, pattern),
      ),
    )
  }

  const where = conditions.length > 0 ? and(...conditions) : undefined

  const [rows, [{ total }]] = await Promise.all([
    db
      .select({
        id: relayLogs.id,
        time: relayLogs.time,
        requestModelName: relayLogs.requestModelName,
        actualModelName: relayLogs.actualModelName,
        channelId: relayLogs.channelId,
        channelName: relayLogs.channelName,
        inputTokens: relayLogs.inputTokens,
        outputTokens: relayLogs.outputTokens,
        ftut: relayLogs.ftut,
        useTime: relayLogs.useTime,
        cost: relayLogs.cost,
        error: relayLogs.error,
        totalAttempts: relayLogs.totalAttempts,
      })
      .from(relayLogs)
      .where(where)
      .orderBy(desc(relayLogs.time))
      .limit(pageSize)
      .offset(offset),
    db.select({ total: count() }).from(relayLogs).where(where),
  ])

  return { logs: rows, total, page, pageSize }
}

export async function deleteLog(db: Database, id: number) {
  await db.delete(relayLogs).where(eq(relayLogs.id, id))
}

export async function clearLogs(db: Database) {
  await db.delete(relayLogs)
}

export async function cleanupOldLogs(db: Database, retentionDays: number) {
  const cutoff = Math.floor(Date.now() / 1000) - retentionDays * 86400
  const result = await db
    .delete(relayLogs)
    .where(lte(relayLogs.time, cutoff))
    .returning({ id: relayLogs.id })
  return result.length
}

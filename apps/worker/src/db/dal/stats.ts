import type { Database } from "../../runtime/types"
import { avg, count, desc, gt, sql, sum } from "drizzle-orm"
import { apiKeys, channels, groups, relayLogs } from "../schema"

// ---------- helpers ----------

function toMetrics(row: Record<string, unknown>) {
  const inputTokens = Number(row.inputTokens ?? 0)
  const outputTokens = Number(row.outputTokens ?? 0)
  const cost = Number(row.cost ?? 0)
  const waitTime = Number(row.waitTime ?? 0)
  const successCount = Number(row.successCount ?? 0)
  const failedCount = Number(row.failedCount ?? 0)
  return {
    input_token: inputTokens,
    output_token: outputTokens,
    input_cost: cost * 0.6,
    output_cost: cost * 0.4,
    wait_time: waitTime,
    request_success: successCount,
    request_failed: failedCount,
  }
}

/**
 * Convert a timezone offset string (e.g. "+08:00", "-05:00") to a SQLite
 * modifier string like "+28800 seconds".
 * Falls back to "+0 seconds" (UTC) for invalid input.
 */
function tzModifier(tz?: string): string {
  if (!tz) return "+0 seconds"
  const m = /^([+-])(\d{1,2}):(\d{2})$/.exec(tz)
  if (!m) return "+0 seconds"
  const sign = m[1] === "-" ? -1 : 1
  const secs = sign * (Number(m[2]) * 3600 + Number(m[3]) * 60)
  return `${secs >= 0 ? "+" : ""}${secs} seconds`
}

/** Parse "+08:00" to offset in minutes (e.g. 480). */
function parseTzMinutes(tz?: string): number {
  if (!tz) return 0
  const m = /^([+-])(\d{1,2}):(\d{2})$/.exec(tz)
  if (!m) return 0
  const sign = m[1] === "-" ? -1 : 1
  return sign * (Number(m[2]) * 60 + Number(m[3]))
}

/** Build a local-date string (YYYYMMDD) from a JS Date in the given tz offset. */
function localDateStr(d: Date, tzOffsetMinutes: number): string {
  const utc = d.getTime() + d.getTimezoneOffset() * 60000
  const local = new Date(utc + tzOffsetMinutes * 60000)
  return (
    local.getFullYear().toString() +
    (local.getMonth() + 1).toString().padStart(2, "0") +
    local.getDate().toString().padStart(2, "0")
  )
}

const metricsSelect = {
  inputTokens: sum(relayLogs.inputTokens),
  outputTokens: sum(relayLogs.outputTokens),
  cost: sum(relayLogs.cost),
  waitTime: sum(relayLogs.useTime),
  successCount: sql<number>`sum(case when ${relayLogs.error} = '' then 1 else 0 end)`,
  failedCount: sql<number>`sum(case when ${relayLogs.error} != '' then 1 else 0 end)`,
}

// ---------- total ----------

export async function getTotalStats(db: Database) {
  const [row] = await db.select(metricsSelect).from(relayLogs)
  return toMetrics(row as Record<string, unknown>)
}

// ---------- today ----------

export async function getTodayStats(db: Database, tz?: string) {
  const mod = tzModifier(tz)
  const todayStr = localDateStr(new Date(), parseTzMinutes(tz))
  const [row] = await db
    .select(metricsSelect)
    .from(relayLogs)
    .where(
      sql`strftime('%Y%m%d', ${relayLogs.time}, 'unixepoch', ${sql.raw(`'${mod}'`)}) = ${todayStr}`,
    )
  return { ...toMetrics(row as Record<string, unknown>), date: todayStr }
}

// ---------- daily (last 1 year) ----------

export async function getDailyStats(db: Database, tz?: string) {
  const mod = tzModifier(tz)
  const rows = await db
    .select({
      date: sql<string>`strftime('%Y%m%d', ${relayLogs.time}, 'unixepoch', ${sql.raw(`'${mod}'`)})`.as(
        "date",
      ),
      ...metricsSelect,
    })
    .from(relayLogs)
    .where(sql`${relayLogs.time} >= unixepoch('now', '-365 days')`)
    .groupBy(sql`date`)
    .orderBy(sql`date`)

  return rows.map((r) => ({
    ...toMetrics(r as unknown as Record<string, unknown>),
    date: r.date,
  }))
}

// ---------- hourly (today, or date range) ----------

export async function getHourlyStats(
  db: Database,
  startDate?: string,
  endDate?: string,
  tz?: string,
) {
  const mod = tzModifier(tz)
  const start = startDate ?? localDateStr(new Date(), parseTzMinutes(tz))
  const end = endDate ?? start
  const rows = await db
    .select({
      hour: sql<number>`cast(strftime('%H', ${relayLogs.time}, 'unixepoch', ${sql.raw(`'${mod}'`)}) as integer)`.as(
        "hour",
      ),
      date: sql<string>`strftime('%Y%m%d', ${relayLogs.time}, 'unixepoch', ${sql.raw(`'${mod}'`)})`.as(
        "date",
      ),
      ...metricsSelect,
    })
    .from(relayLogs)
    .where(
      sql`strftime('%Y%m%d', ${relayLogs.time}, 'unixepoch', ${sql.raw(`'${mod}'`)}) >= ${start} AND strftime('%Y%m%d', ${relayLogs.time}, 'unixepoch', ${sql.raw(`'${mod}'`)}) <= ${end}`,
    )
    .groupBy(sql`date`, sql`hour`)
    .orderBy(sql`date`, sql`hour`)

  return rows.map((r) => ({
    ...toMetrics(r as unknown as Record<string, unknown>),
    hour: r.hour,
    date: r.date,
  }))
}

// ---------- legacy / existing ----------

export async function getGlobalStats(db: Database) {
  const [logStats] = await db
    .select({
      totalRequests: count(),
      totalInputTokens: sum(relayLogs.inputTokens),
      totalOutputTokens: sum(relayLogs.outputTokens),
      totalCost: sum(relayLogs.cost),
    })
    .from(relayLogs)

  const [{ activeChannels }] = await db
    .select({ activeChannels: count() })
    .from(channels)
    .where(sql`${channels.enabled} = 1`)

  const [{ activeGroups }] = await db.select({ activeGroups: count() }).from(groups)

  return {
    totalRequests: logStats.totalRequests ?? 0,
    totalInputTokens: Number(logStats.totalInputTokens ?? 0),
    totalOutputTokens: Number(logStats.totalOutputTokens ?? 0),
    totalCost: Number(logStats.totalCost ?? 0),
    activeChannels: activeChannels ?? 0,
    activeGroups: activeGroups ?? 0,
  }
}

export async function getChannelStats(db: Database) {
  const rows = await db
    .select({
      channelId: relayLogs.channelId,
      channelName: relayLogs.channelName,
      ...metricsSelect,
      totalRequests: count(),
      avgLatency: avg(relayLogs.useTime),
    })
    .from(relayLogs)
    .where(gt(relayLogs.channelId, 0))
    .groupBy(relayLogs.channelId, relayLogs.channelName)

  return rows.map((s) => ({
    channelId: s.channelId,
    channelName: s.channelName,
    totalRequests: s.totalRequests,
    totalCost: Number(s.cost ?? 0),
    avgLatency: Number(s.avgLatency ?? 0),
    ...toMetrics(s as unknown as Record<string, unknown>),
  }))
}

// ---------- model ----------

export async function getModelStats(db: Database) {
  const rows = await db
    .select({
      model: relayLogs.requestModelName,
      ...metricsSelect,
      totalRequests: count(),
      avgLatency: avg(relayLogs.useTime),
      avgFtut: avg(relayLogs.ftut),
    })
    .from(relayLogs)
    .groupBy(relayLogs.requestModelName)
    .orderBy(desc(count()))

  return rows.map((r) => ({
    model: r.model,
    requestCount: r.totalRequests,
    inputTokens: Number(r.inputTokens ?? 0),
    outputTokens: Number(r.outputTokens ?? 0),
    totalCost: Number(r.cost ?? 0),
    avgLatency: Math.round(Number(r.avgLatency ?? 0)),
    avgFirstTokenTime: Math.round(Number(r.avgFtut ?? 0)),
  }))
}

// ---------- apikey ----------

export async function getApiKeyStats(db: Database) {
  const keys = await db.select().from(apiKeys)
  return keys.map((k) => ({
    id: k.id,
    name: k.name,
    enabled: k.enabled,
    totalCost: k.totalCost,
    maxCost: k.maxCost,
    expireAt: k.expireAt,
  }))
}

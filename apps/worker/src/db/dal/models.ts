import type { Database } from "../../runtime/types"
import { eq, sql } from "drizzle-orm"
import { llmPrices, settings } from "../schema"

export async function listLLMPrices(db: Database) {
  return db.select().from(llmPrices).orderBy(llmPrices.name)
}

export async function getLLMPriceByName(db: Database, name: string) {
  const rows = await db.select().from(llmPrices).where(eq(llmPrices.name, name)).limit(1)
  return rows[0] ?? null
}

export async function createLLMPrice(
  db: Database,
  data: {
    name: string
    inputPrice: number
    outputPrice: number
    source?: string
  },
) {
  const result = await db
    .insert(llmPrices)
    .values({
      name: data.name,
      inputPrice: data.inputPrice,
      outputPrice: data.outputPrice,
      source: data.source ?? "manual",
    })
    .returning()
  return result[0]
}

export async function updateLLMPrice(
  db: Database,
  id: number,
  data: Partial<{
    name: string
    inputPrice: number
    outputPrice: number
    cacheReadPrice: number
    cacheWritePrice: number
  }>,
) {
  const result = await db
    .update(llmPrices)
    .set({
      ...data,
      updatedAt: sql`(datetime('now'))`,
    })
    .where(eq(llmPrices.id, id))
    .returning()
  return result[0]
}

export async function deleteLLMPrice(db: Database, id: number) {
  await db.delete(llmPrices).where(eq(llmPrices.id, id))
}

export async function upsertLLMPrice(
  db: Database,
  data: {
    name: string
    inputPrice: number
    outputPrice: number
    cacheReadPrice?: number
    cacheWritePrice?: number
    source: string
  },
): Promise<"created" | "updated"> {
  const existing = await getLLMPriceByName(db, data.name)
  if (existing) {
    // Only update if the source is 'sync' (don't overwrite manual prices)
    if (existing.source === "sync") {
      await db
        .update(llmPrices)
        .set({
          inputPrice: data.inputPrice,
          outputPrice: data.outputPrice,
          cacheReadPrice: data.cacheReadPrice ?? 0,
          cacheWritePrice: data.cacheWritePrice ?? 0,
          updatedAt: sql`(datetime('now'))`,
        })
        .where(eq(llmPrices.id, existing.id))
      return "updated"
    }
    return "updated"
  }
  await db.insert(llmPrices).values({
    name: data.name,
    inputPrice: data.inputPrice,
    outputPrice: data.outputPrice,
    cacheReadPrice: data.cacheReadPrice ?? 0,
    cacheWritePrice: data.cacheWritePrice ?? 0,
    source: data.source,
  })
  return "created"
}

export async function setLastPriceSyncTime(db: Database) {
  const now = new Date().toISOString()
  await db
    .insert(settings)
    .values({ key: "last_price_sync_time", value: now })
    .onConflictDoUpdate({
      target: settings.key,
      set: { value: now },
    })
}

export async function getLastPriceSyncTime(db: Database): Promise<string | null> {
  const rows = await db
    .select()
    .from(settings)
    .where(eq(settings.key, "last_price_sync_time"))
    .limit(1)
  return rows[0]?.value ?? null
}

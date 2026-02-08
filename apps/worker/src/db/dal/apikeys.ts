import type { Database } from "../../runtime/types"
import { eq } from "drizzle-orm"
import { apiKeys } from "../schema"

const CHARSET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

function generateApiKey(): string {
  const bytes = new Uint8Array(48)
  crypto.getRandomValues(bytes)
  let key = "sk-wheel-"
  for (const b of bytes) {
    key += CHARSET[b % CHARSET.length]
  }
  return key
}

export async function listApiKeys(db: Database) {
  return db.select().from(apiKeys)
}

export async function createApiKey(
  db: Database,
  data: Omit<typeof apiKeys.$inferInsert, "apiKey">,
) {
  const key = generateApiKey()
  const [row] = await db
    .insert(apiKeys)
    .values({ ...data, apiKey: key })
    .returning()
  return row
}

export async function updateApiKey(
  db: Database,
  id: number,
  data: Partial<typeof apiKeys.$inferInsert>,
) {
  const [row] = await db.update(apiKeys).set(data).where(eq(apiKeys.id, id)).returning()
  return row
}

export async function deleteApiKey(db: Database, id: number) {
  await db.delete(apiKeys).where(eq(apiKeys.id, id))
}

export async function getApiKeyByKey(db: Database, key: string) {
  const [row] = await db.select().from(apiKeys).where(eq(apiKeys.apiKey, key))
  return row ?? null
}

export async function incrementApiKeyCost(db: Database, id: number, cost: number) {
  const [row] = await db.select().from(apiKeys).where(eq(apiKeys.id, id))
  if (!row) return
  await db
    .update(apiKeys)
    .set({ totalCost: row.totalCost + cost })
    .where(eq(apiKeys.id, id))
}

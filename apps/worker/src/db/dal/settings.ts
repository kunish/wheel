import type { Database } from "../../runtime/types"
import { eq } from "drizzle-orm"
import { settings } from "../schema"

export async function getAllSettings(db: Database) {
  const rows = await db.select().from(settings)
  return Object.fromEntries(rows.map((r) => [r.key, r.value]))
}

export async function getSetting(db: Database, key: string): Promise<string | null> {
  const rows = await db.select().from(settings).where(eq(settings.key, key)).limit(1)
  return rows[0]?.value ?? null
}

export async function updateSettings(db: Database, data: Record<string, string>) {
  for (const [key, value] of Object.entries(data)) {
    await db
      .insert(settings)
      .values({ key, value })
      .onConflictDoUpdate({ target: settings.key, set: { value } })
  }
}

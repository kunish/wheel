import type { Database } from "../../runtime/types"
import { eq } from "drizzle-orm"
import { groupItems, groups } from "../schema"

// D1/SQLite limits ~100 bind variables per statement; group_items has 6 columns
const BATCH_SIZE = 16

async function batchInsertItems(db: Database, items: (typeof groupItems.$inferInsert)[]) {
  for (let i = 0; i < items.length; i += BATCH_SIZE) {
    await db.insert(groupItems).values(items.slice(i, i + BATCH_SIZE))
  }
}

export async function listGroups(db: Database) {
  const rows = await db.select().from(groups)
  const items = await db.select().from(groupItems)
  return rows.map((g) => ({
    ...g,
    items: items.filter((i) => i.groupId === g.id),
  }))
}

export async function getGroup(db: Database, id: number) {
  const [g] = await db.select().from(groups).where(eq(groups.id, id))
  if (!g) return null
  const items = await db.select().from(groupItems).where(eq(groupItems.groupId, id))
  return { ...g, items }
}

export async function createGroup(
  db: Database,
  data: typeof groups.$inferInsert,
  items: Omit<typeof groupItems.$inferInsert, "groupId">[],
) {
  const [g] = await db.insert(groups).values(data).returning()
  if (items.length > 0) {
    await batchInsertItems(
      db,
      items.map((i) => ({ ...i, groupId: g.id })),
    )
  }
  return g
}

export async function updateGroup(
  db: Database,
  id: number,
  data: Partial<typeof groups.$inferInsert>,
  items?: Omit<typeof groupItems.$inferInsert, "groupId">[],
) {
  const [g] = await db.update(groups).set(data).where(eq(groups.id, id)).returning()
  if (items) {
    await db.delete(groupItems).where(eq(groupItems.groupId, id))
    if (items.length > 0) {
      await batchInsertItems(
        db,
        items.map((i) => ({ ...i, groupId: id })),
      )
    }
  }
  return g
}

export async function deleteGroup(db: Database, id: number) {
  await db.delete(groups).where(eq(groups.id, id))
}

export async function getGroupsMap(db: Database) {
  const allGroups = await listGroups(db)
  return allGroups
}

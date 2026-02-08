import type { Database } from "../../runtime/types"
import { eq } from "drizzle-orm"
import { users } from "../schema"

export async function getUser(db: Database) {
  const [user] = await db.select().from(users).limit(1)
  return user ?? null
}

export async function createUser(db: Database, username: string, hashedPassword: string) {
  const [user] = await db.insert(users).values({ username, password: hashedPassword }).returning()
  return user
}

export async function updatePassword(db: Database, id: number, hashedPassword: string) {
  await db.update(users).set({ password: hashedPassword }).where(eq(users.id, id))
}

export async function updateUsername(db: Database, id: number, username: string) {
  await db.update(users).set({ username }).where(eq(users.id, id))
}

import type { Database } from "../runtime/types"
import { drizzle } from "drizzle-orm/d1"
import * as schema from "./schema"

export type { Database }

export function createDb(d1: D1Database): Database {
  return drizzle(d1, { schema }) as unknown as Database
}

export { schema }

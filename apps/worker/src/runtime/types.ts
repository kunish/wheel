import type { BaseSQLiteDatabase } from "drizzle-orm/sqlite-core"
import type * as schema from "../db/schema"

/** In-memory KV store interface */
export interface IKVStore {
  get: <T = unknown>(key: string, format: "json") => Promise<T | null>
  put: (key: string, value: string, opts?: { expirationTtl?: number }) => Promise<void>
  delete: (key: string) => Promise<void>
}

/** Fire-and-forget background work */
export type RunBackground = (promise: Promise<unknown>) => void

/** Database type (better-sqlite3 via Drizzle ORM) */
export type Database = BaseSQLiteDatabase<"sync", unknown, typeof schema>

/** Shared Hono Bindings used across all routes and middleware */
export interface AppBindings {
  DB: Database
  CACHE: IKVStore
  JWT_SECRET: string
  ADMIN_USERNAME: string
  ADMIN_PASSWORD: string
}

/** Shared Hono env type for routes */
export interface AppEnv {
  Bindings: AppBindings
  Variables: {
    runBackground: RunBackground
  }
}

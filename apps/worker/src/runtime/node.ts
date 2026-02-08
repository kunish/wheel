import type { Database, IKVStore, RunBackground } from "./types"

/** In-memory KV store with TTL expiration */
export class MemoryKV implements IKVStore {
  private store = new Map<string, { value: string; expires: number }>()

  async get<T = unknown>(key: string, _format: "json"): Promise<T | null> {
    const entry = this.store.get(key)
    if (!entry) return null
    if (entry.expires > 0 && Date.now() > entry.expires) {
      this.store.delete(key)
      return null
    }
    return JSON.parse(entry.value) as T
  }

  async put(key: string, value: string, opts?: { expirationTtl?: number }): Promise<void> {
    const expires = opts?.expirationTtl ? Date.now() + opts.expirationTtl * 1000 : 0
    this.store.set(key, { value, expires })
  }

  async delete(key: string): Promise<void> {
    this.store.delete(key)
  }
}

/** Node.js: fire-and-forget (process is long-lived) */
export const nodeRunBackground: RunBackground = (promise) => {
  promise.catch(() => {})
}

/** Create a Drizzle database instance from a better-sqlite3 file */
export function createNodeDb(filepath: string): Database {
  // Dynamic imports to avoid bundling in CF Workers
  const BetterSqlite3 = require("better-sqlite3")
  const { drizzle } = require("drizzle-orm/better-sqlite3")
  const schema = require("../db/schema")

  const sqlite = new BetterSqlite3(filepath)
  sqlite.pragma("journal_mode = WAL")
  return drizzle(sqlite, { schema }) as Database
}

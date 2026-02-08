import type { IKVStore, RunBackground } from "./types"

/** Cloudflare KVNamespace wrapper implementing IKVStore */
export class CfKV implements IKVStore {
  constructor(private kv: KVNamespace) {}

  get<T = unknown>(key: string, format: "json"): Promise<T | null> {
    return this.kv.get(key, format) as Promise<T | null>
  }

  put(key: string, value: string, opts?: { expirationTtl?: number }): Promise<void> {
    return this.kv.put(key, value, opts)
  }

  delete(key: string): Promise<void> {
    return this.kv.delete(key)
  }
}

/** CF Workers: delegate to executionCtx.waitUntil */
export function createCfRunBackground(
  waitUntil: (promise: Promise<unknown>) => void,
): RunBackground {
  return (promise) => waitUntil(promise)
}

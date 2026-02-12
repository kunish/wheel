## Context

Wheel Worker 当前直接使用 Cloudflare Workers API（`D1Database`、`KVNamespace`、`WebSocketPair`、`executionCtx.waitUntil()`、`ScheduledEvent`）。这些 API 在 14 个源文件中被引用。要实现 Docker 自托管，需要在不破坏 CF 部署路径的前提下，让相同的应用代码可以运行在 Node.js 环境中。

当前 CF 依赖面：

- `D1Database` — 通过 `drizzle-orm/d1` 封装，全部 DAL 通过 Drizzle 操作
- `KVNamespace` — 3 个方法：`get(key, "json")`、`put(key, val, {expirationTtl})`、`delete(key)`
- `WebSocketPair` — 仅 `index.ts` 1 处，`ws/hub.ts` 使用通用 `WebSocket` 类型
- `executionCtx.waitUntil()` — `handler.ts` 5 处 + `model.ts` 2 处
- `ScheduledEvent` — `index.ts` 1 处 cron

## Goals / Non-Goals

**Goals:**

- Worker 代码可同时运行在 CF Workers 和 Node.js 环境
- Docker Compose 一条命令启动 Web + Worker
- Node.js 模式使用 better-sqlite3 (与 D1 同为 SQLite，schema 零改动)
- 对 CF Workers 部署路径零影响

**Non-Goals:**

- 不支持 PostgreSQL / MySQL（Phase 2）
- 不支持 Redis 作为 KV 后端（Phase 2）
- 不做运行时自动检测（通过入口文件区分，不是运行时 if/else）
- 不修改现有 API 行为或数据库 schema

## Decisions

### 1. 入口分离（非运行时 if/else）

两个独立入口文件，构建时选择：

- `index.ts` → CF Workers（现有，不变）
- `index.node.ts` → Node.js（新增）

**为什么不用运行时检测**: 运行时检测引入 `typeof globalThis.D1Database !== 'undefined'` 风格的脆弱判断。入口分离更清晰，各自 import 对应的 runtime 实现，tree-shaking 也更好。

### 2. IKVStore 接口

```typescript
interface IKVStore {
  get: <T = unknown>(key: string, format: "json") => Promise<T | null>
  put: (key: string, value: string, opts?: { expirationTtl?: number }) => Promise<void>
  delete: (key: string) => Promise<void>
}
```

CF 实现：透传 `KVNamespace`。Node 实现：内存 `Map` + TTL 定时清理。

**为什么不用 Redis**: 当前 KV 仅用于缓存 channels/groups/settings，数据量极小，TTL 300s。内存 Map 完全够用，且零外部依赖。Redis 留到 Phase 2 作为可选。

### 3. Drizzle 双驱动

利用 Drizzle ORM 天然的驱动抽象：

- CF: `drizzle(d1, { schema })` → `DrizzleD1Database`
- Node: `drizzle(new Database(file), { schema })` → `BetterSQLite3Database`

两者都使用 `drizzle-orm/sqlite-core` 的 schema 定义，查询 API 完全一致。DAL 代码零改动。

**关键**: `createDb()` 返回类型需要泛化。当前返回 `DrizzleD1Database<typeof schema>`，需要改为一个联合类型或通用接口。Drizzle 的 `BaseSQLiteDatabase` 可以作为公共基类型。

### 4. waitUntil → runBackground

```typescript
type RunBackground = (promise: Promise<unknown>) => void

// CF: (p) => c.executionCtx.waitUntil(p)
// Node: (p) => { p.catch(() => {}) }  // fire-and-forget
```

Node.js 进程是常驻的，不需要延长请求生命周期。直接让 Promise 在后台执行即可。

### 5. WebSocket 适配

- CF: `WebSocketPair` → 现有代码
- Node: `@hono/node-ws` → 提供 `upgradeWebSocket()` helper

`ws/hub.ts` 使用标准 `WebSocket` 类型，Node.js 的 `ws` 库类型兼容，不需要改动。

### 6. Cron 适配

- CF: `scheduled()` export
- Node: `node-cron` 库，在 `index.node.ts` 中注册

### 7. Bindings 类型重构

将 Bindings 类型提取到共享文件 `runtime/types.ts`：

```typescript
export interface AppBindings {
  DB: unknown // drizzle 实例由 runtime 提供
  CACHE: IKVStore
  JWT_SECRET: string
  ADMIN_USERNAME: string
  ADMIN_PASSWORD: string
}
```

各路由文件 import 此类型，不再各自定义。

## Risks / Trade-offs

- **[Node.js 的 Web Crypto API 兼容性]** → JWT middleware 使用 `crypto.subtle`。Node 18+ 已原生支持，需确保 Dockerfile 使用 Node 22
- **[内存 KV 重启丢失]** → 仅缓存数据，重启后自动重建。可接受
- **[Drizzle 类型泛化]** → `BaseSQLiteDatabase` 是否涵盖所有用到的查询 API，需实际验证。降级方案：使用 `any` 类型 + 运行时测试
- **[SQLite 并发写入]** → better-sqlite3 是同步的，单进程模式不存在并发问题。如果未来需要多进程，需要引入 WAL 模式

## Migration Plan

1. 添加 `runtime/` 层和 Node.js 入口（不影响现有代码）
2. 重构 Bindings 类型为共享定义
3. 替换所有 `KVNamespace` 引用为 `IKVStore`
4. 替换所有 `waitUntil` 为 `runBackground`
5. 验证 CF Workers 部署路径不受影响（`wrangler deploy` 仍然使用 `index.ts`）
6. 添加 Worker Dockerfile + 更新 docker-compose.yml
7. 端到端测试 Docker 部署

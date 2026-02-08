## Why

Worker 当前强依赖 Cloudflare D1/KV 运行时，无法在非 Cloudflare 环境自托管。需要一个 Node.js 运行时适配层，让 Worker 既能跑在 CF Workers 上也能跑在标准 Node.js + Docker 中，实现完整的 Web + Worker 一体化自托管。

## What Changes

- 新增 `runtime/` 适配层：定义 `IKVStore` 接口和 `runBackground()` 抽象，隔离 CF 平台 API
- 新增 Node.js 运行时实现：`better-sqlite3` 驱动 + 内存 KV + `@hono/node-server` 入口
- 统一 Bindings 类型：`CACHE: KVNamespace` → `CACHE: IKVStore`，所有路由文件跟随类型变更
- 新增 `index.node.ts`：Node.js 入口，含 `@hono/node-ws` WebSocket 支持和 `node-cron` 定时任务
- 新增 Worker Dockerfile 和更新 `docker-compose.yml`：完整的 Web + Worker Docker 部署
- `c.executionCtx.waitUntil()` 调用替换为 `runBackground()` 抽象

## Capabilities

### New Capabilities

- `runtime-abstraction`: IKVStore 接口、runBackground 抽象、Bindings 类型统一
- `node-runtime`: Node.js 运行时实现（better-sqlite3、MemoryKV、node-cron、Hono Node server）
- `docker-full-stack`: Worker Dockerfile + 更新 docker-compose.yml 支持 Web + Worker 一体化部署

### Modified Capabilities

_(无现有 spec 需要修改)_

## Impact

- **新增文件**: `runtime/types.ts`, `runtime/cf.ts`, `runtime/node.ts`, `index.node.ts`, `apps/worker/Dockerfile`
- **修改文件**: 14 个引用 `D1Database` / `KVNamespace` 的文件（类型签名变更）、`handler.ts`（7 处 waitUntil）、`index.ts`（WebSocket）、`docker-compose.yml`
- **新增依赖**: `better-sqlite3`, `@hono/node-server`, `@hono/node-ws`, `node-cron`, `ws`
- **不影响**: CF Workers 部署路径完全不变，现有行为不受影响

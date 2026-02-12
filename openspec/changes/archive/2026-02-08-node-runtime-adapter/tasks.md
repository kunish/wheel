## 1. Runtime 抽象层

- [x] 1.1 创建 `src/runtime/types.ts`：定义 `IKVStore` 接口、`RunBackground` 类型、共享 `AppBindings` 类型
- [x] 1.2 创建 `src/runtime/cf.ts`：CF 运行时实现 — `CfKV`(透传 KVNamespace)、`cfRunBackground`(透传 waitUntil)
- [x] 1.3 创建 `src/runtime/node.ts`：Node.js 运行时实现 — `MemoryKV`(Map + TTL)、`nodeRunBackground`(fire-and-forget)、`createNodeDb`(better-sqlite3 + Drizzle)

## 2. 类型重构

- [x] 2.1 修改 `db/index.ts`：泛化 `Database` 类型，支持 D1 和 better-sqlite3 两种驱动
- [ ] 2.2 替换 14 个文件中的本地 Bindings 类型定义为 `import { AppBindings } from '../runtime/types'`（index.ts, handler.ts, channel.ts, group.ts, apikey.ts, log.ts, stats.ts, setting.ts, model.ts, user.ts, jwt.ts, apikey middleware, sync.ts, balancer.ts）

## 3. 平台 API 替换

- [ ] 3.1 替换 `handler.ts` 中 7 处 `c.executionCtx.waitUntil()` 为 `runBackground()`，runBackground 通过 Hono context variable 注入
- [ ] 3.2 替换 `model.ts` 中 2 处 `c.executionCtx.waitUntil()` 为 `runBackground()`
- [ ] 3.3 替换 `handler.ts` 中 `loadChannels`/`loadGroups` 的 `kv: KVNamespace` 参数为 `kv: IKVStore`

## 4. Node.js 入口

- [ ] 4.1 创建 `src/index.node.ts`：Hono + `@hono/node-server` 入口，初始化 better-sqlite3 DB、MemoryKV，注入 Bindings
- [ ] 4.2 在 `index.node.ts` 中集成 `@hono/node-ws` WebSocket 支持
- [ ] 4.3 在 `index.node.ts` 中集成 `node-cron` 定时任务（每 6 小时同步）
- [ ] 4.4 实现 SQLite 自动迁移：首次启动时执行 drizzle migration

## 5. 构建配置

- [ ] 5.1 添加 Node.js 依赖：`better-sqlite3`、`@hono/node-server`、`@hono/node-ws`、`node-cron`、`ws` 及对应 `@types/*`
- [ ] 5.2 添加 `package.json` 构建脚本：`build:node`（使用 tsup/esbuild 打包 index.node.ts）和 `start:node`
- [ ] 5.3 确认 CF Workers 构建路径不受影响：`wrangler deploy` 仍使用 `index.ts`

## 6. Docker 全栈部署

- [ ] 6.1 创建 `apps/worker/Dockerfile`：多阶段构建，安装 better-sqlite3 原生模块，输出精简镜像
- [ ] 6.2 更新 `docker-compose.yml`：添加 worker 服务，Web 连接 Worker，SQLite volume 挂载到 `./data/`
- [ ] 6.3 创建 `.env.example`：列出所有 Docker 部署环境变量及默认值
- [ ] 6.4 更新 `README.md` Docker 部署章节：反映 Web + Worker 全栈 Docker 部署

## 7. 验证

- [ ] 7.1 验证 Node.js 模式启动成功，API 路由正常响应
- [ ] 7.2 验证 Docker Compose 全栈部署（`docker compose up` Web + Worker 均启动）
- [ ] 7.3 验证 CF Workers 部署路径不受影响（`wrangler deploy` 正常）

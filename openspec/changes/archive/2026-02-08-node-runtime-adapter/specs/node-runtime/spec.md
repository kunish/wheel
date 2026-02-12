## ADDED Requirements

### Requirement: Node.js HTTP 入口

系统 SHALL 提供 `index.node.ts` 作为 Node.js 入口，使用 `@hono/node-server` 启动 HTTP 服务器，复用现有的 Hono app 路由。

#### Scenario: Node.js 启动 Worker

- **WHEN** 执行 `node dist/index.node.js`
- **THEN** SHALL 在配置的端口（默认 8787）启动 HTTP 服务器，所有 API 路由与 CF Workers 版本行为一致

### Requirement: Node.js SQLite 数据库

Node.js 运行时 SHALL 使用 `better-sqlite3` 驱动连接本地 SQLite 文件，通过 Drizzle ORM 提供与 D1 一致的查询接口。

#### Scenario: 数据库文件初始化

- **WHEN** Node.js 运行时首次启动且数据库文件不存在
- **THEN** SHALL 自动创建 SQLite 数据库文件并执行 schema 迁移

#### Scenario: 数据持久化

- **WHEN** Node.js 运行时写入数据后重启
- **THEN** 之前写入的数据 SHALL 通过 SQLite 文件持久保留

### Requirement: Node.js 内存 KV 缓存

Node.js 运行时 SHALL 实现 `MemoryKV`，使用内存 `Map` + TTL 过期机制，实现 `IKVStore` 接口。

#### Scenario: 缓存 TTL 过期

- **WHEN** 调用 `put(key, value, { expirationTtl: 300 })` 后等待超过 300 秒
- **THEN** 后续 `get(key)` SHALL 返回 `null`

#### Scenario: 进程重启缓存清空

- **WHEN** Node.js 进程重启
- **THEN** 内存 KV 缓存 SHALL 为空，后续请求会重新从数据库加载并缓存

### Requirement: Node.js WebSocket 支持

Node.js 运行时 SHALL 通过 `@hono/node-ws` 提供 WebSocket 升级支持，复用现有的 `ws/hub.ts` 广播逻辑。

#### Scenario: WebSocket 连接和广播

- **WHEN** 客户端连接 `/api/v1/ws` WebSocket 端点
- **THEN** SHALL 成功升级为 WebSocket 连接，并能接收 `stats-updated` 广播消息

### Requirement: Node.js Cron 定时任务

Node.js 运行时 SHALL 使用 `node-cron` 注册定时任务，替代 CF Workers 的 `scheduled()` handler。

#### Scenario: 定时同步执行

- **WHEN** cron 时间到达（默认每 6 小时）
- **THEN** SHALL 执行价格同步和模型同步，行为与 CF Workers 的 `scheduled()` 一致

## ADDED Requirements

### Requirement: IKVStore 接口

系统 SHALL 定义一个 `IKVStore` 接口，包含 `get<T>(key, "json"): Promise<T | null>`、`put(key, value, opts?): Promise<void>` 和 `delete(key): Promise<void>` 三个方法，作为 KV 缓存的统一抽象。

#### Scenario: CF 和 Node 实现同一接口

- **WHEN** 应用代码调用 `CACHE.get("channels", "json")`
- **THEN** CF 运行时 SHALL 委托给 `KVNamespace.get()`，Node.js 运行时 SHALL 从内存 Map 读取

### Requirement: runBackground 抽象

系统 SHALL 提供 `runBackground(promise)` 函数替代 `c.executionCtx.waitUntil()`，用于在响应发送后执行后台任务。

#### Scenario: CF 运行时调用 runBackground

- **WHEN** 在 CF Workers 环境调用 `runBackground(promise)`
- **THEN** SHALL 委托给 `c.executionCtx.waitUntil(promise)`

#### Scenario: Node.js 运行时调用 runBackground

- **WHEN** 在 Node.js 环境调用 `runBackground(promise)`
- **THEN** SHALL 以 fire-and-forget 方式执行 promise，错误被静默捕获

### Requirement: 共享 Bindings 类型

系统 SHALL 在 `runtime/types.ts` 中定义共享的 `AppBindings` 类型，所有路由和中间件文件 MUST 从该文件 import Bindings 类型，不再各自定义。

#### Scenario: 路由文件使用共享类型

- **WHEN** 路由文件需要 Bindings 类型
- **THEN** SHALL 从 `runtime/types.ts` import `AppBindings`，不再本地定义 `type Env = { Bindings: { DB: D1Database; CACHE: KVNamespace; ... } }`

### Requirement: Database 类型泛化

`createDb()` 返回的 Database 类型 SHALL 兼容 D1 和 better-sqlite3 两种驱动，DAL 代码无需感知底层驱动差异。

#### Scenario: DAL 代码对两种驱动透明

- **WHEN** DAL 函数接收 `db` 参数执行查询
- **THEN** 无论底层是 D1 还是 better-sqlite3，查询行为和返回结果 SHALL 一致

## Why

代码审查发现 27 个跨前后端的质量问题，涵盖 God Function/Component、重复代码、全局可变状态、安全漏洞、风格不一致等。这些问题正在降低可维护性、可测试性和可扩展性，需要系统性地治理。

## What Changes

- 拆分 `logs.tsx`（111KB God Component）为独立子组件和 hooks
- 重构 `handleRelay`（490 行 God Function）为职责清晰的方法链
- 消除 `handler/channel.go` 与 `relay/sync.go` 之间的模型获取函数完全重复
- 将全局可变状态（breakers、sessions、rrCounters）改为结构体 + 依赖注入
- 为 WebSocket 端点添加 JWT 认证，收紧 CORS 配置
- 为 `http.DefaultClient` 添加超时配置
- 拆分 `activity-section.tsx`（1300 行）为独立组件和 hooks
- 统一前端 API 客户端（废弃手写 `api.ts`，迁移到 OpenAPI 生成的 `api-client.ts`）
- 合并 `asyncStreamLog` / `asyncNonStreamLog` 重复代码
- 将业务逻辑从 handler 层下沉到 service 层（model sync、pricing sync、import）
- 抽取可复用组件（ModelPickerBase、PeriodNavBar、RankedList、computePeriodTotals）
- 统一后端代码风格（`any` 替代 `interface{}`、请求结构体定义位置、Handler 结构体组合）
- 统一前端代码风格（导出方式、Props 类型定义、`import type` 使用）
- 添加缓存主动失效机制，熔断器配置缓存化
- 为 Session/CircuitBreaker map 添加过期清理
- 统一 Update handler 实现风格为 struct + 指针字段模式

## Capabilities

### New Capabilities

- `relay-handler-refactor`: 重构 handleRelay God Function，拆分为方法链，合并 asyncLog 重复，全局状态改为 DI
- `frontend-component-decomposition`: 拆分 logs.tsx 和 activity-section.tsx God Component，抽取可复用组件和 hooks
- `model-sync-consolidation`: 消除 handler/channel.go 与 relay/sync.go 之间的模型获取函数重复，统一到 relay 包
- `websocket-auth`: WebSocket 端点 JWT 认证 + CORS origin 校验
- `api-client-unification`: 废弃手写 api.ts，统一使用 OpenAPI 生成的 api-client.ts
- `backend-code-consistency`: 统一 Go 代码风格（any、请求结构体、Handler 组合、Update handler 模式）
- `frontend-code-consistency`: 统一前端代码风格（导出方式、Props 类型、import type）
- `handler-service-separation`: 将业务逻辑从 handler 层下沉到 service 层（model sync、pricing、import）
- `runtime-safety-improvements`: http.Client 超时、缓存主动失效、熔断器配置缓存、Session/CB 过期清理

### Modified Capabilities

- `simplify-proxy`: relay 层重构会改变代理请求的内部执行流程
- `stats-caching`: 缓存失效机制变更影响统计数据的实时性

## Impact

- **后端**: `internal/handler/`、`internal/relay/`、`internal/middleware/`、`internal/cache/`、`internal/ws/`、`internal/types/`、`cmd/worker/main.go` 大范围重构
- **前端**: `pages/logs.tsx`、`pages/dashboard/activity-section.tsx`、`lib/api.ts`、`lib/api-client.ts`、`components/` 多文件拆分和重组
- **API**: WebSocket 端点新增认证要求（**BREAKING** — 前端需同步更新 WS 连接逻辑）
- **测试**: 全局状态改为 DI 后，需要更新或新增单元测试
- **依赖**: 无新增外部依赖

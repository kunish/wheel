## 1. 后端基础设施重构（全局状态 + 类型统一）

- [x] 1.1 将 `relay/circuit.go` 中的 `breakers` map 和 `circuitObserver` 封装为 `CircuitBreakerManager` 结构体，提供 `NewCircuitBreakerManager()` 构造函数
- [x] 1.2 将 `relay/session.go` 中的 `sessions` map 封装为 `SessionManager` 结构体，提供 `NewSessionManager()` 构造函数
- [x] 1.3 将 `relay/balancer.go` 中的 `rrCounters` sync.Map 封装为 `BalancerState` 结构体，提供 `NewBalancerState()` 构造函数
- [x] 1.4 为 `SessionManager` 和 `CircuitBreakerManager` 添加后台过期清理 goroutine（间隔 5 分钟），支持优雅关闭
- [x] 1.5 创建带超时配置的 `*http.Client`（非流式 120s，流式连接 30s），替换 `relay/proxy.go` 中的 `http.DefaultClient`
- [x] 1.6 全局替换 `interface{}` 为 `any`（`user.go:24`、`types/api.go:287` 等）
- [x] 1.7 将 `handler/relay.go` 和 `db/logwriter.go` 中重复的 `BroadcastFunc` 和 `StreamTracker` 定义统一到 `types` 包

## 2. Handler 结构体与请求类型统一

- [x] 2.1 让 `RelayHandler` 嵌入 `Handler`，消除 DB、LogDB、Cache 字段重复
- [x] 2.2 更新 `main.go` 初始化流程，构造 CircuitBreakerManager、SessionManager、BalancerState、http.Client 并注入到 RelayHandler
- [x] 2.3 将 `handler/user.go` 中的 `loginRequest` 迁移到 `types/api.go` 导出为 `LoginRequest`
- [x] 2.4 将 `handler/model.go` 中的匿名结构体替换为 `types.LLMCreateRequest`、`types.LLMUpdateRequest`、`types.LLMDeleteRequest`
- [x] 2.5 将 `UpdateChannel`（channel.go:93-178）和 `UpdateGroup`（group.go:76-119）从 `map[string]interface{}` 重构为 struct + 指针字段模式
- [x] 2.6 清理 `types/api.go` 中未使用的 Response 类型，或在 handler 中实际使用它们

## 3. handleRelay God Function 拆分

- [x] 3.1 提取 `parseRelayRequest(c *gin.Context)` 方法（请求解析、模型名提取、流式判断）
- [x] 3.2 提取 `selectChannels(modelName string)` 方法（加载 channels/groups、匹配模型、session 粘性）
- [x] 3.3 定义 `RelayStrategy` 接口（Execute + HandleSuccess），实现 `NonStreamStrategy` 和 `StreamStrategy`
- [x] 3.4 提取 `executeWithRetry()` 方法（重试循环、熔断器检查、attempt 记录），通过 RelayStrategy 统一流式/非流式
- [x] 3.5 合并 `asyncStreamLog` 和 `asyncNonStreamLog` 为统一的 `asyncRecordLog` 方法
- [x] 3.6 将原始 `handleRelay` 重构为编排方法，依次调用 parseRelayRequest → selectChannels → executeWithRetry → asyncRecordLog

## 4. 模型同步去重与 Service 层

- [x] 4.1 将 `relay/sync.go` 中的 `fetchOpenAIModels`、`fetchAnthropicModels`、`fetchGeminiModels` 导出为公开函数
- [x] 4.2 提取 `checkAPIResponse(resp, body, apiName) error` 辅助函数，替换三个 fetch 函数中的重复错误处理
- [x] 4.3 删除 `handler/channel.go` 中的重复 fetch 函数（fetchOpenAIModels 等），改为调用 sync 包
- [x] 4.4 合并 `handler.syncAllModels` 和 `relay.SyncAllModels` 为统一的 `sync.SyncAllModels`，通过选项支持 isFallback 和 autoGroup
- [x] 4.5 创建 `internal/service/modelsync.go`，将 `fetchAndFlattenMetadata` 和 `SyncPricesFromModelsDev` 从 handler 迁移过来
- [x] 4.6 创建 `internal/service/import.go`，将 `ImportData` 的去重导入逻辑从 handler/setting.go 迁移过来
- [x] 4.7 在 `relay/pricing.go` 中提取 `resolveModelName` 内部函数，让 `CalculateCost` 内部调用 `LookupModelPrice`
- [x] 4.8 将 API Key 认证的 key 提取逻辑统一到 `middleware` 包，`RelayHandler.apiKeyAuthMiddleware()` 复用

## 5. 缓存与运行时安全

- [x] 5.1 在 channel/group 的 Create、Update、Delete handler 中调用 `cache.Delete("channels")` / `cache.Delete("groups")` 主动失效缓存
- [x] 5.2 将 `getThreshold()` 和 `GetCooldownConfig()` 的数据库查询结果缓存到 MemoryKV（TTL 60s）
- [x] 5.3 将 `relay/adapter.go` 中的 `copyBody` 改为 JSON marshal/unmarshal 深拷贝

## 6. WebSocket 认证

- [x] 6.1 在 `ws/hub.go` 的 `HandleWS` 中添加 JWT token 验证（从 `?token=xxx` query parameter 读取）
- [x] 6.2 更新前端 `use-stats-ws.ts`，在 WebSocket URL 中附带 JWT token

## 7. 前端 logs.tsx 拆分

- [x] 7.1 创建 `pages/logs/` 目录结构
- [x] 7.2 提取 `hooks/use-log-query.ts`（TanStack Query 封装、分页、筛选参数）
- [x] 7.3 提取 `pages/logs/log-filter-bar.tsx`（筛选条件面板）
- [x] 7.4 提取 `pages/logs/log-table.tsx`（表格 + 分页）
- [x] 7.5 提取 `pages/logs/log-detail-panel.tsx`（详情侧边栏）
- [x] 7.6 提取 `pages/logs/log-replay.ts`（重放 hook）
- [x] 7.7 重写 `pages/logs/index.tsx` 为容器组件，编排子组件
- [x] 7.8 路由配置自动解析 logs/index.tsx，无需修改

## 8. 前端 activity-section 拆分

- [x] 8.1 提取 `computePeriodTotals(days)` 工具函数，替换三个视图的重复统计计算
- [x] 8.2 提取 `hooks/use-date-navigation.ts`（日期计算、视图切换、prev/next/reset）
- [x] 8.3 提取 `dashboard/hub-model-list.tsx` 和 `dashboard/hub-channel-list.tsx`
- [x] 8.4 提取 `dashboard/inline-stats.tsx`，Month 视图改为使用 InlineStats 组件
- [x] 8.5 提取 `dashboard/data-panel-popover.tsx`
- [x] 8.6 提取 `PeriodNavBar` 可复用组件，替换三个视图的重复导航栏
- [x] 8.7 精简 `activity-section.tsx` 为 ≤400 行的编排组件

## 9. 前端组件复用与 API 客户端统一

- [x] 9.1 提取 `ModelPickerBase` 组件，`model-picker-dialog.tsx` 和 `channel-model-picker-dialog.tsx` 复用
- [x] 9.2 将 `api.ts` 中的统计类型定义提取到 `lib/types/stats.ts`
- [x] 9.3 逐页面将 `api.ts` 函数迁移到 `api-client.ts`（dashboard → model → logs → settings）
- [x] 9.4 迁移 `exportData`、`importData`、`replayLog` 到 api-client，消除绕过 apiFetch 的直接 fetch 调用
- [x] 9.5 迁移完成后删除 `lib/api.ts`

## 10. 前端代码风格统一

- [x] 10.1 将 `components/chart-section.tsx` 的 `export default` 改为命名导出 `export function`
- [x] 10.2 为 2+ 属性的组件统一添加命名 `interface XxxProps`（model-source-badge、chart-section 等）
- [x] 10.3 全局检查并补充 `import type` 语法（login.tsx 等缺失的文件）

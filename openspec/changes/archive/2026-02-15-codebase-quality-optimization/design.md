## Context

Wheel 是一个 LLM API 网关（Go 后端 + React 前端），经过全面代码审查发现 27 个质量问题。项目当前可运行且功能完整，但存在 God Function/Component、重复代码、全局可变状态、安全漏洞、风格不一致等问题，影响可维护性和可扩展性。

当前架构：

- 后端：Go 1.24 + Gin + SQLite/Bun ORM，handler → relay → dal 分层
- 前端：React 19 + TypeScript + Vite + shadcn/ui + TanStack Query + Zustand
- Monorepo：pnpm workspace

## Goals / Non-Goals

**Goals:**

- 消除 God Function/Component，每个函数/组件职责单一
- 消除跨文件重复代码，建立可复用抽象
- 将全局可变状态改为依赖注入，提高可测试性
- 修复 WebSocket 认证缺失的安全漏洞
- 统一前后端代码风格
- 将业务逻辑从 handler 层下沉到 service 层
- 改善运行时安全性（HTTP 超时、缓存失效、内存清理）

**Non-Goals:**

- 不改变任何用户可见的功能行为（纯内部重构）
- 不引入新的外部依赖（JWT 库迁移等留待后续）
- 不重写整个架构（渐进式改进）
- 不涉及测试框架搭建（仅确保重构后代码可测试）
- 不涉及日志库迁移（slog 迁移留待后续）

## Decisions

### D1: handleRelay 拆分策略 — 方法链 + 策略模式

将 490 行的 `handleRelay` 拆分为 `RelayHandler` 上的多个私有方法：

- `parseRequest()` → `selectChannels()` → `executeWithRetry()` → `recordResult()`
- 流式/非流式差异通过 `RelayStrategy` 接口处理

**替代方案**: 拆分为独立的中间件链。放弃原因：relay 的步骤间有大量共享状态（attempts、selectedChannel 等），中间件模式会导致 context 传递过于复杂。

### D2: 全局状态改造 — 结构体封装 + 构造函数注入

创建 `CircuitBreakerManager`、`SessionManager`、`BalancerState` 结构体，在 `main.go` 中构造并注入到 `RelayHandler`。

**替代方案**: 使用 sync.Pool 或 context.Value 传递。放弃原因：结构体 + 构造函数是 Go 中最惯用的 DI 方式，简单直接。

### D3: 模型获取函数去重 — 统一到 relay/sync 包

`handler/channel.go` 中的 `fetchOpenAIModels` 等函数移除，统一使用 `relay/sync.go` 中的版本。handler 层只调用 sync 包的公开函数。

**替代方案**: 抽取到独立的 `provider` 包。放弃原因：当前 relay/sync.go 已经是合理的位置，无需新增包。

### D4: logs.tsx 拆分 — 按功能域拆分为子组件 + hooks

拆分为：

- `pages/logs/index.tsx` — 主页面容器
- `pages/logs/log-table.tsx` — 表格 + 分页
- `pages/logs/log-filters.tsx` — 筛选条件
- `pages/logs/log-detail-panel.tsx` — 详情侧边栏
- `pages/logs/log-replay-dialog.tsx` — 重放对话框
- `hooks/use-log-query.ts` — TanStack Query 封装

### D5: API 客户端统一 — 渐进迁移到 OpenAPI 生成客户端

保留 `api-client.ts`（OpenAPI 生成），逐步将 `api.ts` 中的函数迁移过去。迁移完成后删除 `api.ts`。类型定义提取到独立的 `types/stats.ts`。

**替代方案**: 一次性替换。放弃原因：渐进迁移风险更低，可以逐页面验证。

### D6: handler-service 分离 — 新增 service 层

在 `internal/` 下新增 `service/` 目录：

- `service/modelsync.go` — 模型同步和元数据获取
- `service/pricing.go` — 价格同步
- `service/import.go` — 数据导入

handler 层变为薄包装，只做参数解析和响应格式化。

### D7: activity-section 拆分 — 子组件独立文件 + hooks 提取

- 将 `HubModelList`、`HubChannelList`、`DataPanelPopover` 等提取为独立文件
- 抽取 `useDateNavigation` hook 处理日期计算
- 抽取 `computePeriodTotals` 工具函数
- 抽取 `PeriodNavBar` 和 `RankedList` 可复用组件

### D8: WebSocket 认证 — query parameter JWT

WebSocket 升级前通过 `?token=xxx` query parameter 传递 JWT，在 `HandleWS` 中验证。

**替代方案**: 使用 cookie 认证。放弃原因：当前系统使用 JWT，保持一致性。

## Risks / Trade-offs

- [大范围重构可能引入回归] → 按模块分批实施，每批完成后手动验证核心流程（relay 代理、日志查看、统计面板）
- [WebSocket 认证是 BREAKING CHANGE] → 前后端同步更新，前端 `use-stats-ws.ts` 需要在连接时附带 token
- [API 客户端迁移期间两套并存] → 设置明确的迁移顺序（按页面），每迁移一个页面删除对应的旧函数
- [handleRelay 拆分后性能影响] → 方法调用开销可忽略，但需确保 attempt 状态传递不引入额外分配
- [全局状态改造影响启动流程] → main.go 中的初始化顺序需要仔细编排，确保依赖关系正确

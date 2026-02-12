## Context

Wheel 是一个 LLM API 聚合和负载均衡服务，当前使用 Go 后端 + Next.js 前端的单体架构，通过 Docker 部署。核心功能包括：

- **Relay 代理**：将 `/v1/chat/completions`、`/v1/messages` 等 LLM API 请求代理到多个上游 Provider（OpenAI/Anthropic/Gemini 等）
- **负载均衡**：支持 RoundRobin、Random、Failover、Weighted 四种策略
- **管理面板**：Channel/Group/APIKey 管理、日志查看、统计分析
- **数据存储**：SQLite + GORM，包含 Channel、Group、APIKey、RelayLog、User 等核心实体

当前项目位于 `https://github.com/bestruirui/wheel`，我们将在本仓库（`wheel`）中创建独立的前端 + Edge Worker 项目，复用 Wheel 的 API 契约和设计理念。

**实施约束**：所有依赖使用最新版本；项目初始化尽可能使用官方 CLI 工具（`create-next-app`、`wrangler init`、`pnpm create hono` 等），避免手动脚手架。

## Goals / Non-Goals

**Goals:**

- 将 Wheel 的核心 relay 代理逻辑用 TypeScript 重写，部署到 Cloudflare Worker
- 将前端 Next.js 应用独立化，可部署到 Vercel 或 Cloudflare Pages
- 使用 shadcn/ui 构建增强的管理界面，包括部署向导
- 支持 Cloudflare D1 作为 Edge 数据存储
- 保持与 Wheel 原版 API 的兼容性（`/v1/*` 代理端点 + `/api/v1/*` 管理端点）

**Non-Goals:**

- 不替代 Wheel 原有的 Go 后端（两者并行存在，用户可选择部署方式）
- 不实现 Go 后端的全部功能（如自动更新、系统代理等服务器特有功能）
- 不支持 MySQL/PostgreSQL（Edge 环境使用 D1，简化复杂度）
- 不实现多用户系统（保持 Wheel 的单用户设计）

## Decisions

### 1. 整体架构：Monorepo + 双部署目标

**选择**：使用 pnpm workspace monorepo，包含三个包：

- `packages/core` — 共享类型、工具、API 契约定义
- `apps/worker` — Cloudflare Worker（API 代理 + 管理 API）
- `apps/web` — Next.js 前端（管理面板 + 部署向导）

**理由**：Monorepo 确保类型安全和代码复用；Worker 和 Web 独立部署但共享 API 类型定义。

**替代方案**：

- 单一 Next.js 应用（API Routes + Pages）→ 放弃，因为 Vercel Edge Function 有执行时间和大小限制，不适合长时间的 SSE 流式代理
- 分离仓库 → 放弃，类型同步成本太高

### 2. Edge 数据存储：Cloudflare D1 为主

**选择**：使用 Cloudflare D1（Edge SQLite）作为主数据存储，Drizzle ORM 作为查询层。

**理由**：

- Wheel 原版使用 SQLite，D1 天然兼容，数据模型可几乎直接迁移
- Drizzle ORM 对 D1 有一等支持，类型安全且轻量
- D1 支持事务、JOIN、索引等完整 SQL 能力

**替代方案**：

- Cloudflare KV → 放弃，KV 是键值存储，无法支持 Wheel 的关系数据模型（Channel-Group-GroupItem 关联）
- Turso (LibSQL) → 可行但增加外部依赖，D1 作为 CF 原生服务更简单
- Vercel Postgres → 仅限 Vercel 平台，不符合 CF Worker 部署目标

### 3. Relay 代理：基于 Hono 的轻量 Worker

**选择**：使用 Hono 作为 Worker 的 HTTP 框架，重写 Wheel 的 relay 逻辑。

**理由**：

- Hono 专为 Edge Runtime 设计，零依赖、极轻量（< 20KB）
- 内置中间件支持（CORS、JWT、Bearer Auth）
- 原生支持 Cloudflare Workers 的 Bindings（D1、KV、Durable Objects）
- 与 Wheel 的 Gin 框架模式相似（路由分组、中间件链），降低移植成本

**替代方案**：

- 原生 Workers API → 放弃，手动路由和中间件管理过于繁琐
- itty-router → 功能太简单，缺少中间件生态

### 4. SSE 流式代理：TransformStream 管道

**选择**：使用 Web Streams API（TransformStream）实现 SSE 流式响应代理。

**理由**：

- Cloudflare Workers 原生支持 Web Streams，可实现真正的流式转发
- 参考 Wheel 的 `handleStreamResponse()`，在 TransformStream 中实现：
  - 协议转换（Anthropic → OpenAI 格式）
  - First Token Timeout 检测
  - Token 计数和日志采集
- 避免缓冲整个响应，内存占用低

### 5. 前端：Next.js App Router + shadcn/ui

**选择**：基于 Next.js 15+ App Router，使用 shadcn/ui 构建 UI，Zustand 管理状态。

**理由**：

- 与 Wheel 原版前端技术栈一致（Next.js + shadcn/ui + Zustand + TanStack Query）
- 降低从 Wheel 移植组件的成本
- App Router 的 Server Components 可优化首屏加载
- shadcn/ui 提供完整的组件库，支持暗色模式和响应式

### 6. 部署向导：步骤式表单 + CLI 脚本生成

**选择**：在前端实现可视化部署向导，引导用户配置并生成可执行的部署命令。

**理由**：

- "一键部署"的核心体验是降低认知负担
- 向导收集必要配置（CF API Token、Account ID、D1 数据库名等）后，生成：
  - `wrangler.toml` 配置文件内容
  - 一键执行的 shell 命令序列
  - 或一个 "Deploy to Cloudflare" 按钮链接
- 参考 Vercel 的 "Deploy" 按钮模式

### 7. 认证：兼容 Wheel 的 JWT + API Key 双模式

**选择**：保持与 Wheel 相同的认证模型。

**理由**：

- 管理面板使用 JWT（`username + password` 作为 secret）
- API 代理端点使用 `sk-wheel-*` 格式的 API Key
- 在 Worker 中通过 D1 查询验证，与 Wheel 原版行为一致

## Risks / Trade-offs

**[D1 性能限制]** → D1 单次查询有延迟（~5ms），每个 relay 请求需要查询 APIKey + Group + Channel → 通过 Workers KV 缓存热数据（Channel/Group 配置变化频率低），减少 D1 查询次数

**[Worker 执行时间]** → CF Worker 免费版限制 10ms CPU 时间（付费版 50ms）→ relay 代理的 CPU 消耗极低（主要是 I/O 等待），但复杂的 token 计算可能需要优化或异步化

**[SSE 连接时长]** → CF Worker 对 subrequest 有 30s 超时 → 使用 CF 的 `waitUntil()` 和 streaming 模式可突破此限制；极长对话可能需要 Durable Objects

**[数据迁移]** → 用户从 Docker 版迁移到 Edge 版需要导入数据 → 提供数据导入/导出工具（JSON 格式）

**[功能差异]** → Edge 版无法支持系统代理（`channel.Proxy`）、自定义代理（`ChannelProxy`）等服务器特有功能 → 在 UI 中明确标注，引导用户使用 CF Worker 的原生路由能力

## Migration Plan

### Phase 1: 基础框架搭建

1. 初始化 monorepo（pnpm workspace）
2. 搭建 `packages/core`（类型定义、API 契约）
3. 搭建 `apps/worker` 基础 Hono 应用 + D1 Schema
4. 搭建 `apps/web` 基础 Next.js 应用 + shadcn/ui

### Phase 2: 核心 API 实现

1. 实现 Worker 管理 API（Channel/Group/APIKey CRUD）
2. 实现 Relay 代理逻辑（请求路由、负载均衡、SSE 流式）
3. 实现认证中间件（JWT + API Key）

### Phase 3: 前端 UI

1. 移植并增强 Wheel 管理面板页面
2. 实现部署向导 UI
3. 实现增强的仪表盘

### Phase 4: 部署支持

1. 配置 wrangler.toml 模板
2. 配置 vercel.json
3. 部署脚本和 CI/CD
4. 文档和使用指南

### Rollback Strategy

- 用户随时可切换回 Wheel 原版 Docker 部署
- 数据导出工具支持从 D1 导出为 JSON，可导入到 SQLite

## Open Questions

1. **是否需要支持 Vercel Edge Functions 作为 Worker 的替代？** Vercel Edge Function 有更严格的限制（128KB 代码大小、30s 超时），可能不适合完整的 relay 逻辑
2. **Durable Objects vs D1 用于负载均衡状态？** RoundRobin 计数器在 Edge 环境中需要全局一致性，D1 可以但有延迟，Durable Objects 更适合但增加复杂度
3. **是否需要 WebSocket 支持？** 部分 LLM API 提供 WebSocket 接口，Worker 对 WebSocket 的支持有限

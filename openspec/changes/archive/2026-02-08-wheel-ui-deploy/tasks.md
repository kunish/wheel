## 1. Monorepo 基础搭建

- [x] 1.1 初始化 pnpm workspace monorepo，创建 `pnpm-workspace.yaml`（包含 `packages/*` 和 `apps/*`）
- [x] 1.2 创建 `packages/core` 包，配置 `package.json` 和 `tsconfig.json`，导出共享类型
- [x] 1.3 使用 `pnpm create hono apps/worker` 初始化 Cloudflare Worker 项目（选择 cloudflare-workers 模板）
- [x] 1.4 使用 `pnpm create next-app apps/web` 初始化 Next.js 项目（App Router、TypeScript、Tailwind CSS、ESLint）
- [x] 1.5 在 `apps/web` 中使用 `npx shadcn@latest init` 初始化 shadcn/ui
- [x] 1.6 配置根目录 `tsconfig.json`、ESLint、Prettier 等共享开发配置
- [x] 1.7 在 `apps/worker` 中安装 Drizzle ORM（`drizzle-orm`、`drizzle-kit`）并配置 D1 连接

## 2. 共享类型和 API 契约（packages/core）

- [x] 2.1 定义核心数据模型类型：`Channel`、`ChannelKey`、`BaseUrl`、`Group`、`GroupItem`、`APIKey`、`RelayLog`、`User`、`Setting`
- [x] 2.2 定义 API 请求/响应类型：Channel CRUD、Group CRUD、APIKey CRUD、Log 查询、Stats 查询、User 认证
- [x] 2.3 定义枚举类型：`OutboundType`（OpenAI/Anthropic/Gemini 等）、`GroupMode`（RoundRobin/Random/Failover/Weighted）、`AutoGroupType`
- [x] 2.4 定义 LLM 请求/响应的内部格式类型（`InternalLLMRequest`、`InternalLLMResponse`）

## 3. D1 数据库 Schema 和迁移（apps/worker）

- [x] 3.1 使用 Drizzle ORM 定义 D1 Schema：`users`、`channels`、`channel_keys`、`groups`、`group_items`、`api_keys`、`relay_logs`、`settings` 表
- [x] 3.2 配置 `drizzle.config.ts`，设置 D1 数据库绑定
- [x] 3.3 生成初始 SQL 迁移文件（`drizzle-kit generate`），确保与 Wheel GORM 模型字段兼容
- [x] 3.4 编写数据访问层（DAL）：Channel/Group/APIKey/Log/Stats/User 的查询和写入函数

## 4. Worker 认证中间件

- [x] 4.1 实现 JWT 认证中间件：从环境变量获取 `JWT_SECRET`，验证 `Authorization: Bearer` token
- [x] 4.2 实现 JWT token 生成：`POST /api/v1/user/login` 登录接口
- [x] 4.3 实现 API Key 认证中间件：支持 `Authorization: Bearer sk-wheel-*` 和 `x-api-key` 两种格式
- [x] 4.4 实现 API Key 验证逻辑：检查启用状态、过期时间、成本限额、模型白名单

## 5. Worker 管理 API

- [x] 5.1 实现 Channel 管理 API：`/api/v1/channel/list`、`create`、`update`、`delete`、`enable`
- [x] 5.2 实现 Group 管理 API：`/api/v1/group/list`、`create`、`update`、`delete`、`model-list`
- [x] 5.3 实现 API Key 管理 API：`/api/v1/apikey/list`、`create`、`update`、`delete`、`stats`
- [x] 5.4 实现日志查询 API：`/api/v1/log/list`（分页+筛选）、`/api/v1/log/:id`
- [x] 5.5 实现统计 API：`/api/v1/stats/global`、`/api/v1/stats/channel`
- [x] 5.6 实现设置 API：`/api/v1/setting`（GET/POST）
- [x] 5.7 实现用户 API：`/api/v1/user/login`、`change-password`、`change-username`、`status`

## 6. Worker Relay 代理核心

- [x] 6.1 实现请求解析器：从 request body 提取 model 名称，判断请求类型（OpenAI/Anthropic/Embedding）
- [x] 6.2 实现 Group 匹配逻辑：根据 model 名称和 Group 的 MatchRegex 匹配目标 Group
- [x] 6.3 实现四种负载均衡器：RoundRobin、Random、Failover、Weighted
- [x] 6.4 实现 Channel Key 选择逻辑：过滤启用 Key，跳过 429 状态 Key（5分钟冷却），选择最低成本 Key
- [x] 6.5 实现请求转换器（Adapter 模式）：InternalRequest → OpenAI/Anthropic/Gemini 上游格式
- [x] 6.6 实现响应转换器：上游响应 → InternalResponse → 客户端格式
- [x] 6.7 实现非流式代理路径：发送请求、接收完整响应、转换格式、返回 JSON
- [x] 6.8 实现 SSE 流式代理路径：使用 TransformStream 逐块转发 SSE 事件，实现协议转换
- [x] 6.9 实现 First Token Timeout 检测：在流式代理中设置定时器，超时中断并重试
- [x] 6.10 实现重试逻辑：3 轮 × N Channel 的重试循环，记录每次尝试详情
- [x] 6.11 实现异步日志记录：使用 `ctx.executionCtx.waitUntil()` 异步写入 RelayLog 到 D1

## 7. Worker KV 缓存层

- [x] 7.1 实现 KV 缓存抽象：`get`/`set`/`delete` 方法，支持 TTL（默认 5 分钟）
- [x] 7.2 实现 Channel/Group 配置的 KV 缓存读写
- [x] 7.3 在管理 API 的写操作后自动清除对应 KV 缓存
- [x] 7.4 配置 `wrangler.toml` 中的 KV namespace 绑定

## 8. 前端基础框架（apps/web）

- [x] 8.1 配置 next-themes（暗色模式）、next-intl（可选 i18n）
- [x] 8.2 安装 shadcn/ui 常用组件：button、card、dialog、input、select、table、tabs、badge、switch、separator、toast
- [x] 8.3 配置 Zustand store：auth store（JWT token 管理、登录/登出）
- [x] 8.4 配置 TanStack React Query：API client 基础封装，自动 JWT token 注入，401 自动跳转登录
- [x] 8.5 实现 API 客户端：基于 fetch，支持连接到 CF Worker 后端（通过环境变量 `NEXT_PUBLIC_API_BASE_URL`）
- [x] 8.6 实现应用 Layout：侧边栏导航 + 顶部工具栏 + 主内容区，响应式适配

## 9. 前端登录和认证

- [x] 9.1 实现登录页：用户名/密码表单，调用 `/api/v1/user/login`，存储 JWT token
- [x] 9.2 实现认证路由守卫：未登录自动跳转登录页
- [x] 9.3 实现用户菜单：修改密码、修改用户名、登出

## 10. 前端仪表盘页

- [x] 10.1 实现全局统计卡片组件：请求数、token 消耗、成本、活跃 Channel/Group 数
- [x] 10.2 实现请求趋势图表组件（Recharts）：折线图+柱状图，支持天/周/月切换
- [x] 10.3 实现 Channel 统计图表组件：各 Channel 的请求分布、成本占比

## 11. 前端 Channel 管理页

- [x] 11.1 实现 Channel 列表页：卡片/表格视图，显示名称、类型、状态、模型数、Key 数
- [x] 11.2 实现 Channel 创建/编辑 Dialog：表单包含名称、类型选择、BaseURL 列表编辑、Key 列表编辑、模型选择
- [x] 11.3 实现 Channel 启用/禁用开关（乐观更新）
- [x] 11.4 实现 Channel 删除确认 Dialog

## 12. 前端 Group 管理页

- [x] 12.1 实现 Group 列表页：显示名称、模式、Channel 数量、模型匹配规则
- [x] 12.2 实现 Group 创建/编辑 Dialog：负载均衡模式选择、模型匹配正则、FirstTokenTimeout 配置
- [x] 12.3 实现 GroupItem 编辑：Channel 选择、权重/优先级配置，根据模式动态展示不同 UI
- [x] 12.4 实现模型列表查看（`/api/v1/group/model-list`）

## 13. 前端 API Key 管理页

- [x] 13.1 实现 API Key 列表页：名称、Key（脱敏）、状态、过期时间、成本限额
- [x] 13.2 实现 API Key 创建 Dialog：名称、过期时间、成本限额、模型白名单配置
- [x] 13.3 实现创建后的 Key 展示（一次性可见，带复制按钮）
- [x] 13.4 实现 API Key 编辑和删除

## 14. 前端日志页

- [x] 14.1 实现日志列表页（shadcn DataTable）：分页、排序
- [x] 14.2 实现日志筛选栏：按模型、Channel、状态、时间范围筛选
- [x] 14.3 实现日志详情 Dialog：完整请求/响应内容、重试链路时间线

## 15. 前端设置页

- [x] 15.1 实现系统设置表单：基础配置项展示和修改

## 16. 部署向导 UI

- [x] 16.1 实现 Stepper 步骤条组件（复用 shadcn/ui 模式）
- [x] 16.2 实现部署目标选择步骤：Cloudflare Worker / Vercel 卡片选择
- [x] 16.3 实现 Cloudflare 凭证配置步骤：Account ID、API Token 输入、验证按钮
- [x] 16.4 实现部署配置步骤：Worker 名称、D1 数据库名、KV namespace 等配置
- [x] 16.5 实现配置预览步骤：wrangler.toml / vercel.json 代码预览（语法高亮），支持在线编辑
- [x] 16.6 实现部署命令生成步骤：命令序列展示、一键复制、Vercel Deploy Button URL

## 17. 部署配置生成逻辑

- [x] 17.1 实现 wrangler.toml 模板生成函数：根据用户配置填充 Worker name、D1 binding、KV binding、环境变量
- [x] 17.2 实现 vercel.json 模板生成函数：buildCommand、outputDirectory、rewrites、env
- [x] 17.3 实现 .env.example 模板生成函数：列出所有必需/可选环境变量及注释
- [x] 17.4 实现 D1 初始迁移 SQL 生成：基于 Drizzle Schema 导出 CREATE TABLE 语句
- [x] 17.5 实现配置文件 ZIP 打包下载（使用 JSZip 或类似库）

## 18. 部署和构建配置

- [x] 18.1 配置 `apps/worker/wrangler.toml`：D1 binding、KV binding、compatibility_date、入口文件
- [x] 18.2 配置 `apps/web/vercel.json`：构建命令、输出目录、环境变量
- [x] 18.3 配置 `apps/web/next.config.ts`：output standalone、API rewrites（指向 Worker URL）
- [x] 18.4 在根目录 `package.json` 添加脚本：`dev`（并行启动 worker + web）、`build`、`deploy:worker`、`deploy:web`
- [x] 18.5 编写 README：项目介绍、快速开始、部署指南（CF Worker + Vercel）

## Context

Wheel 是一个 LLM API Gateway，后端为 Go/Gin（`apps/worker/`），前端为 React/Vite（`apps/web/`）。当前没有任何 API 文档，前端 `lib/api.ts` 中的类型和端点路径与后端手动同步。前端部署依赖 Vercel rewrites 或 Docker 反向代理将 `/api/*` 转发到 worker，无法作为纯静态站点部署。

当前 API 请求层（`apiFetch`）使用相对路径（如 `/api/v1/user/login`），无 base URL 配置能力。WebSocket 连接通过 `VITE_API_BASE_URL` 环境变量或 `window.location` 构建。Router 使用 `BrowserRouter`，不兼容 GitHub Pages 的静态文件路由。

## Goals / Non-Goals

**Goals:**

- 为 worker 的全部管理 API 端点生成 OpenAPI 3.0 规范
- 在 worker 中嵌入 Scalar API Reference 页面，无需额外部署即可浏览 API 文档
- 基于 OpenAPI spec 为前端生成 TypeScript 类型，减少手动维护
- 前端支持用户在 UI 中配置 worker base URL，存储到 localStorage
- 前端可构建为纯静态 SPA 部署到 GitHub Pages
- 新增 CI workflow 自动部署到 GitHub Pages

**Non-Goals:**

- 不为 relay API（`/v1/*`）生成 OpenAPI（这些是兼容 OpenAI/Anthropic 的代理端点，有已有规范）
- 不修改后端 API 的行为或路由结构
- 不移除现有 Vercel/Docker 部署方式（保持向后兼容）
- 不做 API 版本升级（保持 v1）

## Decisions

### D1: OpenAPI 生成方式 — swaggo/swag 注解

**选择**: 使用 [swaggo/swag](https://github.com/swaggo/swag) 在 Go handler 上添加注释注解，通过 `swag init` 生成 `docs/swagger.json`。

**替代方案**:

- **手写 OpenAPI YAML**: 与代码同步困难，维护负担大
- **ogen/oapi-codegen（spec-first）**: 需要重构所有 handler 以适配生成的接口，改动过大

**理由**: swaggo/swag 与 Gin 集成成熟，可增量地为每个 handler 添加注解，不改变现有代码结构。生成的 JSON 文件可嵌入二进制或独立分发。

### D2: API 文档可视化 — Scalar

**选择**: 使用 [Scalar](https://github.com/scalar/scalar) 的 CDN 版本，在 worker 中注册 `GET /docs` 路由，返回嵌入 Scalar 的 HTML 页面，引用 `/docs/openapi.json`。

**替代方案**:

- **Swagger UI**: 较传统，UI 体验不如 Scalar 现代
- **Redoc**: 只读文档，无交互式 API 调试能力

**理由**: Scalar 支持交互式请求发送，UI 现代，且通过 CDN 引入无需打包到 Go 二进制，保持简洁。

### D3: 前端类型生成 — openapi-typescript

**选择**: 使用 [openapi-typescript](https://openapi-ts.dev/) 从 `openapi.json` 生成 TypeScript 类型定义文件，搭配 `openapi-fetch` 作为类型安全的请求层。

**替代方案**:

- **openapi-generator**: 生成完整 client 代码但体积大、产出代码风格不可控
- **orval**: 功能强但配置复杂

**理由**: `openapi-typescript` 只生成类型，与现有 `apiFetch` 封装兼容；`openapi-fetch` 提供类型安全的 `GET`/`POST` 方法，轻量且社区活跃。在 `package.json` 中添加 `generate:api` script，从 worker 的 `openapi.json` 生成到 `src/lib/api.gen.d.ts`。

### D4: 动态 API Base URL — Zustand store + 设置页面

**选择**: 在 Zustand auth store 中新增 `apiBaseUrl` 字段，持久化到 localStorage。`apiFetch` 在发起请求时读取该值作为 URL 前缀。在设置页面（或登录页面）添加 URL 配置入口。

**替代方案**:

- **仅环境变量**: 构建时确定，用户无法动态修改
- **URL query param**: 不持久化，每次访问需重新输入

**理由**: localStorage 持久化 + Zustand 统一管理，既支持 GitHub Pages 静态部署场景（用户首次访问时设置），也不影响现有 Docker/Vercel 部署（默认空值时使用相对路径）。

### D5: GitHub Pages 兼容 — HashRouter

**选择**: 当构建目标为 GitHub Pages 时（通过环境变量 `VITE_HASH_ROUTER=true` 控制），使用 `HashRouter` 替代 `BrowserRouter`。

**替代方案**:

- **404.html redirect hack**: 不够健壮，依赖 GitHub Pages 404 行为
- **始终 HashRouter**: 影响所有部署场景的 URL 美观度

**理由**: 通过环境变量条件选择 Router 类型，GitHub Pages 部署使用 hash 路由，其他部署保持 BrowserRouter，两全其美。

### D6: GitHub Pages CI — 独立 workflow

**选择**: 新增 `.github/workflows/pages.yml`，在 push to main 时构建 web 并通过 `actions/deploy-pages` 部署。与现有 `release.yml` 分离。

**理由**: GitHub Pages 部署不依赖 release 发布周期，每次 push 到 main 即可更新。独立 workflow 避免与 Docker 构建相互影响。

## Risks / Trade-offs

- **[swag 注解维护成本]** → 新增/修改 handler 时需同步更新注解。可通过 CI lint 检查 spec 是否过时来缓解。
- **[生成类型的同步]** → 前端类型与后端 spec 可能不同步。在 CI 中加入 `generate:api` 检查步骤，确保提交时类型是最新的。
- **[GitHub Pages CORS]** → 静态站点跨域请求 worker API 需要 CORS 支持。Worker 已有 CORS 中间件，需确认配置覆盖 GitHub Pages 域名（当前已使用通配符 `*`）。
- **[HashRouter SEO]** → Hash 路由对 SEO 不友好，但 Wheel 是管理后台工具，不需要 SEO。
- **[localStorage 安全性]** → API base URL 存在 localStorage 中，可被 XSS 攻击篡改。这是管理工具的可接受风险，与 JWT token 的存储策略一致。

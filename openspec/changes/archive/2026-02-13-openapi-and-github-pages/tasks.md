## 1. OpenAPI 注解与规范生成（后端）

- [x] 1.1 安装 swaggo/swag 依赖并在 Makefile 添加 `docs` target
- [x] 1.2 在 `cmd/worker/main.go`（或新建 `doc.go`）添加 swag 通用 API info 注解（title, version, description, securityDefinitions）
- [x] 1.3 为 User/Auth 相关 handler 添加 swag 注解（login, changePassword, changeUsername, userStatus）
- [x] 1.4 为 Channel handler 添加 swag 注解（list, create, update, enable, delete, fetchModel, fetchModelPreview, syncAllModels, lastSyncTime）
- [x] 1.5 为 Group handler 添加 swag 注解（list, create, update, delete, reorder, modelList）
- [x] 1.6 为 API Key handler 添加 swag 注解（list, create, update, delete, apikeyLogin, apikeyStats）
- [x] 1.7 为 Log handler 添加 swag 注解（list, get, delete, clear, replay）
- [x] 1.8 为 Stats handler 添加 swag 注解（global, channel, total, today, daily, hourly, model, apikey）
- [x] 1.9 为 Setting handler 添加 swag 注解（get, update, export, import）
- [x] 1.10 为 Model handler 添加 swag 注解（list, listByChannel, create, update, delete, metadata, refreshMetadata, updatePrice, lastUpdateTime）
- [x] 1.11 运行 `swag init` 生成 `docs/` 目录并提交到仓库

## 2. API 文档可视化（后端）

- [x] 2.1 在 worker 中注册 `GET /docs` 路由，返回嵌入 Scalar API Reference 的 HTML 页面
- [x] 2.2 在 worker 中注册 `GET /docs/openapi.json` 路由，返回生成的 OpenAPI spec JSON
- [x] 2.3 验证 `/docs` 和 `/docs/openapi.json` 可无认证访问

## 3. 前端 TypeScript 类型生成

- [x] 3.1 安装 `openapi-typescript` 和 `openapi-fetch` 依赖到 web 项目
- [x] 3.2 在 `package.json` 添加 `generate:api` script，从 worker 的 `docs/swagger.json` 生成类型到 `src/lib/api.gen.d.ts`
- [x] 3.3 运行 codegen 生成初始类型文件
- [x] 3.4 使用 `openapi-fetch` 创建类型安全的 API client（`src/lib/api-client.ts`），配合生成的类型
- [x] 3.5 将现有 `lib/api.ts` 中的手动类型和请求函数迁移到新的 client

## 4. 可配置 API Base URL（前端）

- [x] 4.1 在 Zustand auth store 中添加 `apiBaseUrl` 字段，带 localStorage 持久化
- [x] 4.2 修改 `apiFetch`（或新 client）在请求时读取 `apiBaseUrl` 作为前缀
- [x] 4.3 修改 `getWsUrl()` 优先使用 store 中的 `apiBaseUrl`（替代 `VITE_API_BASE_URL`）
- [x] 4.4 在登录页面添加 API URL 配置输入框（未配置时显示）
- [x] 4.5 在设置页面添加 API URL 查看/修改区域
- [x] 4.6 添加连接验证逻辑：保存时调用 `GET /` 检查连通性并反馈

## 5. GitHub Pages 部署支持

- [x] 5.1 修改 `routes.tsx` 支持根据 `VITE_HASH_ROUTER` 环境变量条件选择 `HashRouter` 或 `BrowserRouter`
- [x] 5.2 在 Vite build 配置中支持 GitHub Pages 的 base path（通过 `VITE_BASE_PATH` 环境变量）
- [x] 5.3 构建后将 `index.html` 复制为 `404.html`（可通过 Vite plugin 或 build script 实现）
- [x] 5.4 创建 `.github/workflows/pages.yml` workflow：push to main → build web（VITE_HASH_ROUTER=true）→ deploy to GitHub Pages
- [x] 5.5 在仓库设置中确认 GitHub Pages source 配置为 GitHub Actions
- [x] 5.6 端到端验证：确认 GitHub Pages 部署的站点可以设置 API URL、登录并正常使用

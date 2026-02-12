## Why

Wheel 目前没有任何正式的 API 文档，前端的 TypeScript 类型定义（`lib/api.ts`）与后端 Go handler 之间靠手动同步维护，容易出现不一致。同时 web 前端只能通过 Vercel rewrites 或 Docker 反向代理连接后端，无法作为纯静态站点独立部署到 GitHub Pages 供用户免安装即用——用户需要自行设置后端地址即可管理自己的 Wheel 实例。

## What Changes

- 为 Go worker 添加 OpenAPI 3.0 规范定义，使用注解自动生成 `openapi.yaml`
- 在 worker 中集成 Scalar API 可视化，提供交互式 API 文档界面
- 基于 OpenAPI 规范为前端生成 TypeScript 类型和 API client，替代手动维护的 `lib/api.ts`
- 在 web 前端添加「设置接口地址」功能，允许用户在 UI 中配置 worker 的 base URL
- 将 web 前端改造为可独立部署的纯静态 SPA（支持 GitHub Pages 部署）
- 新增 GitHub Actions workflow，自动构建 web 并部署到 GitHub Pages

## Capabilities

### New Capabilities

- `openapi-spec`: 为 Go worker 的所有 API 端点定义 OpenAPI 3.0 注解并自动生成规范文件
- `api-visualization`: 在 worker 中集成 Scalar API Reference 可视化页面
- `openapi-codegen`: 基于 OpenAPI 规范为前端自动生成 TypeScript 类型和 API client
- `configurable-api-url`: 在 web 前端提供设置界面，允许用户配置 worker base URL，持久化到 localStorage
- `github-pages-deploy`: 将 web 构建为纯静态 SPA 并通过 GitHub Actions 自动部署到 GitHub Pages

### Modified Capabilities

_(无已有 capability 的需求级变更)_

## Impact

- **后端 (`apps/worker/`)**: 新增 OpenAPI 注解、swag 依赖、Scalar UI 路由；handler 代码需添加注释注解
- **前端 (`apps/web/`)**: `lib/api.ts` 将被生成代码替代；新增 API URL 配置页面/组件；API 请求层需支持动态 base URL
- **构建系统**: 新增 `swag init` 步骤生成 OpenAPI spec；新增 `openapi-typescript` 或类似工具生成前端类型
- **CI/CD (`.github/workflows/`)**: 新增 GitHub Pages 部署 workflow
- **Vite 配置**: 调整 proxy 和 build 配置以支持 hash router（GitHub Pages 兼容）
- **依赖**: Go 侧新增 swaggo/swag；JS 侧新增 openapi 代码生成工具

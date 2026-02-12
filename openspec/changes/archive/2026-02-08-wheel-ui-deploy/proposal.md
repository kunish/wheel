## Why

Wheel 当前架构为 Go 单体应用（后端 API + 嵌入 Next.js 前端静态文件），仅支持 Docker 部署。这对个人用户来说存在两个问题：(1) 需要自建服务器运行 Docker，成本和运维门槛较高；(2) 前端 UI 功能相对基础，缺少一键部署能力和更丰富的管理体验。通过将前端独立化并扩展 UI 逻辑，支持 Cloudflare Worker 和 Vercel 一键部署，可以大幅降低用户使用门槛，同时利用边缘计算提升全球访问速度。

## What Changes

- **前端独立化**：将 Next.js 前端从 Go 嵌入模式中解耦，使其可独立部署到 Vercel 或 Cloudflare Pages
- **Edge API 代理层**：新增 Cloudflare Worker 作为 API 代理/中继层，实现核心的 LLM 请求路由和负载均衡逻辑
- **UI 扩展**：基于 shadcn/ui 增强现有管理面板，包括：
  - 一键部署向导页面（支持 CF Worker 和 Vercel）
  - 增强的 Channel/Group 管理体验
  - 实时状态监控仪表盘
  - 部署配置管理界面
- **部署配置生成**：自动生成 `wrangler.toml`、`vercel.json` 等部署配置文件
- **数据存储适配**：支持 Cloudflare D1/KV 作为 Edge 环境下的数据存储方案

## Capabilities

### New Capabilities

- `edge-api-proxy`: Cloudflare Worker 上运行的 API 代理层，实现 LLM 请求路由、负载均衡、协议转换等核心逻辑
- `deploy-wizard`: 一键部署向导 UI，引导用户完成 Cloudflare Worker 和 Vercel 的部署配置
- `deploy-config-gen`: 自动生成目标平台的部署配置文件（wrangler.toml、vercel.json、环境变量模板等）
- `edge-data-store`: Edge 环境下的数据存储抽象层，支持 Cloudflare D1/KV 和 Vercel KV/Postgres
- `enhanced-dashboard`: 增强的管理仪表盘 UI，包括实时状态监控、部署状态跟踪、更丰富的数据可视化

### Modified Capabilities

_无已有 spec 需要修改（这是一个全新的项目扩展）_

## Impact

- **前端代码（`web/`）**：大量新增和修改，需要新增部署向导、增强仪表盘等多个页面模块
- **新增 Worker 代码**：全新的 Cloudflare Worker 项目，使用 TypeScript 实现核心 API 代理逻辑
- **构建流程**：需要新增 Worker 构建和前端独立构建的流程，与原有 Go 嵌入模式并存
- **依赖新增**：Cloudflare Workers SDK（wrangler）、@cloudflare/workers-types、Vercel CLI 等
- **数据库兼容性**：需要设计抽象层，使同一套逻辑同时支持 SQLite（原有）和 D1/KV（Edge）
- **API 契约**：前端需要支持连接不同后端（Go 原版 / CF Worker），API 接口保持兼容

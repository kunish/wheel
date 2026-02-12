## Context

Wheel (Wheel) 是一个 LLM API 聚合服务，由两部分组成：

- **Worker** (Cloudflare Workers + D1 + KV)：API 后端
- **Web** (Next.js + shadcn/ui)：管理仪表盘

当前 README 提供了手动 CLI 部署步骤，但缺少：

1. 一键部署按钮 — 多数开源项目标配，降低部署门槛
2. Docker 自托管方案 — 满足不使用 Serverless 平台的用户
3. 结构化的部署教程 — 当前部署文档分散且缺少截图/链接

## Goals / Non-Goals

**Goals:**

- 添加 Vercel 一键部署按钮，用于 Web 仪表盘
- 添加 Cloudflare Workers 一键部署按钮（Deploy with Workers），用于 API 后端
- 新增 Docker Compose 部署方案，一条命令启动 Web 仪表盘
- 重组 README 使部署路径清晰（一键 > Docker > 手动）
- 保持 README 简洁，详细步骤链接到独立文档（如需要）

**Non-Goals:**

- 不构建 Worker 的 Docker 镜像（Worker 必须部署到 Cloudflare）
- 不修改现有代码逻辑
- 不添加 CI/CD pipeline 配置
- 不支持 Kubernetes Helm Chart（未来可考虑）

## Decisions

### 1. Vercel 一键部署

使用 Vercel Deploy Button 标准方案：

- 链接格式：`https://vercel.com/new/clone?repository-url=...&env=NEXT_PUBLIC_API_BASE_URL`
- 通过 URL 参数指定所需环境变量
- `vercel.json` 已存在，无需额外配置

**为什么不用 Vercel template**：template 需要额外申请审核流程，Deploy Button 即开即用。

### 2. Cloudflare Deploy Button

使用 Cloudflare 的 Deploy with Workers 按钮：

- 链接格式：`https://deploy.workers.cloudflare.com/?url=https://github.com/<repo>`
- 需要在 `wrangler.toml` 中确保 `name` 和绑定 ID 使用 placeholder
- 用户部署后需手动设置 D1 database 和 KV namespace（平台限制）

**为什么分离按钮**：Worker 和 Web 是独立部署单元，合并一键部署会增加复杂性。

### 3. Docker 方案仅覆盖 Web

- Web 仪表盘可以构建为标准 Next.js Docker 镜像
- Worker 无法 Docker 化（依赖 Cloudflare D1/KV 运行时），必须部署到 Cloudflare
- Docker Compose 文件提供 Web 启动，并在文档中说明 Worker 需单独部署

**为什么不用 miniflare**：miniflare 适合开发测试，不适合生产自托管。

### 4. README 结构

采用「渐进式部署」结构：

1. 项目介绍 + 特性亮点（带 badge）
2. 快速开始（一键部署按钮）
3. Docker 部署
4. 手动部署
5. 环境变量参考
6. 开发指南
7. 项目结构

## Risks / Trade-offs

- **[Deploy Button URL 硬编码]** → 需要用户 fork 后修改 URL 中的仓库路径；在 README 中说明这一点
- **[Cloudflare D1 迁移]** → 一键部署后用户仍需手动执行 D1 迁移；在部署后步骤中明确说明
- **[Docker 只覆盖 Web]** → 可能让用户误以为可以完全 Docker 化；在文档中显著标注 Worker 必须部署到 Cloudflare

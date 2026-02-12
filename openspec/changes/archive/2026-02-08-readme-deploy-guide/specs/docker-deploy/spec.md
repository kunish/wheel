## ADDED Requirements

### Requirement: Dockerfile for Web Dashboard

项目 SHALL 提供一个多阶段构建的 Dockerfile，用于构建 Next.js Web 仪表盘的生产镜像。

#### Scenario: 构建 Docker 镜像

- **WHEN** 用户在项目根目录执行 `docker build -f apps/web/Dockerfile -t wheel-web .`
- **THEN** 构建过程 SHALL 安装 pnpm 依赖、编译 Next.js 应用、输出精简的生产镜像

#### Scenario: 运行 Docker 容器

- **WHEN** 用户执行 `docker run -p 3000:3000 -e NEXT_PUBLIC_API_BASE_URL=https://api.example.com wheel-web`
- **THEN** Web 仪表盘 SHALL 在端口 3000 正常启动并连接到指定的 Worker API

### Requirement: Docker Compose 编排

项目 SHALL 提供 `docker-compose.yml`，一条命令启动 Web 仪表盘。

#### Scenario: 一键启动

- **WHEN** 用户执行 `docker compose up -d`
- **THEN** Web 仪表盘容器 SHALL 启动并对外暴露 3000 端口

#### Scenario: 环境变量配置

- **WHEN** docker-compose.yml 中引用环境变量
- **THEN** SHALL 支持通过 `.env` 文件或环境变量设置 `NEXT_PUBLIC_API_BASE_URL`

### Requirement: Worker 不支持 Docker 的说明

Docker 部署文档 SHALL 明确说明 Worker（API 后端）必须部署到 Cloudflare Workers 平台，不支持 Docker 自托管。

#### Scenario: 用户查看 Docker 部署文档

- **WHEN** 用户阅读 Docker 部署章节
- **THEN** 文档 SHALL 在醒目位置说明 "Worker 依赖 Cloudflare D1/KV 运行时，必须部署到 Cloudflare Workers"，并提供 Worker 部署链接

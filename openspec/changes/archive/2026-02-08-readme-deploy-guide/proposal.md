## Why

当前 README 仅包含手动 CLI 部署步骤，缺少一键部署按钮和 Docker 部署方式。新用户需要逐步执行多条命令才能完成部署，门槛较高。添加 Vercel/Cloudflare 一键部署按钮和 Docker 部署方案可以显著降低上手难度，同时提升项目的专业度和可发现性。

## What Changes

- 添加 Vercel 一键部署按钮（Deploy to Vercel），自动配置 Web 仪表盘
- 添加 Cloudflare Workers 一键部署按钮（Deploy to Cloudflare Workers），自动配置 API 后端
- 新增 Docker / Docker Compose 部署方式文档，支持本地自托管
- 新增 Dockerfile 和 docker-compose.yml 配置文件
- 重组 README 结构：特性亮点、部署方式（一键/Docker/手动）、配置说明、开发指南

## Capabilities

### New Capabilities

- `deploy-buttons`: Vercel + Cloudflare 一键部署按钮配置与链接生成
- `docker-deploy`: Docker 镜像构建和 Docker Compose 编排文件，支持 Web + Worker 本地自托管
- `readme-restructure`: README.md 内容重组，包含特性介绍、多种部署方式教程、环境变量说明

### Modified Capabilities

_(无现有 spec 需要修改)_

## Impact

- 新增文件：`Dockerfile`（web）、`docker-compose.yml`
- 修改文件：`README.md`
- 涉及：项目文档、部署配置、CI/CD 入口
- 不影响现有代码逻辑和 API

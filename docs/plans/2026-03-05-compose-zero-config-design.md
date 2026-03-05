# Docker Compose Zero-Config 设计（内置网关）

## 目标

让用户在 Docker Compose 场景下无需手写或挂载根目录 `Caddyfile`，仅通过 `docker compose up -d` 即可完成前端静态托管 + API 反向代理，并保持现有路径兼容：`/v1`、`/api`、`/mcp/sse`、`/mcp/message`、`/docs`。

## 现状问题

- 目前存在两层 Caddy：
  - `apps/web` 容器内 Caddy（静态文件服务）
  - 根目录单独 `caddy` 服务（路由转发）
- 用户部署需要额外准备根 `Caddyfile` 和 volume 挂载，使用门槛偏高。

## 方案

采用单层 Caddy：将网关转发规则并入 `apps/web/Caddyfile`，移除根 `caddy` 服务。

### 路由规则

- `/v1/*` -> `worker:8787`
- `/api/*` -> `worker:8787`
- `/mcp/sse*` -> `worker:8787`
- `/mcp/message*` -> `worker:8787`
- `/docs*` -> `worker:8787`
- 其他路径 -> 静态文件（`/srv`）并 `try_files {path} /index.html`

### Compose 调整

- 删除 `caddy` 服务块（含端口和 Caddyfile 挂载）。
- 在 `web` 服务增加端口映射 `3000:3000`，作为唯一对外入口。

## 兼容性

- 对客户端调用保持不变：仍使用 `http://host:3000/v1`；MCP 使用显式端点 `http://host:3000/mcp/sse`（SSE）与 `http://host:3000/mcp/message`（消息投递）。
- 管理后台和 SPA 路由保持不变。

## 风险与缓解

- 风险：`web` 容器承担静态托管与反代双职责。
  - 缓解：当前流量规模下可接受；后续如需拆分，可恢复独立网关镜像。
- 风险：路径匹配顺序错误导致静态路由吞掉 API。
  - 缓解：在 Caddy 中先定义 API `handle`，最后再定义静态 `handle`。

## 验证标准

- `docker compose up -d` 后无需额外文件即可访问：
  - `GET /` 返回前端页面
  - `GET /api/v1/*` 可达 worker
  - `POST /v1/chat/completions` 可达 worker
  - `GET /mcp/sse` 可达 MCP SSE 端点
  - `POST /mcp/message?sessionId=...` 可达 MCP 消息端点

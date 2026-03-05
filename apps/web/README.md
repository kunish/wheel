# @wheel/web

Wheel 的前端管理界面（Vite + React + TypeScript）。

## 开发

在仓库根目录执行：

```bash
pnpm install
pnpm --filter @wheel/web run dev
```

默认地址：`http://localhost:5173`

开发环境通过 Vite 代理以下后端路径到 `VITE_API_BASE_URL`（默认 `http://localhost:8787`）：

- `/api`
- `/v1`
- `/docs`
- `/mcp/sse`
- `/mcp/message`

## 构建

```bash
pnpm --filter @wheel/web run build
```

构建产物在 `apps/web/dist`。

## 生产部署（容器）

在 Docker Compose 部署中，`web` 镜像内置 Caddy：

- 前端静态资源由 `web` 托管
- `/v1`、`/api`、`/docs`、`/mcp/sse`、`/mcp/message` 反代到 `worker:8787`

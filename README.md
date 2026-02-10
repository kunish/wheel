<div align="center">

<img src="apps/web/src/app/icon.svg" width="80" height="80" alt="Wheel Logo">

# Wheel

**LLM API Gateway — Aggregate, Balance, Observe.**

统一多家 LLM 提供商接口，智能负载均衡与自动故障转移，完整的用量追踪与成本管理。

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https%3A%2F%2Fgithub.com%2Fkunish%2Fwheel&env=NEXT_PUBLIC_API_BASE_URL&envDescription=Worker%20API%20Base%20URL&envLink=https%3A%2F%2Fgithub.com%2Fkunish%2Fwheel%23environment-variables&project-name=wheel)

</div>

---

## Features

### Relay Proxy

- **多提供商聚合** — OpenAI、Anthropic、Google Gemini、Volcengine（火山引擎），统一为 OpenAI 兼容接口
- **协议自动转换** — OpenAI ↔ Anthropic 格式双向转换，包括 tool calling、system prompt、thinking 参数
- **SSE 流式转发** — 首 token 超时检测（可配置），超时自动 failover 到下一通道
- **3 轮重试** — 遍历分组内所有通道，自动跳过 429 限速的 key，上一次失败的错误信息会传递给客户端
- **4 种负载均衡** — Round Robin / Random / Failover（优先级） / Weighted（加权随机）
- **Embeddings 代理** — 支持 `/v1/embeddings` 端点转发
- **`/v1/models` 端点** — 自动检测请求格式，返回 OpenAI 或 Anthropic 格式的模型列表

### Channel Management

- **通道管理** — 增删改查，启用/禁用，批量 key 管理
- **多 Base URL** — 每个通道支持多个端点地址
- **模型自动发现** — 从上游提供商拉取可用模型列表
- **模型自动同步** — 定时从上游同步模型变更，自动更新分组
- **自动分组** — Exact（精确匹配）/ Fuzzy（前缀匹配）两种策略
- **自定义请求头** — 按通道注入额外 HTTP 头
- **参数覆盖** — 按通道覆盖请求参数（JSON merge）
- **通道级代理** — 为特定通道设置 HTTP 代理

### Group Management

- **分组管理** — 增删改查，拖拽排序
- **通道-模型配对** — 每个分组可包含多个通道的不同模型
- **优先级/权重** — 为 Failover 和 Weighted 模式配置
- **首 token 超时** — 每个分组可独立设置超时阈值

### API Key Management

- **API Key 管理** — 增删改查，自动生成密钥
- **模型白名单** — 限制 key 可访问的模型
- **用量配额** — 设置最大成本限额
- **过期时间** — 设置 key 有效期
- **用量追踪** — 每个 key 的累计成本实时统计

### Cost & Pricing

- **自动定价同步** — 从 [models.dev](https://models.dev) 同步 9 家提供商定价（OpenAI、Anthropic、Google、DeepSeek、xAI、阿里、智谱、Minimax、月之暗面）
- **手动定价管理** — 增删改查自定义模型价格
- **缓存 token 计费** — 支持 Anthropic cache_read/cache_write、OpenAI cached_tokens
- **请求级成本计算** — 每次请求实时计算并累计到 API Key 和通道 Key

### Monitoring & Statistics

- **实时仪表盘** — WebSocket 推送，数据即时更新
- **活跃度热力图** — 年视图 / 月视图 / 周视图，点击单元格跳转到对应日志
- **成本趋势图** — 今日（小时）/ 近 7 天 / 近 30 天
- **通道排行** — 按成本 / 请求量排名，动画切换
- **模型统计** — 请求量、token 用量、成本、延迟，可排序
- **多维统计** — 全局 / 每日 / 每小时 / 按通道 / 按模型 / 按 API Key

### Request Logging

- **完整日志记录** — 请求/响应内容（智能截断），token 用量分解
- **重试时间线** — 每次尝试的通道、模型、耗时、错误信息
- **高级过滤** — 时间范围、模型、通道、状态、关键词搜索
- **请求重放** — 一键重新执行历史请求
- **自动清理** — 可配置日志保留天数

### Data Management

- **JSON 导出** — 完整数据库备份（通道、分组、Key、设置，可选日志）
- **JSON 导入** — ID 自动重映射，按名称去重，导入结果摘要
- **部署向导** — 图形化生成 Docker 配置文件

---

## Architecture

```
apps/
  worker/     API Backend (Hono + better-sqlite3 + Drizzle ORM)
  web/        Next.js Dashboard (shadcn/ui + Tailwind)
packages/
  core/       Shared TypeScript types and enums
```

---

## Deploy

### Option 1: Vercel + Docker

Web 仪表盘部署到 Vercel，Worker 后端 Docker 自部署。最简单的上手方式。

**Step 1 — 部署 Worker**

```bash
docker run -d \
  -p 8787:8787 \
  -v wheel-data:/app/data \
  -e JWT_SECRET=$(openssl rand -hex 32) \
  ghcr.io/kunish/wheel-worker
```

**Step 2 — 部署 Web 到 Vercel**

点击按钮一键部署，填入 Worker 的公网地址（如 `https://api.your-domain.com`）作为 `NEXT_PUBLIC_API_BASE_URL`：

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https%3A%2F%2Fgithub.com%2Fkunish%2Fwheel&env=NEXT_PUBLIC_API_BASE_URL&envDescription=Worker%20API%20Base%20URL&envLink=https%3A%2F%2Fgithub.com%2Fkunish%2Fwheel%23environment-variables&project-name=wheel)

默认管理员账号：`admin` / `admin`，请登录后立即修改。

---

### Option 2: Docker Self-Hosted

一键部署 Worker + Web，使用 SQLite 存储，无需云服务账号。

需要一个反向代理（如 Caddy、Nginx）处理 HTTPS 和路由分流。

**docker-compose.yml**：

```yaml
volumes:
  worker-data:

services:
  worker:
    image: ghcr.io/kunish/wheel-worker
    restart: always
    environment:
      JWT_SECRET: ${JWT_SECRET:?Please set JWT_SECRET}
      ADMIN_USERNAME: ${ADMIN_USERNAME:-admin}
      ADMIN_PASSWORD: ${ADMIN_PASSWORD:-admin}
      DB_PATH: /app/data/wheel.db
      PORT: 8787
    volumes:
      - worker-data:/app/data

  web:
    image: ghcr.io/kunish/wheel-web
    restart: always
    depends_on:
      - worker
```

**Caddyfile**（推荐）：

```
your-domain.com {
    handle /api/* {
        reverse_proxy worker:8787
    }
    handle /v1/* {
        reverse_proxy worker:8787
    }
    handle {
        reverse_proxy web:3000
    }
}
```

```bash
# 设置环境变量
echo "JWT_SECRET=$(openssl rand -hex 32)" > .env

# 启动
docker compose up -d
```

默认管理员账号：`admin` / `admin`，请登录后立即修改。

---

### Option 3: Manual Build

#### 前置条件

- Node.js >= 22
- pnpm >= 10

```bash
pnpm install

# Worker
pnpm --filter @wheel/worker build
JWT_SECRET=your-secret node apps/worker/dist/index.js

# Web
pnpm --filter @wheel/web build
node apps/web/.next/standalone/apps/web/server.js
```

---

## Environment Variables

| 变量                       | 组件         | 描述              | 必填              | 默认值            |
| -------------------------- | ------------ | ----------------- | ----------------- | ----------------- |
| `JWT_SECRET`               | Worker       | JWT 签名密钥      | Yes               | —                 |
| `ADMIN_USERNAME`           | Worker       | 管理员用户名      | No                | `admin`           |
| `ADMIN_PASSWORD`           | Worker       | 管理员密码        | No                | `admin`           |
| `DB_PATH`                  | Worker       | SQLite 数据库路径 | No                | `./data/wheel.db` |
| `PORT`                     | Worker       | HTTP 端口         | No                | `8787`            |
| `NEXT_PUBLIC_API_BASE_URL` | Web (Vercel) | Worker API 地址   | Vercel 部署时必填 | —                 |

---

## Development

```bash
pnpm install

# 同时启动 Worker 和 Web
pnpm dev

# 或分别启动
pnpm dev:worker    # http://localhost:8787
pnpm dev:web       # http://localhost:3000
```

---

## License

MIT

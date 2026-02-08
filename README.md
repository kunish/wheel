# Wheel

LLM API 聚合与负载均衡服务 — 部署到 Cloudflare Workers + Vercel。

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**[English](#) · [中文](#)**

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https%3A%2F%2Fgithub.com%2FYOUR_USERNAME%2Fwheel&env=NEXT_PUBLIC_API_BASE_URL&envDescription=Your%20Cloudflare%20Worker%20API%20URL&project-name=wheel-web)
[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https://github.com/YOUR_USERNAME/wheel)

> **Note**: 请将上方按钮链接中的 `YOUR_USERNAME` 替换为你的 GitHub 用户名（fork 后自动生效）。

---

## Features

- **多 LLM 提供商支持** — OpenAI、Anthropic、Google Gemini、DeepSeek 等，协议自动转换
- **4 种负载均衡** — RoundRobin、Random、Failover、Weighted
- **SSE 流式转发** — 首 token 超时检测，自动 failover 到下一通道
- **API Key 管理** — 模型/通道级别权限控制，用量配额与成本追踪
- **管理仪表盘** — 通道、分组、API Key、日志、统计、价格、部署配置
- **模型价格同步** — 自动从 models.dev 同步最新定价

## Architecture

```
apps/
  worker/     API 后端 (Hono + Drizzle ORM)
              ├─ Cloudflare Workers (D1 + KV)
              └─ Node.js (better-sqlite3 + in-memory KV)
  web/        Next.js Dashboard (shadcn/ui)         ← 管理界面
packages/
  core/       Shared TypeScript types and enums
```

---

## Deploy

### Option 1: 一键部署 (推荐)

最快的上手方式 — 分别部署 Worker（API 后端）和 Web（管理仪表盘）。

#### 1a. 部署 Worker 到 Cloudflare

点击上方 **Deploy to Cloudflare Workers** 按钮，按照提示完成部署。

部署完成后，还需要：

```bash
# 创建 D1 数据库
npx wrangler d1 create wheel-db

# 创建 KV 命名空间
npx wrangler kv namespace create wheel-cache

# 更新 wrangler.toml 中的 database_id 和 KV id

# 执行数据库迁移
npx wrangler d1 migrations apply wheel-db

# 设置 JWT 密钥
npx wrangler secret put JWT_SECRET

# 重新部署
npx wrangler deploy
```

#### 1b. 部署 Web 到 Vercel

点击上方 **Deploy with Vercel** 按钮，填写环境变量：

| 变量                       | 值                                                             |
| -------------------------- | -------------------------------------------------------------- |
| `NEXT_PUBLIC_API_BASE_URL` | 你的 Worker URL，如 `https://wheel.your-subdomain.workers.dev` |

部署完成后即可访问管理仪表盘。

---

### Option 2: Docker 全栈部署 (自托管)

一键部署 Web + Worker，使用 SQLite 存储，无需 Cloudflare 账号。

```bash
# 1. 复制环境变量配置
cp .env.example .env
# 编辑 .env，至少设置 JWT_SECRET

# 2. 启动全栈服务
docker compose up -d
```

- Worker API: `http://localhost:8787`
- Web 仪表盘: `http://localhost:3000`

数据持久化在 Docker volume `worker-data` 中。

#### 仅部署 Web 仪表盘 (Worker 在 Cloudflare)

如果 Worker 已经部署到 Cloudflare，可以只用 Docker 运行 Web：

```bash
docker build -f apps/web/Dockerfile -t wheel-web .
docker run -p 3000:3000 -e NEXT_PUBLIC_API_BASE_URL=https://your-worker.workers.dev wheel-web
```

---

### Option 3: 手动部署

#### 前置条件

- Node.js >= 18
- pnpm >= 9
- Cloudflare 账号（Worker、D1、KV）
- Vercel 账号（可选，用于托管仪表盘）

#### 部署 Worker (API)

```bash
# 创建 D1 数据库
npx wrangler d1 create wheel-db

# 创建 KV 命名空间
npx wrangler kv namespace create wheel-cache

# 更新 wrangler.toml 中的 database_id 和 KV id

# 执行数据库迁移
npx wrangler d1 migrations apply wheel-db

# 设置 JWT 密钥
npx wrangler secret put JWT_SECRET

# 部署
npx wrangler deploy
```

#### 部署 Web (仪表盘)

```bash
# 链接项目
npx vercel link

# 设置环境变量
npx vercel env add NEXT_PUBLIC_API_BASE_URL
# 输入 Worker URL: https://wheel.<subdomain>.workers.dev

# 部署
npx vercel deploy --prod
```

或使用仪表盘内的 **Deploy Wizard**（Settings > Deploy）生成配置文件。

---

## Environment Variables

| 变量                       | 组件             | 描述              | 必填 | 默认值                  |
| -------------------------- | ---------------- | ----------------- | ---- | ----------------------- |
| `JWT_SECRET`               | Worker           | JWT 签名密钥      | 是   | —                       |
| `ADMIN_USERNAME`           | Worker           | 管理员用户名      | 否   | `admin`                 |
| `ADMIN_PASSWORD`           | Worker           | 管理员密码        | 否   | `admin123`              |
| `DB_PATH`                  | Worker (Node.js) | SQLite 数据库路径 | 否   | `./data/wheel.db`       |
| `PORT`                     | Worker (Node.js) | HTTP 监听端口     | 否   | `8787`                  |
| `NEXT_PUBLIC_API_BASE_URL` | Web              | Worker API 地址   | 是   | `http://localhost:8787` |

---

## Development

```bash
# 安装依赖
pnpm install

# 同时启动 Worker 和 Web 开发服务器
pnpm dev

# 或分别启动
pnpm dev:worker    # http://localhost:8787
pnpm dev:web       # http://localhost:3000
```

## Project Structure

```
apps/worker/src/
  index.ts              CF Workers 入口
  index.node.ts         Node.js 自托管入口
  runtime/              运行时抽象层 (types, cf, node)
  middleware/            JWT + API Key 认证
  routes/               管理 API (channel, group, apikey, log, stats, setting, user)
  relay/                代理核心 (parser, matcher, balancer, adapter, proxy, handler)
  db/                   Drizzle ORM schema + DAL

apps/web/src/
  app/
    login/              登录页
    (protected)/        需认证的页面
      dashboard/        使用统计 + 图表
      channels/         通道管理
      groups/           分组管理
      apikeys/          API Key 管理
      logs/             日志查看器
      prices/           价格管理
      settings/         系统设置
      deploy/           部署向导
  lib/
    api.ts              API 客户端
    store/auth.ts       Zustand 认证状态
  components/           shadcn/ui + 布局组件
```

## License

MIT

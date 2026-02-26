<div align="center">

<img src="apps/web/public/icon.svg" width="80" height="80" alt="Wheel Logo">

# Wheel

**LLM API 网关 — 聚合、均衡、观测。**

统一多家 LLM 提供商接口，智能负载均衡与自动故障转移，完整的用量追踪与成本管理。

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[![Deploy on Zeabur](https://zeabur.com/button.svg)](https://zeabur.com/templates)

**Go · React · TiDB · Caddy**

</div>

---

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/screenshots/dashboard-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="docs/screenshots/dashboard-light.png">
  <img alt="仪表盘" src="docs/screenshots/dashboard-light.png" width="100%">
</picture>

<details>
<summary>更多截图</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/screenshots/model-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="docs/screenshots/model-light.png">
  <img alt="模型与分组管理" src="docs/screenshots/model-light.png" width="100%">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/screenshots/logs-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="docs/screenshots/logs-light.png">
  <img alt="请求日志" src="docs/screenshots/logs-light.png" width="100%">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/screenshots/settings-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="docs/screenshots/settings-light.png">
  <img alt="系统设置" src="docs/screenshots/settings-light.png" width="100%">
</picture>

</details>

---

## 功能特性

- **多提供商聚合** — OpenAI / Anthropic / Gemini 统一为 OpenAI 兼容接口，协议自动转换
- **智能路由** — 4 种负载均衡（Round Robin / Random / Failover / Weighted），3 轮重试，熔断器，会话保持
- **SSE 流式转发** — 首 token 超时检测，超时自动 failover
- **通道管理** — 多 Base URL、模型自动发现与同步、自定义请求头与参数覆盖
- **分组管理** — 通道-模型配对，优先级/权重，独立超时与会话保持配置
- **API Key 管理** — 模型白名单、用量配额、过期时间
- **成本管理** — 从 [models.dev](https://models.dev) 自动同步 9 家提供商定价，缓存 token 计费，请求级成本计算
- **实时监控** — WebSocket 仪表盘，活跃度热力图，成本趋势，通道/模型/Key 多维统计
- **请求日志** — 完整请求/响应记录，重试时间线，高级过滤，一键重放
- **数据管理** — JSON 导入/导出，图形化系统配置
- **双语 & 主题** — 中文 / English，亮色 / 暗色 / 跟随系统
- **MCP 网关** — 连接外部 MCP 服务器，聚合工具并统一暴露为 MCP Server 端点，支持 HTTP/SSE/STDIO 三种传输协议

---

## 部署

### Zeabur

一键部署，点击上方按钮即可。

### Docker Compose

```yaml
volumes:
  tidb-data:

services:
  tidb:
    image: pingcap/tidb:latest
    restart: always
    volumes:
      - tidb-data:/tmp/tidb

  worker:
    image: ghcr.io/kunish/wheel-worker
    restart: always
    environment:
      JWT_SECRET: ${JWT_SECRET:?请设置 JWT_SECRET}
      ADMIN_PASSWORD: ${ADMIN_PASSWORD:-admin}
      DB_DSN: root:@tcp(tidb:4000)/wheel?parseTime=true&charset=utf8mb4
    depends_on:
      - tidb

  web:
    image: ghcr.io/kunish/wheel-web
    restart: always
    depends_on:
      - worker

  caddy:
    image: caddy:2-alpine
    restart: always
    ports:
      - "3000:3000"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
    depends_on:
      - worker
      - web
```

创建 `Caddyfile`：

```caddyfile
:3000 {
	handle /v1/* {
		reverse_proxy worker:8787
	}

	handle /mcp/* {
		reverse_proxy worker:8787
	}

	handle /api/* {
		reverse_proxy worker:8787
	}

	handle {
		reverse_proxy web:3000
	}
}
```

启动服务：

```bash
echo "JWT_SECRET=$(openssl rand -hex 32)" > .env
docker compose up -d
# 访问 http://localhost:3000
```

### 手动构建

```bash
# Worker (Go >= 1.26, 需要 TiDB/MySQL 实例)
cd apps/worker && go build -o wheel ./cmd/worker
JWT_SECRET=your-secret DB_DSN="root:@tcp(127.0.0.1:4000)/wheel?parseTime=true&charset=utf8mb4" ./wheel

# Web (Node >= 22, pnpm >= 10)
pnpm install && pnpm --filter @wheel/web build
# 静态文件服务器托管 apps/web/dist
```

---

## 使用

Wheel 兼容 OpenAI API 格式，配置好通道和分组后，将任意 AI 工具的 `base_url` 指向 Wheel 即可。

**Claude Code**

```bash
ANTHROPIC_BASE_URL=http://localhost:3000 ANTHROPIC_API_KEY=your-api-key claude
```

**opencode**

```bash
export OPENAI_BASE_URL=http://localhost:3000/v1
export OPENAI_API_KEY=your-api-key
opencode
```

**aider**

```bash
aider --openai-api-base http://localhost:3000/v1 --openai-api-key your-api-key
```

**curl**

```bash
curl http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}'
```

### MCP 网关

Wheel 可以作为 MCP 网关，连接多个外部 MCP 服务器并将所有工具聚合为一个统一的 MCP Server 端点。

#### 1. 添加 MCP 客户端

在管理面板的 MCP 页面中添加 MCP 客户端，支持三种连接方式：

| 连接类型  | 适用场景                                  | 配置                     |
| --------- | ----------------------------------------- | ------------------------ |
| **HTTP**  | 支持 Streamable HTTP 的远程 MCP 服务器    | 填写服务器 URL           |
| **SSE**   | 支持 Server-Sent Events 的远程 MCP 服务器 | 填写服务器 URL           |
| **STDIO** | 本地 MCP 服务器进程                       | 填写命令、参数、环境变量 |

认证方式支持无认证和自定义请求头（用于 Bearer Token 等场景）。

#### 2. 使用聚合 MCP Server

添加并连接 MCP 客户端后，Wheel 会自动发现所有工具并聚合为统一的 MCP Server 端点：

```
MCP Server URL: http://localhost:3000/mcp/
```

在支持 MCP 的客户端中配置此地址即可使用所有聚合工具。需要在请求头中携带 API Key：

```
Authorization: Bearer your-api-key
```

**Claude Desktop 配置示例：**

```json
{
  "mcpServers": {
    "wheel": {
      "url": "http://localhost:3000/mcp/",
      "headers": {
        "Authorization": "Bearer your-api-key"
      }
    }
  }
}
```

#### 3. REST 工具调用

对于不支持 MCP 协议的客户端，可以通过 REST API 直接调用工具：

```bash
curl http://localhost:3000/v1/mcp/tool/execute \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"clientId": 1, "toolName": "tool_name", "arguments": {}}'
```

---

## 环境变量

| 变量                | 组件   | 描述                              | 默认值                                                           |
| ------------------- | ------ | --------------------------------- | ---------------------------------------------------------------- |
| `JWT_SECRET`        | Worker | JWT 签名密钥（必填）              | —                                                                |
| `ADMIN_USERNAME`    | Worker | 管理员用户名                      | `admin`                                                          |
| `ADMIN_PASSWORD`    | Worker | 管理员密码                        | `admin`                                                          |
| `DB_DSN`            | Worker | TiDB/MySQL 连接字符串             | `root:@tcp(127.0.0.1:4000)/wheel?parseTime=true&charset=utf8mb4` |
| `PORT`              | Worker | HTTP 端口                         | `8787`                                                           |
| `VITE_API_BASE_URL` | Web    | Worker API 地址（独立部署时必填） | —                                                                |

---

## 开发

```bash
pnpm install
pnpm dev          # 同时启动 Worker + Web
```

---

## 许可证

MIT

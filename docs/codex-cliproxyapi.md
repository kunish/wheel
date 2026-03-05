# Codex (CLIProxyAPI) 渠道配置指引

本文介绍如何通过 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 将 OpenAI Codex 订阅接入 Wheel，使其可以像普通 API 渠道一样被管理、负载均衡和监控。

## 原理

CLIProxyAPI 将 Codex CLI 的 OAuth 认证包装为标准的 OpenAI 兼容 API（`/v1/chat/completions`、`/v1/models`），Wheel 将其视为一个 OpenAI 兼容渠道进行转发。

```
用户请求 → Wheel → CLIProxyAPI → Codex (OpenAI)
```

## 前置条件

1. 已安装并运行 CLIProxyAPI（[安装指引](https://help.router-for.me/)）
2. 已完成 Codex OAuth 登录：
   ```bash
   cli-proxy-api --codex-login
   ```
3. CLIProxyAPI 正在监听（默认 `http://localhost:8317`）

## 在 Wheel 中创建渠道

### 方式一：管理面板（推荐）

1. 进入 **模型 → 渠道**，点击 **创建渠道**
2. 填写：
   - **名称**：`Codex-CLIProxy`（自定义）
   - **供应商类型**：选择 `Codex (CLIProxyAPI)`
   - **基础 URL**：`http://localhost:8317`（如果 CLIProxyAPI 部署在其他地址，请相应修改）
   - **API 密钥**：填写你在 CLIProxyAPI `config.yaml` 中配置的 `api-keys` 中的一个
3. 点击 **获取模型**，确认可以拉到模型列表
4. 保存

### 方式二：API

```bash
curl http://localhost:3000/api/v1/channel/create \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Codex-CLIProxy",
    "type": 33,
    "enabled": true,
    "baseUrls": [{"url": "http://localhost:8317", "delay": 0}],
    "keys": [{"channelKey": "your-cliproxyapi-key"}],
    "model": ["gpt-5", "gpt-5-mini", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano"]
  }'
```

> `type: 33` 对应 Codex (CLIProxyAPI)。

## 常用模型

CLIProxyAPI Codex 端点通常暴露以下模型（视你的 Codex 订阅而定）：

| 模型           | 说明         |
| -------------- | ------------ |
| `gpt-5`        | GPT-5        |
| `gpt-5-mini`   | GPT-5 Mini   |
| `gpt-5-pro`    | GPT-5 Pro    |
| `gpt-4.1`      | GPT-4.1      |
| `gpt-4.1-mini` | GPT-4.1 Mini |
| `gpt-4.1-nano` | GPT-4.1 Nano |
| `o3`           | o3 推理模型  |
| `o3-pro`       | o3 Pro       |
| `o4-mini`      | o4 Mini      |

具体可用模型取决于你的 Codex 订阅套餐。点击渠道编辑页的 **获取模型** 按钮可以自动发现。

## CLIProxyAPI 最小配置参考

```yaml
# config.yaml
port: 8317
auth-dir: "~/.cli-proxy-api"
api-keys:
  - "your-api-key-for-wheel"
```

完成 OAuth 登录后即可使用，无需额外配置。

## 多账户负载均衡

CLIProxyAPI 本身支持多 Codex 账户轮询。你也可以在 Wheel 中创建多个 Codex 渠道（指向不同的 CLIProxyAPI 实例或使用不同 API Key），然后在同一分组内实现 Wheel 层面的负载均衡。

## 常见问题

### 获取模型为空

- 确认 CLIProxyAPI 已启动并监听目标端口
- 确认已完成 `--codex-login` 且 OAuth 令牌未过期
- 尝试 `curl http://localhost:8317/v1/models -H "Authorization: Bearer your-key"` 验证

### 请求超时

- CLIProxyAPI 会将请求转发到 OpenAI Codex 后端，首次请求可能较慢
- 在分组中适当增加 **首 Token 超时** 时间（建议 30-60 秒）

### OAuth 令牌过期

- 重新运行 `cli-proxy-api --codex-login` 刷新令牌
- CLIProxyAPI 支持自动刷新，通常无需手动操作

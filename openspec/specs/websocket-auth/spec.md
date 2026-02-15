## ADDED Requirements

### Requirement: WebSocket 端点 JWT 认证

WebSocket 端点 `/api/v1/ws` SHALL 在升级连接前验证 JWT token。Token 通过 query parameter `?token=xxx` 传递。

#### Scenario: 有效 token 连接成功

- **WHEN** 客户端携带有效 JWT token 请求 WebSocket 连接
- **THEN** 系统 SHALL 完成 WebSocket 升级，建立连接

#### Scenario: 无 token 拒绝连接

- **WHEN** 客户端未携带 token 请求 WebSocket 连接
- **THEN** 系统 SHALL 返回 HTTP 401，不升级连接

#### Scenario: 过期 token 拒绝连接

- **WHEN** 客户端携带过期 JWT token 请求 WebSocket 连接
- **THEN** 系统 SHALL 返回 HTTP 401，不升级连接

### Requirement: WebSocket CORS Origin 校验

WebSocket upgrader 的 `CheckOrigin` SHALL 验证请求的 Origin header 是否在允许列表中。允许列表从配置读取。

#### Scenario: 允许的 origin 连接成功

- **WHEN** 请求的 Origin 在配置的允许列表中
- **THEN** WebSocket 升级 SHALL 正常进行

#### Scenario: 未知 origin 拒绝连接

- **WHEN** 请求的 Origin 不在配置的允许列表中
- **THEN** WebSocket 升级 SHALL 被拒绝

#### Scenario: 开发模式允许 localhost

- **WHEN** 应用运行在开发模式且 Origin 为 localhost
- **THEN** WebSocket 升级 SHALL 正常进行，无需显式配置

### Requirement: 前端 WebSocket 连接附带 token

前端 `use-stats-ws.ts` 中的 WebSocket 连接 SHALL 在 URL 中附带当前用户的 JWT token。

#### Scenario: 连接时自动附带 token

- **WHEN** 前端建立 WebSocket 连接
- **THEN** 连接 URL SHALL 为 `ws://host/api/v1/ws?token=<jwt_token>`

#### Scenario: token 刷新后重连

- **WHEN** JWT token 刷新
- **THEN** WebSocket SHALL 使用新 token 重新建立连接

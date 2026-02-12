## ADDED Requirements

### Requirement: LLM 请求代理

系统 SHALL 在 Cloudflare Worker 上运行 API 代理，接收符合 OpenAI Chat Completions 格式的请求（`POST /v1/chat/completions`），根据请求中的 model 字段匹配 Group，从 Group 中选择 Channel，将请求转发到上游 Provider 并返回响应。

#### Scenario: 成功代理非流式请求

- **WHEN** 客户端发送 `POST /v1/chat/completions`，body 包含 `model: "gpt-4o"` 且 `stream: false`，携带有效的 `sk-wheel-*` API Key
- **THEN** 系统匹配到对应 Group，选择可用 Channel，将请求转换为上游格式，发送到上游 Provider，返回 200 JSON 响应

#### Scenario: 成功代理流式请求

- **WHEN** 客户端发送 `POST /v1/chat/completions`，body 包含 `stream: true`，携带有效 API Key
- **THEN** 系统返回 `Content-Type: text/event-stream` 响应，逐块转发上游 SSE 事件到客户端

#### Scenario: 无匹配 Group

- **WHEN** 请求的 model 在任何 Group 中都不存在
- **THEN** 系统返回 HTTP 404，body 包含错误信息说明模型不可用

### Requirement: Anthropic Messages API 兼容

系统 SHALL 支持接收 Anthropic Messages 格式的请求（`POST /v1/messages`），将其转换为内部格式后进行路由和代理。

#### Scenario: Anthropic 格式请求代理

- **WHEN** 客户端发送 `POST /v1/messages`，body 符合 Anthropic Messages API 格式，携带 `x-api-key` header
- **THEN** 系统将请求转换为内部格式，匹配 Group 和 Channel，转发到上游并将响应转换回 Anthropic 格式返回

### Requirement: 负载均衡

系统 SHALL 支持四种负载均衡模式：RoundRobin（轮询）、Random（随机）、Failover（故障转移）、Weighted（加权）。Group 配置中指定使用哪种模式。

#### Scenario: RoundRobin 均衡分配

- **WHEN** Group 配置为 RoundRobin 模式，包含 Channel A 和 Channel B
- **THEN** 连续请求交替分配到 Channel A 和 Channel B

#### Scenario: Failover 故障转移

- **WHEN** Group 配置为 Failover 模式，Channel A（priority=1）请求失败
- **THEN** 系统自动将请求重试到 Channel B（priority=2）

#### Scenario: Weighted 加权分配

- **WHEN** Group 配置为 Weighted 模式，Channel A weight=3，Channel B weight=1
- **THEN** 约 75% 的请求分配到 Channel A，25% 到 Channel B

### Requirement: 请求重试

系统 SHALL 在 Channel 请求失败时自动重试，最多 3 轮，每轮遍历 Group 内所有可用 Channel。

#### Scenario: 自动重试成功

- **WHEN** 第一个 Channel 返回 500 错误
- **THEN** 系统自动选择下一个 Channel 重试，成功后返回正常响应

#### Scenario: 所有 Channel 均失败

- **WHEN** Group 内所有 Channel 在 3 轮重试中均返回错误
- **THEN** 系统返回 HTTP 502，body 包含最后一次错误的详细信息

### Requirement: API Key 认证

系统 SHALL 通过 `sk-wheel-*` 格式的 API Key 认证代理请求。支持 `Authorization: Bearer` 和 `x-api-key` 两种 header 格式。

#### Scenario: 有效 API Key

- **WHEN** 请求携带有效的、已启用的、未过期的 `sk-wheel-*` API Key
- **THEN** 系统允许请求通过并进行代理

#### Scenario: 无效 API Key

- **WHEN** 请求携带不存在或已禁用的 API Key
- **THEN** 系统返回 HTTP 401

#### Scenario: 超出成本限额

- **WHEN** API Key 的累计成本已超过 `MaxCost` 限制
- **THEN** 系统返回 HTTP 403，body 说明已超出成本限额

#### Scenario: 模型白名单限制

- **WHEN** API Key 配置了 `SupportedModels`，请求的 model 不在白名单中
- **THEN** 系统返回 HTTP 403，body 说明模型不在允许列表中

### Requirement: First Token Timeout

系统 SHALL 在流式代理中检测首个 token 的响应时间。如果超过 Group 配置的 `FirstTokenTimeOut` 秒数仍未收到有效输出，SHALL 中断连接并尝试下一个 Channel。

#### Scenario: 首 token 超时触发重试

- **WHEN** 流式请求发送后，Channel A 在 `FirstTokenTimeOut` 秒内未返回任何 SSE 事件
- **THEN** 系统中断与 Channel A 的连接，选择下一个 Channel 重试

### Requirement: 请求日志记录

系统 SHALL 将每次 relay 请求的元数据异步记录到 D1 数据库，包括：请求模型、实际模型、Channel 信息、token 数量、耗时、成本、重试次数和详情。

#### Scenario: 成功请求的日志

- **WHEN** 代理请求成功完成
- **THEN** 系统使用 `waitUntil()` 异步写入 RelayLog 到 D1，不阻塞响应返回

#### Scenario: 失败请求的日志

- **WHEN** 代理请求最终失败（所有重试耗尽）
- **THEN** 系统记录日志，包含所有 Channel 的尝试详情和错误信息

## MODIFIED Requirements

### Requirement: Proxy performs single HTTP fetch

`proxyNonStreaming` 和 `proxyStreaming` SHALL 对上游只执行单次 HTTP fetch。不做任何 429 重试循环。SHALL 使用注入的 `*http.Client`（带超时配置）替代 `http.DefaultClient`。

#### Scenario: Non-streaming upstream returns 429

- **WHEN** 上游返回 HTTP 429
- **THEN** proxy SHALL 抛出 ProxyError(message, 429, retryAfterMs)，不做重试

#### Scenario: Non-streaming upstream returns 500

- **WHEN** 上游返回 HTTP 500
- **THEN** proxy SHALL 抛出 ProxyError(message, 500)，不做重试

#### Scenario: Non-streaming upstream returns 200

- **WHEN** 上游返回 HTTP 200
- **THEN** proxy SHALL 解析 JSON 响应，转换格式（如需要），返回 response + usage tokens

#### Scenario: Streaming upstream returns non-2xx before stream starts

- **WHEN** 上游返回非 2xx 状态码
- **THEN** proxy SHALL 通过 rejectFirstChunk 抛出 ProxyError，不做重试

#### Scenario: 非流式请求超时保护

- **WHEN** 上游 120 秒内未响应非流式请求
- **THEN** proxy SHALL 返回超时错误，释放连接

## ADDED Requirements

### Requirement: Proxy performs single HTTP fetch

`proxyNonStreaming` 和 `proxyStreaming` SHALL 对上游只执行单次 HTTP fetch。不做任何 429 重试循环。

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

### Requirement: ProxyError includes retry delay from upstream

ProxyError SHALL 解析上游响应中的重试延迟信息，通过 `retryAfterMs` 属性暴露。

#### Scenario: Upstream provides Retry-After header

- **WHEN** 上游 429 响应包含 `Retry-After: 2` 头
- **THEN** ProxyError.retryAfterMs SHALL 为 2000

#### Scenario: Upstream provides Google Cloud quotaResetDelay

- **WHEN** 上游响应体包含 `quotaResetDelay: "1.5s"`
- **THEN** ProxyError.retryAfterMs SHALL 为 1500

### Requirement: Remove passthrough parameter

`proxyNonStreaming` 和 `proxyStreaming` SHALL 移除 `passthrough` 参数。格式转换统一在 handler 层处理。

#### Scenario: Proxy function signatures

- **WHEN** 调用 proxyNonStreaming 或 proxyStreaming
- **THEN** 函数签名不包含 passthrough 参数

## REMOVED Requirements

### Requirement: Proxy-level 429 retry loop

**Reason**: Go 版本不在 proxy 层做 429 重试，只在 handler 层做 channel 级别重试
**Migration**: handler.ts 的 3 轮重试已覆盖此场景

## ADDED Requirements

### Requirement: No sleep between retry rounds

handler 的 3 轮重试 SHALL 不在轮次之间添加任何延迟（sleep/setTimeout）。失败后立即尝试下一个 channel。

#### Scenario: First round fails with 429

- **WHEN** 第 1 轮所有 channel 都返回 429
- **THEN** handler SHALL 立即开始第 2 轮，不等待

#### Scenario: First round fails with 500

- **WHEN** 第 1 轮所有 channel 都返回 500
- **THEN** handler SHALL 立即开始第 2 轮，不等待

### Requirement: 429 key marking in handler

当 channel key 返回 429 时，handler SHALL 异步更新 key 的 statusCode 为 429。

#### Scenario: Streaming returns 429 before first token

- **WHEN** 流式请求在首 token 前收到 429
- **THEN** handler SHALL 调用 updateChannelKeyStatus(db, key.id, 429)

#### Scenario: Non-streaming returns 429

- **WHEN** 非流式请求的 ProxyError.statusCode 为 429
- **THEN** handler SHALL 调用 updateChannelKeyStatus(db, key.id, 429)

### Requirement: Clear key status on success

当请求成功时，handler SHALL 异步重置 key 的 statusCode 为 0。

#### Scenario: Streaming succeeds after previous 429

- **WHEN** key 的 statusCode 为 429 且流式请求成功（首 token 收到）
- **THEN** handler SHALL 调用 updateChannelKeyStatus(db, key.id, 0)

#### Scenario: Non-streaming succeeds after previous 429

- **WHEN** key 的 statusCode 为 429 且非流式请求成功
- **THEN** handler SHALL 调用 updateChannelKeyStatus(db, key.id, 0)

## REMOVED Requirements

### Requirement: Exponential backoff between retry rounds

**Reason**: Go 版本不在重试之间 sleep
**Migration**: 移除 had429/retryDelay 变量和 setTimeout 逻辑

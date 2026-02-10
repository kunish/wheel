## ADDED Requirements

### Requirement: Base URL selection by lowest delay

`selectBaseUrl` SHALL 选择 `delay` 值最低的 URL，而非随机选择。

#### Scenario: Multiple base URLs with different delays

- **WHEN** channel 有 baseUrls `[{url: "https://a.com", delay: 100}, {url: "https://b.com", delay: 50}]`
- **THEN** SHALL 选择 `https://b.com`（delay=50 最低）

#### Scenario: Single base URL

- **WHEN** channel 只有一个 baseUrl
- **THEN** SHALL 返回该 URL

#### Scenario: No base URLs

- **WHEN** channel 的 baseUrls 为空数组
- **THEN** SHALL 返回 `https://api.openai.com` 作为默认值

#### Scenario: Multiple URLs with same delay

- **WHEN** channel 有多个 delay 相同的 URL
- **THEN** SHALL 返回其中任一个（取第一个）

### Requirement: 429 cooldown period is 5 minutes

key-selector 的 RATE_LIMIT_COOLDOWN SHALL 为 300 秒（5 分钟），与 Go 版一致。

#### Scenario: Key marked 429 within 5 minutes

- **WHEN** key statusCode=429 且 lastUseTimestamp 在 5 分钟内
- **THEN** key SHALL 被标记为冷却中（不优先选择）

#### Scenario: Key marked 429 超过 5 minutes

- **WHEN** key statusCode=429 且 lastUseTimestamp 超过 5 分钟前
- **THEN** key SHALL 可正常选择

### Requirement: Fallback when all keys in cooldown

当所有 enabled key 都在 429 冷却期时，key-selector SHALL fallback 到选择 lastUseTimestamp 最旧的 key。

#### Scenario: Single key in cooldown

- **WHEN** 只有 1 个 enabled key 且在冷却期
- **THEN** SHALL 返回该 key（而非 null）

#### Scenario: Multiple keys all in cooldown

- **WHEN** 3 个 enabled key 都在冷却期，timestamps 分别为 100, 200, 300
- **THEN** SHALL 返回 timestamp=100 的 key（最旧）

### Requirement: Complete OutboundType enumeration

OutboundType 枚举 SHALL 包含所有 Go 版本定义的类型。

#### Scenario: All channel types defined

- **WHEN** 检查 OutboundType 枚举
- **THEN** SHALL 包含: OpenAIChat=0, OpenAI=1, Anthropic=2, Gemini=3, Volcengine=4, OpenAIEmbedding=5

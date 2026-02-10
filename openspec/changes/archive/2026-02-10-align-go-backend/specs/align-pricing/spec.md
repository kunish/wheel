## ADDED Requirements

### Requirement: LLM price includes cache token pricing

LLM 价格表 SHALL 包含 `cacheReadPrice` 和 `cacheWritePrice` 字段，用于 Anthropic 缓存 token 定价。

#### Scenario: Price record has cache fields

- **WHEN** 查询 LLM 价格
- **THEN** 返回的记录 SHALL 包含 inputPrice, outputPrice, cacheReadPrice, cacheWritePrice

#### Scenario: models.dev sync includes cache prices

- **WHEN** 从 models.dev 同步价格
- **THEN** SHALL 提取 `cost.cache_read` 和 `cost.cache_write` 字段

### Requirement: Cost calculation handles Anthropic cache tokens

成本计算 SHALL 区分 Anthropic 和 OpenAI 的 cache token 计算方式。

#### Scenario: Anthropic response with cache tokens

- **WHEN** 响应 usage 包含 `cache_creation_input_tokens=100` 和 `cache_read_input_tokens=50`
- **THEN** inputCost = (promptTokens _ inputPrice + cacheReadTokens _ cacheReadPrice + cacheCreationTokens \* cacheWritePrice) / 1_000_000

#### Scenario: OpenAI response with cached tokens

- **WHEN** 响应 usage 包含 `prompt_tokens=1000` 和 `prompt_tokens_details.cached_tokens=200`
- **THEN** inputCost = (cachedTokens _ cacheReadPrice + (promptTokens - cachedTokens) _ inputPrice) / 1_000_000

#### Scenario: No cache tokens

- **WHEN** 响应 usage 不包含 cache token 字段
- **THEN** inputCost = promptTokens \* inputPrice / 1_000_000（向后兼容）

### Requirement: Price per million token convention

所有价格 SHALL 以 $/百万 token 为单位存储（与 models.dev 一致），计算时除以 1_000_000。

#### Scenario: Calculate cost for 1000 input tokens at $3/MTok

- **WHEN** inputTokens=1000, inputPrice=3.0
- **THEN** inputCost = 1000 \* 3.0 / 1_000_000 = 0.003

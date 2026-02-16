## ADDED Requirements

### Requirement: Channel 到 Bifrost Provider 的映射

`bifrostx.Account` SHALL 将 Wheel 的 Channel 模型映射为 Bifrost 的 Provider 模型：

- 每个 Channel 对应一个 `ModelProvider`，key 格式为 `wheel-ch-{channelID}`
- `GetConfiguredProviders()` SHALL 返回所有已启用 Channel 对应的 provider key 列表
- `GetConfigForProvider(providerKey)` SHALL 根据 provider key 反查 Channel，构建 `ProviderConfig`

#### Scenario: 列出所有 provider

- **WHEN** 调用 `GetConfiguredProviders()`
- **THEN** SHALL 返回所有 `Enabled=true` 的 Channel 对应的 provider key

#### Scenario: 禁用的 Channel 不出现

- **WHEN** Channel 的 `Enabled=false`
- **THEN** `GetConfiguredProviders()` SHALL 不包含该 Channel 的 provider key

#### Scenario: 无效 provider key

- **WHEN** `GetConfigForProvider` 收到不以 `wheel-ch-` 开头的 key
- **THEN** SHALL 返回错误

### Requirement: Channel 类型到 Bifrost BaseProvider 的映射

`baseProviderForChannelType(channelType)` SHALL 将 Wheel 的 `OutboundType` 映射为 Bifrost 的 `ModelProvider`：

- `OutboundAnthropic` → `schemas.Anthropic`
- `OutboundGemini` → `schemas.Gemini`
- 其他（OutboundOpenAI / OutboundOpenAIChat / OutboundOpenAIResponses / OutboundOpenAIEmbedding）→ `schemas.OpenAI`

#### Scenario: Anthropic 渠道映射

- **WHEN** Channel type 为 `OutboundAnthropic`
- **THEN** SHALL 返回 `schemas.Anthropic`

#### Scenario: Gemini 渠道映射

- **WHEN** Channel type 为 `OutboundGemini`
- **THEN** SHALL 返回 `schemas.Gemini`

#### Scenario: OpenAI 兼容渠道映射

- **WHEN** Channel type 为 `OutboundOpenAI` 或 `OutboundOpenAIChat`
- **THEN** SHALL 返回 `schemas.OpenAI`

### Requirement: Provider 配置构建

`GetConfigForProvider` SHALL 从 Channel 构建完整的 `ProviderConfig`，包括：

- `NetworkConfig.BaseURL` — 从 Channel 的 `BaseUrls` 中选择延迟最低的
- `NetworkConfig.ExtraHeaders` — 从 Channel 的 `CustomHeader` 转换
- `NetworkConfig.DefaultRequestTimeoutInSeconds` — 固定 60 秒
- `NetworkConfig.MaxRetries` — 固定 0（重试由 Wheel 的 executeWithRetry 管理）
- `ConcurrencyAndBufferSize` — Concurrency=128, BufferSize=1024
- `ProxyConfig` — 从 Channel 的代理配置构建（支持 HTTP/SOCKS5/环境变量代理）
- `CustomProviderConfig.BaseProviderType` — 由 `baseProviderForChannelType` 决定
- `CustomProviderConfig.RequestPathOverrides` — 按 provider 类型设置正确的 API 路径

#### Scenario: 带自定义 header 的配置

- **WHEN** Channel 配置了 CustomHeader `[{Key: "x-custom", Value: "val"}]`
- **THEN** ProviderConfig.NetworkConfig.ExtraHeaders SHALL 包含 `{"x-custom": "val"}`

#### Scenario: 带 SOCKS5 代理的配置

- **WHEN** Channel 配置了代理 URL `socks5://proxy:1080`
- **THEN** ProviderConfig.ProxyConfig SHALL 设置 Type=Socks5Proxy

#### Scenario: 无代理配置

- **WHEN** Channel 未启用代理
- **THEN** ProviderConfig.ProxyConfig SHALL 为 nil

### Requirement: 请求路径覆盖按 provider 类型设置

`requestPathOverrides(baseProvider)` SHALL 为不同 provider 类型设置正确的 API 路径：

- Anthropic → ChatCompletion 和 Stream 均为 `/v1/messages`
- OpenAI → ChatCompletion `/v1/chat/completions`、Responses `/v1/responses`
- 其他 → nil（使用 Bifrost 默认路径）

#### Scenario: Anthropic provider 路径

- **WHEN** baseProvider 为 `schemas.Anthropic`
- **THEN** ChatCompletionRequest 和 ChatCompletionStreamRequest 路径 SHALL 均为 `/v1/messages`

#### Scenario: OpenAI provider 路径

- **WHEN** baseProvider 为 `schemas.OpenAI`
- **THEN** SHALL 包含 ChatCompletion、Stream、Responses、ResponsesStream 四个路径覆盖

### Requirement: Key 选择通过 Context 注入

`GetKeysForProvider(ctx, providerKey)` SHALL 从 context 中读取 Wheel 预选的 key 信息：

- `contextKeySelectedKey` — 预选的 API key 值
- `contextKeySelectedKeyID` — 预选的 key ID
- `contextKeySelectedModel` — 预选的目标模型

如果 context 中无预选 key，SHALL 从数据库加载 Channel 的第一个启用 key。

#### Scenario: Context 中有预选 key

- **WHEN** context 包含 selectedKey="sk-xxx" 和 selectedKeyID=5
- **THEN** SHALL 返回包含该 key 的 `[]schemas.Key`，不查询数据库

#### Scenario: Context 中无预选 key

- **WHEN** context 不包含 selectedKey
- **THEN** SHALL 从数据库加载 Channel 的 keys，返回第一个 Enabled=true 的 key

#### Scenario: 无可用 key

- **WHEN** Channel 无启用的 key 且 context 无预选 key
- **THEN** SHALL 返回错误

### Requirement: Provider 动态更新

`Client.EnsureProvider(channelID)` SHALL 在每次请求前调用 `bifrost.UpdateProvider(providerKey)` 确保 provider 配置是最新的。这允许 Channel 配置变更（如 BaseURL、key 变更）在不重启的情况下生效。

#### Scenario: Channel 配置变更后生效

- **WHEN** Channel 的 BaseURL 在数据库中被修改
- **THEN** 下次请求调用 `EnsureProvider` 后 SHALL 使用新的 BaseURL

#### Scenario: Bifrost client 为 nil

- **WHEN** `Client` 或 `Client.core` 为 nil
- **THEN** `EnsureProvider` SHALL 返回 nil（不报错），由调用方处理降级

### Requirement: Bifrost Context 注入请求选择信息

`SetRequestSelection(bifrostCtx, channelID, keyID, keyValue, model)` SHALL 将 Wheel 的请求选择信息注入 Bifrost context，使 `GetKeysForProvider` 能读取预选的 key 和 model。

#### Scenario: 注入后 GetKeysForProvider 可读取

- **WHEN** 调用 `SetRequestSelection(ctx, 1, 5, "sk-xxx", "gpt-4")`
- **THEN** 后续 `GetKeysForProvider(ctx, "wheel-ch-1")` SHALL 返回 key="sk-xxx"、model="gpt-4"

### Requirement: Responses API 路径判断

`shouldUseResponsesAPI(channel)` SHALL 当 Channel 类型为 `OutboundOpenAIResponses` 时返回 true，其他情况返回 false。当返回 true 时，Bifrost 请求 SHALL 走 `ResponsesRequest` / `ResponsesStreamRequest` 路径而非 `ChatCompletionRequest`。

#### Scenario: OpenAI Responses 渠道

- **WHEN** Channel type 为 `OutboundOpenAIResponses`
- **THEN** `shouldUseResponsesAPI` SHALL 返回 true

#### Scenario: 普通 OpenAI 渠道

- **WHEN** Channel type 为 `OutboundOpenAI`
- **THEN** `shouldUseResponsesAPI` SHALL 返回 false

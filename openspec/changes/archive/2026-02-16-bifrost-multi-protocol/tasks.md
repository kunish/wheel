## 1. Bifrost Provider 适配层（bifrostx 包）

- [x]1.1 创建 `bifrostx/client.go` — Bifrost SDK 初始化、`Client` 封装、`EnsureProvider` 动态更新
- [x]1.2 创建 `bifrostx/account.go` — 实现 `Account` 接口：`GetConfiguredProviders`、`GetConfigForProvider`（Channel→ProviderConfig 映射、BaseURL 选择、CustomHeader 转换、代理配置构建、RequestPathOverrides）、`GetKeysForProvider`（Context 注入 key 读取 + 数据库 fallback）
- [x]1.3 创建 `bifrostx/context.go` — `SetRequestSelection` 将预选 Channel/Key/Model 注入 BifrostContext

## 2. 入站协议识别与模型提取

- [x]2.1 扩展 `relay/parser.go` — `DetectRequestType` 新增 Gemini 路径模式（`:generateContent` / `:streamGenerateContent`）
- [x]2.2 新增 `ExtractModelForRequest` — Gemini 从 URL 路径提取模型名，其他协议从 body 提取

## 3. 入站归一化管线

- [x]3.1 实现 `normalizeInboundToOpenAIRequest` — 按 requestType 分发到对应转换器
- [x]3.2 实现 `anthropicRequestToOpenAI` — Anthropic Messages → OpenAI Chat（system、messages、tools、tool_use/tool_result 转换）
- [x]3.3 实现 `geminiRequestToOpenAI` — Gemini generateContent → OpenAI Chat（system_instruction、contents、functionCall/functionResponse、generationConfig、tools 转换）
- [x]3.4 实现 `openAIToBifrostChatRequest` — 归一化 OpenAI → Bifrost BifrostChatRequest（messages、params、tools、tool_choice 序列化）

## 4. 响应反归一化

- [x]4.1 实现 `bifrostChatResponseToOpenAI` — BifrostChatResponse → OpenAI Chat Completion 格式
- [x]4.2 实现 `ConvertToGeminiResponseFromOpenAI` — OpenAI → Gemini 格式（candidates/parts/usageMetadata）
- [x]4.3 验证已有 `ConvertToAnthropicResponse` 与 Bifrost 输出兼容

## 5. 流式协议桥接

- [x]5.1 实现 `writeStreamChunk` / `writeStreamDone` — 按 requestType 分发写入（OpenAI 直通 / Anthropic SSE / Gemini SSE）
- [x]5.2 实现 `toGeminiStreamEventFromOpenAI` — OpenAI chunk → Gemini SSE 事件
- [x]5.3 验证已有 `createOpenAIToAnthropicSSEConverter` 与 Bifrost chunk 兼容（tool call name 延迟、finish_reason 降级）
- [x]5.4 实现 `responsesStreamEventToOpenAIChunk` + `responsesStreamBridgeState` — Responses API 流式事件 → OpenAI chunk 桥接

## 6. Bifrost 代理入口

- [x]6.1 实现 `ProxyBifrostNonStreaming` — 归一化 → Bifrost ChatCompletion/Responses 请求 → 响应转 OpenAI
- [x]6.2 实现 `ProxyBifrostStreaming` — 归一化 → Bifrost 流式请求 → chunk 转换 → writeStreamChunk 写入
- [x]6.3 实现 `proxyBifrostResponsesStreaming` — Responses API 流式路径（含 ChatResponse fallback 兼容）
- [x]6.4 实现 `normalizeResponsesRequestForProvider` — system/developer 消息提升为 instructions

## 7. Handler 集成

- [x]7.1 `config.go` 新增 `BifrostDebugRaw` 配置项，移除 `RelayExecutor`
- [x]7.2 `cmd/worker/main.go` 初始化 Bifrost Client（必须），注入 RelayHandler
- [x]7.3 `handler/relay.go` — `parseRelayRequest` 新增 Gemini 路径解析
- [x]7.4 `handler/relay.go` — `executeWithRetry` 移除 `useBifrost` 分支，所有请求直接走 Bifrost 管线；移除原有 `BuildUpstreamRequest` 调用和直连 HTTP 代理路径
- [x]7.5 `handler/relay.go` — `nonStreamStrategy.HandleSuccess` 按入站协议（Anthropic/Gemini/OpenAI）反归一化响应

## 8. 测试

- [x]8.1 `bifrostx/account_test.go` — ProviderKey 解析、baseProviderForChannelType 映射
- [x]8.2 `relay/bifrost_test.go` — 归一化管线单元测试（Anthropic→OpenAI、Gemini→OpenAI、OpenAI 直通）
- [x]8.3 `relay/bifrost_matrix_test.go` — M×N 协议组合矩阵测试（3 入站 × 3 出站 × 流式/非流式）
- [x]8.4 `relay/adapter_test.go` — 响应转换测试（OpenAI↔Anthropic、OpenAI→Gemini）
- [x]8.5 `relay/parser_test.go` — Gemini 路径识别和模型提取测试
- [x]8.6 `handler/relay_bifrost_integration_test.go` — 端到端集成测试

## Why

Wheel 需要成为一个对前端完全透明的协议适配器：用户可以用任意协议（OpenAI / Anthropic / Gemini）请求任意分组（模型），系统自动将请求归一化后路由到任意后端渠道（可能是完全不同的 provider），再将响应按前端协议格式返回。前端无需关心后端渠道实际使用的是哪种协议，实现"任意入站协议 × 任意出站 provider × 任意模型映射"的完全无感适配。

当前 `adapter.go` 中的直连模式只处理 OpenAI↔Anthropic 双向转换，缺少 Gemini 支持，且流式协议桥接逻辑散落在 `proxy.go` 中与 HTTP 代理耦合。引入 Bifrost 作为统一执行引擎，通过 "归一化 → 路由 → 反归一化" 三段式架构，让 M×N 的协议组合降为 M+N 的转换器实现。

## What Changes

- 新增 `bifrostx/` 包，封装 Bifrost SDK 的初始化、provider 动态注册、key 选择
- 新增 `relay/bifrost.go`，实现完整的归一化管线：
  - **Inbound 归一化**：`normalizeInboundToOpenAIRequest()` 将 Anthropic Messages / Gemini generateContent / OpenAI Chat 统一转为 OpenAI Chat 格式
  - **Bifrost 请求构建**：`openAIToBifrostChatRequest()` 将归一化后的 OpenAI 格式转为 Bifrost 原生请求
  - **响应反归一化**：`bifrostChatResponseToOpenAI()` + 按前端协议分发（`ConvertToAnthropicResponse` / `ConvertToGeminiResponseFromOpenAI` / 直通）
- 新增流式协议桥接：所有流式内部统一为 OpenAI SSE chunk，`writeStreamChunk()` 按 `requestType` 转换为目标 SSE 格式
- 新增 Responses API 支持：通过 `shouldUseResponsesAPI()` 判断渠道类型，走 `ResponsesRequest` / `ResponsesStreamRequest` 路径
- `parser.go` 扩展 `DetectRequestType()` 支持 Gemini 路径模式识别
- **BREAKING**：`handler/relay.go` 移除原有直连 HTTP 代理路径，所有请求强制走 Bifrost 管线；移除 `RELAY_EXECUTOR` 配置项；移除 `relayAttemptParams.UseBifrost` 标志

## Capabilities

### New Capabilities

- `bifrost-protocol-normalization`: 多协议归一化管线 — 将任意入站协议（OpenAI Chat / Anthropic Messages / Gemini generateContent）归一化为 OpenAI Chat 内部表示，经 Bifrost 路由到后端 provider 后，将响应按前端协议反归一化输出
- `bifrost-streaming-bridge`: 流式协议桥接 — 内部统一为 OpenAI SSE chunk 格式，按前端请求协议实时转换为 Anthropic SSE events / Gemini SSE events / OpenAI SSE 直通
- `bifrost-provider-adapter`: Bifrost provider 适配层 — 将 Wheel 的 Channel/Key 模型映射为 Bifrost 的 Provider/Key 模型，支持动态注册、代理配置、自定义 header 透传

### Modified Capabilities

- `simplify-proxy`: **BREAKING** 移除原有直连 HTTP 代理路径（`ProxyNonStreaming` / `ProxyStreaming`），所有请求强制走 Bifrost 管线
- `relay-handler-refactor`: 移除 `relayAttemptParams.UseBifrost` 字段，新增 `RequestType` / `IsGeminiInbound` 字段，`parseRelayRequest` 新增 Gemini 路径解析

## Impact

- **新增依赖**：`github.com/maximhq/bifrost` SDK
- **新增包**：`apps/worker/internal/bifrostx/`（client、account、context）
- **修改文件**：`relay/adapter.go`、`relay/proxy.go`、`relay/parser.go`、`handler/relay.go`、`config/config.go`、`cmd/worker/main.go`
- **配置**：移除 `RELAY_EXECUTOR` 环境变量，新增 `BIFROST_DEBUG_RAW` 调试开关
- **API 兼容性**：对前端完全透明，同一端点同时接受 OpenAI / Anthropic / Gemini 格式请求
- **BREAKING**：移除原有直连 HTTP 代理路径，Bifrost 为唯一执行引擎

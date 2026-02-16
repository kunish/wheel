## Context

Wheel 的目标是成为一个对前端完全透明的协议适配器——用户可以用任意协议（OpenAI / Anthropic / Gemini）请求任意分组（模型），系统自动将请求归一化后路由到任意后端渠道，再将响应按前端协议格式返回。前端无需关心后端渠道实际使用的是哪种 provider 协议。

引入 [Bifrost](https://github.com/maximhq/bifrost) SDK 作为唯一执行引擎。Bifrost 已内置对 OpenAI、Anthropic、Gemini 等 provider 的 HTTP 细节处理（认证、端点、请求格式），Wheel 只需负责入站/出站的协议归一化。

### 当前状态

- `adapter.go`：手动构建 HTTP 请求，处理 OpenAI↔Anthropic 格式转换
- `proxy.go`：直接发起 HTTP 请求，处理 SSE 流解析和协议转换
- `handler/relay.go`：重试循环，直接调用 proxy 函数
- 不支持 Gemini 入站协议

### 约束

- Bifrost 为唯一执行引擎，移除原有直连 HTTP 代理路径
- 前端 API 完全透明，同一端点同时接受三种协议
- Bifrost SDK 管理 provider 连接池和并发，Wheel 不做重试（由 `executeWithRetry` 管理）

## Goals / Non-Goals

**Goals:**

- M×N 协议组合降为 M+N：新增协议只需写一个归一化器和一个反归一化器
- 统一流式处理：内部全部走 OpenAI SSE chunk，按前端协议转换输出
- 复用 Bifrost 的 provider 管理能力（连接池、代理、超时、认证）
- 支持 Gemini 原生入站协议
- 支持 OpenAI Responses API 路径

**Non-Goals:**

- 不实现 Bifrost 内部的重试逻辑（MaxRetries=0）
- 不处理 embeddings 的 Bifrost 路径（当前仅 chat/messages/responses）
- 不做协议级别的 schema 校验（信任上游 SDK 的解析）

## Decisions

### Decision 1: OpenAI Chat 格式作为内部 lingua franca

**选择**：所有入站协议归一化为 OpenAI Chat Completions 格式（`map[string]any`），而非定义自有的中间表示。

**理由**：

- OpenAI 格式是事实标准，大多数 LLM 网关和客户端库都支持
- Bifrost SDK 的 `BifrostChatRequest` 本身就是 OpenAI 风格的结构
- 避免引入额外的序列化/反序列化层
- 已有的 `ConvertAnthropicResponse` / `ConvertToAnthropicResponse` 可直接复用

**替代方案**：定义 Wheel 自有的 `NormalizedRequest` struct → 增加维护成本，且与 Bifrost 的输入格式不一致需要额外转换。

### Decision 2: Bifrost 作为执行引擎而非完全替代

**选择**：Bifrost 只负责 "归一化后的请求 → provider HTTP 调用 → 响应"，Wheel 保留重试、熔断、session 粘性、负载均衡等上层逻辑。

**理由**：

- Wheel 的重试逻辑跨 Channel（不同 provider），Bifrost 的重试是单 provider 内部的
- 熔断器和 session 粘性是 Wheel 的核心功能，不应委托给 Bifrost
- `MaxRetries=0` 确保 Bifrost 不做内部重试，失败立即返回给 Wheel 的重试循环

### Decision 3: 通过 Context 注入预选 Key

**选择**：Wheel 在调用 Bifrost 前，通过 `SetRequestSelection()` 将已选择的 Channel/Key/Model 注入 `BifrostContext`，Bifrost 的 `GetKeysForProvider` 从 context 读取。

**理由**：

- Wheel 的 key 选择逻辑（轮询、权重、状态码过滤）已在 `executeWithRetry` 中完成
- Bifrost 的 key 选择是基于权重的随机选择，不满足 Wheel 的需求
- 通过 context 注入避免修改 Bifrost SDK 的接口

### Decision 4: 流式协议转换在写入时执行

**选择**：内部全部使用 OpenAI SSE chunk 格式，在 `writeStreamChunk()` 时根据 `requestType` 转换为目标格式。

**理由**：

- 统一内部格式简化了 token 追踪、content 累积、首 token 计时等逻辑
- 转换器是有状态的（需要跟踪 block index、tool call 映射等），在写入点创建一次即可
- Responses API 的流式事件也先桥接为 OpenAI chunk，复用同一套写入逻辑

### Decision 5: Provider 动态更新而非启动时固定

**选择**：每次请求前调用 `EnsureProvider(channelID)` 触发 `bifrost.UpdateProvider()`。

**理由**：

- Channel 配置（BaseURL、Key、代理）可能在运行时通过管理界面修改
- Bifrost 的 `UpdateProvider` 内部会 diff 配置，无变更时是 no-op
- 避免需要重启 worker 才能生效配置变更

## Risks / Trade-offs

- **[性能]** 每次请求调用 `EnsureProvider` 有额外开销 → Bifrost 内部做了 diff，无变更时开销极小；未来可加 TTL 缓存
- **[依赖]** 引入 `github.com/maximhq/bifrost` 外部依赖 → Bifrost 是 MIT 协议的开源项目，且 Wheel 通过 `bifrostx/` 包做了封装隔离
- **[精度]** `map[string]any` 作为中间格式可能丢失类型信息（如 int vs float64）→ 通过 `readInt` / `readFloat` 辅助函数处理类型转换
- **[Gemini 限制]** Gemini 入站强制要求 Bifrost 模式 → 不再是限制，Bifrost 已是唯一路径

## Open Questions

- Responses API 的 streaming 是否需要支持 synthetic streaming（非流式响应模拟为流式）？当前已实现 `proxyBifrostResponsesSyntheticStreaming` 但未在主路径中使用
- 是否需要支持 Gemini 的 `countTokens` 端点？当前仅支持 `generateContent` / `streamGenerateContent`

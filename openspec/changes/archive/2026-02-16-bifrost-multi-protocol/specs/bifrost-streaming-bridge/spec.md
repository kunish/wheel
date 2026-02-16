## ADDED Requirements

### Requirement: 流式内部统一为 OpenAI SSE chunk 格式

所有流式响应在内部 SHALL 统一为 OpenAI Chat Completion chunk 格式（`chat.completion.chunk` object），无论后端 provider 返回何种格式。Bifrost 的 `ChatCompletionStreamRequest` 和 `ResponsesStreamRequest` 返回的 chunk SHALL 通过 `bifrostChatResponseToOpenAI()` 或 `responsesStreamEventToOpenAIChunk()` 转为 OpenAI chunk。

#### Scenario: Bifrost ChatCompletion 流式 chunk 转换

- **WHEN** Bifrost 返回 `BifrostChatResponse` 流式 chunk
- **THEN** SHALL 通过 `bifrostChatResponseToOpenAI()` 转为 OpenAI chunk 格式

#### Scenario: Bifrost Responses API 流式事件转换

- **WHEN** Bifrost 返回 `BifrostResponsesStreamResponse` 事件
- **THEN** SHALL 通过 `responsesStreamEventToOpenAIChunk()` 转为 OpenAI chunk 格式，包括 text delta、tool call arguments delta、finish reason

### Requirement: 按前端协议写入 SSE 事件

`writeStreamChunk(w, flusher, requestType, openAIChunk, convertToAnthropic)` SHALL 根据 `requestType` 将 OpenAI chunk 转换为目标 SSE 格式写入 ResponseWriter：

- `anthropic-messages` → 通过 `convertToAnthropic` 转换为 Anthropic SSE event lines
- `gemini-stream-generate-content` → 通过 `toGeminiStreamEventFromOpenAI()` 转换为 Gemini SSE data
- 其他 → 直接写入 `data: {openAIChunk}\n\n`

#### Scenario: Anthropic 入站流式输出

- **WHEN** `requestType` 为 `anthropic-messages`
- **THEN** SHALL 将 OpenAI chunk 转换为 Anthropic SSE 事件序列（message_start / content_block_start / content_block_delta / message_delta / message_stop）

#### Scenario: Gemini 入站流式输出

- **WHEN** `requestType` 为 `gemini-stream-generate-content`
- **THEN** SHALL 将 OpenAI chunk 转换为 Gemini SSE 格式（candidates[].content.parts + usageMetadata）

#### Scenario: OpenAI 入站流式输出

- **WHEN** `requestType` 为 `openai-chat`
- **THEN** SHALL 直接写入 `data: {json}\n\n` 格式

### Requirement: 流式结束标记按协议输出

`writeStreamDone(w, flusher, requestType, convertToAnthropic)` SHALL 根据 `requestType` 写入协议对应的流结束标记：

- `anthropic-messages` → 通过 converter 输出 message_delta + message_stop 事件
- `gemini-stream-generate-content` → 输出 `data: {"done":true}\n\n`
- 其他 → 输出 `data: [DONE]\n\n`

#### Scenario: Anthropic 流结束

- **WHEN** 流结束且 `requestType` 为 `anthropic-messages`
- **THEN** SHALL 输出 Anthropic 的 message_delta（含 stop_reason）和 message_stop 事件

#### Scenario: OpenAI 流结束

- **WHEN** 流结束且 `requestType` 为 `openai-chat`
- **THEN** SHALL 输出 `data: [DONE]\n\n`

### Requirement: OpenAI → Anthropic SSE 有状态转换器

`createOpenAIToAnthropicSSEConverter()` SHALL 返回一个有状态的转换函数，维护以下状态：

- message_start 是否已发送
- 当前打开的 content block 索引和类型
- tool_call 到 tool_use block 的映射

转换规则：

- 首个 chunk → 发送 `message_start` 事件
- `delta.content` → `content_block_start`（text）+ `content_block_delta`（text_delta）
- `delta.tool_calls` → `content_block_start`（tool_use）+ `content_block_delta`（input_json_delta）
- `finish_reason` → 关闭所有 open block + `message_delta` + `message_stop`
- `[DONE]` → 如未发送 message_stop 则补发

#### Scenario: 文本内容流式转换

- **WHEN** 收到包含 `delta.content` 的 OpenAI chunk
- **THEN** SHALL 输出 Anthropic 的 content_block_delta（type=text_delta）事件

#### Scenario: Tool call 流式转换

- **WHEN** 收到包含 `delta.tool_calls` 的 OpenAI chunk，且 function name 已知
- **THEN** SHALL 输出 Anthropic 的 content_block_start（type=tool_use）和 content_block_delta（type=input_json_delta）事件

#### Scenario: Tool call name 延迟到达

- **WHEN** 收到 tool_call chunk 但 function name 尚未到达
- **THEN** SHALL 缓存 pending args，等 name 到达后一起发送 content_block_start + 累积的 delta

#### Scenario: finish_reason=tool_calls 但无有效 tool block

- **WHEN** finish_reason 为 `tool_calls` 但没有成功发送过 tool_use block
- **THEN** SHALL 将 stop_reason 降级为 `end_turn` 避免客户端挂起

### Requirement: Responses API 流式事件桥接

`responsesStreamEventToOpenAIChunk()` SHALL 将 Bifrost Responses API 的流式事件转换为 OpenAI chunk，通过 `responsesStreamBridgeState` 维护状态：

- `response.output_text.delta` → `delta.content`
- `response.function_call_arguments.delta` → `delta.tool_calls`
- `response.completed` → `finish_reason`
- `response.reasoning_summary_text.delta` → `delta.reasoning`

#### Scenario: 文本 delta 事件

- **WHEN** 收到 `OutputTextDelta` 事件
- **THEN** SHALL 转换为 OpenAI chunk，delta 包含 `content` 字段

#### Scenario: function call arguments delta 事件

- **WHEN** 收到 `FunctionCallArgumentsDelta` 事件
- **THEN** SHALL 转换为 OpenAI chunk，delta 包含 `tool_calls` 数组

#### Scenario: completed 事件

- **WHEN** 收到 `Completed` 事件
- **THEN** SHALL 生成包含 `finish_reason` 和 `usage` 的最终 chunk

#### Scenario: 错误事件

- **WHEN** 收到 `Failed` / `Incomplete` / `Error` 事件
- **THEN** SHALL 返回 ProxyError，包含上游错误信息

### Requirement: 流式 token 用量和内容追踪

流式代理过程中 SHALL 通过 `streamingState` 持续追踪：

- `inputTokens` / `outputTokens` — 从 chunk 的 usage 字段累积
- `cacheReadTokens` / `cacheCreationTokens` — 从 Bifrost usage 提取
- `responseContent` / `thinkingContent` — 累积文本内容
- `firstTokenTime` — 首个有效 token 的延迟（毫秒）

内容累积 SHALL 通过 `StreamContentCallback` 每 100 字符通知一次。

#### Scenario: 首 token 超时

- **WHEN** 配置了 `FirstTokenTimeout` 且超时前未收到有效 token
- **THEN** SHALL 返回 ProxyError(504, "First token timeout exceeded")

#### Scenario: 客户端断开

- **WHEN** 流式过程中客户端断开连接（context cancelled）
- **THEN** SHALL 返回 ProxyError(499, "Client disconnected")

#### Scenario: 空流

- **WHEN** 流关闭但未收到任何 chunk
- **THEN** SHALL 返回 ProxyError(502, "invalid upstream SSE response: no chunks received")

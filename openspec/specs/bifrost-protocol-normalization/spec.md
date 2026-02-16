## ADDED Requirements

### Requirement: 入站协议自动识别

`DetectRequestType(path)` SHALL 根据 URL 路径自动识别入站协议类型，返回以下值之一：

- `openai-chat` — 路径包含 `/chat/completions`
- `anthropic-messages` — 路径包含 `/v1/messages`
- `gemini-generate-content` — 路径包含 `:generateContent`
- `gemini-stream-generate-content` — 路径包含 `:streamGenerateContent`
- `openai-embeddings` — 路径包含 `/embeddings`
- `openai-responses` — 路径包含 `/responses`

未匹配时 SHALL 返回空字符串。

#### Scenario: OpenAI Chat 路径识别

- **WHEN** 请求路径为 `/v1/chat/completions`
- **THEN** `DetectRequestType` SHALL 返回 `openai-chat`

#### Scenario: Anthropic Messages 路径识别

- **WHEN** 请求路径为 `/v1/messages`
- **THEN** `DetectRequestType` SHALL 返回 `anthropic-messages`

#### Scenario: Gemini generateContent 路径识别

- **WHEN** 请求路径为 `/v1beta/models/gemini-pro:generateContent`
- **THEN** `DetectRequestType` SHALL 返回 `gemini-generate-content`

#### Scenario: Gemini streamGenerateContent 路径识别

- **WHEN** 请求路径为 `/v1beta/models/gemini-pro:streamGenerateContent`
- **THEN** `DetectRequestType` SHALL 返回 `gemini-stream-generate-content`

#### Scenario: 未知路径

- **WHEN** 请求路径为 `/v1/unknown`
- **THEN** `DetectRequestType` SHALL 返回空字符串

### Requirement: Gemini 路径中提取模型名

`ExtractModelForRequest` SHALL 对 Gemini 请求类型从 URL 路径中提取模型名（`/v1beta/models/{model}:action`），而非从请求体中读取。对其他协议 SHALL 从请求体的 `model` 字段提取。

#### Scenario: Gemini 路径模型提取

- **WHEN** 请求类型为 `gemini-generate-content`，路径为 `/v1beta/models/gemini-2.0-flash:generateContent`
- **THEN** SHALL 返回模型名 `gemini-2.0-flash`

#### Scenario: OpenAI 请求体模型提取

- **WHEN** 请求类型为 `openai-chat`，请求体包含 `{"model": "gpt-4"}`
- **THEN** SHALL 返回模型名 `gpt-4`

### Requirement: 入站请求归一化为 OpenAI Chat 格式

`normalizeInboundToOpenAIRequest(requestType, body)` SHALL 将任意支持的入站协议转换为 OpenAI Chat Completions 格式的 `map[string]any`。OpenAI Chat 格式作为内部 lingua franca。

#### Scenario: OpenAI Chat 直通

- **WHEN** `requestType` 为 `openai-chat`
- **THEN** SHALL 返回请求体的深拷贝，不做转换

#### Scenario: Anthropic Messages 归一化

- **WHEN** `requestType` 为 `anthropic-messages`
- **THEN** SHALL 调用 `anthropicRequestToOpenAI(body)` 转换，包括：
  - `system` 字段 → OpenAI `messages` 中 role=system 的消息
  - Anthropic `messages` → OpenAI `messages`（role 映射、content block 展开）
  - `max_tokens` / `temperature` / `top_p` / `stop_sequences` → 对应 OpenAI 参数
  - Anthropic `tools`（name/description/input_schema）→ OpenAI `tools`（type=function/function.name/parameters）
  - assistant 消息中的 `tool_use` block → OpenAI `tool_calls`
  - user 消息中的 `tool_result` block → OpenAI role=tool 消息

#### Scenario: Gemini generateContent 归一化

- **WHEN** `requestType` 为 `gemini-generate-content` 或 `gemini-stream-generate-content`
- **THEN** SHALL 调用 `geminiRequestToOpenAI(body, stream)` 转换，包括：
  - `system_instruction.parts` → OpenAI role=system 消息
  - `contents[].parts[].text` → OpenAI messages（role: user→user, model→assistant）
  - `contents[].parts[].functionCall` → OpenAI `tool_calls`
  - `contents[].parts[].functionResponse` → OpenAI role=tool 消息
  - `generationConfig`（maxOutputTokens/temperature/topP/stopSequences）→ 对应 OpenAI 参数
  - `tools[].functionDeclarations` → OpenAI `tools`

#### Scenario: 不支持的请求类型

- **WHEN** `requestType` 为不支持的值
- **THEN** SHALL 返回错误 `unsupported request type for bifrost: <type>`

### Requirement: 归一化 OpenAI 格式转 Bifrost 原生请求

`openAIToBifrostChatRequest(openAIBody, provider, targetModel)` SHALL 将归一化后的 OpenAI Chat 格式转为 `schemas.BifrostChatRequest`，包括：

- `messages` → `BifrostChatRequest.Input`（通过 JSON 序列化/反序列化）
- `max_tokens` → `ChatParameters.MaxCompletionTokens`
- `temperature` → `ChatParameters.Temperature`
- `top_p` → `ChatParameters.TopP`
- `stop` → `ChatParameters.Stop`
- `tools` → `ChatParameters.Tools`
- `tool_choice` → `ChatParameters.ToolChoice`

#### Scenario: 完整参数转换

- **WHEN** OpenAI body 包含 messages、max_tokens=4096、temperature=0.7、tools
- **THEN** SHALL 生成包含对应 Input、Params 的 BifrostChatRequest

#### Scenario: messages 为空

- **WHEN** OpenAI body 的 messages 为空数组
- **THEN** SHALL 返回错误 `messages is required`

### Requirement: 非流式响应按前端协议反归一化

Bifrost 返回的 `BifrostChatResponse` SHALL 先通过 `bifrostChatResponseToOpenAI()` 转为 OpenAI 格式，然后根据前端入站协议转换为最终响应：

- `openai-chat` → 直接返回 OpenAI 格式
- `anthropic-messages` → 通过 `ConvertToAnthropicResponse()` 转换
- `gemini-generate-content` → 通过 `ConvertToGeminiResponseFromOpenAI()` 转换

#### Scenario: Anthropic 入站 + OpenAI 后端

- **WHEN** 前端以 Anthropic Messages 格式请求，后端渠道为 OpenAI
- **THEN** Bifrost 响应 SHALL 先转为 OpenAI 格式，再转为 Anthropic Messages 格式返回

#### Scenario: Gemini 入站 + Anthropic 后端

- **WHEN** 前端以 Gemini 格式请求，后端渠道为 Anthropic
- **THEN** Bifrost 响应 SHALL 先转为 OpenAI 格式，再转为 Gemini 格式返回（candidates/parts 结构）

#### Scenario: OpenAI 入站 + OpenAI 后端

- **WHEN** 前端以 OpenAI Chat 格式请求，后端渠道为 OpenAI
- **THEN** SHALL 直接返回 OpenAI 格式响应

### Requirement: Anthropic 响应格式转换完整性

`ConvertToAnthropicResponse(openAIResp)` SHALL 将 OpenAI Chat Completion 响应转为 Anthropic Messages 响应，包括：

- `choices[0].message.content` → `content[]` 中 type=text 的 block
- `choices[0].message.tool_calls` → `content[]` 中 type=tool_use 的 block
- `choices[0].finish_reason` → `stop_reason`（stop→end_turn, length→max_tokens, tool_calls→tool_use）
- `usage.prompt_tokens` → `usage.input_tokens`
- `usage.completion_tokens` → `usage.output_tokens`

#### Scenario: 包含 tool_calls 的响应转换

- **WHEN** OpenAI 响应包含 tool_calls
- **THEN** SHALL 转换为 Anthropic content 中的 tool_use block，包含 id/name/input

#### Scenario: 纯文本响应转换

- **WHEN** OpenAI 响应只包含文本 content
- **THEN** SHALL 转换为 Anthropic content 中的 text block

### Requirement: Gemini 响应格式转换完整性

`ConvertToGeminiResponseFromOpenAI(openAIResp)` SHALL 将 OpenAI Chat Completion 响应转为 Gemini 格式，包括：

- `choices[0].message.content` → `candidates[0].content.parts[].text`
- `choices[0].message.tool_calls` → `candidates[0].content.parts[].functionCall`
- `choices[0].finish_reason` → `candidates[0].finishReason`（stop→STOP, length→MAX_TOKENS）
- `usage` → `usageMetadata`（promptTokenCount/candidatesTokenCount/totalTokenCount）

#### Scenario: 包含 functionCall 的响应转换

- **WHEN** OpenAI 响应包含 tool_calls
- **THEN** SHALL 转换为 Gemini parts 中的 functionCall，args 从 JSON string 反序列化为 object

#### Scenario: usage 转换

- **WHEN** OpenAI 响应包含 usage
- **THEN** SHALL 转换为 Gemini usageMetadata 格式

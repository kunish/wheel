## 1. 类型定义与解析逻辑

- [x] 1.1 新增 `ParsedRequestParams` 接口，包含 `model`、`stream`、`temperature`、`max_tokens`、`max_completion_tokens`、`top_p`、`frequency_penalty`、`presence_penalty`、`response_format`、`seed`、`stop`、`n`、`user` 字段（均为 optional）
- [x] 1.2 新增 `ParsedRequestTools` 接口，包含 `tools` 数组（每项含 `type`、`function.name`、`function.description`、`function.parameters`）和 `tool_choice` 字段
- [x] 1.3 新增 `parseRequestParams(content: string)` 函数，从请求 JSON 中提取非 messages 的配置参数，返回 `ParsedRequestParams | null`
- [x] 1.4 新增 `parseRequestTools(content: string)` 函数，从请求 JSON 中提取 `tools` 和 `tool_choice`，返回 `ParsedRequestTools | null`
- [x] 1.5 扩展 `ParsedResponse` 接口，新增 `id`、`model`、`created`、`systemFingerprint`、`usage`（含 `prompt_tokens`、`completion_tokens`、`total_tokens`、`prompt_tokens_details`、`completion_tokens_details`）字段
- [x] 1.6 修改 `parseResponseContent` 函数，提取并返回新增的响应元数据和 usage 字段
- [x] 1.7 扩展 `parseResponseContent` 支持多 choices 解析，返回所有 choices 而非仅 `choices[0]`

## 2. 请求参数摘要组件

- [x] 2.1 新增 `RequestParamsSummary` 组件，以 grid 布局展示请求参数（key-value 对），只显示非空字段
- [x] 2.2 `response_format` 使用 Badge 展示类型值
- [x] 2.3 `stop` 字段展示为逗号分隔的 badge 列表
- [x] 2.4 在 `MessagesTabContent` 的 Conversation 视图中，messages 列表上方插入 `RequestParamsSummary`

## 3. Tools 定义列表组件

- [x] 3.1 新增 `ToolsDefinitionList` 组件，展示 tools 数量标题 + `tool_choice` Badge
- [x] 3.2 每个 tool 渲染为可折叠卡片，默认折叠，展示 `function.name` 和 `function.description`
- [x] 3.3 展开时显示 `function.parameters` JSON Schema，使用已有的格式化 JSON 代码块样式
- [x] 3.4 在 `MessagesTabContent` 的 Conversation 视图中，`RequestParamsSummary` 之后、messages 列表之前插入 `ToolsDefinitionList`

## 4. 响应元数据与 Usage 组件

- [x] 4.1 新增 `ResponseMetadata` 子组件，在 `ResponseBlock` 内部 content/tool_calls 区域之后渲染 `id`、`model`、`created`（格式化时间戳）、`system_fingerprint`
- [x] 4.2 新增 `UsageDetails` 子组件，展示 `prompt_tokens`、`completion_tokens`、`total_tokens`，以及 `cached_tokens`、`reasoning_tokens` 等子字段注解
- [x] 4.3 将 `ResponseMetadata` 和 `UsageDetails` 嵌入 `ResponseBlock` 组件底部
- [x] 4.4 修改 `ResponseBlock` 和 `MessagesTabContent` 支持多 choices 渲染，当 choices 数 > 1 时每个 choice 独立渲染并标注 "Choice #N"

## 5. i18n 翻译

- [x] 5.1 在英文翻译文件 `apps/web/src/i18n/locales/en/logs.json` 中新增所有新 key（`messagesTab.requestParams`、`messagesTab.tools`、`messagesTab.toolChoice`、`messagesTab.responseMetadata`、`messagesTab.usage`、各字段 label 等）
- [x] 5.2 在中文翻译文件 `apps/web/src/i18n/locales/zh-CN/logs.json` 中新增对应的中文翻译

## 6. 验证

- [x] 6.1 验证包含 tools + tool_calls 的日志详情展示完整（请求参数、tools 列表、消息流、响应元数据、usage）
- [x] 6.2 验证不含 tools 的普通聊天日志不展示多余区域
- [x] 6.3 验证非 JSON 格式或截断的请求/响应不会导致错误
- [x] 6.4 验证 Raw 视图不受影响

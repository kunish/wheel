## Context

日志详情面板的 Messages 标签页当前实现了两种视图模式：

- **Conversation 视图**：解析 `requestContent` 中的 `messages` 数组，以气泡形式展示对话流；解析 `responseContent` 中的 `choices[0]` 提取 `content`、`tool_calls`、`finish_reason`
- **Raw 视图**：直接展示完整的请求/响应 JSON

问题在于 Conversation 视图丢弃了大量有价值的结构化信息（请求参数、tools 定义、响应元数据、usage 详情），用户只能切到 Raw 视图手动翻找。后端已在 `requestContent` 和 `responseContent` 字段中存储了完整 JSON，无需后端改动。

当前代码结构：

- `parseMessages(content)` → 仅提取 `messages` 数组
- `parseResponseContent(content)` → 仅提取 `choices[0].message.content/tool_calls` 和 `finish_reason`
- `MessagesTabContent` → 组合 messages + response
- `MessageBubble` → 单条消息渲染
- `ToolCallBlock` → tool call 渲染
- `ResponseBlock` → 响应内容渲染

## Goals / Non-Goals

**Goals:**

- 在 Conversation 视图中结构化展示请求参数、tools 定义、响应元数据和 usage 详情
- 保持 UI 简洁 — 使用可折叠区域，默认折叠次要信息
- 复用已有的设计语言（边框颜色、Badge、CodeBlock 等组件风格）
- 支持 i18n（中英文）

**Non-Goals:**

- 不修改后端存储逻辑或 API
- 不重构 Raw JSON 视图
- 不支持编辑/修改请求参数后重放（replay 功能已有，不在此变更范围）
- 不展示极低频字段（`logit_bias`、`logprobs`、`top_logprobs`）

## Decisions

### 1. 请求参数展示为紧凑的 key-value 网格

**选择**: 在 Conversation 视图顶部、messages 列表之前，新增一个可折叠的 `RequestParamsSummary` 区域，使用 grid 布局展示参数。

**理由**: 请求参数是"配置级"信息，不属于对话流，放在顶部作为上下文最合理。使用 grid 而非 table 可以自适应宽度。默认展开因为用户打开 Messages 标签时通常需要这些上下文。

**替代方案**: 放在单独的 tab 中 — 但增加了 tab 数量，增加了查看成本。

### 2. Tools 定义列表使用可折叠卡片

**选择**: 在请求参数区域下方、messages 列表之前，新增 `ToolsDefinitionList`，每个 tool 是一个可折叠卡片，展示 name、description，展开后显示 parameters JSON Schema。

**理由**: tools 数组可能很长（几十个 tool），必须可折叠。按卡片组织比一整块 JSON 更易扫读。

### 3. 响应元数据和 Usage 嵌入 ResponseBlock 底部

**选择**: 在 `ResponseBlock` 组件内部，content/tool_calls 之后，新增元数据区域和 usage 区域。

**理由**: 这些信息与响应紧密相关，放在同一个 block 内保持上下文关联。用浅色分隔线分隔不同区域。

### 4. 解析逻辑扩展而非重写

**选择**: 扩展 `parseResponseContent` 返回更多字段（usage、id、model 等），新增 `parseRequestParams` 函数从 requestContent 提取非 messages 字段。

**理由**: 最小化改动，保持向后兼容。如果 JSON 中某字段不存在则不渲染，不会影响现有功能。

## Risks / Trade-offs

- **[性能] 大量 tools 定义渲染** → 使用可折叠 + 默认折叠，parameters JSON 只在展开时渲染
- **[UI 拥挤] 参数过多导致区域过长** → 使用 grid 布局 + 只展示非默认值的参数（如 temperature 为 null 则不显示）
- **[兼容性] 非 OpenAI 格式的请求体** → 所有解析均用 optional chaining + 空值检查，不存在的字段静默忽略

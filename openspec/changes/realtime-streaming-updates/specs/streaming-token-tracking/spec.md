## ADDED Requirements

### Requirement: log-stream-start 事件携带定价和预估输入 token

后端在广播 `log-stream-start` WebSocket 事件时，SHALL 在 payload 中包含以下额外字段：

- `inputPrice`: 当前模型的输入 token 单价（每百万 token，USD），从定价表查询
- `outputPrice`: 当前模型的输出 token 单价（每百万 token，USD），从定价表查询
- `estimatedInputTokens`: 基于请求体大小的预估输入 token 数

#### Scenario: 有定价的模型开始流式请求

- **WHEN** 一个流式请求开始，且目标模型在定价表中有配置
- **THEN** `log-stream-start` 事件的 payload SHALL 包含该模型的 `inputPrice` 和 `outputPrice`（正数），以及基于请求体字符数估算的 `estimatedInputTokens`

#### Scenario: 无定价的模型开始流式请求

- **WHEN** 一个流式请求开始，但目标模型在定价表中没有配置
- **THEN** `log-stream-start` 事件的 payload 中 `inputPrice` 和 `outputPrice` SHALL 为 0

### Requirement: log-streaming 事件携带内容长度

后端在广播 `log-streaming` WebSocket 事件时，SHALL 在 payload 中包含 `thinkingLength` 和 `responseLength` 字段，表示当前累计的 thinking 和 response 内容的字符总长度。

#### Scenario: 流式过程中推送内容更新

- **WHEN** 后端的 `onContent` 回调被触发（每 100 字符）
- **THEN** `log-streaming` 事件的 payload SHALL 包含 `thinkingLength`（thinking 内容累计字符数）和 `responseLength`（response 内容累计字符数）

### Requirement: 前端根据内容长度估算 token 数

前端在收到 `log-streaming` 事件时，SHALL 使用内容长度字段，按 `1 token ≈ 3 字符` 的比率估算当前输出 token 数，并更新 pending 流式条目的 `outputTokens` 字段。

#### Scenario: 收到 log-streaming 事件更新 pending 条目

- **WHEN** 前端收到包含 `responseLength` 和 `thinkingLength` 的 `log-streaming` 事件
- **THEN** pending 条目的 `outputTokens` SHALL 更新为 `Math.round((responseLength + thinkingLength) / 3)`

### Requirement: 前端根据预估 token 和定价计算预估费用

前端在更新 pending 条目的 token 数后，SHALL 根据 `log-stream-start` 提供的单价信息计算预估费用，并更新 pending 条目的 `cost` 字段。

#### Scenario: pending 条目显示预估费用

- **WHEN** pending 条目的 `estimatedInputTokens` 和 `outputTokens`（预估值）已更新，且 `inputPrice` 和 `outputPrice` 非零
- **THEN** pending 条目的 `cost` SHALL 计算为 `(estimatedInputTokens × inputPrice + outputTokens × outputPrice) / 1_000_000`

#### Scenario: 无定价时费用显示为零

- **WHEN** `inputPrice` 和 `outputPrice` 均为 0
- **THEN** pending 条目的 `cost` SHALL 保持为 0

### Requirement: pending 条目视觉区分预估值

前端 SHALL 对 pending 流式条目中的预估 token 数和费用以视觉方式与精确值区分（如降低透明度），以暗示这些是预估值。

#### Scenario: 流式中的 pending 条目样式

- **WHEN** 日志表格渲染一个 `_streaming: true` 的 pending 条目
- **THEN** 该条目的 token 数和费用列 SHALL 以降低透明度（如 `opacity-50`）渲染，表示为预估值

#### Scenario: 流式完成后切换到精确值

- **WHEN** `log-created` 事件到达，pending 条目被替换为真实日志
- **THEN** 新日志条目的 token 数和费用列 SHALL 以正常透明度渲染

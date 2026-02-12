## MODIFIED Requirements

### Requirement: pending 流式条目数据模型

前端在收到 `log-stream-start` 事件时创建的 pending 条目 SHALL 包含以下额外字段：`estimatedInputTokens`（预估输入 token 数）、`inputPrice`（输入单价）、`outputPrice`（输出单价）、`cost`（预估费用，初始为 0）。`inputTokens` 字段 SHALL 使用 `estimatedInputTokens` 的值作为初始显示值。

#### Scenario: 创建 pending 条目包含完整预估数据

- **WHEN** 前端收到 `log-stream-start` 事件
- **THEN** 创建的 pending 条目 SHALL 包含：
  - `inputTokens`: 设为 `data.estimatedInputTokens`（来自后端）
  - `outputTokens`: 设为 0
  - `cost`: 设为 0
  - `_inputPrice`: 设为 `data.inputPrice`
  - `_outputPrice`: 设为 `data.outputPrice`
  - 以及现有的所有字段（`id`, `time`, `requestModelName`, `channelName`, `_streaming`, `_streamId`, `_startedAt` 等）

#### Scenario: log-streaming 事件更新 pending 条目的 token 和费用

- **WHEN** 前端收到 `log-streaming` 事件，且对应的 pending 条目存在
- **THEN** SHALL 更新该 pending 条目的 `outputTokens`（基于内容长度估算）和 `cost`（基于预估 token 数和单价计算）
- **AND** SHALL 继续更新 `useTime`（与现有行为一致）

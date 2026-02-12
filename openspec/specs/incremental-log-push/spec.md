# Spec: 增量日志推送

## Summary

将日志从"全量刷新"改为"增量推送"模式。后端广播新日志条目数据，前端直接追加而非重新查询。

## Changes

### Backend

#### `apps/worker/src/ws/hub.ts`

- `broadcast` 函数无需修改（已支持 data 参数）

#### `apps/worker/src/relay/handler.ts`

- 在异步日志写入回调中，将 `broadcast("stats-updated")` 改为两次调用：
  - `broadcast("stats-updated")`（保留，给 stats 用）
  - `broadcast("log-created", { log: { ...摘要字段 } })`
- 日志摘要字段：`id, time, requestModelName, channelName, inputTokens, outputTokens, useTime, error, cost`
- 注意：`createLog` 不返回 id，需要修改 DAL 使其返回插入的行

#### `apps/worker/src/db/dal/logs.ts`

- `createLog` 改为 `.returning()` 以返回插入的行（包含自增 id）

### Frontend

#### `apps/web/src/hooks/use-stats-ws.ts`

- 移除 `log-created` 事件对 `["logs"]` 的 invalidate
- 只保留 `stats-updated` → invalidate `["stats"]`

#### `apps/web/src/app/(protected)/logs/page.tsx`

- 添加 WebSocket 监听：收到 `log-created` 事件时
  - 如果当前在第 1 页且无筛选条件：用 `queryClient.setQueryData` 将新条目插入到 `logs` 数组头部
  - 否则：显示一个 "有新日志" 提示条，点击后 invalidate 刷新
- 添加 `useQueryClient` 和 WebSocket 事件处理逻辑

## Requirements

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

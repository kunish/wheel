## ADDED Requirements

### Requirement: Dashboard 维护进行中请求的增量状态

Dashboard SHALL 维护一个按 `streamId` 索引的增量状态 Map，用于追踪所有进行中的流式请求的预估 token 数和费用。

#### Scenario: 流式请求开始时创建增量条目

- **WHEN** Dashboard 收到 `log-stream-start` WebSocket 事件
- **THEN** SHALL 在增量状态 Map 中创建一个条目，记录 `streamId`、`estimatedInputTokens`、`outputTokens: 0`、`inputPrice`、`outputPrice`、`cost: 0`

#### Scenario: 流式过程中更新增量条目

- **WHEN** Dashboard 收到 `log-streaming` WebSocket 事件
- **THEN** SHALL 更新对应 `streamId` 的增量条目：根据 `responseLength` 和 `thinkingLength` 估算 `outputTokens`，重新计算 `cost`

#### Scenario: 流式完成时移除增量条目

- **WHEN** Dashboard 收到 `log-created` 或 `log-stream-end` WebSocket 事件
- **THEN** SHALL 从增量状态 Map 中移除对应 `streamId` 的条目

### Requirement: Dashboard 统计卡片叠加增量数据

Dashboard 的统计卡片 SHALL 将 React Query 缓存的 stats 数据与进行中请求的增量数据相加后显示。

#### Scenario: 有进行中的流式请求时统计卡片实时递增

- **WHEN** 增量状态 Map 中有一个或多个条目
- **THEN** Dashboard 的 Total Tokens 卡片 SHALL 显示 `缓存的 total tokens + 所有增量条目的 (estimatedInputTokens + outputTokens) 之和`
- **AND** Total Cost 卡片 SHALL 显示 `缓存的 total cost + 所有增量条目的 cost 之和`

#### Scenario: 无进行中请求时统计卡片正常显示

- **WHEN** 增量状态 Map 为空
- **THEN** Dashboard 统计卡片 SHALL 仅显示 React Query 缓存的 stats 数据（与当前行为一致）

### Requirement: 增量到精确值的平滑过渡

当流式请求完成时，Dashboard SHALL 通过 `AnimatedNumber` 组件实现从增量叠加值到全量刷新值的平滑动画过渡。

#### Scenario: 流式完成后 stats 刷新

- **WHEN** `log-created` 事件到达，增量条目被移除，同时 `stats-updated` 触发 React Query 刷新
- **THEN** 统计卡片的数值 SHALL 从"缓存值 + 增量值"平滑动画过渡到新的缓存值（包含了已完成请求的精确数据）

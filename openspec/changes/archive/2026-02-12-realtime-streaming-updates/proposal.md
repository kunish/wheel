## Why

当前架构中，token 计数和费用在流式请求完成后才一次性计算和推送。这导致 Dashboard 统计卡片（请求数、token 数、费用）和日志列表中的费用列在请求进行时显示为 0，请求完成后突然跳变到最终值。对于长时间运行的流式请求（如复杂推理或长对话），用户在等待期间无法感知系统正在消耗多少资源，体验不连贯。

## What Changes

- **流式过程中增量推送 token 计数**：后端在处理 SSE 流式响应的过程中，实时统计已接收的 token 数量，并通过 WebSocket 定期推送 `log-streaming` 事件的增强数据（包含当前累计的 input/output token 数和预估费用）
- **日志列表实时显示 token 和费用**：前端日志页面的 pending 流式条目在收到增量数据后，实时更新 token 数和费用列（而非一直显示 0）
- **日志详情面板实时显示费用**：流式请求的详情面板中实时展示当前累计费用
- **Dashboard 统计卡片实时递增**：Dashboard 的总 token、总费用等统计字段在有流式请求进行时，基于增量推送的数据实时递增，而非等 `stats-updated` 事件触发全量刷新

## Capabilities

### New Capabilities

- `streaming-token-tracking`: 后端在流式代理过程中实时统计 token 数量并通过 WebSocket 推送增量更新
- `realtime-dashboard-stats`: Dashboard 统计卡片在流式请求进行中基于增量数据实时更新，无需等待请求完成

### Modified Capabilities

- `incremental-log-push`: 扩展 pending 流式条目的数据模型，增加实时 token 计数和预估费用字段，使日志列表在流式过程中展示渐进式数据

## Impact

- **后端 relay handler** (`apps/worker/internal/handler/relay.go`)：需要在流式处理循环中加入 token 计数逻辑，并扩展 `log-streaming` 事件的 payload
- **WebSocket hub** (`apps/worker/internal/ws/hub.go`)：`activeStreams` 数据结构可能需要扩展以包含累计 token 数据
- **前端日志页面** (`apps/web/src/pages/logs.tsx`)：`log-streaming` 事件处理器需要更新 pending 条目的 token 和费用字段
- **前端 Dashboard** (`apps/web/src/pages/dashboard.tsx`)：需要监听流式增量数据并叠加到当前统计值上
- **API 无变更**：REST API 端点不受影响，变更仅涉及 WebSocket 事件 payload 的扩展

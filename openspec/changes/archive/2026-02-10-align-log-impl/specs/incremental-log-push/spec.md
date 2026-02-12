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

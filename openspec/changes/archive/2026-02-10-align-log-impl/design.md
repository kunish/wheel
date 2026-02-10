# Design: 对齐日志实现

## Goals

1. 日志页面不再因每个请求而全量刷新
2. 新日志条目通过 WebSocket 实时增量追加到前端
3. 支持日志自动保留清理

## Non-Goals

- 不替换 WebSocket 为 SSE
- 不实现服务端内存日志缓存
- 不实现批量 DB 写入

## Decisions

### D1: WebSocket 广播携带日志数据

当前 `broadcast("stats-updated")` 只发送事件名，前端收到后 invalidate 整个 query。

改为两个独立事件：

- `stats-updated`：仅 invalidate stats query（保持不变）
- `log-created`：携带新日志条目的摘要数据

handler.ts 在异步日志写入完成后，广播 `log-created` 事件并附带日志条目。

### D2: 前端增量追加

日志页面使用 TanStack Query 的 `queryClient.setQueryData` 将新日志条目插入到缓存数据头部（最新在前），而非 invalidate 整个 query 重新 fetch。

当用户在第 1 页时，新日志直接追加到表格顶部；在其他页时，只显示一个提示条（"有新日志"），不影响当前浏览。

### D3: 分离 stats 和 logs 的 WebSocket 事件

修改 `use-stats-ws.ts`：

- `stats-updated` 事件只 invalidate `["stats"]`
- `log-created` 事件由日志页面自行处理

### D4: 日志保留清理

添加设置项：

- `log_retention_days`：日志最大保留天数（默认 30 天）

在每次写入日志时，以 1/100 的概率触发清理检查（避免每次都清理），删除超过保留天数的旧日志。

## Architecture

```
handler.ts → createLog(db) → broadcast("log-created", { log entry })
                            → broadcast("stats-updated")

Frontend:
  use-stats-ws.ts → stats-updated → invalidate ["stats"] (不再 invalidate logs)
  logs/page.tsx   → log-created  → setQueryData 追加到列表头部
```

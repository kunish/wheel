## Why

Stats API 端点（`/stats/*`）每次请求都对 `relay_logs` 表执行全表聚合查询（SUM/AVG/COUNT），随着日志量增长查询延迟线性上升。同时 `LogWriter.Submit()` 在 channel 满时会阻塞调用方 goroutine，高流量下可能级联影响 relay 响应速度。WebSocket 广播在 flush 路径中同步执行，进一步拖慢日志持久化。这些问题在日志量达到数十万级别时会显著影响管理后台和 API 网关的响应体验。

## What Changes

- 为所有 Stats API 端点引入基于 `cache.MemoryKV` 的短 TTL 缓存层，避免每次请求都触发数据库聚合查询
- 将 `LogWriter.Submit()` 改为非阻塞模式，buffer 满时不再阻塞 relay handler，而是采用降级策略（丢弃或溢出队列）
- 将 LogWriter flush 路径中的 WebSocket 广播解耦为异步操作，不阻塞日志持久化
- 在 LogWriter flush 成功后主动失效相关 stats 缓存，保证数据一致性

## Capabilities

### New Capabilities

- `stats-caching`: Stats API 响应缓存层，基于现有 MemoryKV 实现短 TTL 缓存，flush 时主动失效
- `nonblocking-log-submit`: LogWriter 非阻塞提交机制，防止 buffer 满时阻塞 relay 请求处理

### Modified Capabilities

<!-- 无需修改现有 spec 级别的行为要求 -->

## Impact

- `apps/worker/internal/handler/stats.go` — 所有 stats handler 增加缓存读取逻辑
- `apps/worker/internal/db/logwriter.go` — Submit 改为非阻塞，flush 后失效缓存，广播异步化
- `apps/worker/internal/handler/handler.go` — Handler 结构体注入 cache 依赖
- `apps/worker/internal/cache/memory.go` — 可能需要增加批量删除（DeletePrefix）方法
- `apps/worker/cmd/worker/main.go` — 调整依赖注入接线

## Context

Wheel 是一个 LLM API 网关，使用 Go (Gin) + SQLite 架构。当前 Stats API 端点每次请求都直接查询 `relay_logs` 表执行聚合（SUM/AVG/COUNT），无任何缓存。LogWriter 使用 1000 容量的 buffered channel 做异步批量写入，但 `Submit()` 在 channel 满时会阻塞调用方。WebSocket 广播在 flush 路径中同步执行。

现有基础设施：

- `cache.MemoryKV` — 带 TTL 的内存 KV 缓存，已用于 channels/groups 缓存（TTL 300s）
- `LogWriter` — 异步批量日志写入器，50 条或 2 秒触发 flush
- `Observer` — Prometheus + OTEL 观测，nil-safe 设计

## Goals / Non-Goals

**Goals:**

- Stats API 响应时间从 O(n) 数据库聚合降低到 O(1) 缓存读取（常规场景）
- LogWriter.Submit() 在任何负载下都不阻塞 relay handler goroutine
- WebSocket 广播不阻塞日志持久化路径
- 保持数据最终一致性，缓存延迟不超过 flush 周期（~2s）

**Non-Goals:**

- 不引入 Redis 或其他外部缓存依赖
- 不重构 Stats DAL 查询本身（索引优化等属于独立变更）
- 不改变 LogWriter 的 flush 阈值策略
- 不改变 Prometheus/OTEL 指标记录方式

## Decisions

### 1. Stats 缓存策略：写失效 + 短 TTL 兜底

**选择**: LogWriter flush 后主动删除 stats 缓存 key，同时设置 30s TTL 作为兜底。

**替代方案**:

- 纯 TTL 缓存（5-10s）：简单但数据延迟不可控，dashboard 刷新可能看到旧数据
- 写时更新（flush 时重新计算并写入缓存）：flush 路径会变重，违背"不阻塞"原则

**理由**: 写失效最简单，下次请求触发重新查询并缓存。TTL 兜底防止 flush 失败时缓存永不更新。Stats 端点是管理后台使用，QPS 低，cache miss 后的一次 DB 查询完全可接受。

### 2. 缓存 key 设计

Stats handler 使用固定前缀 `stats:` + 端点名 + 参数哈希：

- `stats:global`
- `stats:channel`
- `stats:model`
- `stats:apikey`
- `stats:total`
- `stats:today:{tz}`
- `stats:daily:{tz}`
- `stats:hourly:{start}:{end}:{tz}`

flush 时调用 `cache.DeletePrefix("stats:")` 批量失效。

### 3. LogWriter 非阻塞提交：select + 丢弃 + 计数

**选择**: 使用 `select` + `default` 分支，buffer 满时丢弃日志并递增 atomic 计数器，通过 Prometheus metric 暴露丢弃数量。

**替代方案**:

- 动态扩容 channel：Go channel 不支持动态扩容，需要额外的 ring buffer 实现，复杂度高
- 溢出到磁盘队列：引入额外 I/O 路径，复杂度不匹配收益

**理由**: 日志丢弃在极端负载下是可接受的降级策略。通过 metric 暴露丢弃量，运维可以据此调整 buffer 大小或 flush 频率。relay 请求的正确响应远比日志完整性重要。

### 4. WebSocket 广播异步化

**选择**: flush 完成后，将广播操作放入独立 goroutine 执行。

**理由**: 广播是 fire-and-forget 语义，不需要等待结果。当前 flush 中逐条广播 `log-created` 事件，大批量 flush 时会显著拖慢。goroutine 开销极小，且广播本身已有 hub 内部的并发保护。

### 5. cache.MemoryKV 扩展

新增 `DeletePrefix(prefix string)` 方法，遍历 store map 删除匹配前缀的 key。在当前 stats key 数量（<10）下，遍历开销可忽略。

## Risks / Trade-offs

- **Stats 数据短暂不一致** → 最大延迟等于 flush 周期（2s）+ 下次请求时间。对管理后台场景完全可接受。WebSocket `stats-updated` 事件会触发前端刷新，此时缓存已失效。
- **日志丢弃** → 极端负载下可能丢失日志记录。通过 `wheel_log_drops_total` metric 监控，运维可调整 buffer 大小。丢弃的是持久化日志，Prometheus 指标不受影响（在 relay handler 中独立记录）。
- **DeletePrefix 性能** → 全 map 遍历。当前 cache 中 key 数量很少（channels, groups, stats），不构成问题。如果未来 key 数量增长，可改用 trie 或分桶。

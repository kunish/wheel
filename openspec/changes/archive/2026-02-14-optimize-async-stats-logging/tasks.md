## 1. MemoryKV 扩展

- [x] 1.1 在 `apps/worker/internal/cache/memory.go` 中添加 `DeletePrefix(prefix string)` 方法，遍历 store map 删除所有匹配前缀的 key
- [x] 1.2 为 `DeletePrefix` 编写单元测试，覆盖：匹配删除、无匹配无副作用、不影响其他 key

## 2. LogWriter 非阻塞提交

- [x] 2.1 修改 `LogWriter` 结构体，添加 `dropCount atomic.Int64` 字段用于记录丢弃计数
- [x] 2.2 将 `Submit()` 从阻塞式 channel 发送改为 `select` + `default` 非阻塞模式，buffer 满时丢弃并递增 `dropCount`，返回 `bool` 表示是否成功入队
- [x] 2.3 更新所有 `Submit()` 调用点（`relay.go` 中的 `asyncStreamLog`、`asyncNonStreamLog`、`asyncErrorLog`）适配新的 bool 返回值
- [x] 2.4 在 `Observer` 中注册 `wheel_log_drops_total` Prometheus counter，LogWriter 丢弃时递增该 metric
- [x] 2.5 为非阻塞 Submit 编写单元测试，覆盖：正常入队返回 true、buffer 满丢弃返回 false、丢弃计数正确

## 3. WebSocket 广播异步化

- [x] 3.1 修改 `LogWriter.flush()` 方法，将 WebSocket 广播部分（`stats-updated` 和 `log-created` 事件）移入独立 goroutine 执行
- [x] 3.2 确保广播 goroutine 中使用的数据（logs slice、entries）是安全的副本或在 flush 返回前不会被修改

## 4. Stats 缓存层

- [x] 4.1 修改 `Handler` 结构体，注入 `*cache.MemoryKV` 依赖（如尚未有）
- [x] 4.2 实现 stats 缓存辅助函数 `cachedStats[T](cache, key, ttl, queryFn)` 泛型封装缓存读取/回填逻辑
- [x] 4.3 改造 8 个 Stats handler（GetGlobalStats、GetChannelStats、GetModelStats、GetApiKeyStats、GetTotalStats、GetTodayStats、GetDailyStats、GetHourlyStats）使用缓存，key 格式为 `stats:{endpoint}:{params}`
- [x] 4.4 修改 `LogWriter` 使其持有 `*cache.MemoryKV` 引用，在 flush 成功后调用 `DeletePrefix("stats:")` 失效缓存

## 5. 依赖注入接线

- [x] 5.1 在 `main.go` 中将共享的 `cache.MemoryKV` 实例注入到 `Handler` 和 `LogWriter`
- [x] 5.2 端到端验证：启动服务，确认 stats 端点正常返回、缓存命中生效、flush 后缓存失效

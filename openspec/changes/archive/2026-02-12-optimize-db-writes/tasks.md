## 1. SQLite 配置优化

- [x] 1.1 在 `db/db.go` 的 `Open()` 函数中添加 PRAGMA：`busy_timeout = 5000`、`synchronous = NORMAL`、`cache_size = -64000`

## 2. 数据库索引

- [x] 2.1 创建迁移文件 `0008_add_relay_logs_indexes.sql`，添加 `idx_relay_logs_time` ON `relay_logs(time)`、`idx_relay_logs_channel_id` ON `relay_logs(channel_id)`、部分索引 `idx_relay_logs_error` ON `relay_logs(error)` WHERE `error != ''`

## 3. 日志批量写入服务

- [x] 3.1 创建 `db/logwriter.go`，定义 `LogWriter` 结构体：包含 buffered channel（容量 1000）、数量阈值（50）、时间阈值（2 秒）、`*bun.DB` 引用、`BroadcastFunc` 引用
- [x] 3.2 实现 `LogWriter.Submit(log, costInfo)` 方法，将日志和关联 cost 信息发送到 channel
- [x] 3.3 实现 `LogWriter.Run(ctx)` 后台消费循环：使用 `select` 多路复用 channel 接收和定时器，达到阈值时调用 flush
- [x] 3.4 实现 `LogWriter.flush()` 批量写入逻辑：在单个事务中执行批量 INSERT + cost UPDATE，写入完成后逐条发送 WebSocket `log-created` 广播
- [x] 3.5 实现 `LogWriter.Shutdown()` 方法：关闭 channel 并刷新剩余缓冲

## 4. Cost 事务合并

- [x] 4.1 在 `dal/logs.go` 中添加 `CreateLogsBatch(ctx, tx, logs)` 批量插入函数
- [x] 4.2 在 `dal/apikeys.go` 和 `dal/channels.go` 中添加接受 `bun.Tx` 参数的 cost increment 变体（或使用 `bun.IDB` 接口兼容 DB 和 Tx）

## 5. 集成到 relay handler

- [x] 5.1 修改 `RelayHandler` 结构体，添加 `LogWriter` 字段
- [x] 5.2 修改 `asyncStreamLog`、`asyncNonStreamLog`、`asyncErrorLog`，将 `dal.CreateLog` + cost 更新替换为 `LogWriter.Submit()`
- [x] 5.3 删除 `maybeCleanupLogs()` 方法及其在三个 async log 函数中的调用

## 6. 后台日志清理

- [x] 6.1 创建 `db/cleanup.go`，实现 `StartLogCleanup(ctx, db)` 函数：启动 goroutine，立即执行一次清理，之后每小时执行一次
- [x] 6.2 在应用启动入口（main 或 server init）中调用 `StartLogCleanup`

## 7. 应用生命周期

- [x] 7.1 在应用启动时初始化 `LogWriter` 并启动 `Run()` goroutine
- [x] 7.2 在 graceful shutdown 中调用 `LogWriter.Shutdown()` 确保剩余日志刷新

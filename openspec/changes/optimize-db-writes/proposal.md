## Why

每次 API relay 请求触发 3-4 次独立的 SQLite 写入（日志 INSERT + cost UPDATE × 2），在单连接（MaxOpenConns=1）限制下全部串行化，成为高并发场景的性能瓶颈。`relay_logs` 表没有任何索引，日志清理逻辑嵌在请求路径中。

## What Changes

- 引入日志批量写入机制：通过内存 channel 缓冲日志，定时或定量触发批量 INSERT
- 将关联的 cost 更新（api_key + channel_key）合并到同一事务中，减少磁盘 fsync 次数
- 优化 SQLite 连接配置：增加 busy_timeout、调整 synchronous 模式、设置 cache_size
- 为 `relay_logs` 表添加常用查询字段索引，加速过滤和清理
- 将概率性日志清理从请求路径移至独立的后台定时任务

## Capabilities

### New Capabilities

- `log-batch-writer`: 内存缓冲 + 定时/定量批量写入日志的后台服务
- `background-log-cleanup`: 独立的后台定时日志清理任务（替代请求路径中的概率清理）

### Modified Capabilities

_(无现有 spec 需要修改)_

## Impact

- **代码变更**: `db/db.go`（连接配置）、`db/dal/logs.go`（批量写入）、`handler/relay.go`（异步日志调用方式、事务合并、移除 maybeCleanupLogs）
- **新增迁移文件**: 添加 `relay_logs` 表索引（time、channel_id、request_model_name）
- **行为变更**: 日志写入从同步变为异步批量，存在极端情况下丢失少量未刷新日志的可能（进程崩溃时）
- **依赖**: 无新依赖

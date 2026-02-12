## Context

Wheel 是一个 LLM API 代理网关，使用 SQLite + Bun ORM 作为持久化层。当前每次 relay 请求在异步 goroutine 中触发 3-4 次独立数据库写入：1 次日志 INSERT、2 次 cost UPDATE（api_key + channel_key），以及条件性的 key status UPDATE。

数据库连接被限制为 `MaxOpenConns=1`，所有操作串行化。`relay_logs` 表无任何索引，日志清理以 1% 概率嵌在请求 goroutine 中执行。

## Goals / Non-Goals

**Goals:**

- 将日志写入吞吐量提升一个数量级（通过批量 INSERT）
- 减少每次请求产生的磁盘 fsync 次数（事务合并）
- 优化 SQLite PRAGMA 配置以提升写入性能
- 为 `relay_logs` 高频查询字段添加索引
- 将日志清理移至可控的后台任务

**Non-Goals:**

- 不更换数据库引擎（保持 SQLite）
- 不修改日志的数据结构/字段
- 不改变前端 WebSocket 推送的实时性（log-created 事件仍在写入后立即发送）

## Decisions

### 1. 日志批量写入：channel 缓冲 + 定时器双触发

**选择**: 使用 Go channel 作为缓冲队列，后台 goroutine 消费，同时设置 **数量阈值**（50 条）和 **时间阈值**（2 秒）双触发刷新。

**替代方案**:

- sync.Mutex + slice 缓冲：更简单但无背压能力，高并发下 append 竞争严重
- 外部消息队列（Redis/NATS）：引入新依赖，对于单实例 SQLite 过度设计

**理由**: channel 天然支持多生产者单消费者模式，有背压能力（channel 满时阻塞），且 Go 原生支持 select 多路复用定时器。

### 2. Cost 更新事务合并

**选择**: 将 `IncrementApiKeyCost` 和 `IncrementChannelKeyCost` 合并为单个事务 `IncrementCosts(tx, apiKeyId, channelKeyId, cost)`。

**理由**: 两次 UPDATE 本身就是语义上的原子操作（一笔费用同时记到 api_key 和 channel_key），应当保证一致性。合并事务减少一次 WAL checkpoint。

### 3. SQLite PRAGMA 优化

**选择**: 在 `db.Open()` 中增加以下配置：

- `PRAGMA busy_timeout = 5000`：写冲突时等待 5 秒而非立即报错
- `PRAGMA synchronous = NORMAL`：WAL 模式下 NORMAL 已提供足够的数据安全性，相比 FULL 显著减少 fsync
- `PRAGMA cache_size = -64000`：64MB 页缓存（默认仅 2MB）

**替代方案**:

- `synchronous = OFF`：性能最好但进程崩溃可能损坏数据库，不可接受

**理由**: SQLite 官方文档明确建议 WAL + NORMAL 组合。busy_timeout 避免并发写入时的 `database is locked` 错误。

### 4. 索引策略

**选择**: 添加以下索引：

- `idx_relay_logs_time` ON `relay_logs(time)`：加速时间范围查询和日志清理
- `idx_relay_logs_channel_id` ON `relay_logs(channel_id)`：加速 channel 过滤
- `idx_relay_logs_error` ON `relay_logs(error)` WHERE `error != ''`：部分索引，仅索引有错误的行

**不添加的索引**:

- `request_model_name`：使用 LIKE 模糊查询，普通 B-tree 索引帮助有限
- 全文索引：keyword 搜索是管理后台低频操作，不值得写入开销

### 5. 后台日志清理

**选择**: 在应用启动时创建一个独立 goroutine，每小时执行一次清理，替代当前请求路径中的 1% 概率清理。

**理由**: 可预测的清理时机，不会影响请求延迟。

### 6. MaxOpenConns 保持为 1

**选择**: 不增加 MaxOpenConns。

**理由**: SQLite 写入本质上是串行的（即使 WAL 模式也只允许一个写者）。设置多个连接只会增加 `database is locked` 的概率。通过 busy_timeout 和批量写入已经充分优化了写入吞吐。Bun ORM 层面也不支持独立的读写连接池分离。

## Risks / Trade-offs

- **[日志延迟写入]** → 批量缓冲引入最多 2 秒延迟。对于管理后台的日志查看可接受。WebSocket 推送的 `log-created` 事件改为在批量写入完成后逐条发送。
- **[进程崩溃丢日志]** → 缓冲区中未刷新的日志（最多 50 条或 2 秒内的日志）将丢失。可通过 graceful shutdown 中调用 `Flush()` 缓解。
- **[索引增加写入开销]** → 3 个索引会略微增加 INSERT 开销，但被批量写入的事务合并效应抵消。
- **[synchronous=NORMAL]** → 操作系统崩溃（非进程崩溃）时理论上可能丢失最近一次 WAL checkpoint 的数据。对于日志数据可接受。

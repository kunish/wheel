## ADDED Requirements

### Requirement: LogWriter 非阻塞提交

`LogWriter.Submit()` SHALL 在任何情况下都不阻塞调用方 goroutine。当内部 buffer 已满时 SHALL 丢弃该日志条目而非阻塞等待。

#### Scenario: buffer 未满时正常入队

- **WHEN** 调用 `Submit()` 且内部 channel 未满
- **THEN** 日志条目 SHALL 被成功入队，调用方立即返回

#### Scenario: buffer 已满时丢弃并计数

- **WHEN** 调用 `Submit()` 且内部 channel 已满
- **THEN** 该日志条目 SHALL 被丢弃，丢弃计数器 SHALL 原子递增 1，调用方立即返回

#### Scenario: 丢弃不影响已入队日志

- **WHEN** 一个日志条目因 buffer 满被丢弃
- **THEN** 已在 buffer 中的日志条目 SHALL 不受影响，正常参与后续 flush

### Requirement: 日志丢弃指标暴露

系统 SHALL 通过 Prometheus metric `wheel_log_drops_total` 暴露日志丢弃总数，供运维监控。

#### Scenario: 丢弃时指标递增

- **WHEN** 一个日志条目因 buffer 满被丢弃
- **THEN** `wheel_log_drops_total` counter SHALL 递增 1

#### Scenario: 无丢弃时指标为零

- **WHEN** 系统正常运行且 buffer 从未满
- **THEN** `wheel_log_drops_total` SHALL 保持为 0

### Requirement: WebSocket 广播异步化

LogWriter flush 完成后的 WebSocket 广播（`stats-updated` 和 `log-created` 事件）SHALL 在独立 goroutine 中异步执行，不阻塞 flush 路径。

#### Scenario: flush 后异步广播

- **WHEN** LogWriter 成功完成一次 batch flush
- **THEN** WebSocket 广播 SHALL 在独立 goroutine 中执行，flush 方法 SHALL 在启动广播 goroutine 后立即返回

#### Scenario: 广播失败不影响日志持久化

- **WHEN** WebSocket 广播过程中发生错误
- **THEN** 已持久化的日志数据 SHALL 不受影响

### Requirement: Submit 返回丢弃状态

`LogWriter.Submit()` SHALL 返回一个 bool 值，指示日志是否被成功入队。

#### Scenario: 成功入队返回 true

- **WHEN** 日志条目成功入队到 buffer
- **THEN** `Submit()` SHALL 返回 `true`

#### Scenario: 丢弃返回 false

- **WHEN** 日志条目因 buffer 满被丢弃
- **THEN** `Submit()` SHALL 返回 `false`

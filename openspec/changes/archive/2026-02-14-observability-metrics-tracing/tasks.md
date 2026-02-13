## 1. 依赖与配置

- [x] 1.1 添加 OTel 和 Prometheus Go 依赖到 `apps/worker/go.mod`
- [x] 1.2 扩展 `internal/config/config.go`，添加 `MetricsEnabled`、`OtelEnabled`、`OtelEndpoint`、`OtelServiceName` 字段及环境变量解析

## 2. observe 包核心

- [x] 2.1 创建 `internal/observe/observe.go`：Observer struct、New() 构造函数（Prometheus exporter + OTLP TracerProvider）、Shutdown()、MetricsHandler()、注册 9 个 metric instruments
- [x] 2.2 创建 `internal/observe/metrics.go`：10 个 nil-safe 指标记录方法（RecordRequest、RecordError、RecordDuration、RecordTTFB、RecordRetry、RecordTokens、RecordCost、SetCircuitBreakerState、StreamStarted、StreamEnded）
- [x] 2.3 创建 `internal/observe/tracing.go`：StartRelaySpan、StartAttemptSpan、EndAttemptSpan、AddCircuitBreakerEvent，nil-safe 实现

## 3. 埋点集成

- [x] 3.1 修改 `internal/handler/relay.go`：RelayHandler 添加 Observer 字段，handleRelay 入口创建 relay span
- [x] 3.2 修改 `internal/handler/relay.go`：重试循环内创建 attempt span，失败时 EndAttemptSpan + RecordRetry，成功时 EndAttemptSpan
- [x] 3.3 修改 `internal/handler/relay.go`：streaming 路径添加 StreamStarted/StreamEnded，熔断器跳过处添加 AddCircuitBreakerEvent
- [x] 3.4 修改 `internal/handler/relay.go`：asyncStreamLog 和 asyncNonStreamLog 中记录 RecordRequest、RecordDuration、RecordTokens、RecordCost、RecordTTFB
- [x] 3.5 修改 `internal/handler/relay.go`：exhaustion 路径记录 RecordRequest、RecordError、RecordDuration

## 4. 熔断器集成

- [x] 4.1 修改 `internal/relay/circuit.go`：定义 CircuitObserver interface 和 SetCircuitObserver setter
- [x] 4.2 修改 `internal/relay/circuit.go`：RecordFailure 中 state→open 时调用 SetCircuitBreakerState(+1)，RecordSuccess 中 state→closed 时调用 SetCircuitBreakerState(-1)

## 5. 应用入口接线

- [x] 5.1 修改 `cmd/worker/main.go`：初始化 Observer，注入 RelayHandler，注册 `/metrics` 端点，设置熔断器回调，defer Shutdown

## 6. 验证

- [x] 6.1 `go build ./cmd/worker` 编译通过
- [x] 6.2 `METRICS_ENABLED=true` 启动后 `curl /metrics` 返回 Prometheus 格式且包含所有 `wheel_*` 指标
- [x] 6.3 两者都禁用时 `/metrics` 返回 404，无额外 goroutine

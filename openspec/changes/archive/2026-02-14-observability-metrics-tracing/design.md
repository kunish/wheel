## Context

Wheel Worker 是一个 Go 编写的 LLM API Gateway，负责将入站请求路由到多个上游 LLM provider，支持重试、熔断、负载均衡。当前已有 WebSocket 实时仪表盘和 SQLite 请求日志，但无法与 Prometheus/Grafana 告警体系或 Jaeger/Tempo 分布式追踪集成。

现有代码结构：

- `internal/handler/relay.go` — relay 请求处理，包含重试循环和异步日志
- `internal/relay/circuit.go` — 熔断器实现
- `cmd/worker/main.go` — 应用入口和路由注册

## Goals / Non-Goals

**Goals:**

- 通过 `/metrics` 端点暴露 Prometheus 格式指标，覆盖请求、错误、token、费用、延迟等核心维度
- 通过 OTLP gRPC 导出分布式追踪 span，支持 relay → attempt 层级结构
- 默认关闭，零开销；通过环境变量按需启用
- 所有指标只定义一次（OTel API），避免双重埋点

**Non-Goals:**

- 不实现自定义 Grafana dashboard JSON（用户自行配置）
- 不实现日志导出（现有 SQLite 日志已满足需求）
- 不实现 push-based metrics（只做 pull/scrape 模式）
- 不为 `/metrics` 端点添加认证（遵循 Prometheus 标准抓取模式）

## Decisions

### D1: OTel Prometheus Bridge 而非独立 Prometheus client

使用 `go.opentelemetry.io/otel/exporters/prometheus` 作为 MeterProvider 的 reader，所有指标通过 OTel Metric API 定义。Prometheus exporter 自动将 OTel instruments 转换为 Prometheus 格式。

**替代方案**: 直接使用 `prometheus/client_golang` 定义指标 + 单独用 OTel API 定义 trace。
**选择理由**: Bridge 方案只需定义一次指标，避免维护两套指标定义的同步问题。OTel 是 CNCF 标准，未来可扩展到 OTLP metrics push。

### D2: nil-receiver 模式实现零开销禁用

`Observer` struct 的所有方法以 `if o == nil { return }` 开头。当 metrics/tracing 都禁用时，`New()` 返回 `nil, nil`，所有调用点无需条件判断。

**替代方案**: 定义 noop interface 实现。
**选择理由**: nil-receiver 是 Go 惯用模式，代码更简洁，无需维护 interface + noop 实现。

### D3: 熔断器通过 interface 解耦

`relay/circuit.go` 定义 `CircuitObserver` interface，`observe` 包实现它。避免 `relay` → `observe` 的直接依赖。

**替代方案**: 直接在 circuit.go 中 import observe 包。
**选择理由**: 避免循环依赖风险，保持 relay 包的独立性。

### D4: 异步日志函数中记录指标

指标记录放在 `asyncStreamLog` / `asyncNonStreamLog` 中（goroutine 内），因为此时 token 数量和费用已计算完成。使用 `context.Background()` 避免请求 context 取消问题。

## Risks / Trade-offs

- [高基数标签] `api_key` 标签在 `wheel_requests_total` 上可能导致 Prometheus 高基数问题 → 生产环境可通过 relabel 配置丢弃该标签
- [gRPC 连接] `OTEL_ENABLED=true` 时会建立到 OTLP endpoint 的持久 gRPC 连接 → 如果 endpoint 不可达，OTel SDK 会静默重试，不影响请求处理
- [goroutine 中的指标记录] 异步日志函数中记录指标，理论上存在微小的计时偏差 → 对于监控场景可接受
